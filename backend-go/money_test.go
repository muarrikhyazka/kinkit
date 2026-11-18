package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ---------- pure unit tests ----------

func mdate(s string) time.Time {
	t, err := time.Parse(isoDateLayout, s)
	if err != nil {
		panic(err)
	}
	return t
}

func datesStr(ts []time.Time) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = t.Format(isoDateLayout)
	}
	return strings.Join(parts, ",")
}

func TestRecurringOccurrences(t *testing.T) {
	end := mdate("2026-03-10")
	cases := []struct {
		name     string
		rule     recurRule
		from, to string
		want     string
	}{
		{"monthly day 31 clamps to month end", recurRule{Frequency: "monthly", DayOfMonth: 31, Start: mdate("2026-01-01")},
			"2026-01-01", "2026-04-30", "2026-01-31,2026-02-28,2026-03-31,2026-04-30"},
		{"monthly clamps to Feb 29 in leap year", recurRule{Frequency: "monthly", DayOfMonth: 30, Start: mdate("2028-01-01")},
			"2028-02-01", "2028-03-31", "2028-02-29,2028-03-30"},
		{"from before start is clamped to start", recurRule{Frequency: "monthly", DayOfMonth: 5, Start: mdate("2026-02-10")},
			"2025-01-01", "2026-04-05", "2026-03-05,2026-04-05"},
		{"end date bounds", recurRule{Frequency: "monthly", DayOfMonth: 1, Start: mdate("2026-01-01"), End: &end},
			"2026-01-01", "2026-12-31", "2026-01-01,2026-02-01,2026-03-01"},
		{"weekly on Monday", recurRule{Frequency: "weekly", Weekday: 1, Start: mdate("2026-11-01")},
			"2026-11-01", "2026-11-17", "2026-11-02,2026-11-09,2026-11-16"},
		{"yearly on start month, Feb 29 clamps", recurRule{Frequency: "yearly", DayOfMonth: 29, Start: mdate("2028-02-29")},
			"2028-01-01", "2030-12-31", "2028-02-29,2029-02-28,2030-02-28"},
		{"to before from is empty", recurRule{Frequency: "monthly", DayOfMonth: 1, Start: mdate("2026-01-01")},
			"2026-05-02", "2026-05-01", ""},
	}
	for _, c := range cases {
		got := datesStr(recurringOccurrences(c.rule, mdate(c.from), mdate(c.to)))
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestRecurringCatchUpCap(t *testing.T) {
	r := recurRule{Frequency: "weekly", Weekday: 0, Start: mdate("2000-01-01")}
	if n := len(recurringOccurrences(r, mdate("2000-01-01"), mdate("2030-01-01"))); n != maxRecurringCatchUp {
		t.Fatalf("cap: got %d", n)
	}
}

func TestPendingRangeAndNext(t *testing.T) {
	last := mdate("2026-10-31")
	from, to := pendingRecurringRange(mdate("2026-01-15"), &last, mdate("2026-11-17"))
	if from.Format(isoDateLayout) != "2026-11-01" || to.Format(isoDateLayout) != "2026-11-17" {
		t.Fatalf("range %v %v", from, to)
	}
	from, _ = pendingRecurringRange(mdate("2026-01-15"), nil, mdate("2026-11-17"))
	if from.Format(isoDateLayout) != "2026-01-15" {
		t.Fatalf("no last: %v", from)
	}
	r := recurRule{Frequency: "monthly", DayOfMonth: 31, Start: mdate("2026-01-01")}
	if n := nextRecurringDate(r, mdate("2026-11-17")); n == nil || n.Format(isoDateLayout) != "2026-11-30" {
		t.Fatalf("next: %v", n)
	}
	end := mdate("2026-06-01")
	r.End = &end
	if n := nextRecurringDate(r, mdate("2026-11-17")); n != nil {
		t.Fatalf("ended rule has next %v", n)
	}
}

func TestGoalMonthlyNeeded(t *testing.T) {
	target := mdate("2026-12-31")
	if m, n := goalMonthlyNeeded(1000, mdate("2026-11-17"), &target); m != 2 || n != 500 {
		t.Fatalf("got %d %d", m, n)
	}
	if m, n := goalMonthlyNeeded(1001, mdate("2026-11-17"), &target); m != 2 || n != 501 {
		t.Fatalf("ceil: got %d %d", m, n)
	}
	past := mdate("2026-01-01")
	if m, n := goalMonthlyNeeded(700, mdate("2026-11-17"), &past); m != 1 || n != 700 {
		t.Fatalf("overdue: got %d %d", m, n)
	}
	if m, _ := goalMonthlyNeeded(700, mdate("2026-11-17"), nil); m != 0 {
		t.Fatal("no target date should give 0")
	}
	if m, _ := goalMonthlyNeeded(0, mdate("2026-11-17"), &target); m != 0 {
		t.Fatal("reached goal should give 0")
	}
}

func TestAlertThresholdAndFormat(t *testing.T) {
	for _, c := range []struct {
		spent, budget int64
		want          int
	}{{0, 1000, 0}, {799, 1000, 0}, {800, 1000, 80}, {1000, 1000, 80}, {1001, 1000, 100}, {500, 0, 0}} {
		if got := alertThreshold(c.spent, c.budget); got != c.want {
			t.Errorf("alertThreshold(%d,%d)=%d want %d", c.spent, c.budget, got, c.want)
		}
	}
	if got := formatAlertAmount(1234567, "IDR", nil); got != "Rp1.234.567" {
		t.Errorf("idr: %q", got)
	}
	if got := formatAlertAmount(20000000, "USD", map[string]float64{"USD": 0.00006}); got != "USD 1,200.00" {
		t.Errorf("usd: %q", got)
	}
	if got := formatAlertAmount(500, "XXX", nil); got != "Rp500" {
		t.Errorf("fallback: %q", got)
	}
}

// ---------- integration tests ----------

type setupLists struct {
	Categories    []categoryRow     `json:"categories"`
	IncomeSources []incomeSourceRow `json:"income_sources"`
}

func moneySetup(t *testing.T) (*App, *testClient, int64, setupLists) {
	app, srv := newTestServer(t)
	c := newClient(t, srv)
	_, aid := c.signup()
	var s setupLists
	c.do("GET", "/api/setup", nil, &s)
	if len(s.Categories) == 0 || len(s.IncomeSources) == 0 {
		t.Fatalf("signup should seed categories + income sources: %+v", s)
	}
	return app, c, aid, s
}

func countRecurringSpending(t *testing.T, app *App, aid int64) int {
	var n int
	if err := app.DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM spending WHERE account_id=$1 AND source='recurring'", aid).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRecurringGenerationIntegration(t *testing.T) {
	app, c, aid, s := moneySetup(t)
	ctx := context.Background()
	today := moneyUTCDate(time.Now().UTC())
	start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -3, 0) // 4 month starts ago

	var res struct {
		ID        int64 `json:"id"`
		Generated int   `json:"generated"`
	}
	if code := c.do("POST", "/api/recurring", map[string]any{
		"description": "Rent", "category_id": s.Categories[0].ID, "amount": 5000, "frequency": "monthly",
		"day_of_month": 1, "start_date": start.Format(isoDateLayout),
	}, &res); code != 200 {
		t.Fatalf("create: %d", code)
	}
	if res.Generated != 4 { // catch-up: 4 months incl. this month's 1st
		t.Fatalf("catch-up generated %d, want 4", res.Generated)
	}
	if n := countRecurringSpending(t, app, aid); n != 4 {
		t.Fatalf("spending rows %d", n)
	}
	var cat *string
	app.DB.QueryRow(ctx, "SELECT category_name FROM spending WHERE account_id=$1 AND source='recurring' LIMIT 1", aid).Scan(&cat)
	if cat == nil || *cat != s.Categories[0].Name {
		t.Fatalf("category_name not set: %v", cat)
	}

	// Idempotent: rerun, and rerun after forgetting last_generated_date.
	if n, err := app.generateRecurring(ctx, aid, 0, today); err != nil || n != 0 {
		t.Fatalf("rerun: %d %v", n, err)
	}
	app.DB.Exec(ctx, "UPDATE recurring_transactions SET last_generated_date=NULL WHERE id=$1", res.ID)
	if n, err := app.generateRecurring(ctx, aid, 0, today); err != nil || n != 0 {
		t.Fatalf("rerun after reset: %d %v", n, err)
	}
	// Next month generates exactly one more.
	nextMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	if n, err := app.generateRecurring(ctx, aid, 0, nextMonth); err != nil || n != 1 {
		t.Fatalf("next month: %d %v", n, err)
	}
	app.DB.Exec(ctx, "DELETE FROM spending WHERE account_id=$1 AND tx_date=$2 AND source='recurring'", aid, nextMonth)
	app.DB.Exec(ctx, "UPDATE recurring_transactions SET last_generated_date=$2 WHERE id=$1", res.ID, today)

	// List shows next date; pause stops generation; resume does not backfill.
	var list []recurringRow
	c.do("GET", "/api/recurring", nil, &list)
	if len(list) != 1 || list[0].NextDate == nil || *list[0].NextDate != nextMonth.Format(isoDateLayout) {
		t.Fatalf("list/next: %+v", list)
	}
	if code := c.do("PATCH", fmt.Sprintf("/api/recurring/%d", res.ID), map[string]any{"active": false}, nil); code != 200 {
		t.Fatalf("pause: %d", code)
	}
	if n, _ := app.generateRecurring(ctx, aid, 0, nextMonth.AddDate(0, 2, 0)); n != 0 {
		t.Fatalf("paused rule generated %d", n)
	}
	if code := c.do("PATCH", fmt.Sprintf("/api/recurring/%d", res.ID), map[string]any{"active": true, "amount": 6000}, nil); code != 200 {
		t.Fatalf("resume: %d", code)
	}
	var amount int64
	var active bool
	app.DB.QueryRow(ctx, "SELECT amount, active FROM recurring_transactions WHERE id=$1", res.ID).Scan(&amount, &active)
	if amount != 6000 || !active {
		t.Fatalf("patch not applied: %d %v", amount, active)
	}

	// Validation + isolation.
	if code := c.do("POST", "/api/recurring", map[string]any{"description": "x", "amount": 1, "frequency": "daily"}, nil); code != 400 {
		t.Fatalf("bad frequency: %d", code)
	}
	_, srv2 := newTestServer(t)
	other := newClient(t, srv2)
	other.signup()
	if code := other.do("DELETE", fmt.Sprintf("/api/recurring/%d", res.ID), nil, nil); code != 200 {
		t.Fatalf("other delete: %d", code)
	}
	var still int
	app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM recurring_transactions WHERE id=$1", res.ID).Scan(&still)
	if still != 1 {
		t.Fatal("another account deleted the rule")
	}
	c.do("DELETE", fmt.Sprintf("/api/recurring/%d", res.ID), nil, nil)
	if n := countRecurringSpending(t, app, aid); n != 4 {
		t.Fatalf("deleting a rule must keep generated spending: %d", n)
	}
}

func TestIncomeEntriesCRUDAndReports(t *testing.T) {
	_, c, _, s := moneySetup(t)
	src := s.IncomeSources[0]
	var add struct {
		ID        int64 `json:"id"`
		Duplicate bool  `json:"duplicate"`
	}
	if code := c.do("POST", "/api/income", map[string]any{"source_id": src.ID, "tx_date": "2026-05-25", "description": "Salary", "amount": 9000, "client_id": "inc-1"}, &add); code != 200 {
		t.Fatalf("add: %d", code)
	}
	var dup struct {
		ID        int64 `json:"id"`
		Duplicate bool  `json:"duplicate"`
	}
	c.do("POST", "/api/income", map[string]any{"source_id": src.ID, "tx_date": "2026-05-25", "amount": 9000, "client_id": "inc-1"}, &dup)
	if !dup.Duplicate || dup.ID != add.ID {
		t.Fatalf("client_id idempotency: %+v", dup)
	}
	c.do("POST", "/api/income", map[string]any{"tx_date": "2026-05-02", "description": "Gift", "amount": 1000}, nil)
	c.do("POST", "/api/income", map[string]any{"tx_date": "2026-06-02", "description": "June", "amount": 7}, nil)
	if code := c.do("POST", "/api/income", map[string]any{"tx_date": "2026-06-02", "amount": 0}, nil); code != 400 {
		t.Fatalf("zero amount: %d", code)
	}

	var list struct {
		Rows     []incomeEntryRow `json:"rows"`
		Total    int64            `json:"total"`
		BySource []struct {
			SourceID *int64 `json:"source_id"`
			Total    int64  `json:"total"`
		} `json:"by_source"`
	}
	c.do("GET", "/api/income?year=2026&month=5", nil, &list)
	if len(list.Rows) != 2 || list.Total != 10000 || len(list.BySource) != 2 {
		t.Fatalf("list: %+v", list)
	}
	if list.Rows[0].SourceName == nil || *list.Rows[0].SourceName != src.Name {
		t.Fatalf("source name: %+v", list.Rows[0])
	}
	c.do("GET", "/api/income?year=2026", nil, &list)
	if list.Total != 10007 {
		t.Fatalf("year total: %d", list.Total)
	}

	if code := c.do("PATCH", fmt.Sprintf("/api/income/%d", add.ID), map[string]any{"amount": 9500}, nil); code != 200 {
		t.Fatalf("patch: %d", code)
	}
	var rep map[string]any
	c.do("GET", "/api/report/2026/5", nil, &rep)
	if rep["income_actual"].(float64) != 10500 {
		t.Fatalf("report income_actual: %v", rep["income_actual"])
	}
	if _, ok := rep["total_income"]; !ok {
		t.Fatal("report lost total_income")
	}
	var trend []map[string]any
	c.do("GET", "/api/trend?year=2026&month=6&count=2", nil, &trend)
	if len(trend) != 2 || trend[0]["income_actual"].(float64) != 10500 || trend[1]["income_actual"].(float64) != 7 {
		t.Fatalf("trend: %v", trend)
	}
	var years []map[string]any
	c.do("GET", "/api/years", nil, &years)
	if len(years) == 0 || years[len(years)-1]["income_actual"].(float64) != 10507 {
		t.Fatalf("years: %v", years)
	}

	if code := c.do("DELETE", fmt.Sprintf("/api/income/%d", add.ID), nil, nil); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	c.do("GET", "/api/income?year=2026&month=5", nil, &list)
	if len(list.Rows) != 1 || list.Total != 1000 {
		t.Fatalf("after delete: %+v", list)
	}
}

func TestGoalsProgress(t *testing.T) {
	app, c, aid, s := moneySetup(t)
	ctx := context.Background()
	cat := s.Categories[0]
	today := moneyUTCDate(time.Now().UTC())
	target := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0) // next month -> 2 months left

	var asset struct {
		ID int64 `json:"id"`
	}
	app.DB.QueryRow(ctx, "INSERT INTO assets (account_id, description, value) VALUES ($1,'Piggy',300) RETURNING id", aid).Scan(&asset.ID)

	var add struct {
		ID int64 `json:"id"`
	}
	if code := c.do("POST", "/api/goals", map[string]any{
		"name": "Bike", "target_amount": 10000, "target_date": target.Format(isoDateLayout), "saved_amount": 1000,
		"category_id": cat.ID, "asset_ids": []int64{asset.ID}, "color": "#4f46e5",
	}, &add); code != 200 {
		t.Fatalf("create: %d", code)
	}
	// Linked spending: today counts, before the goal existed does not.
	app.DB.Exec(ctx, "INSERT INTO spending (account_id, tx_date, description, category_id, category_name, amount) VALUES ($1,$2,'save',$3,$4,2000)", aid, today, cat.ID, cat.Name)
	app.DB.Exec(ctx, "INSERT INTO spending (account_id, tx_date, description, category_id, category_name, amount) VALUES ($1,$2,'old',$3,$4,99999)", aid, today.AddDate(0, 0, -40), cat.ID, cat.Name)

	var res struct {
		SavedAmount int64 `json:"saved_amount"`
	}
	if code := c.do("POST", fmt.Sprintf("/api/goals/%d/contribute", add.ID), map[string]any{"amount": 700}, &res); code != 200 || res.SavedAmount != 1700 {
		t.Fatalf("contribute: %d %+v", code, res)
	}
	var goals []goalRow
	c.do("GET", "/api/goals", nil, &goals)
	if len(goals) != 1 {
		t.Fatalf("goals: %+v", goals)
	}
	g := goals[0]
	// 1700 saved + 2000 linked spending + 300 asset = 4000; 6000 left over 2 months.
	if g.LinkedAmount != 2300 || g.Progress != 4000 || g.Remaining != 6000 || g.Pct != 40 {
		t.Fatalf("progress: %+v", g)
	}
	if g.MonthsLeft == nil || *g.MonthsLeft != 2 || g.MonthlyNeeded == nil || *g.MonthlyNeeded != 3000 {
		t.Fatalf("monthly needed: %+v", g)
	}
	// Withdraw can't go below zero; patch keeps unspecified fields.
	c.do("POST", fmt.Sprintf("/api/goals/%d/contribute", add.ID), map[string]any{"amount": -99999}, &res)
	if res.SavedAmount != 0 {
		t.Fatalf("withdraw floor: %+v", res)
	}
	c.do("PATCH", fmt.Sprintf("/api/goals/%d", add.ID), map[string]any{"name": "E-bike", "target_date": nil}, nil)
	c.do("GET", "/api/goals", nil, &goals)
	if goals[0].Name != "E-bike" || goals[0].TargetAmount != 10000 || goals[0].TargetDate != nil || goals[0].MonthlyNeeded != nil || goals[0].CategoryID == nil {
		t.Fatalf("patch: %+v", goals[0])
	}
	c.do("DELETE", fmt.Sprintf("/api/goals/%d", add.ID), nil, nil)
	c.do("GET", "/api/goals", nil, &goals)
	if len(goals) != 0 {
		t.Fatalf("delete: %+v", goals)
	}
}

