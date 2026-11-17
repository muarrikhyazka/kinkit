package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// ---------- pure ----------

func TestDecideRole(t *testing.T) {
	cases := []struct {
		role, method, path string
		want               roleDecision
		wantID             int64
	}{
		{"owner", "DELETE", "/api/categories/3", roleAllow, 0},
		{"parent", "POST", "/api/budgeting/expense", roleAllow, 0},
		{"viewer", "GET", "/api/spending", roleAllow, 0},
		{"viewer", "POST", "/api/spending", roleDeny, 0},
		{"viewer", "POST", "/api/logout", roleAllow, 0},
		{"viewer", "PATCH", "/api/me/settings", roleAllow, 0},
		{"viewer", "POST", "/api/me/password", roleAllow, 0},
		{"viewer", "PUT", "/api/dashboard-layout", roleDeny, 0},
		{"kid", "POST", "/api/spending", roleAllow, 0},
		{"kid", "POST", "/api/budgeting/expense", roleDeny, 0},
		{"kid", "POST", "/api/import/spending", roleDeny, 0},
		{"kid", "PATCH", "/api/spending/42", roleOwnSpending, 42},
		{"kid", "DELETE", "/api/spending/42", roleOwnSpending, 42},
		{"kid", "POST", "/api/spending/42/receipt", roleOwnSpending, 42},
		{"kid", "DELETE", "/api/spending/abc", roleDeny, 0},
		{"kid", "POST", "/api/family/members", roleDeny, 0},
		{"kid", "GET", "/api/family", roleAllow, 0},
	}
	for _, c := range cases {
		got, id := decideRole(c.role, c.method, c.path)
		if got != c.want || id != c.wantID {
			t.Errorf("%s %s %s: got (%v,%d) want (%v,%d)", c.role, c.method, c.path, got, id, c.want, c.wantID)
		}
	}
}

func TestEarnedBadges(t *testing.T) {
	if got := earnedBadges(badgeFacts{}); len(got) != 0 {
		t.Fatalf("no facts -> no badges, got %v", got)
	}
	got := earnedBadges(badgeFacts{HasLogged: true, BestStreak: 8, HasBudget: true,
		PrevExpenseBudget: 100, PrevExpenseSpent: 100, PrevTxCount: 3, PrevSavingsBudget: 50, PrevSavingsRealize: 60})
	for _, k := range []string{"first_log", "first_budget", "streak_7", "under_budget", "saver"} {
		if !slices.Contains(got, k) {
			t.Errorf("missing %s in %v", k, got)
		}
	}
	if slices.Contains(got, "streak_30") {
		t.Errorf("streak_30 with best streak 8: %v", got)
	}
	// over budget / no tx / savings short -> none of the month badges
	got = earnedBadges(badgeFacts{PrevExpenseBudget: 100, PrevExpenseSpent: 101, PrevTxCount: 3, PrevSavingsBudget: 50, PrevSavingsRealize: 49})
	if slices.Contains(got, "under_budget") || slices.Contains(got, "saver") {
		t.Errorf("unexpected month badges: %v", got)
	}
	if got := earnedBadges(badgeFacts{PrevExpenseBudget: 100, PrevTxCount: 0}); slices.Contains(got, "under_budget") {
		t.Errorf("a month with no logging must not be under_budget: %v", got)
	}
	if got := earnedBadges(badgeFacts{BestStreak: 30}); !slices.Contains(got, "streak_30") {
		t.Errorf("streak_30 missing: %v", got)
	}
}

func TestWeekStartUTC(t *testing.T) {
	// 2026-11-17 is a Tuesday -> Monday 2026-11-16
	if got := weekStartUTC(*d("2026-11-17")).Format(dateLayout); got != "2026-11-16" {
		t.Fatalf("got %s", got)
	}
	// Sunday belongs to the week that started the previous Monday
	if got := weekStartUTC(*d("2026-11-22")).Format(dateLayout); got != "2026-11-16" {
		t.Fatalf("sunday: got %s", got)
	}
	// late Sunday in UTC-5 is already Monday in UTC
	ny := time.FixedZone("UTC-5", -5*3600)
	if got := weekStartUTC(time.Date(2026, 11, 22, 22, 0, 0, 0, ny)).Format(dateLayout); got != "2026-11-23" {
		t.Fatalf("tz: got %s", got)
	}
}

