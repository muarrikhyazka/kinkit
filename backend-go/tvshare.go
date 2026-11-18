package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---------- TV / car display share links ----------
// A tv_token lets an always-on screen show the current month's summary at
// /tv?k=<token> without logging in. Read-only, revocable, and it only ever
// exposes what TVView renders (month report + 6-month trend).

const maxTVTokens = 20

type tvTokenRow struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Token      string  `json:"token"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
	Revoked    bool    `json:"revoked"`
}

// GET /api/tv-tokens -> [tvTokenRow] (newest first)
func (app *App) handleListTVTokens(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	rows, err := app.DB.Query(r.Context(),
		"SELECT id, name, token, created_at, last_used_at, revoked FROM tv_tokens WHERE account_id=$1 ORDER BY revoked, id DESC", aid)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()
	out := []tvTokenRow{}
	for rows.Next() {
		var t tvTokenRow
		var created time.Time
		var used *time.Time
		if err := rows.Scan(&t.ID, &t.Name, &t.Token, &created, &used, &t.Revoked); err != nil {
			dbError(w, err)
			return
		}
		t.CreatedAt = created.UTC().Format(time.RFC3339)
		if used != nil {
			s := used.UTC().Format(time.RFC3339)
			t.LastUsedAt = &s
		}
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/tv-tokens {"name": "Living room TV"} -> tvTokenRow
func (app *App) handleCreateTVToken(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var d struct {
		Name string `json:"name"`
	}
	_ = decodeJSON(r, &d)
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		d.Name = "TV display"
	}
	if len(d.Name) > 60 {
		d.Name = d.Name[:60]
	}
	ctx := r.Context()
	var active int
	if err := app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM tv_tokens WHERE account_id=$1 AND NOT revoked", aid).Scan(&active); err != nil {
		dbError(w, err)
		return
	}
	if active >= maxTVTokens {
		jsonError(w, http.StatusBadRequest, "too many active TV links - revoke one first")
		return
	}
	t := tvTokenRow{Name: d.Name, Token: randomToken()}
	var created time.Time
	if err := app.DB.QueryRow(ctx,
		"INSERT INTO tv_tokens (account_id, token, name) VALUES ($1,$2,$3) RETURNING id, created_at",
		aid, t.Token, t.Name).Scan(&t.ID, &created); err != nil {
		dbError(w, err)
		return
	}
	t.CreatedAt = created.UTC().Format(time.RFC3339)
	writeJSON(w, http.StatusOK, t)
}

// DELETE /api/tv-tokens/{id} - revokes (kept for the audit trail).
func (app *App) handleRevokeTVToken(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	tag, err := app.DB.Exec(r.Context(), "UPDATE tv_tokens SET revoked=TRUE WHERE id=$1 AND account_id=$2", id, aid)
	if err != nil {
		dbError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// GET /api/tv/data?k=<token> - no session. Returns
//
//	{year, month, currency, display_name,
//	 report: {total_income, budgeted_expenses, savings, left_for_budgeting, expenses_list:[...]},
//	 trend: [{year, month, income, expense}] (6 months ending this UTC month)}
func (app *App) handleTVData(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("k"))
	if token == "" || len(token) > 200 {
		jsonError(w, http.StatusUnauthorized, "invalid or revoked TV link")
		return
	}
	ctx := r.Context()
	var aid int64
	var currency string
	var display *string
	err := app.DB.QueryRow(ctx,
		`UPDATE tv_tokens t SET last_used_at=now() FROM users u
		 WHERE t.token=$1 AND NOT t.revoked AND u.id=t.account_id AND NOT u.is_disabled
		 RETURNING t.account_id, u.currency, u.display_name`, token).Scan(&aid, &currency, &display)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, http.StatusUnauthorized, "invalid or revoked TV link")
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	now := time.Now().UTC()
	year, month := now.Year(), int(now.Month())
	report, err := app.tvMonthReport(ctx, aid, year, month)
	if err != nil {
		dbError(w, err)
		return
	}
	trend, err := app.tvTrend(ctx, aid, year, month, 6)
	if err != nil {
		dbError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"year": year, "month": month, "currency": currency, "display_name": display,
		"report": report, "trend": trend,
	})
}

type tvExpense struct {
	CategoryName string `json:"category_name"`
	MainCategory string `json:"main_category"`
	Allocation   int64  `json:"allocation"`
	Realization  int64  `json:"realization"`
}

// tvMonthReport mirrors the shape of GET /api/report/{y}/{m} (the fields TVView uses).
func (app *App) tvMonthReport(ctx context.Context, aid int64, year, month int) (map[string]any, error) {
	var income, budgeted, savings int64
	err := app.DB.QueryRow(ctx, `SELECT
		(SELECT COALESCE(SUM(amount),0) FROM budget_income WHERE account_id=$1 AND year=$2 AND month=$3),
		(SELECT COALESCE(SUM(amount),0) FROM budget_expense WHERE account_id=$1 AND year=$2 AND month=$3),
		(SELECT COALESCE(SUM(amount),0) FROM savings_list WHERE account_id=$1 AND year=$2 AND month=$3)`,
		aid, year, month).Scan(&income, &budgeted, &savings)
	if err != nil {
		return nil, err
	}
	rows, err := app.DB.Query(ctx, `SELECT c.name, c.main_category, COALESCE(be.amount,0),
		COALESCE((SELECT SUM(sp.amount) FROM spending sp WHERE sp.category_id=c.id AND sp.account_id=$1
		          AND EXTRACT(YEAR FROM sp.tx_date)::int=$2 AND EXTRACT(MONTH FROM sp.tx_date)::int=$3),0)
		FROM categories c
		LEFT JOIN budget_expense be ON be.category_id=c.id AND be.account_id=$1 AND be.year=$2 AND be.month=$3
		WHERE c.account_id=$1 ORDER BY c.id`, aid, year, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []tvExpense{}
	for rows.Next() {
		var e tvExpense
		if err := rows.Scan(&e.CategoryName, &e.MainCategory, &e.Allocation, &e.Realization); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return map[string]any{
		"total_income": income, "budgeted_expenses": budgeted, "savings": savings,
		"left_for_budgeting": income - budgeted - savings, "expenses_list": list,
	}, rows.Err()
}

type tvTrendRow struct {
	Year    int   `json:"year"`
	Month   int   `json:"month"`
	Income  int64 `json:"income"`
	Expense int64 `json:"expense"`
}

// tvTrend mirrors GET /api/trend: oldest first, `count` months ending at year/month.
func (app *App) tvTrend(ctx context.Context, aid int64, year, month, count int) ([]tvTrendRow, error) {
	out := make([]tvTrendRow, count)
	y, m := year, month
	for i := count - 1; i >= 0; i-- {
		row := tvTrendRow{Year: y, Month: m}
		err := app.DB.QueryRow(ctx, `SELECT
			(SELECT COALESCE(SUM(amount),0) FROM budget_income WHERE account_id=$1 AND year=$2 AND month=$3),
			(SELECT COALESCE(SUM(amount),0) FROM spending WHERE account_id=$1
			   AND EXTRACT(YEAR FROM tx_date)::int=$2 AND EXTRACT(MONTH FROM tx_date)::int=$3)`,
			aid, y, m).Scan(&row.Income, &row.Expense)
		if err != nil {
			return nil, err
		}
		out[i] = row
		m--
		if m == 0 {
			m, y = 12, y-1
		}
	}
	return out, nil
}

// publicCORS lets the packaged TV viewers (tv/web on Tizen/webOS, served from
// file:// or another origin) read the two public, read-only GET endpoints.
// No credentials are allowed, so session cookies are never exposed; the TV
// token travels in ?k=. Plain GETs need no preflight.
func publicCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	})
}
