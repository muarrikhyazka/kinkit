package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// recurring transactions
//
// A rule (recurring_transactions row) generates spending rows on its
// schedule. Generation is idempotent: each occurrence is inserted with
// client_id 'recurring:<rule id>:<YYYY-MM-DD>' and the existing
// UNIQUE(account_id, client_id) on spending makes re-runs no-ops. It runs
// right after a rule is created/edited and hourly from the money worker
// (money.go), catching up every missed occurrence since last_generated_date.
// All dates are UTC calendar dates.

const isoDateLayout = "2006-01-02"

// maxRecurringCatchUp bounds one generation pass for one rule (a weekly rule
// started years in the past should not insert thousands of rows at once).
const maxRecurringCatchUp = 1000

type recurRule struct {
	Frequency  string // monthly | weekly | yearly
	DayOfMonth int    // 1..31, monthly/yearly (clamped to the month's last day)
	Weekday    int    // 0=Sunday..6=Saturday, weekly
	Start      time.Time
	End        *time.Time
}

// moneyUTCDate truncates t to its UTC calendar date.
func moneyUTCDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// clampDay returns y-m-day, with day clamped into the month (31 -> 30/29/28).
func clampDay(y int, m time.Month, day int) time.Time {
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > last {
		day = last
	}
	if day < 1 {
		day = 1
	}
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

// recurringOccurrences is the pure date math: every occurrence d of rule
// with from <= d <= to (inclusive, UTC dates), also bounded by the rule's
// start/end dates. Yearly rules recur on the start date's month.
func recurringOccurrences(r recurRule, from, to time.Time) []time.Time {
	from, to = moneyUTCDate(from), moneyUTCDate(to)
	start := moneyUTCDate(r.Start)
	if from.Before(start) {
		from = start
	}
	if r.End != nil {
		if e := moneyUTCDate(*r.End); e.Before(to) {
			to = e
		}
	}
	if to.Before(from) {
		return nil
	}
	out := []time.Time{}
	add := func(d time.Time) bool {
		if !d.Before(from) && !d.After(to) {
			out = append(out, d)
		}
		return len(out) < maxRecurringCatchUp
	}
	switch r.Frequency {
	case "weekly":
		wd := ((r.Weekday % 7) + 7) % 7
		d := from
		for int(d.Weekday()) != wd {
			d = d.AddDate(0, 0, 1)
		}
		for ; !d.After(to); d = d.AddDate(0, 0, 7) {
			if !add(d) {
				break
			}
		}
	case "monthly":
		y, m := from.Year(), from.Month()
		for {
			d := clampDay(y, m, r.DayOfMonth)
			if d.After(to) || !add(d) {
				break
			}
			if m++; m > 12 {
				m, y = 1, y+1
			}
		}
	case "yearly":
		for y := from.Year(); ; y++ {
			d := clampDay(y, start.Month(), r.DayOfMonth)
			if d.After(to) || !add(d) {
				break
			}
		}
	}
	return out
}

// nextRecurringDate is the first occurrence on or after `from`, or nil if
// the rule has ended.
func nextRecurringDate(r recurRule, from time.Time) *time.Time {
	from = moneyUTCDate(from)
	occ := recurringOccurrences(r, from, from.AddDate(1, 1, 0))
	if len(occ) == 0 {
		return nil
	}
	return &occ[0]
}

// pendingRecurringRange is the [from, to] window a generation pass covers:
// from the day after the last generated date (or the start), up to today.
func pendingRecurringRange(start time.Time, lastGenerated *time.Time, today time.Time) (time.Time, time.Time) {
	from := moneyUTCDate(start)
	if lastGenerated != nil {
		if next := moneyUTCDate(*lastGenerated).AddDate(0, 0, 1); next.After(from) {
			from = next
		}
	}
	return from, moneyUTCDate(today)
}

type recurringRow struct {
	ID                int64     `json:"id"`
	Description       string    `json:"description"`
	CategoryID        *int64    `json:"category_id"`
	CategoryName      *string   `json:"category_name"`
	Amount            int64     `json:"amount"`
	Frequency         string    `json:"frequency"`
	DayOfMonth        *int      `json:"day_of_month"`
	Weekday           *int      `json:"weekday"`
	StartDate         string    `json:"start_date"`
	EndDate           *string   `json:"end_date"`
	LastGeneratedDate *string   `json:"last_generated_date"`
	Active            bool      `json:"active"`
	NextDate          *string   `json:"next_date"`
	CreatedAt         time.Time `json:"created_at"`
}

// recurringInput is the create/patch body. PATCH decodes the body over the
// existing values, so only the fields sent change.
type recurringInput struct {
	Description string  `json:"description"`
	CategoryID  *int64  `json:"category_id"`
	Amount      int64   `json:"amount"`
	Frequency   string  `json:"frequency"`
	DayOfMonth  *int    `json:"day_of_month"`
	Weekday     *int    `json:"weekday"`
	StartDate   string  `json:"start_date"`
	EndDate     *string `json:"end_date"`
	Active      *bool   `json:"active"`
}

func fmtDatePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(isoDateLayout)
	return &s
}

