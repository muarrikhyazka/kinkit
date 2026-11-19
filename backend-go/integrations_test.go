package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestSlackMatrixIntegrations(t *testing.T) {
	_, srv := newTestServer(t)
	c := newClient(t, srv)
	c.signup()
	if code := c.do("POST", "/api/integrations", map[string]any{"platform": "matrix", "platform_token": "tok"}, nil); code != 400 {
		t.Fatalf("matrix without homeserver: %d", code)
	}
	if code := c.do("POST", "/api/integrations", map[string]any{"platform": "whatsapp", "platform_token": "tok"}, nil); code != 400 {
		t.Fatalf("unknown platform: %d", code)
	}
	if code := c.do("POST", "/api/integrations", map[string]any{"platform": "slack", "platform_token": "xoxb-1", "extra": "xapp-SECRET", "chat_id": "C1"}, nil); code != 200 {
		t.Fatalf("slack: %d", code)
	}
	if code := c.do("POST", "/api/integrations", map[string]any{"platform": "matrix", "platform_token": "tok", "extra": "https://m.example.org", "chat_id": "!r:example.org"}, nil); code != 200 {
		t.Fatalf("matrix: %d", code)
	}
	var rows []map[string]any
	c.do("GET", "/api/integrations", nil, &rows)
	if len(rows) != 2 {
		t.Fatalf("rows: %v", rows)
	}
	for _, r := range rows {
		if r["has_extra"] != true {
			t.Errorf("%v: has_extra", r)
		}
		if r["platform"] == "slack" && r["extra"] != nil {
			t.Errorf("slack app token leaked: %v", r)
		}
		if r["platform"] == "matrix" && r["extra"] != "https://m.example.org" {
			t.Errorf("matrix homeserver missing: %v", r)
		}
	}
}

func TestSendSlackAndMatrix(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotAuth, gotPath, gotBody = r.Header.Get("Authorization"), r.URL.Path, string(b)
		if strings.Contains(r.URL.Path, "chat.postMessage") && strings.Contains(gotBody, "badchan") {
			io.WriteString(w, `{"ok":false,"error":"channel_not_found"}`)
			return
		}
		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()
	old := slackAPIBase
	slackAPIBase = srv.URL
	defer func() { slackAPIBase = old }()

	msg := chatMessage{Text: "hi", Mention: "U42", Buttons: [][]chatButton{{{Label: "Done", Data: "rem:1:done"}}}}
	if err := sendReminderChat(context.Background(), "slack", "xoxb-1", "C1", msg); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer xoxb-1" || !strings.HasSuffix(gotPath, "/chat.postMessage") ||
		!strings.Contains(gotBody, `@U42\u003e hi`) || !strings.Contains(gotBody, `"action_id":"rem:1:done"`) {
		t.Fatalf("slack request: %s %s %s", gotAuth, gotPath, gotBody)
	}
	err := sendChatMessage(context.Background(), "slack", "xoxb-1", "badchan", "x")
	if err == nil || !strings.Contains(err.Error(), "channel_not_found") {
		t.Fatalf("slack ok:false must be an error, got %v", err)
	}

	if err := sendChatMessage(context.Background(), "matrix", srv.URL+" mxtok", "!room:ex.org", "hello"); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer mxtok" || !strings.Contains(gotPath, "/_matrix/client/v3/rooms/") || !strings.Contains(gotBody, `"body":"hello"`) {
		t.Fatalf("matrix request: %s %s %s", gotAuth, gotPath, gotBody)
	}
	rem := chatMessage{Text: "Reminder: bins", Buttons: reminderButtons(12)}
	if err := sendReminderChat(context.Background(), "matrix", srv.URL+" mxtok", "!room:ex.org", rem); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotBody, "!rem 12 done") {
		t.Fatalf("matrix reminder lacks reply hint: %s", gotBody)
	}
	if err := sendChatMessage(context.Background(), "matrix", "tokenwithoutserver", "!r", "x"); err == nil {
		t.Fatal("matrix without homeserver must fail")
	}
}
