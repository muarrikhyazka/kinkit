package main

// Shared shopping lists: lists, items (bulk text add), staples (recurring
// items auto-added when due), suggestions and "finish shopping", which logs
// ONE spending row for the trip. Bot endpoints live in lists_bot.go.
//
// Time: all instants are UTC (TIMESTAMPTZ); "today" for staples and for the
// spending row created by a trip is the household's local day
// (householdToday). Amounts are IDR base like every other money column.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func RegisterListRoutes(r chi.Router, app *App) {
	r.Get("/lists", app.handleListLists)
	r.Post("/lists", app.handleCreateList)
	r.Get("/lists/suggestions", app.handleListSuggestions)
	r.Get("/lists/{id}", app.handleGetList)
	r.Patch("/lists/{id}", app.handleUpdateList)
	r.Delete("/lists/{id}", app.handleDeleteList)
	r.Post("/lists/{id}/items", app.handleAddItems)
	r.Patch("/lists/{id}/items/{itemID}", app.handleUpdateItem)
	r.Delete("/lists/{id}/items/{itemID}", app.handleDeleteItem)
	r.Post("/lists/{id}/reorder", app.handleReorderItems)
	r.Post("/lists/{id}/clear-checked", app.handleClearChecked)
	r.Post("/lists/{id}/finish", app.handleFinishShopping)
	r.Get("/staples", app.handleListStaples)
	r.Post("/staples", app.handleCreateStaple)
	r.Patch("/staples/{id}", app.handleUpdateStaple)
	r.Delete("/staples/{id}", app.handleDeleteStaple)
	// bot-token authenticated (lists_bot.go)
	r.Post("/bot/lists/items", botAuth(app, botListAddItems))
	r.Get("/bot/lists", botAuth(app, botListItems))
	r.Post("/bot/lists/finish", botAuth(app, botListFinish))
}

func init() {
	// Kids may add items and tick items off (the handler only lets them
	// change `checked`); everything else on lists stays parent-only.
	registerKidRule(func(method, path string) (roleDecision, int64, bool) {
		rest, ok := strings.CutPrefix(path, "/api/lists/")
		if !ok {
			return 0, 0, false
		}
		parts := strings.Split(rest, "/")
		if len(parts) < 2 || parts[1] != "items" || !listIsDigits(parts[0]) {
			return 0, 0, false
		}
		switch {
		case len(parts) == 2 && method == http.MethodPost:
			return roleAllow, 0, true
		case len(parts) == 3 && method == http.MethodPatch && listIsDigits(parts[2]):
			return roleAllow, 0, true
		}
		return 0, 0, false
	})
	registerTodayProvider("lists", 40, listsTodayProvider)
}

func listIsDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ---------- types ----------

type shoppingList struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Emoji        string `json:"emoji"`
	Store        string `json:"store"`
	SortOrder    int    `json:"sort_order"`
	Archived     bool   `json:"archived"`
	CreatedAt    string `json:"created_at"`
	OpenCount    int    `json:"open_count"`
	CheckedCount int    `json:"checked_count"`
}

type shoppingItem struct {
	ID            int64   `json:"id"`
	ListID        int64   `json:"list_id"`
	Name          string  `json:"name"`
	Qty           string  `json:"qty"`
	Note          string  `json:"note"`
	CategoryID    *int64  `json:"category_id"`
	CategoryName  *string `json:"category_name"`
	EstPrice      *int64  `json:"est_price"`
	Checked       bool    `json:"checked"`
	CheckedBy     *int64  `json:"checked_by"`
	CheckedByName *string `json:"checked_by_name"`
	CheckedAt     *string `json:"checked_at"`
	AddedBy       *int64  `json:"added_by"`
	AddedByName   *string `json:"added_by_name"`
	StapleID      *int64  `json:"staple_id"`
	SortOrder     int     `json:"sort_order"`
	CreatedAt     string  `json:"created_at"`
}

func listTS(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func listTSPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := listTS(*t)
	return &s
}

const listCols = "l.id, l.name, l.emoji, l.store, l.sort_order, l.archived, l.created_at, " +
	"(SELECT COUNT(*) FROM shopping_items i WHERE i.list_id=l.id AND NOT i.checked), " +
	"(SELECT COUNT(*) FROM shopping_items i WHERE i.list_id=l.id AND i.checked)"

func scanShoppingList(row pgx.Row) (shoppingList, error) {
	var l shoppingList
	var created time.Time
	err := row.Scan(&l.ID, &l.Name, &l.Emoji, &l.Store, &l.SortOrder, &l.Archived, &created, &l.OpenCount, &l.CheckedCount)
	l.CreatedAt = listTS(created)
	return l, err
}

const itemCols = "i.id, i.list_id, i.name, i.qty, i.note, i.category_id, c.name, i.est_price, i.checked, i.checked_by, " +
	"mc.display_name, i.checked_at, i.added_by, ma.display_name, i.staple_id, i.sort_order, i.created_at"
const itemJoins = " FROM shopping_items i LEFT JOIN categories c ON c.id=i.category_id " +
	"LEFT JOIN members mc ON mc.id=i.checked_by LEFT JOIN members ma ON ma.id=i.added_by "

