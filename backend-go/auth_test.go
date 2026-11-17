package main

import "testing"

func TestSignupLoginMeFlow(t *testing.T) {
	_, srv := newTestServer(t)
	c := newClient(t, srv)

	var me map[string]any
	c.do("GET", "/api/me", nil, &me)
	if me["authenticated"] != false {
		t.Fatalf("anonymous /me: %v", me)
	}

	// validation
	if code := c.do("POST", "/api/signup", map[string]any{"email": "nope", "password": "x"}, nil); code != 400 {
		t.Fatalf("bad signup: want 400, got %d", code)
	}

	email, _ := c.signup()
	me = nil
	c.do("GET", "/api/me", nil, &me)
	if me["authenticated"] != true || me["username"] != email {
		t.Fatalf("/me after signup: %v", me)
	}

	// duplicate signup
	if code := newClient(t, srv).do("POST", "/api/signup", map[string]any{"email": email, "password": "password123"}, nil); code != 409 {
		t.Fatalf("duplicate signup: want 409, got %d", code)
	}

	// signup seeds default categories/accounts/income sources
	var setup struct {
		Categories    []map[string]any `json:"categories"`
		Accounts      []map[string]any `json:"accounts"`
		IncomeSources []map[string]any `json:"income_sources"`
	}
	c.do("GET", "/api/setup", nil, &setup)
	if len(setup.Categories) == 0 || len(setup.Accounts) == 0 || len(setup.IncomeSources) == 0 {
		t.Fatalf("defaults not seeded: %+v", setup)
	}

	// logout
	c.do("POST", "/api/logout", nil, nil)
	me = nil
	c.do("GET", "/api/me", nil, &me)
	if me["authenticated"] != false {
		t.Fatalf("/me after logout: %v", me)
	}

	// wrong password
	if code := c.do("POST", "/api/login", map[string]any{"email": email, "password": "wrong-password"}, nil); code != 401 {
		t.Fatalf("bad login: want 401, got %d", code)
	}
	// login (case-insensitive email, via legacy "username" field)
	var res map[string]any
	if code := c.do("POST", "/api/login", map[string]any{"username": "  " + email + " ", "password": "password123"}, &res); code != 200 {
		t.Fatalf("login: %d %v", code, res)
	}
	me = nil
	c.do("GET", "/api/me", nil, &me)
	if me["authenticated"] != true || me["username"] != email {
		t.Fatalf("/me after login: %v", me)
	}

	// protected endpoint without session
	if code := newClient(t, srv).do("GET", "/api/spending", nil, nil); code != 401 {
		t.Fatalf("unauthenticated /spending: want 401, got %d", code)
	}
}
