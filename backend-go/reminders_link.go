package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Chat account linking: a member generates a one-time code in the app and
// sends "/link CODE" to the household bot; the bot calls POST /api/bot/link
// with X-Bot-User, which stores the chat user id on the member (used for
// botActor and @-mentions).

const linkCodeTTL = 15 * time.Minute
const linkAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/I

func newLinkCode() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, len(b))
	for i, v := range b {
		out[i] = linkAlphabet[int(v)%len(linkAlphabet)]
	}
	return string(out), nil
}

func hashLinkCode(code string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}

func (app *App) handleChatLinkStatus(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	var tg, dc *string
	_ = app.DB.QueryRow(r.Context(), "SELECT telegram_user_id, discord_user_id FROM members WHERE id=$1 AND account_id=$2", m.ID, m.AccountID).Scan(&tg, &dc)
	platforms := []string{}
	rows, err := app.DB.Query(r.Context(), "SELECT platform FROM integrations WHERE account_id=$1 AND enabled AND COALESCE(platform_token,'')<>'' ORDER BY id", m.AccountID)
	if err == nil {
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil {
				platforms = append(platforms, p)
			}
		}
		rows.Close()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"telegram_linked": tg != nil && *tg != "", "discord_linked": dc != nil && *dc != "",
		"bots": platforms, "can_link": m.ID != 0 && m.Role != roleViewer,
	})
}

func (app *App) handleCreateLinkCode(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	if m.ID == 0 {
		jsonError(w, http.StatusBadRequest, "please log in again first")
		return
	}
	ctx := r.Context()
	// one live code per member
	if _, err := app.DB.Exec(ctx, "DELETE FROM chat_link_codes WHERE member_id=$1 OR expires_at < now() - interval '1 day'", m.ID); err != nil {
		dbError(w, err)
		return
	}
	for attempt := 0; attempt < 5; attempt++ {
		code, err := newLinkCode()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "could not create code")
			return
		}
		exp := time.Now().UTC().Add(linkCodeTTL)
		_, err = app.DB.Exec(ctx, "INSERT INTO chat_link_codes (account_id, member_id, code_hash, expires_at) VALUES ($1,$2,$3,$4)",
			m.AccountID, m.ID, hashLinkCode(code), exp)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"code": code, "expires_at": exp, "command": "/link " + code})
			return
		}
		if !isUniqueViolation(err) {
			dbError(w, err)
			return
		}
	}
	jsonError(w, http.StatusInternalServerError, "could not create code")
}

func linkColumn(platform string) string {
	return map[string]string{"telegram": "telegram_user_id", "discord": "discord_user_id"}[platform]
}

func (app *App) handleUnlinkChat(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	col := linkColumn(chi.URLParam(r, "platform"))
	if col == "" {
		jsonError(w, http.StatusBadRequest, "platform must be telegram or discord")
		return
	}
	if _, err := app.DB.Exec(r.Context(), "UPDATE members SET "+col+"=NULL WHERE id=$1 AND account_id=$2", m.ID, m.AccountID); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// POST /api/bot/link {code} with X-Bot-User "<platform>:<user id>".
func botLink(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	platform, uid, ok := splitBotUser(r.Header.Get("X-Bot-User"))
	col := linkColumn(platform)
	if !ok || col == "" {
		jsonError(w, http.StatusBadRequest, "missing X-Bot-User")
		return
	}
	var d struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &d); err != nil || strings.TrimSpace(d.Code) == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "Usage: /link CODE (get the code in Kinkit › Home › Reminders)"})
		return
	}
	ctx := r.Context()
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var codeID, memberID int64
	err = tx.QueryRow(ctx, `UPDATE chat_link_codes SET used_at=now()
	  WHERE code_hash=$1 AND account_id=$2 AND used_at IS NULL AND expires_at > now() RETURNING id, member_id`,
		hashLinkCode(d.Code), aid).Scan(&codeID, &memberID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "That code is invalid or expired. Create a new one in Kinkit › Home › Reminders."})
		return
	}
	// a chat user maps to one member per household
	if _, err := tx.Exec(ctx, "UPDATE members SET "+col+"=NULL WHERE account_id=$1 AND "+col+"=$2", aid, uid); err != nil {
		dbError(w, err)
		return
	}
	var name string
	if err := tx.QueryRow(ctx, "UPDATE members SET "+col+"=$1 WHERE id=$2 AND account_id=$3 RETURNING COALESCE(NULLIF(display_name,''), split_part(email,'@',1))",
		uid, memberID, aid).Scan(&name); err != nil {
		dbError(w, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "member_id": memberID, "message": "🔗 Linked! Hi " + name + ", reminders for you will mention you here."})
}
