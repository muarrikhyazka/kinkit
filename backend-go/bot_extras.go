package main

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Chat bot extras merged from rumahbot:
//   - learned spending keywords: when someone picks a category for an
//     unrecognised expense in chat, every word (3+ letters) of its
//     description that isn't a keyword yet is mapped to that category, so
//     the next "bakso 15rb" is categorised automatically
//     (POST /bot/keywords/learn). Managed on the web at Settings > Bots
//     (GET/POST /bot-keywords, DELETE /bot-keywords/{id}).
//   - feature requests from chat (/feedback, /fitur -> POST /bot/feedback).

func registerBotExtraRoutes(r chi.Router, app *App) {
	r.Post("/bot/keywords/learn", botAuth(app, botLearnKeywords))
	r.Post("/bot/feedback", botAuth(app, botFeedback))
	r.Get("/bot-keywords", requireLogin(app, handleListKeywords))
	r.Post("/bot-keywords", requireLogin(app, handleUpsertKeyword))
	r.Delete("/bot-keywords/{id}", requireLogin(app, handleDeleteKeyword))
}

var keywordWordRe = regexp.MustCompile(`[a-z]+`)

// keywordCandidates: distinct lower-case words of 3+ letters (pure).
func keywordCandidates(desc string) []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range keywordWordRe.FindAllString(strings.ToLower(desc), -1) {
		if len(w) >= 3 && !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

// canonicalCategory returns the account's category name matching name
// case-insensitively, plus its id.
func (app *App) canonicalCategory(ctx context.Context, aid int64, name string) (int64, string, error) {
	var id int64
	var canon string
	err := app.DB.QueryRow(ctx, "SELECT id, name FROM categories WHERE account_id=$1 AND lower(name)=lower($2) ORDER BY id LIMIT 1",
		aid, strings.TrimSpace(name)).Scan(&id, &canon)
	return id, canon, err
}

// learnKeywords maps unseen words of desc to category; returns the new keywords.
func (app *App) learnKeywords(ctx context.Context, aid int64, desc, category string) ([]string, error) {
	learned := []string{}
	for _, w := range keywordCandidates(desc) {
		tag, err := app.DB.Exec(ctx, "INSERT INTO category_keywords (account_id, keyword, category) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING", aid, w, category)
		if err != nil {
			return learned, err
		}
		if tag.RowsAffected() > 0 {
			learned = append(learned, w)
		}
	}
	return learned, nil
}

// POST /api/bot/keywords/learn {description, category} -> {ok, learned}
func botLearnKeywords(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	var d struct {
		Description string `json:"description"`
		Category    string `json:"category"`
	}
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx := r.Context()
	_, canon, err := app.canonicalCategory(ctx, aid, d.Category)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "unknown category")
		return
	}
	learned, err := app.learnKeywords(ctx, aid, d.Description, canon)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "learned": learned, "category": canon})
}

// POST /api/bot/feedback {text} -> {ok, message}
func botFeedback(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	var d struct {
		Text string `json:"text"`
	}
	_ = decodeJSON(r, &d)
	text := strings.TrimSpace(d.Text)
	if text == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "Usage: /feedback <idea>, e.g. /feedback weekly spending report"})
		return
	}
	if len(text) > 2000 {
		text = text[:2000]
	}
	ctx := r.Context()
	var memberID *int64
	if m, ok := botActor(ctx, app, r, aid); ok {
		memberID = &m.ID
	}
	if _, err := app.DB.Exec(ctx, "INSERT INTO bot_feedback (account_id, member_id, chat_user, text) VALUES ($1,$2,$3,$4)",
		aid, memberID, r.Header.Get("X-Bot-User"), text); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Thanks, feature request saved: " + text})
}

// ---------- session: keyword management ----------

func handleListKeywords(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	rows, err := app.DB.Query(r.Context(), "SELECT id, keyword, category FROM category_keywords WHERE account_id=$1 ORDER BY category, keyword", aid)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var k, c string
		if err := rows.Scan(&id, &k, &c); err != nil {
			dbError(w, err)
			return
		}
		out = append(out, map[string]any{"id": id, "keyword": k, "category": c})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// POST /api/bot-keywords {keyword, category}: add or re-map a keyword.
func handleUpsertKeyword(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	var d struct {
		Keyword  string `json:"keyword"`
		Category string `json:"category"`
	}
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	kw := strings.Join(strings.Fields(strings.ToLower(d.Keyword)), " ")
	if kw == "" || len(kw) > 60 {
		jsonError(w, http.StatusBadRequest, "keyword is required (max 60 characters)")
		return
	}
	ctx := r.Context()
	_, canon, err := app.canonicalCategory(ctx, aid, d.Category)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, http.StatusBadRequest, "unknown category")
		return
	} else if err != nil {
		dbError(w, err)
		return
	}
	var id int64
	if err := app.DB.QueryRow(ctx, `INSERT INTO category_keywords (account_id, keyword, category) VALUES ($1,$2,$3)
	  ON CONFLICT (account_id, keyword) DO UPDATE SET category=EXCLUDED.category RETURNING id`, aid, kw, canon).Scan(&id); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "keyword": kw, "category": canon})
}

func handleDeleteKeyword(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	id, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	tag, err := app.DB.Exec(r.Context(), "DELETE FROM category_keywords WHERE id=$1 AND account_id=$2", id, aid)
	if err != nil {
		dbError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
