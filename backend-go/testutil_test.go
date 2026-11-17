package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
)

// Integration tests need a real Postgres: set TEST_DATABASE_URL (see
// scripts/test.sh, which spins up a throwaway container). Without it they
// t.Skip and only the pure unit tests run. Tests never drop or truncate
// anything - each one works in fresh accounts with unique emails.

var testApp *App

func TestMain(m *testing.M) {
	initSessionStore("test-secret-key-for-go-tests-only", false)
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		os.Exit(m.Run())
	}
	ctx := context.Background()
	pool, err := newPool(ctx, url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "test db connect:", err)
		os.Exit(1)
	}
	if err := pool.Ping(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "test db ping:", err)
		os.Exit(1)
	}
	testApp = &App{DB: pool, Cfg: Config{ServiceToken: "test-service-token"}, quietLogs: true}
	// Run twice: schema.sql must be idempotent.
	for i := 0; i < 2; i++ {
		if err := initSchema(ctx, testApp); err != nil {
			fmt.Fprintln(os.Stderr, "schema init:", err)
			os.Exit(1)
		}
	}
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

func requireDB(t *testing.T) *App {
	t.Helper()
	if testApp == nil {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	return testApp
}

// testClient is an HTTP client with its own cookie jar (one logged-in user).
type testClient struct {
	t    *testing.T
	base string
	c    *http.Client
}

func newTestServer(t *testing.T) (*App, *httptest.Server) {
	app := requireDB(t)
	srv := httptest.NewServer(newRouter(app))
	t.Cleanup(srv.Close)
	return app, srv
}

func newClient(t *testing.T, srv *httptest.Server) *testClient {
	jar, _ := cookiejar.New(nil)
	return &testClient{t: t, base: srv.URL, c: &http.Client{Jar: jar}}
}

// do sends a JSON request and decodes the JSON response into out (if non-nil).
func (tc *testClient) do(method, path string, body any, out any) int {
	tc.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, tc.base+path, rd)
	if err != nil {
		tc.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := tc.c.Do(req)
	if err != nil {
		tc.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			tc.t.Fatalf("%s %s: decode %q: %v", method, path, raw, err)
		}
	}
	return resp.StatusCode
}

func uniqueEmail() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "user-" + hex.EncodeToString(b) + "@example.test"
}

// signup creates a fresh account, logs the client in, returns its id.
func (tc *testClient) signup() (email string, accountID int64) {
	tc.t.Helper()
	email = uniqueEmail()
	var res map[string]any
	if code := tc.do("POST", "/api/signup", map[string]any{"email": email, "password": "password123"}, &res); code != 200 {
		tc.t.Fatalf("signup: status %d %v", code, res)
	}
	if err := testApp.DB.QueryRow(context.Background(), "SELECT id FROM users WHERE email=$1", email).Scan(&accountID); err != nil {
		tc.t.Fatal(err)
	}
	return email, accountID
}
