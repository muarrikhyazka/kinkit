package main

import (
	"fmt"
	"testing"
	"time"
)

func calDay(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }

func startsOf(a activityRow, from, to string, skip map[string]bool) []string {
	out := []string{}
	for _, d := range occurrenceStarts(a, calDay(from), calDay(to), skip) {
		out = append(out, d.Format("2006-01-02"))
	}
	return out
}

func eq(a, b []string) bool { return fmt.Sprint(a) == fmt.Sprint(b) }

func TestOccurrenceStarts(t *testing.T) {
	base := defaultActivity()
	base.StartDate, base.EndDate = "2026-11-02", "2026-11-02" // a Monday

	// none
	a := base
	if got := startsOf(a, "2026-11-01", "2026-11-30", nil); !eq(got, []string{"2026-11-02"}) {
		t.Errorf("none: %v", got)
	}
	if got := startsOf(a, "2026-11-03", "2026-11-30", nil); len(got) != 0 {
		t.Errorf("none outside range: %v", got)
	}

	// daily every 3 days, until the 10th, skipping the 8th
	a = base
	a.Repeat, a.Interval, a.UntilDate = "daily", 3, ptr("2026-11-10")
	if got := startsOf(a, "2026-11-01", "2026-11-30", map[string]bool{"2026-11-08": true}); !eq(got, []string{"2026-11-02", "2026-11-05"}) {
		t.Errorf("daily: %v", got)
	}

	// weekly on Mon+Wed every 2 weeks
	a = base
	a.Repeat, a.Interval, a.Weekdays = "weekly", 2, []int{1, 3}
	if got := startsOf(a, "2026-11-01", "2026-11-30", nil); !eq(got, []string{"2026-11-02", "2026-11-04", "2026-11-16", "2026-11-18", "2026-11-30"}) {
		t.Errorf("weekly/2: %v", got)
	}
	// window starting mid-series keeps the fortnightly rhythm
	if got := startsOf(a, "2026-11-10", "2026-11-20", nil); !eq(got, []string{"2026-11-16", "2026-11-18"}) {
		t.Errorf("weekly window: %v", got)
	}

	// weekly without weekdays = the start's weekday
	a = base
	a.Repeat = "weekly"
	if got := startsOf(a, "2026-11-01", "2026-11-22", nil); !eq(got, []string{"2026-11-02", "2026-11-09", "2026-11-16"}) {
		t.Errorf("weekly default: %v", got)
	}

	// monthly on the 31st clamps to month end
	a = base
	a.StartDate, a.EndDate, a.Repeat = "2026-01-31", "2026-01-31", "monthly"
	if got := startsOf(a, "2026-01-01", "2026-04-30", nil); !eq(got, []string{"2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30"}) {
		t.Errorf("monthly clamp: %v", got)
	}

	// yearly on Feb 29
	a = base
	a.StartDate, a.EndDate, a.Repeat = "2024-02-29", "2024-02-29", "yearly"
	if got := startsOf(a, "2024-01-01", "2026-12-31", nil); !eq(got, []string{"2024-02-29", "2025-02-28", "2026-02-28"}) {
		t.Errorf("yearly: %v", got)
	}

	// a 3-day trip starting before the window is still visible in it
	a = base
	a.StartDate, a.EndDate = "2026-11-08", "2026-11-10"
	if got := startsOf(a, "2026-11-09", "2026-11-15", nil); !eq(got, []string{"2026-11-08"}) {
		t.Errorf("multi-day overlap: %v", got)
	}
}

func ptr[T any](v T) *T { return &v }

func TestApplyActivityInput(t *testing.T) {
	bad := []activityInput{
		{}, // no title
		{Title: ptr("x"), StartDate: ptr("nope")},
		{Title: ptr("x"), StartDate: ptr("2026-11-02"), EndDate: ptr("2026-11-01")},
		{Title: ptr("x"), StartDate: ptr("2026-11-02"), StartTime: ptr("10:00"), EndTime: ptr("09:00")},
		{Title: ptr("x"), StartDate: ptr("2026-11-02"), Repeat: ptr("hourly")},
		{Title: ptr("x"), StartDate: ptr("2026-11-02"), Color: ptr("neon")},
		{Title: ptr("x"), StartDate: ptr("2026-11-02"), Repeat: ptr("weekly"), Weekdays: &[]int{7}},
		{Title: ptr("x"), StartDate: ptr("2026-11-02"), Repeat: ptr("daily"), UntilDate: ptr("2026-10-01")},
		{Title: ptr("x"), StartDate: ptr("2026-11-02"), EndDate: ptr("2027-03-02")},
	}
	for i, in := range bad {
		a := defaultActivity()
		if err := applyActivityInput(&a, in); err == nil {
			t.Errorf("case %d should be rejected", i)
		}
	}
	a := defaultActivity()
	err := applyActivityInput(&a, activityInput{Title: ptr(" Swim class "), StartDate: ptr("2026-11-02"), AllDay: ptr(true), Repeat: ptr("none"), UntilDate: ptr("2027-01-01")})
	if err != nil || a.Title != "Swim class" || a.EndDate != "2026-11-02" || a.StartTime != "00:00" || a.UntilDate != nil {
		t.Fatalf("all-day defaults: %v %+v", err, a)
	}
}

