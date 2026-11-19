package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Meal proposals: every member (kids and viewers included) can propose a dish;
// owners and parents approve it (optionally on a given day) or decline it.
// Approved dishes that have a day appear on that day of the meal plan.
//
//	GET    /api/meal-plan/proposals[?status=pending|approved|declined]
//	POST   /api/meal-plan/proposals {dish, note?, wanted_date?}   any member
//	POST   /api/meal-plan/proposals/{id}/approve {meal_date?, note?}   owner/parent
//	POST   /api/meal-plan/proposals/{id}/decline {note?}               owner/parent
//	DELETE /api/meal-plan/proposals/{id}   the proposer (while pending) or owner/parent

const maxPendingProposalsPerMember = 20

func RegisterMealProposalRoutes(r chi.Router, app *App) {
	r.Get("/meal-plan/proposals", app.handleListMealProposals)
	r.Post("/meal-plan/proposals", app.handleCreateMealProposal)
	r.Post("/meal-plan/proposals/{id}/approve", app.handleDecideMealProposal("approved"))
	r.Post("/meal-plan/proposals/{id}/decline", app.handleDecideMealProposal("declined"))
	r.Delete("/meal-plan/proposals/{id}", app.handleDeleteMealProposal)
	registerTodayProvider("meal_requests", 60, mealRequestsTodayProvider)
}

type mealProposal struct {
	ID           int64   `json:"id"`
	Dish         string  `json:"dish"`
	Note         string  `json:"note"`
	MemberID     *int64  `json:"member_id"`
	MemberName   string  `json:"member_name"`
	WantedDate   *string `json:"wanted_date"`
	Status       string  `json:"status"`
	MealDate     *string `json:"meal_date"`
	DecidedBy    *int64  `json:"decided_by"`
	DecidedName  string  `json:"decided_by_name"`
	DecidedAt    *string `json:"decided_at"`
	DecisionNote string  `json:"decision_note"`
	CreatedAt    string  `json:"created_at"`
	CanDecide    bool    `json:"can_decide"`
	CanDelete    bool    `json:"can_delete"`
}

