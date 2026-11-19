package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

// Family / households.
//
// A household is still one `users` row ("account") and every finance row is
// keyed by account_id. The people who log in are `members` (owner, parent,
// kid, viewer). memberMiddleware resolves the acting member on every /api
// request and enforces role rules centrally, so individual handlers don't
// need to know about roles.

const (
	roleOwner  = "owner"
	roleParent = "parent"
	roleKid    = "kid"
	roleViewer = "viewer"
)

func validRole(r string) bool {
	switch r {
	case roleOwner, roleParent, roleKid, roleViewer:
		return true
	}
	return false
}

// canManageFamily: owners and parents run the household.
func canManageFamily(role string) bool { return role == roleOwner || role == roleParent }

// ---------- role middleware ----------

type roleDecision int

const (
	roleAllow roleDecision = iota
	roleDeny
	roleOwnSpending // allowed only if the spending row in question was logged by this member
)

// roleAllowedPaths: non-GET endpoints every member may call regardless of
// role (session + own profile).
func alwaysAllowed(method, path string) bool {
	switch {
	case method == http.MethodPost && (path == "/api/login" || path == "/api/signup" || path == "/api/logout"):
		return true
	case method == http.MethodPost && strings.HasPrefix(path, "/api/password-reset/"):
		return true
	case method == http.MethodPatch && path == "/api/me/settings":
		return true
	case method == http.MethodPost && path == "/api/me/password":
		return true
	// Everyone may propose a meal and withdraw their own request; deciding
	// (approve/decline) stays owner/parent only and the handlers re-check.
	case method == http.MethodPost && path == "/api/meal-plan/proposals":
		return true
	case method == http.MethodDelete && strings.HasPrefix(path, "/api/meal-plan/proposals/"):
		return true
	}
	return false
}

// spendingIDFromPath parses "/api/spending/{id}[/...]" -> id, rest.
func spendingIDFromPath(path string) (int64, string, bool) {
	rest, ok := strings.CutPrefix(path, "/api/spending/")
	if !ok {
		return 0, "", false
	}
	idPart, tail, _ := strings.Cut(rest, "/")
	id, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil {
		return 0, "", false
	}
	return id, tail, true
}

// decideRole is the pure role policy (unit tested):
//
//	owner/parent: everything (admin endpoints still need users.is_admin)
//	viewer:       read-only (+ session/own-profile endpoints)
//	kid:          read; POST /api/spending; receipts upload on own rows;
//	              PATCH/DELETE only on spending rows they logged
func decideRole(role, method, path string) (roleDecision, int64) {
	if canManageFamily(role) {
		return roleAllow, 0
	}
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
		return roleAllow, 0
	}
	if alwaysAllowed(method, path) {
		return roleAllow, 0
	}
	if role != roleKid {
		return roleDeny, 0
	}
	if method == http.MethodPost && (path == "/api/spending" || path == "/api/receipts" || strings.HasPrefix(path, "/api/receipts/")) {
		return roleAllow, 0
	}
	// Feature files can allow extra kid actions (chores, lists, reminders)
	// without editing this function - see registerKidRule in home_contract.go.
	for _, rule := range kidRules {
		if d, id, ok := rule(method, path); ok {
			return d, id
		}
	}
	if id, tail, ok := spendingIDFromPath(path); ok {
		switch {
		case tail == "" && (method == http.MethodPatch || method == http.MethodDelete):
			return roleOwnSpending, id
		case strings.HasPrefix(tail, "receipt") && (method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete):
			return roleOwnSpending, id
		}
	}
	return roleDeny, 0
}

// sessionOnlyPaths may run even when the session's member no longer exists
// (they overwrite or report on the session).
func sessionOnlyPath(path string) bool {
	switch path {
	case "/api/login", "/api/signup", "/api/logout", "/api/me":
		return true
	}
	return false
}

