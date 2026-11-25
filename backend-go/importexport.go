package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// RegisterDataRoutes wires the "Data & devices" features: CSV, Excel (XLSX)
// and JSON export, CSV import (dry run + commit + undo), receipt photos, the synced custom
// dashboard layout and read-only TV share links. Everything except
// GET /tv/data (token-authenticated) requires a logged-in account.
func RegisterDataRoutes(r chi.Router, app *App) {
	r.Get("/export/spending.csv", app.handleExportSpendingCSV)
	r.Get("/export/income.csv", app.handleExportIncomeCSV)
	r.Get("/export/all.json", app.handleExportAllJSON)
	r.Get("/export/spending.xlsx", app.handleExportSpendingXLSX)
	r.Get("/export/income.xlsx", app.handleExportIncomeXLSX)
	r.Get("/export/all.xlsx", app.handleExportAllXLSX)

	r.Get("/import/presets", app.handleImportPresets)
	r.Post("/import/spending", app.handleImportSpending)
	r.Get("/import/batches", app.handleListImportBatches)
	r.Delete("/import/batches/{batch}", app.handleUndoImport)

	r.Post("/spending/{sid}/receipt", app.handleUploadReceipt)
	r.Get("/spending/{sid}/receipt", app.handleGetReceipt)
	r.Delete("/spending/{sid}/receipt", app.handleDeleteReceipt)

	r.Get("/dashboard-layout", app.handleGetDashboardLayout)
	r.Put("/dashboard-layout", app.handlePutDashboardLayout)

	r.Get("/tv-tokens", app.handleListTVTokens)
	r.Post("/tv-tokens", app.handleCreateTVToken)
	r.Patch("/tv-tokens/{id}", app.handleUpdateTVToken)
	r.Delete("/tv-tokens/{id}", app.handleRevokeTVToken)
	r.With(publicCORS).Get("/tv/data", app.handleTVData) // ?k=<token>, no session; CORS for tv/web
}

// export

// csvSafe neutralizes spreadsheet formula injection in text cells.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func csvHeaders(w http.ResponseWriter, filename string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
}

