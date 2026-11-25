package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

// upload posts a multipart form (one file + text fields) and decodes JSON.
func (tc *testClient) upload(path, filename string, content []byte, fields map[string]string, out any) int {
	tc.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write(content)
	_ = mw.Close()
	req, _ := http.NewRequest("POST", tc.base+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := tc.c.Do(req)
	if err != nil {
		tc.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			tc.t.Fatalf("POST %s: decode %q: %v", path, raw, err)
		}
	}
	return resp.StatusCode
}

// raw GET returning status, content type and body.
func (tc *testClient) get(path string) (int, string, []byte) {
	tc.t.Helper()
	resp, err := tc.c.Get(tc.base + path)
	if err != nil {
		tc.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("Content-Type"), b
}

func addCategory(t *testing.T, app *App, aid int64, name, main string) {
	t.Helper()
	if _, err := app.DB.Exec(context.Background(),
		"INSERT INTO categories (account_id, name, main_category) VALUES ($1,$2,$3) ON CONFLICT (account_id, name) DO NOTHING", aid, name, main); err != nil {
		t.Fatal(err)
	}
}

type importResp struct {
	DryRun   bool           `json:"dry_run"`
	Rows     []parsedRow    `json:"rows"`
	Summary  map[string]int `json:"summary"`
	Imported int            `json:"imported"`
	Batch    string         `json:"batch"`
	Unknown  []string       `json:"unknown_categories"`
	Error    string         `json:"error"`
}

func TestImportDryRunCommitDedupeUndo(t *testing.T) {
	app, srv := newTestServer(t)
	c := newClient(t, srv)
	_, aid := c.signup()
	addCategory(t, app, aid, "Coffee Shops", "Wants")
	addCategory(t, app, aid, "Groceries", "Needs")

	csv := []byte("Date,Description,Amount,Category\n" +
		"2026-02-01,Kopi Kenangan,-35000,coffee shops\n" +
		"2026-02-02,Salary,9000000,\n" +
		"2026-02-03,Supermarket,-250.000,Belanja\n" +
		"2026-02-04,Parking,-5000,\n")
	fields := map[string]string{
		"options":      `{"preset":"generic"}`,
		"category_map": `{"Belanja":"Groceries"}`,
		"overrides":    `{"4":{"skip":true}}`,
	}
	count := func() int {
		var n int
		_ = app.DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM spending WHERE account_id=$1", aid).Scan(&n)
		return n
	}

	var dry importResp
	if code := c.upload("/api/import/spending?dry_run=1", "s.csv", csv, fields, &dry); code != 200 || !dry.DryRun {
		t.Fatalf("dry run: %d %+v", code, dry)
	}
	if dry.Summary["ok"] != 2 || dry.Summary["skip"] != 2 {
		t.Fatalf("dry summary %v rows %+v", dry.Summary, dry.Rows)
	}
	if dry.Rows[0].Category != "Coffee Shops" || dry.Rows[2].Category != "Groceries" {
		t.Fatalf("category resolution: %+v", dry.Rows)
	}
	if len(dry.Unknown) != 1 || dry.Unknown[0] != "Belanja" {
		t.Fatalf("unknown categories %v", dry.Unknown)
	}
	if count() != 0 {
		t.Fatal("dry run wrote rows")
	}

	var commit importResp
	if code := c.upload("/api/import/spending", "s.csv", csv, fields, &commit); code != 200 || commit.Imported != 2 || commit.Batch == "" {
		t.Fatalf("commit: %d %+v", code, commit)
	}
	if count() != 2 {
		t.Fatalf("want 2 rows, got %d", count())
	}
	var catID *int64
	_ = app.DB.QueryRow(context.Background(), "SELECT category_id FROM spending WHERE account_id=$1 AND description='Supermarket'", aid).Scan(&catID)
	if catID == nil {
		t.Fatal("imported row not linked to category id")
	}

	// same file again: nothing new, reported as duplicates
	var again importResp
	c.upload("/api/import/spending", "s.csv", csv, fields, &again)
	if again.Imported != 0 || again.Summary["duplicate"] != 2 || count() != 2 {
		t.Fatalf("re-import: %+v (count %d)", again, count())
	}
	var dry2 importResp
	c.upload("/api/import/spending?dry_run=1", "s.csv", csv, fields, &dry2)
	if dry2.Rows[0].Status != "duplicate" {
		t.Fatalf("dry run after import should flag duplicates: %+v", dry2.Rows[0])
	}

	// another account can't undo it
	c2 := newClient(t, srv)
	c2.signup()
	var undo map[string]any
	c2.do("DELETE", "/api/import/batches/"+commit.Batch, nil, &undo)
	if undo["deleted"] != float64(0) || count() != 2 {
		t.Fatalf("cross-account undo: %v", undo)
	}
	var batches []map[string]any
	c.do("GET", "/api/import/batches", nil, &batches)
	if len(batches) != 1 || batches[0]["count"] != float64(2) {
		t.Fatalf("batches %v", batches)
	}
	c.do("DELETE", "/api/import/batches/"+commit.Batch, nil, &undo)
	if undo["deleted"] != float64(2) || count() != 0 {
		t.Fatalf("undo: %v count %d", undo, count())
	}
}

func TestExports(t *testing.T) {
	app, srv := newTestServer(t)
	c := newClient(t, srv)
	_, aid := c.signup()
	c.do("POST", "/api/spending", map[string]any{"tx_date": "2026-01-10", "description": "=cmd|' /C calc'!A0", "amount": 1000}, nil)
	c.do("POST", "/api/spending", map[string]any{"tx_date": "2026-03-10", "description": "Lunch", "amount": 2000}, nil)
	if _, err := app.DB.Exec(context.Background(),
		"INSERT INTO integrations (account_id, platform, bot_token) VALUES ($1,'telegram',$2)", aid, "secret-"+uniqueEmail()); err != nil {
		t.Fatal(err)
	}

	code, ctype, body := c.get("/api/export/spending.csv?from=2026-03-01&to=2026-03-31")
	if code != 200 || !strings.HasPrefix(ctype, "text/csv") {
		t.Fatalf("csv: %d %s", code, ctype)
	}
	if !strings.Contains(string(body), "Lunch") || strings.Contains(string(body), "calc") {
		t.Fatalf("date filter: %s", body)
	}
	_, _, all := c.get("/api/export/spending.csv")
	if !strings.Contains(string(all), "'=cmd") {
		t.Fatalf("formula not neutralized: %s", all)
	}
	if code, _, _ := c.get("/api/export/spending.csv?from=bad"); code != 400 {
		t.Fatalf("bad date: %d", code)
	}
	if code, _, _ := c.get("/api/export/income.csv"); code != 200 {
		t.Fatalf("income csv %d", code)
	}

	const xlsxType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	flatten := func(parts map[string][]byte) string {
		var sb strings.Builder
		for _, b := range parts {
			sb.Write(b)
		}
		return sb.String()
	}
	code, ctype, xbody := c.get("/api/export/spending.xlsx?from=2026-03-01&to=2026-03-31")
	if code != 200 || !strings.HasPrefix(ctype, xlsxType) {
		t.Fatalf("spending.xlsx: %d %s", code, ctype)
	}
	xs := parseXLSXSheet(t, unzipXLSX(t, xbody), "xl/worksheets/sheet1.xml")
	xtext := flatten(unzipXLSX(t, xbody))
	if len(xs.Rows) != 2 || !strings.Contains(xtext, "Lunch") || strings.Contains(xtext, "calc") {
		t.Fatalf("spending.xlsx date filter: %d rows", len(xs.Rows))
	}
	if cell := xs.Rows[1].Cells[5]; cell.Type != "n" || cell.Value != "2000" {
		t.Fatalf("amount should be numeric: %+v", cell)
	}
	if cell := xs.Rows[1].Cells[6]; cell.Type != "b" || cell.Value != "0" {
		t.Fatalf("checked should be boolean: %+v", cell)
	}
	_, _, xall := c.get("/api/export/spending.xlsx")
	if !strings.Contains(flatten(unzipXLSX(t, xall)), "=cmd") {
		t.Fatal("full spending.xlsx lost the =cmd text")
	}
	if code, _, body := c.get("/api/export/spending.xlsx?from=bad"); code != 400 || !strings.Contains(string(body), "from must be YYYY-MM-DD") {
		t.Fatalf("bad date xlsx: %d %s", code, body)
	}
	if code, ctype, ibody := c.get("/api/export/income.xlsx"); code != 200 || !strings.HasPrefix(ctype, xlsxType) {
		t.Fatalf("income.xlsx: %d %s", code, ctype)
	} else if _, ok := unzipXLSX(t, ibody)["xl/worksheets/sheet1.xml"]; !ok {
		t.Fatal("income.xlsx has no sheet")
	}
	code, ctype, abody := c.get("/api/export/all.xlsx")
	if code != 200 || !strings.HasPrefix(ctype, xlsxType) {
		t.Fatalf("all.xlsx: %d %s", code, ctype)
	}
	aparts := unzipXLSX(t, abody)
	if wb := string(aparts["xl/workbook.xml"]); !strings.Contains(wb, `name="Account"`) || !strings.Contains(wb, `name="spending"`) || strings.Contains(wb, `name="integrations"`) {
		t.Fatalf("all.xlsx sheets: %s", wb)
	}
	if strings.Contains(flatten(aparts), "secret-") {
		t.Fatal("all.xlsx leaked secrets")
	}

	code, _, raw := c.get("/api/export/all.json")
	if code != 200 {
		t.Fatalf("all.json %d", code)
	}
	var dump struct {
		Format  string                       `json:"format"`
		Account map[string]any               `json:"account"`
		Tables  map[string][]json.RawMessage `json:"tables"`
	}
	if err := json.Unmarshal(raw, &dump); err != nil {
		t.Fatal(err)
	}
	if dump.Format != "kinkit-export-v1" || len(dump.Tables["spending"]) != 2 || len(dump.Tables["categories"]) == 0 {
		t.Fatalf("dump: format=%s tables=%v", dump.Format, len(dump.Tables))
	}
	if _, ok := dump.Tables["integrations"]; ok || strings.Contains(string(raw), "secret-") || strings.Contains(string(raw), "password") {
		t.Fatal("export leaked secrets")
	}

	anon := newClient(t, srv)
	for _, path := range []string{"all.json", "all.xlsx", "spending.xlsx", "income.xlsx"} {
		if code, _, _ := anon.get("/api/export/" + path); code != 401 {
			t.Fatalf("anon export %s: %d", path, code)
		}
	}
}

var tinyPNG = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\xff\xff?\x00\x05\xfe\x02\xfe\xa7\x35\x81\x84\x00\x00\x00\x00IEND\xaeB`\x82")

func TestReceiptUploadServeAuth(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	_, srv := newTestServer(t)
	c := newClient(t, srv)
	c.signup()
	var add map[string]any
	c.do("POST", "/api/spending", map[string]any{"tx_date": "2026-04-01", "description": "Dinner", "amount": 90000}, &add)
	id := int64(add["id"].(float64))
	path := "/api/spending/" + jsonInt(id) + "/receipt"

	var res map[string]any
	if code := c.upload(path, "evil.png", []byte("<html>nope</html>"), nil, &res); code != http.StatusUnsupportedMediaType {
		t.Fatalf("non-image accepted: %d %v", code, res)
	}
	if code := c.upload(path, "r.png", tinyPNG, nil, &res); code != 200 || res["has_receipt"] != true {
		t.Fatalf("upload: %d %v", code, res)
	}
	code, ctype, body := c.get(path)
	if code != 200 || ctype != "image/png" || !bytes.Equal(body, tinyPNG) {
		t.Fatalf("serve: %d %s %d bytes", code, ctype, len(body))
	}
	var list struct {
		Rows []spendingRow `json:"rows"`
	}
	c.do("GET", "/api/spending?year=2026&month=4", nil, &list)
	if len(list.Rows) != 1 || !list.Rows[0].HasReceipt {
		t.Fatalf("list has_receipt: %+v", list.Rows)
	}

	// another household can't read, replace or delete it
	other := newClient(t, srv)
	other.signup()
	if code, _, _ := other.get(path); code != 404 {
		t.Fatalf("other account read receipt: %d", code)
	}
	if code := other.upload(path, "r.png", tinyPNG, nil, nil); code != 404 {
		t.Fatalf("other account replaced receipt: %d", code)
	}
	if code := other.do("DELETE", path, nil, nil); code != 404 {
		t.Fatalf("other account deleted receipt: %d", code)
	}
	if code, _, _ := newClient(t, srv).get(path); code != 401 {
		t.Fatalf("anon read receipt: %d", code)
	}

	if code := c.do("DELETE", path, nil, &res); code != 200 || res["has_receipt"] != false {
		t.Fatalf("delete: %d %v", code, res)
	}
	if code, _, _ := c.get(path); code != 404 {
		t.Fatalf("deleted receipt still served: %d", code)
	}
}

func jsonInt(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestDashboardLayoutRoundTrip(t *testing.T) {
	_, srv := newTestServer(t)
	c := newClient(t, srv)
	c.signup()
	var got map[string]any
	c.do("GET", "/api/dashboard-layout", nil, &got)
	if got["layout"] != nil {
		t.Fatalf("fresh account layout: %v", got)
	}
	layout := []map[string]any{{"id": "w1", "source": "year_trend", "chart": "bar"}, {"id": "w2", "source": "assets_totals", "chart": "pie"}}
	if code := c.do("PUT", "/api/dashboard-layout", map[string]any{"layout": layout}, nil); code != 200 {
		t.Fatalf("put %d", code)
	}
	layout = layout[:1]
	c.do("PUT", "/api/dashboard-layout", map[string]any{"layout": layout}, nil) // upsert
	var back struct {
		Layout    []map[string]any `json:"layout"`
		UpdatedAt string           `json:"updated_at"`
	}
	c.do("GET", "/api/dashboard-layout", nil, &back)
	if len(back.Layout) != 1 || back.Layout[0]["source"] != "year_trend" || back.UpdatedAt == "" {
		t.Fatalf("round trip: %+v", back)
	}
	if code := c.do("PUT", "/api/dashboard-layout", map[string]any{"layout": "nope"}, nil); code != 400 {
		t.Fatalf("string layout accepted: %d", code)
	}
	other := newClient(t, srv)
	other.signup()
	var o map[string]any
	other.do("GET", "/api/dashboard-layout", nil, &o)
	if o["layout"] != nil {
		t.Fatalf("layout leaked across accounts: %v", o)
	}
}

func TestTVTokenAccess(t *testing.T) {
	_, srv := newTestServer(t)
	c := newClient(t, srv)
	c.signup()
	c.do("POST", "/api/spending", map[string]any{"tx_date": todayUTC().Format("2006-01-02"), "description": "Bread", "amount": 15000}, nil)

	var tok tvTokenRow
	if code := c.do("POST", "/api/tv-tokens", map[string]any{"name": "Living room"}, &tok); code != 200 || tok.Token == "" {
		t.Fatalf("create: %d %+v", code, tok)
	}
	screen := newClient(t, srv) // no session
	var data struct {
		Year          int              `json:"year"`
		Month         int              `json:"month"`
		HouseholdName *string          `json:"household_name"`
		Report        map[string]any   `json:"report"`
		Trend         []map[string]any `json:"trend"`
	}
	if code := screen.do("GET", "/api/tv/data?k="+tok.Token, nil, &data); code != 200 {
		t.Fatalf("tv data: %d", code)
	}
	if len(data.Trend) != 6 || data.Report["expenses_list"] == nil || data.Trend[5]["expense"] != float64(15000) {
		t.Fatalf("tv data shape: %+v", data)
	}
	if data.HouseholdName == nil {
		t.Fatalf("tv data: household_name missing")
	}
	// Packaged TV viewers (other origins) may read the public endpoints, never session ones.
	for path, want := range map[string]string{"/api/tv/data?k=" + tok.Token: "*", "/api/fx-rates": "*", "/api/me": ""} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != want {
			t.Fatalf("%s: Access-Control-Allow-Origin=%q, want %q", path, got, want)
		}
	}
	if code := screen.do("GET", "/api/tv/data?k=wrong", nil, nil); code != 401 {
		t.Fatalf("bad token: %d", code)
	}
	if code := screen.do("GET", "/api/tv-tokens", nil, nil); code != 401 {
		t.Fatalf("token list without session: %d", code)
	}

	var list []tvTokenRow
	c.do("GET", "/api/tv-tokens", nil, &list)
	if len(list) != 1 || list[0].LastUsedAt == nil {
		t.Fatalf("list: %+v", list)
	}
	other := newClient(t, srv)
	other.signup()
	if code := other.do("DELETE", "/api/tv-tokens/"+jsonInt(tok.ID), nil, nil); code != 404 {
		t.Fatalf("other account revoked token: %d", code)
	}
	if code := c.do("DELETE", "/api/tv-tokens/"+jsonInt(tok.ID), nil, nil); code != 200 {
		t.Fatalf("revoke: %d", code)
	}
	if code := screen.do("GET", "/api/tv/data?k="+tok.Token, nil, nil); code != 401 {
		t.Fatalf("revoked token still works: %d", code)
	}
}
