package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CSV import: pure parsing (no DB)
//
// Pipeline: decode CSV (delimiter auto-detected) -> find the header row ->
// resolve column mapping (user choice > preset header names) -> parse each
// row into {date, description, amount, category}. Only money going OUT
// (expenses) becomes spending; money coming in is reported as skipped.

// importPreset describes a bank statement layout by header names, so a
// format change is a one-line data fix. Header matching is case-insensitive
// and ignores surrounding spaces/punctuation; the first candidate found wins.
type importPreset struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Note        string   `json:"note,omitempty"`
	Date        []string `json:"date"`
	Description []string `json:"description"`
	Amount      []string `json:"amount,omitempty"` // single signed amount column
	Debit       []string `json:"debit,omitempty"`  // money out
	Credit      []string `json:"credit,omitempty"` // money in
	DrCr        []string `json:"drcr,omitempty"`   // separate "DB"/"CR" marker column
	Category    []string `json:"category,omitempty"`
	DateOrder   string   `json:"date_order"` // dmy | mdy | ymd
	Decimal     string   `json:"decimal"`    // "." | "," | "" (auto)
}

var importPresets = []importPreset{
	{
		ID: "generic", Label: "Generic (date, description, amount)",
		Note:        "One amount column. If some amounts are negative, negatives are expenses and positives are skipped as income.",
		Date:        []string{"date", "tanggal", "transaction date", "tgl", "posted date", "booking date"},
		Description: []string{"description", "keterangan", "memo", "details", "narrative", "payee", "name", "uraian"},
		Amount:      []string{"amount", "jumlah", "nominal", "value", "total"},
		Category:    []string{"category", "kategori"},
		DateOrder:   "dmy",
	},
	{
		ID: "debit_credit", Label: "Generic debit / credit columns",
		Note:        "Separate money-out and money-in columns. Only the debit (money out) rows are imported.",
		Date:        []string{"date", "tanggal", "transaction date", "posting date", "tgl", "value date"},
		Description: []string{"description", "keterangan", "memo", "details", "narrative", "remarks", "uraian"},
		Debit:       []string{"debit", "debet", "withdrawal", "withdrawals", "money out", "paid out", "out", "mutasi debet"},
		Credit:      []string{"credit", "kredit", "deposit", "deposits", "money in", "paid in", "in", "mutasi kredit"},
		Category:    []string{"category", "kategori"},
		DateOrder:   "dmy",
	},
	{
		ID: "bca", Label: "BCA (KlikBCA / myBCA statement CSV)",
		Note:        "Amounts look like \"50,000.00 DB\" (DB = money out, CR = money in). Dates without a year use the year you choose.",
		Date:        []string{"tanggal transaksi", "tanggal", "date"},
		Description: []string{"keterangan", "description"},
		Amount:      []string{"jumlah", "mutasi", "amount"},
		DrCr:        []string{"db/cr", "dk", "d/k", "jenis"},
		DateOrder:   "dmy",
		Decimal:     ".",
	},
	{
		ID: "mandiri", Label: "Mandiri (Livin' / internet banking CSV)",
		Note:        "Separate Debit/Kredit columns, amounts like 1.250.000,00.",
		Date:        []string{"tanggal", "date", "posting date", "tanggal transaksi"},
		Description: []string{"keterangan", "description", "remarks", "uraian transaksi"},
		Debit:       []string{"debit", "debet"},
		Credit:      []string{"kredit", "credit"},
		DateOrder:   "dmy",
		Decimal:     "",
	},
}

func findPreset(id string) importPreset {
	for _, p := range importPresets {
		if p.ID == id {
			return p
		}
	}
	return importPresets[0]
}

// importMapping is the resolved column index per field (-1 = not used).
type importMapping struct {
	Date        int `json:"date"`
	Description int `json:"description"`
	Amount      int `json:"amount"`
	Debit       int `json:"debit"`
	Credit      int `json:"credit"`
	DrCr        int `json:"drcr"`
	Category    int `json:"category"`
}

func emptyMapping() importMapping {
	return importMapping{-1, -1, -1, -1, -1, -1, -1}
}