func (app *App) queryMealProposals(r *http.Request, m memberInfo, where string, args ...any) ([]mealProposal, error) {
	ctx := r.Context()
	all := append([]any{m.AccountID}, args...)
	rows, err := app.DB.Query(ctx, `SELECT id, dish, note, member_id, wanted_date, status, meal_date, decided_by, decided_at, decision_note, created_at
	  FROM meal_proposals WHERE account_id=$1 `+where+` ORDER BY (status='pending') DESC, created_at DESC, id DESC LIMIT 100`, all...)
	if err != nil {
		return nil, err
	}
	var out []mealProposal
	for rows.Next() {
		var p mealProposal
		var wanted, meal *time.Time
		var decided *time.Time
		var created time.Time
		if err := rows.Scan(&p.ID, &p.Dish, &p.Note, &p.MemberID, &wanted, &p.Status, &meal, &p.DecidedBy, &decided, &p.DecisionNote, &created); err != nil {
			rows.Close()
			return nil, err
		}
		p.WantedDate, p.MealDate = fmtDatePtr(wanted), fmtDatePtr(meal)
		if decided != nil {
			v := decided.UTC().Format(time.RFC3339)
			p.DecidedAt = &v
		}
		p.CreatedAt = created.UTC().Format(time.RFC3339)
		out = append(out, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	names, err := app.memberNames(ctx, m.AccountID)
	if err != nil {
		return nil, err
	}
	manager := canManageFamily(m.Role)
	for i := range out {
		p := &out[i]
		p.MemberName, p.DecidedName = nameOf(names, p.MemberID), nameOf(names, p.DecidedBy)
		p.CanDecide = manager
		p.CanDelete = manager || (p.Status == "pending" && p.MemberID != nil && *p.MemberID == m.ID)
	}
	if out == nil {
		out = []mealProposal{}
	}
	return out, nil
}

func (app *App) handleListMealProposals(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	where, args := "", []any{}
	if s := r.URL.Query().Get("status"); s != "" {
		if s != "pending" && s != "approved" && s != "declined" {
			jsonError(w, http.StatusBadRequest, "status must be pending, approved or declined")
			return
		}
		where, args = "AND status=$2", []any{s}
	}
	items, err := app.queryMealProposals(r, m, where, args...)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func optionalDay(v *string) (*string, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil, nil
	}
	if _, err := parseDay(*v); err != nil {
		return nil, errors.New("dates must be YYYY-MM-DD")
	}
	return v, nil
}

func (app *App) handleCreateMealProposal(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	var in struct {
		Dish       string  `json:"dish"`
		Note       string  `json:"note"`
		WantedDate *string `json:"wanted_date"`
	}
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	dish := clip(in.Dish, 100)
	if dish == "" {
		jsonError(w, http.StatusBadRequest, "what would you like to eat?")
		return
	}
	wanted, err := optionalDay(in.WantedDate)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := r.Context()
	var pending int
	if err := app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM meal_proposals WHERE account_id=$1 AND member_id=$2 AND status='pending'", m.AccountID, m.ID).Scan(&pending); err != nil {
		dbError(w, err)
		return
	}
	if pending >= maxPendingProposalsPerMember {
		jsonError(w, http.StatusBadRequest, "you already have many requests waiting - let the parents decide first")
		return
	}
	var member *int64
	if m.ID != 0 {
		member = &m.ID
	}
	var id int64
	if err := app.DB.QueryRow(ctx, "INSERT INTO meal_proposals (account_id, member_id, dish, note, wanted_date) VALUES ($1,$2,$3,$4,$5) RETURNING id",
		m.AccountID, member, dish, clip(in.Note, 300), wanted).Scan(&id); err != nil {
		dbError(w, err)
		return
	}
	items, err := app.queryMealProposals(r, m, "AND id=$2", id)
	if err != nil || len(items) == 0 {
		dbError(w, fmt.Errorf("load proposal: %v", err))
		return
	}
	writeJSON(w, http.StatusCreated, items[0])
}

func (app *App) handleDecideMealProposal(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, ok := requireMember(w, r)
		if !ok {
			return
		}
		if !canManageFamily(m.Role) {
			jsonError(w, http.StatusForbidden, "only the family admin and parents can decide")
			return
		}
		id, err := urlInt64(r, "id")
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid id")
			return
		}
		var in struct {
			MealDate *string `json:"meal_date"`
			Note     string  `json:"note"`
		}
		_ = decodeJSON(r, &in)
		var mealDate *string
		if status == "approved" {
			if mealDate, err = optionalDay(in.MealDate); err != nil {
				jsonError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		var decider *int64
		if m.ID != 0 {
			decider = &m.ID
		}
		tag, err := app.DB.Exec(r.Context(), `UPDATE meal_proposals SET status=$3, meal_date=$4, decided_by=$5, decided_at=now(), decision_note=$6
		  WHERE id=$1 AND account_id=$2`, id, m.AccountID, status, mealDate, decider, clip(in.Note, 300))
		if err != nil {
			dbError(w, err)
			return
		}
		if tag.RowsAffected() == 0 {
			jsonError(w, http.StatusNotFound, "not found")
			return
		}
		items, err := app.queryMealProposals(r, m, "AND id=$2", id)
		if err != nil || len(items) == 0 {
			dbError(w, fmt.Errorf("load proposal: %v", err))
			return
		}
		writeJSON(w, http.StatusOK, items[0])
	}
}

func (app *App) handleDeleteMealProposal(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var owner *int64
	var status string
	err = app.DB.QueryRow(r.Context(), "SELECT member_id, status FROM meal_proposals WHERE id=$1 AND account_id=$2", id, m.AccountID).Scan(&owner, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	own := owner != nil && *owner == m.ID && status == "pending"
	if !canManageFamily(m.Role) && !own {
		jsonError(w, http.StatusForbidden, "you can only withdraw your own requests that are still waiting")
		return
	}
	if _, err := app.DB.Exec(r.Context(), "DELETE FROM meal_proposals WHERE id=$1 AND account_id=$2", id, m.AccountID); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- meal plan overlay + Today ----------

type mealPick struct {
	ID         int64  `json:"id"`
	Dish       string `json:"dish"`
	MemberName string `json:"member_name"`
}

// loadMealPicks: approved proposals scheduled within [from, to], by date.
func (app *App) loadMealPicks(ctx context.Context, aid int64, from, to time.Time) (map[string][]mealPick, error) {
	rows, err := app.DB.Query(ctx, "SELECT id, dish, member_id, meal_date FROM meal_proposals WHERE account_id=$1 AND status='approved' AND meal_date >= $2 AND meal_date <= $3 ORDER BY id", aid, from, to)
	if err != nil {
		return nil, err
	}
	type rowT struct {
		p    mealPick
		mid  *int64
		date string
	}
	var all []rowT
	for rows.Next() {
		var x rowT
		var d time.Time
		if err := rows.Scan(&x.p.ID, &x.p.Dish, &x.mid, &d); err != nil {
			rows.Close()
			return nil, err
		}
		x.date = d.Format("2006-01-02")
		all = append(all, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	names, err := app.memberNames(ctx, aid)
	if err != nil {
		return nil, err
	}
	out := map[string][]mealPick{}
	for _, x := range all {
		x.p.MemberName = nameOf(names, x.mid)
		out[x.date] = append(out[x.date], x.p)
	}
	return out, nil
}

func mealRequestsTodayProvider(ctx context.Context, app *App, aid int64, m memberInfo, day time.Time) (*TodaySection, error) {
	if !canManageFamily(m.Role) {
		return nil, nil
	}
	rows, err := app.DB.Query(ctx, "SELECT p.dish, COALESCE(NULLIF(mb.display_name,''), split_part(mb.email,'@',1), '') FROM meal_proposals p LEFT JOIN members mb ON mb.id=p.member_id WHERE p.account_id=$1 AND p.status='pending' ORDER BY p.created_at LIMIT 10", aid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sec := &TodaySection{Key: "meal_requests", Title: "Meal requests", Items: []TodayItem{}}
	for rows.Next() {
		var dish, who string
		if err := rows.Scan(&dish, &who); err != nil {
			return nil, err
		}
		sec.Items = append(sec.Items, TodayItem{Kind: "meal_request", Title: dish, Detail: "requested by " + who, Link: "/home/meals", Actions: []string{}})
	}
	if len(sec.Items) == 0 {
		return nil, rows.Err()
	}
	sec.Summary = fmt.Sprintf("%d meal request%s waiting for a decision", len(sec.Items), plural(len(sec.Items)))
	return sec, rows.Err()
}