// memberMiddleware resolves the acting member for cookie-authenticated
// requests and applies decideRole. Token-authenticated bot/internal routes
// are left alone.
func (app *App) memberMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/bot/") || strings.HasPrefix(path, "/api/internal/") {
			next.ServeHTTP(w, r)
			return
		}
		aid, ok := currentAccountID(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()
		var m memberInfo
		if mid, has := sessionMemberID(r); has {
			err := app.DB.QueryRow(ctx, "SELECT id, account_id, role FROM members WHERE id=$1 AND account_id=$2", mid, aid).
				Scan(&m.ID, &m.AccountID, &m.Role)
			if err != nil {
				// member removed (or household deleted): session is dead
				if sessionOnlyPath(path) {
					next.ServeHTTP(w, r)
					return
				}
				_ = clearSession(w, r)
				jsonError(w, http.StatusUnauthorized, "session expired, please log in again")
				return
			}
		} else {
			// Pre-family session cookie: act as the household owner and
			// upgrade the cookie in place.
			var display *string
			err := app.DB.QueryRow(ctx, "SELECT id, account_id, role, display_name FROM members WHERE account_id=$1 AND role='owner'", aid).
				Scan(&m.ID, &m.AccountID, &m.Role, &display)
			if err != nil {
				m = memberInfo{AccountID: aid, Role: roleOwner}
			} else {
				s := getSession(r)
				s.Values["member_id"] = m.ID
				s.Values["role"] = m.Role
				_ = s.Save(r, w)
			}
		}
		r = withMember(r, m)

		decision, sid := decideRole(m.Role, r.Method, path)
		switch decision {
		case roleDeny:
			jsonError(w, http.StatusForbidden, "your role ("+m.Role+") can't do that")
			return
		case roleOwnSpending:
			var owner *int64
			err := app.DB.QueryRow(ctx, "SELECT member_id FROM spending WHERE id=$1 AND account_id=$2", sid, aid).Scan(&owner)
			if err != nil || owner == nil || *owner != m.ID {
				jsonError(w, http.StatusForbidden, "you can only change spending you logged yourself")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// requireMember is the handler-side counterpart of requireAccount.
func requireMember(w http.ResponseWriter, r *http.Request) (memberInfo, bool) {
	m, ok := currentMember(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return memberInfo{}, false
	}
	return m, true
}

// ---------- routes ----------

func RegisterFamilyRoutes(r chi.Router, app *App) {
	r.Get("/family", app.handleGetFamily)
	r.Patch("/family", app.handleUpdateHousehold)
	r.Post("/family/members", app.handleCreateMember)
	r.Patch("/family/members/{id}", app.handleUpdateMember)
	r.Delete("/family/members/{id}", app.handleDeleteMember)
	r.Get("/family/members/{id}/allowance", app.handleMemberAllowance)
	r.Post("/me/password", app.handleChangeOwnPassword)
}

type familyMember struct {
	ID               int64     `json:"id"`
	Email            string    `json:"email"`
	DisplayName      string    `json:"display_name"`
	Role             string    `json:"role"`
	AllowanceMonthly *float64  `json:"allowance_monthly"`
	CreatedAt        time.Time `json:"created_at"`
	SpentThisMonth   int64     `json:"spent_this_month"`
	PointsThisMonth  int64     `json:"points_this_month"`
	IsMe             bool      `json:"is_me"`
	// --- chores ---
	ChoresDoneWeek   int64    `json:"chores_done_week"`    // done (submitted/approved) this household-local week
	ChoreEarnedMonth float64  `json:"chore_earned_month"`  // chore rewards this UTC month
	EffectiveAllow   *float64 `json:"effective_allowance"` // allowance_monthly + chore_earned_month (nil when both are 0)
}

// utcMonthBounds returns [first day of t's UTC month, first day of next month).
func utcMonthBounds(t time.Time) (time.Time, time.Time) {
	t = t.UTC()
	start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}

func householdName(stored, ownerName *string) string {
	if stored != nil && strings.TrimSpace(*stored) != "" {
		return *stored
	}
	if ownerName != nil && *ownerName != "" {
		return *ownerName + "'s Family"
	}
	return "My Family"
}

func (app *App) loadHouseholdName(ctx context.Context, aid int64) string {
	var stored, owner *string
	_ = app.DB.QueryRow(ctx, "SELECT household_name, display_name FROM users WHERE id=$1", aid).Scan(&stored, &owner)
	return householdName(stored, owner)
}

func (app *App) listFamilyMembers(ctx context.Context, aid, meID int64) ([]familyMember, error) {
	now := time.Now().UTC()
	start, end := utcMonthBounds(now)
	weekStart := localWeekStart(householdToday(ctx, app, aid, now)) // --- chores ---
	rows, err := app.DB.Query(ctx, `
		SELECT m.id, m.email, COALESCE(m.display_name, ''), m.role, m.allowance_monthly::float8, m.created_at,
		       COALESCE((SELECT SUM(amount) FROM spending s WHERE s.account_id=m.account_id AND s.member_id=m.id
		                 AND s.tx_date >= $2 AND s.tx_date < $3), 0)::bigint,
		       COALESCE((SELECT SUM(points) FROM point_events pe WHERE pe.member_id=m.id AND pe.created_at >= $2), 0)::bigint,
		       (SELECT COUNT(*) FROM chore_instances ci WHERE ci.account_id=m.account_id AND ci.member_id=m.id
		                 AND ci.status IN ('submitted','approved') AND ci.due_date >= $4 AND ci.due_date < $5),
		       COALESCE((SELECT SUM(amount) FROM chore_earnings ce WHERE ce.account_id=m.account_id AND ce.member_id=m.id
		                 AND ce.earned_at >= $2 AND ce.earned_at < $3), 0)::float8
		FROM members m WHERE m.account_id=$1
		ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'parent' THEN 1 WHEN 'kid' THEN 2 ELSE 3 END, m.id`,
		aid, start, end, weekStart, weekStart.AddDate(0, 0, 7))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []familyMember{}
	for rows.Next() {
		var fm familyMember
		if err := rows.Scan(&fm.ID, &fm.Email, &fm.DisplayName, &fm.Role, &fm.AllowanceMonthly, &fm.CreatedAt,
			&fm.SpentThisMonth, &fm.PointsThisMonth, &fm.ChoresDoneWeek, &fm.ChoreEarnedMonth); err != nil {
			return nil, err
		}
		fm.EffectiveAllow = effectiveAllowance(fm.AllowanceMonthly, fm.ChoreEarnedMonth)
		if fm.DisplayName == "" {
			fm.DisplayName = strings.SplitN(fm.Email, "@", 2)[0]
		}
		fm.IsMe = fm.ID == meID
		out = append(out, fm)
	}
	return out, rows.Err()
}

// GET /api/family
func (app *App) handleGetFamily(w http.ResponseWriter, r *http.Request) {
	me, ok := requireMember(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	members, err := app.listFamilyMembers(ctx, me.AccountID, me.ID)
	if err != nil {
		dbError(w, err)
		return
	}
	var currency string
	_ = app.DB.QueryRow(ctx, "SELECT currency FROM users WHERE id=$1", me.AccountID).Scan(&currency)
	start, _ := utcMonthBounds(time.Now().UTC())
	writeJSON(w, http.StatusOK, map[string]any{
		"household":  map[string]any{"id": me.AccountID, "name": app.loadHouseholdName(ctx, me.AccountID), "currency": currency},
		"me":         map[string]any{"id": me.ID, "role": me.Role},
		"can_manage": canManageFamily(me.Role),
		"month":      start.Format("2006-01"),
		"members":    members,
	})
}

// PATCH /api/family {name} - owner only.
func (app *App) handleUpdateHousehold(w http.ResponseWriter, r *http.Request) {
	me, ok := requireMember(w, r)
	if !ok {
		return
	}
	if me.Role != roleOwner {
		jsonError(w, http.StatusForbidden, "only the family admin can rename the household")
		return
	}
	var d struct {
		Name string `json:"name"`
	}
	_ = decodeJSON(r, &d)
	name := strings.TrimSpace(d.Name)
	if len(name) > 80 {
		jsonError(w, http.StatusBadRequest, "name too long (max 80)")
		return
	}
	var v *string
	if name != "" {
		v = &name
	}
	if _, err := app.DB.Exec(r.Context(), "UPDATE users SET household_name=$1 WHERE id=$2", v, me.AccountID); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": app.loadHouseholdName(r.Context(), me.AccountID)})
}

// parseAllowance accepts a JSON number, numeric string or null; ok=false on garbage.
func parseAllowance(v any) (*float64, bool) {
	switch t := v.(type) {
	case nil:
		return nil, true
	case float64:
		if t < 0 {
			return nil, false
		}
		if t == 0 {
			return nil, true
		}
		return &t, true
	case string:
		if strings.TrimSpace(t) == "" {
			return nil, true
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil || f < 0 {
			return nil, false
		}
		if f == 0 {
			return nil, true
		}
		return &f, true
	}
	return nil, false
}

// POST /api/family/members {email, password, display_name, role, allowance_monthly}
// Creates the member directly with a temporary password the owner/parent
// hands over (no invitation email - there's no SMTP wired).
func (app *App) handleCreateMember(w http.ResponseWriter, r *http.Request) {
	me, ok := requireMember(w, r)
	if !ok {
		return
	}
	if !canManageFamily(me.Role) {
		jsonError(w, http.StatusForbidden, "only family admins and parents can add members")
		return
	}
	var d map[string]any
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	str := func(k string) string { s, _ := d[k].(string); return strings.TrimSpace(s) }
	email := strings.ToLower(str("email"))
	password, _ := d["password"].(string)
	role := str("role")
	if role == "" {
		role = roleKid
	}
	display := str("display_name")
	if email == "" || !strings.Contains(email, "@") || len(password) < 8 {
		jsonError(w, http.StatusBadRequest, "valid email and temporary password (8+ chars) required")
		return
	}
	if !validRole(role) || role == roleOwner {
		jsonError(w, http.StatusBadRequest, "role must be parent, kid or viewer")
		return
	}
	if display == "" {
		display = strings.SplitN(email, "@", 2)[0]
	}
	allowance, okA := parseAllowance(d["allowance_monthly"])
	if !okA {
		jsonError(w, http.StatusBadRequest, "allowance_monthly must be a non-negative number")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "could not hash password")
		return
	}
	ctx := r.Context()
	// An email that is some account's login already is a member email too
	// (owner rows), so the members.email UNIQUE covers both.
	var id int64
	err = app.DB.QueryRow(ctx,
		"INSERT INTO members (account_id, email, password_hash, display_name, role, allowance_monthly) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id",
		me.AccountID, email, string(hash), display, role, allowance).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			jsonError(w, http.StatusConflict, "that email already belongs to a Kinkit member")
			return
		}
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

// loadTarget fetches a member of the caller's household for edit/delete.
func (app *App) loadTarget(ctx context.Context, w http.ResponseWriter, r *http.Request, me memberInfo) (int64, string, bool) {
	id, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return 0, "", false
	}
	var role string
	if err := app.DB.QueryRow(ctx, "SELECT role FROM members WHERE id=$1 AND account_id=$2", id, me.AccountID).Scan(&role); err != nil {
		jsonError(w, http.StatusNotFound, "member not found")
		return 0, "", false
	}
	return id, role, true
}

// PATCH /api/family/members/{id} {role?, allowance_monthly?, display_name?, password?}
func (app *App) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	me, ok := requireMember(w, r)
	if !ok {
		return
	}
	if !canManageFamily(me.Role) {
		jsonError(w, http.StatusForbidden, "only family admins and parents can edit members")
		return
	}
	ctx := r.Context()
	id, targetRole, ok := app.loadTarget(ctx, w, r, me)
	if !ok {
		return
	}
	if targetRole == roleOwner && me.Role != roleOwner {
		jsonError(w, http.StatusForbidden, "parents can't edit the family admin")
		return
	}
	var d map[string]any
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	sets := []string{}
	args := []any{}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, col+"=$"+strconv.Itoa(len(args)))
	}
	if v, present := d["role"]; present {
		role, _ := v.(string)
		if !validRole(role) || role == roleOwner {
			jsonError(w, http.StatusBadRequest, "role must be parent, kid or viewer")
			return
		}
		if targetRole == roleOwner {
			jsonError(w, http.StatusBadRequest, "the family admin's role can't be changed")
			return
		}
		if id == me.ID && role != me.Role {
			jsonError(w, http.StatusBadRequest, "you can't change your own role")
			return
		}
		add("role", role)
	}
	if v, present := d["allowance_monthly"]; present {
		a, okA := parseAllowance(v)
		if !okA {
			jsonError(w, http.StatusBadRequest, "allowance_monthly must be a non-negative number")
			return
		}
		add("allowance_monthly", a)
	}
	if v, present := d["display_name"]; present {
		s, _ := v.(string)
		if s = strings.TrimSpace(s); s != "" {
			add("display_name", s)
		}
	}
	if v, present := d["password"]; present {
		p, _ := v.(string)
		if len(p) < 8 {
			jsonError(w, http.StatusBadRequest, "password must be 8+ chars")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "could not hash password")
			return
		}
		add("password_hash", string(hash))
	}
	if len(sets) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	args = append(args, id, me.AccountID)
	q := "UPDATE members SET " + strings.Join(sets, ",") + " WHERE id=$" + strconv.Itoa(len(args)-1) + " AND account_id=$" + strconv.Itoa(len(args))
	if _, err := app.DB.Exec(ctx, q, args...); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DELETE /api/family/members/{id} - the owner can't be removed; spending
