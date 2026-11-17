package main

import (
	"testing"
)

func TestParseAmount(t *testing.T) {
	cases := []struct {
		in      string
		decimal string
		want    int64
		neg     bool
		ok      bool
	}{
		{"25000", "", 25000, false, true},
		{"25.000", "", 25000, false, true},
		{"25,000", "", 25000, false, true},
		{"1.234.567", "", 1234567, false, true},
		{"1,234,567.89", "", 1234568, false, true},
		{"1.234.567,49", "", 1234567, false, true},
		{"12,5", "", 13, false, true},
		{"12.5", "", 13, false, true},
		{"-15.000", "", 15000, true, true},
		{"(7,500.00)", "", 7500, true, true},
		{"Rp 50.000", "", 50000, false, true},
		{"IDR 1.500.000,00", "", 1500000, false, true},
		{"50,000.00 DB", "", 50000, true, true},
		{"1,250,000.00 CR", "", 1250000, false, true},
		{"300-", "", 300, true, true},
		{"25.000", ".", 25, false, true}, // explicit decimal point
		{"1.000", ",", 1000, false, true},
		{"", "", 0, false, false},
		{"abc", "", 0, false, false},
	}
	for _, c := range cases {
		got, neg, ok := parseAmount(c.in, c.decimal)
		if got != c.want || neg != c.neg || ok != c.ok {
			t.Errorf("parseAmount(%q,%q) = %d,%v,%v want %d,%v,%v", c.in, c.decimal, got, neg, ok, c.want, c.neg, c.ok)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in, order string
		want      string
		ok        bool
	}{
		{"2026-03-04", "dmy", "2026-03-04", true},
		{"04/03/2026", "dmy", "2026-03-04", true},
		{"4/3/2026", "dmy", "2026-03-04", true},
		{"03/04/2026", "mdy", "2026-03-04", true},
		{"04-03-26", "dmy", "2026-03-04", true},
		{"04.03.2026", "dmy", "2026-03-04", true},
		{"4 Mar 2026", "dmy", "2026-03-04", true},
		{"04 Mei 2026", "dmy", "2026-05-04", true},
		{"17 Agu 2026", "dmy", "2026-08-17", true},
		{"25 Des 2026", "dmy", "2026-12-25", true},
		{"Mar 4, 2026", "dmy", "2026-03-04", true},
		{"2026-03-04 13:45:00", "dmy", "2026-03-04", true},
		{"2026-03-04T23:30:00Z", "dmy", "2026-03-04", true},
		{"04/03", "dmy", "2025-03-04", true}, // BCA: no year -> default year
		{"PEND", "dmy", "", false},
		{"Saldo Awal", "dmy", "", false},
		{"", "dmy", "", false},
	}
	for _, c := range cases {
		got, ok := parseDate(c.in, c.order, 2025)
		if got != c.want || ok != c.ok {
			t.Errorf("parseDate(%q,%q) = %q,%v want %q,%v", c.in, c.order, got, ok, c.want, c.ok)
		}
	}
}

func statuses(rows []parsedRow) map[string]int {
	m := map[string]int{}
	for _, r := range rows {
		m[r.Status]++
	}
	return m
}

func TestParseImportGenericSigned(t *testing.T) {
	csv := "Date,Description,Amount,Category\n" +
		"2026-03-01,Coffee,-35000,Food\n" +
		"2026-03-02,Salary,10000000,\n" +
		"2026-03-03,\"Groceries, weekly\",\"-250,000\",groceries\n" +
		"\n" +
		"not a date,Total,-285000,\n"
	res, err := parseImport([]byte(csv), importOptions{Preset: "generic"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Mapping.Date != 0 || res.Mapping.Description != 1 || res.Mapping.Amount != 2 || res.Mapping.Category != 3 {
		t.Fatalf("mapping: %+v", res.Mapping)
	}
	if len(res.Rows) != 4 {
		t.Fatalf("rows: %+v", res.Rows)
	}
	r0, r1, r2 := res.Rows[0], res.Rows[1], res.Rows[2]
	if r0.Status != "ok" || r0.Amount != 35000 || r0.TxDate != "2026-03-01" || r0.SourceCategory != "Food" {
		t.Errorf("row0 %+v", r0)
	}
	if r1.Status != "skip" { // positive amount in a signed file = income
		t.Errorf("row1 should be skipped as income: %+v", r1)
	}
	if r2.Status != "ok" || r2.Amount != 250000 || r2.Description != "Groceries, weekly" {
		t.Errorf("row2 %+v", r2)
	}
	if res.Rows[3].Status != "skip" {
		t.Errorf("trailer row: %+v", res.Rows[3])
	}
	if r0.ClientID == "" || r0.ClientID == r2.ClientID {
		t.Errorf("client ids: %q %q", r0.ClientID, r2.ClientID)
	}
	if r0.ClientID != importClientID("2026-03-01", "Coffee", 35000, 1) {
		t.Errorf("client id not deterministic")
	}
}

func TestParseImportUnsignedAllExpenses(t *testing.T) {
	csv := "tanggal;keterangan;jumlah\n04/03/2026;Bensin;150.000\n05/03/2026;Parkir;5.000\n"
	res, err := parseImport([]byte(csv), importOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Delimiter != ";" {
		t.Fatalf("delimiter %q", res.Delimiter)
	}
	if st := statuses(res.Rows); st["ok"] != 2 {
		t.Fatalf("statuses %v rows %+v", st, res.Rows)
	}
	if res.Rows[0].Amount != 150000 || res.Rows[0].TxDate != "2026-03-04" {
		t.Errorf("row0 %+v", res.Rows[0])
	}
}

func TestParseImportBCA(t *testing.T) {
	csv := "Informasi Rekening - Mutasi Rekening\n" +
		"No. rekening : ,'1234567890\n" +
		"Nama : ,BUDI\n" +
		"\n" +
		"Tanggal Transaksi,Keterangan,Cabang,Jumlah,,Saldo\n" +
		"'01/03,'TRSF E-BANKING DB 0103/FTSCY/WS95051 TOKO ABC,'0000,\"50,000.00\",DB,\"1,950,000.00\"\n" +
		"'02/03,'SWITCHING CR TRANSFER DARI ANI,'0000,\"500,000.00\",CR,\"2,450,000.00\"\n" +
		"'PEND,'KARTU DEBIT INDOMARET,'0000,\"25,000.00\",DB,\n" +
		"Saldo Awal,\"2,000,000.00\"\n"
	res, err := parseImport([]byte(csv), importOptions{Preset: "bca", DefaultYear: 2026})
	if err != nil {
		t.Fatal(err)
	}
	if res.HeaderRow != 5 {
		t.Fatalf("header line %d", res.HeaderRow)
	}
	if res.Mapping.DrCr != 4 || res.Mapping.Amount != 3 {
		t.Fatalf("mapping %+v", res.Mapping)
	}
	r0 := res.Rows[0]
	if r0.Status != "ok" || r0.Amount != 50000 || r0.TxDate != "2026-03-01" {
		t.Errorf("row0 %+v", r0)
	}
	if res.Rows[1].Status != "skip" {
		t.Errorf("CR row should be skipped: %+v", res.Rows[1])
	}
	if res.Rows[2].Status != "skip" { // PEND = pending, no date yet
		t.Errorf("PEND row: %+v", res.Rows[2])
	}
}

func TestParseImportBCASuffixAmounts(t *testing.T) {
	csv := "Tanggal,Keterangan,Jumlah,Saldo\n" +
		"03/03/2026,QRIS KOPI,\"35,000.00 DB\",\"1,000,000.00\"\n" +
		"04/03/2026,BUNGA,\"1,234.00 CR\",\"1,001,234.00\"\n"
	res, err := parseImport([]byte(csv), importOptions{Preset: "bca"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows[0].Status != "ok" || res.Rows[0].Amount != 35000 || res.Rows[1].Status != "skip" {
		t.Fatalf("rows %+v", res.Rows)
	}
}

func TestParseImportMandiriDebitCredit(t *testing.T) {
	csv := "Tanggal,Keterangan,Debit,Kredit,Saldo\n" +
		"01/03/2026,Pembayaran PLN,\"1.250.000,00\",\"0,00\",\"5.000.000,00\"\n" +
		"02/03/2026,Gaji,,\"10.000.000,00\",\"15.000.000,00\"\n" +
		"03 Mar 2026,Tarik tunai,500.000,,\n"
	res, err := parseImport([]byte(csv), importOptions{Preset: "mandiri"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Mapping.Debit != 2 || res.Mapping.Credit != 3 {
		t.Fatalf("mapping %+v", res.Mapping)
	}
	want := []struct {
		status string
		amount int64
	}{{"ok", 1250000}, {"skip", 10000000}, {"ok", 500000}}
	for i, w := range want {
		if res.Rows[i].Status != w.status || res.Rows[i].Amount != w.amount {
			t.Errorf("row %d = %+v want %v", i, res.Rows[i], w)
		}
	}
}

func TestParseImportUserMappingOverridesPreset(t *testing.T) {
	csv := "when,what,how much,extra\n2026-01-05,Taxi,42000,x\n"
	res, err := parseImport([]byte(csv), importOptions{Preset: "generic", Mapping: map[string]int{"date": 0, "description": 1, "amount": 2}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 0 || len(res.Rows) != 1 || res.Rows[0].Amount != 42000 || res.Rows[0].Description != "Taxi" {
		t.Fatalf("res %+v", res)
	}
	// without a mapping the columns can't be found -> helpful error, no rows
	res, _ = parseImport([]byte(csv), importOptions{Preset: "generic"})
	if len(res.Errors) == 0 || len(res.Rows) != 0 {
		t.Fatalf("expected mapping errors: %+v", res)
	}
}

func TestSniffImage(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	if m, ext, ok := sniffImage(png); !ok || m != "image/png" || ext != "png" {
		t.Errorf("png: %v %v %v", m, ext, ok)
	}
	jpg := []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00")
	if _, ext, ok := sniffImage(jpg); !ok || ext != "jpg" {
		t.Errorf("jpg")
	}
	heic := []byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00")
	if _, ext, ok := sniffImage(heic); !ok || ext != "heic" {
		t.Errorf("heic")
	}
	if _, _, ok := sniffImage([]byte("<html><script>alert(1)</script>")); ok {
		t.Errorf("html accepted as image")
	}
}

func TestCSVSafe(t *testing.T) {
	if csvSafe("=HYPERLINK(1)") != "'=HYPERLINK(1)" || csvSafe("Lunch") != "Lunch" {
		t.Error("csvSafe")
	}
}
