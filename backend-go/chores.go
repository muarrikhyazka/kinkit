package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Chores & tasks.
//
//   chores            definitions (owner/parent manage them)
//   chore_instances   one row per occurrence (household-local due_date),
//                     generated lazily on reads + by an hourly tick
//   chore_earnings    money rewards, shown on top of the kid's allowance
//
// Points go through the shared gamification ledger (point_events +
// member_points + user_points) so leaderboard, levels and badges see them.
// Routes that take {id} after /chores/ and end in an action (complete,
// approve, reject, undo, photo) use the INSTANCE id; PATCH/DELETE
// /chores/{id} use the chore id.

func init() {
	// Kids may complete / undo chores; the handler checks the instance is
	// theirs (or unassigned).
	registerKidRule(func(method, path string) (roleDecision, int64, bool) {
		if method != http.MethodPost {
			return 0, 0, false
		}
		rest, ok := strings.CutPrefix(path, "/api/chores/")
		if !ok {
			return 0, 0, false
		}
		idPart, action, _ := strings.Cut(rest, "/")
		if _, err := strconv.ParseInt(idPart, 10, 64); err != nil {
			return 0, 0, false
		}
		if action == "complete" || action == "undo" {
			return roleAllow, 0, true
		}
		return 0, 0, false
	})
	registerTodayProvider("chores", 20, choresTodayProvider)
}

func RegisterChoreRoutes(r chi.Router, app *App) {
	r.Get("/chores", app.handleListChores)
	r.Post("/chores", app.handleCreateChore)
	r.Get("/chores/today", app.handleChoresToday)
	r.Get("/chores/instances", app.handleChoreInstances)
	r.Patch("/chores/{id}", app.handleUpdateChore)
	r.Delete("/chores/{id}", app.handleDeleteChore)
	r.Post("/chores/{id}/complete", app.handleCompleteChore)
	r.Post("/chores/{id}/approve", app.choreActionHandler("approve"))
	r.Post("/chores/{id}/reject", app.choreActionHandler("reject"))
	r.Post("/chores/{id}/undo", app.choreActionHandler("undo"))
	r.Get("/chores/{id}/photo", app.handleChorePhoto)
	// bot-token authenticated (contract used by bots/)
	r.Get("/bot/chores/today", botAuth(app, botChoresToday))
	r.Post("/bot/chores/{id}/done", botAuth(app, botChoreDone))
}

// chore definitions

