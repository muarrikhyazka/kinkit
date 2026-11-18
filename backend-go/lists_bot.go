package main

// Bot endpoints for shopping lists (X-Bot-Token, see botAuth). The chat
// user is resolved with botActor (X-Bot-User); unlinked chat users act as
// "no member". Contract used by the Python bots:
//
//	POST /api/bot/lists/items  {text, list?}          -> {ok, list, added:[names]}
//	GET  /api/bot/lists?list=                         -> {list, items:[{id,name,qty}]}
//	POST /api/bot/lists/finish {total_amount?, list?} -> {ok, message}

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var errListNotFound = errors.New("list not found")

// resolveBotList finds a list by name (case-insensitive exact, then prefix);
// empty name -> the first (default) list, creating "Groceries" if needed.
func (app *App) resolveBotList(ctx context.Context, aid int64, name string) (shoppingList, error) {
	if err := app.ensureDefaultList(ctx, aid); err != nil {
		return shoppingList{}, err
	}
	name = strings.TrimSpace(name)
	q := "SELECT " + listCols + " FROM shopping_lists l WHERE l.account_id=$1 AND NOT l.archived "
	var row pgx.Row
	if name == "" {
		row = app.DB.QueryRow(ctx, q+"ORDER BY l.sort_order, l.id LIMIT 1", aid)
	} else {
		row = app.DB.QueryRow(ctx, q+"AND (lower(l.name)=lower($2) OR lower(l.name) LIKE lower($3)) "+
			"ORDER BY (lower(l.name)=lower($2)) DESC, l.sort_order, l.id LIMIT 1", aid, name, listEscapeLike(name)+"%")
	}
	l, err := scanShoppingList(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return l, errListNotFound
	}
	return l, err
}

func listEscapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(s)
}

func (app *App) botListNames(ctx context.Context, aid int64) string {
	lists, _ := app.loadLists(ctx, aid, false)
	names := make([]string, 0, len(lists))
	for _, l := range lists {
		names = append(names, l.Name)
	}
	return strings.Join(names, ", ")
}

func botListNotFound(w http.ResponseWriter, app *App, ctx context.Context, aid int64, name string) {
	writeJSON(w, http.StatusNotFound, map[string]any{
		"ok": false, "error": "list not found",
		"message": fmt.Sprintf("No list called %q. Your lists: %s", name, app.botListNames(ctx, aid)),
	})
}

func listBotMember(app *App, r *http.Request, aid int64) (*int64, memberInfo, bool) {
	m, ok := botActor(r.Context(), app, r, aid)
	if !ok {
		return nil, m, false
	}
	id := m.ID
	return &id, m, true
}

func botListAddItems(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	var d struct {
		Text string `json:"text"`
		List string `json:"list"`
	}
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx := r.Context()
	mid, m, linked := listBotMember(app, r, aid)
	if linked && m.Role == roleViewer {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "forbidden", "message": "Viewers can't change shopping lists."})
		return
	}
	parsed := parseItemsText(d.Text)
	if len(parsed) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "nothing to add", "message": "Tell me what to add, e.g. \"eggs, 2 milk, bread x3\"."})
		return
	}
	l, err := app.resolveBotList(ctx, aid, d.List)
	if errors.Is(err, errListNotFound) {
		botListNotFound(w, app, ctx, aid, d.List)
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	items := make([]newListItem, 0, len(parsed))
	for _, p := range parsed {
		items = append(items, newListItem{parsedItem: p})
	}
	added, err := app.addListItems(ctx, aid, l.ID, items, mid)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "list": l.Name, "added": listItemNames(added)})
}

func botListItems(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	ctx := r.Context()
	name := r.URL.Query().Get("list")
	_, _ = app.addDueStaples(ctx, aid, time.Now())
	l, err := app.resolveBotList(ctx, aid, name)
	if errors.Is(err, errListNotFound) {
		botListNotFound(w, app, ctx, aid, name)
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	items, err := app.loadItems(ctx, aid, l.ID, "NOT i.checked")
	if err != nil {
		dbError(w, err)
		return
	}
	type botItem struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Qty  string `json:"qty"`
	}
	out := make([]botItem, 0, len(items))
	for _, it := range items {
		out = append(out, botItem{it.ID, it.Name, it.Qty})
	}
	writeJSON(w, http.StatusOK, map[string]any{"list": l.Name, "items": out})
}

func botListFinish(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	var d struct {
		TotalAmount json.RawMessage `json:"total_amount"`
		List        string          `json:"list"`
		ClientID    string          `json:"client_id"`
	}
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx := r.Context()
	mid, m, linked := listBotMember(app, r, aid)
	if linked && m.Role != roleOwner && m.Role != roleParent {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "Only parents can finish shopping. Ask a parent to log the total."})
		return
	}
	l, err := app.resolveBotList(ctx, aid, d.List)
	if errors.Is(err, errListNotFound) {
		botListNotFound(w, app, ctx, aid, d.List)
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	// all checked items, or everything if nothing was ticked off
	var items []int64
	where := "i.checked"
	if l.CheckedCount == 0 {
		where = "NOT i.checked"
	}
	rows, err := app.loadItems(ctx, aid, l.ID, where)
	if err != nil {
		dbError(w, err)
		return
	}
	items = make([]int64, 0, len(rows))
	for _, it := range rows {
		items = append(items, it.ID)
	}

	total, ok := listBotAmount(d.TotalAmount)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "needs_total": true, "list": l.Name, "items": len(items),
			"message": fmt.Sprintf("How much was the %s shopping (%d item%s)? Send the total amount, e.g. 250000 or 250k.",
				l.Name, len(items), map[bool]string{true: "", false: "s"}[len(items) == 1]),
		})
		return
	}
	res, err := app.finishShopping(ctx, aid, l, finishInput{ClientID: d.ClientID, TotalAmount: total, Items: items}, mid)
	if err != nil {
		if errors.Is(err, errListNoCategory) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "Add a spending category in Kinkit first."})
			return
		}
		dbError(w, err)
		return
	}
	msg := fmt.Sprintf("Logged %s shopping: %s in %s, %d item%s cleared.", l.Name, listFormatIDR(total), res.Category,
		res.Removed, map[bool]string{true: "", false: "s"}[res.Removed == 1])
	if res.Duplicate {
		msg = "That shopping trip was already logged."
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg, "spending_id": res.SpendingID, "duplicate": res.Duplicate})
}

// listBotAmount accepts a JSON number or a string like "250k" / "250.000".
func listBotAmount(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return int64(f), f > 0
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		n, ok := listParseAmount(s)
		return n, ok && n > 0
	}
	return 0, false
}

// listFormatIDR: 250000 -> "Rp250.000" (bot replies; amounts are IDR base).
func listFormatIDR(n int64) string {
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return "Rp" + b.String()
}
