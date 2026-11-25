package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Weekly meal plan (merged from the household "rumahbot" Telegram bot):
//   - a 7-day menu (one protein dish + one vegetable dish per day), the same
//     all week (seeded by ISO week), honouring the household's must-use
//     ingredients (mandatory_ingredients) and preferring dishes whose main
//     ingredient is on promo (grocery_promos, scraped by grocery_scrape.go),
//   - at most two fried dishes a week,
//   - optionally posted to the household chat on a schedule
//     (reminder_schedules job_name='meal_plan', default Friday 06:00 local).
//
// Endpoints:
//   session: GET /meal-plan[?week=this], GET/POST /meal-plan/ingredients,
//            DELETE /meal-plan/ingredients/{id}, GET/PUT /meal-plan/schedule
//   bot:     GET /bot/meal-plan[?week=this], GET/POST /bot/ingredients,
//            POST /bot/ingredients/remove, GET/POST /bot/meal-schedule

const mealJob = "meal_plan"

// A dish group is one protein family. Group and dish Keys are matching
// vocabulary for typed must-use ingredients and for store promo names, which
// are often Indonesian. They are never shown in the UI; only Name and Protein
// are.
type mealDish struct {
	Name  string
	Fried bool
	Keys  []string
}

type mealDishGroup struct {
	Protein string
	Keys    []string
	Dishes  []mealDish
}

type mealSide struct {
	Name string
	Keys []string
}

var mealProteins = []mealDishGroup{
	{"Chicken", []string{"chicken", "ayam"}, []mealDish{
		{Name: "Lemon roast chicken"},
		{Name: "Grilled chicken with herbs"},
		{Name: "Chicken pot pie"},
		{Name: "Crispy fried chicken", Fried: true},
		{Name: "Chicken curry"},
	}},
	{"Fish", []string{"fish", "salmon", "tuna", "cod", "mackerel", "ikan", "tongkol", "lele", "kembung", "nila"}, []mealDish{
		{Name: "Baked salmon with lemon"},
		{Name: "Fish and chips", Fried: true},
		{Name: "Tuna pasta bake"},
		{Name: "Grilled mackerel"},
		{Name: "Fish pie"},
	}},
	{"Tofu & tempeh", []string{"tofu", "tempeh", "tahu", "tempe"}, []mealDish{
		{Name: "Tofu stir-fry"},
		{Name: "Crispy fried tofu", Fried: true},
		{Name: "Tempeh burgers"},
		{Name: "Baked tofu with soy glaze"},
		{Name: "Tofu and vegetable curry"},
	}},
	{"Eggs", []string{"egg", "eggs", "telur"}, []mealDish{
		{Name: "Spanish omelette"},
		{Name: "Shakshuka"},
		{Name: "Vegetable frittata"},
		{Name: "Egg fried rice", Fried: true},
		{Name: "Cheese and onion quiche", Keys: []string{"cheese", "onion"}},
	}},
	{"Prawns & squid", []string{"prawn", "prawns", "shrimp", "squid", "calamari", "udang", "cumi"}, []mealDish{
		{Name: "Garlic butter prawns", Keys: []string{"garlic"}},
		{Name: "Fried calamari", Fried: true},
		{Name: "Prawn linguine"},
		{Name: "Shrimp tacos"},
	}},
	{"Beef", []string{"beef", "mince", "steak", "daging", "sapi"}, []mealDish{
		{Name: "Beef stew"},
		{Name: "Spaghetti bolognese"},
		{Name: "Cottage pie"},
		{Name: "Beef burgers"},
		{Name: "Roast beef"},
	}},
}

var mealVeggies = []mealSide{
	{"Steamed broccoli", []string{"broccoli", "brokoli"}},
	{"Garlic green beans", []string{"green beans", "beans", "buncis"}},
	{"Spinach salad", []string{"spinach", "bayam"}},
	{"Creamed spinach", []string{"spinach", "bayam"}},
	{"Honey glazed carrots", []string{"carrot", "carrots", "wortel"}},
	{"Coleslaw", []string{"cabbage", "kol"}},
	{"Garden salad", []string{"lettuce", "salad", "daun"}},
	{"Sauteed greens", []string{"greens", "kale", "kangkung", "sawi"}},
	{"Stir-fried bok choy", []string{"bok choy", "pak choi", "sawi"}},
	{"Bean sprout salad", []string{"bean sprouts", "sprouts", "tauge"}},
	{"Buttered peas", []string{"peas", "kacang"}},
	{"Roasted vegetables", []string{"vegetables", "sayur"}},
	{"Corn on the cob", []string{"corn", "jagung"}},
	{"Mashed potatoes", []string{"potato", "potatoes", "kentang"}},
	{"Roasted cauliflower", []string{"cauliflower"}},
	{"Grilled asparagus", []string{"asparagus"}},
	{"Minestrone soup", []string{"beans", "vegetables", "sayur"}},
	{"Ratatouille", []string{"eggplant", "zucchini", "tomato", "terong", "tomat"}},
}

