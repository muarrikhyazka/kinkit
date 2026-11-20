package main

// Pure helpers for shopping lists (unit tested in lists_test.go): the bulk
// item parser, staple due-date math and the amount shorthand used by bots.

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// parsedItem is one item from free text such as "eggs, 2 milk, bread x3".
type parsedItem struct {
	Name string `json:"name"`
	Qty  string `json:"qty"`
}

const maxItemNameLen = 120
const maxBulkItems = 100

// Units recognised next to a number ("1kg rice", "rice 500 g", "2 packs pasta").
const listQtyUnits = `kg|kgs|g|gr|gram|grams|mg|l|lt|ltr|liter|liters|litre|litres|ml|lb|lbs|oz|pcs|pc|pieces|piece|pack|packs|pk|dozen|doz|bottle|bottles|btl|can|cans|box|boxes|bag|bags|bunch|bunches|loaf|loaves|jar|jars|roll|rolls|tray|trays|ikat|bks|bungkus|buah|biji|sachet|sachets`

var (
	reListBullet = regexp.MustCompile(`^(?:[-*•·+]+|\[[ xX]?\]|\d+[.)])\s+`)
	// "2 milk", "1kg rice", "500 g flour", "2x milk", "3 x bread"
	reQtyLead = regexp.MustCompile(`(?i)^(\d+(?:[.,]\d+)?)\s*(` + listQtyUnits + `|x|×)?\s+(.+)$`)
	// "1kg" glued with no space before the name is handled above; this one is
	// "milk x2", "bread ×3", "eggs x 12"
	reQtyTrailX = regexp.MustCompile(`(?i)^(.+?)\s+[x×]\s*(\d+(?:[.,]\d+)?)$`)
	// "rice 1kg", "milk 2", "flour 500 g"
	reQtyTrail = regexp.MustCompile(`(?i)^(.+?)\s+(\d+(?:[.,]\d+)?)\s*(` + listQtyUnits + `)?$`)
	// "eggs (12)", "rice (5 kg)"
	reQtyParen   = regexp.MustCompile(`^(.+?)\s*\(([^()]+)\)$`)
	reListSpaces = regexp.MustCompile(`\s+`)
)

