package main

import (
	"context"
	"slices"
	"testing"
	"time"
)

func d(s string) *time.Time {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func TestEffectiveStreak(t *testing.T) {
	today := *d("2026-11-17")
	cases := []struct {
		name   string
		last   *time.Time
		stored int
		want   int
	}{
		{"never logged", nil, 0, 0},
		{"logged today", d("2026-11-17"), 4, 4},
		{"logged yesterday", d("2026-11-16"), 4, 4},
		{"missed one day", d("2026-11-15"), 4, 0},
		{"long gap", d("2025-01-01"), 30, 0},
		{"future date (clock skew)", d("2026-11-18"), 3, 0},
		{"month boundary", d("2026-02-28"), 2, 2}, // today below overridden
	}
	for _, c := range cases {
		td := today
		if c.name == "month boundary" {
			td = *d("2026-03-01")
		}
		if got := effectiveStreak(c.last, c.stored, td); got != c.want {
			t.Errorf("%s: effectiveStreak = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestEffectiveStreakUsesUTCDates(t *testing.T) {
	// 23:30 on Nov 17 in UTC-5 is already Nov 18 in UTC; a log dated Nov 16
	// is therefore two UTC days old and the streak must be 0.
	ny := time.FixedZone("UTC-5", -5*3600)
	today := time.Date(2026, 11, 17, 23, 30, 0, 0, ny)
	if got := effectiveStreak(d("2026-11-16"), 5, today); got != 0 {
		t.Fatalf("want 0 (UTC day is Nov 18), got %d", got)
	}
	if got := effectiveStreak(d("2026-11-17"), 5, today); got != 5 {
		t.Fatalf("want 5 (yesterday in UTC), got %d", got)
	}
}

func TestNextStreak(t *testing.T) {
	today := *d("2026-11-17")
	cases := []struct {
		name      string
		last      *time.Time
		stored    int
		want      int
		wantFirst bool
	}{
		{"first ever log", nil, 0, 1, true},
		{"second log same day", d("2026-11-17"), 3, 3, false},
		{"consecutive day", d("2026-11-16"), 3, 4, true},
		{"gap resets", d("2026-11-14"), 9, 1, true},
		{"same day with corrupt 0 streak", d("2026-11-17"), 0, 1, false},
	}
	for _, c := range cases {
		got, first := nextStreak(c.last, c.stored, today)
		if got != c.want || first != c.wantFirst {
			t.Errorf("%s: nextStreak = (%d,%v), want (%d,%v)", c.name, got, first, c.want, c.wantFirst)
		}
	}
	// year boundary
	if got, _ := nextStreak(d("2026-12-31"), 7, *d("2027-01-01")); got != 8 {
		t.Errorf("year boundary: got %d want 8", got)
	}
}

func TestGamificationStreakIntegration(t *testing.T) {
	app, srv := newTestServer(t)
	c := newClient(t, srv)
	_, aid := c.signup()
	ctx := context.Background()
	today := todayUTC()

	var g gamificationState
	c.do("GET", "/api/gamification", nil, &g)
	if g.Points != 0 || g.StreakDays != 0 || g.LastLogDate != nil {
		t.Fatalf("fresh account: %+v", g)
	}

	add := func() {
		c.do("POST", "/api/spending", map[string]any{"tx_date": today.Format(dateLayout), "description": "x", "amount": 1}, nil)
	}
	add()
	g = gamificationState{}
	c.do("GET", "/api/gamification", nil, &g)
	if g.StreakDays != 1 || g.Points != int64(pointsPerLog+streakBonusPerDay) || g.LastLogDate == nil || *g.LastLogDate != today.Format(dateLayout) {
		t.Fatalf("after first log: %+v", g)
	}
	add() // same day: no extra points
	g2 := gamificationState{}
	c.do("GET", "/api/gamification", nil, &g2)
	if g2.Points != g.Points || g2.StreakDays != 1 {
		t.Fatalf("second log same day changed state: %+v", g2)
	}

	// Pretend the last log was yesterday with a 4-day streak: still shown.
	if _, err := app.DB.Exec(ctx, "UPDATE user_points SET streak_days=4, last_log_date=$1::date WHERE account_id=$2",
		today.AddDate(0, 0, -1).Format(dateLayout), aid); err != nil {
		t.Fatal(err)
	}
	g = gamificationState{}
	c.do("GET", "/api/gamification", nil, &g)
	if g.StreakDays != 4 {
		t.Fatalf("streak from yesterday should still show 4: %+v", g)
	}
	add()
	g = gamificationState{}
	c.do("GET", "/api/gamification", nil, &g)
	if g.StreakDays != 5 {
		t.Fatalf("consecutive day should extend to 5: %+v", g)
	}

	// Last log 2 days ago: the stored streak is stale and must read as 0.
	if _, err := app.DB.Exec(ctx, "UPDATE user_points SET streak_days=5, last_log_date=$1::date WHERE account_id=$2",
		today.AddDate(0, 0, -2).Format(dateLayout), aid); err != nil {
		t.Fatal(err)
	}
	g = gamificationState{}
	c.do("GET", "/api/gamification", nil, &g)
	if g.StreakDays != 0 {
		t.Fatalf("stale streak should read 0: %+v", g)
	}
	add()
	g = gamificationState{}
	c.do("GET", "/api/gamification", nil, &g)
	if g.StreakDays != 1 {
		t.Fatalf("log after gap should restart at 1: %+v", g)
	}
}

func TestLevels(t *testing.T) {
	cases := []struct {
		points int64
		level  int
		title  string
	}{
		{0, 1, "Getting Started"}, {49, 1, "Getting Started"}, {50, 2, "Tracker"},
		{149, 2, "Tracker"}, {150, 3, "Budget Rookie"}, {500, 5, "Money Master"},
		{2250, 10, "Finance Wizard"}, {1 << 40, maxLevel, "Legend"},
	}
	for _, c := range cases {
		if got := levelFor(c.points); got != c.level {
			t.Errorf("levelFor(%d) = %d, want %d", c.points, got, c.level)
		}
		if title, _, _ := currentBadge(c.points); title != c.title {
			t.Errorf("title at %d = %q, want %q", c.points, title, c.title)
		}
	}
	if _, next, at := currentBadge(0); next != "Tracker" || at != 50 {
		t.Errorf("next level from 0: %q at %d", next, at)
	}
	if _, next, _ := currentBadge(1 << 40); next != "" {
		t.Errorf("top level has no next, got %q", next)
	}
}

func TestMoreBadges(t *testing.T) {
	got := earnedBadges(badgeFacts{SpendingCount: 120, GoalsCreated: 1, GoalsReached: 1, AssetsCount: 2,
		IncomeCount: 1, RemindersDone: 10, ProposalsMade: 2, ProposalsApproved: 1, ActivitiesCreated: 1, Points: 600})
	for _, k := range []string{"first_log", "financial_geek", "goal_setter", "goal_crusher", "asset_tracker", "income_insider",
		"never_forget", "menu_maker", "chefs_choice", "planner", "level_5"} {
		if !slices.Contains(got, k) {
			t.Errorf("missing %s in %v", k, got)
		}
	}
	for _, k := range []string{"number_cruncher", "level_10", "streak_3"} {
		if slices.Contains(got, k) {
			t.Errorf("unexpected %s in %v", k, got)
		}
	}
	seen := map[string]bool{}
	for _, b := range badgeCatalog {
		if seen[b.Key] || b.Target < 1 || b.Category == "" || b.Tier == "" {
			t.Errorf("bad catalog entry %+v", b)
		}
		seen[b.Key] = true
	}
}
