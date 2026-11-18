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
	"github.com/jackc/pgx/v5"
)

// Reminders: household reminder rules (reminders) and their concrete due
// instants (reminder_occurrences). The worker (reminders_worker.go)
// materializes occurrences, sends them to chat, escalates and marks missed.
//
// Permissions: owner/parent manage rules; kids may mark occurrences
// assigned to them (or to the whole family) done / snoozed; viewers read.

// reminderCategoryIDs is the ordered list of categories (household first
// in the clients; keep schema.sql's reminders_category_check in sync).
var reminderCategoryIDs = []string{"cleaning", "laundry", "cooking", "groceries", "home_maintenance", "vehicle", "self_care",
	"chores", "bill", "medicine", "school", "birthday", "other"}

var validReminderCategories = func() map[string]bool {
	m := map[string]bool{}
	for _, c := range reminderCategoryIDs {
		m[c] = true
	}
	return m
}()

// reminderCategoryLabels are the human labels used in chat replies.
var reminderCategoryLabels = map[string]string{
	"cleaning": "Cleaning", "laundry": "Laundry", "cooking": "Kitchen & meals", "groceries": "Groceries",
	"home_maintenance": "Home maintenance", "vehicle": "Vehicle", "self_care": "Self-care", "chores": "Chores",
	"bill": "Bills", "medicine": "Medicine", "school": "School", "birthday": "Birthdays", "other": "Other",
}
var validReminderChannels = map[string]bool{"chat": true, "push": true, "in-app": true}

func RegisterReminderRoutes(r chi.Router, app *App) {
	r.Get("/reminders", app.handleListReminders)
	r.Post("/reminders", app.handleCreateReminder)
	r.Get("/reminders/upcoming", app.handleUpcomingReminders)
	r.Patch("/reminders/{id}", app.handleUpdateReminder)
	r.Delete("/reminders/{id}", app.handleDeleteReminder)
	r.Post("/reminders/occurrences/{id}/done", app.handleOccurrenceDone)
	r.Post("/reminders/occurrences/{id}/snooze", app.handleOccurrenceSnooze)

	r.Get("/reminders/chat-link", app.handleChatLinkStatus)
	r.Post("/reminders/link-code", app.handleCreateLinkCode)
	r.Delete("/reminders/chat-link/{platform}", app.handleUnlinkChat)

	r.Get("/household/timezone", app.handleGetTimezone)
	r.Patch("/household/timezone", app.handleSetTimezone)
	r.Get("/household/digest", app.handleGetDigest)
	r.Patch("/household/digest", app.handleSetDigest)

	r.Get("/today", app.handleToday)

	// bot-token authenticated (memberMiddleware skips /api/bot/*)
	r.Get("/bot/today", botAuth(app, botToday))
	r.Get("/bot/reminders", botAuth(app, botListReminders))
	r.Post("/bot/reminders", botAuth(app, botCreateReminder))
	r.Post("/bot/reminders/occurrences/{id}/done", botAuth(app, botOccurrenceDone))
	r.Post("/bot/reminders/occurrences/{id}/snooze", botAuth(app, botOccurrenceSnooze))
	r.Post("/bot/link", botAuth(app, botLink))
}

var kidReminderPathRe = regexp.MustCompile(`^/api/reminders/occurrences/(\d+)/(done|snooze)$`)

func init() {
	// Kids may complete/snooze occurrences (the handler checks assignment)
	// and link/unlink their own chat account.
	registerKidRule(func(method, path string) (roleDecision, int64, bool) {
		if method == http.MethodPost && kidReminderPathRe.MatchString(path) {
			return roleAllow, 0, true
		}
		if method == http.MethodPost && path == "/api/reminders/link-code" {
			return roleAllow, 0, true
		}
		if method == http.MethodDelete && strings.HasPrefix(path, "/api/reminders/chat-link/") {
			return roleAllow, 0, true
		}
		return 0, 0, false
	})
}

// ---------- model ----------