// mealEntry is one catalog dish with its group, flattened for lookups.
// keys is the group keys plus the dish keys.
type mealEntry struct {
	name      string
	fried     bool
	groupKeys []string
	dish      mealDish
	keys      []string
}

var (
	mealEntries     = buildMealEntries()
	mealEntryNames  = mealNames(mealEntries)
	mealSideNames   = sideNames(mealVeggies)
	mealFriedByName = func() map[string]bool {
		m := map[string]bool{}
		for _, e := range mealEntries {
			m[e.name] = e.fried
		}
		return m
	}()
	mealKeysByName = func() map[string][]string {
		m := map[string][]string{}
		for _, e := range mealEntries {
			m[e.name] = e.keys
		}
		for _, s := range mealVeggies {
			m[s.Name] = s.Keys
		}
		return m
	}()

	// Unions of the catalog keys, used to sort a promo into protein or side.
	promoProteinWords = proteinKeyUnion()
	promoVeggieWords  = sideKeyUnion()
)

func buildMealEntries() []mealEntry {
	var out []mealEntry
	for _, g := range mealProteins {
		for _, d := range g.Dishes {
			keys := append(append([]string(nil), g.Keys...), d.Keys...)
			out = append(out, mealEntry{name: d.Name, fried: d.Fried, groupKeys: g.Keys, dish: d, keys: keys})
		}
	}
	return out
}

func mealNames(es []mealEntry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.name
	}
	return out
}

func sideNames(ss []mealSide) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Name
	}
	return out
}

func uniqueKeys(lists ...[]string) []string {
	var out []string
	for _, l := range lists {
		for _, k := range l {
			if !slices.Contains(out, k) {
				out = append(out, k)
			}
		}
	}
	return out
}

func proteinKeyUnion() []string {
	var lists [][]string
	for _, e := range buildMealEntries() {
		lists = append(lists, e.keys)
	}
	return uniqueKeys(lists...)
}

func sideKeyUnion() []string {
	var lists [][]string
	for _, s := range mealVeggies {
		lists = append(lists, s.Keys)
	}
	return uniqueKeys(lists...)
}

var mealDayNames = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

type mealPromo struct {
	Store string `json:"store"`
	Name  string `json:"name"`
	Price string `json:"price"`
	Link  string `json:"link"`
}

type mealDay struct {
	Day     string     `json:"day"`
	Date    string     `json:"date"`
	Protein string     `json:"protein"`
	Veggie  string     `json:"veggie"`
	Fried   bool       `json:"fried"`
	Promo   *mealPromo `json:"promo"`
	Picks   []mealPick `json:"picks"` // approved member requests for this day
}

type mealPlan struct {
	WeekStart   string      `json:"week_start"`
	WeekEnd     string      `json:"week_end"`
	Days        []mealDay   `json:"days"`
	Ingredients []string    `json:"ingredients"`
	Promos      []mealPromo `json:"promos"`      // promos that matched a dish this week
	PromoCount  int         `json:"promo_count"` // promos available when the plan was built
	Missing     []string    `json:"missing"`     // must-use ingredients no dish in the catalog uses
}

// isFried reads the catalog flag, and falls back to the word "fried" for a
// name that is not in the catalog.
func isFried(dish string) bool {
	if f, ok := mealFriedByName[dish]; ok {
		return f
	}
	return strings.Contains(strings.ToLower(dish), "fried")
}

// wordsOf: lower-case ASCII letter runs.
func wordsOf(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !(r >= 'a' && r <= 'z') })
}

