package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// RegisterAdminRoutes wires the super-admin endpoints (/admin/accounts,
// /admin/stats, /admin/reset-demo-data). Integrations CRUD lives in
// integrations.go, bot-token endpoints in bot.go and service-token internal
// endpoints in internal.go.
func RegisterAdminRoutes(r chi.Router, app *App) {
	r.Get("/admin/stats", requireAdmin(app, adminStats))
	r.Get("/admin/accounts", requireAdmin(app, adminListAccounts))
	r.Patch("/admin/accounts/{id}", requireAdmin(app, adminUpdateAccount))
	r.Delete("/admin/accounts/{id}", requireAdmin(app, adminDeleteAccount))
	r.Post("/admin/reset-demo-data", requireAdmin(app, adminResetDemoData))
}

func requireAdmin(app *App, h func(http.ResponseWriter, *http.Request, *App, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		aid, ok := currentAccountID(r)
		if !ok {
			jsonError(w, http.StatusUnauthorized, "login required")
			return
		}
		var isAdmin bool
		err := app.DB.QueryRow(context.Background(), "SELECT is_admin FROM users WHERE id=$1", aid).Scan(&isAdmin)
		if err != nil || !isAdmin {
			jsonError(w, http.StatusForbidden, "admin required")
			return
		}
		// Platform admin is an account flag; only the people who run that
		// household (owner/parent) get it, not its kids/viewers.
		if m, ok := currentMember(r); ok && !canManageFamily(m.Role) {
			jsonError(w, http.StatusForbidden, "admin required")
			return
		}
		h(w, r, app, aid)
	}
}

// ---------- Super-admin ----------

func adminStats(w http.ResponseWriter, r *http.Request, app *App, _ int64) {
	ctx := context.Background()
	var total, active, disabled int64
	if err := app.DB.QueryRow(ctx,
		"SELECT COUNT(*), COALESCE(SUM(CASE WHEN is_disabled THEN 0 ELSE 1 END),0), COALESCE(SUM(CASE WHEN is_disabled THEN 1 ELSE 0 END),0) FROM users").
		Scan(&total, &active, &disabled); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var spendCount, spendTotal int64
	if err := app.DB.QueryRow(ctx, "SELECT COUNT(*), COALESCE(SUM(amount),0) FROM spending").
		Scan(&spendCount, &spendTotal); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, err := app.DB.Query(ctx, "SELECT platform, COUNT(*) FROM integrations WHERE enabled=TRUE GROUP BY platform")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	byPlatform := []map[string]any{}
	for rows.Next() {
		var platform string
		var n int64
		if err := rows.Scan(&platform, &n); err != nil {
			rows.Close()
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		byPlatform = append(byPlatform, map[string]any{"platform": platform, "n": n})
	}
	rows.Close()

	rows2, err := app.DB.Query(ctx,
		"SELECT to_char(created_at,'YYYY-MM-DD') AS day, COUNT(*) FROM users GROUP BY day ORDER BY day DESC LIMIT 30")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	signups := []map[string]any{}
	for rows2.Next() {
		var day string
		var n int64
		if err := rows2.Scan(&day, &n); err != nil {
			rows2.Close()
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		signups = append(signups, map[string]any{"day": day, "n": n})
	}
	rows2.Close()

	writeJSON(w, http.StatusOK, map[string]any{
		"accounts":                 map[string]any{"total": total, "active": active, "disabled": disabled},
		"spending":                 map[string]any{"count": spendCount, "total": spendTotal},
		"integrations_by_platform": byPlatform,
		"signups_by_day":           signups,
	})
}

type adminAccountRow struct {
	ID               int64     `json:"id"`
	Email            string    `json:"email"`
	DisplayName      *string   `json:"display_name"`
	Currency         string    `json:"currency"`
	IsAdmin          bool      `json:"is_admin"`
	IsDisabled       bool      `json:"is_disabled"`
	CreatedAt        time.Time `json:"created_at"`
	SpendingCount    int64     `json:"spending_count"`
	IntegrationCount int64     `json:"integration_count"`
	MemberCount      int64     `json:"member_count"`
}

func adminListAccounts(w http.ResponseWriter, r *http.Request, app *App, _ int64) {
	rows, err := app.DB.Query(context.Background(), `
		SELECT u.id, u.email, u.display_name, u.currency, u.is_admin, u.is_disabled, u.created_at,
		       (SELECT COUNT(*) FROM spending WHERE account_id=u.id) AS spending_count,
		       (SELECT COUNT(*) FROM integrations WHERE account_id=u.id) AS integration_count,
		       (SELECT COUNT(*) FROM members WHERE account_id=u.id) AS member_count
		FROM users u ORDER BY u.id`)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []adminAccountRow{}
	for rows.Next() {
		var a adminAccountRow
		if err := rows.Scan(&a.ID, &a.Email, &a.DisplayName, &a.Currency, &a.IsAdmin, &a.IsDisabled, &a.CreatedAt,
			&a.SpendingCount, &a.IntegrationCount, &a.MemberCount); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, a)
	}
	writeJSON(w, http.StatusOK, out)
}

func adminUpdateAccount(w http.ResponseWriter, r *http.Request, app *App, _ int64) {
	uid, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var d map[string]any
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	var sets []string
	var params []any
	n := 1
	if v, present := d["is_disabled"]; present {
		sets = append(sets, "is_disabled=$"+strconv.Itoa(n))
		params = append(params, v)
		n++
	}
	if v, present := d["is_admin"]; present {
		sets = append(sets, "is_admin=$"+strconv.Itoa(n))
		params = append(params, v)
		n++
	}
	if len(sets) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	params = append(params, uid)
	q := "UPDATE users SET " + strings.Join(sets, ",") + " WHERE id=$" + strconv.Itoa(n)
	if _, err := app.DB.Exec(context.Background(), q, params...); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func adminDeleteAccount(w http.ResponseWriter, r *http.Request, app *App, selfID int64) {
	uid, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if uid == selfID {
		jsonError(w, http.StatusBadRequest, "cannot delete your own admin account")
		return
	}
	ctx := context.Background()
	// members/member_points/point_events/member_badges/challenge_completions
	// cascade from users; user_points does not, so delete it explicitly.
	tables := []string{"user_points", "spending", "budget_income", "budget_expense", "savings_list", "assets", "asset_target",
		"accounts", "categories", "income_sources", "integrations", "reminder_schedules",
		"mandatory_ingredients", "category_keywords"}
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback(ctx)
	for _, t := range tables {
		if _, err := tx.Exec(ctx, "DELETE FROM "+t+" WHERE account_id=$1", uid); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if _, err := tx.Exec(ctx, "DELETE FROM password_resets WHERE user_id=$1", uid); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := tx.Exec(ctx, "DELETE FROM users WHERE id=$1", uid); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(ctx); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// adminResetDemoData: Python's version calls into seed_demo.py (a
// Python-only random-data generator) to wipe + reseed the hardcoded
// demo1/demo2 accounts. That generator isn't ported here - out of scope
// for this Go rewrite - so this stubs the endpoint as 501 rather than
// silently no-op or reimplementing a random demo-data generator.
func adminResetDemoData(w http.ResponseWriter, r *http.Request, app *App, _ int64) {
	jsonError(w, http.StatusNotImplemented, "reset-demo-data not implemented in Go rewrite: depends on Python seed_demo.py, out of scope")
}
