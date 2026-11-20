package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// pure

func TestParseItemsText(t *testing.T) {
	cases := []struct {
		in   string
		want []parsedItem
	}{
		{"eggs, 2 milk, bread x3", []parsedItem{{"eggs", ""}, {"milk", "2"}, {"bread", "3"}}},
		{"1kg rice\n500 g flour\n- apples\n* 2x yogurt", []parsedItem{{"rice", "1kg"}, {"flour", "500g"}, {"apples", ""}, {"yogurt", "2"}}},
		{"milk x2; rice 5kg; eggs (12)", []parsedItem{{"milk", "2"}, {"rice", "5kg"}, {"eggs", "12"}}},
		{"1,5kg sugar, tea", []parsedItem{{"sugar", "1,5kg"}, {"tea", ""}}},
		{"  , ,\n\n", []parsedItem{}},
		{"1. bananas\n2. Kleenex 2", []parsedItem{{"bananas", ""}, {"Kleenex", "2"}}},
		{"3 x Coca   Cola", []parsedItem{{"Coca Cola", "3"}}},
		{"2kg", []parsedItem{{"2kg", ""}}},
		{"[ ] soap ×4", []parsedItem{{"soap", "4"}}},
	}
	for _, c := range cases {
		got := parseItemsText(c.in)
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("parseItemsText(%q) = %v, want %v", c.in, got, c.want)
		}
	}
	if n := len(parseItemsText(strings.Repeat("a,", 500))); n != maxBulkItems {
		t.Errorf("cap: got %d items", n)
	}
}

func TestStapleDue(t *testing.T) {
	jkt, _ := time.LoadLocation("Asia/Jakarta") // UTC+7
	today := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	if !stapleDue(nil, 14, today, jkt) {
		t.Error("never added -> due")
	}
	// added 2026-11-04 local (03:00 local = 11-03 20:00 UTC): due 11-18
	last := time.Date(2026, 11, 3, 20, 0, 0, 0, time.UTC)
	if !stapleDue(&last, 14, today, jkt) {
		t.Error("14 days after local add day should be due")
	}
	// every 15 days: local add day 11-04 -> due 11-19 (not yet); the same
	// instant in UTC is 11-03 -> due 11-18 (today)
	if stapleDue(&last, 15, today, jkt) {
		t.Error("jakarta: not due until 11-19")
	}
	if !stapleDue(&last, 15, today, time.UTC) {
		t.Error("utc: due 11-18")
	}
}

func TestStapleNextDue(t *testing.T) {
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	today := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	last := time.Date(2026, 11, 10, 18, 30, 0, 0, time.UTC) // 11-11 01:30 Jakarta
	if got := stapleNextDue(&last, 7, today, jkt).Format(isoDateLayout); got != "2026-11-18" {
		t.Errorf("jakarta next due = %s", got)
	}
	if got := stapleNextDue(&last, 7, today, time.UTC).Format(isoDateLayout); got != "2026-11-17" {
		t.Errorf("utc next due = %s", got)
	}
	if stapleDue(&last, 8, today, jkt) {
		t.Error("8 days -> due 11-19, not today")
	}
	if !stapleDue(&last, 0, today, jkt) {
		t.Error("every_days<1 treated as 1")
	}
}