// importOptions are the user's choices on the wizard's mapping step.
type importOptions struct {
	Preset       string         `json:"preset"`
	Mapping      map[string]int `json:"mapping"`    // field -> column index; overrides the preset
	DateOrder    string         `json:"date_order"` // dmy | mdy | ymd (default from preset)
	Decimal      string         `json:"decimal"`    // "." | "," | "" auto
	DefaultYear  int            `json:"default_year"`
	NegativeOnly *bool          `json:"negative_is_expense"` // single amount column: nil = auto
}

type parsedRow struct {
	Row            int    `json:"row"` // 1-based data row number (after the header)
	Line           int    `json:"line"`
	TxDate         string `json:"tx_date"`
	Description    string `json:"description"`
	Amount         int64  `json:"amount"`
	SourceCategory string `json:"source_category"`
	Category       string `json:"category"`
	Status         string `json:"status"` // ok | skip | error | duplicate
	Reason         string `json:"reason,omitempty"`
	ClientID       string `json:"client_id,omitempty"`
}

type parseResult struct {
	Preset    string        `json:"preset"`
	Delimiter string        `json:"delimiter"`
	HeaderRow int           `json:"header_line"`
	Headers   []string      `json:"headers"`
	Mapping   importMapping `json:"mapping"`
	DateOrder string        `json:"date_order"`
	Decimal   string        `json:"decimal"`
	Rows      []parsedRow   `json:"rows"`
	Errors    []string      `json:"errors"`
}

const maxImportRows = 5000

// detectDelimiter picks the most frequent of , ; \t | on the first lines.
func detectDelimiter(data []byte) rune {
	lines := strings.SplitN(string(data), "\n", 12)
	best, bestN := ',', 0
	for _, d := range []rune{',', ';', '\t', '|'} {
		n := 0
		for _, l := range lines {
			n += strings.Count(l, string(d))
		}
		if n > bestN {
			best, bestN = d, n
		}
	}
	return best
}

// readCSV returns the records and each record's 1-based line number in the
// file (the csv reader skips blank lines).
func readCSV(data []byte) ([][]string, []int, rune, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // UTF-8 BOM
	delim := detectDelimiter(data)
	rd := csv.NewReader(bytes.NewReader(data))
	rd.Comma = delim
	rd.FieldsPerRecord = -1
	rd.LazyQuotes = true
	rd.TrimLeadingSpace = true
	var out [][]string
	var lines []int
	for {
		rec, err := rd.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return out, lines, delim, err
		}
		line, _ := rd.FieldPos(0)
		for i := range rec {
			rec[i] = cleanCell(rec[i])
		}
		out = append(out, rec)
		lines = append(lines, line)
	}
	return out, lines, delim, nil
}

