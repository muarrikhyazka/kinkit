package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Family calendar: activities (Google-Calendar style events) plus the
// reminders overlay used by the calendar views.
//
// Activities store household-local wall-clock dates/times. A repeating
// activity is one rule row that is expanded into occurrences on read.
// Permissions: owners/parents create, edit and delete; kids and viewers read
// (kids only see activities that include them or the whole family).
//
//	GET    /api/activities?from=YYYY-MM-DD&to=YYYY-MM-DD   expanded occurrences
//	POST   /api/activities
//	GET    /api/activities/{id}                             the rule
//	PATCH  /api/activities/{id}
//	DELETE /api/activities/{id}[?on=YYYY-MM-DD]             whole series, or one occurrence
//	GET    /api/calendar?from=&to=[&include=activities,reminders]

const (
	maxCalendarRangeDays = 100
	maxActivitySpanDays  = 31
)

var activityColors = []string{"indigo", "rose", "amber", "emerald", "sky", "violet", "slate"}

var validRepeatKinds = map[string]bool{"none": true, "daily": true, "weekly": true, "monthly": true, "yearly": true}

func RegisterCalendarRoutes(r chi.Router, app *App) {
	r.Get("/activities", app.handleListActivities)
	r.Post("/activities", app.handleCreateActivity)
	r.Get("/activities/{id}", app.handleGetActivity)
	r.Patch("/activities/{id}", app.handleUpdateActivity)
	r.Delete("/activities/{id}", app.handleDeleteActivity)
	r.Get("/calendar", app.handleCalendar)
}

func init() {
	registerTodayProvider("activities", 15, activitiesTodayProvider)
}

// ---------- model ----------

type activityRow struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Notes       string  `json:"notes"`
	Location    string  `json:"location"`
	Color       string  `json:"color"`
	AllDay      bool    `json:"all_day"`
	StartDate   string  `json:"start_date"`
	StartTime   string  `json:"start_time"`
	EndDate     string  `json:"end_date"`
	EndTime     string  `json:"end_time"`
	Repeat      string  `json:"repeat"`
	Interval    int     `json:"interval"`
	Weekdays    []int   `json:"weekdays"`
	UntilDate   *string `json:"until_date"`
	AttendeeIDs []int64 `json:"attendee_ids"`
	CreatedBy   *int64  `json:"created_by"`
}

// activityOccurrence is one expanded instance. Rule fields describe the
// series (so a client can open the edit form); OccursOn/OccursUntil are this
// instance's local start/end dates.
type activityOccurrence struct {
	activityRow
	OccursOn      string   `json:"occurs_on"`
	OccursUntil   string   `json:"occurs_until"`
	Repeating     bool     `json:"repeating"`
	AttendeeNames []string `json:"attendee_names"`
}

const activityCols = `id, title, notes, location, color, all_day, start_date, start_time, end_date, end_time,
	repeat_kind, repeat_interval, weekdays, until_date, attendee_ids, created_by`

func scanActivity(row pgx.Row) (activityRow, error) {
	var a activityRow
	var sd, ed time.Time
	var until *time.Time
	var wd []int32
	err := row.Scan(&a.ID, &a.Title, &a.Notes, &a.Location, &a.Color, &a.AllDay, &sd, &a.StartTime, &ed, &a.EndTime,
		&a.Repeat, &a.Interval, &wd, &until, &a.AttendeeIDs, &a.CreatedBy)
	if err != nil {
		return a, err
	}
	a.StartDate, a.EndDate = sd.Format("2006-01-02"), ed.Format("2006-01-02")
	a.UntilDate = fmtDatePtr(until)
	a.Weekdays = make([]int, len(wd))
	for i, v := range wd {
		a.Weekdays[i] = int(v)
	}
	if a.AttendeeIDs == nil {
		a.AttendeeIDs = []int64{}
	}
	return a, nil
}

// ---------- expansion (pure, unit tested) ----------