// they logged stays (member_id -> NULL).
func (app *App) handleDeleteMember(w http.ResponseWriter, r *http.Request) {
	me, ok := requireMember(w, r)
	if !ok {
		return
	}
	if !canManageFamily(me.Role) {
		jsonError(w, http.StatusForbidden, "only family admins and parents can remove members")
		return
	}
	ctx := r.Context()
	id, targetRole, ok := app.loadTarget(ctx, w, r, me)
	if !ok {
		return
	}
	if targetRole == roleOwner {
		jsonError(w, http.StatusBadRequest, "the family admin can't be removed")
		return
	}
	if id == me.ID {
		jsonError(w, http.StatusBadRequest, "you can't remove yourself")
		return
	}
	if _, err := app.DB.Exec(ctx, "DELETE FROM members WHERE id=$1 AND account_id=$2 AND role<>'owner'", id, me.AccountID); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type allowanceStatus struct {
	MemberID         int64    `json:"member_id"`
	Month            string   `json:"month"` // YYYY-MM (UTC)
	AllowanceMonthly *float64 `json:"allowance_monthly"`
	Spent            int64    `json:"spent"`
	Remaining        *float64 `json:"remaining"`
	Percent          *float64 `json:"percent"`
	// --- chores --- effective allowance = allowance_monthly + chore_earned
	// (this UTC month); remaining/percent are against the effective amount.
	ChoreEarned        float64  `json:"chore_earned"`
	EffectiveAllowance *float64 `json:"effective_allowance"`
}

// effectiveAllowance: monthly allowance plus chore earnings; nil when there
// is neither.
func effectiveAllowance(allowance *float64, earned float64) *float64 {
	total := earned
	if allowance != nil {
		total += *allowance
	}
	if total <= 0 {
		return nil
	}
	return &total
}

func (app *App) memberAllowance(ctx context.Context, aid, mid int64, now time.Time) (allowanceStatus, error) {
	start, end := utcMonthBounds(now)
	st := allowanceStatus{MemberID: mid, Month: start.Format("2006-01")}
	err := app.DB.QueryRow(ctx, `
		SELECT m.allowance_monthly::float8,
		       COALESCE((SELECT SUM(amount) FROM spending s WHERE s.account_id=m.account_id AND s.member_id=m.id
		                 AND s.tx_date >= $3 AND s.tx_date < $4), 0)::bigint
		FROM members m WHERE m.id=$1 AND m.account_id=$2`, mid, aid, start, end).Scan(&st.AllowanceMonthly, &st.Spent)
	if err != nil {
		return st, err
	}
	st.ChoreEarned = app.choreEarnedMonth(ctx, aid, mid, now)
	st.EffectiveAllowance = effectiveAllowance(st.AllowanceMonthly, st.ChoreEarned)
	if eff := st.EffectiveAllowance; eff != nil {
		rem := *eff - float64(st.Spent)
		pct := float64(st.Spent) / *eff * 100
		st.Remaining, st.Percent = &rem, &pct
	}
	return st, nil
}

// GET /api/family/members/{id}/allowance - spent this UTC month by that
// member vs their allowance. Kids may only look at their own.
func (app *App) handleMemberAllowance(w http.ResponseWriter, r *http.Request) {
	me, ok := requireMember(w, r)
	if !ok {
		return
	}
	id, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if me.Role == roleKid && id != me.ID {
		jsonError(w, http.StatusForbidden, "you can only see your own allowance")
		return
	}
	st, err := app.memberAllowance(r.Context(), me.AccountID, id, time.Now().UTC())
	if err != nil {
		jsonError(w, http.StatusNotFound, "member not found")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// POST /api/me/password {current_password, new_password} - any member,
// e.g. to replace the temporary password they were given.
func (app *App) handleChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	me, ok := requireMember(w, r)
	if !ok {
		return
	}
	var d struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	_ = decodeJSON(r, &d)
	if len(d.New) < 8 {
		jsonError(w, http.StatusBadRequest, "new password must be 8+ chars")
		return
	}
	ctx := r.Context()
	var stored *string
	if err := app.DB.QueryRow(ctx, "SELECT password_hash FROM members WHERE id=$1", me.ID).Scan(&stored); err != nil {
		dbError(w, err)
		return
	}
	if stored == nil {
		jsonError(w, http.StatusBadRequest, "no password set for this login")
		return
	}
	if ok, _ := verifyPasswordHash(*stored, d.Current); !ok {
		jsonError(w, http.StatusUnauthorized, "current password is wrong")
		return
	}
	if err := app.setMemberPassword(ctx, me.ID, d.New); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// setMemberPassword bcrypt-hashes and stores a member's password, mirroring
// it onto users.password_hash for the owner so the account row stays in
// sync (admin tooling / older code still read it).
func (app *App) setMemberPassword(ctx context.Context, memberID int64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return app.storeMemberHash(ctx, memberID, string(hash))
}

func (app *App) storeMemberHash(ctx context.Context, memberID int64, hash string) error {
	var aid int64
	var role string
	if err := app.DB.QueryRow(ctx, "UPDATE members SET password_hash=$1 WHERE id=$2 RETURNING account_id, role", hash, memberID).Scan(&aid, &role); err != nil {
		return err
	}
	if role == roleOwner {
		_, err := app.DB.Exec(ctx, "UPDATE users SET password_hash=$1 WHERE id=$2", hash, aid)
		return err
	}
	return nil
}
