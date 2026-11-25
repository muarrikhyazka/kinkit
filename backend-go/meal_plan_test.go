package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

// pure

func TestMealWeekStart(t *testing.T) {
	d := func(s string) time.Time { v, _ := time.Parse("2006-01-02", s); return v }
	cases := []struct {
		today string
		this  bool
		want  string
	}{
		{"2026-11-20", false, "2026-11-23"}, // Friday -> next Monday
		{"2026-11-23", false, "2026-11-23"}, // Monday -> today
		{"2026-11-22", false, "2026-11-23"}, // Sunday -> tomorrow
		{"2026-11-20", true, "2026-11-16"},  // this week's Monday
		{"2026-11-22", true, "2026-11-16"},
	}
	for _, c := range cases {
		if got := mealWeekStart(d(c.today), c.this).Format("2006-01-02"); got != c.want {
			t.Errorf("mealWeekStart(%s,%v)=%s want %s", c.today, c.this, got, c.want)
		}
	}
}

func TestGenerateMealPlan(t *testing.T) {
	start, _ := time.Parse("2006-01-02", "2026-11-23")
	a := generateMealPlan(start, []string{"udang", "bayam", "durian"}, nil)
	b := generateMealPlan(start, []string{"bayam", "udang", "durian"}, nil)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("plan must be deterministic for the same week and ingredients")
	}
	if len(a.Days) != 7 || a.Days[0].Day != "Monday" || a.Days[6].Date != "2026-11-29" || a.WeekEnd != "2026-11-29" {
		t.Fatalf("days: %+v", a)
	}
	// Indonesian must-use words (stored by existing households) still match.
	var hasPrawn, hasSpinach bool
	fried := 0
	seen := map[string]bool{}
	for _, d := range a.Days {
		if mealGroupOf(d.Protein) == "Prawns & squid" {
			hasPrawn = true
		}
		if strings.Contains(strings.ToLower(d.Veggie), "spinach") {
			hasSpinach = true
		}
		if d.Fried {
			fried++
		}
		if seen[d.Protein] || seen[d.Veggie] {
			t.Fatalf("duplicate dish: %+v", a.Days)
		}
		seen[d.Protein], seen[d.Veggie] = true, true
	}
	if !hasPrawn || !hasSpinach {
		t.Fatalf("must-use ingredients not honoured: %+v", a.Days)
	}
	if fried > 2 {
		t.Fatalf("more than two fried dishes: %d", fried)
	}
	if len(a.Missing) != 1 || a.Missing[0] != "durian" {
		t.Fatalf("missing: %v", a.Missing)
	}
	// different week -> (almost certainly) different menu
	next := generateMealPlan(start.AddDate(0, 0, 7), nil, nil)
	jn, _ := json.Marshal(next.Days)
	jd, _ := json.Marshal(a.Days)
	if string(jn) == string(jd) {
		t.Fatal("next week should differ")
	}
	// promos: a protein promo steers dish choice and is reported
	p := generateMealPlan(start, nil, []mealPromo{{Store: "Superindo", Name: "Ikan Kembung Segar", Price: "Rp25.000", Link: "x"}})
	if len(p.Promos) == 0 || p.PromoCount != 1 {
		t.Fatalf("promo not matched: %+v", p)
	}
	txt := mealPlanText(p)
	if !strings.Contains(txt, "Weekly meal plan") || !strings.Contains(txt, "Ikan Kembung Segar") || !strings.Contains(txt, "Monday 23/11") {
		t.Fatalf("text: %s", txt)
	}
}

// mealGroupOf returns the protein group label of a catalog dish, or "".
func mealGroupOf(dish string) string {
	for _, g := range mealProteins {
		for _, d := range g.Dishes {
			if d.Name == dish {
				return g.Protein
			}
		}
	}
	return ""
}