func parseDatePtr(s *string) (*time.Time, error) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, nil
	}
	t, err := time.Parse(isoDateLayout, strings.TrimSpace(*s))
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// normalize validates in and fills defaults; returns the parsed dates.
func (in *recurringInput) normalize(ctx context.Context, app *App, aid int64) (start time.Time, end *time.Time, err error) {
	in.Description = strings.TrimSpace(in.Description)
	if in.Description == "" {
		return start, nil, fmt.Errorf("description is required")
	}
	if in.Amount <= 0 {
		return start, nil, fmt.Errorf("amount must be positive")
	}
	if in.Frequency == "" {
		in.Frequency = "monthly"
	}
	if in.StartDate == "" {
		in.StartDate = time.Now().UTC().Format(isoDateLayout)
	}
	if start, err = time.Parse(isoDateLayout, in.StartDate); err != nil {
		return start, nil, fmt.Errorf("invalid start_date (want YYYY-MM-DD)")
	}
	if end, err = parseDatePtr(in.EndDate); err != nil {
		return start, nil, fmt.Errorf("invalid end_date (want YYYY-MM-DD)")
	}
	if end != nil && end.Before(start) {
		return start, nil, fmt.Errorf("end_date is before start_date")
	}
	switch in.Frequency {
	case "monthly", "yearly":
		if in.DayOfMonth == nil {
			d := start.Day()
			in.DayOfMonth = &d
		}
		if *in.DayOfMonth < 1 || *in.DayOfMonth > 31 {
			return start, nil, fmt.Errorf("day_of_month must be 1-31")
		}
		in.Weekday = nil
	case "weekly":
		if in.Weekday == nil {
			wd := int(start.Weekday())
			in.Weekday = &wd
		}
		if *in.Weekday < 0 || *in.Weekday > 6 {
			return start, nil, fmt.Errorf("weekday must be 0 (Sunday) - 6 (Saturday)")
		}
		in.DayOfMonth = nil
	default:
		return start, nil, fmt.Errorf("frequency must be monthly, weekly or yearly")
	}
	if in.CategoryID != nil {
		var ok bool
		if err := app.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM categories WHERE id=$1 AND account_id=$2)", *in.CategoryID, aid).Scan(&ok); err != nil || !ok {
			return start, nil, fmt.Errorf("unknown category")
		}
	}
	return start, end, nil
}

func ruleFromRow(freq string, dom, wd *int, start time.Time, end *time.Time) recurRule {
	r := recurRule{Frequency: freq, Start: start, End: end}
	if dom != nil {
		r.DayOfMonth = *dom
	}
	if wd != nil {
		r.Weekday = *wd
	}
	return r
}

const recurringSelect = `SELECT r.id, r.description, r.category_id, c.name, r.amount, r.frequency, r.day_of_month, r.weekday,
       r.start_date, r.end_date, r.last_generated_date, r.active, r.created_at
  FROM recurring_transactions r LEFT JOIN categories c ON c.id = r.category_id`

