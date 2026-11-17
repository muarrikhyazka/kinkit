package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ---------- actual income entries ----------
//
// budget_income is the *plan*; income_entries is what actually came in.
// Reports expose it as income_actual next to the planned total_income.

type incomeEntryRow struct {
	ID          int64     `json:"id"`
	SourceID    *int64    `json:"source_id"`
	SourceName  *string   `json:"source_name"`
	TxDate      string    `json:"tx_date"`
	Description string    `json:"description"`
	Amount      int64     `json:"amount"`
	ClientID    *string   `json:"client_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type incomeEntryInput struct {
	SourceID    *int64  `json:"source_id"`
	TxDate      string  `json:"tx_date"`
	Description string  `json:"description"`
	Amount      int64   `json:"amount"`
	ClientID    *string `json:"client_id"`
}

func (in *incomeEntryInput) normalize(ctx context.Context, app *App, aid int64) (time.Time, error) {
	in.Description = strings.TrimSpace(in.Description)
	if in.Amount <= 0 {
		return time.Time{}, fmt.Errorf("amount must be positive")
	}
	if in.TxDate == "" {
		in.TxDate = time.Now().UTC().Format(isoDateLayout)
	}
	d, err := time.Parse(isoDateLayout, in.TxDate)
	if err != nil {
		return d, fmt.Errorf("invalid tx_date (want YYYY-MM-DD)")
	}
	if in.SourceID != nil {
		var ok bool
		if err := app.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM income_sources WHERE id=$1 AND account_id=$2)", *in.SourceID, aid).Scan(&ok); err != nil || !ok {
			return d, fmt.Errorf("unknown income source")
		}
	}
	if in.ClientID != nil && strings.TrimSpace(*in.ClientID) == "" {
		in.ClientID = nil
	}
	return d, nil
}

const incomeSelect = `SELECT e.id, e.source_id, s.name, e.tx_date, e.description, e.amount, e.client_id, e.created_at
  FROM income_entries e LEFT JOIN income_sources s ON s.id = e.source_id`

func scanIncomeEntries(ctx context.Context, app *App, where string, args ...any) ([]incomeEntryRow, error) {
	rows, err := app.DB.Query(ctx, incomeSelect+" "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []incomeEntryRow{}
	for rows.Next() {
		var e incomeEntryRow
		var d time.Time
		if err := rows.Scan(&e.ID, &e.SourceID, &e.SourceName, &d, &e.Description, &e.Amount, &e.ClientID, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.TxDate = d.Format(isoDateLayout)
		out = append(out, e)
	}
	return out, rows.Err()
}

// monthRange returns [first day, first day of next period) for year/month
// (month 0 = whole year), as UTC dates.
func monthRange(year, month int) (time.Time, time.Time) {
	if month < 1 || month > 12 {
		from := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		return from, from.AddDate(1, 0, 0)
	}
	from := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return from, from.AddDate(0, 1, 0)
}

// GET /income?year=&month= (month optional = whole year; defaults to the
// current UTC month).
func (app *App) handleListIncome(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	year := queryIntDefault(r, "year", now.Year())
	month := queryIntDefault(r, "month", int(now.Month()))
	if r.URL.Query().Get("year") != "" && r.URL.Query().Get("month") == "" {
		month = 0
	}
	from, to := monthRange(year, month)
	ctx := r.Context()
	list, err := scanIncomeEntries(ctx, app, "WHERE e.account_id=$1 AND e.tx_date >= $2 AND e.tx_date < $3 ORDER BY e.tx_date DESC, e.id DESC", aid, from, to)
	if err != nil {
		dbError(w, err)
		return
	}
	type bySource struct {
		SourceID   *int64  `json:"source_id"`
		SourceName *string `json:"source_name"`
		Total      int64   `json:"total"`
	}
	agg := map[string]*bySource{}
	order := []string{}
	var total int64
	for _, e := range list {
		total += e.Amount
		key := "none"
		if e.SourceID != nil {
			key = fmt.Sprint(*e.SourceID)
		}
		if agg[key] == nil {
			agg[key] = &bySource{SourceID: e.SourceID, SourceName: e.SourceName}
			order = append(order, key)
		}
		agg[key].Total += e.Amount
	}
	sources := []bySource{}
	for _, k := range order {
		sources = append(sources, *agg[k])
	}
	var planned int64
	q := "SELECT COALESCE(SUM(amount),0) FROM budget_income WHERE account_id=$1 AND year=$2"
	args := []any{aid, year}
	if month > 0 {
		q += " AND month=$3"
		args = append(args, month)
	}
	if err := app.DB.QueryRow(ctx, q, args...).Scan(&planned); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rows": list, "total": total, "planned": planned, "by_source": sources, "year": year, "month": month,
	})
}

func (app *App) handleAddIncome(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var in incomeEntryInput
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx := r.Context()
	d, err := in.normalize(ctx, app, aid)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.ClientID != nil {
		var existing int64
		if err := app.DB.QueryRow(ctx, "SELECT id FROM income_entries WHERE client_id=$1 AND account_id=$2", *in.ClientID, aid).Scan(&existing); err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": existing, "duplicate": true})
			return
		}
	}
	var id int64
	if err := app.DB.QueryRow(ctx,
		"INSERT INTO income_entries (account_id, source_id, tx_date, description, amount, client_id) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id",
		aid, in.SourceID, d, in.Description, in.Amount, in.ClientID).Scan(&id); err != nil {
		if isUniqueViolation(err) {
			jsonError(w, http.StatusConflict, "duplicate client_id")
			return
		}
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

func (app *App) handleUpdateIncome(w http.ResponseWriter, r *http.Request) {
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
	existing, err := scanIncomeEntries(ctx, app, "WHERE e.account_id=$1 AND e.id=$2", aid, id)
	if err != nil {
		dbError(w, err)
		return
	}
	if len(existing) == 0 {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	cur := existing[0]
	in := incomeEntryInput{SourceID: cur.SourceID, TxDate: cur.TxDate, Description: cur.Description, Amount: cur.Amount}
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	d, err := in.normalize(ctx, app, aid)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := app.DB.Exec(ctx,
		"UPDATE income_entries SET source_id=$1, tx_date=$2, description=$3, amount=$4 WHERE id=$5 AND account_id=$6",
		in.SourceID, d, in.Description, in.Amount, id, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleDeleteIncome(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := app.DB.Exec(r.Context(), "DELETE FROM income_entries WHERE id=$1 AND account_id=$2", id, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
