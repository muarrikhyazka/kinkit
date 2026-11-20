package main

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// RegisterBotRoutes wires bot-token-authenticated endpoints (no session
// cookie): /bot/config, /bot/spending, /bot/spending/recent,
// /bot/spending/search, /bot/spending/<id>/category, /bot/spending/<id>/date.
// More bot endpoints (meal plan, ingredients, keyword learning, feedback)
// live in meal_plan.go and bot_extras.go.
func RegisterBotRoutes(r chi.Router, app *App) {
	r.Get("/bot/config", botAuth(app, botConfig))
	r.Post("/bot/spending", botAuth(app, botAddSpending))
	r.Get("/bot/spending/recent", botAuth(app, botRecentSpending))
	r.Get("/bot/spending/search", botAuth(app, botSearchSpending))
	r.Patch("/bot/spending/{id}/category", botAuth(app, botUpdateCategory))
	r.Patch("/bot/spending/{id}/date", botAuth(app, botUpdateDate))
}

// botAuth resolves the account via X-Bot-Token (matched against
// integrations.bot_token), mirroring Python's _check_bot_token().
func botAuth(app *App, h func(http.ResponseWriter, *http.Request, *App, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Bot-Token")
		if token == "" {
			jsonError(w, http.StatusForbidden, "forbidden")
			return
		}
		var aid int64
		err := app.DB.QueryRow(context.Background(),
			"SELECT account_id FROM integrations WHERE bot_token=$1 AND enabled=TRUE", token).Scan(&aid)
		if err != nil {
			jsonError(w, http.StatusForbidden, "forbidden")
			return
		}
		h(w, r, app, aid)
	}
}

// Bot-token authenticated