func TestChallengeOutcome(t *testing.T) {
	if _, _, _, done := challengeOutcome("log_every_day", weekStats{DaysLogged: 6}); done {
		t.Error("6 days is not done")
	}
	if p, tg, _, done := challengeOutcome("log_every_day", weekStats{DaysLogged: 7}); !done || p != 7 || tg != 7 {
		t.Error("7 days should be done")
	}
	if _, _, _, done := challengeOutcome("no_spend_weekend", weekStats{Ended: false, WeekdayTx: 3}); done {
		t.Error("weekend not over yet")
	}
	if _, _, _, done := challengeOutcome("no_spend_weekend", weekStats{Ended: true, WeekdayTx: 0}); done {
		t.Error("no tracking during the week shouldn't count")
	}
	if _, _, _, done := challengeOutcome("no_spend_weekend", weekStats{Ended: true, WeekdayTx: 2, WeekendTx: 1}); done {
		t.Error("weekend spending fails the challenge")
	}
	if _, _, _, done := challengeOutcome("no_spend_weekend", weekStats{Ended: true, WeekdayTx: 2, WeekendPassed: 2}); !done {
		t.Error("clean weekend should complete")
	}
	if _, _, avail, _ := challengeOutcome("wants_under", weekStats{}); avail {
		t.Error("wants_under needs a Wants budget")
	}
	if _, _, _, done := challengeOutcome("wants_under", weekStats{WantsCap: 100, WantsSpent: 90, TotalTx: 1, Ended: true}); !done {
		t.Error("under cap at week end should complete")
	}
	if _, _, _, done := challengeOutcome("wants_under", weekStats{WantsCap: 100, WantsSpent: 90, TotalTx: 1}); done {
		t.Error("only final once the week is over")
	}
}

// ---------- integration ----------

// addMember creates a member via the API as the (owner/parent) client.
func addMember(t *testing.T, c *testClient, role string, allowance any) (email string, id int64) {
	t.Helper()
	email = uniqueEmail()
	var res map[string]any
	body := map[string]any{"email": email, "password": "temp-pass-123", "display_name": role + "-member", "role": role}
	if allowance != nil {
		body["allowance_monthly"] = allowance
	}
	if code := c.do("POST", "/api/family/members", body, &res); code != 200 {
		t.Fatalf("create %s member: %d %v", role, code, res)
	}
	return email, int64(res["id"].(float64))
}

func loginAs(t *testing.T, c *testClient, email, password string) {
	t.Helper()
	var res map[string]any
	if code := c.do("POST", "/api/login", map[string]any{"email": email, "password": password}, &res); code != 200 {
		t.Fatalf("login %s: %d %v", email, code, res)
	}
}

