package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Widget boards: a named set of widgets arranged by drag and drop, shown on a
// phone, a TV, a car screen or a monitor. A board's layout is a 12-column
// flow grid: {"background"?,"widgets":[{"id","type","w","h","text"?,"color"?}]} where w is the width
// in columns (1-12) and h the height in rows (1-4).
//
// The catalog (widget types + device presets) lives here so web, phone and TV
// clients don't hard-code it. Every widget except "clock" and "note" is
// rendered from a generic card (title, optional headline number, rows) built
// by buildWidgetCards, so clients need no per-widget data logic.
//
//	GET    /api/widgets/catalog
//	GET    /api/widgets/boards
//	POST   /api/widgets/boards {name, device?, layout?|preset?}   owner/parent
//	PATCH  /api/widgets/boards/{id} {name?, device?, layout?}      owner/parent
//	DELETE /api/widgets/boards/{id}                                owner/parent
//	GET    /api/widgets/data[?types=agenda,reminders]
//	PATCH  /api/tv-tokens/{id} {board_id}  (see tvshare.go)       owner/parent

const (
	maxWidgetBoards    = 12
	maxWidgetsPerBoard = 40
	maxBoardBytes      = 32 << 10
)

type widgetType struct {
	Type        string `json:"type"`
	Label       string `json:"label"`
	Description string `json:"description"`
	W           int    `json:"w"` // default size
	H           int    `json:"h"`
	Money       bool   `json:"money,omitempty"` // shows amounts (hidden from kids)
}

var widgetTypes = []widgetType{
	{"clock", "Clock & date", "Large clock with the date, in the family's time zone.", 4, 1, false},
	{"agenda", "Calendar agenda", "The next 7 days of family activities.", 6, 3, false},
	{"reminders", "Reminders today", "What is due today and what is overdue.", 4, 2, false},
	{"chores", "Chores today", "Chores to do today and who has them.", 4, 2, false},
	{"lists", "Shopping lists", "Open shopping lists and what is on them.", 4, 2, false},
	{"meals", "Today's meals", "The dishes planned for today, including approved requests.", 4, 2, false},
	{"meal_requests", "Meal requests", "Dishes family members asked for, waiting for a decision.", 4, 2, false},
	{"budget", "Budget this month", "Money left to spend this month and the busiest categories.", 4, 2, true},
	{"goals", "Savings goals", "Progress on every savings goal.", 4, 2, true},
	{"family", "Family points", "The household's points, streak and badge.", 4, 1, false},
	{"note", "Note", "A short message you write yourself, e.g. the Wi-Fi password or a house rule.", 4, 1, false},
}

type widgetPreset struct {
	ID      string             `json:"id"`
	Name    string             `json:"name"`
	Device  string             `json:"device"`
	Widgets []widgetLayoutItem `json:"widgets"`
}

type widgetLayoutItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	Text string `json:"text,omitempty"`
	// Color is the widget's own fill as #rrggbb ("" = theme default). Clients
	// pick a readable text colour from its brightness.
	Color string `json:"color,omitempty"`
}

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// cleanColor returns c lower-cased when it is #rrggbb, else "" (default).
func cleanColor(c string) string {
	c = strings.TrimSpace(c)
	if !hexColorRe.MatchString(c) {
		return ""
	}
	return strings.ToLower(c)
}

func presetWidgets(types ...string) []widgetLayoutItem {
	out := []widgetLayoutItem{}
	for i, t := range types {
		for _, wt := range widgetTypes {
			if wt.Type == t {
				out = append(out, widgetLayoutItem{ID: fmt.Sprintf("w%d", i+1), Type: t, W: wt.W, H: wt.H})
			}
		}
	}
	return out
}

var widgetPresets = []widgetPreset{
	{"phone", "Phone", "phone", presetWidgets("clock", "agenda", "reminders", "chores", "meals", "lists")},
	{"tv", "Living-room TV", "tv", presetWidgets("clock", "agenda", "reminders", "chores", "meals", "budget", "goals", "family")},
	{"car", "Car screen", "car", presetWidgets("clock", "agenda", "reminders", "lists")},
	{"monitor", "Desk monitor", "monitor", presetWidgets("clock", "agenda", "reminders", "chores", "lists", "budget", "goals", "meals", "family")},
}