type reminderRow struct {
	ID                   int64      `json:"id"`
	Title                string     `json:"title"`
	Notes                string     `json:"notes"`
	AssigneeID           *int64     `json:"assignee_id"`
	AssigneeName         string     `json:"assignee_name,omitempty"`
	CreatedBy            *int64     `json:"created_by"`
	Kind                 string     `json:"kind"`
	TimeOfDay            string     `json:"time_of_day"`
	Weekdays             []int      `json:"weekdays"`
	DayOfMonth           *int       `json:"day_of_month"`
	Interval             int        `json:"interval"`
	StartDate            string     `json:"start_date"`
	EndDate              *string    `json:"end_date"`
	OnceAt               *time.Time `json:"once_at"`
	EscalateTo           *int64     `json:"escalate_to"`
	EscalateToName       string     `json:"escalate_to_name,omitempty"`
	EscalateAfterMinutes *int       `json:"escalate_after_minutes"`
	Channels             []string   `json:"channels"`
	Active               bool       `json:"active"`
	Category             string     `json:"category"`
	Description          string     `json:"description"`
	NextDue              *time.Time `json:"next_due"`
}

const reminderCols = `r.id, r.title, r.notes, r.assignee_id, r.created_by, r.kind, r.time_of_day, r.weekdays,
	r.day_of_month, r.start_date, r.end_date, r.once_at, r.escalate_to, r.escalate_after_minutes, r.channels, r.active, r.category, r.repeat_interval`

func scanReminder(row pgx.Row) (reminderRow, error) {
	var x reminderRow
	var wd []int32
	var dom, esc *int32
	var start time.Time
	var end *time.Time
	err := row.Scan(&x.ID, &x.Title, &x.Notes, &x.AssigneeID, &x.CreatedBy, &x.Kind, &x.TimeOfDay, &wd,
		&dom, &start, &end, &x.OnceAt, &x.EscalateTo, &esc, &x.Channels, &x.Active, &x.Category, &x.Interval)
	if err != nil {
		return x, err
	}
	x.Weekdays = make([]int, len(wd))
	for i, v := range wd {
		x.Weekdays[i] = int(v)
	}
	if dom != nil {
		v := int(*dom)
		x.DayOfMonth = &v
	}
	if esc != nil {
		v := int(*esc)
		x.EscalateAfterMinutes = &v
	}
	x.StartDate = start.Format("2006-01-02")
	if end != nil {
		s := end.Format("2006-01-02")
		x.EndDate = &s
	}
	if x.Channels == nil {
		x.Channels = []string{}
	}
	return x, nil
}

func (x reminderRow) schedule() reminderSchedule {
	s := reminderSchedule{Kind: x.Kind, TimeOfDay: x.TimeOfDay, Weekdays: x.Weekdays, OnceAt: x.OnceAt, Interval: x.Interval}
	if x.DayOfMonth != nil {
		s.DayOfMonth = *x.DayOfMonth
	}
	s.StartDate, _ = time.Parse("2006-01-02", x.StartDate)
	if x.EndDate != nil {
		if e, err := time.Parse("2006-01-02", *x.EndDate); err == nil {
			s.EndDate = &e
		}
	}
	return s
}