func xlsxHeaders(w http.ResponseWriter, filename string) {
	w.Header().Set("Content-Type", xlsxContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
}

// writeXLSXResponse builds the whole workbook in memory first, so a failure
// still gets a clean 500 instead of a truncated download.
func writeXLSXResponse(w http.ResponseWriter, filename string, sheets []xlsxSheet) {
	var buf bytes.Buffer
	if err := writeXLSX(&buf, sheets); err != nil {
		dbError(w, err)
		return
	}
	xlsxHeaders(w, filename)
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	_, _ = w.Write(buf.Bytes())
}

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

var spendingExportHeader = []string{"id", "date", "description", "category", "main_category", "amount", "checked", "source", "has_receipt", "created_at"}

// spendingExportQuery builds the spending export query for an account,
// optionally limited to an inclusive date range.
func spendingExportQuery(aid int64, from, to string) (q string, args []any, err error) {
	q = `SELECT sp.id, sp.tx_date::text, sp.description, COALESCE(c.name, sp.category_name, ''), COALESCE(c.main_category, ''),
	            sp.amount, sp.checked, sp.source, sp.receipt_path IS NOT NULL, sp.created_at
	     FROM spending sp LEFT JOIN categories c ON c.id = sp.category_id WHERE sp.account_id=$1`
	args = []any{aid}
	if from != "" {
		if !validDate(from) {
			return "", nil, errors.New("from must be YYYY-MM-DD")
		}
		args = append(args, from)
		q += fmt.Sprintf(" AND sp.tx_date >= $%d", len(args))
	}
	if to != "" {
		if !validDate(to) {
			return "", nil, errors.New("to must be YYYY-MM-DD")
		}
		args = append(args, to)
		q += fmt.Sprintf(" AND sp.tx_date <= $%d", len(args))
	}
	return q + " ORDER BY sp.tx_date, sp.id", args, nil
}

// exportName is the spending export filename without its extension.
func exportName(from, to string) string {
	name := "kinkit-spending"
	if from != "" || to != "" {
		name += "-" + from + "_" + to
	}
	return name
}

// GET /api/export/spending.csv?from=YYYY-MM-DD&to=YYYY-MM-DD (both optional, inclusive)
func (app *App) handleExportSpendingCSV(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	q, args, err := spendingExportQuery(aid, from, to)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := app.DB.Query(r.Context(), q, args...)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()

	csvHeaders(w, exportName(from, to)+".csv")
	cw := csv.NewWriter(w)
	_ = cw.Write(spendingExportHeader)
	for rows.Next() {
		var id, amount int64
		var date, desc, cat, mainCat, source string
		var checked, hasReceipt bool
		var created time.Time
		if err := rows.Scan(&id, &date, &desc, &cat, &mainCat, &amount, &checked, &source, &hasReceipt, &created); err != nil {
			return // headers already sent; truncated file is the best we can do
		}
		_ = cw.Write([]string{strconv.FormatInt(id, 10), date, csvSafe(desc), csvSafe(cat), mainCat,
			strconv.FormatInt(amount, 10), strconv.FormatBool(checked), source, strconv.FormatBool(hasReceipt),
			created.UTC().Format(time.RFC3339)})
	}
	cw.Flush()
}

// GET /api/export/spending.xlsx?from=YYYY-MM-DD&to=YYYY-MM-DD - the same
// rows and columns as the CSV, with numbers and booleans as typed cells.
func (app *App) handleExportSpendingXLSX(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	q, args, err := spendingExportQuery(aid, from, to)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := app.DB.Query(r.Context(), q, args...)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()

	var data [][]any
	for rows.Next() {
		var id, amount int64
		var date, desc, cat, mainCat, source string
		var checked, hasReceipt bool
		var created time.Time
		if err := rows.Scan(&id, &date, &desc, &cat, &mainCat, &amount, &checked, &source, &hasReceipt, &created); err != nil {
			dbError(w, err)
			return
		}
		data = append(data, []any{id, date, desc, cat, mainCat, amount, checked, source, hasReceipt, created.UTC().Format(time.RFC3339)})
	}
	if err := rows.Err(); err != nil {
		dbError(w, err)
		return
	}
	writeXLSXResponse(w, exportName(from, to)+".xlsx", []xlsxSheet{{Name: "Spending", Header: spendingExportHeader, Rows: data}})
}

func (app *App) tableExists(ctx context.Context, name string) bool {
	var ok bool
	_ = app.DB.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", "public."+name).Scan(&ok)
	return ok
}

// exportTime renders midnight timestamps (date columns) as YYYY-MM-DD and
// everything else as RFC3339 in UTC.
func exportTime(t time.Time) string {
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
		return t.Format("2006-01-02")
	}
	return t.UTC().Format(time.RFC3339)
}

// csvValue formats any scanned DB value for a CSV cell.
func csvValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case time.Time:
		return exportTime(t)
	case string:
		return csvSafe(t)
	case []byte:
		return csvSafe(string(t))
	case map[string]any, []any:
		b, _ := json.Marshal(t)
		return string(b)
	}
	return fmt.Sprint(v)
}

// incomeExportRows returns actual income entries when the income tracking
// table exists, otherwise the planned (budgeted) income per month.
func (app *App) incomeExportRows(ctx context.Context, aid int64) (pgx.Rows, error) {
	if app.tableExists(ctx, "income_entries") {
		return app.DB.Query(ctx, "SELECT * FROM income_entries WHERE account_id=$1 ORDER BY 1", aid)
	}
	return app.DB.Query(ctx,
		`SELECT bi.year, bi.month, s.name AS source, bi.amount, 'budgeted' AS kind
		 FROM budget_income bi JOIN income_sources s ON s.id=bi.source_id
		 WHERE bi.account_id=$1 ORDER BY bi.year, bi.month, s.name`, aid)
}

