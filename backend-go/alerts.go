package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ---------- budget alerts via Telegram / Discord ----------
//
// The money worker (money.go) calls checkBudgetAlerts every few minutes for
// the current UTC month. When a category's spending reaches >=80% or >100%
// of its budget, one message is sent per (category, month, threshold) to
// every enabled integration with a platform token + chat id. Idempotency:
// a budget_alerts_sent row is claimed (INSERT ... ON CONFLICT DO NOTHING)
// before sending; if every send fails the claim is released so the next
// pass retries. Accounts opt out via users.alerts_enabled.

var alertHTTPClient = &http.Client{Timeout: 10 * time.Second}

// Overridable in tests.
var (
	telegramAPIBase = "https://api.telegram.org"
	discordAPIBase  = "https://discord.com/api/v10"
	// sendAlertMessage delivers one chat message. Tests swap it for a stub.
	sendAlertMessage = sendChatMessage
)

// sendChatMessage posts text to a Telegram chat or Discord channel using the
// integration's own bot token. The token never appears in returned errors.
func sendChatMessage(ctx context.Context, platform, token, chatID, text string) error {
	var req *http.Request
	var err error
	switch platform {
	case "telegram":
		body, _ := json.Marshal(map[string]any{"chat_id": chatID, "text": text})
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, telegramAPIBase+"/bot"+token+"/sendMessage", bytes.NewReader(body))
	case "discord":
		body, _ := json.Marshal(map[string]any{"content": text})
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, discordAPIBase+"/channels/"+url.PathEscape(chatID)+"/messages", bytes.NewReader(body))
		if req != nil {
			req.Header.Set("Authorization", "Bot "+token)
		}
	default:
		return fmt.Errorf("unsupported platform %q", platform)
	}
	if err != nil {
		return fmt.Errorf("%s: build request failed", platform)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := alertHTTPClient.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) { // strip the URL (Telegram puts the token in it)
			err = ue.Err
		}
		return fmt.Errorf("%s: %v", platform, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s: HTTP %d", platform, resp.StatusCode)
	}
	return nil
}

// alertThreshold: 100 when spent is over budget, 80 when at/above 80%, else 0.
func alertThreshold(spent, budget int64) int {
	if budget <= 0 {
		return 0
	}
	if spent > budget {
		return 100
	}
	if spent*100 >= budget*80 {
		return 80
	}
	return 0
}

// formatAlertAmount renders an IDR amount in the account's display currency
// using the cached FX table (no network); falls back to IDR.
func formatAlertAmount(idr int64, currency string, rates map[string]float64) string {
	if currency != "" && currency != "IDR" {
		if rate, ok := rates[currency]; ok && rate > 0 {
			return currency + " " + groupThousands(strconv.FormatFloat(float64(idr)*rate, 'f', 2, 64), ",", ".")
		}
	}
	return "Rp" + groupThousands(strconv.FormatInt(idr, 10), ".", ",")
}

// groupThousands inserts sep every 3 digits of the integer part of a
// plain decimal string ("1234567.50") and swaps "." for decSep.
func groupThousands(s, sep, decSep string) string {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	intPart, frac, hasFrac := strings.Cut(s, ".")
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(c)
	}
	out := b.String()
	if hasFrac {
		out += decSep + frac
	}
	if neg {
		out = "-" + out
	}
	return out
}

func alertText(category string, threshold int, spent, budget int64, monthLabel, currency string, rates map[string]float64) string {
	pct := 0
	if budget > 0 {
		pct = int(spent * 100 / budget)
	}
	s, b := formatAlertAmount(spent, currency, rates), formatAlertAmount(budget, currency, rates)
	if threshold >= 100 {
		return fmt.Sprintf("Kinkit budget alert: %s is OVER budget for %s - spent %s of %s (%d%%).", category, monthLabel, s, b, pct)
	}
	return fmt.Sprintf("Kinkit budget alert: %s has used %d%% of its %s budget - spent %s of %s.", category, pct, monthLabel, s, b)
}

