package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/sessions"
)

// Sessions mirrors Flask's cookie-session: account_id stored in the
// cookie, HttpOnly, same lifetime semantics. Replace with a shared
// store (Redis) only if this ever runs >1 instance behind a load balancer.
var store *sessions.CookieStore

func initSessionStore(secret string, secure bool) {
	store = sessions.NewCookieStore([]byte(secret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func getSession(r *http.Request) *sessions.Session {
	s, _ := store.Get(r, "accountamily")
	return s
}

func currentAccountID(r *http.Request) (int64, bool) {
	s := getSession(r)
	v, ok := s.Values["account_id"]
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}

// setSessionMember logs a household member in: the session carries the
// household (account_id) plus who is acting (member_id) and their role.
// The role in the cookie is only a hint - memberMiddleware re-reads it from
// the DB on every request so role changes / removals apply immediately.
func setSessionMember(w http.ResponseWriter, r *http.Request, accountID, memberID int64, role, displayName string) error {
	s := getSession(r)
	s.Values["account_id"] = accountID
	s.Values["member_id"] = memberID
	s.Values["role"] = role
	s.Values["display"] = displayName
	return s.Save(r, w)
}

func sessionMemberID(r *http.Request) (int64, bool) {
	v, ok := getSession(r).Values["member_id"]
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}

// memberInfo is the acting member, resolved once per request by
// memberMiddleware and stored in the request context.
type memberInfo struct {
	ID        int64
	AccountID int64
	Role      string
}

type memberCtxKey struct{}

func withMember(r *http.Request, m memberInfo) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), memberCtxKey{}, m))
}

// currentMember returns the member resolved by memberMiddleware.
func currentMember(r *http.Request) (memberInfo, bool) {
	m, ok := r.Context().Value(memberCtxKey{}).(memberInfo)
	return m, ok
}

// currentMemberIDPtr is the acting member's id for "who did this" columns
// (spending.member_id); nil when unknown.
func currentMemberIDPtr(r *http.Request) *int64 {
	if m, ok := currentMember(r); ok && m.ID != 0 {
		id := m.ID
		return &id
	}
	return nil
}

func clearSession(w http.ResponseWriter, r *http.Request) error {
	s := getSession(r)
	s.Values = map[interface{}]interface{}{}
	s.Options.MaxAge = -1
	return s.Save(r, w)
}

// JSON helpers

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