func TestParseAmountText(t *testing.T) {
	cases := map[string]int64{"250000": 250000, "250.000": 250000, "250,000": 250000, "250k": 250000, "2rb": 2000,
		"1.5jt": 1500000, "1,5jt": 1500000, "Rp 75.500": 75500, "1.000.000": 1000000}
	for in, want := range cases {
		if got, ok := listParseAmount(in); !ok || got != want {
			t.Errorf("listParseAmount(%q) = %d,%v want %d", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "abc", "12x"} {
		if _, ok := listParseAmount(in); ok {
			t.Errorf("listParseAmount(%q) should fail", in)
		}
	}
	if listFormatIDR(1250000) != "Rp1.250.000" || listFormatIDR(500) != "Rp500" {
		t.Error("listFormatIDR")
	}
}

func TestListsKidRule(t *testing.T) {
	cases := []struct {
		method, path string
		want         roleDecision
	}{
		{"POST", "/api/lists/5/items", roleAllow},
		{"PATCH", "/api/lists/5/items/9", roleAllow},
		{"DELETE", "/api/lists/5/items/9", roleDeny},
		{"DELETE", "/api/lists/5", roleDeny},
		{"POST", "/api/lists/5/finish", roleDeny},
		{"POST", "/api/lists", roleDeny},
		{"POST", "/api/staples", roleDeny},
		{"POST", "/api/lists/x/items", roleDeny},
	}
	for _, c := range cases {
		if got, _ := decideRole("kid", c.method, c.path); got != c.want {
			t.Errorf("kid %s %s = %v want %v", c.method, c.path, got, c.want)
		}
	}
	if got, _ := decideRole("viewer", "POST", "/api/lists/5/items"); got != roleDeny {
		t.Error("viewer must not add items")
	}
}

// integration

type listsResp struct {
	Lists []shoppingList `json:"lists"`
}
type listResp struct {
	List  shoppingList   `json:"list"`
	Items []shoppingItem `json:"items"`
}

func defaultListID(t *testing.T, c *testClient) int64 {
	t.Helper()
	var res listsResp
	if code := c.do("GET", "/api/lists", nil, &res); code != 200 || len(res.Lists) == 0 {
		t.Fatalf("lists: %d %+v", code, res)
	}
	return res.Lists[0].ID
}

func TestListsCRUDAndItems(t *testing.T) {
	_, srv := newTestServer(t)
	c := newClient(t, srv)
	c.signup()

	var res listsResp
	c.do("GET", "/api/lists", nil, &res)
	if len(res.Lists) != 1 || res.Lists[0].Name != "Groceries" {
		t.Fatalf("default list: %+v", res.Lists)
	}
	c.do("GET", "/api/lists", nil, &res) // lazy default is created once
	if len(res.Lists) != 1 {
		t.Fatalf("default list duplicated: %+v", res.Lists)
	}
	gid := res.Lists[0].ID

	var hw shoppingList
	if code := c.do("POST", "/api/lists", map[string]any{"name": "Hardware", "store": "Ace"}, &hw); code != 200 || hw.Store != "Ace" {
		t.Fatalf("create list: %d %+v", code, hw)
	}
	if code := c.do("PATCH", fmt.Sprintf("/api/lists/%d", hw.ID), map[string]any{"name": "Tools"}, &hw); code != 200 || hw.Name != "Tools" {
		t.Fatalf("rename: %d %+v", code, hw)
	}
	if code := c.do("POST", "/api/lists", map[string]any{"name": " "}, nil); code != 400 {
		t.Fatalf("empty name: %d", code)
	}

	// bulk add + duplicate merge
	var added struct {
		Added []string       `json:"added"`
		Items []shoppingItem `json:"items"`
	}
	if code := c.do("POST", fmt.Sprintf("/api/lists/%d/items", gid), map[string]any{"text": "eggs, 2 milk, bread x3"}, &added); code != 200 {
		t.Fatalf("bulk add: %d", code)
	}
	if strings.Join(added.Added, "|") != "eggs|milk|bread" || added.Items[1].Qty != "2" {
		t.Fatalf("bulk add result: %+v", added)
	}
	c.do("POST", fmt.Sprintf("/api/lists/%d/items", gid), map[string]any{"text": "Milk x4"}, &added)
	var lr listResp
	c.do("GET", fmt.Sprintf("/api/lists/%d", gid), nil, &lr)
	if len(lr.Items) != 3 {
		t.Fatalf("duplicate should merge, items: %+v", lr.Items)
	}
	milk := lr.Items[1]
	if milk.Qty != "4" {
		t.Fatalf("merged qty: %+v", milk)
	}
	// single add with category
	var cat int64
	_ = testApp.DB.QueryRow(context.Background(), "SELECT id FROM categories WHERE account_id=(SELECT account_id FROM shopping_lists WHERE id=$1) AND name='Food'", gid).Scan(&cat)
	c.do("POST", fmt.Sprintf("/api/lists/%d/items", gid), map[string]any{"name": "Cheese, aged", "qty": "200g", "category_id": cat, "est_price": 45000}, &added)
	if len(added.Items) != 1 || added.Items[0].Name != "Cheese, aged" || added.Items[0].CategoryName == nil || *added.Items[0].CategoryName != "Food" {
		t.Fatalf("single add: %+v", added.Items)
	}

	// check / uncheck / patch fields
	var it shoppingItem
	path := fmt.Sprintf("/api/lists/%d/items/%d", gid, milk.ID)
	if code := c.do("PATCH", path, map[string]any{"checked": true}, &it); code != 200 || !it.Checked || it.CheckedBy == nil || it.CheckedAt == nil {
		t.Fatalf("check: %d %+v", code, it)
	}
	c.do("PATCH", path, map[string]any{"checked": false, "note": "lactose free", "category_id": cat}, &it)
	if it.Checked || it.CheckedBy != nil || it.Note != "lactose free" {
		t.Fatalf("uncheck/patch: %+v", it)
	}
	if code := c.do("PATCH", fmt.Sprintf("/api/lists/%d/items/%d", hw.ID, milk.ID), map[string]any{"checked": true}, nil); code != 404 {
		t.Fatalf("item via wrong list: %d", code)
	}

	// suggestions remember category
	var sg struct {
		Suggestions []struct {
			Name       string `json:"name"`
			Times      int    `json:"times"`
			CategoryID *int64 `json:"category_id"`
		} `json:"suggestions"`
	}
	c.do("GET", "/api/lists/suggestions?q=mi", nil, &sg)
	if len(sg.Suggestions) != 1 || sg.Suggestions[0].Times != 2 || sg.Suggestions[0].CategoryID == nil {
		t.Fatalf("suggestions: %+v", sg)
	}
	c.do("GET", fmt.Sprintf("/api/lists/suggestions?q=mi&list=%d", gid), nil, &sg)
	if len(sg.Suggestions) != 0 {
		t.Fatalf("suggestions should hide items already on the list: %+v", sg)
	}

	// reorder
	c.do("GET", fmt.Sprintf("/api/lists/%d", gid), nil, &lr)
	ids := []int64{}
	for i := len(lr.Items) - 1; i >= 0; i-- {
		ids = append(ids, lr.Items[i].ID)
	}
	c.do("POST", fmt.Sprintf("/api/lists/%d/reorder", gid), map[string]any{"ids": ids}, nil)
	c.do("GET", fmt.Sprintf("/api/lists/%d", gid), nil, &lr)
	if lr.Items[0].ID != ids[0] {
		t.Fatalf("reorder: %+v", lr.Items)
	}

	// clear checked + delete item
	c.do("PATCH", fmt.Sprintf("/api/lists/%d/items/%d", gid, ids[0]), map[string]any{"checked": true}, nil)
	var cl struct {
		Removed int `json:"removed"`
	}
	c.do("POST", fmt.Sprintf("/api/lists/%d/clear-checked", gid), nil, &cl)
	if cl.Removed != 1 {
		t.Fatalf("clear-checked: %+v", cl)
	}
	if code := c.do("DELETE", fmt.Sprintf("/api/lists/%d/items/%d", gid, ids[1]), nil, nil); code != 200 {
		t.Fatalf("delete item: %d", code)
	}
	c.do("GET", fmt.Sprintf("/api/lists/%d", gid), nil, &lr)
	if len(lr.Items) != 2 || lr.List.OpenCount != 2 {
		t.Fatalf("after deletes: %+v", lr)
	}

	// delete list; other households can't see it
	other := newClient(t, srv)
	other.signup()
	if code := other.do("GET", fmt.Sprintf("/api/lists/%d", gid), nil, nil); code != 404 {
		t.Fatalf("cross-household read: %d", code)
	}
	if code := other.do("POST", fmt.Sprintf("/api/lists/%d/items", gid), map[string]any{"text": "x"}, nil); code != 404 {
		t.Fatalf("cross-household add: %d", code)
	}
	if code := c.do("DELETE", fmt.Sprintf("/api/lists/%d", hw.ID), nil, nil); code != 200 {
		t.Fatalf("delete list: %d", code)
	}
	c.do("GET", "/api/lists", nil, &res)
	if len(res.Lists) != 1 {
		t.Fatalf("after delete: %+v", res.Lists)
	}
}

func TestListsFinishShopping(t *testing.T) {
	app, srv := newTestServer(t)
	c := newClient(t, srv)
	_, aid := c.signup()
	ctx := context.Background()
	var ownerID int64
	_ = app.DB.QueryRow(ctx, "SELECT id FROM members WHERE account_id=$1 AND role='owner'", aid).Scan(&ownerID)
	_, _ = app.DB.Exec(ctx, "UPDATE users SET timezone='Pacific/Kiritimati' WHERE id=$1", aid) // UTC+14

	gid := defaultListID(t, c)
	var added struct {
		Items []shoppingItem `json:"items"`
	}
	c.do("POST", fmt.Sprintf("/api/lists/%d/items", gid), map[string]any{"text": "eggs, milk, bread"}, &added)
	for _, it := range added.Items[:2] {
		c.do("PATCH", fmt.Sprintf("/api/lists/%d/items/%d", gid, it.ID), map[string]any{"checked": true}, nil)
	}

	if code := c.do("POST", fmt.Sprintf("/api/lists/%d/finish", gid), map[string]any{"total_amount": 0}, nil); code != 400 {
		t.Fatalf("zero total: %d", code)
	}
	if code := c.do("POST", fmt.Sprintf("/api/lists/%d/finish", gid), map[string]any{"total_amount": 5, "category": "Nope"}, nil); code != 400 {
		t.Fatalf("bad category: %d", code)
	}
	body := map[string]any{"total_amount": 87500, "client_id": "abc-123"}
	var fin struct {
		SpendingID int64  `json:"spending_id"`
		Duplicate  bool   `json:"duplicate"`
		Removed    int    `json:"removed"`
		Category   string `json:"category"`
	}
	if code := c.do("POST", fmt.Sprintf("/api/lists/%d/finish", gid), body, &fin); code != 200 || fin.Duplicate || fin.Removed != 2 || fin.Category != "Food" {
		t.Fatalf("finish: %d %+v", code, fin)
	}
	var fin2 struct {
		SpendingID int64 `json:"spending_id"`
		Duplicate  bool  `json:"duplicate"`
	}
	c.do("POST", fmt.Sprintf("/api/lists/%d/finish", gid), body, &fin2)
	if !fin2.Duplicate || fin2.SpendingID != fin.SpendingID {
		t.Fatalf("retry should be idempotent: %+v vs %+v", fin2, fin)
	}

	var n int
	var amount int64
	var source, clientID, desc string
	var member *int64
	var txDate time.Time
	_ = app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM spending WHERE account_id=$1", aid).Scan(&n)
	_ = app.DB.QueryRow(ctx, "SELECT amount, source, client_id, member_id, description, tx_date FROM spending WHERE id=$1", fin.SpendingID).
		Scan(&amount, &source, &clientID, &member, &desc, &txDate)
	if n != 1 || amount != 87500 || source != "list" || clientID != "trip:abc-123" || member == nil || *member != ownerID || desc != "Groceries (2 items)" {
		t.Fatalf("spending: n=%d amount=%d source=%s client=%s member=%v desc=%q", n, amount, source, clientID, member, desc)
	}
	if want := householdToday(ctx, app, aid, time.Now()); !txDate.Equal(want) {
		t.Fatalf("tx_date %v want household day %v", txDate, want)
	}
	var trips int
	var tripItems string
	_ = app.DB.QueryRow(ctx, "SELECT COUNT(*), MAX(items::text) FROM shopping_trips WHERE account_id=$1", aid).Scan(&trips, &tripItems)
	if trips != 1 || !strings.Contains(tripItems, "eggs") {
		t.Fatalf("trips: %d %s", trips, tripItems)
	}
	var pts int64
	_ = app.DB.QueryRow(ctx, "SELECT points FROM member_points WHERE member_id=$1", ownerID).Scan(&pts)
	if pts <= 0 {
		t.Fatalf("points not awarded: %d", pts)
	}
	var lr listResp
	c.do("GET", fmt.Sprintf("/api/lists/%d", gid), nil, &lr)
	if len(lr.Items) != 1 || lr.Items[0].Name != "bread" {
		t.Fatalf("remaining items: %+v", lr.Items)
	}
}

func TestListsKidAndViewerPermissions(t *testing.T) {
	_, srv := newTestServer(t)
	owner := newClient(t, srv)
	owner.signup()
	gid := defaultListID(t, owner)
	var added struct {
		Items []shoppingItem `json:"items"`
	}
	owner.do("POST", fmt.Sprintf("/api/lists/%d/items", gid), map[string]any{"text": "apples"}, &added)
	itemPath := fmt.Sprintf("/api/lists/%d/items/%d", gid, added.Items[0].ID)

	kidEmail, kidID := addMember(t, owner, "kid", nil)
	viewerEmail, _ := addMember(t, owner, "viewer", nil)
	kid := newClient(t, srv)
	loginAs(t, kid, kidEmail, "temp-pass-123")

	if code := kid.do("POST", fmt.Sprintf("/api/lists/%d/items", gid), map[string]any{"text": "candy"}, &added); code != 200 || *added.Items[0].AddedBy != kidID {
		t.Fatalf("kid add: %d %+v", code, added)
	}
	var it shoppingItem
	if code := kid.do("PATCH", itemPath, map[string]any{"checked": true}, &it); code != 200 || it.CheckedBy == nil || *it.CheckedBy != kidID {
		t.Fatalf("kid check: %d %+v", code, it)
	}
	if code := kid.do("PATCH", itemPath, map[string]any{"qty": "99"}, nil); code != 403 {
		t.Fatalf("kid qty edit: %d", code)
	}
	for _, rq := range []struct{ m, p string }{
		{"POST", fmt.Sprintf("/api/lists/%d/finish", gid)},
		{"DELETE", fmt.Sprintf("/api/lists/%d", gid)},
		{"DELETE", itemPath},
		{"POST", "/api/lists"},
		{"POST", "/api/staples"},
	} {
		if code := kid.do(rq.m, rq.p, map[string]any{"total_amount": 100, "name": "x"}, nil); code != 403 {
			t.Fatalf("kid %s %s: %d", rq.m, rq.p, code)
		}
	}

	viewer := newClient(t, srv)
	loginAs(t, viewer, viewerEmail, "temp-pass-123")
	if code := viewer.do("GET", fmt.Sprintf("/api/lists/%d", gid), nil, nil); code != 200 {
		t.Fatalf("viewer read: %d", code)
	}
	if code := viewer.do("POST", fmt.Sprintf("/api/lists/%d/items", gid), map[string]any{"text": "x"}, nil); code != 403 {
		t.Fatalf("viewer add: %d", code)
	}
	if code := viewer.do("PATCH", itemPath, map[string]any{"checked": false}, nil); code != 403 {
		t.Fatalf("viewer check: %d", code)
	}
}

func TestListsStaples(t *testing.T) {
	app, srv := newTestServer(t)
	c := newClient(t, srv)
	_, aid := c.signup()
	ctx := context.Background()
	gid := defaultListID(t, c)

	var st stapleRow
	if code := c.do("POST", "/api/staples", map[string]any{"list_id": gid, "name": "Rice", "qty": "5kg", "every_days": 14}, &st); code != 200 || st.LastAddedAt == nil {
		t.Fatalf("create staple: %d %+v", code, st)
	}
	if code := c.do("POST", "/api/staples", map[string]any{"list_id": gid, "name": "x", "every_days": 0}, nil); code != 400 {
		t.Fatalf("every_days 0: %d", code)
	}
	countRice := func() int {
		var n int
		_ = app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM shopping_items WHERE account_id=$1 AND name='Rice'", aid).Scan(&n)
		return n
	}
	if countRice() != 1 {
		t.Fatalf("staple should be added on create, got %d", countRice())
	}
	// repeated / concurrent lazy runs add nothing more
	done := make(chan struct{})
	for i := 0; i < 5; i++ {
		go func() { _, _ = app.addDueStaples(ctx, aid, time.Now()); done <- struct{}{} }()
	}
	for i := 0; i < 5; i++ {
		<-done
	}
	c.do("GET", "/api/lists", nil, nil)
	if countRice() != 1 {
		t.Fatalf("auto-add not idempotent: %d", countRice())
	}
	// bought, then 14 days later it's due again
	_, _ = app.DB.Exec(ctx, "DELETE FROM shopping_items WHERE account_id=$1", aid)
	_, _ = app.DB.Exec(ctx, "UPDATE staples SET last_added_at = now() - interval '13 days' WHERE id=$1", st.ID)
	if added, _ := app.addDueStaples(ctx, aid, time.Now()); len(added) != 0 {
		t.Fatalf("not due yet: %v", added)
	}
	_, _ = app.DB.Exec(ctx, "UPDATE staples SET last_added_at = now() - interval '15 days' WHERE id=$1", st.ID)
	var lr listResp
	var resp struct {
		StaplesAdded []string `json:"staples_added"`
	}
	c.do("GET", fmt.Sprintf("/api/lists/%d", gid), nil, &resp)
	c.do("GET", fmt.Sprintf("/api/lists/%d", gid), nil, &lr)
	if len(resp.StaplesAdded) != 1 || countRice() != 1 || lr.Items[0].StapleID == nil || lr.Items[0].Qty != "5kg" {
		t.Fatalf("due staple: %+v %+v", resp, lr.Items)
	}
	// already on the list (unchecked) -> not duplicated, occurrence consumed
	_, _ = app.DB.Exec(ctx, "UPDATE staples SET last_added_at = now() - interval '30 days' WHERE id=$1", st.ID)
	if added, _ := app.addDueStaples(ctx, aid, time.Now()); len(added) != 0 || countRice() != 1 {
		t.Fatalf("duplicate staple: %v %d", added, countRice())
	}
	// pause / list / delete
	c.do("PATCH", fmt.Sprintf("/api/staples/%d", st.ID), map[string]any{"active": false, "every_days": 7}, &st)
	if st.Active || st.EveryDays != 7 {
		t.Fatalf("patch staple: %+v", st)
	}
	var sl struct {
		Staples []stapleRow `json:"staples"`
	}
	c.do("GET", "/api/staples", nil, &sl)
	if len(sl.Staples) != 1 {
		t.Fatalf("list staples: %+v", sl)
	}
	c.do("DELETE", fmt.Sprintf("/api/staples/%d", st.ID), nil, nil)
	c.do("GET", "/api/staples", nil, &sl)
	if len(sl.Staples) != 0 {
		t.Fatalf("delete staple: %+v", sl)
	}
}

func TestListsTodayProvider(t *testing.T) {
	app, srv := newTestServer(t)
	c := newClient(t, srv)
	_, aid := c.signup()
	ctx := context.Background()
	day := householdToday(ctx, app, aid, time.Now())
	sec, err := listsTodayProvider(ctx, app, aid, memberInfo{}, day)
	if err != nil || sec != nil {
		t.Fatalf("empty lists -> nil section, got %+v %v", sec, err)
	}
	gid := defaultListID(t, c)
	c.do("POST", fmt.Sprintf("/api/lists/%d/items", gid), map[string]any{"text": "a, b, c, d, e, f, g"}, nil)
	c.do("POST", "/api/staples", map[string]any{"list_id": gid, "name": "Rice", "every_days": 14}, nil)
	sec, err = listsTodayProvider(ctx, app, aid, memberInfo{}, day)
	if err != nil || sec == nil || len(sec.Items) != 1 {
		t.Fatalf("section: %+v %v", sec, err)
	}
	it := sec.Items[0]
	if it.Kind != "list" || it.Title != "Groceries: 8 items to buy" || it.Link != fmt.Sprintf("/home/lists?list=%d", gid) ||
		!strings.Contains(it.Detail, "Staples added today: Rice") || len(it.Actions) != 0 {
		t.Fatalf("item: %+v", it)
	}
	if sec.Summary != "Groceries: 8 items to buy" {
		t.Fatalf("summary: %q", sec.Summary)
	}
}

func TestListsBotEndpoints(t *testing.T) {
	app, srv := newTestServer(t)
	c := newClient(t, srv)
	_, aid := c.signup()
	ctx := context.Background()
	var integ map[string]any
	if code := c.do("POST", "/api/integrations", map[string]any{"platform": "telegram", "platform_token": "123:abc"}, &integ); code != 200 {
		t.Fatalf("integration: %d %v", code, integ)
	}
	var token string
	_ = app.DB.QueryRow(ctx, "SELECT bot_token FROM integrations WHERE account_id=$1", aid).Scan(&token)
	kidEmail, kidID := addMember(t, c, "kid", nil)
	_ = kidEmail
	_, _ = app.DB.Exec(ctx, "UPDATE members SET telegram_user_id='777' WHERE id=$1", kidID)

	bot := func(method, path, user string, body any, out any) int {
		t.Helper()
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = strings.NewReader(string(b))
		}
		req, _ := http.NewRequest(method, srv.URL+path, rd)
		req.Header.Set("X-Bot-Token", token)
		if user != "" {
			req.Header.Set("X-Bot-User", user)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if out != nil {
			_ = json.NewDecoder(resp.Body).Decode(out)
		}
		return resp.StatusCode
	}

	var addRes struct {
		OK    bool     `json:"ok"`
		List  string   `json:"list"`
		Added []string `json:"added"`
	}
	if code := bot("POST", "/api/bot/lists/items", "telegram:777", map[string]any{"text": "eggs, 2 milk\nbread x3"}, &addRes); code != 200 ||
		!addRes.OK || addRes.List != "Groceries" || strings.Join(addRes.Added, ",") != "eggs,milk,bread" {
		t.Fatalf("bot add: %d %+v", code, addRes)
	}
	var addedBy *int64
	_ = app.DB.QueryRow(ctx, "SELECT added_by FROM shopping_items WHERE account_id=$1 AND name='eggs'", aid).Scan(&addedBy)
	if addedBy == nil || *addedBy != kidID {
		t.Fatalf("bot actor attribution: %v", addedBy)
	}
	if code := bot("POST", "/api/bot/lists/items", "", map[string]any{"text": "nails", "list": "nope"}, nil); code != 404 {
		t.Fatalf("unknown list: %d", code)
	}
	c.do("POST", "/api/lists", map[string]any{"name": "Hardware"}, nil)
	bot("POST", "/api/bot/lists/items", "", map[string]any{"text": "nails", "list": "hard"}, &addRes)
	if addRes.List != "Hardware" {
		t.Fatalf("prefix list match: %+v", addRes)
	}

	var items struct {
		List  string `json:"list"`
		Items []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Qty  string `json:"qty"`
		} `json:"items"`
	}
	if code := bot("GET", "/api/bot/lists", "", nil, &items); code != 200 || items.List != "Groceries" || len(items.Items) != 3 || items.Items[1].Qty != "2" {
		t.Fatalf("bot list: %d %+v", code, items)
	}
	if code := bot("GET", "/api/bot/lists", "", nil, nil); code != 200 {
		t.Fatal("bot list again")
	}
	req, _ := http.NewRequest("GET", srv.URL+"/api/bot/lists", nil)
	req.Header.Set("X-Bot-Token", "wrong")
	if resp, _ := http.DefaultClient.Do(req); resp.StatusCode != 403 {
		t.Fatalf("bad token: %d", resp.StatusCode)
	}

	var fin struct {
		OK         bool   `json:"ok"`
		Message    string `json:"message"`
		NeedsTotal bool   `json:"needs_total"`
		SpendingID int64  `json:"spending_id"`
	}
	bot("POST", "/api/bot/lists/finish", "", map[string]any{}, &fin)
	if fin.OK || !fin.NeedsTotal || !strings.Contains(fin.Message, "3 items") {
		t.Fatalf("finish without total: %+v", fin)
	}
	bot("POST", "/api/bot/lists/finish", "telegram:777", map[string]any{"total_amount": 1000}, &fin)
	if fin.OK {
		t.Fatalf("kid must not finish via bot: %+v", fin)
	}
	// one item checked -> only that one is bought
	c.do("PATCH", fmt.Sprintf("/api/lists/%d/items/%d", defaultListID(t, c), items.Items[0].ID), map[string]any{"checked": true}, nil)
	bot("POST", "/api/bot/lists/finish", "", map[string]any{"total_amount": "120k"}, &fin)
	if !fin.OK || fin.SpendingID == 0 || !strings.Contains(fin.Message, "Rp120.000") {
		t.Fatalf("bot finish: %+v", fin)
	}
	var member *int64
	var amount int64
	_ = app.DB.QueryRow(ctx, "SELECT member_id, amount FROM spending WHERE id=$1", fin.SpendingID).Scan(&member, &amount)
	if member != nil || amount != 120000 {
		t.Fatalf("unlinked bot spending: member=%v amount=%d", member, amount)
	}
	bot("GET", "/api/bot/lists", "", nil, &items)
	if len(items.Items) != 2 {
		t.Fatalf("after checked finish: %+v", items)
	}
	// nothing checked -> everything
	bot("POST", "/api/bot/lists/finish", "", map[string]any{"total_amount": 5000}, &fin)
	bot("GET", "/api/bot/lists", "", nil, &items)
	if !fin.OK || len(items.Items) != 0 {
		t.Fatalf("finish all: %+v %+v", fin, items)
	}
}