type alertChannel struct{ platform, token, chatID string }

// checkBudgetAlerts runs one alert pass for the UTC month containing now.
// Errors for one account are logged and do not stop the others.
func (app *App) checkBudgetAlerts(ctx context.Context, now time.Time) error {
	rows, err := app.DB.Query(ctx, `SELECT u.id, COALESCE(u.currency,'IDR') FROM users u
	  WHERE u.alerts_enabled AND NOT u.is_disabled AND EXISTS (
	    SELECT 1 FROM integrations i WHERE i.account_id=u.id AND i.enabled
	      AND COALESCE(i.chat_id,'') <> '' AND COALESCE(i.platform_token,'') <> '')`)
	if err != nil {
		return err
	}
	type acct struct {
		id       int64
		currency string
	}
	var accts []acct
	for rows.Next() {
		var a acct
		if err := rows.Scan(&a.id, &a.currency); err != nil {
			rows.Close()
			return err
		}
		accts = append(accts, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	var rates map[string]float64
	if c := loadFxCacheFile(fxCachePath()); c != nil {
		rates = c.Rates
	}
	for _, a := range accts {
		if _, err := app.checkAccountAlerts(ctx, a.id, a.currency, rates, now); err != nil {
			log.Printf("budget alerts: account %d: %v", a.id, err)
		}
	}
	return nil
}

// checkAccountAlerts evaluates one account and returns the number of alerts
// delivered (to at least one channel).
func (app *App) checkAccountAlerts(ctx context.Context, aid int64, currency string, rates map[string]float64, now time.Time) (int, error) {
	now = now.UTC()
	year, month := now.Year(), int(now.Month())
	from, to := monthRange(year, month)

	chRows, err := app.DB.Query(ctx, `SELECT platform, platform_token, chat_id FROM integrations
	  WHERE account_id=$1 AND enabled AND COALESCE(chat_id,'') <> '' AND COALESCE(platform_token,'') <> '' ORDER BY id`, aid)
	if err != nil {
		return 0, err
	}
	var channels []alertChannel
	for chRows.Next() {
		var c alertChannel
		if err := chRows.Scan(&c.platform, &c.token, &c.chatID); err != nil {
			chRows.Close()
			return 0, err
		}
		channels = append(channels, c)
	}
	chRows.Close()
	if len(channels) == 0 {
		return 0, nil
	}

	type catState struct {
		id            int64
		name          string
		budget, spent int64
	}
	rows, err := app.DB.Query(ctx, `SELECT c.id, c.name, be.amount,
	    COALESCE((SELECT SUM(sp.amount) FROM spending sp WHERE sp.account_id=$1 AND sp.category_id=c.id
	              AND sp.tx_date >= $2 AND sp.tx_date < $3),0)
	  FROM budget_expense be JOIN categories c ON c.id=be.category_id
	  WHERE be.account_id=$1 AND be.year=$4 AND be.month=$5 AND be.amount > 0 ORDER BY c.id`, aid, from, to, year, month)
	if err != nil {
		return 0, err
	}
	var cats []catState
	for rows.Next() {
		var c catState
		if err := rows.Scan(&c.id, &c.name, &c.budget, &c.spent); err != nil {
			rows.Close()
			return 0, err
		}
		cats = append(cats, c)
	}
	rows.Close()

	claim := func(catID int64, th int, spent, budget int64) (bool, error) {
		tag, err := app.DB.Exec(ctx, `INSERT INTO budget_alerts_sent (account_id, category_id, year, month, threshold, spent, budget)
		  VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (account_id, category_id, year, month, threshold) DO NOTHING`,
			aid, catID, year, month, th, spent, budget)
		return err == nil && tag.RowsAffected() == 1, err
	}

	monthLabel := now.Format("January 2006")
	sent := 0
	for _, c := range cats {
		th := alertThreshold(c.spent, c.budget)
		if th == 0 {
			continue
		}
		if th == 100 {
			// Jumping straight past 100% sends only the "over" message.
			if _, err := claim(c.id, 80, c.spent, c.budget); err != nil {
				return sent, err
			}
		}
		claimed, err := claim(c.id, th, c.spent, c.budget)
		if err != nil {
			return sent, err
		}
		if !claimed {
			continue
		}
		text := alertText(c.name, th, c.spent, c.budget, monthLabel, currency, rates)
		delivered := 0
		for _, ch := range channels {
			sctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			err := sendAlertMessage(sctx, ch.platform, ch.token, ch.chatID, text)
			cancel()
			if err != nil {
				log.Printf("budget alerts: account %d category %d: %v", aid, c.id, err)
				continue
			}
			delivered++
		}
		if delivered == 0 { // release the claim so the next pass retries
			_, _ = app.DB.Exec(ctx, "DELETE FROM budget_alerts_sent WHERE account_id=$1 AND category_id=$2 AND year=$3 AND month=$4 AND threshold=$5",
				aid, c.id, year, month, th)
			continue
		}
		sent++
	}
	return sent, nil
}

// GET /alerts/settings
func (app *App) handleGetAlertSettings(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var enabled bool
	if err := app.DB.QueryRow(ctx, "SELECT alerts_enabled FROM users WHERE id=$1", aid).Scan(&enabled); err != nil {
		dbError(w, err)
		return
	}
	type channel struct {
		Platform string `json:"platform"`
		Enabled  bool   `json:"enabled"`
		Ready    bool   `json:"ready"` // enabled + token + chat id: alerts will be delivered here
	}
	channels := []channel{}
	rows, err := app.DB.Query(ctx, `SELECT platform, enabled, COALESCE(chat_id,'') <> '' AND COALESCE(platform_token,'') <> ''
	  FROM integrations WHERE account_id=$1 ORDER BY id`, aid)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var c channel
		var configured bool
		if err := rows.Scan(&c.Platform, &c.Enabled, &configured); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		c.Ready = c.Enabled && configured
		channels = append(channels, c)
	}
	rows.Close()

	type recent struct {
		CategoryName string    `json:"category_name"`
		Year         int       `json:"year"`
		Month        int       `json:"month"`
		Threshold    int       `json:"threshold"`
		Spent        int64     `json:"spent"`
		Budget       int64     `json:"budget"`
		SentAt       time.Time `json:"sent_at"`
	}
	recentList := []recent{}
	rows, err = app.DB.Query(ctx, `SELECT c.name, b.year, b.month, b.threshold, b.spent, b.budget, b.sent_at
	  FROM budget_alerts_sent b JOIN categories c ON c.id=b.category_id
	  WHERE b.account_id=$1 ORDER BY b.sent_at DESC, b.id DESC LIMIT 10`, aid)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var x recent
		if err := rows.Scan(&x.CategoryName, &x.Year, &x.Month, &x.Threshold, &x.Spent, &x.Budget, &x.SentAt); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		recentList = append(recentList, x)
	}
	rows.Close()
	writeJSON(w, http.StatusOK, map[string]any{"alerts_enabled": enabled, "channels": channels, "recent": recentList})
}

// PATCH /alerts/settings {alerts_enabled}
func (app *App) handleUpdateAlertSettings(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var d struct {
		AlertsEnabled *bool `json:"alerts_enabled"`
	}
	if err := decodeJSON(r, &d); err != nil || d.AlertsEnabled == nil {
		jsonError(w, http.StatusBadRequest, "alerts_enabled (boolean) is required")
		return
	}
	if _, err := app.DB.Exec(r.Context(), "UPDATE users SET alerts_enabled=$1 WHERE id=$2", *d.AlertsEnabled, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "alerts_enabled": *d.AlertsEnabled})
}
