package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type xlsxTestCell struct {
	Ref    string `xml:"r,attr"`
	Type   string `xml:"t,attr"`
	Style  string `xml:"s,attr"`
	Value  string `xml:"v"`
	Text   string `xml:"is>t"`
	Formul string `xml:"f"`
}

type xlsxTestRow struct {
	R     int            `xml:"r,attr"`
	Cells []xlsxTestCell `xml:"c"`
}

type xlsxTestSheet struct {
	Rows []xlsxTestRow `xml:"sheetData>row"`
	Pane struct {
		YSplit string `xml:"ySplit,attr"`
		State  string `xml:"state,attr"`
	} `xml:"sheetViews>sheetView>pane"`
}

func buildXLSX(t *testing.T, sheets []xlsxSheet) map[string][]byte {
	t.Helper()
	var buf bytes.Buffer
	if err := writeXLSX(&buf, sheets); err != nil {
		t.Fatal(err)
	}
	return unzipXLSX(t, buf.Bytes())
}

func unzipXLSX(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		parts[f.Name] = b
	}
	return parts
}

func parseXLSXSheet(t *testing.T, parts map[string][]byte, name string) xlsxTestSheet {
	t.Helper()
	raw, ok := parts[name]
	if !ok {
		t.Fatalf("missing part %s", name)
	}
	var sh xlsxTestSheet
	if err := xml.Unmarshal(raw, &sh); err != nil {
		t.Fatalf("%s is not valid XML: %v", name, err)
	}
	return sh
}

func TestXLSXPartsAreWellFormed(t *testing.T) {
	parts := buildXLSX(t, []xlsxSheet{{Name: "One", Header: []string{"a"}}, {Name: "Two", Header: []string{"b"}}})
	for _, name := range []string{
		"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels",
		"xl/styles.xml", "xl/worksheets/sheet1.xml", "xl/worksheets/sheet2.xml",
	} {
		raw, ok := parts[name]
		if !ok {
			t.Fatalf("missing part %s", name)
		}
		var v struct{}
		if err := xml.Unmarshal(raw, &v); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	wb := string(parts["xl/workbook.xml"])
	if !strings.Contains(wb, `name="One"`) || !strings.Contains(wb, `name="Two"`) {
		t.Fatalf("workbook: %s", wb)
	}
	if !strings.Contains(string(parts["[Content_Types].xml"]), "/xl/worksheets/sheet2.xml") {
		t.Fatal("content types miss sheet2")
	}
	if !strings.Contains(string(parts["xl/_rels/workbook.xml.rels"]), `Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"`) {
		t.Fatalf("styles rel: %s", parts["xl/_rels/workbook.xml.rels"])
	}
}

func TestXLSXCellTypesAndHeader(t *testing.T) {
	when := time.Date(2026, 3, 10, 8, 30, 0, 0, time.UTC)
	parts := buildXLSX(t, []xlsxSheet{{
		Name:   "Spending",
		Header: []string{"id", "note", "ok", "when", "day", "blank", "price", "ratio"},
		Rows: [][]any{{
			int64(7), "a & b < c > d", true, when, time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), nil,
			pgtype.Numeric{Int: nil, Valid: false}, 1.5,
		}, {
			int32(8), "=cmd|' /C calc'!A0", false, nil, nil, nil, nil, nil,
		}},
	}})
	sh := parseXLSXSheet(t, parts, "xl/worksheets/sheet1.xml")
	if sh.Pane.YSplit != "1" || sh.Pane.State != "frozen" {
		t.Fatalf("header not frozen: %+v", sh.Pane)
	}
	if len(sh.Rows) != 3 {
		t.Fatalf("rows: %d", len(sh.Rows))
	}
	head := sh.Rows[0]
	if len(head.Cells) != 8 || head.Cells[0].Ref != "A1" || head.Cells[7].Ref != "H1" || head.Cells[1].Text != "note" {
		t.Fatalf("header row: %+v", head)
	}
	for _, c := range head.Cells {
		if c.Style != "1" {
			t.Fatalf("header cell %s not bold", c.Ref)
		}
	}
	r := sh.Rows[1].Cells
	if r[0].Ref != "A2" || r[0].Type != "n" || r[0].Value != "7" {
		t.Fatalf("number cell: %+v", r[0])
	}
	if r[1].Type != "inlineStr" || r[1].Text != "a & b < c > d" {
		t.Fatalf("escaped text round trip: %+v", r[1])
	}
	if r[2].Type != "b" || r[2].Value != "1" {
		t.Fatalf("bool cell: %+v", r[2])
	}
	if r[3].Text != "2026-03-10T08:30:00Z" || r[4].Text != "2026-03-10" {
		t.Fatalf("time cells: %+v %+v", r[3], r[4])
	}
	for _, c := range r {
		if c.Ref == "F2" || c.Ref == "G2" {
			t.Fatalf("nil and invalid numeric should be omitted, got %+v", c)
		}
	}
	last := r[len(r)-1]
	if last.Ref != "H2" || last.Type != "n" || last.Value != "1.5" {
		t.Fatalf("float cell: %+v", last)
	}
	second := sh.Rows[2].Cells
	if second[1].Type != "inlineStr" || second[1].Text != "=cmd|' /C calc'!A0" || second[2].Value != "0" {
		t.Fatalf("formula text and false bool: %+v", second)
	}
}