func fieldNames(rows pgx.Rows) []string {
	names := []string{}
	for _, f := range rows.FieldDescriptions() {
		names = append(names, f.Name)
	}
	return names
}

// collectValues drains rows into memory and closes them.
func collectValues(rows pgx.Rows) ([][]any, error) {
	defer rows.Close()
	var out [][]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		out = append(out, vals)
	}
	return out, rows.Err()
}

// GET /api/export/income.csv - actual income entries when the income
// tracking table exists, otherwise the planned (budgeted) income per month.
func (app *App) handleExportIncomeCSV(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	rows, err := app.incomeExportRows(r.Context(), aid)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()
	csvHeaders(w, "kinkit-income.csv")
	cw := csv.NewWriter(w)
	_ = cw.Write(fieldNames(rows))
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			break
		}
		rec := make([]string, len(vals))
		for i, v := range vals {
			rec[i] = csvValue(v)
		}
		_ = cw.Write(rec)
	}
	cw.Flush()
}

// GET /api/export/income.xlsx - the same rows and columns as income.csv.
func (app *App) handleExportIncomeXLSX(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	rows, err := app.incomeExportRows(r.Context(), aid)
	if err != nil {
		dbError(w, err)
		return
	}
	header := fieldNames(rows)
	data, err := collectValues(rows)
	if err != nil {
		dbError(w, err)
		return
	}
	writeXLSXResponse(w, "kinkit-income.xlsx", []xlsxSheet{{Name: "Income", Header: header, Rows: data}})
}

// exportSkipTables hold secrets (bot tokens, share links) or are not user data.
var exportSkipTables = map[string]bool{"integrations": true, "tv_tokens": true, "password_resets": true, "users": true}

func sensitiveColumn(name string) bool {
	n := strings.ToLower(name)
	for _, bad := range []string{"token", "password", "hash", "secret", "pin", "receipt_path"} {
		if strings.Contains(n, bad) {
			return true
		}
	}
	return false
}

type exportTable struct {
	name string
	cols []string // already without secret columns
}

// exportTables lists every table that carries account_id (found at runtime,
// so tables added later are included automatically), minus secret tables
// and secret columns.
func (app *App) exportTables(ctx context.Context) ([]exportTable, error) {
	rows, err := app.DB.Query(ctx, `
		SELECT c.table_name, array_agg(c.column_name::text ORDER BY c.ordinal_position)
		FROM information_schema.columns c
		JOIN information_schema.tables t ON t.table_schema=c.table_schema AND t.table_name=c.table_name AND t.table_type='BASE TABLE'
		WHERE c.table_schema = current_schema()
		GROUP BY c.table_name
		HAVING bool_or(c.column_name = 'account_id')
		ORDER BY c.table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []exportTable
	for rows.Next() {
		var t exportTable
		var cols []string
		if err := rows.Scan(&t.name, &cols); err != nil {
			return nil, err
		}
		if exportSkipTables[t.name] {
			continue
		}
		for _, c := range cols {
			if !sensitiveColumn(c) {
				t.cols = append(t.cols, c)
			}
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (t exportTable) selectSQL() string {
	quoted := make([]string, len(t.cols))
	for i, c := range t.cols {
		quoted[i] = pgx.Identifier{c}.Sanitize()
	}
	return fmt.Sprintf("SELECT %s FROM %s WHERE account_id=$1", strings.Join(quoted, ", "), pgx.Identifier{t.name}.Sanitize())
}

// GET /api/export/all.json - every exportable table, keyed by table name.
func (app *App) handleExportAllJSON(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	tables, err := app.exportTables(ctx)
	if err != nil {
		dbError(w, err)
		return
	}

	out := map[string]json.RawMessage{}
	for _, t := range tables {
		q := "SELECT COALESCE(json_agg(x), '[]'::json) FROM (" + t.selectSQL() + ") x"
		var raw json.RawMessage
		if err := app.DB.QueryRow(ctx, q, aid).Scan(&raw); err != nil {
			dbError(w, err)
			return
		}
		out[t.name] = raw
	}

	var profile json.RawMessage
	if err := app.DB.QueryRow(ctx,
		"SELECT json_build_object('id', id, 'email', email, 'display_name', display_name, 'currency', currency, 'locale', locale, 'created_at', created_at) FROM users WHERE id=$1",
		aid).Scan(&profile); err != nil {
		dbError(w, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="kinkit-export-`+time.Now().UTC().Format("2006-01-02")+`.json"`)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"format":      "kinkit-export-v1",
		"exported_at": time.Now().UTC().Format(time.RFC3339),
		"account":     profile,
		"tables":      out,
	})
}