// memberNames maps member id -> display name for a household.
func (app *App) memberNames(ctx context.Context, aid int64) (map[int64]string, error) {
	rows, err := app.DB.Query(ctx, `SELECT id, COALESCE(NULLIF(display_name,''), split_part(email,'@',1)) FROM members WHERE account_id=$1`, aid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var n string
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

func nameOf(names map[int64]string, id *int64) string {
	if id == nil {
		return ""
	}
	return names[*id]
}

// kidFilter returns the SQL fragment restricting reminders (alias r) to the
// member's own + whole-family ones when the member is a kid.
func kidFilter(m memberInfo, argN int) (string, []any) {
	if m.Role == roleKid {
		return fmt.Sprintf(" AND (r.assignee_id IS NULL OR r.assignee_id=$%d)", argN), []any{m.ID}
	}
	return "", nil
}

// ---------- CRUD ----------

func (app *App) handleListReminders(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	aid := m.AccountID
	loc := householdLocation(ctx, app, aid)
	extra, args := kidFilter(m, 2)
	rows, err := app.DB.Query(ctx, "SELECT "+reminderCols+" FROM reminders r WHERE r.account_id=$1"+extra+" ORDER BY r.active DESC, r.title, r.id",
		append([]any{aid}, args...)...)
	if err != nil {
		dbError(w, err)
		return
	}
	var list []reminderRow
	for rows.Next() {
		x, err := scanReminder(rows)
		if err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		list = append(list, x)
	}
	rows.Close()
	names, err := app.memberNames(ctx, aid)
	if err != nil {
		dbError(w, err)
		return
	}
	now := time.Now().UTC()
	out := []reminderRow{}
	for _, x := range list {
		x.AssigneeName, x.EscalateToName = nameOf(names, x.AssigneeID), nameOf(names, x.EscalateTo)
		s := x.schedule()
		x.Description = describeSchedule(s, loc)
		if x.Active {
			if next := occurrencesBetween(s, loc, now, now.AddDate(1, 1, 0)); len(next) > 0 {
				x.NextDue = &next[0]
			}
		}
		out = append(out, x)
	}
	cats := make([]map[string]string, 0, len(reminderCategoryIDs))
	for _, c := range reminderCategoryIDs {
		cats = append(cats, map[string]string{"id": c, "label": reminderCategoryLabels[c]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"timezone": loc.String(), "reminders": out, "categories": cats})
}

// reminderInput is decoded on top of the existing row for PATCH, so absent
// fields keep their values (JSON null clears pointers).
type reminderInput struct {
	Title                string     `json:"title"`
	Notes                string     `json:"notes"`
	AssigneeID           *int64     `json:"assignee_id"`
	Kind                 string     `json:"kind"`
	TimeOfDay            string     `json:"time_of_day"`
	Weekdays             []int      `json:"weekdays"`
	DayOfMonth           *int       `json:"day_of_month"`
	Interval             int        `json:"interval"` // weekly/monthly: every N weeks/months (0 = 1)
	StartDate            string     `json:"start_date"`
	EndDate              *string    `json:"end_date"`
	OnceAt               *time.Time `json:"once_at"`
	EscalateTo           *int64     `json:"escalate_to"`
	EscalateAfterMinutes *int       `json:"escalate_after_minutes"`
	Channels             []string   `json:"channels"`
	Active               *bool      `json:"active"`
	Category             string     `json:"category"`
}

type validReminder struct {
	in       reminderInput
	sched    reminderSchedule
	startD   time.Time
	endD     *time.Time
	active   bool
	channels []string
}

// validateReminderInput normalizes and checks input. onceFromLocal: compute
// once_at from start_date + time_of_day in loc (when the client sent no
// explicit once_at).
func (app *App) validateReminderInput(ctx context.Context, aid int64, in reminderInput, loc *time.Location, onceFromLocal bool) (validReminder, error) {
	v := validReminder{in: in, active: true}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return v, errors.New("title is required")
	}
	if len(in.Title) > 200 {
		return v, errors.New("title is too long (max 200)")
	}
	if len(in.Notes) > 2000 {
		return v, errors.New("notes are too long (max 2000)")
	}
	if in.Category == "" {
		in.Category = "other"
	}
	if !validReminderCategories[in.Category] {
		return v, errors.New("category must be one of: " + strings.Join(reminderCategoryIDs, ", "))
	}
	if in.TimeOfDay == "" {
		in.TimeOfDay = "09:00"
	}
	hh, mm, err := parseHHMM(in.TimeOfDay)
	if err != nil {
		return v, err
	}
	in.TimeOfDay = fmtHHMM(hh, mm)
	today := householdToday(ctx, app, aid, time.Now())
	v.startD = today
	if in.StartDate != "" {
		d, err := time.Parse("2006-01-02", in.StartDate)
		if err != nil {
			return v, errors.New("start_date must be YYYY-MM-DD")
		}
		v.startD = d
	}
	if in.EndDate != nil && *in.EndDate != "" {
		d, err := time.Parse("2006-01-02", *in.EndDate)
		if err != nil {
			return v, errors.New("end_date must be YYYY-MM-DD")
		}
		v.endD = &d
	} else {
		in.EndDate = nil
	}
	if in.Kind == "once" && (onceFromLocal || in.OnceAt == nil) {
		t := time.Date(v.startD.Year(), v.startD.Month(), v.startD.Day(), hh, mm, 0, 0, loc).UTC()
		in.OnceAt = &t
	}
	if in.Kind == "once" && in.OnceAt != nil {
		t := in.OnceAt.UTC().Truncate(time.Minute)
		in.OnceAt = &t
		v.startD = dateOnly(t.In(loc))
	}
	if in.Kind != "weekly" {
		in.Weekdays = nil
	}
	if in.Kind != "monthly" && in.Kind != "yearly" {
		in.DayOfMonth = nil
	}
	if in.Interval < 0 || in.Interval > maxReminderInterval {
		return v, fmt.Errorf("interval must be 1-%d", maxReminderInterval)
	}
	if in.Interval == 0 || (in.Kind != "weekly" && in.Kind != "monthly") {
		in.Interval = 1
	}
	v.sched = reminderSchedule{Kind: in.Kind, TimeOfDay: in.TimeOfDay, Weekdays: in.Weekdays, StartDate: v.startD, EndDate: v.endD, OnceAt: in.OnceAt, Interval: in.Interval}
	if in.DayOfMonth != nil {
		v.sched.DayOfMonth = *in.DayOfMonth
	}
	if err := v.sched.validate(); err != nil {
		return v, err
	}
	if in.Kind != "once" {
		in.OnceAt = nil
	}
	if in.Channels == nil {
		in.Channels = []string{"chat", "in-app"}
	}
	for _, c := range in.Channels {
		if !validReminderChannels[c] {
			return v, errors.New("channels may contain chat, push, in-app")
		}
		if !slices.Contains(v.channels, c) {
			v.channels = append(v.channels, c)
		}
	}
	if v.channels == nil {
		v.channels = []string{}
	}
	for _, id := range []*int64{in.AssigneeID, in.EscalateTo} {
		if id == nil {
			continue
		}
		var x int64
		if err := app.DB.QueryRow(ctx, "SELECT id FROM members WHERE id=$1 AND account_id=$2", *id, aid).Scan(&x); err != nil {
			return v, errors.New("unknown household member")
		}
	}
	if in.EscalateTo == nil {
		in.EscalateAfterMinutes = nil
	} else {
		if in.EscalateAfterMinutes == nil {
			n := 30
			in.EscalateAfterMinutes = &n
		}
		if *in.EscalateAfterMinutes < 1 || *in.EscalateAfterMinutes > 1440 {
			return v, errors.New("escalate_after_minutes must be 1-1440")
		}
	}
	if in.Active != nil {
		v.active = *in.Active
	}
	v.in = in
	return v, nil
}

func int32Slice(xs []int) []int32 {
	out := make([]int32, 0, len(xs))
	for _, x := range xs {
		out = append(out, int32(x))
	}
	return out
}

// insertReminder stores a validated reminder and materializes its first
// occurrences right away.
func (app *App) insertReminder(ctx context.Context, aid int64, createdBy *int64, v validReminder) (int64, error) {
	in := v.in
	var id int64
	err := app.DB.QueryRow(ctx, `INSERT INTO reminders (account_id, title, notes, assignee_id, created_by, kind, time_of_day, weekdays,
	    day_of_month, start_date, end_date, once_at, escalate_to, escalate_after_minutes, channels, active, category, repeat_interval, schedule_from)
	  VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18, now() - interval '1 minute') RETURNING id`,
		aid, strings.TrimSpace(in.Title), in.Notes, in.AssigneeID, createdBy, in.Kind, in.TimeOfDay, int32Slice(in.Weekdays),
		in.DayOfMonth, v.startD, v.endD, in.OnceAt, in.EscalateTo, in.EscalateAfterMinutes, v.channels, v.active, in.Category, in.Interval).Scan(&id)
	if err != nil {
		return 0, err
	}
	if _, err := app.materialize(ctx, aid, time.Now().UTC(), materializeHorizon); err != nil {
		return id, err
	}
	return id, nil
}

func (app *App) handleCreateReminder(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var in reminderInput
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	loc := householdLocation(ctx, app, m.AccountID)
	v, err := app.validateReminderInput(ctx, m.AccountID, in, loc, in.OnceAt == nil)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if v.in.Kind == "once" && v.in.OnceAt.Before(time.Now().Add(-time.Minute)) {
		jsonError(w, http.StatusBadRequest, "that time has already passed")
		return
	}
	id, err := app.insertReminder(ctx, m.AccountID, currentMemberIDPtr(r), v)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "description": describeSchedule(v.sched, loc)})
}

