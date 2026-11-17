package main

import "testing"

func TestBudgetingUpsert(t *testing.T) {
	_, srv := newTestServer(t)
	c := newClient(t, srv)
	c.signup()

	var setup struct {
		Categories    []categoryRow     `json:"categories"`
		IncomeSources []incomeSourceRow `json:"income_sources"`
	}
	c.do("GET", "/api/setup", nil, &setup)
	src, cat := setup.IncomeSources[0], setup.Categories[0]

	for _, amt := range []int64{1000, 2500} { // second call must update, not duplicate
		if code := c.do("POST", "/api/budgeting/income", map[string]any{"year": 2026, "month": 5, "source_id": src.ID, "amount": amt}, nil); code != 200 {
			t.Fatalf("set income: %d", code)
		}
		if code := c.do("POST", "/api/budgeting/expense", map[string]any{"year": 2026, "month": 5, "category_id": cat.ID, "amount": amt * 2}, nil); code != 200 {
			t.Fatalf("set expense: %d", code)
		}
	}

	var b struct {
		Income []struct {
			Month      int64  `json:"month"`
			SourceID   int64  `json:"source_id"`
			Amount     int64  `json:"amount"`
			SourceName string `json:"source_name"`
		} `json:"income"`
		Expense []struct {
			Month        int64  `json:"month"`
			CategoryID   int64  `json:"category_id"`
			Amount       int64  `json:"amount"`
			CategoryName string `json:"category_name"`
		} `json:"expense"`
	}
	if code := c.do("GET", "/api/budgeting/2026", nil, &b); code != 200 {
		t.Fatalf("get budgeting: %d", code)
	}
	if len(b.Income) != 1 || b.Income[0].Amount != 2500 || b.Income[0].SourceID != src.ID || b.Income[0].SourceName != src.Name {
		t.Fatalf("income upsert wrong: %+v", b.Income)
	}
	if len(b.Expense) != 1 || b.Expense[0].Amount != 5000 || b.Expense[0].CategoryID != cat.ID {
		t.Fatalf("expense upsert wrong: %+v", b.Expense)
	}

	// another year is empty
	var empty map[string]any
	c.do("GET", "/api/budgeting/2025", nil, &empty)
	if inc, _ := empty["income"].([]any); len(inc) != 0 {
		t.Fatalf("2025 should be empty: %v", empty)
	}
}
