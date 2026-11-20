package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// savings goals
//
// progress = saved_amount (manual contributions)
//          + spending in the linked category since the goal was created
//            (e.g. a "Savings" category you log transfers under)
//          + current value of linked assets.

type goalRow struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	TargetAmount  int64     `json:"target_amount"`
	TargetDate    *string   `json:"target_date"`
	SavedAmount   int64     `json:"saved_amount"`
	CategoryID    *int64    `json:"category_id"`
	CategoryName  *string   `json:"category_name"`
	AssetIDs      []int64   `json:"asset_ids"`
	Emoji         string    `json:"emoji"`
	Color         string    `json:"color"`
	CreatedAt     time.Time `json:"created_at"`
	LinkedAmount  int64     `json:"linked_amount"`
	Progress      int64     `json:"progress"`
	Remaining     int64     `json:"remaining"`
	Pct           float64   `json:"pct"`
	MonthsLeft    *int      `json:"months_left"`
	MonthlyNeeded *int64    `json:"monthly_needed"`
}

type goalInput struct {
	Name         string  `json:"name"`
	TargetAmount int64   `json:"target_amount"`
	TargetDate   *string `json:"target_date"`
	SavedAmount  int64   `json:"saved_amount"`
	CategoryID   *int64  `json:"category_id"`
	AssetIDs     []int64 `json:"asset_ids"`
	Emoji        string  `json:"emoji"`
	Color        string  `json:"color"`
}

// goalMonthlyNeeded: months left counts the current month and the target
// month (Nov 17 -> Dec 31 = 2), minimum 1 (an overdue goal needs it all
// now); needed = ceil(remaining / months). Returns (0, 0) without a target
// date or when nothing remains.
func goalMonthlyNeeded(remaining int64, today time.Time, target *time.Time) (months int, needed int64) {
	if target == nil || remaining <= 0 {
		return 0, 0
	}
	t, n := moneyUTCDate(*target), moneyUTCDate(today)
	months = (t.Year()-n.Year())*12 + int(t.Month()-n.Month()) + 1
	if months < 1 {
		months = 1
	}
	return months, (remaining + int64(months) - 1) / int64(months)
}

func (in *goalInput) normalize(ctx context.Context, app *App, aid int64) (*time.Time, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if in.TargetAmount <= 0 {
		return nil, fmt.Errorf("target_amount must be positive")
	}
	if in.SavedAmount < 0 {
		return nil, fmt.Errorf("saved_amount cannot be negative")
	}
	td, err := parseDatePtr(in.TargetDate)
	if err != nil {
		return nil, fmt.Errorf("invalid target_date (want YYYY-MM-DD)")
	}
	if in.CategoryID != nil {
		var ok bool
		if err := app.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM categories WHERE id=$1 AND account_id=$2)", *in.CategoryID, aid).Scan(&ok); err != nil || !ok {
			return nil, fmt.Errorf("unknown category")
		}
	}
	if in.AssetIDs == nil {
		in.AssetIDs = []int64{}
	}
	if len(in.AssetIDs) > 0 {
		var n int
		if err := app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM assets WHERE id = ANY($1) AND account_id=$2", in.AssetIDs, aid).Scan(&n); err != nil || n != len(in.AssetIDs) {
			return nil, fmt.Errorf("unknown asset")
		}
	}
	in.Emoji = "" // accepted for older clients but no longer stored: Kinkit shows no emoji
	if len(in.Color) > 32 {
		in.Color = in.Color[:32]
	}
	return td, nil
}