// cleanCell trims spaces and the leading apostrophe some banks (BCA) use
// to force Excel to keep a cell as text.
func cleanCell(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "'")
	return strings.TrimSpace(s)
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9/]+`)

func normHeader(s string) string {
	return strings.TrimSpace(nonAlnum.ReplaceAllString(strings.ToLower(s), " "))
}

func findColumn(headers []string, candidates []string, taken map[int]bool) int {
	norm := make([]string, len(headers))
	for i, h := range headers {
		norm[i] = normHeader(h)
	}
	for _, c := range candidates {
		nc := normHeader(c)
		for i, h := range norm {
			if h == nc && !taken[i] {
				return i
			}
		}
	}
	return -1
}

// findHeaderRow returns the index of the first row that contains a
// date-like header for the preset (bank exports often start with account
// info lines). Falls back to the first row with >= 2 non-empty cells.
func findHeaderRow(records [][]string, p importPreset) int {
	limit := len(records)
	if limit > 30 {
		limit = 30
	}
	for i := 0; i < limit; i++ {
		if findColumn(records[i], p.Date, nil) >= 0 {
			return i
		}
	}
	for i := 0; i < limit; i++ {
		n := 0
		for _, c := range records[i] {
			if c != "" {
				n++
			}
		}
		if n >= 2 {
			return i
		}
	}
	return 0
}

func resolveMapping(headers []string, p importPreset, user map[string]int) importMapping {
	m := emptyMapping()
	taken := map[int]bool{}
	set := func(dst *int, field string, cands []string) {
		if v, ok := user[field]; ok {
			*dst = v
			if v >= 0 {
				taken[v] = true
			}
			return
		}
		if len(cands) == 0 {
			return
		}
		if i := findColumn(headers, cands, taken); i >= 0 {
			*dst = i
			taken[i] = true
		}
	}
	set(&m.Date, "date", p.Date)
	set(&m.Description, "description", p.Description)
	set(&m.Debit, "debit", p.Debit)
	set(&m.Credit, "credit", p.Credit)
	set(&m.Amount, "amount", p.Amount)
	set(&m.DrCr, "drcr", p.DrCr)
	set(&m.Category, "category", p.Category)
	// BCA: the DB/CR marker is often an unnamed column right after "Jumlah".
	if p.ID == "bca" && m.DrCr < 0 && m.Amount >= 0 && m.Amount+1 < len(headers) && strings.TrimSpace(headers[m.Amount+1]) == "" {
		m.DrCr = m.Amount + 1
	}
	return m
}

// amounts

var amountJunk = regexp.MustCompile(`(?i)(rp\.?|idr|usd|eur|sgd|myr|[$€£¥₹]|\s)`)

// parseAmount parses "1.234.567", "1,234,567.89", "1.234,56", "(500)",
// "-12,5", "50,000.00 DB", "25.000 CR". decimal is "." / "," / "" (auto).
// Returns the absolute value rounded to whole units and whether it is
// negative / a debit (money out per the DB/CR suffix or sign).
func parseAmount(s, decimal string) (abs int64, negative bool, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false, false
	}
	up := strings.ToUpper(s)
	switch {
	case strings.HasSuffix(up, "DB"), strings.HasSuffix(up, "DR"):
		negative = true
		s = strings.TrimSpace(s[:len(s)-2])
	case strings.HasSuffix(up, "CR"):
		s = strings.TrimSpace(s[:len(s)-2])
	}
	s = amountJunk.ReplaceAllString(s, "")
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		negative = true
		s = s[1 : len(s)-1]
	}
	if strings.HasPrefix(s, "-") {
		negative = true
		s = s[1:]
	} else if strings.HasSuffix(s, "-") {
		negative = true
		s = s[:len(s)-1]
	}
	s = strings.TrimPrefix(s, "+")
	if s == "" {
		return 0, false, false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9') && c != '.' && c != ',' {
			return 0, false, false
		}
	}
	norm := normalizeNumber(s, decimal)
	f, err := strconv.ParseFloat(norm, 64)
	if err != nil {
		return 0, false, false
	}
	return int64(math.Round(f)), negative, true
}

// normalizeNumber turns a grouped number into "1234.56" form.
func normalizeNumber(s, decimal string) string {
	if decimal == "" {
		decimal = guessDecimal(s)
	}
	thousands := ","
	if decimal == "," {
		thousands = "."
	}
	s = strings.ReplaceAll(s, thousands, "")
	if decimal == "," {
		s = strings.ReplaceAll(s, ",", ".")
	}
	return s
}

// guessDecimal: with both separators the last one is the decimal mark; a
// lone separator used more than once is grouping; a single separator
// followed by exactly 3 digits is treated as grouping (25.000 / 25,000 -
// overwhelmingly thousands in bank exports), otherwise decimal.
func guessDecimal(s string) string {
	lastDot, lastComma := strings.LastIndex(s, "."), strings.LastIndex(s, ",")
	switch {
	case lastDot >= 0 && lastComma >= 0:
		if lastDot > lastComma {
			return "."
		}
		return ","
	case lastDot >= 0:
		if strings.Count(s, ".") > 1 || len(s)-lastDot-1 == 3 {
			return ","
		}
		return "."
	case lastComma >= 0:
		if strings.Count(s, ",") > 1 || len(s)-lastComma-1 == 3 {
			return "."
		}
		return ","
	}
	return "."
}

// dates

var monthAliases = strings.NewReplacer(
	// Indonesian month abbreviations/names -> English
	"januari", "jan", "februari", "feb", "maret", "mar", "april", "apr", "mei", "may", "juni", "jun",
	"juli", "jul", "agustus", "aug", "agu", "aug", "agt", "aug", "september", "sep", "oktober", "oct", "okt", "oct",
	"november", "nov", "desember", "dec", "des", "dec", "sept", "sep",
)

func dateLayouts(order string) []string {
	dmy := []string{"02/01/2006", "2/1/2006", "02-01-2006", "2-1-2006", "02.01.2006", "2.1.2006", "02/01/06", "2/1/06", "02-01-06"}
	mdy := []string{"01/02/2006", "1/2/2006", "01-02-2006", "1-2-2006", "01.02.2006", "01/02/06", "1/2/06"}
	ymd := []string{"2006-01-02", "2006/01/02", "2006.01.02", "20060102", "2006-1-2", "2006/1/2"}
	named := []string{"02 Jan 2006", "2 Jan 2006", "02-Jan-2006", "2-Jan-2006", "02 Jan 06", "02-Jan-06", "Jan 2, 2006", "Jan 02, 2006", "January 2, 2006", "2 January 2006", "02/Jan/2006"}
	switch order {
	case "mdy":
		return append(append(append(ymd, mdy...), named...), dmy...)
	case "ymd":
		return append(append(append(ymd, named...), dmy...), mdy...)
	}
	return append(append(append(ymd, dmy...), named...), mdy...)
}

// parseDate accepts many common formats; a day/month without a year
// ("01/03", BCA) uses defaultYear. Returns YYYY-MM-DD (UTC calendar date).
func parseDate(s, order string, defaultYear int) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format("2006-01-02"), true
	}
	candidates := []string{s}
	// "2026-03-04 10:22:01" / "04/03/2026 10:22" -> date part
	if i := strings.IndexAny(s, " T"); i > 0 {
		candidates = append(candidates, s[:i])
	}
	layouts := dateLayouts(order)
	for _, c := range candidates {
		lc := c
		if strings.ContainsAny(strings.ToLower(c), "abcdefghijklmnopqrstuvwxyz") {
			// Indonesian month names -> English; time.Parse wants "Jan", not "jan"
			lc = titleMonths(monthAliases.Replace(strings.ToLower(c)))
		}
		for _, l := range layouts {
			if t, err := time.Parse(l, lc); err == nil && t.Year() >= 1970 && t.Year() <= 2200 {
				return t.Format("2006-01-02"), true
			}
		}
	}
	// day/month only
	if defaultYear > 0 {
		short := []string{"02/01", "2/1", "02-01", "2-1"}
		if order == "mdy" {
			short = []string{"01/02", "1/2", "01-02", "1-2"}
		}
		for _, l := range short {
			if t, err := time.Parse(l, s); err == nil {
				return time.Date(defaultYear, t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02"), true
			}
		}
	}
	return "", false
}

var monthWord = regexp.MustCompile(`[a-z]+`)

func titleMonths(s string) string {
	return monthWord.ReplaceAllStringFunc(s, func(w string) string {
		return strings.ToUpper(w[:1]) + w[1:]
	})
}

// rows

// importClientID is the dedupe key: same date|description|amount|row in a
// re-uploaded file maps to the same spending row (UNIQUE(account_id, client_id)).
func importClientID(date, desc string, amount int64, row int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%d", date, strings.TrimSpace(desc), amount, row)))
	return "csv:" + hex.EncodeToString(h[:12])
}

func cell(rec []string, i int) string {
	if i < 0 || i >= len(rec) {
		return ""
	}
	return rec[i]
}

// parseImport turns raw CSV bytes into parsed rows (categories unresolved).
func parseImport(data []byte, opt importOptions) (*parseResult, error) {
	records, lines, delim, err := readCSV(data)
	if err != nil && len(records) == 0 {
		return nil, fmt.Errorf("could not read CSV: %v", err)
	}
	if len(records) == 0 {
		return nil, errors.New("the file is empty")
	}
	p := findPreset(opt.Preset)
	res := &parseResult{Preset: p.ID, Delimiter: string(delim), Errors: []string{}, Rows: []parsedRow{}}
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("CSV read stopped early: %v", err))
	}
	hi := findHeaderRow(records, p)
	res.HeaderRow = lines[hi]
	res.Headers = records[hi]
	m := resolveMapping(res.Headers, p, opt.Mapping)
	res.Mapping = m
	order := opt.DateOrder
	if order == "" {
		order = p.DateOrder
	}
	decimal := opt.Decimal
	if decimal == "" {
		decimal = p.Decimal
	}
	res.DateOrder, res.Decimal = order, decimal
	year := opt.DefaultYear
	if year == 0 {
		year = time.Now().UTC().Year()
	}

	if m.Date < 0 {
		res.Errors = append(res.Errors, "no date column found. Choose one in the column mapping")
	}
	if m.Amount < 0 && m.Debit < 0 {
		res.Errors = append(res.Errors, "no amount or debit column found. Choose one in the column mapping")
	}
	if len(res.Errors) > 0 && (m.Date < 0 || (m.Amount < 0 && m.Debit < 0)) {
		return res, nil
	}

	// Single amount column without a DB/CR marker: if any amount is negative
	// the file is signed (negative = expense), else every amount is an expense.
	negIsExpense := false
	if m.Amount >= 0 && m.Debit < 0 && m.DrCr < 0 {
		if opt.NegativeOnly != nil {
			negIsExpense = *opt.NegativeOnly
		} else {
			for _, rec := range records[hi+1:] {
				if _, neg, ok := parseAmount(cell(rec, m.Amount), decimal); ok && neg && !hasDrCrSuffix(cell(rec, m.Amount)) {
					negIsExpense = true
					break
				}
			}
		}
	}

	rowNum := 0
	for li, rec := range records[hi+1:] {
		if rowNum >= maxImportRows {
			res.Errors = append(res.Errors, fmt.Sprintf("only the first %d rows are imported", maxImportRows))
			break
		}
		blank := true
		for _, c := range rec {
			if c != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		rowNum++
		pr := parsedRow{Row: rowNum, Line: lines[hi+1+li], Status: "ok"}
		pr.Description = strings.Join(strings.Fields(cell(rec, m.Description)), " ")
		pr.SourceCategory = cell(rec, m.Category)

		rawDate := cell(rec, m.Date)
		d, ok := parseDate(rawDate, order, year)
		if !ok {
			// trailer lines ("Saldo Awal", totals, PEND) - not a transaction
			pr.Status, pr.Reason = "skip", fmt.Sprintf("not a transaction date: %q", rawDate)
			if rawDate != "" && looksLikeDate(rawDate) {
				pr.Status = "error"
			}
			res.Rows = append(res.Rows, pr)
			continue
		}
		pr.TxDate = d

		var amount int64
		var out bool
		switch {
		case m.Debit >= 0:
			if a, _, ok := parseAmount(cell(rec, m.Debit), decimal); ok && a != 0 {
				amount, out = a, true
			} else if a, _, ok := parseAmount(cell(rec, m.Credit), decimal); ok && a != 0 {
				amount, out = a, false
			} else {
				pr.Status, pr.Reason = "error", "no debit or credit amount"
				res.Rows = append(res.Rows, pr)
				continue
			}
		default:
			raw := cell(rec, m.Amount)
			a, neg, ok := parseAmount(raw, decimal)
			if !ok {
				pr.Status, pr.Reason = "error", fmt.Sprintf("could not read amount %q", raw)
				res.Rows = append(res.Rows, pr)
				continue
			}
			amount = a
			switch {
			case m.DrCr >= 0 && cell(rec, m.DrCr) != "":
				mark := strings.ToUpper(cell(rec, m.DrCr))
				out = strings.HasPrefix(mark, "D")
			case hasDrCrSuffix(raw):
				out = neg
			case negIsExpense:
				out = neg
			default:
				out = true
			}
		}
		pr.Amount = amount
		if !out {
			pr.Status, pr.Reason = "skip", "money in (credit/income), not imported as spending"
		} else if amount <= 0 {
			pr.Status, pr.Reason = "skip", "zero amount"
		}
		if pr.Description == "" {
			pr.Description = "Imported transaction"
		}
		pr.ClientID = importClientID(pr.TxDate, pr.Description, pr.Amount, pr.Row)
		res.Rows = append(res.Rows, pr)
	}
	return res, nil
}

func hasDrCrSuffix(s string) bool {
	up := strings.ToUpper(strings.TrimSpace(s))
	return strings.HasSuffix(up, "DB") || strings.HasSuffix(up, "CR") || strings.HasSuffix(up, "DR")
}

var dateish = regexp.MustCompile(`^\d{1,4}[/.\-]\d{1,2}([/.\-]\d{1,4})?$`)

func looksLikeDate(s string) bool { return dateish.MatchString(strings.TrimSpace(s)) }