func botConfig(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	ctx := context.Background()
	reminders := []map[string]any{}
	rows, err := app.DB.Query(ctx, "SELECT id, account_id, job_name, days, hour, minute, enabled, last_run FROM reminder_schedules WHERE account_id=$1 AND enabled=TRUE", aid)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for rows.Next() {
		var id, accountID, hour, minute int64
		var jobName, days string
		var enabled bool
		var lastRun *string
		if err := rows.Scan(&id, &accountID, &jobName, &days, &hour, &minute, &enabled, &lastRun); err != nil {
			rows.Close()
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		reminders = append(reminders, map[string]any{
			"id": id, "account_id": accountID, "job_name": jobName, "days": days,
			"hour": hour, "minute": minute, "enabled": enabled, "last_run": lastRun,
		})
	}
	rows.Close()

	ingredients := []string{}
	rows2, err := app.DB.Query(ctx, "SELECT name FROM mandatory_ingredients WHERE account_id=$1", aid)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for rows2.Next() {
		var name string
		if err := rows2.Scan(&name); err != nil {
			rows2.Close()
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ingredients = append(ingredients, name)
	}
	rows2.Close()

	keywords := map[string]string{}
	rows3, err := app.DB.Query(ctx, "SELECT keyword, category FROM category_keywords WHERE account_id=$1", aid)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for rows3.Next() {
		var k, c string
		if err := rows3.Scan(&k, &c); err != nil {
			rows3.Close()
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		keywords[k] = c
	}
	rows3.Close()

	categories := []string{}
	categoryList := []map[string]any{} // [{id, name}] for chat category pickers (callback data carries the id)
	rows4, err := app.DB.Query(ctx, "SELECT id, name FROM categories WHERE account_id=$1 ORDER BY id", aid)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for rows4.Next() {
		var id int64
		var name string
		if err := rows4.Scan(&id, &name); err != nil {
			rows4.Close()
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		categories = append(categories, name)
		categoryList = append(categoryList, map[string]any{"id": id, "name": name})
	}
	rows4.Close()

	writeJSON(w, http.StatusOK, map[string]any{
		"reminders":             reminders,
		"mandatory_ingredients": ingredients,
		"category_keywords":     keywords,
		"categories":            categories,
		"category_list":         categoryList,
		// --- reminders ---: bots log spending on the household's local date
		"timezone": householdLocation(ctx, app, aid).String(),
		"today":    householdToday(ctx, app, aid, time.Now()).Format("2006-01-02"),
		// --- end reminders ---
	})
}

func botAddSpending(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	var d struct {
		TxDate      string  `json:"tx_date"`
		Description string  `json:"description"`
		Category    *string `json:"category"`
		CategoryID  *int64  `json:"category_id"` // alternative to category (chat pickers send the id)
		Amount      int64   `json:"amount"`
		Source      *string `json:"source"`
	}
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if _, err := time.Parse("2006-01-02", d.TxDate); err != nil {
		jsonError(w, http.StatusBadRequest, "tx_date must be YYYY-MM-DD")
		return
	}
	ctx := context.Background()
	var catID *int64
	if d.CategoryID != nil {
		var name string
		if err := app.DB.QueryRow(ctx, "SELECT name FROM categories WHERE id=$1 AND account_id=$2", *d.CategoryID, aid).Scan(&name); err != nil {
			jsonError(w, http.StatusBadRequest, "unknown category")
			return
		}
		catID, d.Category = d.CategoryID, &name
	} else if d.Category != nil && *d.Category != "" {
		var id int64
		err := app.DB.QueryRow(ctx, "SELECT id FROM categories WHERE name=$1 AND account_id=$2", *d.Category, aid).Scan(&id)
		if err == nil {
			catID = &id
		}
	}
	source := "telegram"
	if d.Source != nil && *d.Source != "" {
		source = *d.Source
	}
	// attribute the row to the linked household member (if any)
	var memberID *int64
	if m, ok := botActor(ctx, app, r, aid); ok {
		memberID = &m.ID
	}
	var newID int64
	err := app.DB.QueryRow(ctx,
		"INSERT INTO spending (account_id, tx_date, description, category_id, category_name, amount, source, member_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id",
		aid, d.TxDate, d.Description, catID, d.Category, d.Amount, source, memberID).Scan(&newID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	app.awardPointsForLog(ctx, aid)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": newID, "category": d.Category})
}

func botRecentSpending(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	n := 5
	if v := r.URL.Query().Get("n"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			n = min(max(parsed, 1), 50)
		}
	}
	rows, err := app.DB.Query(context.Background(),
		"SELECT id, tx_date, description, category_name, amount FROM spending WHERE account_id=$1 ORDER BY id DESC LIMIT $2", aid, n)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	res, ok := scanSpendingRows(rows)
	if !ok {
		jsonError(w, http.StatusInternalServerError, "scan failed")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func botSearchSpending(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	q := r.URL.Query().Get("q")
	rows, err := app.DB.Query(context.Background(),
		"SELECT id, tx_date, description, category_name, amount FROM spending WHERE account_id=$1 AND description ILIKE $2 ORDER BY id DESC LIMIT 50",
		aid, "%"+q+"%")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	res, ok := scanSpendingRows(rows)
	if !ok {
		jsonError(w, http.StatusInternalServerError, "scan failed")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// scanSpendingRows scans (id, tx_date, description, category_name, amount)
// rows into the [{...}] shape bot_recent_spending/bot_search_spending return.
func scanSpendingRows(rows pgx.Rows) ([]map[string]any, bool) {
	out := []map[string]any{}
	for rows.Next() {
		var id, amount int64
		var txDate time.Time
		var description string
		var categoryName *string
		if err := rows.Scan(&id, &txDate, &description, &categoryName, &amount); err != nil {
			return nil, false
		}
		out = append(out, map[string]any{
			"id": id, "tx_date": txDate.Format("2006-01-02"), "description": description,
			"category_name": categoryName, "amount": amount,
		})
	}
	return out, true
}

func botUpdateCategory(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	sid, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var d struct {
		Category   *string `json:"category"`
		CategoryID *int64  `json:"category_id"` // alternative to category
	}
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx := context.Background()
	var catID *int64
	if d.CategoryID != nil {
		var name string
		if err := app.DB.QueryRow(ctx, "SELECT name FROM categories WHERE id=$1 AND account_id=$2", *d.CategoryID, aid).Scan(&name); err != nil {
			jsonError(w, http.StatusBadRequest, "unknown category")
			return
		}
		catID, d.Category = d.CategoryID, &name
	} else if d.Category != nil {
		var id int64
		if err := app.DB.QueryRow(ctx, "SELECT id FROM categories WHERE name=$1 AND account_id=$2", *d.Category, aid).Scan(&id); err == nil {
			catID = &id
		}
	}
	tag, err := app.DB.Exec(ctx, "UPDATE spending SET category_id=$1, category_name=$2 WHERE id=$3 AND account_id=$4",
		catID, d.Category, sid, aid)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "category": d.Category})
}

func botUpdateDate(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	sid, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var d struct {
		TxDate string `json:"tx_date"`
	}
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if _, err := time.Parse("2006-01-02", d.TxDate); err != nil {
		jsonError(w, http.StatusBadRequest, "tx_date must be YYYY-MM-DD")
		return
	}
	ctx := context.Background()
	var exists int64
	if err := app.DB.QueryRow(ctx, "SELECT id FROM spending WHERE id=$1 AND account_id=$2", sid, aid).Scan(&exists); err != nil {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	if _, err := app.DB.Exec(ctx, "UPDATE spending SET tx_date=$1 WHERE id=$2", d.TxDate, sid); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