var validWidgetDevices = []string{"any", "phone", "tv", "car", "monitor"}

func RegisterWidgetRoutes(r chi.Router, app *App) {
	r.Get("/widgets/catalog", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"widgets": widgetTypes, "presets": widgetPresets, "devices": validWidgetDevices})
	})
	r.Get("/widgets/boards", app.handleListWidgetBoards)
	r.Post("/widgets/boards", app.handleCreateWidgetBoard)
	r.Patch("/widgets/boards/{id}", app.handleUpdateWidgetBoard)
	r.Delete("/widgets/boards/{id}", app.handleDeleteWidgetBoard)
	r.Get("/widgets/data", app.handleWidgetData)
}

// boards

type widgetBoard struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	Device    string          `json:"device"`
	Layout    json.RawMessage `json:"layout"`
	UpdatedAt string          `json:"updated_at"`
}

func (app *App) loadBoards(ctx context.Context, aid int64, where string, args ...any) ([]widgetBoard, error) {
	all := append([]any{aid}, args...)
	rows, err := app.DB.Query(ctx, "SELECT id, name, device, layout, updated_at FROM widget_boards WHERE account_id=$1 "+where+" ORDER BY id", all...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []widgetBoard{}
	for rows.Next() {
		var b widgetBoard
		var t time.Time
		if err := rows.Scan(&b.ID, &b.Name, &b.Device, &b.Layout, &t); err != nil {
			return nil, err
		}
		b.UpdatedAt = t.UTC().Format(time.RFC3339)
		out = append(out, b)
	}
	return out, rows.Err()
}

func (app *App) handleListWidgetBoards(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	boards, err := app.loadBoards(r.Context(), m.AccountID, "")
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"boards": boards})
}

// normalizeLayout validates a layout and returns canonical JSON.
func normalizeLayout(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) > maxBoardBytes {
		return nil, errors.New("layout is too large")
	}
	var l struct {
		Widgets    []widgetLayoutItem `json:"widgets"`
		Background string             `json:"background"`
	}
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, errors.New("layout must be {\"widgets\": [...]}")
	}
	if len(l.Widgets) > maxWidgetsPerBoard {
		return nil, fmt.Errorf("a board can hold at most %d widgets", maxWidgetsPerBoard)
	}
	known := map[string]bool{}
	for _, t := range widgetTypes {
		known[t.Type] = true
	}
	seen := map[string]bool{}
	out := make([]widgetLayoutItem, 0, len(l.Widgets))
	for i, x := range l.Widgets {
		if !known[x.Type] {
			return nil, fmt.Errorf("unknown widget type %q", x.Type)
		}
		x.W, x.H = min(max(x.W, 1), 12), min(max(x.H, 1), 4)
		x.Text = clip(x.Text, 400)
		x.Color = cleanColor(x.Color)
		if x.ID == "" || seen[x.ID] || len(x.ID) > 40 {
			x.ID = fmt.Sprintf("w%d-%d", i+1, time.Now().UnixNano()%100000)
		}
		seen[x.ID] = true
		out = append(out, x)
	}
	res := map[string]any{"widgets": out}
	if bg := cleanColor(l.Background); bg != "" {
		res["background"] = bg // whole-board backdrop
	}
	return json.Marshal(res)
}

