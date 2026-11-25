package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// interval schedules (pure)

func TestOccurrencesEveryTwoWeeks(t *testing.T) {
	loc := mustLoc(t, "Asia/Jakarta")
	// start Wed 2026-11-18; Fridays every 2 weeks -> anchored on the week of
	// Sun 2026-11-15: Nov 20, Dec 4, Dec 18 (not Nov 27 / Dec 11).
	s := reminderSchedule{Kind: "weekly", TimeOfDay: "10:00", Weekdays: []int{5}, StartDate: rd("2026-11-18"), Interval: 2}
	got := occurrencesBetween(s, loc, utc("2026-11-17T00:00:00Z"), utc("2026-12-20T00:00:00Z"))
	want := "2026-11-20T03:00:00Z 2026-12-04T03:00:00Z 2026-12-18T03:00:00Z"
	if fmtTimes(got) != want {
		t.Fatalf("got %s want %s", fmtTimes(got), want)
	}
	// two weekdays in the anchored week: Tue (before start, skipped) + Fri
	s2 := reminderSchedule{Kind: "weekly", TimeOfDay: "19:00", Weekdays: []int{2, 5}, StartDate: rd("2026-11-18"), Interval: 2}
	got = occurrencesBetween(s2, loc, utc("2026-11-15T00:00:00Z"), utc("2026-12-06T00:00:00Z"))
	want = "2026-11-20T12:00:00Z 2026-12-01T12:00:00Z 2026-12-04T12:00:00Z"
	if fmtTimes(got) != want {
		t.Fatalf("two days: got %s want %s", fmtTimes(got), want)
	}
	// interval 1 behaves like a plain weekly schedule
	s3 := s
	s3.Interval = 1
	if n := len(occurrencesBetween(s3, loc, utc("2026-11-17T00:00:00Z"), utc("2026-12-20T00:00:00Z"))); n != 5 {
		t.Fatalf("weekly interval 1: %d occurrences", n)
	}
}

func TestOccurrencesEveryThreeAndSixMonths(t *testing.T) {
	loc := mustLoc(t, "Asia/Jakarta")
	// anchored on the start month (Nov): Nov 1 is before the start, so
	// first is Feb 1, then May 1, Aug 1.
	s := reminderSchedule{Kind: "monthly", TimeOfDay: "09:00", DayOfMonth: 1, StartDate: rd("2026-11-18"), Interval: 3}
	got := occurrencesBetween(s, loc, utc("2026-11-01T00:00:00Z"), utc("2027-09-01T00:00:00Z"))
	want := "2027-02-01T02:00:00Z 2027-05-01T02:00:00Z 2027-08-01T02:00:00Z"
	if fmtTimes(got) != want {
		t.Fatalf("3 months: got %s want %s", fmtTimes(got), want)
	}
	// every 6 months on the 31st clamps to month end (Nov 30 -> May 31 -> Nov 30)
	s6 := reminderSchedule{Kind: "monthly", TimeOfDay: "08:00", DayOfMonth: 31, StartDate: rd("2026-11-01"), Interval: 6}
	got = occurrencesBetween(s6, loc, utc("2026-11-01T00:00:00Z"), utc("2027-12-01T00:00:00Z"))
	want = "2026-11-30T01:00:00Z 2027-05-31T01:00:00Z 2027-11-30T01:00:00Z"
	if fmtTimes(got) != want {
		t.Fatalf("6 months: got %s want %s", fmtTimes(got), want)
	}
	// interval is ignored for daily/yearly
	d := reminderSchedule{Kind: "daily", TimeOfDay: "07:00", StartDate: rd("2026-11-18"), Interval: 3}
	if n := len(occurrencesBetween(d, loc, utc("2026-11-18T00:00:00Z"), utc("2026-11-21T00:00:00Z"))); n != 3 {
		t.Fatalf("daily ignores interval: %d", n)
	}
	// validation
	bad := reminderSchedule{Kind: "weekly", TimeOfDay: "07:00", StartDate: rd("2026-11-18"), Interval: 99}
	if bad.validate() == nil {
		t.Fatal("interval 99 should be invalid")
	}
}

