package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Integration tests for reminders / today / digest / chat linking (skip
// without TEST_DATABASE_URL).

type sentChat struct {
	platform, chatID string
	msg              chatMessage
}

// stubChat replaces the reminder chat sender; fail makes every send error.
func stubChat(t *testing.T) (*[]sentChat, *bool) {
	var mu sync.Mutex
	var sent []sentChat
	fail := false
	old := sendReminderChat
	sendReminderChat = func(_ context.Context, platform, _ string, chatID string, msg chatMessage) error {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			return errors.New("stub failure")
		}
		sent = append(sent, sentChat{platform, chatID, msg})
		return nil
	}
	t.Cleanup(func() { sendReminderChat = old })
	return &sent, &fail
}

func sentTo(all []sentChat, chatID string) []sentChat {
	var out []sentChat
	for _, s := range all {
		if s.chatID == chatID {
			out = append(out, s)
		}
	}
	return out
}

// remHousehold: owner client with a Telegram integration (stubbed delivery).
type remHousehold struct {
	app      *App
	srv      *httptest.Server
	owner    *testClient
	aid      int64
	ownerMID int64
	botToken string
	chatID   string
}

func newRemHousehold(t *testing.T, tz string) *remHousehold {
	app, srv := newTestServer(t)
	c := newClient(t, srv)
	_, aid := c.signup()
	h := &remHousehold{app: app, srv: srv, owner: c, aid: aid, chatID: fmt.Sprintf("chat-%d", aid)}
	if err := app.DB.QueryRow(context.Background(), "SELECT id FROM members WHERE account_id=$1 AND role='owner'", aid).Scan(&h.ownerMID); err != nil {
		t.Fatal(err)
	}
	var res map[string]any
	if code := c.do("POST", "/api/integrations", map[string]any{"platform": "telegram", "platform_token": "tg-secret-token", "chat_id": h.chatID}, &res); code != 200 {
		t.Fatalf("integration: %d %v", code, res)
	}
	h.botToken = res["bot_token"].(string)
	if tz != "" {
		if code := c.do("PATCH", "/api/household/timezone", map[string]any{"timezone": tz}, &res); code != 200 {
			t.Fatalf("set tz: %d %v", code, res)
		}
	}
	return h
}

func (h *remHousehold) bot(t *testing.T, user, method, path string, body any, out any) int {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, h.srv.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bot-Token", h.botToken)
	if user != "" {
		req.Header.Set("X-Bot-User", user)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("%s %s: decode %q: %v", method, path, raw, err)
		}
	}
	return resp.StatusCode
}

func (h *remHousehold) createReminder(t *testing.T, c *testClient, body map[string]any) int64 {
	t.Helper()
	var res map[string]any
	if code := c.do("POST", "/api/reminders", body, &res); code != 200 {
		t.Fatalf("create reminder: %d %v", code, res)
	}
	return int64(res["id"].(float64))
}

// occFor returns the (first) occurrence id + status of a reminder.
func (h *remHousehold) occFor(t *testing.T, rid int64) (int64, string) {
	t.Helper()
	var id int64
	var st string
	if err := h.app.DB.QueryRow(context.Background(), "SELECT id, status FROM reminder_occurrences WHERE reminder_id=$1 ORDER BY due_at LIMIT 1", rid).Scan(&id, &st); err != nil {
		t.Fatalf("occurrence for reminder %d: %v", rid, err)
	}
	return id, st
}