func TestFamilyMemberCreateAndLogin(t *testing.T) {
	_, srv := newTestServer(t)
	owner := newClient(t, srv)
	_, aid := owner.signup()

	var me map[string]any
	owner.do("GET", "/api/me", nil, &me)
	if me["role"] != "owner" || me["member_id"] == nil || me["household_name"] == nil {
		t.Fatalf("owner /me: %v", me)
	}

	kidEmail, kidID := addMember(t, owner, "kid", 150000)

	// duplicate email (any household) rejected
	if code := owner.do("POST", "/api/family/members", map[string]any{"email": kidEmail, "password": "temp-pass-123", "role": "viewer"}, nil); code != 409 {
		t.Fatalf("duplicate member email: want 409, got %d", code)
	}
	// owner role can't be granted; bad input
	if code := owner.do("POST", "/api/family/members", map[string]any{"email": uniqueEmail(), "password": "temp-pass-123", "role": "owner"}, nil); code != 400 {
		t.Fatalf("role owner: want 400, got %d", code)
	}
	// signup with a member's email is rejected too
	if code := newClient(t, srv).do("POST", "/api/signup", map[string]any{"email": kidEmail, "password": "password123"}, nil); code != 409 {
		t.Fatalf("signup with member email: want 409, got %d", code)
	}

	kid := newClient(t, srv)
	loginAs(t, kid, kidEmail, "temp-pass-123")
	me = nil
	kid.do("GET", "/api/me", nil, &me)
	if me["role"] != "kid" || me["username"] != kidEmail || int64(me["account_id"].(float64)) != aid || int64(me["member_id"].(float64)) != kidID {
		t.Fatalf("kid /me: %v", me)
	}

	// kid sees the same household data
	var fam struct {
		Household map[string]any `json:"household"`
		Members   []familyMember `json:"members"`
		CanManage bool           `json:"can_manage"`
	}
	kid.do("GET", "/api/family", nil, &fam)
	if len(fam.Members) != 2 || fam.CanManage || fam.Members[0].Role != "owner" {
		t.Fatalf("family as kid: %+v", fam)
	}

	// owner renames household; kid can't
	if code := owner.do("PATCH", "/api/family", map[string]any{"name": "The Testers"}, nil); code != 200 {
		t.Fatalf("rename: %d", code)
	}
	if code := kid.do("PATCH", "/api/family", map[string]any{"name": "Kid Land"}, nil); code != 403 {
		t.Fatalf("kid rename: want 403, got %d", code)
	}
	me = nil
	kid.do("GET", "/api/me", nil, &me)
	if me["household_name"] != "The Testers" {
		t.Fatalf("household_name: %v", me["household_name"])
	}

	// change own password with the temporary one
	if code := kid.do("POST", "/api/me/password", map[string]any{"current_password": "nope", "new_password": "new-pass-456"}, nil); code != 401 {
		t.Fatalf("wrong current password: want 401, got %d", code)
	}
	if code := kid.do("POST", "/api/me/password", map[string]any{"current_password": "temp-pass-123", "new_password": "new-pass-456"}, nil); code != 200 {
		t.Fatalf("change password: %d", code)
	}
	loginAs(t, newClient(t, srv), kidEmail, "new-pass-456")

	// allowance: kid logs 30k this month
	today := todayUTC().Format(dateLayout)
	if code := kid.do("POST", "/api/spending", map[string]any{"tx_date": today, "description": "snack", "amount": 30000}, nil); code != 200 {
		t.Fatalf("kid add spending: %d", code)
	}
	var al allowanceStatus
	kid.do("GET", fmt.Sprintf("/api/family/members/%d/allowance", kidID), nil, &al)
	if al.Spent != 30000 || al.AllowanceMonthly == nil || *al.AllowanceMonthly != 150000 || al.Remaining == nil || *al.Remaining != 120000 {
		t.Fatalf("allowance: %+v", al)
	}
	// kid can't look at other members' allowance
	var ownerMemberID int64
	_ = testApp.DB.QueryRow(context.Background(), "SELECT id FROM members WHERE account_id=$1 AND role='owner'", aid).Scan(&ownerMemberID)
	if code := kid.do("GET", fmt.Sprintf("/api/family/members/%d/allowance", ownerMemberID), nil, nil); code != 403 {
		t.Fatalf("kid other allowance: want 403, got %d", code)
	}

	// leaderboard: kid earned points this month
	var g gamificationResponse
	owner.do("GET", "/api/gamification", nil, &g)
	found := false
	for _, lr := range g.Leaderboard {
		if lr.MemberID == kidID && lr.PointsMonth > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("kid missing from leaderboard with points: %+v", g.Leaderboard)
	}

	// owner can't be removed or demoted
	if code := owner.do("DELETE", fmt.Sprintf("/api/family/members/%d", ownerMemberID), nil, nil); code != 400 {
		t.Fatalf("remove owner: want 400, got %d", code)
	}
	if code := owner.do("PATCH", fmt.Sprintf("/api/family/members/%d", ownerMemberID), map[string]any{"role": "kid"}, nil); code != 400 {
		t.Fatalf("demote owner: want 400, got %d", code)
	}
	// edit kid -> viewer + allowance cleared
	if code := owner.do("PATCH", fmt.Sprintf("/api/family/members/%d", kidID), map[string]any{"role": "viewer", "allowance_monthly": nil}, nil); code != 200 {
		t.Fatalf("edit kid: %d", code)
	}
	// role change applies to the live session immediately
	if code := kid.do("POST", "/api/spending", map[string]any{"tx_date": today, "description": "x", "amount": 1}, nil); code != 403 {
		t.Fatalf("now-viewer add spending: want 403, got %d", code)
	}
	// remove -> the member's session dies, their spending stays (member_id NULL)
	if code := owner.do("DELETE", fmt.Sprintf("/api/family/members/%d", kidID), nil, nil); code != 200 {
		t.Fatalf("remove kid: %d", code)
	}
	if code := kid.do("GET", "/api/spending", nil, nil); code != 401 {
		t.Fatalf("removed member: want 401, got %d", code)
	}
	var n int64
	_ = testApp.DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM spending WHERE account_id=$1 AND description='snack' AND member_id IS NULL", aid).Scan(&n)
	if n != 1 {
		t.Fatalf("removed member's spending should remain with NULL member_id, got %d rows", n)
	}
}