func (app *App) handleCreateWidgetBoard(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	var in struct {
		Name   string          `json:"name"`
		Device string          `json:"device"`
		Layout json.RawMessage `json:"layout"`
		Preset string          `json:"preset"`
	}
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	layout := in.Layout
	if in.Preset != "" {
		idx := slices.IndexFunc(widgetPresets, func(p widgetPreset) bool { return p.ID == in.Preset })
		if idx < 0 {
			jsonError(w, http.StatusBadRequest, "unknown preset")
			return
		}
		layout, _ = json.Marshal(map[string]any{"widgets": widgetPresets[idx].Widgets})
		if in.Device == "" {
			in.Device = widgetPresets[idx].Device
		}
		if strings.TrimSpace(in.Name) == "" {
			in.Name = widgetPresets[idx].Name
		}
	}
	if len(layout) == 0 {
		layout = json.RawMessage(`{"widgets":[]}`)
	}
	layout, err := normalizeLayout(layout)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	name := clip(in.Name, 60)
	if name == "" {
		name = "My board"
	}
	if in.Device == "" {
		in.Device = "any"
	}
	if !slices.Contains(validWidgetDevices, in.Device) {
		jsonError(w, http.StatusBadRequest, "unknown device")
		return
	}
	ctx := r.Context()
	var n int
	if err := app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM widget_boards WHERE account_id=$1", m.AccountID).Scan(&n); err != nil {
		dbError(w, err)
		return
	}
	if n >= maxWidgetBoards {
		jsonError(w, http.StatusBadRequest, "too many boards. Delete one first")
		return
	}
	var creator *int64
	if m.ID != 0 {
		creator = &m.ID
	}
	var id int64
	if err := app.DB.QueryRow(ctx, "INSERT INTO widget_boards (account_id, name, device, layout, created_by) VALUES ($1,$2,$3,$4,$5) RETURNING id",
		m.AccountID, name, in.Device, string(layout), creator).Scan(&id); err != nil {
		dbError(w, err)
		return
	}
	boards, err := app.loadBoards(ctx, m.AccountID, "AND id=$2", id)
	if err != nil || len(boards) == 0 {
		dbError(w, fmt.Errorf("load board: %v", err))
		return
	}
	writeJSON(w, http.StatusCreated, boards[0])
}

