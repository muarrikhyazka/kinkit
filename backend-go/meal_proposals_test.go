package main

import (
	"fmt"
	"testing"
	"time"
)

func TestMealProposalFlow(t *testing.T) {
	_, srv := newTestServer(t)
	owner := newClient(t, srv)
	owner.signup()
	kidEmail, _ := addMember(t, owner, "kid", nil)
	viewerEmail, _ := addMember(t, owner, "viewer", nil)
	kid, viewer := newClient(t, srv), newClient(t, srv)
	loginAs(t, kid, kidEmail, "temp-pass-123")
	loginAs(t, viewer, viewerEmail, "temp-pass-123")

	var p, p2 mealProposal
	if code := kid.do("POST", "/api/meal-plan/proposals", map[string]any{"dish": "Nasi goreng", "note": "with egg"}, &p); code != 201 || p.Status != "pending" || p.MemberName != "kid-member" {
		t.Fatalf("kid propose: %d %+v", code, p)
	}
	if code := viewer.do("POST", "/api/meal-plan/proposals", map[string]any{"dish": "Pizza"}, &p2); code != 201 {
		t.Fatalf("viewer propose: %d", code)
	}
	if code := kid.do("POST", "/api/meal-plan/proposals", map[string]any{"dish": "  "}, nil); code != 400 {
		t.Fatalf("empty dish: %d", code)
	}

	// only owner/parent decide
	for _, c := range []*testClient{kid, viewer} {
		if code := c.do("POST", fmt.Sprintf("/api/meal-plan/proposals/%d/approve", p.ID), map[string]any{}, nil); code != 403 {
			t.Fatalf("non-manager approve: %d", code)
		}
	}
	// a kid can't withdraw someone else's request
	if code := kid.do("DELETE", fmt.Sprintf("/api/meal-plan/proposals/%d", p2.ID), nil, nil); code != 403 {
		t.Fatalf("kid delete other: %d", code)
	}

	// approve on a day of the current week
	monday := mealWeekStart(householdToday(t.Context(), testApp, 0, time.Now().UTC()), true)
	wed := monday.AddDate(0, 0, 2).Format("2006-01-02")
	var ap mealProposal
	if code := owner.do("POST", fmt.Sprintf("/api/meal-plan/proposals/%d/approve", p.ID), map[string]any{"meal_date": wed, "note": "Sure!"}, &ap); code != 200 || ap.Status != "approved" || ap.MealDate == nil || *ap.MealDate != wed {
		t.Fatalf("approve: %d %+v", code, ap)
	}
	var plan struct {
		Plan mealPlan `json:"plan"`
	}
	owner.do("GET", "/api/meal-plan?week=this", nil, &plan)
	var found bool
	for _, d := range plan.Plan.Days {
		if d.Date == wed && len(d.Picks) == 1 && d.Picks[0].Dish == "Nasi goreng" && d.Picks[0].MemberName == "kid-member" {
			found = true
		}
	}
	if !found {
		t.Fatalf("approved dish missing from plan: %+v", plan.Plan.Days)
	}

	if code := owner.do("POST", fmt.Sprintf("/api/meal-plan/proposals/%d/decline", p2.ID), map[string]any{"note": "Not this week"}, &ap); code != 200 || ap.Status != "declined" {
		t.Fatalf("decline: %d %+v", code, ap)
	}
	var list struct {
		Items []mealProposal `json:"items"`
	}
	viewer.do("GET", "/api/meal-plan/proposals?status=declined", nil, &list)
	if len(list.Items) != 1 || list.Items[0].DecisionNote != "Not this week" || list.Items[0].CanDecide || list.Items[0].CanDelete {
		t.Fatalf("viewer list: %+v", list.Items)
	}
	if code := owner.do("GET", "/api/meal-plan/proposals?status=bogus", nil, nil); code != 400 {
		t.Fatalf("bad status: %d", code)
	}

	// a kid can withdraw their own pending request, not a decided one
	var p3 mealProposal
	kid.do("POST", "/api/meal-plan/proposals", map[string]any{"dish": "Soto"}, &p3)
	if code := kid.do("DELETE", fmt.Sprintf("/api/meal-plan/proposals/%d", p3.ID), nil, nil); code != 200 {
		t.Fatalf("withdraw own: %d", code)
	}
	if code := kid.do("DELETE", fmt.Sprintf("/api/meal-plan/proposals/%d", p.ID), nil, nil); code != 403 {
		t.Fatalf("withdraw decided: %d", code)
	}
}