func parseDay(s string) (time.Time, error) { return time.Parse("2006-01-02", s) }

// addMonthsClamped adds n months to d keeping `day` (clamped to the month end).
func addMonthsClamped(d time.Time, day, n int) time.Time {
	total := d.Year()*12 + int(d.Month()) - 1 + n
	y, m := total/12, time.Month(total%12+1)
	return time.Date(y, m, min(day, daysIn(y, m)), 0, 0, 0, 0, time.UTC)
}

// occurrenceStarts lists the start dates of the activity that fall in
// [from, to] (inclusive; occurrences that merely overlap the window because
// they began earlier are included via `span`). skip holds YYYY-MM-DD dates.
func occurrenceStarts(a activityRow, from, to time.Time, skip map[string]bool) []time.Time {
	start, err := parseDay(a.StartDate)
	if err != nil {
		return nil
	}
	end, err := parseDay(a.EndDate)
	if err != nil {
		return nil
	}
	span := int(end.Sub(start).Hours() / 24)
	var until *time.Time
	if a.UntilDate != nil {
		if u, err := parseDay(*a.UntilDate); err == nil {
			until = &u
		}
	}
	interval := max(a.Interval, 1)
	lo := from.AddDate(0, 0, -span) // an occurrence starting at lo is still visible on `from`
	var out []time.Time
	add := func(d time.Time) {
		if d.Before(start) || d.Before(lo) || d.After(to) {
			return
		}
		if until != nil && d.After(*until) {
			return
		}
		if skip[d.Format("2006-01-02")] {
			return
		}
		out = append(out, d)
	}
	switch a.Repeat {
	case "daily":
		d := start
		if lo.After(start) {
			k := int(lo.Sub(start).Hours()/24) / interval
			d = start.AddDate(0, 0, k*interval)
		}
		for ; !d.After(to); d = d.AddDate(0, 0, interval) {
			add(d)
		}
	case "weekly":
		wds := a.Weekdays
		if len(wds) == 0 {
			wds = []int{int(start.Weekday())}
		}
		weekStart := start.AddDate(0, 0, -int(start.Weekday())) // Sunday of the first week
		d := start
		if lo.After(start) {
			d = lo
		}
		for ; !d.After(to); d = d.AddDate(0, 0, 1) {
			weeks := int(d.Sub(weekStart).Hours()/24) / 7
			if weeks%interval == 0 && slices.Contains(wds, int(d.Weekday())) {
				add(d)
			}
		}
	case "monthly":
		for k := 0; k < 2400; k++ {
			d := addMonthsClamped(start, start.Day(), k*interval)
			if d.After(to) {
				break
			}
			add(d)
		}
	case "yearly":
		for k := 0; k < 200; k++ {
			d := addMonthsClamped(start, start.Day(), 12*k*interval)
			if d.After(to) {
				break
			}
			add(d)
		}
	default:
		add(start)
	}
	return out
}

// expandActivities turns rules into sorted occurrences within [from, to].
func expandActivities(rules []activityRow, skips map[int64]map[string]bool, from, to time.Time, names map[int64]string) []activityOccurrence {
	out := []activityOccurrence{}
	for _, a := range rules {
		start, _ := parseDay(a.StartDate)
		end, _ := parseDay(a.EndDate)
		span := end.Sub(start)
		for _, d := range occurrenceStarts(a, from, to, skips[a.ID]) {
			o := activityOccurrence{activityRow: a, OccursOn: d.Format("2006-01-02"), OccursUntil: d.Add(span).Format("2006-01-02"),
				Repeating: a.Repeat != "none", AttendeeNames: []string{}}
			for _, id := range a.AttendeeIDs {
				if n, ok := names[id]; ok {
					o.AttendeeNames = append(o.AttendeeNames, n)
				}
			}
			out = append(out, o)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.OccursOn != b.OccursOn {
			return a.OccursOn < b.OccursOn
		}
		if a.AllDay != b.AllDay {
			return a.AllDay
		}
		if a.StartTime != b.StartTime {
			return a.StartTime < b.StartTime
		}
		return a.ID < b.ID
	})
	return out
}