// accountSheet is the "Account" sheet: one field/value row per profile column.
func (app *App) accountSheet(ctx context.Context, aid int64) (xlsxSheet, error) {
	rows, err := app.DB.Query(ctx,
		"SELECT id, email, display_name, currency, locale, created_at FROM users WHERE id=$1", aid)
	if err != nil {
		return xlsxSheet{}, err
	}
	names := fieldNames(rows)
	data, err := collectValues(rows)
	if err != nil {
		return xlsxSheet{}, err
	}
	if len(data) == 0 {
		return xlsxSheet{}, pgx.ErrNoRows
	}
	sheet := xlsxSheet{Name: "Account", Header: []string{"field", "value"}}
	for i, name := range names {
		sheet.Rows = append(sheet.Rows, []any{name, data[0][i]})
	}
	return sheet, nil
}

// GET /api/export/all.xlsx - the all.json content as a workbook: an
// "Account" sheet, then one sheet per table.
func (app *App) handleExportAllXLSX(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	tables, err := app.exportTables(ctx)
	if err != nil {
		dbError(w, err)
		return
	}
	account, err := app.accountSheet(ctx, aid)
	if err != nil {
		dbError(w, err)
		return
	}
	sheets := []xlsxSheet{account}
	for _, t := range tables {
		rows, err := app.DB.Query(ctx, t.selectSQL(), aid)
		if err != nil {
			dbError(w, err)
			return
		}
		data, err := collectValues(rows)
		if err != nil {
			dbError(w, err)
			return
		}
		sheets = append(sheets, xlsxSheet{Name: t.name, Header: t.cols, Rows: data})
	}
	writeXLSXResponse(w, "kinkit-export-"+time.Now().UTC().Format("2006-01-02")+".xlsx", sheets)
}

// import

const maxImportBytes = 5 << 20

func (app *App) handleImportPresets(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.requireAccount(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, importPresets)
}

type rowOverride struct {
	Category string `json:"category"`
	Skip     bool   `json:"skip"`
}

// categoryResolver maps a CSV row to one of the account's categories.
type categoryResolver struct {
	byLower  map[string]string // lower(name) -> name
	ids      map[string]int64  // name -> id
	keywords [][2]string       // lower(keyword), category
	userMap  map[string]string // lower(csv category) -> chosen category
}

func (app *App) loadCategoryResolver(ctx context.Context, aid int64, userMap map[string]string) (*categoryResolver, error) {
	cr := &categoryResolver{byLower: map[string]string{}, ids: map[string]int64{}, userMap: map[string]string{}}
	rows, err := app.DB.Query(ctx, "SELECT id, name FROM categories WHERE account_id=$1", aid)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		cr.byLower[strings.ToLower(name)] = name
		cr.ids[name] = id
	}
	rows.Close()
	rows, err = app.DB.Query(ctx, "SELECT keyword, category FROM category_keywords WHERE account_id=$1 ORDER BY length(keyword) DESC", aid)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var k, c string
		if err := rows.Scan(&k, &c); err != nil {
			rows.Close()
			return nil, err
		}
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			cr.keywords = append(cr.keywords, [2]string{k, c})
		}
	}
	rows.Close()
	for k, v := range userMap {
		cr.userMap[strings.ToLower(strings.TrimSpace(k))] = v
	}
	return cr, nil
}