func TestMealPlanEnglishInputs(t *testing.T) {
	start, _ := time.Parse("2006-01-02", "2026-11-23")
	p := generateMealPlan(start, []string{"Prawn", "spinach", "eggplant"}, nil)
	var prawn, spinach, ratatouille bool
	for _, d := range p.Days {
		if mealGroupOf(d.Protein) == "Prawns & squid" {
			prawn = true
		}
		if strings.Contains(strings.ToLower(d.Veggie), "spinach") {
			spinach = true
		}
		if d.Veggie == "Ratatouille" {
			ratatouille = true
		}
	}
	if !prawn || !spinach || !ratatouille || len(p.Missing) != 0 {
		t.Fatalf("english must-use not honoured: %+v missing=%v", p.Days, p.Missing)
	}
	// "eggplant" is a side ingredient, not an egg dish
	var eggs int
	for _, d := range p.Days {
		if mealGroupOf(d.Protein) == "Eggs" {
			eggs++
		}
	}
	if eggs > 2 {
		t.Fatalf("eggplant pulled in egg dishes: %+v", p.Days)
	}
	// plural and Indonesian synonyms still resolve
	for ing, want := range map[string]string{"shrimps": "Prawns & squid", "tahu": "Tofu & tempeh", "ikan kembung": "Fish", "telur": "Eggs", "ayam": "Chicken", "daging sapi": "Beef"} {
		q := generateMealPlan(start, []string{ing}, nil)
		got := false
		for _, d := range q.Days {
			if mealGroupOf(d.Protein) == want {
				got = true
			}
		}
		if !got || len(q.Missing) != 0 {
			t.Errorf("%q should force a %s dish: %+v missing=%v", ing, want, q.Days, q.Missing)
		}
	}
}

func TestMealCatalogShape(t *testing.T) {
	indonesian := []string{"ayam", "ikan", "tahu", "tempe", "telur", "udang", "cumi", "daging", "sapi", "tongkol", "lele", "kembung", "nila",
		"goreng", "bakar", "tumis", "sayur", "bayam", "kangkung", "sawi", "buncis", "tauge", "brokoli", "wortel", "kol", "kacang", "jagung",
		"kentang", "terong", "tomat", "daun", "sop", "asem", "lodeh", "urap", "pecel", "rendang", "semur", "balado", "pepes"}
	ascii := func(s string) bool {
		for _, r := range s {
			if r > 126 {
				return false
			}
		}
		return true
	}
	check := func(label, name string) {
		if name == "" || !ascii(name) {
			t.Errorf("%s %q must be non-empty ASCII", label, name)
		}
		words := wordsOf(name)
		for _, w := range indonesian {
			if slices.Contains(words, w) {
				t.Errorf("%s %q contains Indonesian word %q", label, name, w)
			}
		}
	}
	nonFried := 0
	names := map[string]bool{}
	for _, g := range mealProteins {
		check("group", g.Protein)
		if len(g.Keys) == 0 || len(g.Dishes) == 0 {
			t.Errorf("group %s needs keys and dishes", g.Protein)
		}
		for _, d := range g.Dishes {
			check("dish", d.Name)
			if names[d.Name] {
				t.Errorf("duplicate dish %q", d.Name)
			}
			names[d.Name] = true
			if !d.Fried {
				nonFried++
			}
			if d.Fried != isFried(d.Name) {
				t.Errorf("isFried(%q) disagrees with the catalog", d.Name)
			}
		}
	}
	if nonFried < 7 {
		t.Errorf("only %d non-fried protein dishes; a week needs at least 7", nonFried)
	}
	if len(mealVeggies) < 14 {
		t.Errorf("only %d sides", len(mealVeggies))
	}
	for _, s := range mealVeggies {
		check("side", s.Name)
		if len(s.Keys) == 0 {
			t.Errorf("side %q needs keys", s.Name)
		}
	}
	for _, g := range []string{"Chicken", "Fish", "Tofu & tempeh", "Eggs", "Prawns & squid", "Beef"} {
		found := false
		for _, mg := range mealProteins {
			found = found || mg.Protein == g
		}
		if !found {
			t.Errorf("missing group %q", g)
		}
	}
	if !isFried("Something fried in oil") || isFried("Chicken curry") {
		t.Error("isFried fallback")
	}
	for _, w := range []string{"ayam", "daging", "ikan", "telur", "udang", "tahu", "tempe", "lele", "tongkol", "cumi"} {
		if !slices.Contains(promoProteinWords, w) {
			t.Errorf("promoProteinWords lost %q", w)
		}
	}
	for _, w := range []string{"bayam", "kangkung", "sawi", "brokoli", "buncis", "tauge", "wortel", "kol", "kacang", "daun"} {
		if !slices.Contains(promoVeggieWords, w) {
			t.Errorf("promoVeggieWords lost %q", w)
		}
	}
}

