package main

import (
	"context"
	"net/http"
)

// ---------- budgeting ----------

func (app *App) handleGetBudgeting(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	year, err := urlInt64(r, "year")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid year")
		return
	}
	ctx := context.Background()

	type incomeRow struct {
		ID         int64  `json:"id"`
		AccountID  int64  `json:"account_id"`
		Year       int64  `json:"year"`
		Month      int64  `json:"month"`
		SourceID   int64  `json:"source_id"`
		Amount     int64  `json:"amount"`
		SourceName string `json:"source_name"`
	}
	income := []incomeRow{}
	rows, err := app.DB.Query(ctx,
		"SELECT bi.id, bi.account_id, bi.year, bi.month, bi.source_id, bi.amount, s.name FROM budget_income bi "+
			"JOIN income_sources s ON s.id = bi.source_id WHERE bi.account_id=$1 AND bi.year=$2", aid, year)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var ir incomeRow
		if err := rows.Scan(&ir.ID, &ir.AccountID, &ir.Year, &ir.Month, &ir.SourceID, &ir.Amount, &ir.SourceName); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		income = append(income, ir)
	}
	rows.Close()

	type expenseRow struct {
		ID           int64  `json:"id"`
		AccountID    int64  `json:"account_id"`
		Year         int64  `json:"year"`
		Month        int64  `json:"month"`
		CategoryID   int64  `json:"category_id"`
		Amount       int64  `json:"amount"`
		CategoryName string `json:"category_name"`
		MainCategory string `json:"main_category"`
	}
	expense := []expenseRow{}
	rows, err = app.DB.Query(ctx,
		"SELECT be.id, be.account_id, be.year, be.month, be.category_id, be.amount, c.name, c.main_category FROM budget_expense be "+
			"JOIN categories c ON c.id = be.category_id WHERE be.account_id=$1 AND be.year=$2", aid, year)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var er expenseRow
		if err := rows.Scan(&er.ID, &er.AccountID, &er.Year, &er.Month, &er.CategoryID, &er.Amount, &er.CategoryName, &er.MainCategory); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		expense = append(expense, er)
	}
	rows.Close()

	type savingsRow struct {
		ID        int64  `json:"id"`
		AccountID int64  `json:"account_id"`
		Year      int64  `json:"year"`
		Month     int64  `json:"month"`
		Name      string `json:"name"`
		Amount    int64  `json:"amount"`
	}
	savings := []savingsRow{}
	rows, err = app.DB.Query(ctx, "SELECT id, account_id, year, month, name, amount FROM savings_list WHERE account_id=$1 AND year=$2", aid, year)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var sr savingsRow
		if err := rows.Scan(&sr.ID, &sr.AccountID, &sr.Year, &sr.Month, &sr.Name, &sr.Amount); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		savings = append(savings, sr)
	}
	rows.Close()

	incomeHistory := map[int64]map[int64]int64{}
	rows, err = app.DB.Query(ctx, "SELECT source_id, year, SUM(amount) FROM budget_income WHERE account_id=$1 AND year<$2 GROUP BY source_id, year", aid, year)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var sourceID, y, total int64
		if err := rows.Scan(&sourceID, &y, &total); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		if incomeHistory[sourceID] == nil {
			incomeHistory[sourceID] = map[int64]int64{}
		}
		incomeHistory[sourceID][y] = total
	}
	rows.Close()

	expenseHistory := map[int64]map[int64]int64{}
	rows, err = app.DB.Query(ctx, "SELECT category_id, year, SUM(amount) FROM budget_expense WHERE account_id=$1 AND year<$2 GROUP BY category_id, year", aid, year)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var catID, y, total int64
		if err := rows.Scan(&catID, &y, &total); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		if expenseHistory[catID] == nil {
			expenseHistory[catID] = map[int64]int64{}
		}
		expenseHistory[catID][y] = total
	}
	rows.Close()

	writeJSON(w, http.StatusOK, map[string]any{
		"income": income, "expense": expense, "savings": savings,
		"income_history": incomeHistory, "expense_history": expenseHistory,
	})
}

func (app *App) handleSetBudgetIncome(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var d struct {
		Year     int64 `json:"year"`
		Month    int64 `json:"month"`
		SourceID int64 `json:"source_id"`
		Amount   int64 `json:"amount"`
	}
	_ = decodeJSON(r, &d)
	_, err := app.DB.Exec(context.Background(),
		"INSERT INTO budget_income (account_id, year, month, source_id, amount) VALUES ($1,$2,$3,$4,$5) "+
			"ON CONFLICT (account_id, year, month, source_id) DO UPDATE SET amount=excluded.amount",
		aid, d.Year, d.Month, d.SourceID, d.Amount)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleSetBudgetExpense(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var d struct {
		Year       int64 `json:"year"`
		Month      int64 `json:"month"`
		CategoryID int64 `json:"category_id"`
		Amount     int64 `json:"amount"`
	}
	_ = decodeJSON(r, &d)
	_, err := app.DB.Exec(context.Background(),
		"INSERT INTO budget_expense (account_id, year, month, category_id, amount) VALUES ($1,$2,$3,$4,$5) "+
			"ON CONFLICT (account_id, year, month, category_id) DO UPDATE SET amount=excluded.amount",
		aid, d.Year, d.Month, d.CategoryID, d.Amount)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// GET /budgeting/{year}/actuals: what actually happened, to show next to
// the plan - spending per category per month and income entries per source
// per month (source_id null = entries without a source).
func (app *App) handleBudgetActuals(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	year, err := urlInt64(r, "year")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid year")
		return
	}
	from, to := monthRange(int(year), 0)
	ctx := r.Context()

	type expenseActual struct {
		CategoryID int64 `json:"category_id"`
		Month      int   `json:"month"`
		Amount     int64 `json:"amount"`
	}
	expense := []expenseActual{}
	rows, err := app.DB.Query(ctx,
		"SELECT category_id, EXTRACT(MONTH FROM tx_date)::int, SUM(amount) FROM spending "+
			"WHERE account_id=$1 AND tx_date >= $2 AND tx_date < $3 AND category_id IS NOT NULL GROUP BY 1, 2 ORDER BY 1, 2",
		aid, from, to)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var e expenseActual
		if err := rows.Scan(&e.CategoryID, &e.Month, &e.Amount); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		expense = append(expense, e)
	}
	rows.Close()

	type incomeActual struct {
		SourceID *int64 `json:"source_id"`
		Month    int    `json:"month"`
		Amount   int64  `json:"amount"`
	}
	income := []incomeActual{}
	rows, err = app.DB.Query(ctx,
		"SELECT source_id, EXTRACT(MONTH FROM tx_date)::int, SUM(amount) FROM income_entries "+
			"WHERE account_id=$1 AND tx_date >= $2 AND tx_date < $3 GROUP BY 1, 2 ORDER BY 1, 2",
		aid, from, to)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var e incomeActual
		if err := rows.Scan(&e.SourceID, &e.Month, &e.Amount); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		income = append(income, e)
	}
	rows.Close()

	writeJSON(w, http.StatusOK, map[string]any{"year": year, "expense": expense, "income": income})
}