func scanShoppingItems(rows pgx.Rows) ([]shoppingItem, error) {
	defer rows.Close()
	out := []shoppingItem{}
	for rows.Next() {
		var it shoppingItem
		var checkedAt *time.Time
		var created time.Time
		if err := rows.Scan(&it.ID, &it.ListID, &it.Name, &it.Qty, &it.Note, &it.CategoryID, &it.CategoryName, &it.EstPrice,
			&it.Checked, &it.CheckedBy, &it.CheckedByName, &checkedAt, &it.AddedBy, &it.AddedByName, &it.StapleID,
			&it.SortOrder, &created); err != nil {
			return nil, err
		}
		it.CheckedAt = listTSPtr(checkedAt)
		it.CreatedAt = listTS(created)
		out = append(out, it)
	}
	return out, rows.Err()
}

func (app *App) loadItems(ctx context.Context, aid, listID int64, where string, args ...any) ([]shoppingItem, error) {
	q := "SELECT " + itemCols + itemJoins + "WHERE i.account_id=$1 AND i.list_id=$2"
	if where != "" {
		q += " AND " + where
	}
	q += " ORDER BY i.checked, i.sort_order, i.id"
	rows, err := app.DB.Query(ctx, q, append([]any{aid, listID}, args...)...)
	if err != nil {
		return nil, err
	}
	return scanShoppingItems(rows)
}

func (app *App) loadItem(ctx context.Context, aid, itemID int64) (shoppingItem, error) {
	rows, err := app.DB.Query(ctx, "SELECT "+itemCols+itemJoins+"WHERE i.account_id=$1 AND i.id=$2", aid, itemID)
	if err != nil {
		return shoppingItem{}, err
	}
	items, err := scanShoppingItems(rows)
	if err != nil {
		return shoppingItem{}, err
	}
	if len(items) == 0 {
		return shoppingItem{}, pgx.ErrNoRows
	}
	return items[0], nil
}

// ---------- lists ----------

// ensureDefaultList creates "Groceries" for a household that has no list at
// all (archived lists count, so archiving everything doesn't resurrect it).
// The users row lock serialises concurrent first visits.
func (app *App) ensureDefaultList(ctx context.Context, aid int64) error {
	var n int
	if err := app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM shopping_lists WHERE account_id=$1", aid).Scan(&n); err != nil || n > 0 {
		return err
	}
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", aid); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO shopping_lists (account_id, name, emoji) SELECT $1, 'Groceries', '🛒' "+
			"WHERE NOT EXISTS (SELECT 1 FROM shopping_lists WHERE account_id=$1)", aid); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (app *App) loadLists(ctx context.Context, aid int64, includeArchived bool) ([]shoppingList, error) {
	q := "SELECT " + listCols + " FROM shopping_lists l WHERE l.account_id=$1"
	if !includeArchived {
		q += " AND NOT l.archived"
	}
	rows, err := app.DB.Query(ctx, q+" ORDER BY l.archived, l.sort_order, l.id", aid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []shoppingList{}
	for rows.Next() {
		l, err := scanShoppingList(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (app *App) loadList(ctx context.Context, aid, id int64) (shoppingList, error) {
	return scanShoppingList(app.DB.QueryRow(ctx, "SELECT "+listCols+" FROM shopping_lists l WHERE l.account_id=$1 AND l.id=$2", aid, id))
}

// listFromPath loads {id} of the current household or writes 404.
func (app *App) listFromPath(w http.ResponseWriter, r *http.Request, aid int64) (shoppingList, bool) {
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid list id")
		return shoppingList{}, false
	}
	l, err := app.loadList(r.Context(), aid, id)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "list not found")
		return l, false
	}
	if err != nil {
		dbError(w, err)
		return l, false
	}
	return l, true
}

func (app *App) handleListLists(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if err := app.ensureDefaultList(ctx, aid); err != nil {
		dbError(w, err)
		return
	}
	added, _ := app.addDueStaples(ctx, aid, time.Now())
	lists, err := app.loadLists(ctx, aid, r.URL.Query().Get("archived") == "1")
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lists": lists, "staples_added": added})
}

type listInput struct {
	Name      *string `json:"name"`
	Emoji     *string `json:"emoji"`
	Store     *string `json:"store"`
	SortOrder *int    `json:"sort_order"`
	Archived  *bool   `json:"archived"`
}

func (app *App) handleCreateList(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var d listInput
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	name := ""
	if d.Name != nil {
		name = listClip(strings.TrimSpace(*d.Name), 60)
	}
	if name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	emoji, store := "", ""
	if d.Emoji != nil {
		emoji = listClip(strings.TrimSpace(*d.Emoji), 8)
	}
	if d.Store != nil {
		store = listClip(strings.TrimSpace(*d.Store), 80)
	}
	ctx := r.Context()
	var id int64
	err := app.DB.QueryRow(ctx,
		"INSERT INTO shopping_lists (account_id, name, emoji, store, sort_order) VALUES ($1,$2,$3,$4,"+
			"(SELECT COALESCE(MAX(sort_order),0)+1 FROM shopping_lists WHERE account_id=$1)) RETURNING id",
		aid, name, emoji, store).Scan(&id)
	if err != nil {
		dbError(w, err)
		return
	}
	l, err := app.loadList(ctx, aid, id)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (app *App) handleGetList(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	added, _ := app.addDueStaples(ctx, aid, time.Now())
	l, ok := app.listFromPath(w, r, aid)
	if !ok {
		return
	}
	items, err := app.loadItems(ctx, aid, l.ID, "")
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"list": l, "items": items, "staples_added": added})
}