// known returns the canonical category name, or "" if it isn't one of ours.
func (cr *categoryResolver) known(name string) string {
	return cr.byLower[strings.ToLower(strings.TrimSpace(name))]
}

func (cr *categoryResolver) resolve(sourceCat, desc string) string {
	if sourceCat != "" {
		if v, ok := cr.userMap[strings.ToLower(strings.TrimSpace(sourceCat))]; ok && v != "" {
			if k := cr.known(v); k != "" {
				return k
			}
			return v
		}
		if k := cr.known(sourceCat); k != "" {
			return k
		}
	}
	ld := strings.ToLower(desc)
	for _, kw := range cr.keywords {
		if strings.Contains(ld, kw[0]) {
			if k := cr.known(kw[1]); k != "" {
				return k
			}
		}
	}
	return "Other"
}

func formJSON(r *http.Request, field string, v any) error {
	s := r.FormValue(field)
	if s == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(s), v); err != nil {
		return fmt.Errorf("invalid %s: %v", field, err)
	}
	return nil
}

// POST /api/import/spending[?dry_run=1]  multipart:
//
//	file          CSV file (max 5 MB, 5000 rows)
//	options       JSON importOptions {preset, mapping:{date:0,...}, date_order, decimal, default_year}
//	category_map  JSON {"<csv category>": "<Kinkit category>"}
//	overrides     JSON {"<row>": {"category": "...", "skip": true}}
//
// dry_run returns the parsed rows with status ok|skip|error|duplicate,
// the detected mapping and the account's categories, writing nothing.
// Without dry_run the same parse runs and the "ok" rows are inserted in
// one transaction (dedupe: client_id + ON CONFLICT DO NOTHING).
func (app *App) handleImportSpending(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	dryRun := r.URL.Query().Get("dry_run") == "1" || r.URL.Query().Get("dry_run") == "true"
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes+256<<10)
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		jsonError(w, http.StatusRequestEntityTooLarge, "upload too large or malformed (max 5 MB)")
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	f, _, err := r.FormFile("file")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "missing file")
		return
	}
	data, err := io.ReadAll(io.LimitReader(f, maxImportBytes+1))
	f.Close()
	if err != nil || len(data) > maxImportBytes {
		jsonError(w, http.StatusRequestEntityTooLarge, "file too large (max 5 MB)")
		return
	}
	var opt importOptions
	catMap := map[string]string{}
	overrides := map[string]rowOverride{}
	for field, dst := range map[string]any{"options": &opt, "category_map": &catMap, "overrides": &overrides} {
		if err := formJSON(r, field, dst); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	res, err := parseImport(data, opt)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := r.Context()
	cr, err := app.loadCategoryResolver(ctx, aid, catMap)
	if err != nil {
		dbError(w, err)
		return
	}

	unknown := map[string]bool{}
	ids := []string{}
	for i := range res.Rows {
		pr := &res.Rows[i]
		if pr.SourceCategory != "" && cr.known(pr.SourceCategory) == "" {
			unknown[pr.SourceCategory] = true
		}
		if pr.Status != "ok" {
			continue
		}
		pr.Category = cr.resolve(pr.SourceCategory, pr.Description)
		if ov, ok := overrides[strconv.Itoa(pr.Row)]; ok {
			if ov.Skip {
				pr.Status, pr.Reason = "skip", "skipped by you"
			} else if ov.Category != "" {
				pr.Category = ov.Category
				if k := cr.known(ov.Category); k != "" {
					pr.Category = k
				}
			}
		}
		if pr.Status == "ok" {
			ids = append(ids, pr.ClientID)
		}
	}
	existing := map[string]bool{}
	if len(ids) > 0 {
		rows, err := app.DB.Query(ctx, "SELECT client_id FROM spending WHERE account_id=$1 AND client_id = ANY($2)", aid, ids)
		if err != nil {
			dbError(w, err)
			return
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				existing[id] = true
			}
		}
		rows.Close()
	}
	summary := map[string]int{"total": len(res.Rows), "ok": 0, "skip": 0, "error": 0, "duplicate": 0}
	for i := range res.Rows {
		pr := &res.Rows[i]
		if pr.Status == "ok" && existing[pr.ClientID] {
			pr.Status, pr.Reason = "duplicate", "already imported"
		}
		summary[pr.Status]++
	}
	unknownList := []string{}
	for k := range unknown {
		unknownList = append(unknownList, k)
	}
	categories := []string{}
	for name := range cr.ids {
		categories = append(categories, name)
	}

	if dryRun {
		writeJSON(w, http.StatusOK, map[string]any{
			"dry_run": true, "preset": res.Preset, "delimiter": res.Delimiter, "header_line": res.HeaderRow,
			"headers": res.Headers, "mapping": res.Mapping, "date_order": res.DateOrder, "decimal": res.Decimal,
			"rows": res.Rows, "errors": res.Errors, "summary": summary,
			"unknown_categories": unknownList, "categories": categories, "presets": importPresets,
		})
		return
	}
	if summary["ok"] == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "imported": 0, "summary": summary, "errors": res.Errors})
		return
	}

	batch := time.Now().UTC().Format("20060102-150405") + "-" + randomHex(4)
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	imported, dupes := 0, 0
	var total int64
	for _, pr := range res.Rows {
		if pr.Status != "ok" {
			continue
		}
		var catID *int64
		if id, ok := cr.ids[pr.Category]; ok {
			catID = &id
		}
		tag, err := tx.Exec(ctx,
			`INSERT INTO spending (account_id, tx_date, description, category_id, category_name, amount, source, client_id, import_batch)
			 VALUES ($1,$2,$3,$4,$5,$6,'import',$7,$8) ON CONFLICT (account_id, client_id) DO NOTHING`,
			aid, pr.TxDate, pr.Description, catID, pr.Category, pr.Amount, pr.ClientID, batch)
		if err != nil {
			dbError(w, err)
			return
		}
		if tag.RowsAffected() == 1 {
			imported++
			total += pr.Amount
		} else {
			dupes++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		dbError(w, err)
		return
	}
	summary["duplicate"] += dupes
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "imported": imported, "total_amount": total, "batch": batch,
		"summary": summary, "errors": res.Errors,
	})
}

