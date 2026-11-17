package main

import (
	"context"
	"strings"
	"testing"
)

func TestMaskSecret(t *testing.T) {
	if got := maskSecret("123456789:ABCdefGHIjklWXYZ"); got != "••••WXYZ" {
		t.Fatalf("got %q", got)
	}
	if got := maskSecret("short"); got != "••••" {
		t.Fatalf("short secret must not leak chars: %q", got)
	}
}

func TestIntegrationsMasking(t *testing.T) {
	app, srv := newTestServer(t)
	c := newClient(t, srv)
	_, aid := c.signup()
	const secret = "123456789:SuperSecretTelegramTokenABCD"

	var created map[string]any
	if code := c.do("POST", "/api/integrations", map[string]any{"platform": "telegram", "platform_token": secret}, &created); code != 200 {
		t.Fatalf("create: %d %v", code, created)
	}
	id := int64(created["id"].(float64))

	list := func() map[string]any {
		var rows []map[string]any
		if code := c.do("GET", "/api/integrations", nil, &rows); code != 200 || len(rows) != 1 {
			t.Fatalf("list: %d %v", code, rows)
		}
		return rows[0]
	}
	row := list()
	if _, leaked := row["platform_token"]; leaked {
		t.Fatalf("platform_token must not be returned: %v", row)
	}
	for _, v := range row {
		if s, ok := v.(string); ok && strings.Contains(s, "SuperSecret") {
			t.Fatalf("secret leaked in %v", row)
		}
	}
	if row["has_platform_token"] != true || row["platform_token_masked"] != "••••ABCD" {
		t.Fatalf("masking fields wrong: %v", row)
	}

	// PATCH with empty/absent platform_token keeps the stored secret.
	c.do("PATCH", "/api/integrations/"+itoa(id), map[string]any{"platform_token": "", "chat_id": "42", "enabled": true}, nil)
	c.do("PATCH", "/api/integrations/"+itoa(id), map[string]any{"platform_token": nil}, nil)
	var stored string
	if err := app.DB.QueryRow(context.Background(), "SELECT platform_token FROM integrations WHERE id=$1", id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != secret {
		t.Fatalf("empty PATCH overwrote token: %q", stored)
	}
	if row = list(); row["chat_id"] != "42" {
		t.Fatalf("chat_id not updated: %v", row)
	}

	// Non-empty value replaces it.
	c.do("PATCH", "/api/integrations/"+itoa(id), map[string]any{"platform_token": "999:NewTokenValueWXYZ"}, nil)
	if row = list(); row["platform_token_masked"] != "••••WXYZ" {
		t.Fatalf("token not replaced: %v", row)
	}

	// The service-token internal endpoint (used by bots/) still sees the raw token.
	sc := newClient(t, srv)
	var internal []map[string]any
	sc.c.Transport = headerRT{"X-Service-Token", "wrong"}
	if code := sc.do("GET", "/api/internal/integrations", nil, nil); code != 403 {
		t.Fatalf("internal with bad token: %d", code)
	}
	sc.c.Transport = headerRT{"X-Service-Token", app.Cfg.ServiceToken}
	if code := sc.do("GET", "/api/internal/integrations?platform=telegram", nil, &internal); code != 200 {
		t.Fatalf("internal: %d", code)
	}
	found := false
	for _, it := range internal {
		if int64(it["account_id"].(float64)) == aid {
			found = it["platform_token"] == "999:NewTokenValueWXYZ"
		}
	}
	if !found {
		t.Fatalf("internal endpoint must expose raw token for bots: %v", internal)
	}
}