// werkzeugPBKDF2 builds a legacy Flask-era hash.
func werkzeugPBKDF2(password, salt string, iter int) string {
	key := pbkdf2.Key([]byte(password), []byte(salt), iter, 32, sha256.New)
	return fmt.Sprintf("pbkdf2:sha256:%d$%s$%s", iter, salt, hex.EncodeToString(key))
}

func TestOwnerMigrationOfExistingAccount(t *testing.T) {
	app, srv := newTestServer(t)
	ctx := context.Background()
	email := uniqueEmail()
	// A pre-family account: users row only, legacy password hash, no member.
	var aid int64
	if err := app.DB.QueryRow(ctx,
		"INSERT INTO users (email, password_hash, display_name) VALUES ($1,$2,'Old Timer') RETURNING id",
		email, werkzeugPBKDF2("legacy-pass-1", "saltsalt", 1000)).Scan(&aid); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(ctx, "INSERT INTO spending (account_id, tx_date, description, amount) VALUES ($1, CURRENT_DATE, 'old row', 5)", aid); err != nil {
		t.Fatal(err)
	}
	// boot migration (twice: idempotent)
	for i := 0; i < 2; i++ {
		if err := initSchema(ctx, app); err != nil {
			t.Fatal(err)
		}
	}
	var n int64
	var role, display string
	_ = app.DB.QueryRow(ctx, "SELECT COUNT(*), MIN(role), MIN(display_name) FROM members WHERE account_id=$1", aid).Scan(&n, &role, &display)
	if n != 1 || role != "owner" || display != "Old Timer" {
		t.Fatalf("owner migration: n=%d role=%s display=%s", n, role, display)
	}

	// legacy password still logs in, and is upgraded to bcrypt on both rows
	c := newClient(t, srv)
	loginAs(t, c, email, "legacy-pass-1")
	var mh, uh string
	_ = app.DB.QueryRow(ctx, "SELECT password_hash FROM members WHERE account_id=$1", aid).Scan(&mh)
	_ = app.DB.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1", aid).Scan(&uh)
	if !strings.HasPrefix(mh, "$2") || mh != uh {
		t.Fatalf("legacy hash not upgraded: member=%q user=%q", mh, uh)
	}
	var list struct {
		Rows []map[string]any `json:"rows"`
	}
	c.do("GET", "/api/spending", nil, &list)
	if len(list.Rows) != 1 {
		t.Fatalf("old data not visible to migrated owner: %v", list)
	}
	loginAs(t, newClient(t, srv), email, "legacy-pass-1") // bcrypt path now

	// A pre-family session cookie (account_id only) keeps working as owner.
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	sess, _ := store.New(req, "accountamily")
	sess.Values["account_id"] = aid
	if err := sess.Save(req, rec); err != nil {
		t.Fatal(err)
	}
	old := newClient(t, srv)
	u, _ := url.Parse(srv.URL)
	old.c.Jar.SetCookies(u, rec.Result().Cookies())
	var me map[string]any
	old.do("GET", "/api/me", nil, &me)
	if me["authenticated"] != true || me["role"] != "owner" || me["username"] != email {
		t.Fatalf("pre-family session /me: %v", me)
	}
}

