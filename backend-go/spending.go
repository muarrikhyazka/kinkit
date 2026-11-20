package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

// spending

type spendingRow struct {
	ID             int64   `json:"id"`
	AccountID      int64   `json:"account_id"`
	TxDate         string  `json:"tx_date"`
	Description    string  `json:"description"`
	CategoryID     *int64  `json:"category_id"`
	CategoryName   *string `json:"category_name"`
	Amount         int64   `json:"amount"`
	Checked        bool    `json:"checked"`
	Source         string  `json:"source"`
	ClientID       *string `json:"client_id"`
	CreatedAt      string  `json:"created_at"`
	CategoryMaster *string `json:"category_master"`
	HasReceipt     bool    `json:"has_receipt"` // --- data ---
	MemberID       *int64  `json:"member_id"`
	MemberName     *string `json:"member_name"` // who logged it (household members)
}

func (app *App) handleListSpending(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	year := queryIntPtr(r, "year")
	month := queryIntPtr(r, "month")
	page := queryIntDefault(r, "page", 1)
	if page < 1 {
		page = 1
	}
	perPage := queryIntDefault(r, "per_page", 20)
	if perPage > 500 {
		perPage = 500
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	sortParam := r.URL.Query().Get("sort")
	if sortParam == "" {
		sortParam = "tx_date"
	}
	order := strings.ToLower(r.URL.Query().Get("order"))

	base := "FROM spending sp LEFT JOIN categories c ON c.id = sp.category_id " +
		"LEFT JOIN members m ON m.id = sp.member_id WHERE sp.account_id=$1"
	args := []any{aid}
	// Kids only see the spending they logged themselves.
	if m, ok := currentMember(r); ok && m.Role == "kid" {
		args = append(args, m.ID)
		base += fmt.Sprintf(" AND sp.member_id = $%d", len(args))
	}
	if year != nil {
		args = append(args, *year)
		base += fmt.Sprintf(" AND EXTRACT(YEAR FROM sp.tx_date)::int = $%d", len(args))
	}
	if month != nil {
		args = append(args, *month)
		base += fmt.Sprintf(" AND EXTRACT(MONTH FROM sp.tx_date)::int = $%d", len(args))
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		base += fmt.Sprintf(" AND sp.description ILIKE $%d", len(args))
	}
	if category != "" {
		args = append(args, category)
		base += fmt.Sprintf(" AND sp.category_name = $%d", len(args))
	}
	base, args = appendCeiling(base, args, r, "sp.amount")

	sortCols := map[string]string{"tx_date": "sp.tx_date", "description": "sp.description", "category_name": "sp.category_name", "amount": "sp.amount"}
	sortCol, ok2 := sortCols[sortParam]
	if !ok2 {
		sortCol = "sp.tx_date"
	}
	orderDir := "DESC"
	if order == "asc" {
		orderDir = "ASC"
	}

	ctx := context.Background()
	var total int64
	if err := app.DB.QueryRow(ctx, "SELECT COUNT(*) "+base, args...).Scan(&total); err != nil {
		dbError(w, err)
		return
	}

	q := "SELECT sp.id, sp.account_id, sp.tx_date::text, sp.description, sp.category_id, sp.category_name, " +
		"sp.amount, sp.checked, sp.source, sp.client_id, sp.created_at::text, c.name, sp.receipt_path IS NOT NULL, sp.member_id, m.display_name " + base +
		fmt.Sprintf(" ORDER BY %s %s, sp.id DESC LIMIT $%d OFFSET $%d", sortCol, orderDir, len(args)+1, len(args)+2)
	listArgs := append(append([]any{}, args...), perPage, (page-1)*perPage)
	rows, err := app.DB.Query(ctx, q, listArgs...)
	if err != nil {
		dbError(w, err)
		return
	}
	out := []spendingRow{}
	for rows.Next() {
		var s spendingRow
		if err := rows.Scan(&s.ID, &s.AccountID, &s.TxDate, &s.Description, &s.CategoryID, &s.CategoryName,
			&s.Amount, &s.Checked, &s.Source, &s.ClientID, &s.CreatedAt, &s.CategoryMaster, &s.HasReceipt, &s.MemberID, &s.MemberName); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		out = append(out, s)
	}
	rows.Close()
	writeJSON(w, http.StatusOK, map[string]any{"rows": out, "total": total, "page": page, "per_page": perPage})
}

func (app *App) handleAddSpending(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var d struct {
		TxDate      string  `json:"tx_date"`
		Description string  `json:"description"`
		Category    string  `json:"category"`
		Amount      int64   `json:"amount"`
		Source      string  `json:"source"`
		ClientID    *string `json:"client_id"`
	}
	_ = decodeJSON(r, &d)
	if d.Source == "" {
		d.Source = "web"
	}
	ctx := context.Background()

	var catID *int64
	if d.Category != "" {
		var id int64
		if err := app.DB.QueryRow(ctx, "SELECT id FROM categories WHERE name=$1 AND account_id=$2", d.Category, aid).Scan(&id); err == nil {
			catID = &id
		}
	}

	if d.ClientID != nil && *d.ClientID != "" {
		var existing int64
		if err := app.DB.QueryRow(ctx, "SELECT id FROM spending WHERE client_id=$1 AND account_id=$2", *d.ClientID, aid).Scan(&existing); err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": existing, "duplicate": true})
			return
		}
	}

	var categoryName *string
	if d.Category != "" {
		categoryName = &d.Category
	}
	var newID int64
	err := app.DB.QueryRow(ctx,
		"INSERT INTO spending (account_id, tx_date, description, category_id, category_name, amount, source, client_id, member_id) "+
			"VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id",
		aid, d.TxDate, d.Description, catID, categoryName, d.Amount, d.Source, d.ClientID, currentMemberIDPtr(r)).Scan(&newID) // family: member_id = who logged it
	if err != nil {
		dbError(w, err)
		return
	}
	app.awardPointsForMember(ctx, aid, currentMemberIDPtr(r)) // family: per-member points
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": newID})
}