func (app *App) loadReminder(ctx context.Context, aid, id int64) (reminderRow, error) {
	return scanReminder(app.DB.QueryRow(ctx, "SELECT "+reminderCols+" FROM reminders r WHERE r.id=$1 AND r.account_id=$2", id, aid))
}

func (app *App) handleUpdateReminder(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	id, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := r.Context()
	aid := m.AccountID
	cur, err := app.loadReminder(ctx, aid, id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "reminder not found")
		return
	}
	loc := householdLocation(ctx, app, aid)
	// copies: json decoding writes through non-nil pointers, which would
	// silently change `cur` and hide schedule changes.
	in := reminderInput{Title: cur.Title, Notes: cur.Notes, AssigneeID: clonePtr(cur.AssigneeID), Kind: cur.Kind, TimeOfDay: cur.TimeOfDay,
		Weekdays: slices.Clone(cur.Weekdays), DayOfMonth: clonePtr(cur.DayOfMonth), Interval: cur.Interval, StartDate: cur.StartDate, EndDate: clonePtr(cur.EndDate),
		OnceAt: clonePtr(cur.OnceAt), EscalateTo: clonePtr(cur.EscalateTo), EscalateAfterMinutes: clonePtr(cur.EscalateAfterMinutes),
		Channels: slices.Clone(cur.Channels), Active: clonePtr(&cur.Active), Category: cur.Category}
	if cur.OnceAt != nil { // expose the once instant as local date + time for edits
		l := cur.OnceAt.In(loc)
		in.StartDate, in.TimeOfDay = l.Format("2006-01-02"), l.Format("15:04")
	}
	var raw map[string]json.RawMessage
	if err := decodeJSON(r, &raw); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	b, _ := json.Marshal(raw)
	if err := json.Unmarshal(b, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid field: "+err.Error())
		return
	}
	_, hasOnce := raw["once_at"]
	v, err := app.validateReminderInput(ctx, aid, in, loc, !hasOnce)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	n := v.in
	schedChanged := n.Kind != cur.Kind || n.TimeOfDay != cur.TimeOfDay || fmt.Sprint(n.Weekdays) != fmt.Sprint(cur.Weekdays) ||
		fmt.Sprint(derefInt(n.DayOfMonth)) != fmt.Sprint(derefInt(cur.DayOfMonth)) || v.startD.Format("2006-01-02") != cur.StartDate ||
		fmt.Sprint(derefStr(n.EndDate)) != fmt.Sprint(derefStr(cur.EndDate)) || !timePtrEq(n.OnceAt, cur.OnceAt) || v.active != cur.Active || n.Interval != cur.Interval
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `UPDATE reminders SET title=$1, notes=$2, assignee_id=$3, kind=$4, time_of_day=$5, weekdays=$6, day_of_month=$7,
	    start_date=$8, end_date=$9, once_at=$10, escalate_to=$11, escalate_after_minutes=$12, channels=$13, active=$14, category=$15, repeat_interval=$16, updated_at=now()
	  WHERE id=$17 AND account_id=$18`,
		strings.TrimSpace(n.Title), n.Notes, n.AssigneeID, n.Kind, n.TimeOfDay, int32Slice(n.Weekdays), n.DayOfMonth,
		v.startD, v.endD, n.OnceAt, n.EscalateTo, n.EscalateAfterMinutes, v.channels, v.active, n.Category, n.Interval, id, aid)
	if err != nil {
		dbError(w, err)
		return
	}
	if schedChanged {
		if err := resetFutureOccurrences(ctx, tx, "id=$1", id); err != nil {
			dbError(w, err)
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		dbError(w, err)
		return
	}
	if _, err := app.materialize(ctx, aid, time.Now().UTC(), materializeHorizon); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "description": describeSchedule(v.sched, loc)})
}