func TestRoleMiddleware(t *testing.T) {
	_, srv := newTestServer(t)
	owner := newClient(t, srv)
	owner.signup()
	today := todayUTC().Format(dateLayout)

	var ownerRow map[string]any
	owner.do("POST", "/api/spending", map[string]any{"tx_date": today, "description": "owner buy", "amount": 100}, &ownerRow)
	ownerSID := int64(ownerRow["id"].(float64))

	viewerEmail, _ := addMember(t, owner, "viewer", nil)
	kidEmail, _ := addMember(t, owner, "kid", nil)
	parentEmail, _ := addMember(t, owner, "parent", nil)

	viewer := newClient(t, srv)
	loginAs(t, viewer, viewerEmail, "temp-pass-123")
	kid := newClient(t, srv)
	loginAs(t, kid, kidEmail, "temp-pass-123")
	parent := newClient(t, srv)
	loginAs(t, parent, parentEmail, "temp-pass-123")

	// viewer: read-only
	if code := viewer.do("GET", "/api/spending", nil, nil); code != 200 {
		t.Fatalf("viewer GET: %d", code)
	}
	if code := viewer.do("POST", "/api/spending", map[string]any{"tx_date": today, "description": "v", "amount": 1}, nil); code != 403 {
		t.Fatalf("viewer POST spending: want 403, got %d", code)
	}
	if code := viewer.do("POST", "/api/categories", map[string]any{"name": "Nope", "main_category": "Wants"}, nil); code != 403 {
		t.Fatalf("viewer POST categories: want 403, got %d", code)
	}
	if code := viewer.do("PATCH", "/api/me/settings", map[string]any{"display_name": "Viewer V"}, nil); code != 200 {
		t.Fatalf("viewer own display name: %d", code)
	}
	if code := viewer.do("PATCH", "/api/me/settings", map[string]any{"currency": "USD"}, nil); code != 403 {
		t.Fatalf("viewer household currency: want 403, got %d", code)
	}

	// kid: may log spending, nothing else
	var kidRow map[string]any
	if code := kid.do("POST", "/api/spending", map[string]any{"tx_date": today, "description": "kid buy", "amount": 50}, &kidRow); code != 200 {
		t.Fatalf("kid POST spending: %d", code)
	}
	kidSID := int64(kidRow["id"].(float64))
	if code := kid.do("POST", "/api/budgeting/expense", map[string]any{"year": 2026, "month": 1, "category_id": 1, "amount": 5}, nil); code != 403 {
		t.Fatalf("kid budgeting: want 403, got %d", code)
	}
	if code := kid.do("POST", "/api/family/members", map[string]any{"email": uniqueEmail(), "password": "temp-pass-123", "role": "kid"}, nil); code != 403 {
		t.Fatalf("kid add member: want 403, got %d", code)
	}
	if code := kid.do("PATCH", fmt.Sprintf("/api/spending/%d", ownerSID), map[string]any{"amount": 1}, nil); code != 403 {
		t.Fatalf("kid edit owner's spending: want 403, got %d", code)
	}
	if code := kid.do("DELETE", fmt.Sprintf("/api/spending/%d", ownerSID), nil, nil); code != 403 {
		t.Fatalf("kid delete owner's spending: want 403, got %d", code)
	}
	if code := kid.do("PATCH", fmt.Sprintf("/api/spending/%d", kidSID), map[string]any{"amount": 60}, nil); code != 200 {
		t.Fatalf("kid edit own spending: %d", code)
	}
	if code := kid.do("DELETE", fmt.Sprintf("/api/spending/%d", kidSID), nil, nil); code != 200 {
		t.Fatalf("kid delete own spending: %d", code)
	}

	// parent: full household access, but can't touch the owner
	if code := parent.do("POST", "/api/categories", map[string]any{"name": "Parent Cat", "main_category": "Wants"}, nil); code != 200 {
		t.Fatalf("parent POST categories: %d", code)
	}
	if code := parent.do("PATCH", fmt.Sprintf("/api/spending/%d", ownerSID), map[string]any{"amount": 101}, nil); code != 200 {
		t.Fatalf("parent edit spending: %d", code)
	}
	var fam struct {
		Members []familyMember `json:"members"`
	}
	parent.do("GET", "/api/family", nil, &fam)
	if code := parent.do("PATCH", fmt.Sprintf("/api/family/members/%d", fam.Members[0].ID), map[string]any{"display_name": "x"}, nil); code != 403 {
		t.Fatalf("parent edit owner: want 403, got %d", code)
	}
	// non-admin household can't reach admin endpoints
	if code := parent.do("GET", "/api/admin/accounts", nil, nil); code != 403 {
		t.Fatalf("admin as parent: want 403, got %d", code)
	}
}

