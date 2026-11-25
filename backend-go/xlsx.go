package main

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	xlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	xlsxMaxCellLen  = 32767
	xlsxMaxNameLen  = 31
	xlsxXMLHeader   = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"
	xlsxMainNS      = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	xlsxRelNS       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	xlsxPkgRelNS    = "http://schemas.openxmlformats.org/package/2006/relationships"
	xlsxStyleBold   = 1
)

// xlsxSheet is one worksheet: a bold, frozen header row and the data rows
// below it. Cell values may be ints, floats, bools, time.Time, strings,
// []byte, pgtype.Numeric, maps and slices (written as JSON), or nil (blank).
type xlsxSheet struct {
	Name   string
	Header []string
	Rows   [][]any
}

var xlsxEscaper = strings.NewReplacer(
	"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;", "\r", "&#xD;", "\n", "&#xA;",
)

var xlsxSheetNameStripper = strings.NewReplacer("[", "", "]", "", ":", "", "*", "", "?", "", "/", "", `\`, "")

// xlsxColName converts a zero-based column index to its letters (0 -> A, 26 -> AA).
func xlsxColName(i int) string {
	var b []byte
	for i++; i > 0; i = (i - 1) / 26 {
		b = append([]byte{byte('A' + (i-1)%26)}, b...)
	}
	return string(b)
}

// xlsxSheetName makes name a valid, unique worksheet name. used holds the
// lower-cased names already taken (Excel compares names case-insensitively)
// and is updated with the returned one.
func xlsxSheetName(name string, used map[string]bool) string {
	name = strings.Trim(xlsxSheetNameStripper.Replace(name), " '")
	if name == "" {
		name = "Sheet"
	}
	candidate := truncateRunes(name, xlsxMaxNameLen)
	for n := 2; used[strings.ToLower(candidate)]; n++ {
		suffix := "~" + strconv.Itoa(n)
		candidate = truncateRunes(name, xlsxMaxNameLen-len(suffix)) + suffix
	}
	used[strings.ToLower(candidate)] = true
	return candidate
}

func xlsxValidRune(r rune) bool {
	switch {
	case r == '\t', r == '\n', r == '\r':
		return true
	case r < 0x20, r == 0xFFFE, r == 0xFFFF:
		return false
	}
	return true
}

// xlsxText returns s ready to sit inside an XML text node or attribute:
// characters XML cannot carry are dropped, the length is capped at Excel's
// per-cell limit, and markup characters are escaped.
func xlsxText(s string) string {
	var b strings.Builder
	units := 0
	for _, r := range s {
		if !xlsxValidRune(r) {
			continue
		}
		n := utf16.RuneLen(r)
		if n < 0 {
			n = 1
		}
		if units+n > xlsxMaxCellLen {
			break
		}
		units += n
		b.WriteRune(r)
	}
	return xlsxEscaper.Replace(b.String())
}

func xlsxInlineString(ref string, style int, s string) string {
	styleAttr := ""
	if style != 0 {
		styleAttr = ` s="` + strconv.Itoa(style) + `"`
	}
	return `<c r="` + ref + `"` + styleAttr + ` t="inlineStr"><is><t xml:space="preserve">` + xlsxText(s) + `</t></is></c>`
}

func xlsxNumber(ref, literal string) string {
	return `<c r="` + ref + `" t="n"><v>` + literal + `</v></c>`
}

// xlsxCell renders one cell, or "" for nil. Text is always an inline string,
// never a formula, so a value such as "=SUM(A1)" stays literal text.
func xlsxCell(ref string, v any) (string, error) {
	switch t := v.(type) {
	case nil:
		return "", nil
	case int:
		return xlsxNumber(ref, strconv.FormatInt(int64(t), 10)), nil
	case int8:
		return xlsxNumber(ref, strconv.FormatInt(int64(t), 10)), nil
	case int16:
		return xlsxNumber(ref, strconv.FormatInt(int64(t), 10)), nil
	case int32:
		return xlsxNumber(ref, strconv.FormatInt(int64(t), 10)), nil
	case int64:
		return xlsxNumber(ref, strconv.FormatInt(t, 10)), nil
	case uint:
		return xlsxNumber(ref, strconv.FormatUint(uint64(t), 10)), nil
	case uint8:
		return xlsxNumber(ref, strconv.FormatUint(uint64(t), 10)), nil
	case uint16:
		return xlsxNumber(ref, strconv.FormatUint(uint64(t), 10)), nil
	case uint32:
		return xlsxNumber(ref, strconv.FormatUint(uint64(t), 10)), nil
	case uint64:
		return xlsxNumber(ref, strconv.FormatUint(t, 10)), nil
	case float32:
		return xlsxFloat(ref, float64(t)), nil
	case float64:
		return xlsxFloat(ref, t), nil
	case pgtype.Numeric:
		return xlsxNumeric(ref, t)
	case bool:
		if t {
			return `<c r="` + ref + `" t="b"><v>1</v></c>`, nil
		}
		return `<c r="` + ref + `" t="b"><v>0</v></c>`, nil
	case time.Time:
		return xlsxInlineString(ref, 0, exportTime(t)), nil
	case string:
		return xlsxInlineString(ref, 0, t), nil
	case []byte:
		return xlsxInlineString(ref, 0, string(t)), nil
	}
	switch reflect.ValueOf(v).Kind() {
	case reflect.Map, reflect.Slice:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("cell %s: %w", ref, err)
		}
		return xlsxInlineString(ref, 0, string(b)), nil
	}
	return xlsxInlineString(ref, 0, fmt.Sprint(v)), nil
}

func xlsxFloat(ref string, f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return xlsxInlineString(ref, 0, strconv.FormatFloat(f, 'g', -1, 64))
	}
	return xlsxNumber(ref, strconv.FormatFloat(f, 'g', -1, 64))
}

func xlsxNumeric(ref string, n pgtype.Numeric) (string, error) {
	if !n.Valid {
		return "", nil
	}
	dv, err := n.Value()
	if err != nil {
		return "", fmt.Errorf("cell %s: %w", ref, err)
	}
	s := fmt.Sprint(dv)
	if f, err := strconv.ParseFloat(s, 64); err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return xlsxInlineString(ref, 0, s), nil
	}
	return xlsxNumber(ref, s), nil
}

const xlsxStyles = xlsxXMLHeader +
	`<styleSheet xmlns="` + xlsxMainNS + `">` +
	`<fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts>` +
	`<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>` +
	`<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>` +
	`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
	`<cellXfs count="2"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>` +
	`<xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/></cellXfs>` +
	`<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>` +
	`</styleSheet>`

func xlsxSheetPath(i int) string { return "xl/worksheets/sheet" + strconv.Itoa(i+1) + ".xml" }

func xlsxContentTypes(n int) string {
	var b strings.Builder
	b.WriteString(xlsxXMLHeader)
	b.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	b.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	b.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	b.WriteString(`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	b.WriteString(`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	for i := 0; i < n; i++ {
		b.WriteString(`<Override PartName="/` + xlsxSheetPath(i) + `" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`)
	}
	b.WriteString(`</Types>`)
	return b.String()
}

func xlsxWorkbook(names []string) string {
	var b strings.Builder
	b.WriteString(xlsxXMLHeader)
	b.WriteString(`<workbook xmlns="` + xlsxMainNS + `" xmlns:r="` + xlsxRelNS + `"><sheets>`)
	for i, name := range names {
		id := strconv.Itoa(i + 1)
		b.WriteString(`<sheet name="` + xlsxText(name) + `" sheetId="` + id + `" r:id="rId` + id + `"/>`)
	}
	b.WriteString(`</sheets></workbook>`)
	return b.String()
}

func xlsxWorkbookRels(n int) string {
	var b strings.Builder
	b.WriteString(xlsxXMLHeader)
	b.WriteString(`<Relationships xmlns="` + xlsxPkgRelNS + `">`)
	for i := 0; i < n; i++ {
		id := strconv.Itoa(i + 1)
		b.WriteString(`<Relationship Id="rId` + id + `" Type="` + xlsxRelNS + `/worksheet" Target="worksheets/sheet` + id + `.xml"/>`)
	}
	b.WriteString(`<Relationship Id="rId` + strconv.Itoa(n+1) + `" Type="` + xlsxRelNS + `/styles" Target="styles.xml"/>`)
	b.WriteString(`</Relationships>`)
	return b.String()
}

const xlsxRootRels = xlsxXMLHeader +
	`<Relationships xmlns="` + xlsxPkgRelNS + `">` +
	`<Relationship Id="rId1" Type="` + xlsxRelNS + `/officeDocument" Target="xl/workbook.xml"/>` +
	`</Relationships>`

func writeXLSXSheet(w io.Writer, s xlsxSheet) error {
	bw := bufio.NewWriter(w)
	bw.WriteString(xlsxXMLHeader)
	bw.WriteString(`<worksheet xmlns="` + xlsxMainNS + `"><sheetViews><sheetView workbookViewId="0">`)
	if len(s.Header) > 0 {
		bw.WriteString(`<pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/><selection pane="bottomLeft"/>`)
	}
	bw.WriteString(`</sheetView></sheetViews><sheetFormatPr defaultRowHeight="15"/><sheetData>`)
	row := 1
	if len(s.Header) > 0 {
		bw.WriteString(`<row r="1">`)
		for i, h := range s.Header {
			bw.WriteString(xlsxInlineString(xlsxColName(i)+"1", xlsxStyleBold, h))
		}
		bw.WriteString(`</row>`)
		row++
	}
	for _, r := range s.Rows {
		bw.WriteString(`<row r="` + strconv.Itoa(row) + `">`)
		for i, v := range r {
			cell, err := xlsxCell(xlsxColName(i)+strconv.Itoa(row), v)
			if err != nil {
				return err
			}
			bw.WriteString(cell)
		}
		bw.WriteString(`</row>`)
		row++
	}
	bw.WriteString(`</sheetData></worksheet>`)
	return bw.Flush()
}

// writeXLSX writes sheets as a minimal Office Open XML workbook using only
// the standard library.
func writeXLSX(w io.Writer, sheets []xlsxSheet) error {
	if len(sheets) == 0 {
		return fmt.Errorf("xlsx: workbook needs at least one sheet")
	}
	used := map[string]bool{}
	names := make([]string, len(sheets))
	for i, s := range sheets {
		names[i] = xlsxSheetName(s.Name, used)
	}

	zw := zip.NewWriter(w)
	add := func(name string, write func(io.Writer) error) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		return write(f)
	}
	text := func(s string) func(io.Writer) error {
		return func(f io.Writer) error {
			_, err := io.WriteString(f, s)
			return err
		}
	}
	parts := []struct {
		name  string
		write func(io.Writer) error
	}{
		{"[Content_Types].xml", text(xlsxContentTypes(len(sheets)))},
		{"_rels/.rels", text(xlsxRootRels)},
		{"xl/workbook.xml", text(xlsxWorkbook(names))},
		{"xl/_rels/workbook.xml.rels", text(xlsxWorkbookRels(len(sheets)))},
		{"xl/styles.xml", text(xlsxStyles)},
	}
	for _, p := range parts {
		if err := add(p.name, p.write); err != nil {
			return err
		}
	}
	for i, s := range sheets {
		if err := add(xlsxSheetPath(i), func(f io.Writer) error { return writeXLSXSheet(f, s) }); err != nil {
			return err
		}
	}
	return zw.Close()
}
