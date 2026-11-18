package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is the shared Postgres connection pool, available to every handler via
// the App struct. All SQL in this project is raw (no ORM) - same philosophy
// as the Python version it replaces.
type App struct {
	DB        *pgxpool.Pool
	Cfg       Config
	quietLogs bool // tests: skip per-request access logging
}

type Config struct {
	Port          string
	DatabaseURL   string
	SessionSecret string
	StaticDir     string
	AppBaseURL    string
	ServiceToken  string // shared secret for /internal/* endpoints (bot multiplexers)
	AppEnv        string // "development" relaxes production-only safety checks
	CookieSecure  bool   // session cookie Secure flag (COOKIE_SECURE; default true when APP_BASE_URL is https)
	// TrustedProxies are the peers whose X-Forwarded-For / X-Real-IP headers
	// we honor (TRUSTED_PROXIES, comma-separated CIDRs or bare IPs).
	TrustedProxies []netip.Prefix
}

// defaultTrustedProxies covers loopback + private ranges (docker networks,
// a reverse proxy on the same host).
const defaultTrustedProxies = "127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,::1"

// insecureSecrets are placeholder SECRET_KEY values that must never reach
// production (they'd let anyone forge session cookies).
var insecureSecrets = map[string]bool{
	"":                             true,
	"dev-secret-change-me":         true,
	"change-me-to-a-random-string": true,
}

func loadConfig() Config {
	cfg := Config{
		Port:          getenv("PORT", "5000"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		SessionSecret: getenv("SECRET_KEY", "dev-secret-change-me"),
		StaticDir:     getenv("STATIC_DIR", "/app/react_dist"),
		AppBaseURL:    getenv("APP_BASE_URL", "http://localhost:5173"),
		ServiceToken:  os.Getenv("SERVICE_TOKEN"),
		AppEnv:        strings.ToLower(os.Getenv("APP_ENV")),
	}
	cfg.CookieSecure = parseBoolDefault(os.Getenv("COOKIE_SECURE"), strings.HasPrefix(strings.ToLower(cfg.AppBaseURL), "https://"))
	proxies, err := parseTrustedProxies(getenv("TRUSTED_PROXIES", defaultTrustedProxies))
	if err != nil {
		log.Fatalf("TRUSTED_PROXIES: %v", err)
	}
	cfg.TrustedProxies = proxies
	return cfg
}

// validateConfig enforces production safety rules; returns an error the
// caller should treat as fatal.
func validateConfig(cfg Config) error {
	if cfg.AppEnv != "development" && insecureSecrets[os.Getenv("SECRET_KEY")] {
		return fmt.Errorf("SECRET_KEY is unset or a placeholder; set a long random value (or APP_ENV=development for local dev)")
	}
	return nil
}

func parseBoolDefault(v string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

// newPool opens a pgx pool whose sessions run in UTC, so now()/CURRENT_DATE
// and date casts agree with the server's time.Now().UTC().
func newPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	pc.ConnConfig.RuntimeParams["timezone"] = "UTC"
	return pgxpool.NewWithConfig(ctx, pc)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	cfg := loadConfig()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if err := validateConfig(cfg); err != nil {
		log.Fatal(err)
	}
	trustedProxies = cfg.TrustedProxies

	pool, err := newPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	app := &App{DB: pool, Cfg: cfg}
	if err := initSchema(context.Background(), app); err != nil {
		log.Fatalf("schema init: %v", err)
	}
	initSessionStore(cfg.SessionSecret, cfg.CookieSecure)
	app.startMoneyWorker(context.Background())     // --- money ---: recurring generation + budget alerts
	app.startRemindersWorker(context.Background()) // --- reminders ---: occurrences, chat sends, escalation, digest
	app.startChoreWorker(context.Background())     // --- chores ---: hourly chore instance generation

	r := newRouter(app)

	log.Printf("kinkit listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}

// newRouter builds the full HTTP handler (API + SPA fallback).
func newRouter(app *App) http.Handler {
	r := chi.NewRouter()
	if !app.quietLogs {
		r.Use(middleware.Logger)
	}
	r.Use(middleware.Recoverer)

	r.Route("/api", func(api chi.Router) {
		// --- family ---
		api.Use(app.memberMiddleware) // resolves acting member + central role rules (family.go)
		// --- end family ---
		api.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		})
		RegisterAuthRoutes(api, app)         // login/signup/me/logout/password-reset/verify-email
		RegisterFinanceRoutes(api, app)      // spending/budgeting/assets/setup/categories/accounts/income-sources/reports/trend/fx-rates
		RegisterAdminRoutes(api, app)        // super-admin: accounts/stats/reset-demo-data
		RegisterIntegrationRoutes(api, app)  // per-account integrations CRUD
		RegisterBotRoutes(api, app)          // bot-token authenticated /api/bot/*
		RegisterInternalRoutes(api, app)     // service-token authenticated /api/internal/*
		RegisterGamificationRoutes(api, app) // points/streak/badge
		RegisterFamilyRoutes(api, app)       // --- family --- household members/roles/allowance
		RegisterMoneyRoutes(api, app)        // --- money ---: recurring/income/goals/budget actuals/alerts
		RegisterDataRoutes(api, app)         // --- data ---: export/import, receipts, dashboard layout, TV links
		RegisterReminderRoutes(api, app)     // --- reminders ---: reminders, today, household tz/digest, chat link, bot
		RegisterChoreRoutes(api, app)        // --- chores ---: chores, instances, approvals, /bot/chores/*
		RegisterListRoutes(api, app)         // --- lists ---: shopping lists, staples, finish shopping, /bot/lists/*
	})

	if app.Cfg.StaticDir != "" {
		// SPA static files - serve react build, fallback to index.html for client routing
		r.Get("/*", SPAHandler(app.Cfg.StaticDir))
	}
	return r
}
