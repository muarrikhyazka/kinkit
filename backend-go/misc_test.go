package main

import (
	"net/http"
	"strconv"
	"testing"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// headerRT adds a fixed header to every request.
type headerRT struct{ k, v string }

func (h headerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set(h.k, h.v)
	return http.DefaultTransport.RoundTrip(r)
}

func TestValidateConfigSecretKey(t *testing.T) {
	for _, tc := range []struct {
		secret, env string
		wantErr     bool
	}{
		{"", "", true},
		{"dev-secret-change-me", "", true},
		{"change-me-to-a-random-string", "production", true},
		{"change-me-to-a-random-string", "development", false},
		{"", "development", false},
		{"a-long-random-production-secret", "", false},
	} {
		t.Setenv("SECRET_KEY", tc.secret)
		err := validateConfig(Config{AppEnv: tc.env})
		if (err != nil) != tc.wantErr {
			t.Errorf("secret=%q env=%q: err=%v wantErr=%v", tc.secret, tc.env, err, tc.wantErr)
		}
	}
}

func TestCookieSecureDefault(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	for _, tc := range []struct {
		base, flag string
		want       bool
	}{
		{"https://kinkit.example", "", true},
		{"http://localhost:5173", "", false},
		{"https://kinkit.example", "false", false},
		{"http://localhost:5173", "true", true},
	} {
		t.Setenv("APP_BASE_URL", tc.base)
		t.Setenv("COOKIE_SECURE", tc.flag)
		if got := loadConfig().CookieSecure; got != tc.want {
			t.Errorf("base=%q flag=%q: got %v want %v", tc.base, tc.flag, got, tc.want)
		}
	}
}