func (app *App) loadGoals(ctx context.Context, where string, args ...any) ([]goalRow, error) {
	rows, err := app.DB.Query(ctx, `SELECT g.id, g.name, g.target_amount, g.target_date, g.saved_amount, g.category_id, c.name,
	        g.asset_ids, g.emoji, g.color, g.created_at,
	        COALESCE((SELECT SUM(sp.amount) FROM spending sp WHERE sp.account_id=g.account_id AND sp.category_id=g.category_id
	                  AND sp.tx_date >= (g.created_at AT TIME ZONE 'UTC')::date),0)
	      + COALESCE((SELECT SUM(a.value) FROM assets a WHERE a.account_id=g.account_id AND a.id = ANY(g.asset_ids)),0)
	   FROM savings_goals g LEFT JOIN categories c ON c.id = g.category_id `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	today := time.Now().UTC()
	out := []goalRow{}
	for rows.Next() {
		var g goalRow
		var td *time.Time
		if err := rows.Scan(&g.ID, &g.Name, &g.TargetAmount, &td, &g.SavedAmount, &g.CategoryID, &g.CategoryName,
			&g.AssetIDs, &g.Emoji, &g.Color, &g.CreatedAt, &g.LinkedAmount); err != nil {
			return nil, err
		}
		if g.AssetIDs == nil {
			g.AssetIDs = []int64{}
		}
		g.TargetDate = fmtDatePtr(td)
		g.Progress = g.SavedAmount + g.LinkedAmount
		g.Remaining = max(g.TargetAmount-g.Progress, 0)
		if g.TargetAmount > 0 {
			g.Pct = float64(g.Progress) * 100 / float64(g.TargetAmount)
		}
		if months, needed := goalMonthlyNeeded(g.Remaining, today, td); months > 0 {
			g.MonthsLeft, g.MonthlyNeeded = &months, &needed
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (app *App) handleListGoals(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	list, err := app.loadGoals(r.Context(), "WHERE g.account_id=$1 ORDER BY g.target_date NULLS LAST, g.id", aid)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (app *App) handleAddGoal(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var in goalInput
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx := r.Context()
	td, err := in.normalize(ctx, app, aid)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	var id int64
	if err := app.DB.QueryRow(ctx,
		"INSERT INTO savings_goals (account_id, name, target_amount, target_date, saved_amount, category_id, asset_ids, emoji, color) "+
			"VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id",
		aid, in.Name, in.TargetAmount, td, in.SavedAmount, in.CategoryID, in.AssetIDs, in.Emoji, in.Color).Scan(&id); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

func (app *App) handleUpdateGoal(w http.ResponseWriter, r *http.Request) {
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
	existing, err := app.loadGoals(ctx, "WHERE g.account_id=$1 AND g.id=$2", aid, id)
	if err != nil {
		dbError(w, err)
		return
	}
	if len(existing) == 0 {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	cur := existing[0]
	in := goalInput{Name: cur.Name, TargetAmount: cur.TargetAmount, TargetDate: cur.TargetDate, SavedAmount: cur.SavedAmount,
		CategoryID: cur.CategoryID, AssetIDs: cur.AssetIDs, Emoji: cur.Emoji, Color: cur.Color}
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	td, err := in.normalize(ctx, app, aid)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := app.DB.Exec(ctx,
		"UPDATE savings_goals SET name=$1, target_amount=$2, target_date=$3, saved_amount=$4, category_id=$5, asset_ids=$6, emoji=$7, color=$8 "+
			"WHERE id=$9 AND account_id=$10",
		in.Name, in.TargetAmount, td, in.SavedAmount, in.CategoryID, in.AssetIDs, in.Emoji, in.Color, id, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// POST /goals/{id}/contribute {amount}: adds to saved_amount (a negative
// amount withdraws, never below zero).
func (app *App) handleContributeGoal(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var d struct {
		Amount int64 `json:"amount"`
	}
	if err := decodeJSON(r, &d); err != nil || d.Amount == 0 {
		jsonError(w, http.StatusBadRequest, "amount is required")
		return
	}
	var saved int64
	if err := app.DB.QueryRow(r.Context(),
		"UPDATE savings_goals SET saved_amount = GREATEST(saved_amount + $1, 0) WHERE id=$2 AND account_id=$3 RETURNING saved_amount",
		d.Amount, id, aid).Scan(&saved); err != nil {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "saved_amount": saved})
}

func (app *App) handleDeleteGoal(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := app.DB.Exec(r.Context(), "DELETE FROM savings_goals WHERE id=$1 AND account_id=$2", id, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