func TestBudgetActuals(t *testing.T) {
	app, c, aid, s := moneySetup(t)
	ctx := context.Background()
	cat, src := s.Categories[0], s.IncomeSources[0]
	app.DB.Exec(ctx, "INSERT INTO spending (account_id, tx_date, description, category_id, category_name, amount) VALUES ($1,'2026-03-05','a',$2,$3,100),($1,'2026-03-20','b',$2,$3,250),($1,'2026-04-01','c',$2,$3,7),($1,'2025-03-01','old',$2,$3,1)", aid, cat.ID, cat.Name)
	app.DB.Exec(ctx, "INSERT INTO income_entries (account_id, source_id, tx_date, amount) VALUES ($1,$2,'2026-03-25',5000),($1,NULL,'2026-03-26',10)", aid, src.ID)

	var res struct {
		Expense []struct {
			CategoryID int64 `json:"category_id"`
			Month      int   `json:"month"`
			Amount     int64 `json:"amount"`
		} `json:"expense"`
		Income []struct {
			SourceID *int64 `json:"source_id"`
			Month    int    `json:"month"`
			Amount   int64  `json:"amount"`
		} `json:"income"`
	}
	if code := c.do("GET", "/api/budgeting/2026/actuals", nil, &res); code != 200 {
		t.Fatalf("actuals: %d", code)
	}
	if len(res.Expense) != 2 || res.Expense[0].Month != 3 || res.Expense[0].Amount != 350 || res.Expense[1].Amount != 7 {
		t.Fatalf("expense actuals: %+v", res.Expense)
	}
	if len(res.Income) != 2 {
		t.Fatalf("income actuals: %+v", res.Income)
	}
	for _, i := range res.Income {
		if (i.SourceID != nil && i.Amount != 5000) || (i.SourceID == nil && i.Amount != 10) {
			t.Fatalf("income actuals: %+v", res.Income)
		}
	}
}