func TestCalendarAPI(t *testing.T) {
	_, srv := newTestServer(t)
	owner := newClient(t, srv)
	owner.signup()
	kidEmail, kidID := addMember(t, owner, "kid", nil)
	_, parentID := addMember(t, owner, "parent", nil)
	kid := newClient(t, srv)
	loginAs(t, kid, kidEmail, "temp-pass-123")

	if code := kid.do("POST", "/api/activities", map[string]any{"title": "Nope", "start_date": "2026-11-02"}, nil); code != 403 {
		t.Fatalf("kid create: %d", code)
	}
	var created activityRow
	if code := owner.do("POST", "/api/activities", map[string]any{
		"title": "Football practice", "start_date": "2026-11-02", "start_time": "16:00", "end_time": "17:30",
		"repeat": "weekly", "weekdays": []int{1, 3}, "attendee_ids": []int64{kidID}, "location": "Field", "color": "emerald",
	}, &created); code != 201 || created.ID == 0 {
		t.Fatalf("create: %d %+v", code, created)
	}
	var family activityRow
	owner.do("POST", "/api/activities", map[string]any{"title": "Family dinner", "start_date": "2026-11-05", "all_day": true}, &family)
	var private activityRow
	owner.do("POST", "/api/activities", map[string]any{"title": "Parents date", "start_date": "2026-11-06", "attendee_ids": []int64{parentID}}, &private)
	if code := owner.do("POST", "/api/activities", map[string]any{"title": "x", "start_date": "2026-11-02", "attendee_ids": []int64{999999999}}, nil); code != 400 {
		t.Fatalf("foreign attendee: %d", code)
	}

	var list struct {
		Items []activityOccurrence `json:"items"`
	}
	if code := owner.do("GET", "/api/activities?from=2026-11-01&to=2026-11-14", nil, &list); code != 200 {
		t.Fatalf("list: %d", code)
	}
	if len(list.Items) != 7 { // Mon/Wed twice (4) + dinner + date + ... = 4 football + 2 one-offs = 6
		// 2,4,9,11 football + dinner + date = 6
		if len(list.Items) != 6 {
			t.Fatalf("owner sees %d items: %+v", len(list.Items), list.Items)
		}
	}
	if list.Items[0].OccursOn != "2026-11-02" || list.Items[0].AttendeeNames[0] != "kid-member" {
		t.Fatalf("first item: %+v", list.Items[0])
	}

	// the kid sees the activity they attend and the whole-family one, not the parents' date
	var kidList struct {
		Items []activityOccurrence `json:"items"`
	}
	kid.do("GET", "/api/activities?from=2026-11-01&to=2026-11-14", nil, &kidList)
	for _, it := range kidList.Items {
		if it.Title == "Parents date" {
			t.Fatalf("kid saw a private activity")
		}
	}
	if len(kidList.Items) != 5 {
		t.Fatalf("kid sees %d", len(kidList.Items))
	}
	if code := kid.do("GET", fmt.Sprintf("/api/activities/%d", private.ID), nil, nil); code != 404 {
		t.Fatalf("kid get private: %d", code)
	}

	// delete a single occurrence, then edit the series
	if code := owner.do("DELETE", fmt.Sprintf("/api/activities/%d?on=2026-11-04", created.ID), nil, nil); code != 200 {
		t.Fatalf("skip: %d", code)
	}
	owner.do("GET", "/api/activities?from=2026-11-01&to=2026-11-07", nil, &list)
	n := 0
	for _, it := range list.Items {
		if it.ID == created.ID {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("after skip expected 1 football in week, got %d", n)
	}
	var upd activityRow
	if code := owner.do("PATCH", fmt.Sprintf("/api/activities/%d", created.ID), map[string]any{"title": "Football", "repeat": "none"}, &upd); code != 200 || upd.Title != "Football" || upd.Repeat != "none" {
		t.Fatalf("patch: %d %+v", code, upd)
	}
	if code := owner.do("PATCH", fmt.Sprintf("/api/activities/%d", created.ID), map[string]any{"start_time": "18:00", "end_time": "17:00"}, nil); code != 400 {
		t.Fatalf("bad times: %d", code)
	}
	if code := owner.do("GET", "/api/activities?from=2026-01-01&to=2026-12-31", nil, nil); code != 400 {
		t.Fatalf("range cap: %d", code)
	}

	// calendar overlay carries reminders too
	var rem struct {
		ID int64 `json:"id"`
	}
	owner.do("POST", "/api/reminders", map[string]any{"title": "Pay rent", "kind": "once", "once_at": "2026-11-20T02:00:00Z"}, &rem)
	var cal struct {
		Activities []activityOccurrence `json:"activities"`
		Reminders  []occurrenceRow      `json:"reminders"`
	}
	if code := owner.do("GET", "/api/calendar?from=2026-11-01&to=2026-11-30", nil, &cal); code != 200 {
		t.Fatalf("calendar: %d", code)
	}
	found := false
	for _, r := range cal.Reminders {
		found = found || r.Title == "Pay rent"
	}
	if !found || len(cal.Activities) == 0 {
		t.Fatalf("calendar overlay: reminders=%d activities=%d", len(cal.Reminders), len(cal.Activities))
	}
	var only struct {
		Activities []activityOccurrence `json:"activities"`
		Reminders  []occurrenceRow      `json:"reminders"`
	}
	owner.do("GET", "/api/calendar?from=2026-11-01&to=2026-11-30&include=reminders", nil, &only)
	if len(only.Activities) != 0 || len(only.Reminders) == 0 {
		t.Fatalf("include=reminders: %d/%d", len(only.Activities), len(only.Reminders))
	}
}
