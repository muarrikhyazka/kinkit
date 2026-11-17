package main

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterInternalRoutes wires service-token-authenticated endpoints for the
// bot multiplexer processes (/internal/integrations,
// /internal/integrations-version).
func RegisterInternalRoutes(r chi.Router, app *App) {
	r.Get("/internal/integrations", serviceAuth(app, internalListIntegrations))
	r.Get("/internal/integrations-version", serviceAuth(app, internalIntegrationsVersion))
}

// serviceAuth checks X-Service-Token against app.Cfg.ServiceToken.
func serviceAuth(app *App, h func(http.ResponseWriter, *http.Request, *App)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.Cfg.ServiceToken == "" || r.Header.Get("X-Service-Token") != app.Cfg.ServiceToken {
			jsonError(w, http.StatusForbidden, "forbidden")
			return
		}
		h(w, r, app)
	}
}

// ---------- Service-token authenticated internal (bot multiplexers) ----------

// internalListIntegrations mirrors Python's /api/internal/integrations
// response shape exactly: a bare JSON array of
// {account_id, platform, bot_token, platform_token, chat_id} - this is the
// contract bots/telegram_bot.py and bots/discord_bot.py parse directly.
func internalListIntegrations(w http.ResponseWriter, r *http.Request, app *App) {
	q := "SELECT account_id, platform, bot_token, platform_token, chat_id FROM integrations " +
		"WHERE enabled=TRUE AND platform_token IS NOT NULL AND platform_token != ''"
	var args []any
	if platform := r.URL.Query().Get("platform"); platform != "" {
		q += " AND platform=$1"
		args = append(args, platform)
	}
	rows, err := app.DB.Query(context.Background(), q, args...)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var accountID int64
		var platform, botToken string
		var platformToken, chatID *string
		if err := rows.Scan(&accountID, &platform, &botToken, &platformToken, &chatID); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, map[string]any{
			"account_id": accountID, "platform": platform, "bot_token": botToken,
			"platform_token": platformToken, "chat_id": chatID,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// internalIntegrationsVersion mirrors Python's /api/internal/integrations-version
// response shape exactly: {"version": "<unix ts string or empty>"}.
func internalIntegrationsVersion(w http.ResponseWriter, r *http.Request, app *App) {
	var version string
	err := app.DB.QueryRow(context.Background(), "SELECT value FROM meta WHERE key='integrations_version'").Scan(&version)
	if err != nil {
		version = ""
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": version})
}