// ---------- validation ----------

type activityInput struct {
	Title       *string  `json:"title"`
	Notes       *string  `json:"notes"`
	Location    *string  `json:"location"`
	Color       *string  `json:"color"`
	AllDay      *bool    `json:"all_day"`
	StartDate   *string  `json:"start_date"`
	StartTime   *string  `json:"start_time"`
	EndDate     *string  `json:"end_date"`
	EndTime     *string  `json:"end_time"`
	Repeat      *string  `json:"repeat"`
	Interval    *int     `json:"interval"`
	Weekdays    *[]int   `json:"weekdays"`
	UntilDate   *string  `json:"until_date"` // "" clears
	AttendeeIDs *[]int64 `json:"attendee_ids"`
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// applyActivityInput merges in onto a (new rows start from defaults) and
// validates the result.
func applyActivityInput(a *activityRow, in activityInput) error {
	if in.Title != nil {
		a.Title = clip(*in.Title, 120)
	}
	if a.Title == "" {
		return errors.New("title is required")
	}
	if in.Notes != nil {
		a.Notes = clip(*in.Notes, 2000)
	}
	if in.Location != nil {
		a.Location = clip(*in.Location, 200)
	}
	if in.Color != nil {
		if !slices.Contains(activityColors, *in.Color) {
			return errors.New("unknown color")
		}
		a.Color = *in.Color
	}
	if in.AllDay != nil {
		a.AllDay = *in.AllDay
	}
	if in.StartDate != nil {
		a.StartDate = *in.StartDate
	}
	if in.StartTime != nil {
		a.StartTime = *in.StartTime
	}
	if in.EndDate != nil {
		a.EndDate = *in.EndDate
	}
	if in.EndTime != nil {
		a.EndTime = *in.EndTime
	}
	start, err := parseDay(a.StartDate)
	if err != nil {
		return errors.New("start_date must be YYYY-MM-DD")
	}
	if in.StartDate != nil && in.EndDate == nil && a.EndDate < a.StartDate {
		a.EndDate = a.StartDate // moved the start past the old end: keep a one-day event
	}
	if a.EndDate == "" {
		a.EndDate = a.StartDate
	}
	end, err := parseDay(a.EndDate)
	if err != nil {
		return errors.New("end_date must be YYYY-MM-DD")
	}
	if end.Before(start) {
		return errors.New("the activity can't end before it starts")
	}
	if end.Sub(start) > maxActivitySpanDays*24*time.Hour {
		return fmt.Errorf("an activity can span at most %d days", maxActivitySpanDays)
	}
	if a.AllDay {
		a.StartTime, a.EndTime = "00:00", "23:59"
	} else {
		sh, sm, err := parseHHMM(a.StartTime)
		if err != nil {
			return errors.New("start_time must be HH:MM")
		}
		eh, em, err := parseHHMM(a.EndTime)
		if err != nil {
			return errors.New("end_time must be HH:MM")
		}
		if a.EndDate == a.StartDate && eh*60+em < sh*60+sm {
			return errors.New("the activity can't end before it starts")
		}
	}
	if in.Repeat != nil {
		if !validRepeatKinds[*in.Repeat] {
			return errors.New("repeat must be none, daily, weekly, monthly or yearly")
		}
		a.Repeat = *in.Repeat
	}
	if in.Interval != nil {
		a.Interval = *in.Interval
	}
	a.Interval = min(max(a.Interval, 1), 99)
	if in.Weekdays != nil {
		wds := []int{}
		for _, d := range *in.Weekdays {
			if d < 0 || d > 6 {
				return errors.New("weekdays must be 0 (Sunday) to 6 (Saturday)")
			}
			if !slices.Contains(wds, d) {
				wds = append(wds, d)
			}
		}
		sort.Ints(wds)
		a.Weekdays = wds
	}
	if a.Repeat != "weekly" {
		a.Weekdays = []int{}
	}
	if in.UntilDate != nil {
		if *in.UntilDate == "" {
			a.UntilDate = nil
		} else {
			if _, err := parseDay(*in.UntilDate); err != nil {
				return errors.New("until_date must be YYYY-MM-DD")
			}
			v := *in.UntilDate
			a.UntilDate = &v
		}
	}
	if a.Repeat == "none" {
		a.UntilDate = nil
	}
	if a.UntilDate != nil && *a.UntilDate < a.StartDate {
		return errors.New("until_date can't be before the start")
	}
	if in.AttendeeIDs != nil {
		a.AttendeeIDs = slices.Clone(*in.AttendeeIDs)
	}
	if a.AttendeeIDs == nil {
		a.AttendeeIDs = []int64{}
	}
	return nil
}

func defaultActivity() activityRow {
	return activityRow{Color: "indigo", StartTime: "09:00", EndTime: "10:00", Repeat: "none", Interval: 1, Weekdays: []int{}, AttendeeIDs: []int64{}}
}

// checkAttendees: every id must be a member of the household.
func (app *App) checkAttendees(ctx context.Context, aid int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	var n int
	if err := app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM members WHERE account_id=$1 AND id = ANY($2)", aid, ids).Scan(&n); err != nil {
		return err
	}
	if n != len(slices.Compact(slices.Sorted(slices.Values(ids)))) {
		return errors.New("attendees must be members of this family")
	}
	return nil
}