func (app *App) handleUpdateList(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	l, ok := app.listFromPath(w, r, aid)
	if !ok {
		return
	}
	var d listInput
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	sets, args := []string{}, []any{}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s=$%d", col, len(args)))
	}
	if d.Name != nil {
		n := listClip(strings.TrimSpace(*d.Name), 60)
		if n == "" {
			jsonError(w, http.StatusBadRequest, "name can't be empty")
			return
		}
		add("name", n)
	}
	if d.Emoji != nil {
		add("emoji", listClip(strings.TrimSpace(*d.Emoji), 8))
	}
	if d.Store != nil {
		add("store", listClip(strings.TrimSpace(*d.Store), 80))
	}
	if d.SortOrder != nil {
		add("sort_order", *d.SortOrder)
	}
	if d.Archived != nil {
		add("archived", *d.Archived)
	}
	ctx := r.Context()
	if len(sets) > 0 {
		args = append(args, l.ID, aid)
		q := fmt.Sprintf("UPDATE shopping_lists SET %s WHERE id=$%d AND account_id=$%d", strings.Join(sets, ","), len(args)-1, len(args))
		if _, err := app.DB.Exec(ctx, q, args...); err != nil {
			dbError(w, err)
			return
		}
	}
	l, err := app.loadList(ctx, aid, l.ID)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (app *App) handleDeleteList(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	l, ok := app.listFromPath(w, r, aid)
	if !ok {
		return
	}
	// items + staples cascade; past trips keep their spending row (list_id -> NULL)
	if _, err := app.DB.Exec(r.Context(), "DELETE FROM shopping_lists WHERE id=$1 AND account_id=$2", l.ID, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- items ----------

// newListItem is one item to add; CategoryID/EstPrice/Note only come from the
// single-item web form.
type newListItem struct {
	parsedItem
	Note       string
	CategoryID *int64
	EstPrice   *int64
}

// addListItems adds items to a list. An item whose name is already on the
// list (unchecked) is not duplicated: its qty is updated when a new one was
// given. Category defaults to what this household used last time for the
// same name. Returns the resulting rows in input order.
func (app *App) addListItems(ctx context.Context, aid, listID int64, items []newListItem, memberID *int64) ([]shoppingItem, error) {
	out := []shoppingItem{}
	for _, it := range items {
		name := listClip(strings.TrimSpace(reListSpaces.ReplaceAllString(it.Name, " ")), maxItemNameLen)
		if name == "" {
			continue
		}
		key := listItemKey(name)
		catID := it.CategoryID
		if catID != nil {
			var ok bool
			_ = app.DB.QueryRow(ctx, "SELECT TRUE FROM categories WHERE id=$1 AND account_id=$2", *catID, aid).Scan(&ok)
			if !ok {
				catID = nil
			}
		}
		if catID == nil {
			_ = app.DB.QueryRow(ctx, "SELECT category_id FROM shopping_item_stats WHERE account_id=$1 AND name_key=$2", aid, key).Scan(&catID)
		}
		var id int64
		err := app.DB.QueryRow(ctx,
			"SELECT id FROM shopping_items WHERE account_id=$1 AND list_id=$2 AND NOT checked AND lower(name)=$3 ORDER BY id LIMIT 1",
			aid, listID, key).Scan(&id)
		switch {
		case err == nil:
			if it.Qty != "" {
				if _, err := app.DB.Exec(ctx, "UPDATE shopping_items SET qty=$1 WHERE id=$2", it.Qty, id); err != nil {
					return out, err
				}
			}
		case errors.Is(err, pgx.ErrNoRows):
			err = app.DB.QueryRow(ctx,
				"INSERT INTO shopping_items (list_id, account_id, name, qty, note, category_id, est_price, added_by, sort_order) "+
					"VALUES ($1,$2,$3,$4,$5,$6,$7,$8,(SELECT COALESCE(MAX(sort_order),0)+1 FROM shopping_items WHERE list_id=$1)) RETURNING id",
				listID, aid, name, it.Qty, listClip(strings.TrimSpace(it.Note), 200), catID, it.EstPrice, memberID).Scan(&id)
			if err != nil {
				return out, err
			}
		default:
			return out, err
		}
		if _, err := app.DB.Exec(ctx,
			"INSERT INTO shopping_item_stats (account_id, name_key, name, times, category_id, last_added_at) VALUES ($1,$2,$3,1,$4,now()) "+
				"ON CONFLICT (account_id, name_key) DO UPDATE SET times=shopping_item_stats.times+1, name=EXCLUDED.name, "+
				"category_id=COALESCE(EXCLUDED.category_id, shopping_item_stats.category_id), last_added_at=now()",
			aid, key, name, catID); err != nil {
			return out, err
		}
		row, err := app.loadItem(ctx, aid, id)
		if err != nil {
			return out, err
		}
		out = append(out, row)
	}
	return out, nil
}

func listItemNames(items []shoppingItem) []string {
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}
	return names
}

func (app *App) handleAddItems(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	l, ok := app.listFromPath(w, r, aid)
	if !ok {
		return
	}
	var d struct {
		Text       string       `json:"text"`
		Name       string       `json:"name"`
		Qty        string       `json:"qty"`
		Note       string       `json:"note"`
		CategoryID *int64       `json:"category_id"`
		EstPrice   *int64       `json:"est_price"`
		Items      []parsedItem `json:"items"`
	}
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	var items []newListItem
	switch {
	case strings.TrimSpace(d.Name) != "":
		// a single item from the form: name taken literally
		if d.EstPrice != nil && *d.EstPrice < 0 {
			jsonError(w, http.StatusBadRequest, "est_price can't be negative")
			return
		}
		items = []newListItem{{parsedItem: parsedItem{Name: d.Name, Qty: listClip(strings.TrimSpace(d.Qty), 40)}, Note: d.Note, CategoryID: d.CategoryID, EstPrice: d.EstPrice}}
	case len(d.Items) > 0:
		for i, it := range d.Items {
			if i >= maxBulkItems {
				break
			}
			items = append(items, newListItem{parsedItem: parsedItem{Name: it.Name, Qty: listClip(strings.TrimSpace(it.Qty), 40)}})
		}
	default:
		for _, it := range parseItemsText(d.Text) {
			items = append(items, newListItem{parsedItem: it})
		}
	}
	if len(items) == 0 {
		jsonError(w, http.StatusBadRequest, "nothing to add")
		return
	}
	added, err := app.addListItems(r.Context(), aid, l.ID, items, currentMemberIDPtr(r))
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "items": added, "added": listItemNames(added)})
}

