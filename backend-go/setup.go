package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// row types (mirror sqlite dict(r) shapes)

type accountRow struct {
	ID        int64  `json:"id"`
	AccountID int64  `json:"account_id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
}

type categoryRow struct {
	ID           int64  `json:"id"`
	AccountID    int64  `json:"account_id"`
	Name         string `json:"name"`
	MainCategory string `json:"main_category"`
}

type incomeSourceRow struct {
	ID        int64  `json:"id"`
	AccountID int64  `json:"account_id"`
	Name      string `json:"name"`
}

// setup / master lists

func (app *App) handleGetSetup(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	ctx := context.Background()

	accounts := []accountRow{}
	rows, err := app.DB.Query(ctx, "SELECT id, account_id, name, kind FROM accounts WHERE account_id=$1 ORDER BY id", aid)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var a accountRow
		if err := rows.Scan(&a.ID, &a.AccountID, &a.Name, &a.Kind); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		accounts = append(accounts, a)
	}
	rows.Close()

	categories := []categoryRow{}
	rows, err = app.DB.Query(ctx, "SELECT id, account_id, name, main_category FROM categories WHERE account_id=$1 ORDER BY id", aid)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var c categoryRow
		if err := rows.Scan(&c.ID, &c.AccountID, &c.Name, &c.MainCategory); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		categories = append(categories, c)
	}
	rows.Close()

	incomeSources := []incomeSourceRow{}
	rows, err = app.DB.Query(ctx, "SELECT id, account_id, name FROM income_sources WHERE account_id=$1 ORDER BY id", aid)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var s incomeSourceRow
		if err := rows.Scan(&s.ID, &s.AccountID, &s.Name); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		incomeSources = append(incomeSources, s)
	}
	rows.Close()

	writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts, "categories": categories, "income_sources": incomeSources})
}

// categories CRUD

func (app *App) handleAddCategory(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var d struct {
		Name         string `json:"name"`
		MainCategory string `json:"main_category"`
	}
	_ = decodeJSON(r, &d)
	name := strings.TrimSpace(d.Name)
	if name == "" || (d.MainCategory != "Needs" && d.MainCategory != "Wants" && d.MainCategory != "Savings") {
		jsonError(w, http.StatusBadRequest, "name and valid main_category required")
		return
	}
	var newID int64
	err := app.DB.QueryRow(context.Background(),
		"INSERT INTO categories (account_id, name, main_category) VALUES ($1,$2,$3) RETURNING id",
		aid, name, d.MainCategory).Scan(&newID)
	if err != nil {
		if isUniqueViolation(err) {
			jsonError(w, http.StatusConflict, "category already exists")
			return
		}
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": newID})
}

func (app *App) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	cid, err := urlInt64(r, "cid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var d map[string]any
	_ = decodeJSON(r, &d)
	fields := []string{}
	args := []any{}
	if v, _ := d["name"].(string); strings.TrimSpace(v) != "" {
		args = append(args, strings.TrimSpace(v))
		fields = append(fields, fmt.Sprintf("name=$%d", len(args)))
	}
	if v, _ := d["main_category"].(string); v == "Needs" || v == "Wants" || v == "Savings" {
		args = append(args, v)
		fields = append(fields, fmt.Sprintf("main_category=$%d", len(args)))
	}
	if len(fields) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	args = append(args, cid, aid)
	query := fmt.Sprintf("UPDATE categories SET %s WHERE id=$%d AND account_id=$%d", strings.Join(fields, ","), len(args)-1, len(args))
	if _, err := app.DB.Exec(context.Background(), query, args...); err != nil {
		if isUniqueViolation(err) {
			jsonError(w, http.StatusConflict, "name already used")
			return
		}
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	cid, err := urlInt64(r, "cid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := context.Background()
	var spendingCount, budgetCount int64
	if err := app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM spending WHERE category_id=$1 AND account_id=$2", cid, aid).Scan(&spendingCount); err != nil {
		dbError(w, err)
		return
	}
	if err := app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM budget_expense WHERE category_id=$1 AND account_id=$2", cid, aid).Scan(&budgetCount); err != nil {
		dbError(w, err)
		return
	}
	force := r.URL.Query().Get("force") == "1"
	if (spendingCount > 0 || budgetCount > 0) && !force {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "category in use", "spending_count": spendingCount, "budget_count": budgetCount})
		return
	}
	if _, err := app.DB.Exec(ctx, "UPDATE spending SET category_id=NULL WHERE category_id=$1 AND account_id=$2", cid, aid); err != nil {
		dbError(w, err)
		return
	}
	if _, err := app.DB.Exec(ctx, "DELETE FROM budget_expense WHERE category_id=$1 AND account_id=$2", cid, aid); err != nil {
		dbError(w, err)
		return
	}
	if _, err := app.DB.Exec(ctx, "DELETE FROM categories WHERE id=$1 AND account_id=$2", cid, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// accounts CRUD (bank + asset accounts)

func (app *App) handleAddAccount(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var d struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
	}
	_ = decodeJSON(r, &d)
	if d.Kind == "" {
		d.Kind = "bank"
	}
	name := strings.TrimSpace(d.Name)
	if name == "" || (d.Kind != "bank" && d.Kind != "asset") {
		jsonError(w, http.StatusBadRequest, "name and valid kind required")
		return
	}
	var newID int64
	err := app.DB.QueryRow(context.Background(),
		"INSERT INTO accounts (account_id, name, kind) VALUES ($1,$2,$3) RETURNING id", aid, name, d.Kind).Scan(&newID)
	if err != nil {
		if isUniqueViolation(err) {
			jsonError(w, http.StatusConflict, "account already exists")
			return
		}
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": newID})
}

func (app *App) handleUpdateAccount(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	accID, err := urlInt64(r, "aid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var d struct {
		Name string `json:"name"`
	}
	_ = decodeJSON(r, &d)
	if strings.TrimSpace(d.Name) == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	_, err = app.DB.Exec(context.Background(), "UPDATE accounts SET name=$1 WHERE id=$2 AND account_id=$3", strings.TrimSpace(d.Name), accID, aid)
	if err != nil {
		if isUniqueViolation(err) {
			jsonError(w, http.StatusConflict, "name already used")
			return
		}
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	accID, err := urlInt64(r, "aid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := context.Background()
	var used int64
	if err := app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM assets WHERE account_ref=$1 AND account_id=$2", accID, aid).Scan(&used); err != nil {
		dbError(w, err)
		return
	}
	if used > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "account in use", "count": used})
		return
	}
	if _, err := app.DB.Exec(ctx, "DELETE FROM accounts WHERE id=$1 AND account_id=$2", accID, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// income sources CRUD

func (app *App) handleAddIncomeSource(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var d struct {
		Name string `json:"name"`
	}
	_ = decodeJSON(r, &d)
	name := strings.TrimSpace(d.Name)
	if name == "" {
		jsonError(w, http.StatusBadRequest, "name required")
		return
	}
	var newID int64
	err := app.DB.QueryRow(context.Background(),
		"INSERT INTO income_sources (account_id, name) VALUES ($1,$2) RETURNING id", aid, name).Scan(&newID)
	if err != nil {
		if isUniqueViolation(err) {
			jsonError(w, http.StatusConflict, "income source already exists")
			return
		}
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": newID})
}

func (app *App) handleUpdateIncomeSource(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	sid, err := urlInt64(r, "sid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var d struct {
		Name string `json:"name"`
	}
	_ = decodeJSON(r, &d)
	if strings.TrimSpace(d.Name) == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	_, err = app.DB.Exec(context.Background(), "UPDATE income_sources SET name=$1 WHERE id=$2 AND account_id=$3", strings.TrimSpace(d.Name), sid, aid)
	if err != nil {
		if isUniqueViolation(err) {
			jsonError(w, http.StatusConflict, "name already used")
			return
		}
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleDeleteIncomeSource(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	sid, err := urlInt64(r, "sid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := context.Background()
	var used int64
	if err := app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM budget_income WHERE source_id=$1 AND account_id=$2", sid, aid).Scan(&used); err != nil {
		dbError(w, err)
		return
	}
	force := r.URL.Query().Get("force") == "1"
	if used > 0 && !force {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "income source in use", "count": used})
		return
	}
	if _, err := app.DB.Exec(ctx, "DELETE FROM budget_income WHERE source_id=$1 AND account_id=$2", sid, aid); err != nil {
		dbError(w, err)
		return
	}
	if _, err := app.DB.Exec(ctx, "DELETE FROM income_sources WHERE id=$1 AND account_id=$2", sid, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