// resetFutureOccurrences drops not-yet-sent future occurrences and restarts
// materialization from now (schedule/time zone/pause changes).
func resetFutureOccurrences(ctx context.Context, tx pgx.Tx, where string, arg int64) error {
	if _, err := tx.Exec(ctx, "DELETE FROM reminder_occurrences WHERE status='pending' AND due_at > now() AND reminder_id IN (SELECT id FROM reminders WHERE "+where+")", arg); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "UPDATE reminders SET schedule_from=now(), materialized_until=NULL WHERE "+where, arg)
	return err
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func derefInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
func derefStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}
func timePtrEq(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func (app *App) handleDeleteReminder(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	id, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	tag, err := app.DB.Exec(r.Context(), "DELETE FROM reminders WHERE id=$1 AND account_id=$2", id, m.AccountID)
	if err != nil {
		dbError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, http.StatusNotFound, "reminder not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- occurrences ----------

type occurrenceRow struct {
	ID           int64      `json:"id"`
	ReminderID   int64      `json:"reminder_id"`
	Title        string     `json:"title"`
	Notes        string     `json:"notes"`
	Category     string     `json:"category"`
	Kind         string     `json:"kind"`
	AssigneeID   *int64     `json:"assignee_id"`
	AssigneeName string     `json:"assignee_name,omitempty"`
	DueAt        time.Time  `json:"due_at"`
	Status       string     `json:"status"`
	SnoozedUntil *time.Time `json:"snoozed_until"`
	DoneAt       *time.Time `json:"done_at"`
	DoneBy       *int64     `json:"done_by"`
	DoneByName   string     `json:"done_by_name,omitempty"`
	Overdue      bool       `json:"overdue"`
	CanAct       bool       `json:"can_act"`
}

// effective is when the occurrence is (next) due: snoozed_until or due_at.
func (o occurrenceRow) effective() time.Time {
	if o.Status == "snoozed" && o.SnoozedUntil != nil {
		return *o.SnoozedUntil
	}
	return o.DueAt
}

func (o occurrenceRow) open() bool {
	return o.Status == "pending" || o.Status == "sent" || o.Status == "snoozed"
}

// queryOccurrences returns occurrences (alias o, reminder alias r) matching
// the extra WHERE clause; args start at $2 ($1 = account id).
func (app *App) queryOccurrences(ctx context.Context, aid int64, m memberInfo, where string, args ...any) ([]occurrenceRow, error) {
	all := append([]any{aid}, args...)
	kf, kargs := kidFilter(m, len(all)+1)
	all = append(all, kargs...)
	rows, err := app.DB.Query(ctx, `SELECT o.id, o.reminder_id, r.title, r.notes, r.category, r.kind, r.assignee_id, o.due_at, o.status,
	    o.snoozed_until, o.done_at, o.done_by
	  FROM reminder_occurrences o JOIN reminders r ON r.id=o.reminder_id
	  WHERE o.account_id=$1 AND (`+where+`)`+kf+` ORDER BY COALESCE(CASE WHEN o.status='snoozed' THEN o.snoozed_until END, o.due_at), o.id`, all...)
	if err != nil {
		return nil, err
	}
	var out []occurrenceRow
	for rows.Next() {
		var o occurrenceRow
		if err := rows.Scan(&o.ID, &o.ReminderID, &o.Title, &o.Notes, &o.Category, &o.Kind, &o.AssigneeID, &o.DueAt, &o.Status,
			&o.SnoozedUntil, &o.DoneAt, &o.DoneBy); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	names, err := app.memberNames(ctx, aid)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for i := range out {
		o := &out[i]
		o.AssigneeName, o.DoneByName = nameOf(names, o.AssigneeID), nameOf(names, o.DoneBy)
		o.Overdue = o.open() && o.effective().Before(now)
		o.CanAct = canActOnOccurrence(m, o.AssigneeID)
	}
	return out, nil
}

// canActOnOccurrence: viewers never; kids only their own / whole-family
// occurrences; everyone else (incl. an unlinked chat user) yes.
func canActOnOccurrence(m memberInfo, assignee *int64) bool {
	switch m.Role {
	case roleViewer:
		return false
	case roleKid:
		return assignee == nil || *assignee == m.ID
	}
	return true
}

func (app *App) handleUpcomingReminders(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	aid := m.AccountID
	days := queryIntDefault(r, "days", 7)
	days = max(1, min(days, 31))
	now := time.Now().UTC()
	if _, err := app.materialize(ctx, aid, now, time.Duration(days+1)*24*time.Hour); err != nil {
		dbError(w, err)
		return
	}
	loc := householdLocation(ctx, app, aid)
	start, _ := localDayBounds(now, loc)
	y, mo, d := now.In(loc).Date()
	end := time.Date(y, mo, d+days, 0, 0, 0, 0, loc).UTC()
	occ, err := app.queryOccurrences(ctx, aid, m,
		`(o.due_at >= $2 AND o.due_at < $3) OR (o.status='snoozed' AND o.snoozed_until >= $2 AND o.snoozed_until < $3)`, start, end)
	if err != nil {
		dbError(w, err)
		return
	}
	if occ == nil {
		occ = []occurrenceRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"timezone": loc.String(), "today": dateOnly(now.In(loc)).Format("2006-01-02"), "items": occ})
}

func (app *App) loadOccurrence(ctx context.Context, aid, id int64) (occurrenceRow, error) {
	occ, err := app.queryOccurrences(ctx, aid, memberInfo{}, "o.id=$2", id)
	if err != nil {
		return occurrenceRow{}, err
	}
	if len(occ) == 0 {
		return occurrenceRow{}, pgx.ErrNoRows
	}
	return occ[0], nil
}

var errOccForbidden = errors.New("you can only update reminders assigned to you or the whole family")

// markOccurrenceDone completes an occurrence on behalf of m (m.ID 0 =
// unknown chat user). Returns the occurrence and whether it was already done.
func (app *App) markOccurrenceDone(ctx context.Context, aid int64, m memberInfo, id int64) (occurrenceRow, bool, error) {
	o, err := app.loadOccurrence(ctx, aid, id)
	if err != nil {
		return o, false, err
	}
	if !canActOnOccurrence(m, o.AssigneeID) {
		return o, false, errOccForbidden
	}
	if o.Status == "done" {
		return o, true, nil
	}
	var by *int64
	if m.ID != 0 {
		by = &m.ID
	}
	_, err = app.DB.Exec(ctx, "UPDATE reminder_occurrences SET status='done', done_at=now(), done_by=$1 WHERE id=$2 AND account_id=$3 AND status<>'done'", by, id, aid)
	o.Status = "done"
	return o, false, err
}

// snoozeOccurrence: minutes > 0, or until ("tomorrow" or RFC3339).
func (app *App) snoozeOccurrence(ctx context.Context, aid int64, m memberInfo, id int64, minutes int, until string) (occurrenceRow, time.Time, error) {
	o, err := app.loadOccurrence(ctx, aid, id)
	if err != nil {
		return o, time.Time{}, err
	}
	if !canActOnOccurrence(m, o.AssigneeID) {
		return o, time.Time{}, errOccForbidden
	}
	if o.Status == "done" {
		return o, time.Time{}, errors.New("already done")
	}
	now := time.Now().UTC()
	var t time.Time
	switch {
	case until == "tomorrow":
		t = sameLocalTimeNextDay(o.DueAt, now, householdLocation(ctx, app, aid))
	case until != "":
		t, err = time.Parse(time.RFC3339, until)
		if err != nil {
			return o, t, errors.New("until must be \"tomorrow\" or an RFC3339 time")
		}
	case minutes > 0 && minutes <= 7*24*60:
		t = now.Add(time.Duration(minutes) * time.Minute)
	default:
		return o, t, errors.New("snooze needs minutes (1-10080) or until")
	}
	if !t.After(now) {
		return o, t, errors.New("snooze time must be in the future")
	}
	t = t.UTC().Truncate(time.Minute)
	_, err = app.DB.Exec(ctx, "UPDATE reminder_occurrences SET status='snoozed', snoozed_until=$1 WHERE id=$2 AND account_id=$3", t, id, aid)
	return o, t, err
}

func occErrorStatus(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		jsonError(w, http.StatusNotFound, "reminder not found")
	case errors.Is(err, errOccForbidden):
		jsonError(w, http.StatusForbidden, err.Error())
	default:
		var pe interface{ SQLState() string }
		if errors.As(err, &pe) {
			dbError(w, err)
			return
		}
		jsonError(w, http.StatusBadRequest, err.Error())
	}
}

func (app *App) handleOccurrenceDone(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	id, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	o, already, err := app.markOccurrenceDone(r.Context(), m.AccountID, m, id)
	if err != nil {
		occErrorStatus(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "already_done": already, "title": o.Title})
}

type snoozeBody struct {
	Minutes int    `json:"minutes"`
	Until   string `json:"until"`
}

func (app *App) handleOccurrenceSnooze(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	id, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var b snoozeBody
	if err := decodeJSON(r, &b); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	o, until, err := app.snoozeOccurrence(r.Context(), m.AccountID, m, id, b.Minutes, b.Until)
	if err != nil {
		occErrorStatus(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "snoozed_until": until, "title": o.Title})
}