// textHasKey: do the words contain key (one or more words) as a run, the last
// word allowed to be a plural ("prawns", "tomatoes")? Whole words only, so
// "eggplant" does not count as "egg".
func textHasKey(words []string, key string) bool {
	kw := wordsOf(key)
	if len(kw) == 0 {
		return false
	}
	for i := 0; i+len(kw) <= len(words); i++ {
		ok := true
		for j, k := range kw {
			w := words[i+j]
			if j == len(kw)-1 {
				ok = w == k || w == k+"s" || w == k+"es"
			} else {
				ok = w == k
			}
			if !ok {
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func textHasAnyKey(text string, keys []string) bool {
	words := wordsOf(text)
	for _, k := range keys {
		if textHasKey(words, k) {
			return true
		}
	}
	return false
}

// keysUse: does a dish or side with this name and keys use the ingredient?
// True when the name contains it (3+ letters), or when it equals or contains a
// key as a whole word.
func keysUse(name string, keys []string, ing string) bool {
	ing = strings.ToLower(strings.TrimSpace(ing))
	if ing == "" {
		return false
	}
	if len(ing) >= 3 && strings.Contains(strings.ToLower(name), ing) {
		return true
	}
	return textHasAnyKey(ing, keys)
}

// dishUses: does the dish (in a group with groupKeys) use the ingredient?
func dishUses(groupKeys []string, d mealDish, ing string) bool {
	return keysUse(d.Name, append(append([]string(nil), groupKeys...), d.Keys...), ing)
}

func sideUses(s mealSide, ing string) bool { return keysUse(s.Name, s.Keys, ing) }

func containsAnyWord(text string, words []string) bool {
	text = strings.ToLower(text)
	for _, w := range words {
		if strings.Contains(text, w) {
			return true
		}
	}
	return false
}

// mealWeekStart is the Monday the plan covers: the next Monday, or today if
// today is a Monday (rumahbot posts on Friday for the coming week). this=true
// gives the Monday of the current week instead.
func mealWeekStart(localToday time.Time, this bool) time.Time {
	d := dateOnly(localToday)
	wd := (int(d.Weekday()) + 6) % 7 // Monday=0 .. Sunday=6
	if this {
		return d.AddDate(0, 0, -wd)
	}
	if wd == 0 {
		return d
	}
	return d.AddDate(0, 0, 7-wd)
}

// generateMealPlan is pure (unit tested): same inputs, same plan.
func generateMealPlan(weekStart time.Time, ingredients []string, promos []mealPromo) mealPlan {
	year, week := weekStart.ISOWeek()
	rng := rand.New(rand.NewSource(int64(year*100 + week)))
	ings := append([]string(nil), ingredients...)
	sort.Strings(ings)

	allDishes := mealEntryNames
	var proteinPromos, veggiePromos []mealPromo
	for _, p := range promos {
		switch {
		case textHasAnyKey(p.Name, promoProteinWords):
			proteinPromos = append(proteinPromos, p)
		case textHasAnyKey(p.Name, promoVeggieWords):
			veggiePromos = append(veggiePromos, p)
		}
	}

	// 1. must-use ingredients: force one dish (protein, else vegetable) each
	var proteins, veggies, missing []string
	forced := map[string]bool{}
	for _, ing := range ings {
		pick := func(pool []string, uses func(name string) bool, taken []string) string {
			var c []string
			for _, d := range pool {
				if uses(d) && !slices.Contains(taken, d) {
					c = append(c, d)
				}
			}
			if len(c) == 0 {
				return ""
			}
			return c[rng.Intn(len(c))]
		}
		if len(proteins) < 7 {
			usesDish := func(name string) bool {
				for _, e := range mealEntries {
					if e.name == name {
						return dishUses(e.groupKeys, e.dish, ing)
					}
				}
				return false
			}
			if d := pick(allDishes, usesDish, proteins); d != "" {
				proteins = append(proteins, d)
				forced[d] = true
				continue
			}
		}
		if len(veggies) < 7 {
			usesSide := func(name string) bool {
				for _, sd := range mealVeggies {
					if sd.Name == name {
						return sideUses(sd, ing)
					}
				}
				return false
			}
			if d := pick(mealSideNames, usesSide, veggies); d != "" {
				veggies = append(veggies, d)
				continue
			}
		}
		missing = append(missing, ing)
	}

	// 2. fill proteins, preferring dishes whose group or dish keys appear in a
	// protein promo name
	for len(proteins) < 7 {
		var cand []string
		if len(proteinPromos) > 0 {
			for _, d := range allDishes {
				if !slices.Contains(proteins, d) && promoUsesDish(proteinPromos, d) {
					cand = append(cand, d)
				}
			}
		}
		if len(cand) == 0 {
			for _, d := range allDishes {
				if !slices.Contains(proteins, d) {
					cand = append(cand, d)
				}
			}
		}
		proteins = append(proteins, cand[rng.Intn(len(cand))])
	}
	for len(veggies) < 7 {
		var cand []string
		for _, v := range mealSideNames {
			if !slices.Contains(veggies, v) {
				cand = append(cand, v)
			}
		}
		veggies = append(veggies, cand[rng.Intn(len(cand))])
	}
	rng.Shuffle(len(proteins), func(i, j int) { proteins[i], proteins[j] = proteins[j], proteins[i] })
	rng.Shuffle(len(veggies), func(i, j int) { veggies[i], veggies[j] = veggies[j], veggies[i] })

	// 3. at most two fried dishes; must-use picks are kept (counted first)
	fried := 0
	for p := range forced {
		if isFried(p) {
			fried++
		}
	}
	for i, p := range proteins {
		if !isFried(p) || forced[p] {
			continue
		}
		fried++
		if fried <= 2 {
			continue
		}
		var nonFried []string
		for _, d := range allDishes {
			if !isFried(d) && !slices.Contains(proteins, d) {
				nonFried = append(nonFried, d)
			}
		}
		if len(nonFried) > 0 {
			proteins[i] = nonFried[rng.Intn(len(nonFried))]
		}
	}

	plan := mealPlan{
		WeekStart: weekStart.Format("2006-01-02"), WeekEnd: weekStart.AddDate(0, 0, 6).Format("2006-01-02"),
		Ingredients: ings, Promos: []mealPromo{}, PromoCount: len(promos), Missing: missing,
	}
	if plan.Ingredients == nil {
		plan.Ingredients = []string{}
	}
	if plan.Missing == nil {
		plan.Missing = []string{}
	}
	matchable := append(append([]mealPromo(nil), proteinPromos...), veggiePromos...)
	seen := map[string]bool{}
	for i := 0; i < 7; i++ {
		d := mealDay{Day: mealDayNames[i], Date: weekStart.AddDate(0, 0, i).Format("2006-01-02"),
			Protein: proteins[i], Veggie: veggies[i], Fried: isFried(proteins[i])}
		for _, p := range matchable {
			if promoUsesDish([]mealPromo{p}, proteins[i]) || promoUsesDish([]mealPromo{p}, veggies[i]) {
				pc := p
				d.Promo = &pc
				if !seen[p.Name] {
					seen[p.Name] = true
					plan.Promos = append(plan.Promos, p)
				}
				break
			}
		}
		plan.Days = append(plan.Days, d)
	}
	return plan
}

// promoUsesDish: does any promo name contain a key of the dish or side (by
// catalog name)?
func promoUsesDish(promos []mealPromo, dish string) bool {
	keys := mealKeysByName[dish]
	for _, p := range promos {
		if textHasAnyKey(p.Name, keys) {
			return true
		}
	}
	return false
}

// mealPlanText renders the plan for chat (plain text).
func mealPlanText(p mealPlan) string {
	ws, _ := time.Parse("2006-01-02", p.WeekStart)
	we, _ := time.Parse("2006-01-02", p.WeekEnd)
	var b strings.Builder
	b.WriteString("Weekly meal plan\n")
	fmt.Fprintf(&b, "%s - %s\n", ws.Format("2 Jan"), we.Format("2 Jan 2006"))
	if len(p.Ingredients) > 0 {
		fmt.Fprintf(&b, "Must-use ingredients: %s\n", strings.Join(p.Ingredients, ", "))
	}
	if len(p.Missing) > 0 {
		fmt.Fprintf(&b, "No dish uses: %s\n", strings.Join(p.Missing, ", "))
	}
	if p.PromoCount > 0 {
		b.WriteString("Based on this week's grocery promos\n")
	}
	for _, d := range p.Days {
		dt, _ := time.Parse("2006-01-02", d.Date)
		fried := ""
		if d.Fried {
			fried = " (fried)"
		}
		fmt.Fprintf(&b, "\n%s %s\n  Main: %s%s\n  Side: %s\n", d.Day, dt.Format("02/01"), d.Protein, fried, d.Veggie)
		if d.Promo != nil {
			fmt.Fprintf(&b, "  Promo: %s %s (%s)\n", d.Promo.Name, d.Promo.Price, d.Promo.Store)
		}
	}
	if len(p.Promos) > 0 {
		b.WriteString("\nPromos used this week:\n")
		for _, pr := range p.Promos {
			fmt.Fprintf(&b, "- %s %s (%s) %s\n", pr.Name, pr.Price, pr.Store, pr.Link)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// data access

func (app *App) loadIngredients(ctx context.Context, aid int64) ([]map[string]any, []string, error) {
	rows, err := app.DB.Query(ctx, "SELECT id, name FROM mandatory_ingredients WHERE account_id=$1 ORDER BY name", aid)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items, names := []map[string]any{}, []string{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, nil, err
		}
		items = append(items, map[string]any{"id": id, "name": name})
		names = append(names, name)
	}
	return items, names, rows.Err()
}

func (app *App) loadPromos(ctx context.Context) ([]mealPromo, *time.Time, error) {
	rows, err := app.DB.Query(ctx, "SELECT store, name, price, link, scraped_at FROM grocery_promos ORDER BY id")
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var out []mealPromo
	var at *time.Time
	for rows.Next() {
		var p mealPromo
		var t time.Time
		if err := rows.Scan(&p.Store, &p.Name, &p.Price, &p.Link, &t); err != nil {
			return nil, nil, err
		}
		at = &t
		out = append(out, p)
	}
	return out, at, rows.Err()
}

func (app *App) buildMealPlan(ctx context.Context, aid int64, this bool, now time.Time) (mealPlan, error) {
	_, names, err := app.loadIngredients(ctx, aid)
	if err != nil {
		return mealPlan{}, err
	}
	promos, _, err := app.loadPromos(ctx)
	if err != nil {
		return mealPlan{}, err
	}
	start := mealWeekStart(householdToday(ctx, app, aid, now), this)
	return generateMealPlan(start, names, promos), nil
}

// normalizeIngredient: lower-case, single spaces, max 60 chars.
func normalizeIngredient(s string) string {
	s = strings.Join(strings.Fields(strings.ToLower(s)), " ")
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

func (app *App) addIngredient(ctx context.Context, aid int64, raw string) (string, bool, error) {
	name := normalizeIngredient(raw)
	if name == "" {
		return "", false, errors.New("ingredient name is required")
	}
	tag, err := app.DB.Exec(ctx, "INSERT INTO mandatory_ingredients (account_id, name) VALUES ($1,$2) ON CONFLICT DO NOTHING", aid, name)
	if err != nil {
		return name, false, err
	}
	return name, tag.RowsAffected() > 0, nil
}

// schedule

type mealSchedule struct {
	Enabled   bool    `json:"enabled"`
	Days      []int   `json:"days"` // 0=Sunday .. 6=Saturday
	Time      string  `json:"time"` // HH:MM household-local
	StartDate *string `json:"start_date"`
	LastRun   *string `json:"last_run"`
	NextRun   *string `json:"next_run"` // household-local "YYYY-MM-DD HH:MM"
}

func parseDaysCSV(s string) []int {
	out := []int{}
	for _, p := range strings.Split(s, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && n >= 0 && n <= 6 && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

func daysCSV(days []int) string {
	parts := make([]string, len(days))
	for i, d := range days {
		parts[i] = strconv.Itoa(d)
	}
	return strings.Join(parts, ",")
}

func (app *App) loadMealSchedule(ctx context.Context, aid int64, now time.Time) (mealSchedule, error) {
	s := mealSchedule{Days: []int{5}, Time: "06:00"}
	var days string
	var hour, minute int
	var start *time.Time
	err := app.DB.QueryRow(ctx, "SELECT enabled, days, hour, minute, start_date, last_run FROM reminder_schedules WHERE account_id=$1 AND job_name=$2",
		aid, mealJob).Scan(&s.Enabled, &days, &hour, &minute, &start, &s.LastRun)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return s, err
	}
	if err == nil {
		s.Days = parseDaysCSV(days)
		s.Time = fmt.Sprintf("%02d:%02d", hour, minute)
		if start != nil {
			v := start.Format("2006-01-02")
			s.StartDate = &v
		}
	}
	if s.Enabled {
		if next, ok := nextMealRun(s, now.In(householdLocation(ctx, app, aid))); ok {
			v := next.Format("2006-01-02 15:04")
			s.NextRun = &v
		}
	}
	return s, nil
}

// nextMealRun: the first local slot at/after localNow (pure, unit tested).
func nextMealRun(s mealSchedule, localNow time.Time) (time.Time, bool) {
	hh, mm, err := parseHHMM(s.Time)
	if err != nil || len(s.Days) == 0 {
		return time.Time{}, false
	}
	for i := 0; i < 400; i++ {
		d := localNow.AddDate(0, 0, i)
		at := time.Date(d.Year(), d.Month(), d.Day(), hh, mm, 0, 0, localNow.Location())
		if at.Before(localNow) || !slices.Contains(s.Days, int(at.Weekday())) {
			continue
		}
		if s.StartDate != nil && at.Format("2006-01-02") < *s.StartDate {
			continue
		}
		if s.LastRun != nil && at.Format("2006-01-02") == *s.LastRun {
			continue
		}
		return at, true
	}
	return time.Time{}, false
}

type mealScheduleInput struct {
	Enabled   *bool   `json:"enabled"`
	Days      []int   `json:"days"`
	Time      *string `json:"time"`
	StartDate *string `json:"start_date"` // "" clears
}

func (app *App) saveMealSchedule(ctx context.Context, aid int64, in mealScheduleInput, now time.Time) (mealSchedule, error) {
	cur, err := app.loadMealSchedule(ctx, aid, now)
	if err != nil {
		return cur, err
	}
	if in.Enabled != nil {
		cur.Enabled = *in.Enabled
	}
	if in.Days != nil {
		days := []int{}
		for _, d := range in.Days {
			if d < 0 || d > 6 {
				return cur, errors.New("days must be 0 (Sunday) to 6 (Saturday)")
			}
			if !slices.Contains(days, d) {
				days = append(days, d)
			}
		}
		if len(days) == 0 {
			return cur, errors.New("pick at least one day")
		}
		sort.Ints(days)
		cur.Days = days
	}
	if in.Time != nil {
		if _, _, err := parseHHMM(*in.Time); err != nil {
			return cur, err
		}
		cur.Time = *in.Time
	}
	if in.StartDate != nil {
		if *in.StartDate == "" {
			cur.StartDate = nil
		} else {
			if _, err := time.Parse("2006-01-02", *in.StartDate); err != nil {
				return cur, errors.New("start_date must be YYYY-MM-DD")
			}
			v := *in.StartDate
			cur.StartDate = &v
		}
	}
	hh, mm, _ := parseHHMM(cur.Time)
	_, err = app.DB.Exec(ctx, `INSERT INTO reminder_schedules (account_id, job_name, days, hour, minute, enabled, start_date)
	  VALUES ($1,$2,$3,$4,$5,$6,$7)
	  ON CONFLICT (account_id, job_name) DO UPDATE SET days=EXCLUDED.days, hour=EXCLUDED.hour, minute=EXCLUDED.minute,
	    enabled=EXCLUDED.enabled, start_date=EXCLUDED.start_date`,
		aid, mealJob, daysCSV(cur.Days), hh, mm, cur.Enabled, cur.StartDate)
	if err != nil {
		return cur, err
	}
	return app.loadMealSchedule(ctx, aid, now)
}

var shortDayNames = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

func mealScheduleText(s mealSchedule, tz string) string {
	names := make([]string, len(s.Days))
	for i, d := range s.Days {
		names[i] = shortDayNames[d]
	}
	state := "off"
	if s.Enabled {
		state = "on"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Weekly meal plan post: %s\nDays: %s at %s (%s)", state, strings.Join(names, ", "), s.Time, tz)
	if s.StartDate != nil {
		fmt.Fprintf(&b, "\nStarts: %s", *s.StartDate)
	}
	last := "never"
	if s.LastRun != nil {
		last = *s.LastRun
	}
	fmt.Fprintf(&b, "\nLast post: %s", last)
	if s.NextRun != nil {
		fmt.Fprintf(&b, "\nNext post: %s", *s.NextRun)
	}
	return b.String()
}

// worker

const mealLateWindow = 30 * time.Minute

func (app *App) startMealPlanWorker(ctx context.Context) {
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			func() {
				defer func() {
					if p := recover(); p != nil {
						log.Printf("meal plan worker: panic: %v", p)
					}
				}()
				pctx, cancel := context.WithTimeout(ctx, 55*time.Second)
				defer cancel()
				if _, err := app.sendMealPlans(pctx, time.Now().UTC()); err != nil {
					log.Printf("meal plan worker: %v", err)
				}
			}()
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

// sendMealPlans posts the weekly plan to every household whose slot is due.
func (app *App) sendMealPlans(ctx context.Context, now time.Time) (int, error) {
	rows, err := app.DB.Query(ctx, `SELECT rs.account_id, u.timezone, rs.days, rs.hour, rs.minute, rs.start_date, rs.last_run
	  FROM reminder_schedules rs JOIN users u ON u.id=rs.account_id
	  WHERE rs.job_name=$1 AND rs.enabled AND NOT u.is_disabled AND EXISTS (
	    SELECT 1 FROM integrations i WHERE i.account_id=rs.account_id AND i.enabled
	      AND COALESCE(i.chat_id,'') <> '' AND COALESCE(i.platform_token,'') <> '')`, mealJob)
	if err != nil {
		return 0, err
	}
	type due struct {
		aid     int64
		tz      string
		lastRun *string
		date    string
	}
	var list []due
	for rows.Next() {
		var aid int64
		var tz, days string
		var hour, minute int
		var start *time.Time
		var lastRun *string
		if err := rows.Scan(&aid, &tz, &days, &hour, &minute, &start, &lastRun); err != nil {
			rows.Close()
			return 0, err
		}
		local := now.In(loadLoc(tz))
		at := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, local.Location())
		date := local.Format("2006-01-02")
		if !slices.Contains(parseDaysCSV(days), int(local.Weekday())) || local.Before(at) || local.Sub(at) >= mealLateWindow {
			continue
		}
		if start != nil && date < start.Format("2006-01-02") {
			continue
		}
		if lastRun != nil && *lastRun == date {
			continue
		}
		list = append(list, due{aid, tz, lastRun, date})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(list) == 0 {
		return 0, nil
	}
	app.refreshPromosIfStale(ctx, now)
	sent := 0
	for _, d := range list {
		tag, err := app.DB.Exec(ctx, `UPDATE reminder_schedules SET last_run=$1
		  WHERE account_id=$2 AND job_name=$3 AND last_run IS NOT DISTINCT FROM $4`, d.date, d.aid, mealJob, d.lastRun)
		if err != nil {
			return sent, err
		}
		if tag.RowsAffected() == 0 {
			continue // another instance claimed it
		}
		plan, err := app.buildMealPlan(ctx, d.aid, false, now)
		if err != nil {
			return sent, err
		}
		chs, err := app.loadChatChannels(ctx, d.aid, "all")
		if err != nil {
			return sent, err
		}
		text := mealPlanText(plan)
		if app.deliver(ctx, chs, func(string) chatMessage { return chatMessage{Text: text} }, fmt.Sprintf("meal plan account %d", d.aid)) == 0 {
			log.Printf("meal plan: account %d: no channel delivered, will retry", d.aid)
			_, _ = app.DB.Exec(ctx, "UPDATE reminder_schedules SET last_run=$1 WHERE account_id=$2 AND job_name=$3", d.lastRun, d.aid, mealJob)
			continue
		}
		sent++
	}
	return sent, nil
}

// routes

// RegisterMealPlanRoutes wires the meal plan, must-use ingredients, the
// chat post schedule, bot keyword management and the extra bot endpoints
// merged from rumahbot (bot_extras.go).
func RegisterMealPlanRoutes(r chi.Router, app *App) {
	r.Get("/meal-plan", requireLogin(app, handleGetMealPlan))
	r.Get("/meal-plan/ingredients", requireLogin(app, handleListIngredients))
	r.Post("/meal-plan/ingredients", requireLogin(app, handleAddIngredient))
	r.Delete("/meal-plan/ingredients/{id}", requireLogin(app, handleDeleteIngredient))
	r.Get("/meal-plan/schedule", requireLogin(app, handleGetMealSchedule))
	r.Put("/meal-plan/schedule", requireLogin(app, handlePutMealSchedule))

	r.Get("/bot/meal-plan", botAuth(app, botMealPlan))
	r.Get("/bot/ingredients", botAuth(app, botListIngredients))
	r.Post("/bot/ingredients", botAuth(app, botAddIngredient))
	r.Post("/bot/ingredients/remove", botAuth(app, botRemoveIngredient))
	r.Get("/bot/meal-schedule", botAuth(app, botGetMealSchedule))
	r.Post("/bot/meal-schedule", botAuth(app, botSetMealSchedule))

	registerBotExtraRoutes(r, app)
}

func handleGetMealPlan(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	plan, err := app.buildMealPlan(r.Context(), aid, r.URL.Query().Get("week") == "this", time.Now().UTC())
	if err != nil {
		dbError(w, err)
		return
	}
	if len(plan.Days) > 0 {
		from, _ := parseDay(plan.Days[0].Date)
		to, _ := parseDay(plan.Days[len(plan.Days)-1].Date)
		if picks, err := app.loadMealPicks(r.Context(), aid, from, to); err == nil {
			for i := range plan.Days {
				plan.Days[i].Picks = picks[plan.Days[i].Date]
			}
		}
	}
	_, at, _ := app.loadPromos(r.Context())
	var scraped *string
	if at != nil {
		v := at.UTC().Format(time.RFC3339)
		scraped = &v
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan, "text": mealPlanText(plan), "promos_scraped_at": scraped,
		"promo_sources": app.promoStores()})
}

func handleListIngredients(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	items, _, err := app.loadIngredients(r.Context(), aid)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleAddIngredient(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	var d struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	name, added, err := app.addIngredient(r.Context(), aid, d.Name)
	if err != nil {
		if name == "" {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		dbError(w, err)
		return
	}
	if !added {
		jsonError(w, http.StatusConflict, name+" is already on the list")
		return
	}
	items, _, _ := app.loadIngredients(r.Context(), aid)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "items": items})
}

func handleDeleteIngredient(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	id, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	tag, err := app.DB.Exec(r.Context(), "DELETE FROM mandatory_ingredients WHERE id=$1 AND account_id=$2", id, aid)
	if err != nil {
		dbError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func handleGetMealSchedule(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	s, err := app.loadMealSchedule(r.Context(), aid, time.Now().UTC())
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedule": s, "timezone": householdLocation(r.Context(), app, aid).String()})
}

func handlePutMealSchedule(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	var in mealScheduleInput
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	s, err := app.saveMealSchedule(r.Context(), aid, in, time.Now().UTC())
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "schedule": s, "timezone": householdLocation(r.Context(), app, aid).String()})
}

// bot

func botMealPlan(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	plan, err := app.buildMealPlan(r.Context(), aid, r.URL.Query().Get("week") == "this", time.Now().UTC())
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": mealPlanText(plan), "plan": plan})
}

func ingredientsText(names []string) string {
	if len(names) == 0 {
		return "No must-use ingredients yet.\nAdd one: /addingredient chicken"
	}
	var b strings.Builder
	b.WriteString("Must-use ingredients for the meal plan:\n")
	for _, n := range names {
		b.WriteString("- " + n + "\n")
	}
	b.WriteString("\nAdd: /addingredient <name>  Remove: /removeingredient <name>")
	return b.String()
}

func botListIngredients(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	items, names, err := app.loadIngredients(r.Context(), aid)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "text": ingredientsText(names)})
}

func botAddIngredient(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	var d struct {
		Name string `json:"name"`
	}
	_ = decodeJSON(r, &d)
	name, added, err := app.addIngredient(r.Context(), aid, d.Name)
	switch {
	case name == "":
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "Which ingredient? e.g. /addingredient chicken"})
	case err != nil:
		dbError(w, err)
	case !added:
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": name + " is already on the must-use list."})
	default:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Added " + name + " to the must-use ingredients. /menu shows the updated plan."})
	}
}

func botRemoveIngredient(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	var d struct {
		Name string `json:"name"`
	}
	_ = decodeJSON(r, &d)
	name := normalizeIngredient(d.Name)
	if name == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "Which ingredient? e.g. /removeingredient chicken"})
		return
	}
	tag, err := app.DB.Exec(r.Context(), "DELETE FROM mandatory_ingredients WHERE account_id=$1 AND name=$2", aid, name)
	if err != nil {
		dbError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": name + " is not on the list."})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Removed " + name + " from the must-use ingredients."})
}

func botGetMealSchedule(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	s, err := app.loadMealSchedule(r.Context(), aid, time.Now().UTC())
	if err != nil {
		dbError(w, err)
		return
	}
	tz := householdLocation(r.Context(), app, aid).String()
	writeJSON(w, http.StatusOK, map[string]any{"schedule": s, "timezone": tz, "text": mealScheduleText(s, tz)})
}

func botSetMealSchedule(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	ctx := r.Context()
	if m, linked := botActor(ctx, app, r, aid); linked && !canManageFamily(m.Role) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "Only parents can change the meal plan schedule."})
		return
	}
	var in mealScheduleInput
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "invalid request"})
		return
	}
	s, err := app.saveMealSchedule(ctx, aid, in, time.Now().UTC())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	tz := householdLocation(ctx, app, aid).String()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "schedule": s, "message": "Schedule saved.\n" + mealScheduleText(s, tz)})
}