func TestMealPromoMatchesEnglishDishes(t *testing.T) {
	start, _ := time.Parse("2006-01-02", "2026-11-23")
	for _, c := range []struct{ name, group string }{
		{"Ikan Kembung Segar", "Fish"},
		{"Udang Kupas 250g", "Prawns & squid"},
		{"Fresh Salmon Fillet", "Fish"},
		{"Ayam Fillet 500g", "Chicken"},
	} {
		p := generateMealPlan(start, nil, []mealPromo{{Store: "Superindo", Name: c.name, Price: "Rp25.000", Link: "x"}})
		if len(p.Promos) == 0 || p.PromoCount != 1 {
			t.Fatalf("%s: promo not matched: %+v", c.name, p)
		}
		var onDay bool
		for _, d := range p.Days {
			if d.Promo != nil && mealGroupOf(d.Protein) == c.group {
				onDay = true
			}
		}
		if !onDay {
			t.Errorf("%s should attach to a %s dish: %+v", c.name, c.group, p.Days)
		}
	}
	// a side promo attaches to a side that uses it
	p := generateMealPlan(start, []string{"bayam"}, []mealPromo{{Store: "Superindo", Name: "Bayam Hijau", Price: "Rp5.000", Link: "x"}})
	var side bool
	for _, d := range p.Days {
		if d.Promo != nil && strings.Contains(d.Veggie, "spinach") {
			side = true
		}
	}
	if !side {
		t.Errorf("Bayam Hijau should attach to a spinach side: %+v", p.Days)
	}
}

func TestNextMealRun(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, loc) // Wednesday
	s := mealSchedule{Enabled: true, Days: []int{5}, Time: "06:00"}
	got, ok := nextMealRun(s, now)
	if !ok || got.Format("2006-01-02 15:04") != "2026-11-20 06:00" {
		t.Fatalf("next: %v %v", got, ok)
	}
	start := "2026-11-25"
	s.StartDate = &start
	got, _ = nextMealRun(s, now)
	if got.Format("2006-01-02") != "2026-11-27" {
		t.Fatalf("start date: %v", got)
	}
	s = mealSchedule{Days: []int{3}, Time: "09:00"}
	last := "2026-11-18"
	s.LastRun = &last
	got, _ = nextMealRun(s, now)
	if got.Format("2006-01-02") != "2026-11-25" {
		t.Fatalf("already ran today: %v", got)
	}
}

func TestKeywordCandidates(t *testing.T) {
	got := strings.Join(keywordCandidates("Bakso Pak Kumis + es teh, bakso"), ",")
	if got != "bakso,pak,kumis,teh" {
		t.Fatalf("got %s", got)
	}
}

func TestParseAltPromos(t *testing.T) {
	page := `<img alt="logo"><img alt="Ayam Fillet 500g"><img alt="Sabun"><img alt="x"><span>Rp 10.000</span><span>Rp25.900</span><span>Rp 3.000</span>`
	got := parseAltPromos(page, "Superindo", "l", map[string]bool{"logo": true}, true)
	if len(got) != 2 || got[0].Name != "Ayam Fillet 500g" || got[0].Price != "Rp25.900" || got[1].Name != "Sabun" {
		t.Fatalf("got %+v", got)
	}
}

// integration

func botCaller(t *testing.T, srvURL, token string) func(method, path, user string, body any, out any) int {
	return func(method, path, user string, body any, out any) int {
		t.Helper()
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = strings.NewReader(string(b))
		}
		req, _ := http.NewRequest(method, srvURL+path, rd)
		req.Header.Set("X-Bot-Token", token)
		req.Header.Set("Content-Type", "application/json")
		if user != "" {
			req.Header.Set("X-Bot-User", user)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if out != nil {
			_ = json.NewDecoder(resp.Body).Decode(out)
		}
		return resp.StatusCode
	}
}