// ---------- handlers ----------

func (app *App) handleCreateActivity(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	var in activityInput
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	a := defaultActivity()
	if err := applyActivityInput(&a, in); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := r.Context()
	if err := app.checkAttendees(ctx, m.AccountID, a.AttendeeIDs); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	var creator *int64
	if m.ID != 0 {
		creator = &m.ID
	}
	err := app.DB.QueryRow(ctx, `INSERT INTO activities (account_id, title, notes, location, color, all_day, start_date, start_time,
	    end_date, end_time, repeat_kind, repeat_interval, weekdays, until_date, attendee_ids, created_by)
	  VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING id`,
		m.AccountID, a.Title, a.Notes, a.Location, a.Color, a.AllDay, a.StartDate, a.StartTime, a.EndDate, a.EndTime,
		a.Repeat, a.Interval, a.Weekdays, a.UntilDate, a.AttendeeIDs, creator).Scan(&a.ID)
	if err != nil {
		dbError(w, err)
		return
	}
	a.CreatedBy = creator
	writeJSON(w, http.StatusCreated, a)
}

func (app *App) loadActivity(ctx context.Context, aid, id int64) (activityRow, error) {
	return scanActivity(app.DB.QueryRow(ctx, "SELECT "+activityCols+" FROM activities WHERE id=$1 AND account_id=$2", id, aid))
}