func TestDescribeIntervalSchedules(t *testing.T) {
	cases := map[string]reminderSchedule{
		"Every 2 weeks on Fri at 10:00":         {Kind: "weekly", TimeOfDay: "10:00", Weekdays: []int{5}, StartDate: rd("2026-11-18"), Interval: 2},
		"Every 2 weeks on Tue and Fri at 19:00": {Kind: "weekly", TimeOfDay: "19:00", Weekdays: []int{5, 2}, StartDate: rd("2026-11-18"), Interval: 2},
		"Every 3 months on the 1st at 09:00":    {Kind: "monthly", TimeOfDay: "09:00", DayOfMonth: 1, StartDate: rd("2026-11-18"), Interval: 3},
		"Every month on the 20th at 09:00":      {Kind: "monthly", TimeOfDay: "09:00", DayOfMonth: 20, StartDate: rd("2026-11-18"), Interval: 1},
		"Every Saturday at 08:00":               {Kind: "weekly", TimeOfDay: "08:00", Weekdays: []int{6}, StartDate: rd("2026-11-18")},
	}
	for want, s := range cases {
		if got := describeSchedule(s, time.UTC); got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}

func TestParseIntervalReminders(t *testing.T) {
	loc := mustLoc(t, "Asia/Jakarta")
	now := utc("2026-11-18T03:00:00Z") // Wed 18 Nov 2026, 10:00 WIB
	type want struct {
		kind, title, when string
		interval          int
	}
	cases := map[string]want{
		"every 2 weeks on fri 10:00 change bed sheets":       {"weekly", "change bed sheets", "Every 2 weeks on Fri at 10:00", 2},
		"every other week on tue and fri at 7pm trash":       {"weekly", "trash", "Every 2 weeks on Tue and Fri at 19:00", 2},
		"every 2 weeks check tyre pressure":                  {"weekly", "check tyre pressure", "Every 2 weeks on Wed at 09:00", 2},
		"every 3 months on the 1st change car oil":           {"monthly", "change car oil", "Every 3 months on the 1st at 09:00", 3},
		"every 6 months 10am service the AC":                 {"monthly", "service the AC", "Every 6 months on the 18th at 10:00", 6},
		"every 1 week on sat clean the bathroom":             {"weekly", "clean the bathroom", "Every Saturday at 09:00", 0},
		"every week on thu 20:30 plan meals":                 {"weekly", "plan meals", "Every Thursday at 20:30", 0},
		"tiap 2 minggu jumat jam 9:00 cuci sajadah":          {"weekly", "cuci sajadah", "Every 2 weeks on Fri at 09:00", 2},
		"setiap 3 bulan kuras tandon":                        {"monthly", "kuras tandon", "Every 3 months on the 18th at 09:00", 3},
		"setiap 3 bulan tanggal 5 jam 08:00 ganti oli motor": {"monthly", "ganti oli motor", "Every 3 months on the 5th at 08:00", 3},
		"tiap hari jam 07:00 siram tanaman":                  {"daily", "siram tanaman", "Every day at 07:00", 0},
		"setiap sabtu jam 08:00 belanja mingguan":            {"weekly", "belanja mingguan", "Every Saturday at 08:00", 0},
		"every month on the 20th pay electricity":            {"monthly", "pay electricity", "Every month on the 20th at 09:00", 0},
	}
	for text, w := range cases {
		p, err := parseReminderText(text, now, loc)
		if err != nil {
			t.Errorf("%q: unexpected error %v", text, err)
			continue
		}
		got := describeSchedule(p.Schedule, loc)
		if p.Schedule.Kind != w.kind || p.Title != w.title || got != w.when || p.Schedule.Interval != w.interval {
			t.Errorf("%q: got kind=%s title=%q when=%q interval=%d; want %+v", text, p.Schedule.Kind, p.Title, got, p.Schedule.Interval, w)
		}
		if err := p.Schedule.validate(); err != nil {
			t.Errorf("%q: invalid schedule: %v", text, err)
		}
	}
	for _, bad := range []string{"every 2 days water plants", "every 0 weeks x", "every 99 weeks x", "every 2 fortnights x", "every 3"} {
		if _, err := parseReminderText(bad, now, loc); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

func TestGuessHouseholdCategory(t *testing.T) {
	cases := map[string]string{
		"change bed sheets & towels": "laundry", "wash the curtains": "laundry", "take out the trash": "cleaning",
		"clean the bathroom": "cleaning", "weekly grocery run": "groceries", "plan next week's meals": "cooking",
		"check tyre pressure": "vehicle", "change car oil": "vehicle", "clean the water tank": "home_maintenance",
		"water the plants": "home_maintenance", "pay electricity bill": "bill", "kuras tandon": "home_maintenance",
		"take medicine": "medicine", "something else": "other",
	}
	for title, want := range cases {
		if got := guessReminderCategory(title); got != want {
			t.Errorf("%q: got %s want %s", title, got, want)
		}
		if !validReminderCategories[guessReminderCategory(title)] {
			t.Errorf("%q: guessed an invalid category", title)
		}
	}
}

// integration

func TestRemindersHouseholdCategoriesAndInterval(t *testing.T) {
	h := newRemHousehold(t, "Asia/Jakarta")
	c := h.owner
	var res map[string]any
	for _, cat := range []string{"cleaning", "laundry", "cooking", "groceries", "vehicle", "home_maintenance", "self_care"} {
		h.createReminder(t, c, map[string]any{"title": "cat " + cat, "kind": "daily", "category": cat})
	}
	if code := c.do("POST", "/api/reminders", map[string]any{"title": "x", "kind": "weekly", "weekdays": []int{5}, "interval": 0, "category": "laundry"}, &res); code != 200 {
		t.Fatalf("interval 0 defaults to 1: %d %v", code, res)
	}
	if code := c.do("POST", "/api/reminders", map[string]any{"title": "x", "kind": "weekly", "interval": 53}, &res); code != 400 {
		t.Fatalf("interval 53: %d", code)
	}
	rid := h.createReminder(t, c, map[string]any{"title": "Change bed sheets", "kind": "weekly", "weekdays": []int{5},
		"time_of_day": "10:00", "interval": 2, "category": "laundry"})
	row, err := h.app.loadReminder(context.Background(), h.aid, rid)
	if err != nil || row.Interval != 2 || row.Category != "laundry" {
		t.Fatalf("row: %+v %v", row, err)
	}
	var list struct {
		Reminders  []reminderRow       `json:"reminders"`
		Categories []map[string]string `json:"categories"`
	}
	if code := c.do("GET", "/api/reminders", nil, &list); code != 200 || len(list.Categories) != len(reminderCategoryIDs) {
		t.Fatalf("list: %d %d", code, len(list.Categories))
	}
	for _, r := range list.Reminders {
		if r.ID == rid && (r.Description != "Every 2 weeks on Fri at 10:00" || r.Interval != 2 || r.NextDue == nil) {
			t.Fatalf("interval row: %+v", r)
		}
	}
	// occurrences 14 days apart
	var gaps []float64
	rows, _ := h.app.DB.Query(context.Background(), `SELECT EXTRACT(EPOCH FROM due_at - lag(due_at) OVER (ORDER BY due_at))/86400 FROM reminder_occurrences WHERE reminder_id=$1`, rid)
	for rows.Next() {
		var g *float64
		_ = rows.Scan(&g)
		if g != nil {
			gaps = append(gaps, *g)
		}
	}
	rows.Close()
	for _, g := range gaps {
		if g != 14 {
			t.Fatalf("gap between occurrences %v days, want 14", gaps)
		}
	}
	// PATCH interval only (kind kept); switching to daily resets it to 1
	if code := c.do("PATCH", fmt.Sprintf("/api/reminders/%d", rid), map[string]any{"interval": 3}, &res); code != 200 || res["description"] != "Every 3 weeks on Fri at 10:00" {
		t.Fatalf("patch interval: %d %v", code, res)
	}
	if code := c.do("PATCH", fmt.Sprintf("/api/reminders/%d", rid), map[string]any{"kind": "daily"}, &res); code != 200 {
		t.Fatalf("patch kind: %d", code)
	}
	row, _ = h.app.loadReminder(context.Background(), h.aid, rid)
	if row.Interval != 1 {
		t.Fatalf("daily keeps interval %d", row.Interval)
	}
	// schema constraint itself accepts the new categories and rejects junk
	if _, err := h.app.DB.Exec(context.Background(), "UPDATE reminders SET category='self_care' WHERE id=$1", rid); err != nil {
		t.Fatalf("db rejects self_care: %v", err)
	}
	if _, err := h.app.DB.Exec(context.Background(), "UPDATE reminders SET category='junk' WHERE id=$1", rid); err == nil {
		t.Fatal("db accepted junk category")
	}
	if _, err := h.app.DB.Exec(context.Background(), "UPDATE reminders SET repeat_interval=0 WHERE id=$1", rid); err == nil {
		t.Fatal("db accepted interval 0")
	}
}

func TestBotRemindInterval(t *testing.T) {
	stubChat(t)
	h := newRemHousehold(t, "Asia/Jakarta")
	var res map[string]any
	if c := h.bot(t, "", "POST", "/api/bot/reminders", map[string]any{"text": "every 2 weeks on fri 10:00 change bed sheets"}, &res); c != 200 || res["ok"] != true {
		t.Fatalf("bot remind: %d %v", c, res)
	}
	rid := int64(res["id"].(float64))
	row, _ := h.app.loadReminder(context.Background(), h.aid, rid)
	if row.Interval != 2 || row.Category != "laundry" || res["category"] != "laundry" {
		t.Fatalf("bot row: %+v %v", row, res)
	}
	h.bot(t, "", "POST", "/api/bot/reminders", map[string]any{"text": "tomorrow 9am wash the towels"}, &res)
	var list map[string]any
	if c := h.bot(t, "", "GET", "/api/bot/reminders", nil, &list); c != 200 {
		t.Fatalf("bot list: %d", c)
	}
	items, _ := list["items"].([]any)
	if len(items) == 0 || items[0].(map[string]any)["category_label"] != "Laundry" {
		t.Fatalf("bot list items: %v", list)
	}
}
