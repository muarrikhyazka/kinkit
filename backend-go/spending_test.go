package main

import (
	"context"
	"testing"
)

func TestAddSpendingAndClientIDIdempotency(t *testing.T) {
	app, srv := newTestServer(t)
	c := newClient(t, srv)
	_, aid := c.signup()

	body := map[string]any{
		"tx_date": "2026-03-04", "description": "Groceries", "category": "Food",
		"amount": 125000, "client_id": "offline-abc-1",
	}
	var first, second map[string]any
	if code := c.do("POST", "/api/spending", body, &first); code != 200 || first["ok"] != true {
		t.Fatalf("add spending: %d %v", code, first)
	}
	if first["duplicate"] != nil {
		t.Fatalf("first insert flagged duplicate: %v", first)
	}
	if code := c.do("POST", "/api/spending", body, &second); code != 200 {
		t.Fatalf("replay: %d %v", code, second)
	}
	if second["duplicate"] != true || second["id"] != first["id"] {
		t.Fatalf("replay should return existing id with duplicate=true: first=%v second=%v", first, second)
	}

	// a different client_id is a new row; no client_id is always new
	c.do("POST", "/api/spending", map[string]any{"tx_date": "2026-03-04", "description": "Bus", "amount": 5000, "client_id": "offline-abc-2"}, nil)
	c.do("POST", "/api/spending", map[string]any{"tx_date": "2026-03-04", "description": "Coffee", "amount": 3000}, nil)

	var n int
	if err := app.DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM spending WHERE account_id=$1", aid).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("want 3 spending rows, got %d", n)
	}

	var list struct {
		Rows  []spendingRow `json:"rows"`
		Total int64         `json:"total"`
	}
	c.do("GET", "/api/spending?year=2026&month=3", nil, &list)
	if list.Total != 3 || len(list.Rows) != 3 {
		t.Fatalf("list: %+v", list)
	}
	var food *spendingRow
	for i := range list.Rows {
		if list.Rows[i].Description == "Groceries" {
			food = &list.Rows[i]
		}
	}
	if food == nil || food.CategoryID == nil || food.CategoryName == nil || *food.CategoryName != "Food" || food.TxDate != "2026-03-04" {
		t.Fatalf("groceries row wrong: %+v", food)
	}

	// client_id idempotency is per account: another user may reuse it
	c2 := newClient(t, srv)
	c2.signup()
	var other map[string]any
	c2.do("POST", "/api/spending", body, &other)
	if other["duplicate"] != nil || other["id"] == first["id"] {
		t.Fatalf("client_id leaked across accounts: %v", other)
	}
}
