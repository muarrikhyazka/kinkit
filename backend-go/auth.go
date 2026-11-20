package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

// static data (ported from Python CURRENCIES / DEFAULT_* dicts)

type currencyInfo struct {
	Symbol string `json:"symbol"`
	Locale string `json:"locale"`
}

var currencies = map[string]currencyInfo{
	"IDR": {"Rp", "id-ID"},
	"USD": {"$", "en-US"},
	"EUR": {"€", "de-DE"},
	"GBP": {"£", "en-GB"},
	"SGD": {"S$", "en-SG"},
	"MYR": {"RM", "ms-MY"},
	"JPY": {"¥", "ja-JP"},
	"AUD": {"A$", "en-AU"},
	"INR": {"₹", "en-IN"},
}

var defaultCategories = map[string][]string{
	"Needs": {"Home Rent", "Utilities", "Food", "Supplies", "Transportation", "Healthcare",
		"Energy", "Skincare & Beauty", "Services", "Others"},
	"Wants": {"Clothes & Bags", "Application", "Charity", "Travel", "Gifts", "Learning",
		"Jajan", "Internet & Pulsa", "Electricity", "Entertainment", "Business"},
	"Savings": {"Child", "Emergency Funds", "House", "Car"},
}
var defaultIncomeSources = []string{"Salary", "Freelance"}
var defaultBankAccounts = []string{"Bank 1", "Bank 2"}

// seedDefaults ports Python's seed_defaults(): starter categories/accounts/
// income sources for a brand-new signup. All ON CONFLICT DO NOTHING, same
// as SQLite's INSERT OR IGNORE.
func seedDefaults(ctx context.Context, app *App, accountID int64) error {
	for _, name := range defaultBankAccounts {
		if _, err := app.DB.Exec(ctx,
			"INSERT INTO accounts (account_id, name, kind) VALUES ($1,$2,'bank') ON CONFLICT (account_id, name) DO NOTHING",
			accountID, name); err != nil {
			return err
		}
	}
	for _, name := range defaultIncomeSources {
		if _, err := app.DB.Exec(ctx,
			"INSERT INTO income_sources (account_id, name) VALUES ($1,$2) ON CONFLICT (account_id, name) DO NOTHING",
			accountID, name); err != nil {
			return err
		}
	}
	for mainCat, names := range defaultCategories {
		for _, name := range names {
			if _, err := app.DB.Exec(ctx,
				"INSERT INTO categories (account_id, name, main_category) VALUES ($1,$2,$3) ON CONFLICT (account_id, name) DO NOTHING",
				accountID, name, mainCat); err != nil {
				return err
			}
		}
	}
	return nil
}

// rate limiting
// ponytail: in-memory sliding window, single-process only (same limitation
// as the Python original) - swap for a shared store (Redis) if we ever run
// >1 instance.
const (
	rateLimitWindow = 5 * time.Minute
	rateLimitMax    = 5
)

var (
	rateLimitMu   sync.Mutex
	rateLimitHits = map[string][]time.Time{}
)

func rateLimited(key string) bool {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	now := time.Now().UTC()
	hits := rateLimitHits[key][:0:0]
	for _, t := range rateLimitHits[key] {
		if now.Sub(t) < rateLimitWindow {
			hits = append(hits, t)
		}
	}
	hits = append(hits, now)
	rateLimitHits[key] = hits
	return len(hits) > rateLimitMax
}

// trustedProxies is set from Config.TrustedProxies at startup.
var trustedProxies []netip.Prefix

func clientIP(r *http.Request) string {
	return resolveClientIP(r.RemoteAddr, r.Header.Values("X-Forwarded-For"), r.Header.Get("X-Real-IP"), trustedProxies)
}

// resolveClientIP returns the real client address. Forwarding headers are
// only honored when the direct peer (remoteAddr) is a trusted proxy, and
// then we walk X-Forwarded-For right-to-left, skipping trusted hops, and
// take the first untrusted address (left-most entries are client-controlled
// and can be spoofed). X-Real-IP is the fallback when XFF is absent.
func resolveClientIP(remoteAddr string, xff []string, xRealIP string, trusted []netip.Prefix) string {
	peer := stripPort(remoteAddr)
	peerAddr, err := netip.ParseAddr(peer)
	if err != nil || !isTrusted(peerAddr, trusted) {
		return peer
	}
	var hops []string
	for _, h := range xff {
		for _, p := range strings.Split(h, ",") {
			if p = strings.TrimSpace(p); p != "" {
				hops = append(hops, p)
			}
		}
	}
	for i := len(hops) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(stripPort(hops[i]))
		if err != nil {
			// garbage in the chain: stop trusting anything further left
			return peer
		}
		if !isTrusted(a, trusted) {
			return a.String()
		}
		if i == 0 {
			return a.String() // every hop trusted - left-most is the origin
		}
	}
	if xr := strings.TrimSpace(xRealIP); xr != "" {
		if a, err := netip.ParseAddr(stripPort(xr)); err == nil {
			return a.String()
		}
	}
	return peer
}