func (app *App) handleDeleteSpending(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	sid, err := urlInt64(r, "sid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var receipt *string // --- data ---: delete the receipt photo with the row
	if err := app.DB.QueryRow(context.Background(), "DELETE FROM spending WHERE id=$1 AND account_id=$2 RETURNING receipt_path", sid, aid).Scan(&receipt); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		dbError(w, err)
		return
	}
	removeReceiptFile(receipt)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleUpdateSpending(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	sid, err := urlInt64(r, "sid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var d map[string]any
	_ = decodeJSON(r, &d)

	fields := []string{}
	args := []any{}
	for _, k := range []string{"tx_date", "description", "amount", "checked"} {
		v, present := d[k]
		if !present {
			continue
		}
		if k == "amount" {
			if n, ok := asInt64(v); ok {
				v = n
			}
		}
		args = append(args, v)
		fields = append(fields, fmt.Sprintf("%s=$%d", k, len(args)))
	}
	if catName, ok := d["category"].(string); ok && catName != "" {
		ctx := context.Background()
		var catID *int64
		var id int64
		if err := app.DB.QueryRow(ctx, "SELECT id FROM categories WHERE name=$1 AND account_id=$2", catName, aid).Scan(&id); err == nil {
			catID = &id
		}
		args = append(args, catID)
		fields = append(fields, fmt.Sprintf("category_id=$%d", len(args)))
		args = append(args, catName)
		fields = append(fields, fmt.Sprintf("category_name=$%d", len(args)))
	}
	if len(fields) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	args = append(args, sid, aid)
	query := fmt.Sprintf("UPDATE spending SET %s WHERE id=$%d AND account_id=$%d", strings.Join(fields, ","), len(args)-1, len(args))
	if _, err := app.DB.Exec(context.Background(), query, args...); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