func TestBudgetAlertsIdempotent(t *testing.T) {
	app, c, aid, s := moneySetup(t)
	ctx := context.Background()
	cat := s.Categories[0]
	now := time.Now().UTC()
	chat := "chat-" + uniqueEmail()

	type sent struct{ platform, token, chat, text string }
	var msgs []sent
	fail := false
	orig := sendAlertMessage
	sendAlertMessage = func(_ context.Context, platform, token, chatID, text string) error {
		if chatID != chat {
			return nil // another test account
		}
		if fail {
			return errors.New("boom")
		}
		msgs = append(msgs, sent{platform, token, chatID, text})
		return nil
	}
	t.Cleanup(func() { sendAlertMessage = orig })

	if code := c.do("POST", "/api/integrations", map[string]any{"platform": "telegram", "platform_token": "123456:secret-token", "chat_id": chat}, nil); code != 200 {
		t.Fatalf("integration: %d", code)
	}
	c.do("POST", "/api/budgeting/expense", map[string]any{"year": now.Year(), "month": int(now.Month()), "category_id": cat.ID, "amount": 1000}, nil)
	spend := func(amt int64) {
		if _, err := app.DB.Exec(ctx, "INSERT INTO spending (account_id, tx_date, description, category_id, category_name, amount) VALUES ($1,$2,'x',$3,$4,$5)",
			aid, moneyUTCDate(now), cat.ID, cat.Name, amt); err != nil {
			t.Fatal(err)
		}
	}
	run := func() { _ = app.checkBudgetAlerts(ctx, now) }

	spend(500)
	run()
	if len(msgs) != 0 {
		t.Fatalf("50%% should not alert: %+v", msgs)
	}
	spend(350) // 85%
	run()
	run()
	if len(msgs) != 1 || !strings.Contains(msgs[0].text, "85%") || msgs[0].token != "123456:secret-token" || msgs[0].platform != "telegram" {
		t.Fatalf("80%% alert once: %+v", msgs)
	}

	spend(200) // 105%, delivery fails -> claim released, retried later
	fail = true
	run()
	fail = false
	if len(msgs) != 1 {
		t.Fatalf("failed send recorded: %+v", msgs)
	}
	run()
	run()
	if len(msgs) != 2 || !strings.Contains(msgs[1].text, "OVER") {
		t.Fatalf("100%% alert once after retry: %+v", msgs)
	}

	var settings struct {
		AlertsEnabled bool `json:"alerts_enabled"`
		Channels      []struct {
			Platform string `json:"platform"`
			Ready    bool   `json:"ready"`
		} `json:"channels"`
		Recent []struct {
			Threshold int `json:"threshold"`
		} `json:"recent"`
	}
	c.do("GET", "/api/alerts/settings", nil, &settings)
	if !settings.AlertsEnabled || len(settings.Channels) != 1 || !settings.Channels[0].Ready || len(settings.Recent) != 2 {
		t.Fatalf("settings: %+v", settings)
	}

	// Opt-out: a second category crossing 100% sends nothing.
	cat2 := s.Categories[1]
	var out map[string]any
	if code := c.do("PATCH", "/api/alerts/settings", map[string]any{"alerts_enabled": false}, &out); code != 200 || out["alerts_enabled"] != false {
		t.Fatalf("disable: %d %v", code, out)
	}
	c.do("POST", "/api/budgeting/expense", map[string]any{"year": now.Year(), "month": int(now.Month()), "category_id": cat2.ID, "amount": 10}, nil)
	app.DB.Exec(ctx, "INSERT INTO spending (account_id, tx_date, description, category_id, category_name, amount) VALUES ($1,$2,'y',$3,$4,50)", aid, moneyUTCDate(now), cat2.ID, cat2.Name)
	run()
	if len(msgs) != 2 {
		t.Fatalf("disabled account alerted: %+v", msgs)
	}
	if code := c.do("PATCH", "/api/alerts/settings", map[string]any{}, nil); code != 400 {
		t.Fatalf("missing field: %d", code)
	}
}