func TestXLSXNeverWritesFormulas(t *testing.T) {
	parts := buildXLSX(t, []xlsxSheet{{
		Name: "S", Header: []string{"=SUM(A1)"}, Rows: [][]any{{"=1+1", "+x", "-x", "@y", "\tz"}},
	}})
	if bytes.Contains(parts["xl/worksheets/sheet1.xml"], []byte("<f>")) || bytes.Contains(parts["xl/worksheets/sheet1.xml"], []byte("<f ")) {
		t.Fatal("worksheet contains a formula element")
	}
	sh := parseXLSXSheet(t, parts, "xl/worksheets/sheet1.xml")
	if got := sh.Rows[1].Cells[0].Text; got != "=1+1" {
		t.Fatalf("text altered: %q", got)
	}
}

func TestXLSXTextStripsInvalidAndTruncates(t *testing.T) {
	if got := xlsxText("a\x00b\x01c\x1fd￾e\tf\ng\rh"); got != "abcde\tf&#xA;g&#xD;h" {
		t.Fatalf("control chars: %q", got)
	}
	if got := xlsxText(`"quote" 'apos'`); got != "&quot;quote&quot; &apos;apos&apos;" {
		t.Fatalf("escape: %q", got)
	}
	long := strings.Repeat("x", xlsxMaxCellLen+500)
	if got := xlsxText(long); len(got) != xlsxMaxCellLen {
		t.Fatalf("truncate: %d", len(got))
	}
	emoji := strings.Repeat("\U0001F600", xlsxMaxCellLen)
	if got := []rune(xlsxText(emoji)); len(got)*2 > xlsxMaxCellLen {
		t.Fatalf("astral characters count as two UTF-16 units: %d runes", len(got))
	}

	parts := buildXLSX(t, []xlsxSheet{{Name: "S", Rows: [][]any{{"bad\x00\x08char", long}}}})
	sh := parseXLSXSheet(t, parts, "xl/worksheets/sheet1.xml")
	if sh.Rows[0].Cells[0].Text != "badchar" || len(sh.Rows[0].Cells[1].Text) != xlsxMaxCellLen {
		t.Fatalf("cells: %q len %d", sh.Rows[0].Cells[0].Text, len(sh.Rows[0].Cells[1].Text))
	}
}

func TestXLSXJSONAndFallbackValues(t *testing.T) {
	cell, err := xlsxCell("A1", map[string]any{"k": []any{1, "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cell, "{&quot;k&quot;:[1,&quot;x&quot;]}") {
		t.Fatalf("map: %s", cell)
	}
	cell, err = xlsxCell("A1", []any{int64(1), int64(2)})
	if err != nil || !strings.Contains(cell, ">[1,2]<") {
		t.Fatalf("slice: %s %v", cell, err)
	}
	if cell, _ := xlsxCell("A1", [16]byte{1}); !strings.Contains(cell, "inlineStr") {
		t.Fatalf("fallback: %s", cell)
	}
	if cell, _ := xlsxCell("A1", nil); cell != "" {
		t.Fatalf("nil: %q", cell)
	}
}

func TestXLSXColName(t *testing.T) {
	for i, want := range map[int]string{0: "A", 1: "B", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA", 701: "ZZ", 702: "AAA"} {
		if got := xlsxColName(i); got != want {
			t.Errorf("xlsxColName(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestXLSXSheetNames(t *testing.T) {
	used := map[string]bool{}
	cases := []struct{ in, want string }{
		{"spending", "spending"},
		{"Spending", "Spending~2"},
		{"spending", "spending~3"},
		{"a[b]c:d*e?f/g\\h", "abcdefgh"},
		{"", "Sheet"},
		{"'quoted'", "quoted"},
		{strings.Repeat("n", 40), strings.Repeat("n", 31)},
		{strings.Repeat("n", 40), strings.Repeat("n", 29) + "~2"},
	}
	for _, c := range cases {
		if got := xlsxSheetName(c.in, used); got != c.want {
			t.Errorf("xlsxSheetName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestXLSXWorkbookSheetNamesEscaped(t *testing.T) {
	parts := buildXLSX(t, []xlsxSheet{{Name: "R&D"}, {Name: "R&D"}})
	wb := string(parts["xl/workbook.xml"])
	if !strings.Contains(wb, `name="R&amp;D"`) || !strings.Contains(wb, `name="R&amp;D~2"`) {
		t.Fatalf("workbook: %s", wb)
	}
}

func TestXLSXNeedsASheet(t *testing.T) {
	if err := writeXLSX(io.Discard, nil); err == nil {
		t.Fatal("expected an error for an empty workbook")
	}
}