func setupBotAccount(t *testing.T) (*App, *testClient, int64, func(method, path, user string, body any, out any) int, *httptest.Server) {
	app, srv := newTestServer(t)
	c := newClient(t, srv)
	_, aid := c.signup()
	ctx := context.Background()
	if code := c.do("POST", "/api/integrations", map[string]any{"platform": "telegram", "platform_token": "123:abc", "chat_id": "-100777"}, nil); code != 200 {
		t.Fatalf("integration: %d", code)
	}
	var token string
	_ = app.DB.QueryRow(ctx, "SELECT bot_token FROM integrations WHERE account_id=$1", aid).Scan(&token)
	_, _ = app.DB.Exec(ctx, "UPDATE integrations SET chat_id='-100777' WHERE account_id=$1", aid)
	_, _ = app.DB.Exec(ctx, "UPDATE users SET timezone='Asia/Jakarta' WHERE id=$1", aid)
	for _, n := range []string{"Food", "Jajan", "Home Rent"} {
		_, _ = app.DB.Exec(ctx, "INSERT INTO categories (account_id, name, main_category) VALUES ($1,$2,'Needs') ON CONFLICT DO NOTHING", aid, n)
	}
	return app, c, aid, botCaller(t, srv.URL, token), srv
}

func TestBotSpendingPickerFlow(t *testing.T) {
	app, _, aid, bot, _ := setupBotAccount(t)
	ctx := context.Background()

	var cfg struct {
		CategoryList []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"category_list"`
		Today string `json:"today"`
	}
	if code := bot("GET", "/api/bot/config", "", nil, &cfg); code != 200 || len(cfg.CategoryList) == 0 || cfg.Today == "" {
		t.Fatalf("config: %d %+v", code, cfg)
	}
	var jajan int64
	for _, c := range cfg.CategoryList {
		if c.Name == "Jajan" {
			jajan = c.ID
		}
	}
	// linked member attribution + category by id
	var ownerID int64
	_ = app.DB.QueryRow(ctx, "SELECT id FROM members WHERE account_id=$1 AND role='owner'", aid).Scan(&ownerID)
	_, _ = app.DB.Exec(ctx, "UPDATE members SET telegram_user_id='4242' WHERE id=$1", ownerID)
	var add struct {
		OK       bool   `json:"ok"`
		ID       int64  `json:"id"`
		Category string `json:"category"`
	}
	if code := bot("POST", "/api/bot/spending", "telegram:4242", map[string]any{"tx_date": cfg.Today, "description": "bakso pak kumis", "category_id": jajan, "amount": 15000}, &add); code != 200 || !add.OK || add.Category != "Jajan" {
		t.Fatalf("add by id: %d %+v", code, add)
	}
	var member *int64
	var catName string
	_ = app.DB.QueryRow(ctx, "SELECT member_id, category_name FROM spending WHERE id=$1", add.ID).Scan(&member, &catName)
	if member == nil || *member != ownerID || catName != "Jajan" {
		t.Fatalf("attribution: %v %s", member, catName)
	}
	if code := bot("POST", "/api/bot/spending", "", map[string]any{"tx_date": cfg.Today, "description": "x", "category_id": 999999999, "amount": 1}, nil); code != 400 {
		t.Fatalf("foreign category id: %d", code)
	}
	if code := bot("POST", "/api/bot/spending", "", map[string]any{"tx_date": "nope", "description": "x", "amount": 1}, nil); code != 400 {
		t.Fatalf("bad date: %d", code)
	}

	// learn keywords from the picked category
	var learn struct {
		OK       bool     `json:"ok"`
		Learned  []string `json:"learned"`
		Category string   `json:"category"`
	}
	if code := bot("POST", "/api/bot/keywords/learn", "", map[string]any{"description": "bakso pak kumis", "category": "jajan"}, &learn); code != 200 ||
		strings.Join(learn.Learned, ",") != "bakso,pak,kumis" || learn.Category != "Jajan" {
		t.Fatalf("learn: %d %+v", code, learn)
	}
	bot("POST", "/api/bot/keywords/learn", "", map[string]any{"description": "bakso urat", "category": "Food"}, &learn)
	if strings.Join(learn.Learned, ",") != "urat" {
		t.Fatalf("existing keywords must not be re-mapped: %+v", learn)
	}
	if code := bot("POST", "/api/bot/keywords/learn", "", map[string]any{"description": "x", "category": "Nope"}, nil); code != 400 {
		t.Fatalf("unknown category: %d", code)
	}
	var cfg2 struct {
		Keywords map[string]string `json:"category_keywords"`
	}
	bot("GET", "/api/bot/config", "", nil, &cfg2)
	if cfg2.Keywords["bakso"] != "Jajan" || cfg2.Keywords["urat"] != "Food" {
		t.Fatalf("keywords in config: %v", cfg2.Keywords)
	}

	// /koreksi by id, /koreksitgl, /last N
	var upd struct {
		OK       bool   `json:"ok"`
		Category string `json:"category"`
	}
	var food int64
	for _, c := range cfg.CategoryList {
		if c.Name == "Food" {
			food = c.ID
		}
	}
	if code := bot("PATCH", fmt.Sprintf("/api/bot/spending/%d/category", add.ID), "", map[string]any{"category_id": food}, &upd); code != 200 || upd.Category != "Food" {
		t.Fatalf("fix category: %d %+v", code, upd)
	}
	if code := bot("PATCH", "/api/bot/spending/999999999/category", "", map[string]any{"category": "Food"}, nil); code != 404 {
		t.Fatalf("fix missing row: %d", code)
	}
	if code := bot("PATCH", fmt.Sprintf("/api/bot/spending/%d/date", add.ID), "", map[string]any{"tx_date": "2026-01-02"}, nil); code != 200 {
		t.Fatalf("fix date: %d", code)
	}
	var recent []map[string]any
	bot("GET", "/api/bot/spending/recent?n=500", "", nil, &recent)
	if len(recent) != 1 || recent[0]["tx_date"] != "2026-01-02" || recent[0]["category_name"] != "Food" {
		t.Fatalf("recent: %v", recent)
	}

	// feedback
	var fb struct {
		OK bool `json:"ok"`
	}
	bot("POST", "/api/bot/feedback", "telegram:4242", map[string]any{"text": "weekly report please"}, &fb)
	var n int
	_ = app.DB.QueryRow(ctx, "SELECT count(*) FROM bot_feedback WHERE account_id=$1 AND member_id=$2", aid, ownerID).Scan(&n)
	if !fb.OK || n != 1 {
		t.Fatalf("feedback: %+v %d", fb, n)
	}
}

func TestBotKeywordsWebCRUD(t *testing.T) {
	_, c, _, _, srv := setupBotAccount(t)
	var res struct {
		ID       int64  `json:"id"`
		Category string `json:"category"`
	}
	if code := c.do("POST", "/api/bot-keywords", map[string]any{"keyword": " Bensin ", "category": "food"}, &res); code != 200 || res.Category != "Food" {
		t.Fatalf("add: %d %+v", code, res)
	}
	c.do("POST", "/api/bot-keywords", map[string]any{"keyword": "bensin", "category": "Jajan"}, &res)
	var list struct {
		Items []map[string]any `json:"items"`
	}
	c.do("GET", "/api/bot-keywords", nil, &list)
	if len(list.Items) != 1 || list.Items[0]["category"] != "Jajan" || list.Items[0]["keyword"] != "bensin" {
		t.Fatalf("list after upsert: %v", list.Items)
	}
	if code := c.do("POST", "/api/bot-keywords", map[string]any{"keyword": "x", "category": "Nope"}, nil); code != 400 {
		t.Fatalf("unknown category: %d", code)
	}
	kidEmail, _ := addMember(t, c, "kid", nil)
	kid := newClient(t, srv)
	loginAs(t, kid, kidEmail, "temp-pass-123")
	if code := kid.do("DELETE", fmt.Sprintf("/api/bot-keywords/%d", res.ID), nil, nil); code != 403 {
		t.Fatalf("kid delete: %d", code)
	}
	if code := c.do("DELETE", fmt.Sprintf("/api/bot-keywords/%d", res.ID), nil, nil); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	if code := c.do("DELETE", fmt.Sprintf("/api/bot-keywords/%d", res.ID), nil, nil); code != 404 {
		t.Fatalf("delete twice: %d", code)
	}
}

func TestMealPlanEndpointsAndWorker(t *testing.T) {
	app, c, aid, bot, _ := setupBotAccount(t)
	ctx := context.Background()

	// ingredients via bot
	var msg struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
		Text    string `json:"text"`
	}
	bot("POST", "/api/bot/ingredients", "", map[string]any{"name": "  Udang "}, &msg)
	if !msg.OK {
		t.Fatalf("add: %+v", msg)
	}
	bot("POST", "/api/bot/ingredients", "", map[string]any{"name": "udang"}, &msg)
	if msg.OK || !strings.Contains(msg.Message, "already") {
		t.Fatalf("dupe: %+v", msg)
	}
	bot("POST", "/api/bot/ingredients", "", map[string]any{"name": "bayam"}, nil)
	bot("GET", "/api/bot/ingredients", "", nil, &msg)
	if !strings.Contains(msg.Text, "- bayam") || !strings.Contains(msg.Text, "- udang") {
		t.Fatalf("list text: %s", msg.Text)
	}
	bot("POST", "/api/bot/ingredients/remove", "", map[string]any{"name": "bayam"}, &msg)
	if !msg.OK {
		t.Fatalf("remove: %+v", msg)
	}
	bot("POST", "/api/bot/ingredients/remove", "", map[string]any{"name": "bayam"}, &msg)
	if msg.OK {
		t.Fatalf("remove twice: %+v", msg)
	}

	// ingredients via web
	var ing struct {
		Items []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if code := c.do("POST", "/api/meal-plan/ingredients", map[string]any{"name": "Tempe"}, &ing); code != 200 || len(ing.Items) != 2 {
		t.Fatalf("web add: %d %+v", code, ing)
	}
	if code := c.do("POST", "/api/meal-plan/ingredients", map[string]any{"name": "tempe"}, nil); code != 409 {
		t.Fatalf("web dupe: %d", code)
	}
	if code := c.do("DELETE", fmt.Sprintf("/api/meal-plan/ingredients/%d", ing.Items[0].ID), nil, nil); code != 200 {
		t.Fatalf("web delete: %d", code)
	}

	// plan honours the remaining ingredient (udang)
	var plan struct {
		Plan mealPlan `json:"plan"`
		Text string   `json:"text"`
	}
	if code := c.do("GET", "/api/meal-plan", nil, &plan); code != 200 || len(plan.Plan.Days) != 7 {
		t.Fatalf("plan: %d %+v", code, plan)
	}
	var botPlan struct {
		Text string `json:"text"`
	}
	bot("GET", "/api/bot/meal-plan", "", nil, &botPlan)
	prawnDish := false
	for _, g := range mealProteins {
		if g.Protein != "Prawns & squid" {
			continue
		}
		for _, d := range g.Dishes {
			prawnDish = prawnDish || strings.Contains(botPlan.Text, d.Name)
		}
	}
	if !prawnDish || botPlan.Text != plan.Text {
		t.Fatalf("bot plan: %s", botPlan.Text)
	}

	// schedule: default off; kid can't change it via bot; parent can
	var sch struct {
		OK       bool         `json:"ok"`
		Message  string       `json:"message"`
		Schedule mealSchedule `json:"schedule"`
		Text     string       `json:"text"`
	}
	bot("GET", "/api/bot/meal-schedule", "", nil, &sch)
	if sch.Schedule.Enabled || sch.Schedule.Time != "06:00" || len(sch.Schedule.Days) != 1 || sch.Schedule.Days[0] != 5 {
		t.Fatalf("default schedule: %+v", sch)
	}
	_, kidID := addMember(t, c, "kid", nil)
	_, _ = app.DB.Exec(ctx, "UPDATE members SET telegram_user_id='555' WHERE id=$1", kidID)
	bot("POST", "/api/bot/meal-schedule", "telegram:555", map[string]any{"enabled": true}, &sch)
	if sch.OK {
		t.Fatalf("kid changed schedule: %+v", sch)
	}
	bot("POST", "/api/bot/meal-schedule", "", map[string]any{"enabled": true, "days": []int{3, 3}, "time": "09:30"}, &sch)
	if !sch.OK || !sch.Schedule.Enabled || sch.Schedule.Time != "09:30" || len(sch.Schedule.Days) != 1 || sch.Schedule.NextRun == nil {
		t.Fatalf("set schedule: %+v", sch)
	}
	if code := c.do("PUT", "/api/meal-plan/schedule", map[string]any{"days": []int{9}}, nil); code != 400 {
		t.Fatalf("bad day: %d", code)
	}
	if code := c.do("PUT", "/api/meal-plan/schedule", map[string]any{"days": []int{3}, "time": "09:30", "start_date": ""}, &sch); code != 200 {
		t.Fatalf("web schedule: %d", code)
	}

	// worker: Wednesday 09:35 Jakarta = 02:35 UTC -> due once
	sent, fail := stubChat(t)
	at := time.Date(2026, 11, 18, 2, 35, 0, 0, time.UTC)
	*fail = true
	if n, err := app.sendMealPlans(ctx, at); err != nil || n != 0 {
		t.Fatalf("failing send: %d %v", n, err)
	}
	var lastRun *string
	_ = app.DB.QueryRow(ctx, "SELECT last_run FROM reminder_schedules WHERE account_id=$1 AND job_name='meal_plan'", aid).Scan(&lastRun)
	if lastRun != nil {
		t.Fatalf("claim must be released after a failed send: %v", *lastRun)
	}
	*fail = false
	if n, err := app.sendMealPlans(ctx, at); err != nil || n != 1 {
		t.Fatalf("send: %d %v", n, err)
	}
	if n, _ := app.sendMealPlans(ctx, at.Add(time.Minute)); n != 0 {
		t.Fatal("sent twice the same day")
	}
	got := sentTo(*sent, "-100777")
	if len(got) != 1 || !strings.Contains(got[0].msg.Text, "Weekly meal plan") {
		t.Fatalf("sent: %+v", got)
	}
	if n, _ := app.sendMealPlans(ctx, at.Add(2*time.Hour)); n != 0 {
		t.Fatal("outside the late window")
	}
}

func TestPromoScrapeStored(t *testing.T) {
	app := requireDB(t)
	ctx := context.Background()
	old := promoFetch
	promoFetch = func(_ context.Context, url string) (string, error) {
		return `<img alt="Udang Kupas 250g"><img alt="Bayam Hijau"><p>Rp 30.000</p><p>Rp 5.000</p>`, nil
	}
	t.Cleanup(func() { promoFetch = old })
	t.Setenv("MEAL_PROMO_STORES", "superindo, nope")
	if got := app.promoStores(); len(got) != 1 || got[0] != "superindo" {
		t.Fatalf("stores: %v", got)
	}
	_, _ = app.DB.Exec(ctx, "DELETE FROM meta WHERE key='grocery_promos_attempt'")
	_, _ = app.DB.Exec(ctx, "UPDATE grocery_promos SET scraped_at = now() - interval '2 days'")
	app.refreshPromosIfStale(ctx, time.Now().UTC())
	promos, at, err := app.loadPromos(ctx)
	if err != nil || at == nil || len(promos) < 2 || promos[0].Name != "Udang Kupas 250g" || promos[0].Price != "Rp30.000" {
		t.Fatalf("promos: %+v %v %v", promos, at, err)
	}
	start, _ := time.Parse("2006-01-02", "2026-11-23")
	plan := generateMealPlan(start, nil, promos)
	if len(plan.Promos) == 0 {
		t.Fatalf("plan should use stored promos: %+v", plan)
	}
}