func isTrusted(a netip.Addr, trusted []netip.Prefix) bool {
	a = a.Unmap()
	for _, p := range trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// stripPort removes a :port suffix from "ip:port" / "[v6]:port"; bare IPs
// (including bare IPv6) are returned unchanged.
func stripPort(s string) string {
	s = strings.TrimSpace(s)
	if host, _, err := net.SplitHostPort(s); err == nil {
		return host
	}
	return strings.Trim(s, "[]")
}

// parseTrustedProxies parses a comma-separated list of CIDRs or bare IPs.
func parseTrustedProxies(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			p, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, err
			}
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(part)
		if err != nil {
			return nil, err
		}
		a = a.Unmap()
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}

// rateLimitMiddleware mirrors Python's rate_limit(prefix) decorator: 5
// attempts/5min per IP+email (email/username read from the JSON body,
// body is restored for the real handler afterwards).
func rateLimitMiddleware(prefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var d map[string]any
			_ = decodeJSON(r, &d)
			ident := ""
			if d != nil {
				if v, ok := d["email"].(string); ok && v != "" {
					ident = v
				} else if v, ok := d["username"].(string); ok {
					ident = v
				}
			}
			ident = strings.ToLower(strings.TrimSpace(ident))
			key := prefix + ":" + clientIP(r) + ":" + ident
			if rateLimited(key) {
				jsonError(w, http.StatusTooManyRequests, "too many attempts, try again in a few minutes")
				return
			}
			// handler needs to read the JSON body again - re-marshal what we decoded.
			if d != nil {
				b, _ := json.Marshal(d)
				r.Body = io.NopCloser(bytes.NewReader(b))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// auth routes

func RegisterAuthRoutes(r chi.Router, app *App) {
	r.With(rateLimitMiddleware("signup")).Post("/signup", app.handleSignup)
	r.With(rateLimitMiddleware("login")).Post("/login", app.handleLogin)
	r.Post("/logout", app.handleLogout)
	r.Get("/me", app.handleMe)
	r.Patch("/me/settings", app.handleUpdateSettings)
	r.Get("/currencies", app.handleCurrencies)
	r.Get("/verify-email", app.handleVerifyEmail)
	r.With(rateLimitMiddleware("pwreset")).Post("/password-reset/request", app.handlePasswordResetRequest)
	r.Post("/password-reset/confirm", app.handlePasswordResetConfirm)
}

func (app *App) handleSignup(w http.ResponseWriter, r *http.Request) {
	var d struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	_ = decodeJSON(r, &d)
	email := strings.ToLower(strings.TrimSpace(d.Email))
	password := d.Password
	displayName := strings.TrimSpace(d.DisplayName)
	if displayName == "" {
		displayName = strings.SplitN(email, "@", 2)[0]
	}
	if email == "" || !strings.Contains(email, "@") || len(password) < 8 {
		jsonError(w, http.StatusBadRequest, "valid email and password (8+ chars) required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "could not hash password")
		return
	}
	verificationToken := randomToken()
	ctx := context.Background()
	// The account (household) and its owner member are created together;
	// members.email is UNIQUE across households, so an email already used
	// by any member (e.g. someone's kid) is rejected too.
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback(ctx)
	var accountID, memberID int64
	err = tx.QueryRow(ctx,
		"INSERT INTO users (email, password_hash, display_name, is_verified, verification_token) VALUES ($1,$2,$3,FALSE,$4) RETURNING id",
		email, string(hash), displayName, verificationToken).Scan(&accountID)
	if err == nil {
		err = tx.QueryRow(ctx,
			"INSERT INTO members (account_id, email, password_hash, display_name, role) VALUES ($1,$2,$3,$4,'owner') RETURNING id",
			accountID, email, string(hash), displayName).Scan(&memberID)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		jsonError(w, http.StatusConflict, "email already registered")
		return
	}
	if err := seedDefaults(ctx, app, accountID); err != nil {
		jsonError(w, http.StatusInternalServerError, "seed failed")
		return
	}
	// ponytail: no SMTP wired yet, add real email send when SMTP creds exist
	// in env - for now the verify link just goes to the log, same as Python.
	log.Printf("verify-email link for %s: %s/verify-email?token=%s", email, app.Cfg.AppBaseURL, verificationToken)
	if err := setSessionMember(w, r, accountID, memberID, roleOwner, displayName); err != nil {
		jsonError(w, http.StatusInternalServerError, "session error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "display_name": displayName, "role": roleOwner})
}

func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var d struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	_ = decodeJSON(r, &d)
	email := d.Email
	if email == "" {
		email = d.Username
	}
	email = strings.ToLower(strings.TrimSpace(email))

	// Login authenticates a household member (owners included: every
	// account has an owner member carrying its original credentials).
	var memberID, accountID int64
	var passwordHash *string
	var displayName *string
	var role string
	var isDisabled bool
	ctx := context.Background()
	err := app.DB.QueryRow(ctx,
		"SELECT m.id, m.account_id, m.password_hash, m.display_name, m.role, u.is_disabled "+
			"FROM members m JOIN users u ON u.id = m.account_id WHERE m.email=$1", email).
		Scan(&memberID, &accountID, &passwordHash, &displayName, &role, &isDisabled)
	if err != nil || passwordHash == nil {
		jsonError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	ok, isLegacy := verifyPasswordHash(*passwordHash, d.Password)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	// Migrated accounts carry werkzeug (scrypt/pbkdf2) hashes, not bcrypt.
	// Transparently upgrade to bcrypt on first successful login so the
	// legacy verify path is only ever needed once per user.
	if isLegacy {
		if newHash, herr := bcrypt.GenerateFromPassword([]byte(d.Password), bcrypt.DefaultCost); herr == nil {
			_ = app.storeMemberHash(ctx, memberID, string(newHash))
		}
	}
	if isDisabled {
		jsonError(w, http.StatusForbidden, "account disabled")
		return
	}
	name := email
	if displayName != nil && *displayName != "" {
		name = *displayName
	}
	// ponytail: not blocking unverified accounts yet, matches Python.
	if err := setSessionMember(w, r, accountID, memberID, role, name); err != nil {
		jsonError(w, http.StatusInternalServerError, "session error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "display_name": name, "role": role})
}

func (app *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := clearSession(w, r); err != nil {
		jsonError(w, http.StatusInternalServerError, "session error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleMe(w http.ResponseWriter, r *http.Request) {
	m, ok := currentMember(r)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	resp := map[string]any{
		"authenticated": true,
		"account_id":    m.AccountID,
		"member_id":     m.ID,
		"role":          m.Role,
		"display_name":  getSession(r).Values["display"],
		"username":      nil,
		"currency":      "IDR",
		"locale":        "id-ID",
		"is_admin":      false,
	}
	var currency, locale string
	var isAdmin bool
	var household, ownerName *string
	err := app.DB.QueryRow(r.Context(),
		"SELECT currency, locale, is_admin, household_name, display_name FROM users WHERE id=$1", m.AccountID).
		Scan(&currency, &locale, &isAdmin, &household, &ownerName)
	if err == nil {
		resp["currency"] = currency
		resp["locale"] = locale
		resp["is_admin"] = isAdmin
		resp["household_name"] = householdName(household, ownerName)
	}
	var email string
	var display *string
	if err := app.DB.QueryRow(r.Context(), "SELECT email, display_name FROM members WHERE id=$1", m.ID).Scan(&email, &display); err == nil {
		resp["username"] = email
		resp["email"] = email
		if display != nil && *display != "" {
			resp["display_name"] = *display
		} else {
			resp["display_name"] = email
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleUpdateSettings: display_name is the acting member's own name (any
// role); currency is household-wide, so only owners/parents may change it.
func (app *App) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	m, ok := currentMember(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var d struct {
		DisplayName string `json:"display_name"`
		Currency    string `json:"currency"`
	}
	_ = decodeJSON(r, &d)
	ctx := r.Context()

	info, currencyOK := currencies[d.Currency]
	if currencyOK && !canManageFamily(m.Role) {
		jsonError(w, http.StatusForbidden, "only family admins and parents can change the household currency")
		return
	}
	name := strings.TrimSpace(d.DisplayName)
	if name != "" && m.ID != 0 {
		if _, err := app.DB.Exec(ctx, "UPDATE members SET display_name=$1 WHERE id=$2", name, m.ID); err != nil {
			jsonError(w, http.StatusInternalServerError, "db error")
			return
		}
	}

	sets := []string{}
	args := []any{}
	if name != "" && m.Role == roleOwner {
		args = append(args, name)
		sets = append(sets, "display_name=$"+strconv.Itoa(len(args)))
	}
	if currencyOK {
		args = append(args, d.Currency)
		sets = append(sets, "currency=$"+strconv.Itoa(len(args)))
		args = append(args, info.Locale)
		sets = append(sets, "locale=$"+strconv.Itoa(len(args)))
	}
	if len(sets) > 0 {
		args = append(args, m.AccountID)
		query := "UPDATE users SET " + strings.Join(sets, ",") + " WHERE id=$" + strconv.Itoa(len(args))
		if _, err := app.DB.Exec(ctx, query, args...); err != nil {
			jsonError(w, http.StatusInternalServerError, "db error")
			return
		}
	}
	if name != "" {
		s := getSession(r)
		s.Values["display"] = name
		_ = s.Save(r, w)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleCurrencies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currencies)
}

func (app *App) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		jsonError(w, http.StatusBadRequest, "token required")
		return
	}
	ctx := context.Background()
	var id int64
	err := app.DB.QueryRow(ctx, "SELECT id FROM users WHERE verification_token=$1", token).Scan(&id)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid or already-used token")
		return
	}
	if _, err := app.DB.Exec(ctx, "UPDATE users SET is_verified=TRUE, verification_token=NULL WHERE id=$1", id); err != nil {
		jsonError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handlePasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	var d struct {
		Email string `json:"email"`
	}
	_ = decodeJSON(r, &d)
	email := strings.ToLower(strings.TrimSpace(d.Email))

	ctx := context.Background()
	var userID, memberID int64
	err := app.DB.QueryRow(ctx, "SELECT account_id, id FROM members WHERE email=$1", email).Scan(&userID, &memberID)
	if err == nil {
		token := randomToken()
		expiresAt := time.Now().UTC().Add(time.Hour)
		if _, err := app.DB.Exec(ctx,
			"INSERT INTO password_resets (user_id, member_id, token, expires_at) VALUES ($1,$2,$3,$4)",
			userID, memberID, token, expiresAt); err == nil {
			// ponytail: no SMTP wired yet - reset link just goes to the log.
			log.Printf("password-reset link for %s: %s/reset-password?token=%s", email, app.Cfg.AppBaseURL, token)
		}
	}
	// Always 200 regardless of whether the email exists, to avoid leaking
	// which emails are registered.
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handlePasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	var d struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	_ = decodeJSON(r, &d)
	if d.Token == "" || len(d.NewPassword) < 8 {
		jsonError(w, http.StatusBadRequest, "token and password (8+ chars) required")
		return
	}
	ctx := context.Background()
	var id, userID int64
	var memberID *int64
	var expiresAt time.Time
	err := app.DB.QueryRow(ctx,
		"SELECT id, user_id, member_id, expires_at FROM password_resets WHERE token=$1 AND used=FALSE", d.Token).
		Scan(&id, &userID, &memberID, &expiresAt)
	if err != nil || time.Now().UTC().After(expiresAt) {
		jsonError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(d.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "could not hash password")
		return
	}
	// Pre-family tokens have no member_id: they reset the account owner.
	if memberID == nil {
		var ownerID int64
		if err := app.DB.QueryRow(ctx, "SELECT id FROM members WHERE account_id=$1 AND role='owner'", userID).Scan(&ownerID); err != nil {
			jsonError(w, http.StatusInternalServerError, "db error")
			return
		}
		memberID = &ownerID
	}
	if err := app.storeMemberHash(ctx, *memberID, string(hash)); err != nil {
		jsonError(w, http.StatusInternalServerError, "db error")
		return
	}
	if _, err := app.DB.Exec(ctx, "UPDATE password_resets SET used=TRUE WHERE id=$1", id); err != nil {
		jsonError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// small helpers

var errInvalidToken = &tokenError{"invalid token"}

type tokenError struct{ msg string }

func (e *tokenError) Error() string { return e.msg }

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
