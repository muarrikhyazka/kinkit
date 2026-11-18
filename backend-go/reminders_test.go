package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------- schedule math (pure) ----------

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func rd(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func utc(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func fmtTimes(ts []time.Time) string {
	var out []string
	for _, t := range ts {
		out = append(out, t.UTC().Format(time.RFC3339))
	}
	return strings.Join(out, " ")
}

func TestOccurrencesDailyJakarta(t *testing.T) {
	loc := mustLoc(t, "Asia/Jakarta") // UTC+7, no DST
	s := reminderSchedule{Kind: "daily", TimeOfDay: "08:00", StartDate: rd("2026-11-18")}
	got := occurrencesBetween(s, loc, utc("2026-11-17T00:00:00Z"), utc("2026-11-20T00:00:00Z"))
	want := "2026-11-18T01:00:00Z 2026-11-19T01:00:00Z"
	if fmtTimes(got) != want {
		t.Fatalf("got %s want %s", fmtTimes(got), want)
	}
}

func TestOccurrencesDSTNewYork(t *testing.T) {
	loc := mustLoc(t, "America/New_York")
	s := reminderSchedule{Kind: "daily", TimeOfDay: "08:00", StartDate: rd("2026-01-01")}
	// DST starts Sun 2026-03-08: 08:00 EST = 13:00Z before, 08:00 EDT = 12:00Z after.
	got := occurrencesBetween(s, loc, utc("2026-03-07T00:00:00Z"), utc("2026-03-10T00:00:00Z"))
	want := "2026-03-07T13:00:00Z 2026-03-08T12:00:00Z 2026-03-09T12:00:00Z"
	if fmtTimes(got) != want {
		t.Fatalf("spring: got %s want %s", fmtTimes(got), want)
	}
	// DST ends Sun 2026-11-01.
	got = occurrencesBetween(s, loc, utc("2026-10-31T00:00:00Z"), utc("2026-11-03T00:00:00Z"))
	want = "2026-10-31T12:00:00Z 2026-11-01T13:00:00Z 2026-11-02T13:00:00Z"
	if fmtTimes(got) != want {
		t.Fatalf("fall: got %s want %s", fmtTimes(got), want)
	}
	// A time inside the spring-forward gap still fires exactly once that day.
	gap := reminderSchedule{Kind: "daily", TimeOfDay: "02:30", StartDate: rd("2026-01-01")}
	got = occurrencesBetween(gap, loc, utc("2026-03-08T05:00:00Z"), utc("2026-03-09T05:00:00Z"))
	if len(got) != 1 {
		t.Fatalf("gap day: want 1 occurrence, got %s", fmtTimes(got))
	}
	// An ambiguous fall-back time fires once.
	amb := reminderSchedule{Kind: "daily", TimeOfDay: "01:30", StartDate: rd("2026-01-01")}
	got = occurrencesBetween(amb, loc, utc("2026-11-01T04:00:00Z"), utc("2026-11-02T05:00:00Z"))
	if len(got) != 1 {
		t.Fatalf("ambiguous day: want 1 occurrence, got %s", fmtTimes(got))
	}
}

func TestOccurrencesWeeklyBerlinDST(t *testing.T) {
	loc := mustLoc(t, "Europe/Berlin")
	// Mondays 07:00; DST in Berlin starts Sun 2026-03-29.
	s := reminderSchedule{Kind: "weekly", TimeOfDay: "07:00", Weekdays: []int{1}, StartDate: rd("2026-03-01")}
	got := occurrencesBetween(s, loc, utc("2026-03-20T00:00:00Z"), utc("2026-04-07T00:00:00Z"))
	want := "2026-03-23T06:00:00Z 2026-03-30T05:00:00Z 2026-04-06T05:00:00Z"
	if fmtTimes(got) != want {
		t.Fatalf("got %s want %s", fmtTimes(got), want)
	}
	// multiple weekdays, end date inclusive
	end := rd("2026-03-27")
	s2 := reminderSchedule{Kind: "weekly", TimeOfDay: "18:00", Weekdays: []int{1, 3, 5}, StartDate: rd("2026-03-23"), EndDate: &end}
	if got := occurrencesBetween(s2, loc, utc("2026-03-01T00:00:00Z"), utc("2026-05-01T00:00:00Z")); len(got) != 3 {
		t.Fatalf("mon/wed/fri in one week: got %s", fmtTimes(got))
	}
	// empty weekdays = start date's weekday (2026-03-25 is a Wednesday)
	s3 := reminderSchedule{Kind: "weekly", TimeOfDay: "09:00", StartDate: rd("2026-03-25")}
	got = occurrencesBetween(s3, time.UTC, utc("2026-03-01T00:00:00Z"), utc("2026-04-02T00:00:00Z"))
	if fmtTimes(got) != "2026-03-25T09:00:00Z 2026-04-01T09:00:00Z" {
		t.Fatalf("default weekday: %s", fmtTimes(got))
	}
}

func TestOccurrencesMonthEndAndYearly(t *testing.T) {
	s := reminderSchedule{Kind: "monthly", TimeOfDay: "09:00", DayOfMonth: 31, StartDate: rd("2026-01-01")}
	got := occurrencesBetween(s, time.UTC, utc("2026-01-01T00:00:00Z"), utc("2026-05-01T00:00:00Z"))
	want := "2026-01-31T09:00:00Z 2026-02-28T09:00:00Z 2026-03-31T09:00:00Z 2026-04-30T09:00:00Z"
	if fmtTimes(got) != want {
		t.Fatalf("monthly 31: got %s want %s", fmtTimes(got), want)
	}
	// leap-day birthday
	y := reminderSchedule{Kind: "yearly", TimeOfDay: "08:00", DayOfMonth: 29, StartDate: rd("2024-02-29")}
	got = occurrencesBetween(y, time.UTC, utc("2026-01-01T00:00:00Z"), utc("2029-01-01T00:00:00Z"))
	want = "2026-02-28T08:00:00Z 2027-02-28T08:00:00Z 2028-02-29T08:00:00Z"
	if fmtTimes(got) != want {
		t.Fatalf("yearly feb 29: got %s want %s", fmtTimes(got), want)
	}
	// start date bounds the first occurrence
	s2 := reminderSchedule{Kind: "monthly", TimeOfDay: "09:00", DayOfMonth: 5, StartDate: rd("2026-02-10")}
	got = occurrencesBetween(s2, time.UTC, utc("2026-01-01T00:00:00Z"), utc("2026-04-01T00:00:00Z"))
	if fmtTimes(got) != "2026-03-05T09:00:00Z" {
		t.Fatalf("start bound: %s", fmtTimes(got))
	}
}

func TestOccurrencesOnceAndValidate(t *testing.T) {
	at := utc("2026-11-19T01:00:00Z")
	s := reminderSchedule{Kind: "once", OnceAt: &at}
	if got := occurrencesBetween(s, time.UTC, utc("2026-11-18T00:00:00Z"), utc("2026-11-20T00:00:00Z")); len(got) != 1 {
		t.Fatalf("once in range: %v", got)
	}
	if got := occurrencesBetween(s, time.UTC, utc("2026-11-19T01:00:01Z"), utc("2026-11-20T00:00:00Z")); len(got) != 0 {
		t.Fatalf("once before range: %v", got)
	}
	bad := []reminderSchedule{
		{Kind: "hourly", TimeOfDay: "08:00"},
		{Kind: "daily", TimeOfDay: "25:00"},
		{Kind: "daily", TimeOfDay: "8"},
		{Kind: "weekly", TimeOfDay: "08:00", Weekdays: []int{7}},
		{Kind: "once"},
	}
	for _, b := range bad {
		if b.validate() == nil {
			t.Errorf("expected validation error for %+v", b)
		}
	}
}

func TestDescribeSchedule(t *testing.T) {
	cases := map[string]reminderSchedule{
		"Every Monday at 07:00":                       {Kind: "weekly", TimeOfDay: "07:00", Weekdays: []int{1}},
		"Every weekday at 06:30":                      {Kind: "weekly", TimeOfDay: "06:30", Weekdays: []int{5, 4, 3, 2, 1}},
		"Every Monday, Wednesday and Friday at 18:00": {Kind: "weekly", TimeOfDay: "18:00", Weekdays: []int{1, 3, 5}},
		"Every day at 08:00":                          {Kind: "daily", TimeOfDay: "08:00"},
		"Every month on the 1st at 09:00":             {Kind: "monthly", TimeOfDay: "09:00", DayOfMonth: 1},
		"Every month on the 22nd at 09:00":            {Kind: "monthly", TimeOfDay: "09:00", DayOfMonth: 22},
		"Every year on Mar 14 at 08:00":               {Kind: "yearly", TimeOfDay: "08:00", StartDate: rd("2026-03-14")},
	}
	for want, s := range cases {
		if got := describeSchedule(s, time.UTC); got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}

func TestSnoozeTomorrowAndDayBounds(t *testing.T) {
	loc := mustLoc(t, "Asia/Jakarta")
	due := utc("2026-11-18T01:00:00Z") // 08:00 WIB
	now := utc("2026-11-18T03:00:00Z")
	if got := sameLocalTimeNextDay(due, now, loc); !got.Equal(utc("2026-11-19T01:00:00Z")) {
		t.Fatalf("snooze tomorrow: %v", got)
	}
	ny := mustLoc(t, "America/New_York")
	s, e := localDayBounds(utc("2026-03-08T15:00:00Z"), ny)
	if e.Sub(s) != 23*time.Hour {
		t.Fatalf("spring-forward day should be 23h, got %v", e.Sub(s))
	}
}

// ---------- natural-language parser (pure) ----------

func TestParseReminderText(t *testing.T) {
	loc := mustLoc(t, "Asia/Jakarta")
	now := utc("2026-11-18T03:00:00Z") // Wed 18 Nov 2026, 10:00 WIB
	type want struct {
		kind, title, assignee, when string // when: once -> RFC3339 UTC; else describeSchedule
	}
	cases := map[string]want{
		"tomorrow 8am take medicine":           {"once", "take medicine", "", "2026-11-19T01:00:00Z"},
		"tomorrow at 8:30 pm to call mom":      {"once", "call mom", "", "2026-11-19T13:30:00Z"},
		"8am tomorrow vitamins":                {"once", "vitamins", "", "2026-11-19T01:00:00Z"},
		"today 18:00 pick up kids @budi":       {"once", "pick up kids", "budi", "2026-11-18T11:00:00Z"},
		"tonight put the bins out":             {"once", "put the bins out", "", "2026-11-18T13:00:00Z"},
		"9pm lock the door":                    {"once", "lock the door", "", "2026-11-18T14:00:00Z"},
		"8am water plants":                     {"once", "water plants", "", "2026-11-19T01:00:00Z"}, // 8am passed -> tomorrow
		"friday 9am dentist":                   {"once", "dentist", "", "2026-11-20T02:00:00Z"},
		"wednesday 9am too late":               {"once", "too late", "", "2026-11-25T02:00:00Z"}, // same weekday, passed
		"next monday 7:00 dentist":             {"once", "dentist", "", "2026-11-23T00:00:00Z"},
		"on 2026-12-01 10:00 renew passport":   {"once", "renew passport", "", "2026-12-01T03:00:00Z"},
		"dec 24 9:00 wrap presents":            {"once", "wrap presents", "", "2026-12-24T02:00:00Z"},
		"in 2 hours call grandma":              {"once", "call grandma", "", "2026-11-18T05:00:00Z"},
		"in 30 min check the oven":             {"once", "check the oven", "", "2026-11-18T03:30:00Z"},
		"in an hour stretch":                   {"once", "stretch", "", "2026-11-18T04:00:00Z"},
		"every monday 7:00 trash":              {"weekly", "trash", "", "Every Monday at 07:00"},
		"every mon, wed and fri 6pm gym @Sari": {"weekly", "gym", "Sari", "Every Monday, Wednesday and Friday at 18:00"},
		"every weekday at 6:30 school run":     {"weekly", "school run", "", "Every weekday at 06:30"},
		"every day 7am vitamins":               {"daily", "vitamins", "", "Every day at 07:00"},
		"daily 21:00 brush teeth":              {"daily", "brush teeth", "", "Every day at 21:00"},
		"every month on the 5th pay rent":      {"monthly", "pay rent", "", "Every month on the 5th at 09:00"},
		"every month on the 25th 10am salary":  {"monthly", "salary", "", "Every month on the 25th at 10:00"},
		"every year on mar 14 mom's birthday":  {"yearly", "mom's birthday", "", "Every year on Mar 14 at 09:00"},
	}
	for text, w := range cases {
		p, err := parseReminderText(text, now, loc)
		if err != nil {
			t.Errorf("%q: unexpected error %v", text, err)
			continue
		}
		var when string
		if p.Schedule.Kind == "once" {
			when = p.Schedule.OnceAt.UTC().Format(time.RFC3339)
		} else {
			when = describeSchedule(p.Schedule, loc)
		}
		if p.Schedule.Kind != w.kind || p.Title != w.title || p.Assignee != w.assignee || when != w.when {
			t.Errorf("%q: got kind=%s title=%q assignee=%q when=%s; want %+v", text, p.Schedule.Kind, p.Title, p.Assignee, when, w)
		}
		if err := p.Schedule.validate(); err != nil {
			t.Errorf("%q: schedule invalid: %v", text, err)
		}
	}
	for _, bad := range []string{"", "take medicine", "tomorrow 8am", "in 2 lightyears do x", "every blursday 7:00 x", "today 7am too early"} {
		if _, err := parseReminderText(bad, now, loc); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

// ---------- digest text (pure) ----------

func TestBuildDigestText(t *testing.T) {
	day := rd("2026-11-19")
	if got := buildDigestText("Good morning", day, nil); !strings.Contains(got, "Nothing scheduled") || !strings.Contains(got, "Thursday, 19 Nov") {
		t.Fatalf("empty digest: %q", got)
	}
	var items []TodayItem
	for i := 0; i < 8; i++ {
		items = append(items, TodayItem{Kind: "chore", Title: "Chore " + string(rune('A'+i))})
	}
	secs := []TodaySection{
		{Key: "reminders", Title: "Reminders", Items: []TodayItem{
			{Kind: "reminder", Title: "Take medicine", Detail: "08:00", MemberName: "Budi"},
			{Kind: "reminder", Title: "Water plants", Detail: "07:00", Done: true},
		}},
		{Key: "chores", Title: "Chores", Items: items},
		{Key: "budget", Title: "Budget", Summary: "Spent Rp1.000 of Rp2.000 this month (50%)", Items: []TodayItem{
			{Kind: "budget", Title: "Food is over budget", Detail: "Rp600 of Rp500", Overdue: true},
		}},
	}
	got := buildDigestText("Good morning", day, secs)
	for _, want := range []string{
		"Good morning! Thursday, 19 Nov",
		"Reminders\n• Take medicine (08:00, Budi)\n[x] Water plants (07:00)",
		"Chores\n• Chore A",
		"…and 2 more",
		"Budget: Spent Rp1.000 of Rp2.000 this month (50%)",
		"! Food is over budget (Rp600 of Rp500)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("digest missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Chore G") {
		t.Errorf("digest should cap items per section:\n%s", got)
	}
}

func TestDigestDue(t *testing.T) {
	loc := mustLoc(t, "Europe/Berlin")
	at := func(s string) time.Time { tt, _ := time.ParseInLocation("2006-01-02 15:04", s, loc); return tt }
	if digestDue("07:00", at("2026-11-19 06:59")) {
		t.Error("before time")
	}
	if !digestDue("07:00", at("2026-11-19 07:00")) || !digestDue("07:00", at("2026-11-19 09:30")) {
		t.Error("should be due")
	}
	if digestDue("07:00", at("2026-11-19 10:30")) {
		t.Error("too late")
	}
}

// ---------- chat sender (HTTP stubbed) ----------

func TestSendChatMessageWithButtons(t *testing.T) {
	var got []map[string]any
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		got = append(got, m)
		paths = append(paths, r.URL.Path+" "+r.Header.Get("Authorization"))
		w.WriteHeader(200)
	}))
	defer srv.Close()
	oldT, oldD := telegramAPIBase, discordAPIBase
	telegramAPIBase, discordAPIBase = srv.URL, srv.URL
	defer func() { telegramAPIBase, discordAPIBase = oldT, oldD }()

	msg := chatMessage{Text: "Reminder: Pay <rent>", Mention: "42", MentionName: "Budi", Buttons: reminderButtons(7)}
	if err := sendChatMessageWithButtons(context.Background(), "telegram", "TKN", "100", msg); err != nil {
		t.Fatal(err)
	}
	if err := sendChatMessageWithButtons(context.Background(), "discord", "DTKN", "200", msg); err != nil {
		t.Fatal(err)
	}
	if paths[0] != "/botTKN/sendMessage " || paths[1] != "/channels/200/messages Bot DTKN" {
		t.Fatalf("paths: %v", paths)
	}
	tg := got[0]
	if tg["parse_mode"] != "HTML" || !strings.Contains(tg["text"].(string), `tg://user?id=42`) || !strings.Contains(tg["text"].(string), "&lt;rent&gt;") {
		t.Fatalf("telegram body: %v", tg)
	}
	kb := tg["reply_markup"].(map[string]any)["inline_keyboard"].([]any)[0].([]any)
	if len(kb) != 3 || kb[0].(map[string]any)["callback_data"] != "rem:7:done" || kb[2].(map[string]any)["callback_data"] != "rem:7:snooze_tomorrow" {
		t.Fatalf("telegram keyboard: %v", kb)
	}
	dc := got[1]
	if !strings.HasPrefix(dc["content"].(string), "<@42> ") {
		t.Fatalf("discord content: %v", dc["content"])
	}
	comps := dc["components"].([]any)[0].(map[string]any)["components"].([]any)
	if len(comps) != 3 || comps[1].(map[string]any)["custom_id"] != "rem:7:snooze60" {
		t.Fatalf("discord components: %v", comps)
	}
	// errors never leak the token
	telegramAPIBase = "http://127.0.0.1:1"
	if err := sendChatMessageWithButtons(context.Background(), "telegram", "SECRET", "1", chatMessage{Text: "x"}); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("expected token-free error, got %v", err)
	}
}

func TestKidRuleReminders(t *testing.T) {
	cases := []struct {
		method, path string
		want         roleDecision
	}{
		{"POST", "/api/reminders/occurrences/5/done", roleAllow},
		{"POST", "/api/reminders/occurrences/5/snooze", roleAllow},
		{"POST", "/api/reminders/link-code", roleAllow},
		{"DELETE", "/api/reminders/chat-link/telegram", roleAllow},
		{"POST", "/api/reminders", roleDeny},
		{"PATCH", "/api/reminders/5", roleDeny},
		{"PATCH", "/api/household/timezone", roleDeny},
	}
	for _, c := range cases {
		if got, _ := decideRole("kid", c.method, c.path); got != c.want {
			t.Errorf("kid %s %s: got %v want %v", c.method, c.path, got, c.want)
		}
	}
	if got, _ := decideRole("viewer", "POST", "/api/reminders/occurrences/5/done"); got != roleDeny {
		t.Error("viewer must not complete reminders")
	}
}
