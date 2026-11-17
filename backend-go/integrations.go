package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// RegisterIntegrationRoutes wires per-account integrations CRUD (login required).
func RegisterIntegrationRoutes(r chi.Router, app *App) {
	r.Get("/integrations", requireLogin(app, listIntegrations))
	r.Post("/integrations", requireLogin(app, addIntegration))
	r.Patch("/integrations/{id}", requireLogin(app, updateIntegration))
	r.Delete("/integrations/{id}", requireLogin(app, deleteIntegration))
}

// bumpIntegrationsVersion signals bot multiplexer processes that
// integrations changed; ported from Python's bump_integrations_version().
func bumpIntegrationsVersion(ctx context.Context, app *App) error {
	_, err := app.DB.Exec(ctx,
		"INSERT INTO meta (key, value) VALUES ('integrations_version', $1) "+
			"ON CONFLICT (key) DO UPDATE SET value=excluded.value",
		strconv.FormatInt(time.Now().UTC().Unix(), 10))
	return err
}

// ---------- Integrations CRUD ----------

// integrationRow is the user-facing shape of an integration. The upstream
// platform token (Telegram/Discord bot secret) is never echoed back: only a
// masked hint and a presence flag. The raw token is only exposed to the bot
// processes via the service-token /internal/integrations endpoint.
type integrationRow struct {
	ID                  int64     `json:"id"`
	Platform            string    `json:"platform"`
	ChatID              *string   `json:"chat_id"`
	Enabled             bool      `json:"enabled"`
	BotToken            string    `json:"bot_token"`
	HasPlatformToken    bool      `json:"has_platform_token"`
	PlatformTokenMasked *string   `json:"platform_token_masked"`
	CreatedAt           time.Time `json:"created_at"`
}

// maskSecret renders a secret as "••••" + its last 4 characters (or just
// "••••" if it is too short to reveal anything safely).
func maskSecret(s string) string {
	r := []rune(s)
	if len(r) <= 8 {
		return "••••"
	}
	return "••••" + string(r[len(r)-4:])
}

func listIntegrations(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	rows, err := app.DB.Query(context.Background(),
		"SELECT id, platform, chat_id, enabled, bot_token, platform_token, created_at FROM integrations WHERE account_id=$1 ORDER BY id", aid)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []integrationRow{}
	for rows.Next() {
		var ir integrationRow
		var platformToken *string
		if err := rows.Scan(&ir.ID, &ir.Platform, &ir.ChatID, &ir.Enabled, &ir.BotToken, &platformToken, &ir.CreatedAt); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if platformToken != nil && *platformToken != "" {
			ir.HasPlatformToken = true
			m := maskSecret(*platformToken)
			ir.PlatformTokenMasked = &m
		}
		out = append(out, ir)
	}
	writeJSON(w, http.StatusOK, out)
}

func genBotToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func addIntegration(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	var d struct {
		Platform      string  `json:"platform"`
		PlatformToken *string `json:"platform_token"`
		ChatID        *string `json:"chat_id"`
	}
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if d.Platform != "telegram" && d.Platform != "discord" {
		jsonError(w, http.StatusBadRequest, "platform must be telegram or discord")
		return
	}
	botToken, err := genBotToken()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var newID int64
	err = app.DB.QueryRow(context.Background(),
		"INSERT INTO integrations (account_id, platform, bot_token, platform_token, chat_id) VALUES ($1,$2,$3,$4,$5) RETURNING id",
		aid, d.Platform, botToken, d.PlatformToken, d.ChatID).Scan(&newID)
	if err != nil {
		// unique violation (account_id,platform) or bot_token collision
		jsonError(w, http.StatusConflict, "integration for this platform already exists")
		return
	}
	if err := bumpIntegrationsVersion(context.Background(), app); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": newID, "bot_token": botToken})
}

func updateIntegration(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	iid, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var d map[string]any
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	var sets []string
	var params []any
	n := 1
	// platform_token is write-only: only replaced when a non-empty string is
	// sent, so the UI can PATCH other fields without knowing the secret.
	if v, ok := d["platform_token"].(string); ok && strings.TrimSpace(v) != "" {
		sets = append(sets, "platform_token=$"+strconv.Itoa(n))
		params = append(params, strings.TrimSpace(v))
		n++
	}
	if v, present := d["chat_id"]; present {
		sets = append(sets, "chat_id=$"+strconv.Itoa(n))
		params = append(params, v)
		n++
	}
	if v, present := d["enabled"]; present {
		sets = append(sets, "enabled=$"+strconv.Itoa(n))
		params = append(params, v)
		n++
	}
	if len(sets) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	params = append(params, iid, aid)
	q := "UPDATE integrations SET " + strings.Join(sets, ",") +
		" WHERE id=$" + strconv.Itoa(n) + " AND account_id=$" + strconv.Itoa(n+1)
	if _, err := app.DB.Exec(context.Background(), q, params...); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := bumpIntegrationsVersion(context.Background(), app); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func deleteIntegration(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	iid, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := app.DB.Exec(context.Background(), "DELETE FROM integrations WHERE id=$1 AND account_id=$2", iid, aid); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := bumpIntegrationsVersion(context.Background(), app); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
