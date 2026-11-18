package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

// ---------- pure ----------

func day(s string) time.Time {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		panic(err)
	}
	return t
}

func fmtDates(ds []time.Time) []string {
	out := []string{}
	for _, d := range ds {
		out = append(out, d.Format(dateLayout))
	}
	return out
}

func TestChoreScheduleDueDates(t *testing.T) {
	// 2026-11-16 is a Monday
	cases := []struct {
		name     string
		s        choreSchedule
		from, to string
		want     []string
	}{
		{"once", choreSchedule{Kind: choreOnce, Start: day("2026-11-18")}, "2026-11-01", "2026-11-30", []string{"2026-11-18"}},
		{"once outside window", choreSchedule{Kind: choreOnce, Start: day("2026-11-18")}, "2026-11-19", "2026-11-30", []string{}},
		{"daily from start", choreSchedule{Kind: choreDaily, Start: day("2026-11-17")}, "2026-11-15", "2026-11-19", []string{"2026-11-17", "2026-11-18", "2026-11-19"}},
		{"daily until end", choreSchedule{Kind: choreDaily, Start: day("2026-11-01"), End: ptrTime(day("2026-11-17"))}, "2026-11-16", "2026-11-19", []string{"2026-11-16", "2026-11-17"}},
		{"weekly mon+thu", choreSchedule{Kind: choreWeekly, Weekdays: []int{1, 4}, Start: day("2026-11-01")}, "2026-11-16", "2026-11-29", []string{"2026-11-16", "2026-11-19", "2026-11-23", "2026-11-26"}},
		{"weekly default weekday = start's", choreSchedule{Kind: choreWeekly, Start: day("2026-11-18")}, "2026-11-16", "2026-12-02", []string{"2026-11-18", "2026-11-25", "2026-12-02"}},
		{"monthly 31 clamps", choreSchedule{Kind: choreMonthly, DayOfMonth: 31, Start: day("2026-01-01")}, "2026-01-01", "2026-04-30", []string{"2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30"}},
		{"monthly default day = start's", choreSchedule{Kind: choreMonthly, Start: day("2026-11-05")}, "2026-11-01", "2027-01-31", []string{"2026-11-05", "2026-12-05", "2027-01-05"}},
		{"unknown kind", choreSchedule{Kind: "hourly", Start: day("2026-11-01")}, "2026-11-01", "2026-11-03", []string{}},
	}
	for _, c := range cases {
		got := fmtDates(c.s.dueDates(day(c.from), day(c.to)))
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestGenerationWindow(t *testing.T) {
	today := day("2026-11-18")
	if f, to, ok := generationWindow(day("2026-11-18"), nil, today); !ok || !f.Equal(today) || !to.Equal(today) {
		t.Fatalf("fresh chore today: %v %v %v", f, to, ok)
	}
	// long idle: backfill is capped
	if f, _, ok := generationWindow(day("2026-01-01"), nil, today); !ok || f.Format(dateLayout) != "2026-11-11" {
		t.Fatalf("backfill cap: %v %v", f, ok)
	}
	// already generated today
	if _, _, ok := generationWindow(day("2026-01-01"), ptrTime(today), today); ok {
		t.Fatal("nothing to do when generated today")
	}
	// resume from the day after the last run
	if f, _, ok := generationWindow(day("2026-01-01"), ptrTime(day("2026-11-16")), today); !ok || f.Format(dateLayout) != "2026-11-17" {
		t.Fatalf("resume: %v", f)
	}
	// future start
	if _, _, ok := generationWindow(day("2026-11-20"), nil, today); ok {
		t.Fatal("future start has nothing to generate yet")
	}
}

func TestAssignOccurrences(t *testing.T) {
	ids := func(ps []*int64) []int64 {
		out := []int64{}
		for _, p := range ps {
			if p == nil {
				out = append(out, 0)
			} else {
				out = append(out, *p)
			}
		}
		return out
	}
	got, cur := assignOccurrences(nil, []int64{7, 8, 9}, 1, 5)
	if !slices.Equal(ids(got), []int64{8, 9, 7, 8, 9}) || cur != 6 {
		t.Fatalf("rotation: %v cursor %d", ids(got), cur)
	}
	fixed := int64(3)
	got, cur = assignOccurrences(&fixed, nil, 4, 2)
	if !slices.Equal(ids(got), []int64{3, 3}) || cur != 4 {
		t.Fatalf("fixed: %v %d", ids(got), cur)
	}
	got, _ = assignOccurrences(&fixed, []int64{5, 6}, 0, 2)
	if !slices.Equal(ids(got), []int64{5, 6}) {
		t.Fatalf("rotation wins over fixed: %v", ids(got))
	}
	got, _ = assignOccurrences(nil, nil, 0, 2)
	if !slices.Equal(ids(got), []int64{0, 0}) {
		t.Fatalf("unassigned: %v", ids(got))
	}
	// a cursor past the end (members removed from the rotation) still wraps
	got, _ = assignOccurrences(nil, []int64{1, 2}, 7, 1)
	if ids(got)[0] != 2 {
		t.Fatalf("wrap: %v", ids(got))
	}
}

func TestChoreRewardDeltaIdempotent(t *testing.T) {
	awarded := 0
	apply := func(status string) int {
		d := choreRewardDelta(awarded, status, 10)
		awarded += d
		return d
	}
	steps := []struct {
		status string
		want   int
	}{
		{choreSubmitted, 0}, {choreApproved, 10}, {choreApproved, 0}, // approve twice -> once
		{choreRejected, -10}, {choreRejected, 0}, // reject reverses once
		{choreApproved, 10}, {choreTodo, -10}, {choreTodo, 0}, // undo reverses
	}
	for i, s := range steps {
		if got := apply(s.status); got != s.want {
			t.Fatalf("step %d (%s): delta %d want %d", i, s.status, got, s.want)
		}
	}
	if awarded != 0 {
		t.Fatalf("ledger should net to 0, got %d", awarded)
	}
}

func TestChoreHelpers(t *testing.T) {
	if got := choreDoneMessage("Andi", false, 10, choreSubmitted); got != "Nice work Andi! +10 points ⭐ (waiting for approval)" {
		t.Errorf("message: %q", got)
	}
	if got := choreDoneMessage("Andi", true, 0, choreApproved); got != "Marked done for Andi! ✅" {
		t.Errorf("message: %q", got)
	}
	for in, ok := range map[string]bool{"07:30": true, "7:05": true, "23:59": true, "24:00": false, "7:5": false, "abc": false, "": false} {
		if _, _, got := parseDueTime(in); got != ok {
			t.Errorf("parseDueTime(%q)=%v", in, got)
		}
	}
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	if got := dueInstant(day("2026-11-18"), "08:00", jkt); got != "2026-11-18T01:00:00Z" {
		t.Errorf("dueInstant: %s", got)
	}
	if got := localWeekStart(day("2026-11-22")).Format(dateLayout); got != "2026-11-16" {
		t.Errorf("week start: %s", got)
	}
	if v := effectiveAllowance(nil, 0); v != nil {
		t.Errorf("effective of nothing: %v", *v)
	}
	a := 100.0
	if v := effectiveAllowance(&a, 25); v == nil || *v != 125 {
		t.Errorf("effective: %v", v)
	}
}

func TestChoreRolesAndRewardsRules(t *testing.T) {
	cases := []struct {
		role, method, path string
		want               roleDecision
	}{
		{"kid", "POST", "/api/chores/5/complete", roleAllow},
		{"kid", "POST", "/api/chores/5/undo", roleAllow},
		{"kid", "POST", "/api/chores/5/approve", roleDeny},
		{"kid", "POST", "/api/chores/5/reject", roleDeny},
		{"kid", "POST", "/api/chores", roleDeny},
		{"kid", "PATCH", "/api/chores/5", roleDeny},
		{"kid", "POST", "/api/chores/abc/complete", roleDeny},
		{"viewer", "POST", "/api/chores/5/complete", roleDeny},
		{"kid", "GET", "/api/chores/today", roleAllow},
	}
	for _, c := range cases {
		if got, _ := decideRole(c.role, c.method, c.path); got != c.want {
			t.Errorf("%s %s %s: got %v want %v", c.role, c.method, c.path, got, c.want)
		}
	}
	if got := earnedBadges(badgeFacts{ChoresDone: 10}); !slices.Contains(got, "chores_10") || slices.Contains(got, "chores_50") {
		t.Errorf("10 chores: %v", got)
	}
	if got := earnedBadges(badgeFacts{ChoresDone: 50}); !slices.Contains(got, "chores_50") {
		t.Errorf("50 chores: %v", got)
	}
	if _, _, avail, _ := challengeOutcome("all_chores", weekStats{}); avail {
		t.Error("all_chores needs chores")
	}
	if _, _, _, done := challengeOutcome("all_chores", weekStats{ChoresTotal: 3, ChoresDone: 3}); done {
		t.Error("only final when the week ends")
	}
	if _, _, _, done := challengeOutcome("all_chores", weekStats{ChoresTotal: 3, ChoresDone: 2, Ended: true}); done {
		t.Error("2/3 is not all")
	}
	if p, tg, _, done := challengeOutcome("all_chores", weekStats{ChoresTotal: 3, ChoresDone: 3, Ended: true}); !done || p != 3 || tg != 3 {
		t.Error("3/3 at week end should complete")
	}
}

// ---------- integration ----------

type choreFixture struct {
	app                   *App
	owner, k1, k2, viewer *testClient
	aid                   int64
	k1ID, k2ID            int64
	k1Email               string
	today                 time.Time
}

func newChoreFixture(t *testing.T) *choreFixture {
	app, srv := newTestServer(t)
	f := &choreFixture{app: app, owner: newClient(t, srv)}
	_, f.aid = f.owner.signup()
	var e1, e2, ev string
	e1, f.k1ID = addMember(t, f.owner, "kid", 100000)
	e2, f.k2ID = addMember(t, f.owner, "kid", nil)
	ev, _ = addMember(t, f.owner, "viewer", nil)
	f.k1Email = e1
	f.k1, f.k2, f.viewer = newClient(t, srv), newClient(t, srv), newClient(t, srv)
	loginAs(t, f.k1, e1, "temp-pass-123")
	loginAs(t, f.k2, e2, "temp-pass-123")
	loginAs(t, f.viewer, ev, "temp-pass-123")
	f.today = householdToday(context.Background(), app, f.aid, time.Now())
	return f
}

func (f *choreFixture) createChore(t *testing.T, body map[string]any) int64 {
	t.Helper()
	var res map[string]any
	if code := f.owner.do("POST", "/api/chores", body, &res); code != 200 {
		t.Fatalf("create chore: %d %v", code, res)
	}
	return int64(res["id"].(float64))
}

func (f *choreFixture) todayInstance(t *testing.T, choreID int64) (id int64, member *int64, status string) {
	t.Helper()
	err := f.app.DB.QueryRow(context.Background(), "SELECT id, member_id, status FROM chore_instances WHERE chore_id=$1 AND due_date=$2",
		choreID, f.today).Scan(&id, &member, &status)
	if err != nil {
		t.Fatalf("today's instance of chore %d: %v", choreID, err)
	}
	return
}

func (f *choreFixture) memberPoints(t *testing.T, mid int64) (ledger, total int64) {
	t.Helper()
	ctx := context.Background()
	_ = f.app.DB.QueryRow(ctx, "SELECT COALESCE(SUM(points),0)::bigint FROM point_events WHERE member_id=$1 AND reason LIKE 'chore:%'", mid).Scan(&ledger)
	_ = f.app.DB.QueryRow(ctx, "SELECT COALESCE((SELECT points FROM member_points WHERE member_id=$1),0)", mid).Scan(&total)
	return
}

func TestChoresCRUDAndRotation(t *testing.T) {
	f := newChoreFixture(t)
	ctx := context.Background()

	// validation + permissions
	if code := f.owner.do("POST", "/api/chores", map[string]any{"title": ""}, nil); code != 400 {
		t.Fatalf("empty title: %d", code)
	}
	if code := f.owner.do("POST", "/api/chores", map[string]any{"title": "x", "schedule": "hourly"}, nil); code != 400 {
		t.Fatalf("bad schedule: %d", code)
	}
	if code := f.owner.do("POST", "/api/chores", map[string]any{"title": "x", "member_id": 999999999}, nil); code != 400 {
		t.Fatalf("foreign member: %d", code)
	}
	if code := f.k1.do("POST", "/api/chores", map[string]any{"title": "x"}, nil); code != 403 {
		t.Fatalf("kid create: %d", code)
	}
	if code := f.viewer.do("POST", "/api/chores", map[string]any{"title": "x"}, nil); code != 403 {
		t.Fatalf("viewer create: %d", code)
	}

	id := f.createChore(t, map[string]any{"title": "Dishes", "emoji": "🍽️", "points": 5, "schedule": "daily",
		"rotation_member_ids": []int64{f.k1ID, f.k2ID}})
	_, who, status := f.todayInstance(t, id)
	if who == nil || *who != f.k1ID || status != "todo" {
		t.Fatalf("today's dishes should be kid1/todo, got %v %s", who, status)
	}

	var list struct {
		Chores []choreRow `json:"chores"`
	}
	if code := f.k1.do("GET", "/api/chores", nil, &list); code != 200 || len(list.Chores) != 1 {
		t.Fatalf("list: %d %+v", code, list)
	}
	c := list.Chores[0]
	if c.Title != "Dishes" || len(c.RotationMemberIDs) != 2 || c.NextAssigneeID == nil || *c.NextAssigneeID != f.k2ID {
		t.Fatalf("chore row: %+v", c)
	}

	// week view projects tomorrow to kid2
	tomorrow := f.today.AddDate(0, 0, 1)
	var inst struct {
		Instances []choreInstanceView `json:"instances"`
	}
	q := "/api/chores/instances?from=" + f.today.Format(dateLayout) + "&to=" + tomorrow.AddDate(0, 0, 1).Format(dateLayout)
	if code := f.owner.do("GET", q, nil, &inst); code != 200 || len(inst.Instances) != 3 {
		t.Fatalf("instances: %+v", inst)
	}
	if p := inst.Instances[1]; !p.Projected || p.MemberID == nil || *p.MemberID != f.k2ID || p.DueDate != tomorrow.Format(dateLayout) {
		t.Fatalf("projected tomorrow: %+v", p)
	}
	if p := inst.Instances[2]; p.MemberID == nil || *p.MemberID != f.k1ID {
		t.Fatalf("projected day after: %+v", p)
	}
	if code := f.owner.do("GET", "/api/chores/instances?from=2026-01-01&to=2026-12-31", nil, nil); code != 400 {
		t.Fatalf("too long range: %d", code)
	}

	// generator advancing over the next days rotates round-robin, idempotently
	for i, want := range []int64{f.k2ID, f.k1ID, f.k2ID} {
		d := f.today.AddDate(0, 0, i+1)
		for range 2 {
			if err := f.app.ensureChoreInstances(ctx, f.aid, d); err != nil {
				t.Fatal(err)
			}
		}
		var got int64
		if err := f.app.DB.QueryRow(ctx, "SELECT member_id FROM chore_instances WHERE chore_id=$1 AND due_date=$2", id, d).Scan(&got); err != nil || got != want {
			t.Fatalf("day +%d: got %d want %d (%v)", i+1, got, want, err)
		}
	}
	var n int
	_ = f.app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM chore_instances WHERE chore_id=$1", id).Scan(&n)
	if n != 4 {
		t.Fatalf("want 4 instances, got %d", n)
	}
	// older todo ones are superseded -> missed
	var missed int
	_ = f.app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM chore_instances WHERE chore_id=$1 AND status='missed'", id).Scan(&missed)
	if missed != 3 {
		t.Fatalf("want 3 missed, got %d", missed)
	}

	// PATCH: non-structural edit keeps instances; switching to a fixed
	// member rebuilds today's todo instance.
	if code := f.owner.do("PATCH", "/api/chores/"+itoa(id), map[string]any{"title": "Wash dishes"}, nil); code != 200 {
		t.Fatalf("patch title: %d", code)
	}
	if code := f.k1.do("PATCH", "/api/chores/"+itoa(id), map[string]any{"title": "nope"}, nil); code != 403 {
		t.Fatalf("kid patch: %d", code)
	}
	fixed := f.createChore(t, map[string]any{"title": "Feed cat", "member_id": f.k1ID, "schedule": "daily"})
	if code := f.owner.do("PATCH", "/api/chores/"+itoa(fixed), map[string]any{"member_id": f.k2ID}, nil); code != 200 {
		t.Fatalf("patch member: %d", code)
	}
	if _, who, _ := f.todayInstance(t, fixed); who == nil || *who != f.k2ID {
		t.Fatalf("reassigned instance should be kid2: %v", who)
	}

	// DELETE: no history -> gone; with history -> archived
	if code := f.owner.do("DELETE", "/api/chores/"+itoa(fixed), nil, nil); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	var exists bool
	_ = f.app.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM chores WHERE id=$1)", fixed).Scan(&exists)
	if exists {
		t.Fatal("chore without history should be hard deleted")
	}
	var res map[string]any
	f.owner.do("DELETE", "/api/chores/"+itoa(id), nil, &res)
	if res["archived"] != true {
		t.Fatalf("chore with missed history should be archived: %v", res)
	}
	list.Chores = nil
	f.owner.do("GET", "/api/chores", nil, &list)
	if len(list.Chores) != 0 {
		t.Fatalf("archived chore still listed: %+v", list.Chores)
	}
	if code := f.owner.do("DELETE", "/api/chores/"+itoa(id), nil, nil); code != 404 {
		t.Fatalf("double delete: %d", code)
	}
}

func TestChoreCompleteApproveRewards(t *testing.T) {
	f := newChoreFixture(t)
	ctx := context.Background()
	cid := f.createChore(t, map[string]any{"title": "Tidy room", "points": 10, "reward_amount": 5000,
		"member_id": f.k1ID, "schedule": "daily", "requires_approval": true})
	iid, _, _ := f.todayInstance(t, cid)
	path := "/api/chores/" + itoa(iid)

	// someone else's / unknown / viewer
	if code := f.k2.do("POST", path+"/complete", nil, nil); code != 403 {
		t.Fatalf("kid2 completing kid1's chore: %d", code)
	}
	if code := f.k1.do("POST", "/api/chores/999999999/complete", nil, nil); code != 404 {
		t.Fatalf("unknown instance: %d", code)
	}
	if code := f.viewer.do("POST", path+"/complete", nil, nil); code != 403 {
		t.Fatalf("viewer complete: %d", code)
	}
	// another household can't see it
	_, srv := newTestServer(t)
	other := newClient(t, srv)
	other.signup()
	if code := other.do("POST", path+"/complete", nil, nil); code != 404 {
		t.Fatalf("other household: %d", code)
	}

	var res map[string]any
	if code := f.k1.do("POST", path+"/complete", map[string]any{"note": "all clean"}, &res); code != 200 || res["status"] != "submitted" {
		t.Fatalf("kid complete: %d %v", code, res)
	}
	if !strings.Contains(res["message"].(string), "waiting for approval") {
		t.Fatalf("message: %v", res["message"])
	}
	if code := f.k1.do("POST", path+"/complete", nil, nil); code != 409 {
		t.Fatalf("double complete: %d", code)
	}
	if l, _ := f.memberPoints(t, f.k1ID); l != 0 {
		t.Fatalf("no points before approval, got %d", l)
	}
	if code := f.k1.do("POST", path+"/approve", nil, nil); code != 403 {
		t.Fatalf("kid approve: %d", code)
	}
	// kid can take back a submission
	if code := f.k1.do("POST", path+"/undo", nil, nil); code != 200 {
		t.Fatalf("kid undo submitted: %d", code)
	}
	f.k1.do("POST", path+"/complete", nil, nil)

	// approve awards once
	if code := f.owner.do("POST", path+"/approve", nil, &res); code != 200 || res["status"] != "approved" || res["points_delta"].(float64) != 10 {
		t.Fatalf("approve: %d %v", code, res)
	}
	if code := f.owner.do("POST", path+"/approve", nil, nil); code != 409 {
		t.Fatalf("double approve: %d", code)
	}
	if l, tot := f.memberPoints(t, f.k1ID); l != 10 || tot != 10 {
		t.Fatalf("after approve: ledger %d total %d", l, tot)
	}
	var g gamificationResponse
	f.owner.do("GET", "/api/gamification", nil, &g)
	for _, r := range g.Leaderboard {
		if r.MemberID == f.k1ID && r.PointsMonth != 10 {
			t.Fatalf("leaderboard: %+v", r)
		}
	}

	// earnings in allowance + family page
	var al allowanceStatus
	if code := f.k1.do("GET", "/api/family/members/"+itoa(f.k1ID)+"/allowance", nil, &al); code != 200 {
		t.Fatalf("allowance: %d", code)
	}
	if al.ChoreEarned != 5000 || al.EffectiveAllowance == nil || *al.EffectiveAllowance != 105000 {
		t.Fatalf("allowance with earnings: %+v", al)
	}
	var fam struct {
		Members []familyMember `json:"members"`
	}
	f.owner.do("GET", "/api/family", nil, &fam)
	for _, m := range fam.Members {
		if m.ID == f.k1ID && (m.ChoresDoneWeek != 1 || m.ChoreEarnedMonth != 5000) {
			t.Fatalf("family member chores: %+v", m)
		}
	}

	// kid can't undo an approved chore; reject reverses points + earnings
	if code := f.k1.do("POST", path+"/undo", nil, nil); code != 403 {
		t.Fatalf("kid undo approved: %d", code)
	}
	if code := f.owner.do("POST", path+"/reject", map[string]any{"note": "bed not made"}, &res); code != 200 || res["status"] != "rejected" {
		t.Fatalf("reject: %d %v", code, res)
	}
	if l, tot := f.memberPoints(t, f.k1ID); l != 0 || tot != 0 {
		t.Fatalf("after reject: ledger %d total %d", l, tot)
	}
	var earned float64
	_ = f.app.DB.QueryRow(ctx, "SELECT COALESCE(SUM(amount),0)::float8 FROM chore_earnings WHERE member_id=$1", f.k1ID).Scan(&earned)
	if earned != 0 {
		t.Fatalf("earnings after reject: %v", earned)
	}
	// redo -> approve -> owner undo also reverses
	f.k1.do("POST", path+"/complete", nil, nil)
	f.owner.do("POST", path+"/approve", nil, nil)
	if l, _ := f.memberPoints(t, f.k1ID); l != 10 {
		t.Fatalf("re-approve: %d", l)
	}
	if code := f.owner.do("POST", path+"/undo", nil, &res); code != 200 || res["status"] != "todo" {
		t.Fatalf("owner undo: %d %v", code, res)
	}
	if l, tot := f.memberPoints(t, f.k1ID); l != 0 || tot != 0 {
		t.Fatalf("after undo: ledger %d total %d", l, tot)
	}
	if code := f.owner.do("POST", path+"/undo", nil, nil); code != 409 {
		t.Fatalf("undo todo: %d", code)
	}

	// no-approval chore: points right away; kid undo takes them back
	quick := f.createChore(t, map[string]any{"title": "Water plants", "points": 3, "member_id": f.k2ID})
	qid, _, _ := f.todayInstance(t, quick)
	if code := f.k2.do("POST", "/api/chores/"+itoa(qid)+"/complete", nil, &res); code != 200 || res["status"] != "approved" {
		t.Fatalf("quick complete: %d %v", code, res)
	}
	if l, _ := f.memberPoints(t, f.k2ID); l != 3 {
		t.Fatalf("quick points: %d", l)
	}
	if code := f.k2.do("POST", "/api/chores/"+itoa(qid)+"/undo", nil, nil); code != 200 {
		t.Fatalf("kid undo own quick: %d", code)
	}
	if l, _ := f.memberPoints(t, f.k2ID); l != 0 {
		t.Fatalf("quick undo points: %d", l)
	}

	// unassigned chore: whoever does it gets the credit
	anyone := f.createChore(t, map[string]any{"title": "Take out trash", "points": 4})
	aiid, _, _ := f.todayInstance(t, anyone)
	f.k2.do("POST", "/api/chores/"+itoa(aiid)+"/complete", nil, nil)
	if l, _ := f.memberPoints(t, f.k2ID); l != 4 {
		t.Fatalf("anyone chore points: %d", l)
	}
}

func TestChorePhotoRequired(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	f := newChoreFixture(t)
	cid := f.createChore(t, map[string]any{"title": "Make bed", "member_id": f.k1ID, "requires_photo": true, "points": 2})
	iid, _, _ := f.todayInstance(t, cid)
	path := "/api/chores/" + itoa(iid)
	if code := f.k1.do("POST", path+"/complete", nil, nil); code != 400 {
		t.Fatalf("photo required: %d", code)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("note", "done!")
	fw, _ := mw.CreateFormFile("photo", "bed.png")
	_, _ = fw.Write(tinyPNG)
	_ = mw.Close()
	req, _ := http.NewRequest("POST", f.k1.base+path+"/complete", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := f.k1.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("multipart complete: %d %s", resp.StatusCode, raw)
	}
	var res struct {
		Instance choreInstanceView `json:"instance"`
	}
	_ = json.Unmarshal(raw, &res)
	if !res.Instance.HasPhoto || res.Instance.Note != "done!" {
		t.Fatalf("instance after photo: %+v", res.Instance)
	}
	resp, err = f.owner.c.Get(f.owner.base + path + "/photo")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("photo get: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	// undo removes the photo
	f.k1.do("POST", path+"/undo", nil, nil)
	resp, _ = f.owner.c.Get(f.owner.base + path + "/photo")
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("photo after undo: %d", resp.StatusCode)
	}
}

type multiHeaderRT map[string]string

func (h multiHeaderRT) RoundTrip(r *http.Request) (*http.Response, error) {
	for k, v := range h {
		r.Header.Set(k, v)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func TestChoreBotEndpoints(t *testing.T) {
	f := newChoreFixture(t)
	ctx := context.Background()
	if code := f.owner.do("POST", "/api/integrations", map[string]any{"platform": "telegram", "platform_token": "1:abc"}, nil); code != 200 {
		t.Fatalf("integration: %d", code)
	}
	var token string
	if err := f.app.DB.QueryRow(ctx, "SELECT bot_token FROM integrations WHERE account_id=$1", f.aid).Scan(&token); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.DB.Exec(ctx, "UPDATE members SET telegram_user_id='tg-kid1', display_name='Andi' WHERE id=$1", f.k1ID); err != nil {
		t.Fatal(err)
	}
	mine := f.createChore(t, map[string]any{"title": "Homework", "emoji": "📚", "points": 10, "member_id": f.k1ID, "requires_approval": true})
	theirs := f.createChore(t, map[string]any{"title": "Vacuum", "points": 5, "member_id": f.k2ID})
	mid, _, _ := f.todayInstance(t, mine)
	tid, _, _ := f.todayInstance(t, theirs)

	bot := func(user string) *testClient {
		h := multiHeaderRT{"X-Bot-Token": token}
		if user != "" {
			h["X-Bot-User"] = user
		}
		return &testClient{t: t, base: f.owner.base, c: &http.Client{Transport: h}}
	}
	type botItems struct {
		Items []struct {
			ID           int64  `json:"id"`
			Title        string `json:"title"`
			AssigneeName string `json:"assignee_name"`
			Done         bool   `json:"done"`
			Points       int    `json:"points"`
		} `json:"items"`
	}
	var all, kid botItems
	if code := bot("").do("GET", "/api/bot/chores/today", nil, &all); code != 200 || len(all.Items) != 2 {
		t.Fatalf("bot today (no actor): %d %+v", code, all)
	}
	bot("telegram:tg-kid1").do("GET", "/api/bot/chores/today", nil, &kid)
	if len(kid.Items) != 1 || kid.Items[0].ID != mid || kid.Items[0].Title != "📚 Homework" || kid.Items[0].AssigneeName != "Andi" || kid.Items[0].Points != 10 {
		t.Fatalf("bot today (kid): %+v", kid)
	}
	noTok := &testClient{t: t, base: f.owner.base, c: &http.Client{}}
	if code := noTok.do("GET", "/api/bot/chores/today", nil, nil); code != 403 {
		t.Fatalf("no bot token: %d", code)
	}

	var res map[string]any
	if code := bot("").do("POST", "/api/bot/chores/"+itoa(mid)+"/done", nil, &res); code != 403 || res["ok"] != false || res["message"] == "" {
		t.Fatalf("done without actor: %d %v", code, res)
	}
	if code := bot("telegram:tg-kid1").do("POST", "/api/bot/chores/"+itoa(tid)+"/done", nil, &res); code != 403 || res["ok"] != false {
		t.Fatalf("kid done on other's: %d %v", code, res)
	}
	if code := bot("telegram:tg-kid1").do("POST", "/api/bot/chores/"+itoa(mid)+"/done", nil, &res); code != 200 || res["ok"] != true {
		t.Fatalf("kid done own: %d %v", code, res)
	}
	if msg := res["message"].(string); msg != "Nice work Andi! +10 points ⭐ (waiting for approval)" {
		t.Fatalf("message: %q", msg)
	}
	if code := bot("telegram:tg-kid1").do("POST", "/api/bot/chores/"+itoa(mid)+"/done", nil, &res); code != 409 || res["ok"] != false {
		t.Fatalf("done twice: %d %v", code, res)
	}
	kid = botItems{}
	bot("telegram:tg-kid1").do("GET", "/api/bot/chores/today", nil, &kid)
	if !kid.Items[0].Done {
		t.Fatalf("should be done now: %+v", kid)
	}
}

func TestChoresTodayProvider(t *testing.T) {
	f := newChoreFixture(t)
	ctx := context.Background()
	yesterday := f.today.AddDate(0, 0, -1)
	f.createChore(t, map[string]any{"title": "Homework", "points": 10, "member_id": f.k1ID, "requires_approval": true, "due_time": "17:00"})
	f.createChore(t, map[string]any{"title": "Vacuum", "points": 5, "member_id": f.k2ID})
	late := f.createChore(t, map[string]any{"title": "Return library book", "member_id": f.k1ID, "start_date": yesterday.Format(dateLayout)})
	var lateID int64
	if err := f.app.DB.QueryRow(ctx, "SELECT id FROM chore_instances WHERE chore_id=$1 AND due_date=$2", late, yesterday).Scan(&lateID); err != nil {
		t.Fatalf("overdue instance: %v", err)
	}
	var ownerID int64
	_ = f.app.DB.QueryRow(ctx, "SELECT id FROM members WHERE account_id=$1 AND role='owner'", f.aid).Scan(&ownerID)
	kid := memberInfo{ID: f.k1ID, AccountID: f.aid, Role: roleKid}
	owner := memberInfo{ID: ownerID, AccountID: f.aid, Role: roleOwner}

	sec, err := choresTodayProvider(ctx, f.app, f.aid, kid, f.today)
	if err != nil || sec == nil {
		t.Fatalf("kid section: %v %v", sec, err)
	}
	if sec.Key != "chores" || len(sec.Items) != 2 {
		t.Fatalf("kid sees own 2 (incl. overdue): %+v", sec.Items)
	}
	var hw TodayItem
	for _, it := range sec.Items {
		if it.Kind != "chore" || it.Link != "/home/chores" || !slices.Equal(it.Actions, []string{"done"}) || it.MemberID == nil || *it.MemberID != f.k1ID {
			t.Fatalf("kid item: %+v", it)
		}
		if it.ID == lateID && !it.Overdue {
			t.Fatalf("library book should be overdue: %+v", it)
		}
		if strings.Contains(it.Title, "Homework") {
			hw = it
		}
	}
	if hw.Due != f.today.Format(dateLayout)+"T17:00:00Z" { // household tz is UTC
		t.Fatalf("due instant: %q", hw.Due)
	}

	// kid submits homework -> owner sees it with "approve" + summary
	f.k1.do("POST", "/api/chores/"+itoa(hw.ID)+"/complete", nil, nil)
	sec, _ = choresTodayProvider(ctx, f.app, f.aid, owner, f.today)
	if sec == nil || len(sec.Items) != 3 || sec.Summary != "1 waiting for approval" {
		t.Fatalf("owner section: %+v", sec)
	}
	for _, it := range sec.Items {
		if it.ID == hw.ID && (!it.Done || !slices.Equal(it.Actions, []string{"approve"})) {
			t.Fatalf("submitted item for owner: %+v", it)
		}
	}
	// viewer: read-only
	sec, _ = choresTodayProvider(ctx, f.app, f.aid, memberInfo{ID: 0, AccountID: f.aid, Role: roleViewer}, f.today)
	for _, it := range sec.Items {
		if len(it.Actions) != 0 {
			t.Fatalf("viewer item has actions: %+v", it)
		}
	}
	// HTTP view mirrors it
	var td struct {
		Items   []choreInstanceView `json:"items"`
		Waiting []choreInstanceView `json:"waiting"`
	}
	f.owner.do("GET", "/api/chores/today", nil, &td)
	if len(td.Items) != 3 || len(td.Waiting) != 1 {
		t.Fatalf("GET today (owner): %+v", td)
	}
	td.Items, td.Waiting = nil, nil
	f.k2.do("GET", "/api/chores/today", nil, &td)
	if len(td.Items) != 1 || len(td.Waiting) != 0 {
		t.Fatalf("GET today (kid2): %+v", td)
	}
}