func (app *App) handleUpdateWidgetBoard(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBoardBytes+4096)
	var in struct {
		Name   *string         `json:"name"`
		Device *string         `json:"device"`
		Layout json.RawMessage `json:"layout"`
	}
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json or layout too large")
		return
	}
	ctx := r.Context()
	cur, err := app.loadBoards(ctx, m.AccountID, "AND id=$2", id)
	if err != nil {
		dbError(w, err)
		return
	}
	if len(cur) == 0 {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	b := cur[0]
	if in.Name != nil {
		if b.Name = clip(*in.Name, 60); b.Name == "" {
			jsonError(w, http.StatusBadRequest, "name is required")
			return
		}
	}
	if in.Device != nil {
		if !slices.Contains(validWidgetDevices, *in.Device) {
			jsonError(w, http.StatusBadRequest, "unknown device")
			return
		}
		b.Device = *in.Device
	}
	if len(in.Layout) > 0 {
		if b.Layout, err = normalizeLayout(in.Layout); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if _, err := app.DB.Exec(ctx, "UPDATE widget_boards SET name=$3, device=$4, layout=$5, updated_at=now() WHERE id=$1 AND account_id=$2",
		id, m.AccountID, b.Name, b.Device, string(b.Layout)); err != nil {
		dbError(w, err)
		return
	}
	b.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	writeJSON(w, http.StatusOK, b)
}

func (app *App) handleDeleteWidgetBoard(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	tag, err := app.DB.Exec(r.Context(), "DELETE FROM widget_boards WHERE id=$1 AND account_id=$2", id, m.AccountID)
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

// widget data (generic cards)

type widgetRow struct {
	Title  string   `json:"title"`
	Detail string   `json:"detail,omitempty"`
	Done   bool     `json:"done,omitempty"`
	Alert  bool     `json:"alert,omitempty"`  // overdue / over budget
	Amount *int64   `json:"amount,omitempty"` // money in IDR; clients format it with the display currency
	Pct    *float64 `json:"pct,omitempty"`    // 0-100 progress bar
}

type widgetCard struct {
	Type      string      `json:"type"`
	Title     string      `json:"title"`
	Big       string      `json:"big,omitempty"`        // headline text
	BigAmount *int64      `json:"big_amount,omitempty"` // headline money (IDR); wins over Big
	Caption   string      `json:"caption,omitempty"`
	Rows      []widgetRow `json:"rows"`
	Empty     string      `json:"empty,omitempty"` // shown when there are no rows
}

func sectionCard(typ, title string, secs []TodaySection, key, empty string) widgetCard {
	c := widgetCard{Type: typ, Title: title, Rows: []widgetRow{}, Empty: empty}
	for _, s := range secs {
		if s.Key != key {
			continue
		}
		c.Caption = s.Summary
		for _, it := range s.Items {
			row := widgetRow{Title: it.Title, Detail: it.Detail, Done: it.Done, Alert: it.Overdue}
			if it.MemberName != "" && !strings.Contains(row.Detail, it.MemberName) {
				if row.Detail != "" {
					row.Detail += " · "
				}
				row.Detail += it.MemberName
			}
			c.Rows = append(c.Rows, row)
		}
	}
	return c
}

// buildWidgetCards renders the requested widget types for a household. m is
// the viewer (kids don't get money widgets); a zero-ID owner-like member is
// used for TV links.
func (app *App) buildWidgetCards(ctx context.Context, m memberInfo, types []string, now time.Time) (map[string]widgetCard, error) {
	aid := m.AccountID
	day := householdToday(ctx, app, aid, now)
	cards := map[string]widgetCard{}
	want := func(t string) bool { return slices.Contains(types, t) }
	secs := []TodaySection{}
	if want("reminders") || want("chores") || want("lists") || want("meal_requests") {
		secs = runTodayProviders(ctx, app, aid, m, day)
	}
	if want("reminders") {
		cards["reminders"] = sectionCard("reminders", "Reminders today", secs, "reminders", "Nothing due today")
	}
	if want("chores") {
		cards["chores"] = sectionCard("chores", "Chores today", secs, "chores", "No chores today")
	}
	if want("lists") {
		cards["lists"] = sectionCard("lists", "Shopping lists", secs, "lists", "No open shopping lists")
	}
	if want("meal_requests") {
		cards["meal_requests"] = sectionCard("meal_requests", "Meal requests", secs, "meal_requests", "No requests waiting")
	}
	if want("agenda") {
		occ, err := app.activitiesOn(ctx, m, day, 7)
		if err != nil {
			return nil, err
		}
		c := widgetCard{Type: "agenda", Title: "Next 7 days", Rows: []widgetRow{}, Empty: "Nothing planned"}
		for _, o := range occ {
			when := o.OccursOn
			if t, err := parseDay(o.OccursOn); err == nil {
				switch {
				case t.Equal(day):
					when = "Today"
				case t.Equal(day.AddDate(0, 0, 1)):
					when = "Tomorrow"
				default:
					when = t.Format("Mon 2 Jan")
				}
			}
			c.Rows = append(c.Rows, widgetRow{Title: o.Title, Detail: when + " · " + activityTimeLabel(o, o.OccursOn)})
			if len(c.Rows) >= 12 {
				break
			}
		}
		cards["agenda"] = c
	}
	if want("meals") {
		c := widgetCard{Type: "meals", Title: "Today's meals", Rows: []widgetRow{}, Empty: "No meal planned"}
		if plan, err := app.buildMealPlan(ctx, aid, true, now); err == nil {
			d := day.Format("2006-01-02")
			for _, md := range plan.Days {
				if md.Date == d {
					c.Rows = append(c.Rows, widgetRow{Title: md.Protein, Detail: "Main dish"}, widgetRow{Title: md.Veggie, Detail: "Vegetables"})
				}
			}
		}
		if picks, err := app.loadMealPicks(ctx, aid, day, day); err == nil {
			for _, p := range picks[day.Format("2006-01-02")] {
				c.Rows = append([]widgetRow{{Title: p.Dish, Detail: "Requested by " + p.MemberName}}, c.Rows...)
			}
		}
		cards["meals"] = c
	}
	if want("family") {
		st := app.loadGamificationState(ctx, aid)
		c := widgetCard{Type: "family", Title: "Family points", Big: fmt.Sprintf("%d pts", st.Points), Rows: []widgetRow{}}
		c.Caption = fmt.Sprintf("%d-day streak", st.StreakDays)
		if st.Badge != "" {
			c.Caption += " · " + st.Badge
		}
		cards["family"] = c
	}
	if m.Role != roleKid {
		if want("budget") {
			c, err := app.budgetCard(ctx, aid, day)
			if err != nil {
				return nil, err
			}
			cards["budget"] = c
		}
		if want("goals") {
			goals, err := app.loadGoals(ctx, "WHERE g.account_id=$1 ORDER BY g.id", aid)
			if err != nil {
				return nil, err
			}
			c := widgetCard{Type: "goals", Title: "Savings goals", Rows: []widgetRow{}, Empty: "No goals yet"}
			for _, g := range goals {
				pct := min(g.Pct, 100)
				prog := g.Progress
				c.Rows = append(c.Rows, widgetRow{Title: g.Name, Detail: fmt.Sprintf("%.0f%%", g.Pct), Amount: &prog, Pct: &pct})
			}
			cards["goals"] = c
		}
	}
	return cards, nil
}

// budgetCard: money left this month (budgeted minus spent, excluding savings
// categories) and the categories closest to / over their budget.
func (app *App) budgetCard(ctx context.Context, aid int64, day time.Time) (widgetCard, error) {
	year, month := day.Year(), int(day.Month())
	report, err := app.tvMonthReport(ctx, aid, year, month)
	if err != nil {
		return widgetCard{}, err
	}
	list, _ := report["expenses_list"].([]tvExpense)
	var budget, spent int64
	type cat struct {
		name  string
		alloc int64
		real  int64
	}
	var cats []cat
	for _, e := range list {
		if e.MainCategory == "Savings" {
			continue
		}
		budget += e.Allocation
		spent += e.Realization
		if e.Allocation > 0 || e.Realization > 0 {
			cats = append(cats, cat{e.CategoryName, e.Allocation, e.Realization})
		}
	}
	left := budget - spent
	c := widgetCard{Type: "budget", Title: "Budget this month", BigAmount: &left, Rows: []widgetRow{}, Empty: "No budget set for this month"}
	c.Caption = "left of budget"
	if budget == 0 && spent == 0 {
		c.BigAmount = nil
		return c, nil
	}
	slices.SortFunc(cats, func(a, b cat) int {
		pa, pb := float64(a.real)/float64(max(a.alloc, 1)), float64(b.real)/float64(max(b.alloc, 1))
		switch {
		case pa > pb:
			return -1
		case pa < pb:
			return 1
		}
		return 0
	})
	for i, x := range cats {
		if i >= 5 {
			break
		}
		pct := 0.0
		if x.alloc > 0 {
			pct = min(float64(x.real)*100/float64(x.alloc), 100)
		}
		real := x.real
		c.Rows = append(c.Rows, widgetRow{Title: x.name, Amount: &real, Pct: &pct, Alert: x.alloc > 0 && x.real > x.alloc})
	}
	return c, nil
}

func parseWidgetTypes(q string) []string {
	if strings.TrimSpace(q) == "" {
		out := make([]string, len(widgetTypes))
		for i, t := range widgetTypes {
			out[i] = t.Type
		}
		return out
	}
	var out []string
	for _, t := range strings.Split(q, ",") {
		if t = strings.TrimSpace(t); t != "" && slices.ContainsFunc(widgetTypes, func(w widgetType) bool { return w.Type == t }) {
			out = append(out, t)
		}
	}
	return out
}

// GET /api/widgets/data -> {timezone, now (local), date, household_name, currency, cards: {type: card}}
func (app *App) handleWidgetData(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	now := time.Now().UTC()
	cards, err := app.buildWidgetCards(ctx, m, parseWidgetTypes(r.URL.Query().Get("types")), now)
	if err != nil {
		dbError(w, err)
		return
	}
	loc := householdLocation(ctx, app, m.AccountID)
	var currency string
	_ = app.DB.QueryRow(ctx, "SELECT COALESCE(currency,'IDR') FROM users WHERE id=$1", m.AccountID).Scan(&currency)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"timezone": loc.String(), "now": now.In(loc).Format(time.RFC3339), "date": householdToday(ctx, app, m.AccountID, now).Format("2006-01-02"),
		"household_name": app.loadHouseholdName(ctx, m.AccountID), "currency": currency, "cards": cards,
	})
}

// boardTypes lists the widget types a layout uses.
func boardTypes(layout json.RawMessage) []string {
	var l struct {
		Widgets []widgetLayoutItem `json:"widgets"`
	}
	_ = json.Unmarshal(layout, &l)
	var out []string
	for _, x := range l.Widgets {
		if !slices.Contains(out, x.Type) {
			out = append(out, x.Type)
		}
	}
	return out
}