type choreRow struct {
	ID                int64     `json:"id"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	Emoji             string    `json:"emoji"`
	Points            int       `json:"points"`
	RewardAmount      *float64  `json:"reward_amount"`
	MemberID          *int64    `json:"member_id"`
	RotationMemberIDs []int64   `json:"rotation_member_ids"`
	RotationCursor    int       `json:"rotation_cursor"`
	Schedule          string    `json:"schedule"`
	Weekdays          []int     `json:"weekdays"`
	DayOfMonth        *int      `json:"day_of_month"`
	StartDate         string    `json:"start_date"`
	EndDate           *string   `json:"end_date"`
	DueTime           *string   `json:"due_time"`
	RequiresApproval  bool      `json:"requires_approval"`
	RequiresPhoto     bool      `json:"requires_photo"`
	Active            bool      `json:"active"`
	CreatedAt         time.Time `json:"created_at"`
	NextAssigneeID    *int64    `json:"next_assignee_id"` // who gets the next not-yet-generated occurrence
}

const choreCols = `id, title, description, emoji, points, reward_amount::float8, member_id, rotation_member_ids,
	rotation_cursor, schedule, weekdays, day_of_month, start_date::text, end_date::text, due_time,
	requires_approval, requires_photo, active, created_at`

func scanChore(row pgx.Row) (choreRow, error) {
	var c choreRow
	err := row.Scan(&c.ID, &c.Title, &c.Description, &c.Emoji, &c.Points, &c.RewardAmount, &c.MemberID, &c.RotationMemberIDs,
		&c.RotationCursor, &c.Schedule, &c.Weekdays, &c.DayOfMonth, &c.StartDate, &c.EndDate, &c.DueTime,
		&c.RequiresApproval, &c.RequiresPhoto, &c.Active, &c.CreatedAt)
	if c.RotationMemberIDs == nil {
		c.RotationMemberIDs = []int64{}
	}
	if c.Weekdays == nil {
		c.Weekdays = []int{}
	}
	if err == nil {
		next, _ := assignOccurrences(c.MemberID, c.RotationMemberIDs, c.RotationCursor, 1)
		c.NextAssigneeID = next[0]
	}
	return c, err
}

func (c choreRow) schedule() choreSchedule {
	s := choreSchedule{Kind: c.Schedule, Weekdays: c.Weekdays}
	s.Start, _ = time.Parse(dateLayout, c.StartDate)
	if c.DayOfMonth != nil {
		s.DayOfMonth = *c.DayOfMonth
	}
	if c.EndDate != nil {
		if e, err := time.Parse(dateLayout, *c.EndDate); err == nil {
			s.End = &e
		}
	}
	return s
}

// structuralKey changes when an edit affects which instances exist or who
// they belong to (then not-yet-done instances from today on are rebuilt).
func (c choreRow) structuralKey() string {
	return fmt.Sprint(c.Schedule, c.Weekdays, c.DayOfMonth, c.StartDate, c.EndDate, c.MemberID, c.RotationMemberIDs, c.Active)
}

func optStr(v any) (*string, bool) {
	switch t := v.(type) {
	case nil:
		return nil, true
	case string:
		if s := strings.TrimSpace(t); s != "" {
			return &s, true
		}
		return nil, true
	}
	return nil, false
}

func optInt64(v any) (*int64, bool) {
	if v == nil {
		return nil, true
	}
	n, ok := asInt64(v)
	if !ok || n <= 0 {
		return nil, false
	}
	return &n, true
}

// applyChoreInput validates a create/PATCH body onto c. Only keys present
// in d are touched.
func applyChoreInput(c *choreRow, d map[string]any) error {
	if v, ok := d["title"]; ok {
		s, _ := v.(string)
		c.Title = strings.TrimSpace(s)
	}
	if c.Title == "" || len(c.Title) > 120 {
		return errors.New("title is required (max 120 chars)")
	}
	if v, ok := d["description"]; ok {
		s, _ := v.(string)
		if c.Description = strings.TrimSpace(s); len(c.Description) > 1000 {
			return errors.New("description too long (max 1000)")
		}
	}
	// "emoji" is accepted for older clients but no longer stored: Kinkit shows no emoji.
	if _, ok := d["emoji"]; ok {
		c.Emoji = ""
	}
	if v, ok := d["points"]; ok {
		n, okN := asInt64(v)
		if !okN || n < 0 || n > 10000 {
			return errors.New("points must be 0..10000")
		}
		c.Points = int(n)
	}
	if v, ok := d["reward_amount"]; ok {
		a, okA := parseAllowance(v)
		if !okA {
			return errors.New("reward_amount must be a non-negative number")
		}
		c.RewardAmount = a
	}
	if v, ok := d["member_id"]; ok {
		id, okI := optInt64(v)
		if !okI {
			return errors.New("invalid member_id")
		}
		c.MemberID = id
	}
	if v, ok := d["rotation_member_ids"]; ok {
		ids := []int64{}
		if v != nil {
			arr, okA := v.([]any)
			if !okA {
				return errors.New("rotation_member_ids must be a list")
			}
			for _, x := range arr {
				n, okN := asInt64(x)
				if !okN || n <= 0 {
					return errors.New("invalid rotation member id")
				}
				if !slices.Contains(ids, n) {
					ids = append(ids, n)
				}
			}
		}
		c.RotationMemberIDs = ids
	}
	if v, ok := d["schedule"]; ok {
		c.Schedule, _ = v.(string)
	}
	switch c.Schedule {
	case choreOnce, choreDaily, choreWeekly, choreMonthly:
	default:
		return errors.New("schedule must be once, daily, weekly or monthly")
	}
	if v, ok := d["weekdays"]; ok {
		days := []int{}
		if v != nil {
			arr, okA := v.([]any)
			if !okA {
				return errors.New("weekdays must be a list of 0 (Sun) .. 6 (Sat)")
			}
			for _, x := range arr {
				n, okN := asInt64(x)
				if !okN || n < 0 || n > 6 {
					return errors.New("weekdays must be 0 (Sun) .. 6 (Sat)")
				}
				if !slices.Contains(days, int(n)) {
					days = append(days, int(n))
				}
			}
		}
		slices.Sort(days)
		c.Weekdays = days
	}
	if v, ok := d["day_of_month"]; ok {
		if v == nil {
			c.DayOfMonth = nil
		} else {
			n, okN := asInt64(v)
			if !okN || n < 1 || n > 31 {
				return errors.New("day_of_month must be 1..31")
			}
			dm := int(n)
			c.DayOfMonth = &dm
		}
	}
	if v, ok := d["start_date"]; ok {
		s, _ := v.(string)
		if _, err := time.Parse(dateLayout, s); err != nil {
			return errors.New("start_date must be YYYY-MM-DD")
		}
		c.StartDate = s
	}
	if v, ok := d["end_date"]; ok {
		s, okS := optStr(v)
		if !okS {
			return errors.New("end_date must be YYYY-MM-DD")
		}
		if s != nil {
			if _, err := time.Parse(dateLayout, *s); err != nil {
				return errors.New("end_date must be YYYY-MM-DD")
			}
		}
		c.EndDate = s
	}
	if c.EndDate != nil && *c.EndDate < c.StartDate {
		return errors.New("end_date is before start_date")
	}
	if v, ok := d["due_time"]; ok {
		s, okS := optStr(v)
		if !okS {
			return errors.New("due_time must be HH:MM")
		}
		if s != nil {
			h, m, okT := parseDueTime(*s)
			if !okT {
				return errors.New("due_time must be HH:MM")
			}
			norm := fmt.Sprintf("%02d:%02d", h, m)
			s = &norm
		}
		c.DueTime = s
	}
	for key, dst := range map[string]*bool{"requires_approval": &c.RequiresApproval, "requires_photo": &c.RequiresPhoto, "active": &c.Active} {
		if v, ok := d[key]; ok {
			b, okB := v.(bool)
			if !okB {
				return errors.New(key + " must be true or false")
			}
			*dst = b
		}
	}
	// A one-person rotation is just a fixed assignee; a real rotation wins
	// over a fixed member.
	if len(c.RotationMemberIDs) == 1 {
		id := c.RotationMemberIDs[0]
		c.MemberID, c.RotationMemberIDs = &id, []int64{}
	} else if len(c.RotationMemberIDs) > 1 {
		c.MemberID = nil
	}
	return nil
}

// checkChoreMembers: every referenced member belongs to the household.
func (app *App) checkChoreMembers(ctx context.Context, aid int64, c choreRow) error {
	ids := append([]int64{}, c.RotationMemberIDs...)
	if c.MemberID != nil {
		ids = append(ids, *c.MemberID)
	}
	if len(ids) == 0 {
		return nil
	}
	var n int
	if err := app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM members WHERE account_id=$1 AND id = ANY($2)", aid, ids).Scan(&n); err != nil {
		return err
	}
	if n != len(ids) {
		return errors.New("assignee is not a member of this household")
	}
	return nil
}

type choreMemberRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func (app *App) choreMembers(ctx context.Context, aid int64) []choreMemberRef {
	out := []choreMemberRef{}
	rows, err := app.DB.Query(ctx, `SELECT id, COALESCE(NULLIF(display_name,''), split_part(email,'@',1)), role FROM members
		WHERE account_id=$1 ORDER BY CASE role WHEN 'owner' THEN 0 WHEN 'parent' THEN 1 WHEN 'kid' THEN 2 ELSE 3 END, id`, aid)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var m choreMemberRef
		if rows.Scan(&m.ID, &m.Name, &m.Role) == nil {
			out = append(out, m)
		}
	}
	return out
}

func requireChoreManager(w http.ResponseWriter, r *http.Request) (memberInfo, bool) {
	me, ok := requireMember(w, r)
	if !ok {
		return me, false
	}
	if !canManageFamily(me.Role) {
		jsonError(w, http.StatusForbidden, "only family admins and parents can manage chores")
		return me, false
	}
	return me, true
}

// GET /api/chores -> {chores, members, today}
func (app *App) handleListChores(w http.ResponseWriter, r *http.Request) {
	me, ok := requireMember(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	rows, err := app.DB.Query(ctx, "SELECT "+choreCols+" FROM chores WHERE account_id=$1 AND deleted_at IS NULL ORDER BY active DESC, title, id", me.AccountID)
	if err != nil {
		dbError(w, err)
		return
	}
	list := []choreRow{}
	for rows.Next() {
		c, err := scanChore(rows)
		if err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		list = append(list, c)
	}
	rows.Close()
	writeJSON(w, http.StatusOK, map[string]any{
		"chores":     list,
		"members":    app.choreMembers(ctx, me.AccountID),
		"today":      householdToday(ctx, app, me.AccountID, time.Now()).Format(dateLayout),
		"can_manage": canManageFamily(me.Role),
	})
}

func (app *App) loadChore(ctx context.Context, aid, id int64) (choreRow, error) {
	return scanChore(app.DB.QueryRow(ctx, "SELECT "+choreCols+" FROM chores WHERE id=$1 AND account_id=$2 AND deleted_at IS NULL", id, aid))
}

// POST /api/chores
func (app *App) handleCreateChore(w http.ResponseWriter, r *http.Request) {
	me, ok := requireChoreManager(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var d map[string]any
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	today := householdToday(ctx, app, me.AccountID, time.Now())
	c := choreRow{Points: 10, Schedule: choreOnce, StartDate: today.Format(dateLayout), Active: true,
		RotationMemberIDs: []int64{}, Weekdays: []int{}}
	if err := applyChoreInput(&c, d); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := app.checkChoreMembers(ctx, me.AccountID, c); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	var createdBy *int64
	if me.ID != 0 {
		createdBy = &me.ID
	}
	var id int64
	err := app.DB.QueryRow(ctx, `INSERT INTO chores (account_id, title, description, emoji, points, reward_amount, member_id,
		rotation_member_ids, schedule, weekdays, day_of_month, start_date, end_date, due_time, requires_approval, requires_photo,
		active, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::date,$13::date,$14,$15,$16,$17,$18) RETURNING id`,
		me.AccountID, c.Title, c.Description, c.Emoji, c.Points, c.RewardAmount, c.MemberID, c.RotationMemberIDs, c.Schedule,
		c.Weekdays, c.DayOfMonth, c.StartDate, c.EndDate, c.DueTime, c.RequiresApproval, c.RequiresPhoto, c.Active, createdBy).Scan(&id)
	if err != nil {
		dbError(w, err)
		return
	}
	if err := app.ensureChoreInstances(ctx, me.AccountID, today); err != nil {
		log.Printf("chores: generate after create: %v", err)
	}
	c, err = app.loadChore(ctx, me.AccountID, id)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "chore": c})
}

// PATCH /api/chores/{id} (chore id)
func (app *App) handleUpdateChore(w http.ResponseWriter, r *http.Request) {
	me, ok := requireChoreManager(w, r)
	if !ok {
		return
	}
	id, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := r.Context()
	before, err := app.loadChore(ctx, me.AccountID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "chore not found")
		return
	} else if err != nil {
		dbError(w, err)
		return
	}
	var d map[string]any
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	c := before
	c.RotationMemberIDs = append([]int64{}, before.RotationMemberIDs...)
	c.Weekdays = append([]int{}, before.Weekdays...)
	if err := applyChoreInput(&c, d); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := app.checkChoreMembers(ctx, me.AccountID, c); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	today := householdToday(ctx, app, me.AccountID, time.Now())
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		dbError(w, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	cursor := before.RotationCursor
	resetGen := before.structuralKey() != c.structuralKey()
	if resetGen {
		// Rebuild not-yet-done occurrences from today on; done/approved
		// history is never touched.
		tag, err := tx.Exec(ctx, "DELETE FROM chore_instances WHERE chore_id=$1 AND account_id=$2 AND status='todo' AND due_date >= $3", id, me.AccountID, today)
		if err != nil {
			dbError(w, err)
			return
		}
		if len(before.RotationMemberIDs) > 0 {
			cursor = max(0, cursor-int(tag.RowsAffected()))
		}
	}
	_, err = tx.Exec(ctx, `UPDATE chores SET title=$1, description=$2, emoji=$3, points=$4, reward_amount=$5, member_id=$6,
		rotation_member_ids=$7, schedule=$8, weekdays=$9, day_of_month=$10, start_date=$11::date, end_date=$12::date, due_time=$13,
		requires_approval=$14, requires_photo=$15, active=$16, rotation_cursor=$17,
		last_generated_date = CASE WHEN $18 AND last_generated_date >= $19::date THEN $19::date - 1 ELSE last_generated_date END
		WHERE id=$20 AND account_id=$21`,
		c.Title, c.Description, c.Emoji, c.Points, c.RewardAmount, c.MemberID, c.RotationMemberIDs, c.Schedule, c.Weekdays,
		c.DayOfMonth, c.StartDate, c.EndDate, c.DueTime, c.RequiresApproval, c.RequiresPhoto, c.Active, cursor,
		resetGen, today.Format(dateLayout), id, me.AccountID)
	if err != nil {
		dbError(w, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		dbError(w, err)
		return
	}
	if err := app.ensureChoreInstances(ctx, me.AccountID, today); err != nil {
		log.Printf("chores: generate after update: %v", err)
	}
	c, _ = app.loadChore(ctx, me.AccountID, id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "chore": c})
}

// DELETE /api/chores/{id} (chore id): open occurrences go away; a chore with
// history is archived so past points/earnings and the week view stay.
func (app *App) handleDeleteChore(w http.ResponseWriter, r *http.Request) {
	me, ok := requireChoreManager(w, r)
	if !ok {
		return
	}
	id, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := r.Context()
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		dbError(w, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT TRUE FROM chores WHERE id=$1 AND account_id=$2 AND deleted_at IS NULL FOR UPDATE", id, me.AccountID).Scan(&exists); err != nil {
		jsonError(w, http.StatusNotFound, "chore not found")
		return
	}
	if _, err := tx.Exec(ctx, "DELETE FROM chore_instances WHERE chore_id=$1 AND status='todo'", id); err != nil {
		dbError(w, err)
		return
	}
	var history bool
	_ = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM chore_instances WHERE chore_id=$1)", id).Scan(&history)
	if history {
		_, err = tx.Exec(ctx, "UPDATE chores SET deleted_at=now(), active=FALSE WHERE id=$1", id)
	} else {
		_, err = tx.Exec(ctx, "DELETE FROM chores WHERE id=$1", id)
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "archived": history})
}

// instance generation

// ensureChoreInstances creates every due occurrence up to `today` (the
// household-local date) for the household's active chores, then marks
// stale todo occurrences (superseded by a newer one) as missed. Idempotent
// and cheap when already up to date.
func (app *App) ensureChoreInstances(ctx context.Context, aid int64, today time.Time) error {
	today = choreDay(today)
	rows, err := app.DB.Query(ctx, `SELECT id FROM chores WHERE account_id=$1 AND active AND deleted_at IS NULL
		AND (last_generated_date IS NULL OR last_generated_date < $2)`, aid, today)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	if len(ids) > 0 {
		valid := map[int64]bool{}
		if mrows, err := app.DB.Query(ctx, "SELECT id FROM members WHERE account_id=$1", aid); err == nil {
			for mrows.Next() {
				var id int64
				if mrows.Scan(&id) == nil {
					valid[id] = true
				}
			}
			mrows.Close()
		}
		for _, id := range ids {
			if err := app.generateChore(ctx, aid, id, today, valid); err != nil {
				return err
			}
		}
	}
	_, err = app.DB.Exec(ctx, `UPDATE chore_instances ci SET status='missed'
		WHERE ci.account_id=$1 AND ci.status='todo' AND ci.due_date < $2
		  AND EXISTS (SELECT 1 FROM chore_instances n WHERE n.chore_id=ci.chore_id AND n.due_date > ci.due_date AND n.due_date <= $2)`,
		aid, today)
	return err
}

func (app *App) generateChore(ctx context.Context, aid, id int64, today time.Time, valid map[int64]bool) error {
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	c, err := scanChore(tx.QueryRow(ctx, "SELECT "+choreCols+" FROM chores WHERE id=$1 AND account_id=$2 AND active AND deleted_at IS NULL FOR UPDATE", id, aid))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	var lastGen *time.Time
	if err := tx.QueryRow(ctx, "SELECT last_generated_date FROM chores WHERE id=$1", id).Scan(&lastGen); err != nil {
		return err
	}
	sched := c.schedule()
	cursor := c.RotationCursor
	if from, to, ok := generationWindow(sched.Start, lastGen, today); ok {
		rotation := []int64{}
		for _, m := range c.RotationMemberIDs {
			if valid[m] {
				rotation = append(rotation, m)
			}
		}
		fixed := c.MemberID
		if fixed != nil && !valid[*fixed] {
			fixed = nil
		}
		for _, day := range sched.dueDates(from, to) {
			who, next := assignOccurrences(fixed, rotation, cursor, 1)
			tag, err := tx.Exec(ctx, `INSERT INTO chore_instances (account_id, chore_id, due_date, member_id) VALUES ($1,$2,$3,$4)
				ON CONFLICT (chore_id, due_date) DO NOTHING`, aid, id, day, who[0])
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 1 {
				cursor = next
			}
		}
	}
	if _, err := tx.Exec(ctx, "UPDATE chores SET last_generated_date=$1, rotation_cursor=$2 WHERE id=$3", today, cursor, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// choreWorkerInterval: background generation so the morning digest / bots
// see today's chores even if nobody opened the app.
const choreWorkerInterval = time.Hour

func (app *App) runChoreGeneration(ctx context.Context, now time.Time) error {
	rows, err := app.DB.Query(ctx, "SELECT DISTINCT account_id FROM chores WHERE active AND deleted_at IS NULL")
	if err != nil {
		return err
	}
	var aids []int64
	for rows.Next() {
		var a int64
		if rows.Scan(&a) == nil {
			aids = append(aids, a)
		}
	}
	rows.Close()
	for _, aid := range aids {
		if err := app.ensureChoreInstances(ctx, aid, householdToday(ctx, app, aid, now)); err != nil {
			log.Printf("chore worker: account %d: %v", aid, err)
		}
	}
	return nil
}

// startChoreWorker runs generation on start and hourly (panic-safe).
func (app *App) startChoreWorker(ctx context.Context) {
	go func() {
		run := func() {
			defer func() {
				if p := recover(); p != nil {
					log.Printf("chore worker: panic: %v", p)
				}
			}()
			c, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			if err := app.runChoreGeneration(c, time.Now()); err != nil {
				log.Printf("chore worker: %v", err)
			}
		}
		run()
		t := time.NewTicker(choreWorkerInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				run()
			}
		}
	}()
}

// instances

type choreInstanceView struct {
	ID               int64      `json:"id"` // 0 for projected (future, not generated yet)
	ChoreID          int64      `json:"chore_id"`
	Title            string     `json:"title"`
	Emoji            string     `json:"emoji"`
	Points           int        `json:"points"`
	RewardAmount     *float64   `json:"reward_amount"`
	DueDate          string     `json:"due_date"`
	DueTime          *string    `json:"due_time"`
	MemberID         *int64     `json:"member_id"`
	MemberName       string     `json:"member_name"`
	Status           string     `json:"status"`
	Done             bool       `json:"done"`
	Overdue          bool       `json:"overdue"`
	CompletedAt      *time.Time `json:"completed_at"`
	ApprovedAt       *time.Time `json:"approved_at"`
	ApprovedByName   string     `json:"approved_by_name,omitempty"`
	HasPhoto         bool       `json:"has_photo"`
	Note             string     `json:"note"`
	RequiresApproval bool       `json:"requires_approval"`
	RequiresPhoto    bool       `json:"requires_photo"`
	PointsAwarded    int        `json:"points_awarded"`
	Projected        bool       `json:"projected,omitempty"`
}

type instanceFilter struct {
	From, To  *time.Time // due_date range, inclusive
	TodayView *time.Time // due today, or overdue (todo/rejected) from earlier days
	MemberID  *int64
	// with MemberID: also include unassigned occurrences (anyone may do them)
	IncludeUnassigned bool
	Status            string
	InstanceID        int64
}

func (app *App) loadChoreInstances(ctx context.Context, aid int64, f instanceFilter, today time.Time) ([]choreInstanceView, error) {
	q := `SELECT ci.id, ci.chore_id, c.title, c.emoji, c.points, c.reward_amount::float8, ci.due_date, c.due_time,
		ci.member_id, COALESCE(NULLIF(m.display_name,''), split_part(m.email,'@',1), ''), ci.status, ci.completed_at, ci.approved_at,
		COALESCE(NULLIF(am.display_name,''), split_part(am.email,'@',1), ''), ci.photo_path IS NOT NULL, ci.note,
		c.requires_approval, c.requires_photo, ci.points_awarded
		FROM chore_instances ci JOIN chores c ON c.id = ci.chore_id
		LEFT JOIN members m ON m.id = ci.member_id LEFT JOIN members am ON am.id = ci.approved_by
		WHERE ci.account_id=$1`
	args := []any{aid}
	arg := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
	if f.InstanceID != 0 {
		q += " AND ci.id=" + arg(f.InstanceID)
	}
	if f.From != nil {
		q += " AND ci.due_date >= " + arg(*f.From)
	}
	if f.To != nil {
		q += " AND ci.due_date <= " + arg(*f.To)
	}
	if f.TodayView != nil {
		p := arg(*f.TodayView)
		q += " AND (ci.due_date = " + p + " OR (ci.due_date < " + p + " AND ci.status IN ('todo','rejected')))"
	}
	if f.MemberID != nil {
		p := arg(*f.MemberID)
		if f.IncludeUnassigned {
			q += " AND (ci.member_id = " + p + " OR ci.member_id IS NULL)"
		} else {
			q += " AND ci.member_id = " + p
		}
	}
	if f.Status != "" {
		q += " AND ci.status = " + arg(f.Status)
	}
	q += " ORDER BY ci.due_date, c.due_time NULLS LAST, c.title, ci.id"
	rows, err := app.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []choreInstanceView{}
	for rows.Next() {
		var v choreInstanceView
		var due time.Time
		if err := rows.Scan(&v.ID, &v.ChoreID, &v.Title, &v.Emoji, &v.Points, &v.RewardAmount, &due, &v.DueTime,
			&v.MemberID, &v.MemberName, &v.Status, &v.CompletedAt, &v.ApprovedAt, &v.ApprovedByName, &v.HasPhoto, &v.Note,
			&v.RequiresApproval, &v.RequiresPhoto, &v.PointsAwarded); err != nil {
			return nil, err
		}
		v.DueDate = due.Format(dateLayout)
		v.Done = choreIsDone(v.Status)
		v.Overdue = (v.Status == choreTodo || v.Status == choreRejected) && choreDay(due).Before(choreDay(today))
		out = append(out, v)
	}
	return out, rows.Err()
}

func parseDateParam(r *http.Request, key string) (*time.Time, bool) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil, true
	}
	t, err := time.Parse(dateLayout, v)
	if err != nil {
		return nil, false
	}
	return &t, true
}

// GET /api/chores/instances?from=&to=&member= -> {from, to, today, instances}
// Dates are household-local YYYY-MM-DD (default: this Mon..Sun, max 62
// days). Future days not generated yet come back as projected rows (id 0).
func (app *App) handleChoreInstances(w http.ResponseWriter, r *http.Request) {
	me, ok := requireMember(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	today := householdToday(ctx, app, me.AccountID, time.Now())
	from, okF := parseDateParam(r, "from")
	to, okT := parseDateParam(r, "to")
	if !okF || !okT {
		jsonError(w, http.StatusBadRequest, "from/to must be YYYY-MM-DD")
		return
	}
	if from == nil {
		ws := localWeekStart(today)
		from = &ws
	}
	if to == nil {
		e := from.AddDate(0, 0, 6)
		to = &e
	}
	if to.Before(*from) || to.Sub(*from) > 62*24*time.Hour {
		jsonError(w, http.StatusBadRequest, "range must be 0..62 days")
		return
	}
	var memberID *int64
	if v := r.URL.Query().Get("member"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid member")
			return
		}
		memberID = &n
	}
	if err := app.ensureChoreInstances(ctx, me.AccountID, today); err != nil {
		dbError(w, err)
		return
	}
	list, err := app.loadChoreInstances(ctx, me.AccountID, instanceFilter{From: from, To: to, MemberID: memberID}, today)
	if err != nil {
		dbError(w, err)
		return
	}
	if !to.Before(today.AddDate(0, 0, 1)) {
		proj, err := app.projectChores(ctx, me.AccountID, today.AddDate(0, 0, 1), *to, memberID)
		if err != nil {
			dbError(w, err)
			return
		}
		for _, p := range proj {
			if p.DueDate >= from.Format(dateLayout) {
				list = append(list, p)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from": from.Format(dateLayout), "to": to.Format(dateLayout), "today": today.Format(dateLayout),
		"instances": list, "members": app.choreMembers(ctx, me.AccountID),
	})
}

// projectChores previews future occurrences (after the last generated day)
// with the assignee rotation would pick. Nothing is stored.
func (app *App) projectChores(ctx context.Context, aid int64, from, to time.Time, memberID *int64) ([]choreInstanceView, error) {
	rows, err := app.DB.Query(ctx, "SELECT "+choreCols+", last_generated_date FROM chores WHERE account_id=$1 AND active AND deleted_at IS NULL ORDER BY title, id", aid)
	if err != nil {
		return nil, err
	}
	type withGen struct {
		c       choreRow
		lastGen *time.Time
	}
	var chores []withGen
	for rows.Next() {
		var c choreRow
		var lg *time.Time
		if err := rows.Scan(&c.ID, &c.Title, &c.Description, &c.Emoji, &c.Points, &c.RewardAmount, &c.MemberID, &c.RotationMemberIDs,
			&c.RotationCursor, &c.Schedule, &c.Weekdays, &c.DayOfMonth, &c.StartDate, &c.EndDate, &c.DueTime,
			&c.RequiresApproval, &c.RequiresPhoto, &c.Active, &c.CreatedAt, &lg); err != nil {
			rows.Close()
			return nil, err
		}
		chores = append(chores, withGen{c, lg})
	}
	rows.Close()
	names := map[int64]string{}
	for _, m := range app.choreMembers(ctx, aid) {
		names[m.ID] = m.Name
	}
	out := []choreInstanceView{}
	for _, wg := range chores {
		start := from
		if wg.lastGen != nil && !wg.lastGen.Before(start) {
			start = choreDay(*wg.lastGen).AddDate(0, 0, 1)
		}
		dates := wg.c.schedule().dueDates(start, to)
		rotation := []int64{}
		for _, id := range wg.c.RotationMemberIDs {
			if _, ok := names[id]; ok {
				rotation = append(rotation, id)
			}
		}
		who, _ := assignOccurrences(wg.c.MemberID, rotation, wg.c.RotationCursor, len(dates))
		for i, d := range dates {
			if memberID != nil && (who[i] == nil || *who[i] != *memberID) {
				continue
			}
			v := choreInstanceView{ChoreID: wg.c.ID, Title: wg.c.Title, Emoji: wg.c.Emoji, Points: wg.c.Points, RewardAmount: wg.c.RewardAmount,
				DueDate: d.Format(dateLayout), DueTime: wg.c.DueTime, MemberID: who[i], Status: choreTodo,
				RequiresApproval: wg.c.RequiresApproval, RequiresPhoto: wg.c.RequiresPhoto, Projected: true}
			if who[i] != nil {
				v.MemberName = names[*who[i]]
			}
			out = append(out, v)
		}
	}
	return out, nil
}

type choreWeekStats struct {
	WeekStart   string  `json:"week_start"`
	Points      int64   `json:"points"`       // chore points earned this (local) week by the viewer
	Done        int64   `json:"done"`         // chores done this week by the viewer
	Total       int64   `json:"total"`        // chores due this week so far for the viewer
	EarnedMonth float64 `json:"earned_month"` // chore money this (UTC) month
}

func (app *App) memberChoreWeek(ctx context.Context, aid, mid int64, today time.Time) choreWeekStats {
	ws := localWeekStart(today)
	st := choreWeekStats{WeekStart: ws.Format(dateLayout)}
	_ = app.DB.QueryRow(ctx, `SELECT COALESCE(SUM(points_awarded),0)::bigint,
		COUNT(*) FILTER (WHERE status IN ('submitted','approved')), COUNT(*)
		FROM chore_instances WHERE account_id=$1 AND member_id=$2 AND due_date >= $3 AND due_date < $4`,
		aid, mid, ws, ws.AddDate(0, 0, 7)).Scan(&st.Points, &st.Done, &st.Total)
	st.EarnedMonth = app.choreEarnedMonth(ctx, aid, mid, time.Now().UTC())
	return st
}

func (app *App) choreEarnedMonth(ctx context.Context, aid, mid int64, now time.Time) float64 {
	start, end := utcMonthBounds(now)
	var v float64
	_ = app.DB.QueryRow(ctx, "SELECT COALESCE(SUM(amount),0)::float8 FROM chore_earnings WHERE account_id=$1 AND member_id=$2 AND earned_at >= $3 AND earned_at < $4",
		aid, mid, start, end).Scan(&v)
	return v
}

// GET /api/chores/today -> {date, me, items, waiting, week, members}
//
//	items:   today's occurrences + overdue ones (kids: own + unassigned)
//	waiting: submitted occurrences waiting for approval (owner/parent only)
func (app *App) handleChoresToday(w http.ResponseWriter, r *http.Request) {
	me, ok := requireMember(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	today := householdToday(ctx, app, me.AccountID, time.Now())
	if err := app.ensureChoreInstances(ctx, me.AccountID, today); err != nil {
		dbError(w, err)
		return
	}
	f := instanceFilter{TodayView: &today}
	if me.Role == roleKid {
		f.MemberID, f.IncludeUnassigned = &me.ID, true
	}
	items, err := app.loadChoreInstances(ctx, me.AccountID, f, today)
	if err != nil {
		dbError(w, err)
		return
	}
	waiting := []choreInstanceView{}
	if canManageFamily(me.Role) {
		if waiting, err = app.loadChoreInstances(ctx, me.AccountID, instanceFilter{Status: choreSubmitted}, today); err != nil {
			dbError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date":    today.Format(dateLayout),
		"me":      map[string]any{"id": me.ID, "role": me.Role, "can_manage": canManageFamily(me.Role)},
		"items":   items,
		"waiting": waiting,
		"week":    app.memberChoreWeek(ctx, me.AccountID, me.ID, today),
		"members": app.choreMembers(ctx, me.AccountID),
	})
}

// actions (complete / approve / reject / undo)

type choreErr struct {
	status int
	msg    string
}

func (e *choreErr) Error() string { return e.msg }

type choreTransitionResult struct {
	Status      string
	MemberID    *int64
	MemberName  string
	Points      int
	PointsDelta int
	ForOther    bool // actor completed someone else's chore
}

// choreTransition applies an action to an instance in one transaction,
// including the reward ledger. photo (relative path, already stored) and
// note are used by complete (note also by reject). Returns the old photo
// path that should be removed from disk, if any.
func (app *App) choreTransition(ctx context.Context, aid int64, actor memberInfo, iid int64, action string, photo *string, note string) (choreTransitionResult, *string, error) {
	var res choreTransitionResult
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		return res, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status, title string
	var memberID, completedBy *int64
	var awarded, points int
	var oldPhoto *string
	var reward *float64
	var reqApproval, reqPhoto bool
	err = tx.QueryRow(ctx, `SELECT ci.status, ci.member_id, ci.completed_by, ci.points_awarded, ci.photo_path,
		c.points, c.reward_amount::float8, c.requires_approval, c.requires_photo, c.title
		FROM chore_instances ci JOIN chores c ON c.id = ci.chore_id
		WHERE ci.id=$1 AND ci.account_id=$2 FOR UPDATE OF ci`, iid, aid).
		Scan(&status, &memberID, &completedBy, &awarded, &oldPhoto, &points, &reward, &reqApproval, &reqPhoto, &title)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, nil, &choreErr{http.StatusNotFound, "chore not found"}
	} else if err != nil {
		return res, nil, err
	}
	manager := canManageFamily(actor.Role)
	own := memberID == nil || (actor.ID != 0 && *memberID == actor.ID)
	var actorPtr *int64
	if actor.ID != 0 {
		id := actor.ID
		actorPtr = &id
	}
	var removePhoto *string
	newStatus := status

	switch action {
	case "complete":
		if !manager && !own {
			return res, nil, &choreErr{http.StatusForbidden, "that chore belongs to someone else"}
		}
		if actor.Role == roleViewer {
			return res, nil, &choreErr{http.StatusForbidden, "viewers can't complete chores"}
		}
		if choreIsDone(status) {
			return res, nil, &choreErr{http.StatusConflict, "already done"}
		}
		if reqPhoto && photo == nil && oldPhoto == nil && !manager {
			return res, nil, &choreErr{http.StatusBadRequest, "this chore needs a photo. Take one to finish it"}
		}
		newStatus = choreSubmitted
		if !reqApproval || manager {
			newStatus = choreApproved
		}
		if memberID == nil {
			memberID = actorPtr
		}
		var approvedBy *int64
		if newStatus == choreApproved && reqApproval {
			approvedBy = actorPtr
		}
		if photo != nil && oldPhoto != nil {
			removePhoto = oldPhoto
		}
		_, err = tx.Exec(ctx, `UPDATE chore_instances SET status=$1, member_id=$2, completed_at=now(), completed_by=$3,
			approved_by=$4, approved_at=CASE WHEN $1='approved' THEN now() END, photo_path=COALESCE($5, photo_path), note=$6
			WHERE id=$7`, newStatus, memberID, actorPtr, approvedBy, photo, note, iid)
	case "approve":
		if !manager {
			return res, nil, &choreErr{http.StatusForbidden, "only family admins and parents can approve chores"}
		}
		if status != choreSubmitted {
			return res, nil, &choreErr{http.StatusConflict, "nothing to approve (status: " + status + ")"}
		}
		newStatus = choreApproved
		if memberID == nil {
			memberID = completedBy
		}
		_, err = tx.Exec(ctx, "UPDATE chore_instances SET status='approved', member_id=$1, approved_by=$2, approved_at=now() WHERE id=$3",
			memberID, actorPtr, iid)
	case "reject":
		if !manager {
			return res, nil, &choreErr{http.StatusForbidden, "only family admins and parents can reject chores"}
		}
		if !choreIsDone(status) {
			return res, nil, &choreErr{http.StatusConflict, "only a finished chore can be sent back (status: " + status + ")"}
		}
		newStatus = choreRejected
		_, err = tx.Exec(ctx, "UPDATE chore_instances SET status='rejected', approved_by=$1, approved_at=now(), note=$2 WHERE id=$3",
			actorPtr, note, iid)
	case "undo":
		if !manager {
			if !own {
				return res, nil, &choreErr{http.StatusForbidden, "that chore belongs to someone else"}
			}
			if status == choreApproved && reqApproval {
				return res, nil, &choreErr{http.StatusForbidden, "a parent already approved this. Ask them to undo it"}
			}
		}
		if status == choreTodo || status == choreMissed {
			return res, nil, &choreErr{http.StatusConflict, "nothing to undo"}
		}
		newStatus = choreTodo
		removePhoto = oldPhoto
		_, err = tx.Exec(ctx, `UPDATE chore_instances SET status='todo', completed_at=NULL, completed_by=NULL, approved_by=NULL,
			approved_at=NULL, photo_path=NULL, note='' WHERE id=$1`, iid)
	default:
		return res, nil, &choreErr{http.StatusBadRequest, "unknown action"}
	}
	if err != nil {
		return res, nil, err
	}

	// Reward ledger: move what this instance carries to its status' target.
	delta := choreRewardDelta(awarded, newStatus, points)
	if delta != 0 && memberID != nil {
		reason := "chore:" + strconv.FormatInt(iid, 10)
		if delta > 0 {
			_, err = tx.Exec(ctx, "INSERT INTO point_events (account_id, member_id, points, reason) VALUES ($1,$2,$3,$4)", aid, *memberID, delta, reason)
		} else {
			_, err = tx.Exec(ctx, "DELETE FROM point_events WHERE account_id=$1 AND reason=$2", aid, reason)
		}
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO member_points (member_id, account_id, points) VALUES ($1,$2,$3)
				ON CONFLICT (member_id) DO UPDATE SET points = member_points.points + EXCLUDED.points`, *memberID, aid, delta)
		}
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO user_points (account_id, points, streak_days) VALUES ($1,$2,0)
				ON CONFLICT (account_id) DO UPDATE SET points = user_points.points + EXCLUDED.points`, aid, delta)
		}
		if err == nil {
			_, err = tx.Exec(ctx, "UPDATE chore_instances SET points_awarded = points_awarded + $1 WHERE id=$2", delta, iid)
		}
		if err != nil {
			return res, nil, err
		}
		res.PointsDelta = delta
	}
	if newStatus == choreApproved && reward != nil && *reward > 0 && memberID != nil {
		_, err = tx.Exec(ctx, `INSERT INTO chore_earnings (account_id, member_id, instance_id, amount, description) VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (instance_id) DO NOTHING`, aid, *memberID, iid, *reward, title)
	} else if newStatus != choreApproved {
		_, err = tx.Exec(ctx, "DELETE FROM chore_earnings WHERE instance_id=$1", iid)
	}
	if err != nil {
		return res, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return res, nil, err
	}
	res.Status, res.MemberID, res.Points = newStatus, memberID, points
	res.ForOther = memberID != nil && (actor.ID == 0 || *memberID != actor.ID)
	if memberID != nil {
		_ = app.DB.QueryRow(ctx, "SELECT COALESCE(NULLIF(display_name,''), split_part(email,'@',1)) FROM members WHERE id=$1", *memberID).Scan(&res.MemberName)
	}
	return res, removePhoto, nil
}

func writeChoreErr(w http.ResponseWriter, err error) {
	var ce *choreErr
	if errors.As(err, &ce) {
		jsonError(w, ce.status, ce.msg)
		return
	}
	dbError(w, err)
}

func (app *App) writeChoreResult(w http.ResponseWriter, ctx context.Context, aid, iid int64, res choreTransitionResult, msg string) {
	today := householdToday(ctx, app, aid, time.Now())
	list, _ := app.loadChoreInstances(ctx, aid, instanceFilter{InstanceID: iid}, today)
	var inst *choreInstanceView
	if len(list) == 1 {
		inst = &list[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "message": msg, "status": res.Status, "points_delta": res.PointsDelta, "instance": inst,
	})
}

const maxChorePhotoBytes = 5 << 20

func choresPhotoRoot() string { return pathJoinClean(dataDir(), "chores") }

// chorePhotoAbsPath resolves a stored relative path, refusing anything that
// would escape DATA_DIR/chores.
func chorePhotoAbsPath(rel string) (string, bool) {
	root := choresPhotoRoot()
	p := pathJoinClean(root, rel)
	if !strings.HasPrefix(p, root+"/") {
		return "", false
	}
	return p, true
}

// POST /api/chores/{id}/complete (instance id). JSON {note} or multipart
// with fields photo (image) + note.
func (app *App) handleCompleteChore(w http.ResponseWriter, r *http.Request) {
	me, ok := requireMember(w, r)
	if !ok {
		return
	}
	iid, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := r.Context()
	var note string
	var photo *string
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		rel, n, cerr := storeChorePhoto(w, r, me.AccountID)
		if cerr != nil {
			jsonError(w, cerr.status, cerr.msg)
			return
		}
		photo, note = rel, n
	} else {
		var d struct {
			Note string `json:"note"`
		}
		_ = decodeJSON(r, &d) // empty body is fine
		note = d.Note
	}
	note = truncateRunes(strings.TrimSpace(note), 500)
	res, oldPhoto, err := app.choreTransition(ctx, me.AccountID, me, iid, "complete", photo, note)
	if err != nil {
		if photo != nil {
			removeChorePhoto(photo)
		}
		writeChoreErr(w, err)
		return
	}
	removeChorePhoto(oldPhoto)
	app.writeChoreResult(w, ctx, me.AccountID, iid, res, choreDoneMessage(res.MemberName, res.ForOther, res.Points, res.Status))
}

func truncateRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func storeChorePhoto(w http.ResponseWriter, r *http.Request, aid int64) (*string, string, *choreErr) {
	r.Body = http.MaxBytesReader(w, r.Body, maxChorePhotoBytes+64<<10)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		return nil, "", &choreErr{http.StatusRequestEntityTooLarge, "upload too large or malformed (max 5 MB)"}
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	note := r.FormValue("note")
	f, _, err := r.FormFile("photo")
	if err != nil {
		if f, _, err = r.FormFile("file"); err != nil {
			return nil, note, nil // note only
		}
	}
	defer f.Close()
	data, err := readLimited(f, maxChorePhotoBytes)
	if err != nil {
		return nil, "", &choreErr{http.StatusRequestEntityTooLarge, "photo too large (max 5 MB)"}
	}
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	_, ext, ok := sniffImage(head)
	if !ok {
		return nil, "", &choreErr{http.StatusUnsupportedMediaType, "unsupported image type (use JPEG, PNG, WebP or HEIC)"}
	}
	rel := strconv.FormatInt(aid, 10) + "/" + randomHex(16) + "." + ext
	abs, _ := chorePhotoAbsPath(rel)
	if err := writeFileMkdir(abs, data); err != nil {
		return nil, "", &choreErr{http.StatusInternalServerError, "could not store photo"}
	}
	return &rel, note, nil
}

func removeChorePhoto(rel *string) {
	if rel == nil || *rel == "" {
		return
	}
	if p, ok := chorePhotoAbsPath(*rel); ok {
		removeFileQuiet(p)
	}
}

// POST /api/chores/{id}/approve|reject|undo (instance id). reject takes {note}.
func (app *App) choreActionHandler(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := requireMember(w, r)
		if !ok {
			return
		}
		iid, ok := pathInt64(r, "id")
		if !ok {
			jsonError(w, http.StatusBadRequest, "invalid id")
			return
		}
		var d struct {
			Note string `json:"note"`
		}
		_ = decodeJSON(r, &d)
		ctx := r.Context()
		res, oldPhoto, err := app.choreTransition(ctx, me.AccountID, me, iid, action, nil, truncateRunes(strings.TrimSpace(d.Note), 500))
		if err != nil {
			writeChoreErr(w, err)
			return
		}
		removeChorePhoto(oldPhoto)
		msg := map[string]string{"approve": "Approved", "reject": "Sent back", "undo": "Undone"}[action]
		if action == "approve" && res.MemberName != "" {
			msg = fmt.Sprintf("Approved - +%d points for %s", res.Points, res.MemberName)
		}
		app.writeChoreResult(w, ctx, me.AccountID, iid, res, msg)
	}
}

// GET /api/chores/{id}/photo (instance id)
func (app *App) handleChorePhoto(w http.ResponseWriter, r *http.Request) {
	me, ok := requireMember(w, r)
	if !ok {
		return
	}
	iid, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var rel *string
	if err := app.DB.QueryRow(r.Context(), "SELECT photo_path FROM chore_instances WHERE id=$1 AND account_id=$2", iid, me.AccountID).Scan(&rel); err != nil || rel == nil {
		jsonError(w, http.StatusNotFound, "no photo")
		return
	}
	abs, ok := chorePhotoAbsPath(*rel)
	if !ok {
		jsonError(w, http.StatusNotFound, "no photo")
		return
	}
	serveImageFile(w, r, abs)
}

// Today provider

func choresTodayProvider(ctx context.Context, app *App, aid int64, member memberInfo, day time.Time) (*TodaySection, error) {
	if err := app.ensureChoreInstances(ctx, aid, day); err != nil {
		return nil, err
	}
	f := instanceFilter{TodayView: &day}
	if member.Role == roleKid {
		f.MemberID, f.IncludeUnassigned = &member.ID, true
	}
	list, err := app.loadChoreInstances(ctx, aid, f, day)
	if err != nil {
		return nil, err
	}
	manager := canManageFamily(member.Role)
	seen := map[int64]bool{}
	if manager {
		waiting, err := app.loadChoreInstances(ctx, aid, instanceFilter{Status: choreSubmitted}, day)
		if err != nil {
			return nil, err
		}
		for _, v := range list {
			seen[v.ID] = true
		}
		for _, v := range waiting {
			if !seen[v.ID] {
				list = append(list, v)
			}
		}
	}
	if len(list) == 0 {
		return nil, nil
	}
	loc := householdLocation(ctx, app, aid)
	sec := &TodaySection{Key: "chores", Title: "Chores", Order: 20, Items: []TodayItem{}}
	waitingN := 0
	for _, v := range list {
		title := v.Title
		it := TodayItem{Kind: "chore", ID: v.ID, Title: title, MemberID: v.MemberID, MemberName: v.MemberName,
			Done: v.Done, Overdue: v.Overdue, Link: "/home/chores"}
		due, _ := time.Parse(dateLayout, v.DueDate)
		if v.DueTime != nil {
			it.Due = dueInstant(due, *v.DueTime, loc)
		} else {
			it.Due = v.DueDate
		}
		parts := []string{}
		if v.Points > 0 {
			parts = append(parts, fmt.Sprintf("+%d pts", v.Points))
		}
		switch v.Status {
		case choreSubmitted:
			waitingN++
			parts = append(parts, "waiting for approval")
			if manager {
				it.Actions = []string{"approve"}
			}
		case choreRejected:
			if v.Note != "" {
				parts = append(parts, "try again: "+v.Note)
			} else {
				parts = append(parts, "try again")
			}
		case choreApproved:
			parts = append(parts, "done")
		}
		if v.Overdue {
			parts = append(parts, "from "+due.Format("Mon 2 Jan"))
		}
		if (v.Status == choreTodo || v.Status == choreRejected) && member.Role != roleViewer &&
			(manager || v.MemberID == nil || *v.MemberID == member.ID) {
			it.Actions = []string{"done"}
		}
		it.Detail = strings.Join(parts, " · ")
		sec.Items = append(sec.Items, it)
	}
	if manager && waitingN > 0 {
		sec.Summary = fmt.Sprintf("%d waiting for approval", waitingN)
	}
	return sec, nil
}

// bot endpoints

// GET /api/bot/chores/today -> {items:[{id,title,assignee_name,done,points}]}
// If X-Bot-User links to a kid, only their chores.
func botChoresToday(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	ctx := r.Context()
	today := householdToday(ctx, app, aid, time.Now())
	if err := app.ensureChoreInstances(ctx, aid, today); err != nil {
		dbError(w, err)
		return
	}
	f := instanceFilter{TodayView: &today}
	if actor, ok := botActor(ctx, app, r, aid); ok && actor.Role == roleKid {
		f.MemberID = &actor.ID
	}
	list, err := app.loadChoreInstances(ctx, aid, f, today)
	if err != nil {
		dbError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, v := range list {
		title := v.Title
		name := v.MemberName
		if v.MemberID == nil {
			name = "Anyone"
		}
		items = append(items, map[string]any{"id": v.ID, "title": title, "assignee_name": name, "done": v.Done, "points": v.Points,
			"status": v.Status, "overdue": v.Overdue, "due_date": v.DueDate})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/bot/chores/{id}/done (instance id) -> {ok, message}
// Requires a linked chat account (X-Bot-User); kids only their own.
func botChoreDone(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	ctx := r.Context()
	iid, ok := pathInt64(r, "id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "Invalid chore id.", "error": "invalid id"})
		return
	}
	actor, ok := botActor(ctx, app, r, aid)
	if !ok {
		msg := "I don't know who you are yet. Link your chat account in Kinkit first."
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "message": msg, "error": msg})
		return
	}
	if actor.Role == roleViewer {
		msg := "Viewers can't complete chores."
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "message": msg, "error": msg})
		return
	}
	res, _, err := app.choreTransition(ctx, aid, actor, iid, "complete", nil, "")
	if err != nil {
		var ce *choreErr
		if errors.As(err, &ce) {
			msg := ce.msg
			switch ce.status {
			case http.StatusNotFound:
				msg = "I couldn't find that chore."
			case http.StatusConflict:
				msg = "That chore is already done."
			case http.StatusBadRequest:
				msg = "This chore needs a photo. Finish it in the Kinkit app."
			case http.StatusForbidden:
				msg = "That chore belongs to someone else."
			}
			writeJSON(w, ce.status, map[string]any{"ok": false, "message": msg, "error": ce.msg})
			return
		}
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": choreDoneMessage(res.MemberName, res.ForOther, res.Points, res.Status),
		"status": res.Status, "points": res.Points})
}
