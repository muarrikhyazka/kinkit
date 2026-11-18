package main

import (
	"net/http"
	"strings"
	"time"
)

// Household time zone (users.timezone, IANA name). GET: any member;
// PATCH: owner/parent (non-GET requests from kids/viewers are already
// rejected by memberMiddleware).

func validTimezone(tz string) (*time.Location, bool) {
	tz = strings.TrimSpace(tz)
	if tz == "" || tz == "Local" || len(tz) > 64 {
		return nil, false
	}
	loc, err := time.LoadLocation(tz)
	return loc, err == nil
}

func (app *App) handleGetTimezone(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	var tz string
	if err := app.DB.QueryRow(r.Context(), "SELECT timezone FROM users WHERE id=$1", m.AccountID).Scan(&tz); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"timezone": tz, "is_default": tz == "UTC", "can_edit": canManageFamily(m.Role)})
}

func (app *App) handleSetTimezone(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	if !canManageFamily(m.Role) {
		jsonError(w, http.StatusForbidden, "only family admins and parents can change the household time zone")
		return
	}
	var d struct {
		Timezone string `json:"timezone"`
	}
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	loc, ok := validTimezone(d.Timezone)
	if !ok {
		jsonError(w, http.StatusBadRequest, "unknown time zone (use an IANA name like Asia/Jakarta)")
		return
	}
	ctx := r.Context()
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var old string
	if err := tx.QueryRow(ctx, "SELECT timezone FROM users WHERE id=$1 FOR UPDATE", m.AccountID).Scan(&old); err != nil {
		dbError(w, err)
		return
	}
	if _, err := tx.Exec(ctx, "UPDATE users SET timezone=$1 WHERE id=$2", loc.String(), m.AccountID); err != nil {
		dbError(w, err)
		return
	}
	// "08:00" now means a different UTC instant: regenerate future occurrences.
	if old != loc.String() {
		if err := resetFutureOccurrences(ctx, tx, "account_id=$1", m.AccountID); err != nil {
			dbError(w, err)
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		dbError(w, err)
		return
	}
	if _, err := app.materialize(ctx, m.AccountID, time.Now().UTC(), materializeHorizon); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "timezone": loc.String()})
}

// ---------- digest settings ----------

type digestSettings struct {
	Enabled   bool   `json:"enabled"`
	TimeOfDay string `json:"time_of_day"`
	Channel   string `json:"channel"`
}

func (app *App) loadDigestSettings(r *http.Request, aid int64) digestSettings {
	s := digestSettings{Enabled: true, TimeOfDay: "07:00", Channel: "all"}
	_ = app.DB.QueryRow(r.Context(), "SELECT enabled, time_of_day, channel FROM digest_settings WHERE account_id=$1", aid).
		Scan(&s.Enabled, &s.TimeOfDay, &s.Channel)
	return s
}

func (app *App) handleGetDigest(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, app.loadDigestSettings(r, m.AccountID))
}

func (app *App) handleSetDigest(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	if !canManageFamily(m.Role) {
		jsonError(w, http.StatusForbidden, "only family admins and parents can change the digest")
		return
	}
	s := app.loadDigestSettings(r, m.AccountID)
	if err := decodeJSON(r, &s); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	hh, mm, err := parseHHMM(s.TimeOfDay)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.TimeOfDay = fmtHHMM(hh, mm)
	if s.Channel != "all" && s.Channel != "telegram" && s.Channel != "discord" {
		jsonError(w, http.StatusBadRequest, "channel must be all, telegram or discord")
		return
	}
	if _, err := app.DB.Exec(r.Context(), `INSERT INTO digest_settings (account_id, enabled, time_of_day, channel) VALUES ($1,$2,$3,$4)
	  ON CONFLICT (account_id) DO UPDATE SET enabled=excluded.enabled, time_of_day=excluded.time_of_day, channel=excluded.channel`,
		m.AccountID, s.Enabled, s.TimeOfDay, s.Channel); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "settings": s})
}