func (app *App) itemFromPath(w http.ResponseWriter, r *http.Request, aid int64) (shoppingItem, bool) {
	lid, err1 := urlInt64(r, "id")
	iid, err2 := urlInt64(r, "itemID")
	if err1 != nil || err2 != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return shoppingItem{}, false
	}
	it, err := app.loadItem(r.Context(), aid, iid)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && it.ListID != lid) {
		jsonError(w, http.StatusNotFound, "item not found")
		return it, false
	}
	if err != nil {
		dbError(w, err)
		return it, false
	}
	return it, true
}

func (app *App) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	it, ok := app.itemFromPath(w, r, aid)
	if !ok {
		return
	}
	var d map[string]json.RawMessage
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if m, ok := currentMember(r); ok && m.Role == roleKid {
		for k := range d {
			if k != "checked" {
				jsonError(w, http.StatusForbidden, "kids can only tick items off")
				return
			}
		}
	}
	sets, args := []string{}, []any{}
	add := func(expr string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf(expr, len(args)))
	}
	bad := func(msg string) { jsonError(w, http.StatusBadRequest, msg) }
	if raw, ok := d["checked"]; ok {
		var v bool
		if json.Unmarshal(raw, &v) != nil {
			bad("checked must be true/false")
			return
		}
		if v != it.Checked {
			add("checked=$%d", v)
			if v {
				add("checked_by=$%d", currentMemberIDPtr(r))
				sets = append(sets, "checked_at=now()")
			} else {
				sets = append(sets, "checked_by=NULL", "checked_at=NULL")
			}
		}
	}
	strField := func(key, col string, max int, required bool) bool {
		raw, ok := d[key]
		if !ok {
			return true
		}
		var v string
		if json.Unmarshal(raw, &v) != nil {
			bad(key + " must be a string")
			return false
		}
		v = listClip(strings.TrimSpace(v), max)
		if required && v == "" {
			bad(key + " can't be empty")
			return false
		}
		add(col+"=$%d", v)
		return true
	}
	if !strField("name", "name", maxItemNameLen, true) || !strField("qty", "qty", 40, false) || !strField("note", "note", 200, false) {
		return
	}
	ctx := r.Context()
	var newCat *int64
	catChanged := false
	if raw, ok := d["category_id"]; ok {
		if json.Unmarshal(raw, &newCat) != nil {
			bad("category_id must be a number or null")
			return
		}
		if newCat != nil {
			var exists bool
			_ = app.DB.QueryRow(ctx, "SELECT TRUE FROM categories WHERE id=$1 AND account_id=$2", *newCat, aid).Scan(&exists)
			if !exists {
				bad("unknown category")
				return
			}
		}
		add("category_id=$%d", newCat)
		catChanged = true
	}
	if raw, ok := d["est_price"]; ok {
		var v *int64
		if json.Unmarshal(raw, &v) != nil || (v != nil && *v < 0) {
			bad("est_price must be a non-negative number or null")
			return
		}
		add("est_price=$%d", v)
	}
	if len(sets) > 0 {
		args = append(args, it.ID, aid)
		q := fmt.Sprintf("UPDATE shopping_items SET %s WHERE id=$%d AND account_id=$%d", strings.Join(sets, ","), len(args)-1, len(args))
		if _, err := app.DB.Exec(ctx, q, args...); err != nil {
			dbError(w, err)
			return
		}
	}
	if catChanged {
		// remember the category for next time this item is added
		_, _ = app.DB.Exec(ctx, "UPDATE shopping_item_stats SET category_id=$1 WHERE account_id=$2 AND name_key=$3", newCat, aid, listItemKey(it.Name))
	}
	updated, err := app.loadItem(ctx, aid, it.ID)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (app *App) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	it, ok := app.itemFromPath(w, r, aid)
	if !ok {
		return
	}
	if _, err := app.DB.Exec(r.Context(), "DELETE FROM shopping_items WHERE id=$1 AND account_id=$2", it.ID, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleReorderItems(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	l, ok := app.listFromPath(w, r, aid)
	if !ok {
		return
	}
	var d struct {
		IDs []int64 `json:"ids"`
	}
	if err := decodeJSON(r, &d); err != nil || len(d.IDs) == 0 {
		jsonError(w, http.StatusBadRequest, "ids required")
		return
	}
	// ids not on this list are ignored by the WHERE
	if _, err := app.DB.Exec(r.Context(),
		"UPDATE shopping_items i SET sort_order=o.ord FROM unnest($1::bigint[]) WITH ORDINALITY AS o(id, ord) "+
			"WHERE i.id=o.id AND i.list_id=$2 AND i.account_id=$3", d.IDs, l.ID, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleClearChecked(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	l, ok := app.listFromPath(w, r, aid)
	if !ok {
		return
	}
	tag, err := app.DB.Exec(r.Context(), "DELETE FROM shopping_items WHERE list_id=$1 AND account_id=$2 AND checked", l.ID, aid)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "removed": tag.RowsAffected()})
}

// ---------- suggestions ----------

func (app *App) handleListSuggestions(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	q := listItemKey(r.URL.Query().Get("q"))
	limit := queryIntDefault(r, "limit", 8)
	if limit < 1 || limit > 50 {
		limit = 8
	}
	args := []any{aid, limit}
	where := "s.account_id=$1"
	if q != "" {
		args = append(args, q+"%", "%"+q+"%")
		where += " AND s.name_key LIKE $4"
	}
	order := "s.times DESC, s.last_added_at DESC"
	if q != "" {
		order = "(s.name_key LIKE $3) DESC, " + order
	}
	// hide names already waiting on the given list
	if lid, err := strconv.ParseInt(r.URL.Query().Get("list"), 10, 64); err == nil {
		args = append(args, lid)
		where += fmt.Sprintf(" AND NOT EXISTS (SELECT 1 FROM shopping_items i WHERE i.list_id=$%d AND NOT i.checked AND lower(i.name)=s.name_key)", len(args))
	}
	rows, err := app.DB.Query(r.Context(),
		"SELECT s.name, s.times, s.category_id, c.name FROM shopping_item_stats s LEFT JOIN categories c ON c.id=s.category_id "+
			"WHERE "+where+" ORDER BY "+order+" LIMIT $2", args...)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()
	type sugg struct {
		Name         string  `json:"name"`
		Times        int     `json:"times"`
		CategoryID   *int64  `json:"category_id"`
		CategoryName *string `json:"category_name"`
	}
	out := []sugg{}
	for rows.Next() {
		var s sugg
		if err := rows.Scan(&s.Name, &s.Times, &s.CategoryID, &s.CategoryName); err != nil {
			dbError(w, err)
			return
		}
		out = append(out, s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": out})
}

// ---------- finish shopping ----------

type finishInput struct {
	ClientID    string  `json:"client_id"`
	TotalAmount int64   `json:"total_amount"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Items       []int64 `json:"items"` // nil -> all checked items
}

type finishResult struct {
	SpendingID int64  `json:"spending_id"`
	TripID     int64  `json:"trip_id"`
	Duplicate  bool   `json:"duplicate"`
	Removed    int    `json:"removed"`
	Amount     int64  `json:"amount"`
	Category   string `json:"category"`
}

var errListNoCategory = errors.New("no spending category available")

// defaultShoppingCategory picks a groceries-like category, else the first
// Needs category, else any category.
func (app *App) defaultShoppingCategory(ctx context.Context, aid int64) (int64, string, error) {
	var id int64
	var name string
	err := app.DB.QueryRow(ctx, `SELECT id, name FROM categories WHERE account_id=$1 ORDER BY
		CASE WHEN name ILIKE '%grocer%' THEN 0 WHEN name ILIKE '%belanja%' THEN 1 WHEN name ILIKE '%supermarket%' THEN 2
		     WHEN name ILIKE 'food' THEN 3 WHEN name ILIKE '%food%' OR name ILIKE '%makan%' THEN 4
		     WHEN name ILIKE 'supplies' THEN 5 WHEN main_category='Needs' THEN 6 ELSE 7 END, id LIMIT 1`, aid).Scan(&id, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", errListNoCategory
	}
	return id, name, err
}

// finishShopping logs one spending row (source 'list') for the trip, records
// a shopping_trips row and removes the bought items - all in one
// transaction. Idempotent on client_id ("trip:<client uuid>"): a retry
// returns the first trip's spending id with Duplicate=true. Points are
// awarded like any other logged spending.
func (app *App) finishShopping(ctx context.Context, aid int64, l shoppingList, in finishInput, memberID *int64) (finishResult, error) {
	res := finishResult{Amount: in.TotalAmount}
	clientID := strings.TrimSpace(in.ClientID)
	if clientID == "" {
		clientID = randomToken()
	}
	if !strings.HasPrefix(clientID, "trip:") {
		clientID = "trip:" + clientID
	}
	clientID = listClip(clientID, 120)

	var existing int64
	if err := app.DB.QueryRow(ctx, "SELECT id FROM spending WHERE account_id=$1 AND client_id=$2", aid, clientID).Scan(&existing); err == nil {
		res.SpendingID, res.Duplicate = existing, true
		_ = app.DB.QueryRow(ctx, "SELECT id FROM shopping_trips WHERE account_id=$1 AND client_id=$2", aid, clientID).Scan(&res.TripID)
		return res, nil
	}

	var catID int64
	var catName string
	if c := strings.TrimSpace(in.Category); c != "" {
		if err := app.DB.QueryRow(ctx, "SELECT id, name FROM categories WHERE account_id=$1 AND name=$2", aid, c).Scan(&catID, &catName); err != nil {
			return res, fmt.Errorf("unknown category %q", c)
		}
	} else {
		var err error
		if catID, catName, err = app.defaultShoppingCategory(ctx, aid); err != nil {
			return res, err
		}
	}
	res.Category = catName

	tx, err := app.DB.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx)

	q := "SELECT id, name, qty, est_price FROM shopping_items WHERE account_id=$1 AND list_id=$2 AND "
	args := []any{aid, l.ID}
	if in.Items == nil {
		q += "checked"
	} else {
		q += "id = ANY($3)"
		args = append(args, in.Items)
	}
	rows, err := tx.Query(ctx, q+" ORDER BY sort_order, id FOR UPDATE", args...)
	if err != nil {
		return res, err
	}
	type tripItem struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Qty      string `json:"qty,omitempty"`
		EstPrice *int64 `json:"est_price,omitempty"`
	}
	bought := []tripItem{}
	ids := []int64{}
	for rows.Next() {
		var t tripItem
		if err := rows.Scan(&t.ID, &t.Name, &t.Qty, &t.EstPrice); err != nil {
			rows.Close()
			return res, err
		}
		bought = append(bought, t)
		ids = append(ids, t.ID)
	}
	rows.Close()
	if rows.Err() != nil {
		return res, rows.Err()
	}

	desc := listClip(strings.TrimSpace(in.Description), 200)
	if desc == "" {
		desc = l.Name
		if n := len(bought); n > 0 {
			desc = fmt.Sprintf("%s (%d item%s)", l.Name, n, map[bool]string{true: "", false: "s"}[n == 1])
		}
	}
	day := householdToday(ctx, app, aid, time.Now())
	var spendingID int64
	err = tx.QueryRow(ctx,
		"INSERT INTO spending (account_id, tx_date, description, category_id, category_name, amount, source, client_id, member_id) "+
			"VALUES ($1,$2,$3,$4,$5,$6,'list',$7,$8) ON CONFLICT (account_id, client_id) DO NOTHING RETURNING id",
		aid, day, desc, catID, catName, in.TotalAmount, clientID, memberID).Scan(&spendingID)
	if errors.Is(err, pgx.ErrNoRows) {
		// lost a race with a concurrent retry of the same trip
		_ = tx.Rollback(ctx)
		_ = app.DB.QueryRow(ctx, "SELECT id FROM spending WHERE account_id=$1 AND client_id=$2", aid, clientID).Scan(&res.SpendingID)
		_ = app.DB.QueryRow(ctx, "SELECT id FROM shopping_trips WHERE account_id=$1 AND client_id=$2", aid, clientID).Scan(&res.TripID)
		res.Duplicate = true
		return res, nil
	}
	if err != nil {
		return res, err
	}
	itemsJSON, _ := json.Marshal(bought)
	if err := tx.QueryRow(ctx,
		"INSERT INTO shopping_trips (account_id, list_id, total_amount, spending_id, member_id, client_id, items) "+
			"VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id",
		aid, l.ID, in.TotalAmount, spendingID, memberID, clientID, itemsJSON).Scan(&res.TripID); err != nil {
		return res, err
	}
	if len(ids) > 0 {
		if _, err := tx.Exec(ctx, "DELETE FROM shopping_items WHERE account_id=$1 AND id = ANY($2)", aid, ids); err != nil {
			return res, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return res, err
	}
	res.SpendingID, res.Removed = spendingID, len(ids)
	// same side effect as POST /api/spending (spending.go handleAddSpending)
	if memberID != nil {
		app.awardPointsForMember(ctx, aid, memberID)
	} else {
		app.awardPointsForLog(ctx, aid)
	}
	return res, nil
}

func (app *App) handleFinishShopping(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	l, ok := app.listFromPath(w, r, aid)
	if !ok {
		return
	}
	var in finishInput
	if err := decodeJSON(r, &in); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.TotalAmount <= 0 {
		jsonError(w, http.StatusBadRequest, "total_amount must be more than 0")
		return
	}
	res, err := app.finishShopping(r.Context(), aid, l, in, currentMemberIDPtr(r))
	if err != nil {
		if errors.Is(err, errListNoCategory) || strings.HasPrefix(err.Error(), "unknown category") {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "spending_id": res.SpendingID, "trip_id": res.TripID, "duplicate": res.Duplicate,
		"removed": res.Removed, "amount": res.Amount, "category": res.Category,
	})
}

// ---------- staples ----------

type stapleRow struct {
	ID          int64   `json:"id"`
	ListID      int64   `json:"list_id"`
	ListName    string  `json:"list_name"`
	Name        string  `json:"name"`
	Qty         string  `json:"qty"`
	EveryDays   int     `json:"every_days"`
	LastAddedAt *string `json:"last_added_at"`
	NextDue     string  `json:"next_due"` // household-local YYYY-MM-DD
	Active      bool    `json:"active"`
}

func (app *App) loadStaples(ctx context.Context, aid int64, where string, args ...any) ([]stapleRow, error) {
	q := "SELECT s.id, s.list_id, l.name, s.name, s.qty, s.every_days, s.last_added_at, s.active " +
		"FROM staples s JOIN shopping_lists l ON l.id=s.list_id WHERE s.account_id=$1"
	if where != "" {
		q += " AND " + where
	}
	rows, err := app.DB.Query(ctx, q+" ORDER BY l.sort_order, l.id, lower(s.name)", append([]any{aid}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	loc := householdLocation(ctx, app, aid)
	today := stapleLocalDay(time.Now(), loc)
	out := []stapleRow{}
	for rows.Next() {
		var s stapleRow
		var last *time.Time
		if err := rows.Scan(&s.ID, &s.ListID, &s.ListName, &s.Name, &s.Qty, &s.EveryDays, &last, &s.Active); err != nil {
			return nil, err
		}
		s.LastAddedAt = listTSPtr(last)
		s.NextDue = stapleNextDue(last, s.EveryDays, today, loc).Format(isoDateLayout)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (app *App) handleListStaples(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	where, args := "", []any{}
	if lid, err := strconv.ParseInt(r.URL.Query().Get("list"), 10, 64); err == nil {
		where, args = "s.list_id=$2", []any{lid}
	}
	st, err := app.loadStaples(r.Context(), aid, where, args...)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"staples": st})
}

type stapleInput struct {
	ListID    *int64  `json:"list_id"`
	Name      *string `json:"name"`
	Qty       *string `json:"qty"`
	EveryDays *int    `json:"every_days"`
	Active    *bool   `json:"active"`
}

func (app *App) handleCreateStaple(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var d stapleInput
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if d.ListID == nil || d.Name == nil || strings.TrimSpace(*d.Name) == "" || d.EveryDays == nil {
		jsonError(w, http.StatusBadRequest, "list_id, name and every_days are required")
		return
	}
	if *d.EveryDays < 1 || *d.EveryDays > 366 {
		jsonError(w, http.StatusBadRequest, "every_days must be between 1 and 366")
		return
	}
	ctx := r.Context()
	if _, err := app.loadList(ctx, aid, *d.ListID); err != nil {
		jsonError(w, http.StatusNotFound, "list not found")
		return
	}
	qty := ""
	if d.Qty != nil {
		qty = listClip(strings.TrimSpace(*d.Qty), 40)
	}
	var id int64
	if err := app.DB.QueryRow(ctx,
		"INSERT INTO staples (account_id, list_id, name, qty, every_days) VALUES ($1,$2,$3,$4,$5) RETURNING id",
		aid, *d.ListID, listClip(strings.TrimSpace(*d.Name), maxItemNameLen), qty, *d.EveryDays).Scan(&id); err != nil {
		dbError(w, err)
		return
	}
	// a new staple is due right away: put it on the list now
	_, _ = app.addDueStaples(ctx, aid, time.Now())
	st, err := app.loadStaples(ctx, aid, "s.id=$2", id)
	if err != nil || len(st) == 0 {
		dbError(w, fmt.Errorf("reload staple: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, st[0])
}

func (app *App) handleUpdateStaple(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var d stapleInput
	if err := decodeJSON(r, &d); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx := r.Context()
	sets, args := []string{}, []any{}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s=$%d", col, len(args)))
	}
	if d.Name != nil {
		n := listClip(strings.TrimSpace(*d.Name), maxItemNameLen)
		if n == "" {
			jsonError(w, http.StatusBadRequest, "name can't be empty")
			return
		}
		add("name", n)
	}
	if d.Qty != nil {
		add("qty", listClip(strings.TrimSpace(*d.Qty), 40))
	}
	if d.EveryDays != nil {
		if *d.EveryDays < 1 || *d.EveryDays > 366 {
			jsonError(w, http.StatusBadRequest, "every_days must be between 1 and 366")
			return
		}
		add("every_days", *d.EveryDays)
	}
	if d.Active != nil {
		add("active", *d.Active)
	}
	if d.ListID != nil {
		if _, err := app.loadList(ctx, aid, *d.ListID); err != nil {
			jsonError(w, http.StatusNotFound, "list not found")
			return
		}
		add("list_id", *d.ListID)
	}
	if len(sets) > 0 {
		args = append(args, id, aid)
		tag, err := app.DB.Exec(ctx, fmt.Sprintf("UPDATE staples SET %s WHERE id=$%d AND account_id=$%d", strings.Join(sets, ","), len(args)-1, len(args)), args...)
		if err != nil {
			dbError(w, err)
			return
		}
		if tag.RowsAffected() == 0 {
			jsonError(w, http.StatusNotFound, "staple not found")
			return
		}
	}
	st, err := app.loadStaples(ctx, aid, "s.id=$2", id)
	if err != nil {
		dbError(w, err)
		return
	}
	if len(st) == 0 {
		jsonError(w, http.StatusNotFound, "staple not found")
		return
	}
	writeJSON(w, http.StatusOK, st[0])
}

func (app *App) handleDeleteStaple(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	id, err := urlInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := app.DB.Exec(r.Context(), "DELETE FROM staples WHERE id=$1 AND account_id=$2", id, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// addDueStaples puts every due staple of the household on its list (lazy:
// called from list reads, the Today provider and the bot). Idempotent and
// race-safe: last_added_at is advanced with a compare-and-set, so only one
// caller adds a given occurrence. A staple already waiting on the list
// (unchecked) is not duplicated - its occurrence is just marked done.
// Returns the names that were added.
func (app *App) addDueStaples(ctx context.Context, aid int64, now time.Time) ([]string, error) {
	loc := householdLocation(ctx, app, aid)
	today := stapleLocalDay(now, loc)
	rows, err := app.DB.Query(ctx,
		"SELECT s.id, s.list_id, s.name, s.qty, s.every_days, s.last_added_at FROM staples s "+
			"JOIN shopping_lists l ON l.id=s.list_id WHERE s.account_id=$1 AND s.active AND NOT l.archived", aid)
	if err != nil {
		return nil, err
	}
	type due struct {
		id, listID int64
		name, qty  string
		last       *time.Time
	}
	var dues []due
	for rows.Next() {
		var d due
		var every int
		if err := rows.Scan(&d.id, &d.listID, &d.name, &d.qty, &every, &d.last); err != nil {
			rows.Close()
			return nil, err
		}
		if stapleDue(d.last, every, today, loc) {
			dues = append(dues, d)
		}
	}
	rows.Close()
	added := []string{}
	for _, d := range dues {
		ok, err := app.addOneStaple(ctx, aid, d.id, d.listID, d.name, d.qty, d.last, now)
		if err != nil {
			return added, err
		}
		if ok {
			added = append(added, d.name)
		}
	}
	return added, nil
}

func (app *App) addOneStaple(ctx context.Context, aid, stapleID, listID int64, name, qty string, last *time.Time, now time.Time) (bool, error) {
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "UPDATE staples SET last_added_at=$1 WHERE id=$2 AND last_added_at IS NOT DISTINCT FROM $3", now.UTC(), stapleID, last)
	if err != nil || tag.RowsAffected() == 0 {
		return false, err // someone else handled this occurrence
	}
	var exists bool
	_ = tx.QueryRow(ctx, "SELECT TRUE FROM shopping_items WHERE list_id=$1 AND NOT checked AND lower(name)=$2 LIMIT 1", listID, listItemKey(name)).Scan(&exists)
	if !exists {
		var catID *int64
		_ = tx.QueryRow(ctx, "SELECT category_id FROM shopping_item_stats WHERE account_id=$1 AND name_key=$2", aid, listItemKey(name)).Scan(&catID)
		if _, err := tx.Exec(ctx,
			"INSERT INTO shopping_items (list_id, account_id, name, qty, category_id, staple_id, sort_order) "+
				"VALUES ($1,$2,$3,$4,$5,$6,(SELECT COALESCE(MAX(sort_order),0)+1 FROM shopping_items WHERE list_id=$1))",
			listID, aid, name, qty, catID, stapleID); err != nil {
			return false, err
		}
	}
	return !exists, tx.Commit(ctx)
}

// ---------- Today provider ----------

func listsTodayProvider(ctx context.Context, app *App, aid int64, _ memberInfo, day time.Time) (*TodaySection, error) {
	_, _ = app.addDueStaples(ctx, aid, time.Now())
	lists, err := app.loadLists(ctx, aid, false)
	if err != nil {
		return nil, err
	}
	// staples put on a list during the household's local `day`
	loc := householdLocation(ctx, app, aid)
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	stapleNames := map[int64][]string{}
	rows, err := app.DB.Query(ctx,
		"SELECT list_id, name FROM staples WHERE account_id=$1 AND active AND last_added_at >= $2 AND last_added_at < $3 ORDER BY lower(name)",
		aid, dayStart.UTC(), dayStart.AddDate(0, 0, 1).UTC())
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var lid int64
		var n string
		if err := rows.Scan(&lid, &n); err != nil {
			rows.Close()
			return nil, err
		}
		stapleNames[lid] = append(stapleNames[lid], n)
	}
	rows.Close()

	sec := &TodaySection{Key: "lists", Title: "Shopping lists", Items: []TodayItem{}}
	var summary []string
	for _, l := range lists {
		if l.OpenCount == 0 {
			continue
		}
		title := fmt.Sprintf("%s: %d item%s to buy", l.Name, l.OpenCount, map[bool]string{true: "", false: "s"}[l.OpenCount == 1])
		names := []string{}
		if items, err := app.loadItems(ctx, aid, l.ID, "NOT i.checked"); err == nil {
			for i, it := range items {
				if i == 5 {
					names = append(names, "…")
					break
				}
				names = append(names, it.Name)
			}
		}
		detail := strings.Join(names, ", ")
		if st := stapleNames[l.ID]; len(st) > 0 {
			detail = strings.TrimSpace(detail + " · Staples added today: " + strings.Join(st, ", "))
		}
		sec.Items = append(sec.Items, TodayItem{
			Kind: "list", ID: l.ID, Title: title, Detail: detail,
			Link: "/home/lists?list=" + strconv.FormatInt(l.ID, 10),
		})
		summary = append(summary, title)
	}
	if len(sec.Items) == 0 {
		return nil, nil
	}
	sec.Summary = strings.Join(summary, " · ")
	return sec, nil
}