func (h *remHousehold) status(t *testing.T, occID int64) string {
	var st string
	if err := h.app.DB.QueryRow(context.Background(), "SELECT status FROM reminder_occurrences WHERE id=$1", occID).Scan(&st); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestRemindersCRUDAndUpcoming(t *testing.T) {
	h := newRemHousehold(t, "Asia/Jakarta")
	c := h.owner
	var res map[string]any
	// validation
	if code := c.do("POST", "/api/reminders", map[string]any{"title": "", "kind": "daily"}, &res); code != 400 {
		t.Fatalf("empty title: %d", code)
	}
	if code := c.do("POST", "/api/reminders", map[string]any{"title": "x", "kind": "hourly"}, &res); code != 400 {
		t.Fatalf("bad kind: %d", code)
	}
	if code := c.do("POST", "/api/reminders", map[string]any{"title": "x", "kind": "daily", "category": "nope"}, &res); code != 400 {
		t.Fatalf("bad category: %d", code)
	}
	rid := h.createReminder(t, c, map[string]any{"title": "Vitamins", "kind": "daily", "time_of_day": "07:30", "category": "medicine"})
	h.createReminder(t, c, map[string]any{"title": "Trash", "kind": "weekly", "time_of_day": "06:00", "weekdays": []int{1, 4}, "assignee_id": h.ownerMID})

	var list struct {
		Timezone  string        `json:"timezone"`
		Reminders []reminderRow `json:"reminders"`
	}
	if code := c.do("GET", "/api/reminders", nil, &list); code != 200 || len(list.Reminders) != 2 || list.Timezone != "Asia/Jakarta" {
		t.Fatalf("list: %d %+v", code, list)
	}
	for _, r := range list.Reminders {
		if r.Title == "Trash" && (r.Description != "Every Monday and Thursday at 06:00" || r.AssigneeName == "" || r.NextDue == nil) {
			t.Fatalf("trash row: %+v", r)
		}
	}
	// materialized: daily -> ~8 occurrences in the 8 day horizon
	var n int
	_ = h.app.DB.QueryRow(context.Background(), "SELECT count(*) FROM reminder_occurrences WHERE reminder_id=$1", rid).Scan(&n)
	if n < 7 || n > 9 {
		t.Fatalf("expected ~8 materialized daily occurrences, got %d", n)
	}
	// re-materializing is idempotent
	if added, err := h.app.materialize(context.Background(), h.aid, time.Now().UTC(), materializeHorizon); err != nil || added != 0 {
		t.Fatalf("re-materialize added %d (%v)", added, err)
	}
	var up struct {
		Items []occurrenceRow `json:"items"`
	}
	if code := c.do("GET", "/api/reminders/upcoming?days=7", nil, &up); code != 200 || len(up.Items) < 7 {
		t.Fatalf("upcoming: %d %d", code, len(up.Items))
	}
	// schedule change: future pending occurrences regenerate at the new time
	if code := c.do("PATCH", fmt.Sprintf("/api/reminders/%d", rid), map[string]any{"time_of_day": "21:00"}, &res); code != 200 {
		t.Fatalf("patch: %d %v", code, res)
	}
	var bad int
	_ = h.app.DB.QueryRow(context.Background(), `SELECT count(*) FROM reminder_occurrences WHERE reminder_id=$1 AND status='pending'
	  AND to_char(due_at AT TIME ZONE 'Asia/Jakarta','HH24:MI') <> '21:00'`, rid).Scan(&bad)
	if bad != 0 {
		t.Fatalf("%d pending occurrences still at the old time", bad)
	}
	// pause removes future pending occurrences; title edit keeps the rest
	if code := c.do("PATCH", fmt.Sprintf("/api/reminders/%d", rid), map[string]any{"active": false}, &res); code != 200 {
		t.Fatalf("pause: %d", code)
	}
	_ = h.app.DB.QueryRow(context.Background(), "SELECT count(*) FROM reminder_occurrences WHERE reminder_id=$1 AND status='pending' AND due_at > now()", rid).Scan(&n)
	if n != 0 {
		t.Fatalf("paused reminder still has %d pending occurrences", n)
	}
	row, _ := h.app.loadReminder(context.Background(), h.aid, rid)
	if row.Active || row.TimeOfDay != "21:00" || row.Category != "medicine" {
		t.Fatalf("patch semantics: %+v", row)
	}
	// once: local date + time -> UTC instant
	tomorrow := time.Now().In(mustLoc(t, "Asia/Jakarta")).AddDate(0, 0, 1).Format("2006-01-02")
	oid := h.createReminder(t, c, map[string]any{"title": "Dentist", "kind": "once", "start_date": tomorrow, "time_of_day": "10:15"})
	o, _ := h.app.loadReminder(context.Background(), h.aid, oid)
	if o.OnceAt == nil || o.OnceAt.In(mustLoc(t, "Asia/Jakarta")).Format("2006-01-02 15:04") != tomorrow+" 10:15" {
		t.Fatalf("once_at: %v", o.OnceAt)
	}
	if code := c.do("DELETE", fmt.Sprintf("/api/reminders/%d", rid), nil, &res); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	if code := c.do("DELETE", fmt.Sprintf("/api/reminders/%d", rid), nil, &res); code != 404 {
		t.Fatalf("delete again: %d", code)
	}
}

func TestRemindersSendIdempotentAndEscalate(t *testing.T) {
	sent, fail := stubChat(t)
	h := newRemHousehold(t, "Europe/Berlin")
	ctx := context.Background()
	// link the owner's telegram id so the escalation mentions them
	if _, err := h.app.DB.Exec(ctx, "UPDATE members SET telegram_user_id='777' WHERE id=$1", h.ownerMID); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(10 * time.Minute).Truncate(time.Minute)
	rid := h.createReminder(t, h.owner, map[string]any{"title": "Take pills", "kind": "once", "once_at": at.Format(time.RFC3339),
		"escalate_to": h.ownerMID, "escalate_after_minutes": 15, "category": "medicine"})
	occ, st := h.occFor(t, rid)
	if st != "pending" {
		t.Fatalf("status %s", st)
	}
	// not due yet
	if _, err := h.app.sendDueReminders(ctx, at.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(sentTo(*sent, h.chatID)) != 0 {
		t.Fatal("sent too early")
	}
	// all sends fail -> claim released, still pending
	*fail = true
	if _, err := h.app.sendDueReminders(ctx, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if s := h.status(t, occ); s != "pending" {
		t.Fatalf("after failed send status=%s, want pending", s)
	}
	*fail = false
	for i := 0; i < 2; i++ { // twice: idempotent
		if _, err := h.app.sendDueReminders(ctx, at.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	msgs := sentTo(*sent, h.chatID)
	if len(msgs) != 1 || !strings.Contains(msgs[0].msg.Text, "Take pills") || msgs[0].msg.Buttons[0][0].Data != fmt.Sprintf("rem:%d:done", occ) {
		t.Fatalf("sent: %+v", msgs)
	}
	if h.status(t, occ) != "sent" {
		t.Fatal("not marked sent")
	}
	// escalation: sent_at = at+1m, after 15 min
	if _, err := h.app.escalateReminders(ctx, at.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(sentTo(*sent, h.chatID)) != 1 {
		t.Fatal("escalated too early")
	}
	for i := 0; i < 2; i++ {
		if _, err := h.app.escalateReminders(ctx, at.Add(20*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	msgs = sentTo(*sent, h.chatID)
	if len(msgs) != 2 || !strings.Contains(msgs[1].msg.Text, "Not done yet") || msgs[1].msg.Mention != "777" {
		t.Fatalf("escalation: %+v", msgs)
	}
	// snooze (web) -> resend after the snooze ends
	var res map[string]any
	if code := h.owner.do("POST", fmt.Sprintf("/api/reminders/occurrences/%d/snooze", occ), map[string]any{"minutes": 60}, &res); code != 200 {
		t.Fatalf("snooze: %d %v", code, res)
	}
	if h.status(t, occ) != "snoozed" {
		t.Fatal("not snoozed")
	}
	if _, err := h.app.sendDueReminders(ctx, time.Now().UTC().Add(61*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(sentTo(*sent, h.chatID)) != 3 || h.status(t, occ) != "sent" {
		t.Fatalf("resend after snooze: %d msgs, status %s", len(sentTo(*sent, h.chatID)), h.status(t, occ))
	}
	// done; snoozing a done occurrence fails
	if code := h.owner.do("POST", fmt.Sprintf("/api/reminders/occurrences/%d/done", occ), nil, &res); code != 200 {
		t.Fatalf("done: %d %v", code, res)
	}
	var doneBy *int64
	_ = h.app.DB.QueryRow(ctx, "SELECT done_by FROM reminder_occurrences WHERE id=$1", occ).Scan(&doneBy)
	if h.status(t, occ) != "done" || doneBy == nil || *doneBy != h.ownerMID {
		t.Fatal("done not recorded")
	}
	if code := h.owner.do("POST", fmt.Sprintf("/api/reminders/occurrences/%d/snooze", occ), map[string]any{"minutes": 10}, &res); code != 400 {
		t.Fatalf("snooze done: %d", code)
	}
}

func TestRemindersMissedAtEndOfDay(t *testing.T) {
	stubChat(t)
	h := newRemHousehold(t, "America/New_York")
	ctx := context.Background()
	at := time.Now().UTC().Add(5 * time.Minute).Truncate(time.Minute)
	rid := h.createReminder(t, h.owner, map[string]any{"title": "Call school", "kind": "once", "once_at": at.Format(time.RFC3339)})
	occ, _ := h.occFor(t, rid)
	if _, err := h.app.markMissed(ctx, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if h.status(t, occ) != "pending" {
		t.Fatal("missed before the day ended")
	}
	_, dayEnd := localDayBounds(at, mustLoc(t, "America/New_York"))
	if _, err := h.app.markMissed(ctx, dayEnd.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if h.status(t, occ) != "missed" {
		t.Fatalf("status %s, want missed", h.status(t, occ))
	}
}

func TestRemindersKidAndViewerPermissions(t *testing.T) {
	stubChat(t)
	h := newRemHousehold(t, "")
	kidEmail, kidID := addMember(t, h.owner, "kid", nil)
	viewerEmail, _ := addMember(t, h.owner, "viewer", nil)
	at := time.Now().UTC().Add(30 * time.Minute).Format(time.RFC3339)
	mine := h.createReminder(t, h.owner, map[string]any{"title": "Homework", "kind": "once", "once_at": at, "assignee_id": kidID})
	family := h.createReminder(t, h.owner, map[string]any{"title": "Feed the cat", "kind": "once", "once_at": at})
	parents := h.createReminder(t, h.owner, map[string]any{"title": "Pay rent", "kind": "once", "once_at": at, "assignee_id": h.ownerMID})
	mineOcc, _ := h.occFor(t, mine)
	famOcc, _ := h.occFor(t, family)
	parOcc, _ := h.occFor(t, parents)

	kid := newClient(t, h.srv)
	loginAs(t, kid, kidEmail, "temp-pass-123")
	var res map[string]any
	if code := kid.do("POST", "/api/reminders", map[string]any{"title": "x", "kind": "daily"}, &res); code != 403 {
		t.Fatalf("kid create: %d", code)
	}
	if code := kid.do("POST", fmt.Sprintf("/api/reminders/occurrences/%d/done", parOcc), nil, &res); code != 403 {
		t.Fatalf("kid done on parent's: %d", code)
	}
	if code := kid.do("POST", fmt.Sprintf("/api/reminders/occurrences/%d/done", mineOcc), nil, &res); code != 200 {
		t.Fatalf("kid done own: %d %v", code, res)
	}
	if code := kid.do("POST", fmt.Sprintf("/api/reminders/occurrences/%d/snooze", famOcc), map[string]any{"until": "tomorrow"}, &res); code != 200 {
		t.Fatalf("kid snooze family: %d %v", code, res)
	}
	if code := kid.do("PATCH", "/api/household/timezone", map[string]any{"timezone": "Asia/Tokyo"}, &res); code != 403 {
		t.Fatalf("kid tz: %d", code)
	}
	// kid sees own + family only
	var list struct {
		Reminders []reminderRow `json:"reminders"`
	}
	kid.do("GET", "/api/reminders", nil, &list)
	if len(list.Reminders) != 2 {
		t.Fatalf("kid list: %+v", list.Reminders)
	}
	var code int
	if code = kid.do("POST", "/api/reminders/link-code", nil, &res); code != 200 || len(res["code"].(string)) != 6 {
		t.Fatalf("kid link code: %d %v", code, res)
	}

	viewer := newClient(t, h.srv)
	loginAs(t, viewer, viewerEmail, "temp-pass-123")
	if code := viewer.do("POST", fmt.Sprintf("/api/reminders/occurrences/%d/done", famOcc), nil, &res); code != 403 {
		t.Fatalf("viewer done: %d", code)
	}
	if code := viewer.do("GET", "/api/reminders", nil, &list); code != 200 || len(list.Reminders) != 3 {
		t.Fatalf("viewer list: %d %d", code, len(list.Reminders))
	}
}

func TestChatLinkAndBotFlow(t *testing.T) {
	stubChat(t)
	h := newRemHousehold(t, "Asia/Jakarta")
	kidEmail, kidID := addMember(t, h.owner, "kid", nil)
	var res map[string]any
	if code := h.owner.do("POST", "/api/reminders/link-code", nil, &res); code != 200 {
		t.Fatalf("link code: %d %v", code, res)
	}
	code := res["code"].(string)
	// missing X-Bot-User
	if c := h.bot(t, "", "POST", "/api/bot/link", map[string]any{"code": code}, &res); c != 400 {
		t.Fatalf("no actor: %d", c)
	}
	// wrong code
	h.bot(t, "telegram:555", "POST", "/api/bot/link", map[string]any{"code": "ZZZZZZ"}, &res)
	if res["ok"] != false {
		t.Fatalf("bad code accepted: %v", res)
	}
	h.bot(t, "telegram:555", "POST", "/api/bot/link", map[string]any{"code": strings.ToLower(code)}, &res)
	if res["ok"] != true {
		t.Fatalf("link: %v", res)
	}
	var tg *string
	_ = h.app.DB.QueryRow(context.Background(), "SELECT telegram_user_id FROM members WHERE id=$1", h.ownerMID).Scan(&tg)
	if tg == nil || *tg != "555" {
		t.Fatalf("telegram id not stored: %v", tg)
	}
	h.bot(t, "telegram:555", "POST", "/api/bot/link", map[string]any{"code": code}, &res)
	if res["ok"] != false {
		t.Fatal("code reused")
	}
	// expired code
	kid := newClient(t, h.srv)
	loginAs(t, kid, kidEmail, "temp-pass-123")
	kid.do("POST", "/api/reminders/link-code", nil, &res)
	kcode := res["code"].(string)
	_, _ = h.app.DB.Exec(context.Background(), "UPDATE chat_link_codes SET expires_at=now()-interval '1 minute' WHERE member_id=$1", kidID)
	h.bot(t, "telegram:666", "POST", "/api/bot/link", map[string]any{"code": kcode}, &res)
	if res["ok"] != false {
		t.Fatal("expired code accepted")
	}
	var st map[string]any
	h.owner.do("GET", "/api/reminders/chat-link", nil, &st)
	if st["telegram_linked"] != true {
		t.Fatalf("status: %v", st)
	}

	// /remind via bot, as the linked owner, assigned to the kid by name
	if c := h.bot(t, "telegram:555", "POST", "/api/bot/reminders", map[string]any{"text": "tomorrow 8am take medicine @kid-member"}, &res); c != 200 || res["ok"] != true {
		t.Fatalf("bot remind: %d %v", c, res)
	}
	rid := int64(res["id"].(float64))
	row, _ := h.app.loadReminder(context.Background(), h.aid, rid)
	if row.AssigneeID == nil || *row.AssigneeID != kidID || row.CreatedBy == nil || *row.CreatedBy != h.ownerMID || row.Category != "medicine" {
		t.Fatalf("bot reminder row: %+v", row)
	}
	if row.OnceAt.In(mustLoc(t, "Asia/Jakarta")).Format("15:04") != "08:00" {
		t.Fatalf("once_at %v", row.OnceAt)
	}
	h.bot(t, "telegram:555", "POST", "/api/bot/reminders", map[string]any{"text": "blah"}, &res)
	if res["ok"] != false {
		t.Fatalf("unparseable accepted: %v", res)
	}
	h.bot(t, "telegram:555", "POST", "/api/bot/reminders", map[string]any{"text": "tomorrow 9am x @nobody"}, &res)
	if res["ok"] != false || !strings.Contains(res["message"].(string), "nobody") {
		t.Fatalf("unknown assignee: %v", res)
	}
	// /reminders and /today
	h.bot(t, "telegram:555", "GET", "/api/bot/reminders", nil, &res)
	if !strings.Contains(res["text"].(string), "take medicine") {
		t.Fatalf("bot list: %v", res)
	}
	h.bot(t, "telegram:999", "GET", "/api/bot/today", nil, &res)
	if !strings.Contains(res["text"].(string), "☀️") || res["timezone"] != "Asia/Jakarta" {
		t.Fatalf("bot today: %v", res)
	}
	// button callbacks
	occ, _ := h.occFor(t, rid)
	h.bot(t, "telegram:555", "POST", fmt.Sprintf("/api/bot/reminders/occurrences/%d/snooze", occ), map[string]any{"minutes": 60}, &res)
	if res["ok"] != true || h.status(t, occ) != "snoozed" {
		t.Fatalf("bot snooze: %v", res)
	}
	h.bot(t, "telegram:555", "POST", fmt.Sprintf("/api/bot/reminders/occurrences/%d/done", occ), nil, &res)
	if res["ok"] != true || h.status(t, occ) != "done" || !strings.Contains(res["message"].(string), "Done") {
		t.Fatalf("bot done: %v", res)
	}
	// bad bot token
	h.botToken = "nope"
	if c := h.bot(t, "telegram:555", "GET", "/api/bot/today", nil, &res); c != 403 {
		t.Fatalf("bad token: %d", c)
	}
}

// failing/panicking provider for one account only (registered once).
var todayTestFailAID int64

func init() {
	registerTodayProvider("test_failing", 5, func(ctx context.Context, app *App, aid int64, m memberInfo, day time.Time) (*TodaySection, error) {
		if aid != todayTestFailAID || aid == 0 {
			return nil, nil
		}
		return nil, errors.New("boom")
	})
	registerTodayProvider("test_panicking", 6, func(ctx context.Context, app *App, aid int64, m memberInfo, day time.Time) (*TodaySection, error) {
		if aid == todayTestFailAID && aid != 0 {
			panic("kaboom")
		}
		return nil, nil
	})
}

func TestTodayEndpointAndDigest(t *testing.T) {
	sent, _ := stubChat(t)
	h := newRemHousehold(t, "Asia/Jakarta")
	todayTestFailAID = h.aid
	t.Cleanup(func() { todayTestFailAID = 0 })
	ctx := context.Background()
	loc := mustLoc(t, "Asia/Jakarta")
	// a reminder later today (or right after midnight-safe: 1 minute ahead)
	now := time.Now().UTC()
	at := now.Add(2 * time.Minute).Truncate(time.Minute)
	if at.In(loc).Day() != now.In(loc).Day() {
		t.Skip("too close to local midnight")
	}
	h.createReminder(t, h.owner, map[string]any{"title": "Water plants", "kind": "once", "once_at": at.Format(time.RFC3339)})
	h.createReminder(t, h.owner, map[string]any{"title": "Grandma", "kind": "yearly", "category": "birthday",
		"start_date": now.In(loc).AddDate(0, 0, 10).Format("2006-01-02"), "time_of_day": "09:00"})

	var res struct {
		Date     string         `json:"date"`
		Timezone string         `json:"timezone"`
		Greeting string         `json:"greeting"`
		Sections []TodaySection `json:"sections"`
	}
	if code := h.owner.do("GET", "/api/today", nil, &res); code != 200 {
		t.Fatalf("today: %d", code)
	}
	if res.Timezone != "Asia/Jakarta" || res.Date != now.In(loc).Format("2006-01-02") || res.Greeting == "" {
		t.Fatalf("today header: %+v", res)
	}
	keys := map[string]TodaySection{}
	for _, s := range res.Sections {
		keys[s.Key] = s
	}
	if r, ok := keys["reminders"]; !ok || len(r.Items) != 1 || r.Items[0].Title != "Water plants" || len(r.Items[0].Actions) != 2 {
		t.Fatalf("reminders section: %+v", res.Sections)
	}
	if c, ok := keys["coming_up"]; !ok || !strings.Contains(c.Items[0].Title, "Grandma") {
		t.Fatalf("coming up: %+v", res.Sections)
	}
	if _, ok := keys["test_failing"]; ok {
		t.Fatal("failing provider leaked into sections")
	}

	// digest: due at local time_of_day, sent once per local day
	hhmm := now.In(loc).Format("15:04")
	if code := h.owner.do("PATCH", "/api/household/digest", map[string]any{"enabled": true, "time_of_day": hhmm, "channel": "all"}, nil); code != 200 {
		t.Fatalf("digest settings: %d", code)
	}
	if code := h.owner.do("PATCH", "/api/household/digest", map[string]any{"time_of_day": "7am"}, nil); code != 400 {
		t.Fatalf("bad digest time: %d", code)
	}
	for i := 0; i < 2; i++ {
		if _, err := h.app.sendDigests(ctx, now.Add(30*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	msgs := sentTo(*sent, h.chatID)
	if len(msgs) != 1 || !strings.Contains(msgs[0].msg.Text, "Water plants") || !strings.Contains(msgs[0].msg.Text, "Coming up") {
		t.Fatalf("digest messages: %+v", msgs)
	}
	// disabled -> nothing on another day
	h.owner.do("PATCH", "/api/household/digest", map[string]any{"enabled": false}, nil)
	if _, err := h.app.sendDigests(ctx, now.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(sentTo(*sent, h.chatID)) != 1 {
		t.Fatal("disabled digest was sent")
	}
}

func TestHouseholdTimezoneEndpoint(t *testing.T) {
	h := newRemHousehold(t, "")
	var res map[string]any
	h.owner.do("GET", "/api/household/timezone", nil, &res)
	if res["timezone"] != "UTC" || res["is_default"] != true {
		t.Fatalf("default tz: %v", res)
	}
	for _, bad := range []string{"", "Local", "Mars/Olympus"} {
		if code := h.owner.do("PATCH", "/api/household/timezone", map[string]any{"timezone": bad}, &res); code != 400 {
			t.Fatalf("tz %q: %d", bad, code)
		}
	}
	// changing the zone moves pending occurrences to the new local time
	rid := h.createReminder(t, h.owner, map[string]any{"title": "Daily", "kind": "daily", "time_of_day": "08:00"})
	if code := h.owner.do("PATCH", "/api/household/timezone", map[string]any{"timezone": "Europe/Berlin"}, &res); code != 200 {
		t.Fatalf("set: %d %v", code, res)
	}
	var bad int
	_ = h.app.DB.QueryRow(context.Background(), `SELECT count(*) FROM reminder_occurrences WHERE reminder_id=$1 AND status='pending'
	  AND to_char(due_at AT TIME ZONE 'Europe/Berlin','HH24:MI') <> '08:00'`, rid).Scan(&bad)
	if bad != 0 {
		t.Fatalf("%d occurrences not moved to Berlin time", bad)
	}
}