// GET /api/import/batches - recent imports (for "undo import").
func (app *App) handleListImportBatches(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	rows, err := app.DB.Query(r.Context(),
		`SELECT import_batch, COUNT(*), COALESCE(SUM(amount),0), MIN(tx_date)::text, MAX(tx_date)::text, MIN(created_at)
		 FROM spending WHERE account_id=$1 AND import_batch IS NOT NULL
		 GROUP BY import_batch ORDER BY MIN(created_at) DESC LIMIT 20`, aid)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()
	type batchRow struct {
		Batch      string `json:"batch"`
		Count      int64  `json:"count"`
		Total      int64  `json:"total"`
		From       string `json:"from"`
		To         string `json:"to"`
		ImportedAt string `json:"imported_at"`
	}
	out := []batchRow{}
	for rows.Next() {
		var b batchRow
		var at time.Time
		if err := rows.Scan(&b.Batch, &b.Count, &b.Total, &b.From, &b.To, &at); err != nil {
			dbError(w, err)
			return
		}
		b.ImportedAt = at.UTC().Format(time.RFC3339)
		out = append(out, b)
	}
	writeJSON(w, http.StatusOK, out)
}

// DELETE /api/import/batches/{batch} - removes every row of one import.
func (app *App) handleUndoImport(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	batch := chi.URLParam(r, "batch")
	if batch == "" {
		jsonError(w, http.StatusBadRequest, "missing batch")
		return
	}
	rows, err := app.DB.Query(r.Context(),
		"DELETE FROM spending WHERE account_id=$1 AND import_batch=$2 RETURNING receipt_path", aid, batch)
	if err != nil {
		dbError(w, err)
		return
	}
	n := 0
	for rows.Next() {
		var p *string
		if rows.Scan(&p) == nil {
			removeReceiptFile(p)
		}
		n++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": n})
}