func TestBadgeAndChallengeEvaluation(t *testing.T) {
	app, srv := newTestServer(t)
	ctx := context.Background()
	c := newClient(t, srv)
	_, aid := c.signup()
	now := time.Now().UTC()

	var g gamificationResponse
	c.do("GET", "/api/gamification", nil, &g)
	if g.Member == nil || len(g.Badges) != len(badgeCatalog) || len(g.Challenges) != len(challengeDefs) {
		t.Fatalf("shape: %+v", g)
	}
	for _, b := range g.Badges {
		if b.Earned {
			t.Fatalf("fresh member has badge %s", b.Key)
		}
	}
	basePoints := g.Points

	// Household existed for a while; last closed month: Needs+Wants budget
	// 1000, spent 800 (under budget); Savings budget 300, realized 300.
	if _, err := app.DB.Exec(ctx, "UPDATE users SET created_at = now() - interval '60 days' WHERE id=$1", aid); err != nil {
		t.Fatal(err)
	}
	thisMonth, _ := utcMonthBounds(now)
	prev := thisMonth.AddDate(0, -1, 0)
	catID := func(main string) (id int64, name string) {
		if err := app.DB.QueryRow(ctx, "SELECT id, name FROM categories WHERE account_id=$1 AND main_category=$2 ORDER BY id LIMIT 1", aid, main).Scan(&id, &name); err != nil {
			t.Fatal(err)
		}
		return
	}
	needs, needsName := catID("Needs")
	savings, savingsName := catID("Savings")
	for _, b := range []struct {
		cat int64
		amt int
	}{{needs, 1000}, {savings, 300}} {
		if _, err := app.DB.Exec(ctx, "INSERT INTO budget_expense (account_id, year, month, category_id, amount) VALUES ($1,$2,$3,$4,$5)",
			aid, prev.Year(), int(prev.Month()), b.cat, b.amt); err != nil {
			t.Fatal(err)
		}
	}
	day := prev.AddDate(0, 0, 3).Format(dateLayout)
	for _, s := range []struct {
		cat  int64
		name string
		amt  int
	}{{needs, needsName, 800}, {savings, savingsName, 300}} {
		if _, err := app.DB.Exec(ctx, "INSERT INTO spending (account_id, tx_date, description, category_id, category_name, amount) VALUES ($1,$2,'t',$3,$4,$5)",
			aid, day, s.cat, s.name, s.amt); err != nil {
			t.Fatal(err)
		}
	}
	// Last week: spending Mon-Fri, nothing on the weekend -> no-spend weekend.
	lastMon := weekStartUTC(now).AddDate(0, 0, -7)
	for i := 0; i < 5; i++ {
		if _, err := app.DB.Exec(ctx, "INSERT INTO spending (account_id, tx_date, description, amount) VALUES ($1,$2,'wk',1)",
			aid, lastMon.AddDate(0, 0, i).Format(dateLayout)); err != nil {
			t.Fatal(err)
		}
	}
	// and log today via the API (first_log)
	c.do("POST", "/api/spending", map[string]any{"tx_date": now.Format(dateLayout), "description": "today", "amount": 1}, nil)

	g = gamificationResponse{}
	c.do("GET", "/api/gamification", nil, &g)
	earned := map[string]bool{}
	for _, b := range g.Badges {
		if b.Earned {
			earned[b.Key] = true
			if b.EarnedAt == nil {
				t.Fatalf("badge %s without earned_at", b.Key)
			}
		}
	}
	for _, k := range []string{"first_log", "first_budget", "under_budget", "saver"} {
		if !earned[k] {
			t.Errorf("badge %s not earned: %+v", k, g.Badges)
		}
	}
	if earned["streak_7"] {
		t.Errorf("streak_7 should not be earned")
	}
	var nsw *challengeView
	for i := range g.Challenges {
		if g.Challenges[i].Key == "no_spend_weekend" {
			nsw = &g.Challenges[i]
		}
	}
	if nsw == nil || !nsw.LastWeekDone {
		t.Fatalf("no-spend weekend not awarded for last week: %+v", g.Challenges)
	}
	logPts := int64(pointsPerLog + streakBonusPerDay)
	if g.Points != basePoints+logPts+bonusNoSpendWkend {
		t.Fatalf("points: got %d want %d", g.Points, basePoints+logPts+bonusNoSpendWkend)
	}
	// second GET: no double award, badges stay earned once
	g2 := gamificationResponse{}
	c.do("GET", "/api/gamification", nil, &g2)
	if g2.Points != g.Points {
		t.Fatalf("challenge bonus awarded twice: %d -> %d", g.Points, g2.Points)
	}
	var nb int64
	_ = app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM member_badges WHERE member_id=$1 AND badge_key='under_budget'", g.Member.ID).Scan(&nb)
	if nb != 1 {
		t.Fatalf("under_budget stored %d times", nb)
	}
	// streak badge from member streak
	if _, err := app.DB.Exec(ctx, "UPDATE member_points SET best_streak=7 WHERE member_id=$1", g.Member.ID); err != nil {
		t.Fatal(err)
	}
	g3 := gamificationResponse{}
	c.do("GET", "/api/gamification", nil, &g3)
	for _, b := range g3.Badges {
		if b.Key == "streak_7" && !b.Earned {
			t.Fatalf("streak_7 not earned with best_streak 7")
		}
	}
}
