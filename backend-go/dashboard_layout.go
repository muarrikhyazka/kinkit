package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// custom dashboard layout (synced across devices)
// One account-wide layout (member_id NULL). The layout is opaque JSON owned
// by the frontend (CustomDashboard.jsx), capped in size.

const maxLayoutBytes = 64 << 10

// GET /api/dashboard-layout -> {"layout": <json|null>, "updated_at": "..."|null}
func (app *App) handleGetDashboardLayout(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var layout json.RawMessage
	var updated time.Time
	err := app.DB.QueryRow(r.Context(),
		"SELECT layout, updated_at FROM dashboard_layouts WHERE account_id=$1 AND member_id IS NULL", aid).Scan(&layout, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]any{"layout": nil, "updated_at": nil})
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"layout": layout, "updated_at": updated.UTC().Format(time.RFC3339)})
}

// PUT /api/dashboard-layout {"layout": <json array|object>}
func (app *App) handlePutDashboardLayout(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxLayoutBytes)
	var body struct {
		Layout json.RawMessage `json:"layout"`
	}
	if err := decodeJSON(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON or layout too large")
		return
	}
	if len(body.Layout) == 0 || (body.Layout[0] != '[' && body.Layout[0] != '{') {
		jsonError(w, http.StatusBadRequest, "layout must be a JSON array or object")
		return
	}
	var updated time.Time
	err := app.DB.QueryRow(r.Context(),
		`INSERT INTO dashboard_layouts (account_id, member_id, layout, updated_at) VALUES ($1, NULL, $2, now())
		 ON CONFLICT (account_id, (COALESCE(member_id, 0))) DO UPDATE SET layout=EXCLUDED.layout, updated_at=now()
		 RETURNING updated_at`, aid, string(body.Layout)).Scan(&updated)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated_at": updated.UTC().Format(time.RFC3339)})
}