func (app *App) handleGetActivity(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	a, err := app.loadActivity(r.Context(), m.AccountID, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !activityVisibleTo(a, m)) {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (app *App) handleUpdateActivity(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in activityInput
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx := r.Context()
	a, err := app.loadActivity(ctx, m.AccountID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if err := applyActivityInput(&a, in); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := app.checkAttendees(ctx, m.AccountID, a.AttendeeIDs); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err = app.DB.Exec(ctx, `UPDATE activities SET title=$3, notes=$4, location=$5, color=$6, all_day=$7, start_date=$8, start_time=$9,
	    end_date=$10, end_time=$11, repeat_kind=$12, repeat_interval=$13, weekdays=$14, until_date=$15, attendee_ids=$16, updated_at=now()
	  WHERE id=$1 AND account_id=$2`,
		id, m.AccountID, a.Title, a.Notes, a.Location, a.Color, a.AllDay, a.StartDate, a.StartTime, a.EndDate, a.EndTime,
		a.Repeat, a.Interval, a.Weekdays, a.UntilDate, a.AttendeeIDs)
	if err != nil {
		dbError(w, err)
		return
	}
	// A schedule change makes old single-occurrence skips meaningless.
	if in.StartDate != nil || in.Repeat != nil || in.Weekdays != nil || in.Interval != nil {
		_, _ = app.DB.Exec(ctx, "DELETE FROM activity_skips WHERE activity_id=$1", id)
	}
	writeJSON(w, http.StatusOK, a)
}

func (app *App) handleDeleteActivity(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := r.Context()
	a, err := app.loadActivity(ctx, m.AccountID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if on := r.URL.Query().Get("on"); on != "" && a.Repeat != "none" {
		if _, err := parseDay(on); err != nil {
			jsonError(w, http.StatusBadRequest, "on must be YYYY-MM-DD")
			return
		}
		if _, err := app.DB.Exec(ctx, "INSERT INTO activity_skips (activity_id, skip_date) VALUES ($1,$2) ON CONFLICT DO NOTHING", id, on); err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "skipped": on})
		return
	}
	if _, err := app.DB.Exec(ctx, "DELETE FROM activities WHERE id=$1 AND account_id=$2", id, m.AccountID); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// activityVisibleTo: kids only see whole-family activities and their own.
func activityVisibleTo(a activityRow, m memberInfo) bool {
	if m.Role != roleKid || len(a.AttendeeIDs) == 0 {
		return true
	}
	return slices.Contains(a.AttendeeIDs, m.ID)
}

// parseRange reads from/to (YYYY-MM-DD), defaulting to the household's
// today .. today+30d, and enforces maxCalendarRangeDays.
func (app *App) parseRange(r *http.Request, aid int64) (time.Time, time.Time, error) {
	today := householdToday(r.Context(), app, aid, time.Now().UTC())
	from, to := today, today.AddDate(0, 0, 30)
	var err error
	if v := r.URL.Query().Get("from"); v != "" {
		if from, err = parseDay(v); err != nil {
			return from, to, errors.New("from must be YYYY-MM-DD")
		}
		to = from.AddDate(0, 0, 30)
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if to, err = parseDay(v); err != nil {
			return from, to, errors.New("to must be YYYY-MM-DD")
		}
	}
	if to.Before(from) {
		return from, to, errors.New("to can't be before from")
	}
	if to.Sub(from) > maxCalendarRangeDays*24*time.Hour {
		return from, to, fmt.Errorf("the range can be at most %d days", maxCalendarRangeDays)
	}
	return from, to, nil
}

// loadActivityOccurrences expands the household's activities in [from, to].
func (app *App) loadActivityOccurrences(ctx context.Context, m memberInfo, from, to time.Time) ([]activityOccurrence, error) {
	rows, err := app.DB.Query(ctx, "SELECT "+activityCols+` FROM activities
	  WHERE account_id=$1 AND start_date <= $3 AND (repeat_kind <> 'none' OR end_date >= $2)
	    AND (until_date IS NULL OR until_date >= $2::date - 31)`, m.AccountID, from, to)
	if err != nil {
		return nil, err
	}
	var rules []activityRow
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		if activityVisibleTo(a, m) {
			rules = append(rules, a)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	skips := map[int64]map[string]bool{}
	if len(rules) > 0 {
		ids := make([]int64, len(rules))
		for i, a := range rules {
			ids[i] = a.ID
		}
		srows, err := app.DB.Query(ctx, "SELECT activity_id, skip_date FROM activity_skips WHERE activity_id = ANY($1)", ids)
		if err != nil {
			return nil, err
		}
		for srows.Next() {
			var id int64
			var d time.Time
			if err := srows.Scan(&id, &d); err != nil {
				srows.Close()
				return nil, err
			}
			if skips[id] == nil {
				skips[id] = map[string]bool{}
			}
			skips[id][d.Format("2006-01-02")] = true
		}
		srows.Close()
	}
	names, err := app.memberNames(ctx, m.AccountID)
	if err != nil {
		return nil, err
	}
	return expandActivities(rules, skips, from, to, names), nil
}

func (app *App) handleListActivities(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	from, to, err := app.parseRange(r, m.AccountID)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := app.loadActivityOccurrences(r.Context(), m, from, to)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02"), "items": items})
}

// GET /api/calendar: activities and reminder occurrences for a date range,
// for the month/week/day views (web and phone).
func (app *App) handleCalendar(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	aid := m.AccountID
	from, to, err := app.parseRange(r, aid)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	include := r.URL.Query().Get("include")
	wantActs := include == "" || strings.Contains(include, "activities")
	wantRems := include == "" || strings.Contains(include, "reminders")
	loc := householdLocation(ctx, app, aid)
	out := map[string]any{
		"timezone": loc.String(), "today": householdToday(ctx, app, aid, time.Now().UTC()).Format("2006-01-02"),
		"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02"),
		"activities": []activityOccurrence{}, "reminders": []occurrenceRow{},
	}
	if wantActs {
		acts, err := app.loadActivityOccurrences(ctx, m, from, to)
		if err != nil {
			dbError(w, err)
			return
		}
		out["activities"] = acts
	}
	if wantRems {
		now := time.Now().UTC()
		startUTC := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc).UTC()
		endUTC := time.Date(to.Year(), to.Month(), to.Day()+1, 0, 0, 0, 0, loc).UTC()
		if horizon := endUTC.Sub(now); horizon > 0 {
			if _, err := app.materialize(ctx, aid, now, min(horizon, 130*24*time.Hour)); err != nil {
				dbError(w, err)
				return
			}
		}
		occ, err := app.queryOccurrences(ctx, aid, m,
			`(o.due_at >= $2 AND o.due_at < $3) OR (o.status='snoozed' AND o.snoozed_until >= $2 AND o.snoozed_until < $3)`, startUTC, endUTC)
		if err != nil {
			dbError(w, err)
			return
		}
		if occ == nil {
			occ = []occurrenceRow{}
		}
		out["reminders"] = occ
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// ---------- Today / widget helpers ----------

// activityTimeLabel: "09:00-10:00" or "All day" for one occurrence on `day`.
func activityTimeLabel(o activityOccurrence, day string) string {
	if o.AllDay {
		return "All day"
	}
	switch {
	case o.OccursOn == o.OccursUntil:
		return o.StartTime + "–" + o.EndTime
	case day == o.OccursOn:
		return "from " + o.StartTime
	case day == o.OccursUntil:
		return "until " + o.EndTime
	}
	return "All day"
}

func activityItem(o activityOccurrence, day string) TodayItem {
	detail := activityTimeLabel(o, day)
	if o.Location != "" {
		detail += " · " + o.Location
	}
	return TodayItem{Kind: "activity", ID: o.ID, Title: o.Title, Detail: detail, Link: "/calendar"}
}

// activitiesOn: occurrences touching `day` (UTC-midnight household-local date).
func (app *App) activitiesOn(ctx context.Context, m memberInfo, day time.Time, days int) ([]activityOccurrence, error) {
	return app.loadActivityOccurrences(ctx, m, day, day.AddDate(0, 0, max(days, 1)-1))
}

func activitiesTodayProvider(ctx context.Context, app *App, aid int64, m memberInfo, day time.Time) (*TodaySection, error) {
	m.AccountID = aid
	occ, err := app.activitiesOn(ctx, m, day, 1)
	if err != nil || len(occ) == 0 {
		return nil, err
	}
	d := day.Format("2006-01-02")
	sec := &TodaySection{Key: "activities", Title: "Activities", Items: []TodayItem{}}
	for _, o := range occ {
		sec.Items = append(sec.Items, activityItem(o, d))
	}
	sec.Summary = fmt.Sprintf("%d activities today", len(occ))
	if len(occ) == 1 {
		sec.Summary = "1 activity today"
	}
	return sec, nil
}