func (app *App) loadRecurring(ctx context.Context, where string, args ...any) ([]recurringRow, error) {
	rows, err := app.DB.Query(ctx, recurringSelect+" "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	today := time.Now().UTC()
	out := []recurringRow{}
	for rows.Next() {
		var rr recurringRow
		var start time.Time
		var end, last *time.Time
		if err := rows.Scan(&rr.ID, &rr.Description, &rr.CategoryID, &rr.CategoryName, &rr.Amount, &rr.Frequency,
			&rr.DayOfMonth, &rr.Weekday, &start, &end, &last, &rr.Active, &rr.CreatedAt); err != nil {
			return nil, err
		}
		rr.StartDate = start.Format(isoDateLayout)
		rr.EndDate = fmtDatePtr(end)
		rr.LastGeneratedDate = fmtDatePtr(last)
		if rr.Active {
			from := moneyUTCDate(today)
			if last != nil && !moneyUTCDate(*last).Before(from) {
				from = moneyUTCDate(*last).AddDate(0, 0, 1)
			}
			rr.NextDate = fmtDatePtr(nextRecurringDate(ruleFromRow(rr.Frequency, rr.DayOfMonth, rr.Weekday, start, end), from))
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

// generateRecurring inserts every due occurrence (up to today, UTC) for the
// active rules matching aid/ruleID (0 = any) and returns how many spending
// rows were created. Safe to run concurrently / repeatedly.
func (app *App) generateRecurring(ctx context.Context, aid, ruleID int64, today time.Time) (int, error) {
	type due struct {
		id, aid       int64
		desc          string
		catID         *int64
		catName       *string
		amount        int64
		rule          recurRule
		lastGenerated *time.Time
	}
	rows, err := app.DB.Query(ctx,
		`SELECT r.id, r.account_id, r.description, r.category_id, c.name, r.amount, r.frequency, r.day_of_month, r.weekday,
		        r.start_date, r.end_date, r.last_generated_date
		   FROM recurring_transactions r LEFT JOIN categories c ON c.id = r.category_id
		  WHERE r.active AND ($1::bigint = 0 OR r.account_id = $1) AND ($2::bigint = 0 OR r.id = $2)
		    AND (r.last_generated_date IS NULL OR r.last_generated_date < $3)`, aid, ruleID, moneyUTCDate(today))
	if err != nil {
		return 0, err
	}
	var list []due
	for rows.Next() {
		var d due
		var freq string
		var dom, wd *int
		var start time.Time
		var end *time.Time
		if err := rows.Scan(&d.id, &d.aid, &d.desc, &d.catID, &d.catName, &d.amount, &freq, &dom, &wd, &start, &end, &d.lastGenerated); err != nil {
			rows.Close()
			return 0, err
		}
		d.rule = ruleFromRow(freq, dom, wd, start, end)
		list = append(list, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	created := 0
	for _, d := range list {
		from, to := pendingRecurringRange(d.rule.Start, d.lastGenerated, today)
		dates := recurringOccurrences(d.rule, from, to)
		scanned := to
		if len(dates) == maxRecurringCatchUp {
			scanned = dates[len(dates)-1] // capped: resume after the last one next pass
		}
		for _, day := range dates {
			tag, err := app.DB.Exec(ctx,
				"INSERT INTO spending (account_id, tx_date, description, category_id, category_name, amount, source, client_id) "+
					"VALUES ($1,$2,$3,$4,$5,$6,'recurring',$7) ON CONFLICT (account_id, client_id) DO NOTHING",
				d.aid, day, d.desc, d.catID, d.catName, d.amount, fmt.Sprintf("recurring:%d:%s", d.id, day.Format(isoDateLayout)))
			if err != nil {
				return created, err
			}
			created += int(tag.RowsAffected())
		}
		if _, err := app.DB.Exec(ctx,
			"UPDATE recurring_transactions SET last_generated_date = GREATEST(COALESCE(last_generated_date, $2), $2) WHERE id=$1",
			d.id, scanned); err != nil {
			return created, err
		}
	}
	return created, nil
}

func (app *App) handleListRecurring(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	list, err := app.loadRecurring(r.Context(), "WHERE r.account_id=$1 ORDER BY r.active DESC, r.id", aid)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (app *App) handleAddRecurring(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var in recurringInput
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx := r.Context()
	start, end, err := in.normalize(ctx, app, aid)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	active := in.Active == nil || *in.Active
	var id int64
	if err := app.DB.QueryRow(ctx,
		"INSERT INTO recurring_transactions (account_id, description, category_id, amount, frequency, day_of_month, weekday, start_date, end_date, active) "+
			"VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id",
		aid, in.Description, in.CategoryID, in.Amount, in.Frequency, in.DayOfMonth, in.Weekday, start, end, active).Scan(&id); err != nil {
		dbError(w, err)
		return
	}
	generated := 0
	if active {
		if generated, err = app.generateRecurring(ctx, aid, id, time.Now().UTC()); err != nil {
			dbError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "generated": generated})
}

func (app *App) handleUpdateRecurring(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := r.Context()
	existing, err := app.loadRecurring(ctx, "WHERE r.account_id=$1 AND r.id=$2", aid, id)
	if err != nil {
		dbError(w, err)
		return
	}
	if len(existing) == 0 {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	cur := existing[0]
	wasActive := cur.Active
	in := recurringInput{
		Description: cur.Description, CategoryID: cur.CategoryID, Amount: cur.Amount, Frequency: cur.Frequency,
		DayOfMonth: cur.DayOfMonth, Weekday: cur.Weekday, StartDate: cur.StartDate, EndDate: cur.EndDate, Active: &wasActive,
	}
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	start, end, err := in.normalize(ctx, app, aid)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	active := in.Active == nil || *in.Active
	// Resuming a paused rule does not backfill the paused period: skip
	// everything before today.
	resume := active && !wasActive
	yesterday := moneyUTCDate(time.Now().UTC()).AddDate(0, 0, -1)
	if _, err := app.DB.Exec(ctx,
		`UPDATE recurring_transactions SET description=$1, category_id=$2, amount=$3, frequency=$4, day_of_month=$5, weekday=$6,
		        start_date=$7, end_date=$8, active=$9,
		        last_generated_date = CASE WHEN $10 THEN GREATEST(COALESCE(last_generated_date, $11), $11) ELSE last_generated_date END
		  WHERE id=$12 AND account_id=$13`,
		in.Description, in.CategoryID, in.Amount, in.Frequency, in.DayOfMonth, in.Weekday, start, end, active,
		resume, yesterday, id, aid); err != nil {
		dbError(w, err)
		return
	}
	generated := 0
	if active {
		if generated, err = app.generateRecurring(ctx, aid, id, time.Now().UTC()); err != nil {
			dbError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "generated": generated})
}

// handleDeleteRecurring removes the rule; spending it already generated stays.
func (app *App) handleDeleteRecurring(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := app.DB.Exec(r.Context(), "DELETE FROM recurring_transactions WHERE id=$1 AND account_id=$2", id, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