// splitItemsText splits on newlines, semicolons and commas - except a comma
// between two digits ("1,5kg rice" keeps its decimal comma).
func splitItemsText(text string) []string {
	var parts []string
	var cur strings.Builder
	rs := []rune(text)
	for i, r := range rs {
		sep := r == '\n' || r == '\r' || r == ';'
		if r == ',' {
			digitBefore := i > 0 && rs[i-1] >= '0' && rs[i-1] <= '9'
			digitAfter := i+1 < len(rs) && rs[i+1] >= '0' && rs[i+1] <= '9'
			sep = !(digitBefore && digitAfter)
		}
		if sep {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	parts = append(parts, cur.String())
	return parts
}

// listFormatQty normalises a number + unit: ("2", "") -> "2", ("1", "KG") -> "1kg",
// ("2", "x") -> "2".
func listFormatQty(num, unit string) string {
	unit = strings.ToLower(strings.TrimSpace(unit))
	if unit == "x" || unit == "×" {
		unit = ""
	}
	return num + unit
}

// parseItemLine turns one line into an item; ok=false for blank lines.
func parseItemLine(line string) (parsedItem, bool) {
	s := strings.TrimSpace(reListSpaces.ReplaceAllString(line, " "))
	for {
		t := reListBullet.ReplaceAllString(s, "")
		if t == s {
			break
		}
		s = strings.TrimSpace(t)
	}
	if s == "" {
		return parsedItem{}, false
	}
	it := parsedItem{Name: s}
	switch {
	case reQtyLead.MatchString(s):
		m := reQtyLead.FindStringSubmatch(s)
		it = parsedItem{Name: m[3], Qty: listFormatQty(m[1], m[2])}
	case reQtyTrailX.MatchString(s):
		m := reQtyTrailX.FindStringSubmatch(s)
		it = parsedItem{Name: m[1], Qty: m[2]}
	case reQtyParen.MatchString(s):
		m := reQtyParen.FindStringSubmatch(s)
		it = parsedItem{Name: m[1], Qty: strings.TrimSpace(m[2])}
	case reQtyTrail.MatchString(s):
		m := reQtyTrail.FindStringSubmatch(s)
		it = parsedItem{Name: m[1], Qty: listFormatQty(m[2], m[3])}
	}
	it.Name = strings.TrimSpace(it.Name)
	if it.Name == "" { // e.g. the whole line was "2kg"
		it = parsedItem{Name: s}
	}
	it.Name = listClip(it.Name, maxItemNameLen)
	it.Qty = listClip(it.Qty, 40)
	return it, true
}

// parseItemsText parses bulk text ("eggs, 2 milk, bread x3" or one item per
// line) into items, skipping blanks and capping at maxBulkItems.
func parseItemsText(text string) []parsedItem {
	out := []parsedItem{}
	for _, part := range splitItemsText(text) {
		if it, ok := parseItemLine(part); ok {
			out = append(out, it)
			if len(out) >= maxBulkItems {
				break
			}
		}
	}
	return out
}

func listClip(s string, n int) string {
	rs := []rune(s)
	if len(rs) > n {
		return strings.TrimSpace(string(rs[:n]))
	}
	return s
}

// listItemKey is the case/space-insensitive identity used for duplicates and
// suggestions.
func listItemKey(name string) string {
	return strings.ToLower(strings.TrimSpace(reListSpaces.ReplaceAllString(name, " ")))
}

// staples

// stapleLocalDay is t's calendar date in loc, as midnight UTC (comparable with
// householdToday).
func stapleLocalDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// stapleNextDue is the household-local day the staple should next be put on
// the list: never added -> today; otherwise the day it was last added plus
// every_days.
func stapleNextDue(lastAdded *time.Time, everyDays int, today time.Time, loc *time.Location) time.Time {
	if everyDays < 1 {
		everyDays = 1
	}
	if lastAdded == nil {
		return today
	}
	return stapleLocalDay(*lastAdded, loc).AddDate(0, 0, everyDays)
}

// stapleDue: due on or after its next due day.
func stapleDue(lastAdded *time.Time, everyDays int, today time.Time, loc *time.Location) bool {
	return !today.Before(stapleNextDue(lastAdded, everyDays, today, loc))
}

// amounts

var reListAmount = regexp.MustCompile(`^(\d+(?:[.,]\d+)?)(k|rb|ribu|jt|juta|m|mio)?$`)

// listParseAmount accepts "250000", "250.000", "250k", "2rb", "1.5jt",
// "1,5jt" (same shorthand as the web MoneyInput). ok=false if unparseable.
func listParseAmount(s string) (int64, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "rp")
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	if s == "" {
		return 0, false
	}
	m := reListAmount.FindStringSubmatch(s)
	if m == nil {
		// plain number with thousands separators: "250.000" / "250,000"
		digits := strings.NewReplacer(".", "", ",", "").Replace(s)
		n, err := strconv.ParseInt(digits, 10, 64)
		return n, err == nil && n >= 0
	}
	num, suffix := m[1], m[2]
	if suffix == "" {
		// "250.000" matched as a decimal - treat 3-digit groups as thousands
		if i := strings.IndexAny(num, ".,"); i >= 0 && len(num)-i-1 == 3 {
			n, err := strconv.ParseInt(strings.NewReplacer(".", "", ",", "").Replace(num), 10, 64)
			return n, err == nil
		}
	}
	f, err := strconv.ParseFloat(strings.Replace(num, ",", ".", 1), 64)
	if err != nil {
		return 0, false
	}
	mult := map[string]float64{"": 1, "k": 1e3, "rb": 1e3, "ribu": 1e3, "jt": 1e6, "juta": 1e6, "m": 1e6, "mio": 1e6}[suffix]
	return int64(math.Round(f * mult)), true
}
