package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Natural-language reminder parser used by the chat bots' /remind command
// (pure, unit tested). Supported shapes (case-insensitive, "@name" anywhere
// assigns the reminder):
//
//	tomorrow 8am take medicine        today 18:30 call the plumber
//	tonight put the bins out          monday 9am dentist
//	8pm call mom                      on 2026-12-01 10:00 renew passport
//	dec 24 9:00 wrap presents         in 2 hours call grandma
//	in 30 min check the oven          every day 7:00 vitamins
//	every monday 7:00 trash           every mon, wed and fri 6pm gym
//	every weekday 6:30 school run     every month on the 5th pay rent
//	every year on mar 14 mom's birthday
//	every 2 weeks on fri 10:00 change bed sheets
//	every other week on tue and fri trash     every 3 months on the 1st oil change
//	tiap 2 minggu jumat jam 9 cuci sajadah     setiap 3 bulan kuras tandon
//
// Times: 8am, 8:30pm, 8.30, 20:15, noon, midnight (optionally after "at").
// Without a time, 09:00 is used ("tonight" = 20:00).

type parsedReminder struct {
	Title    string
	Assignee string // name after "@", "" = whole family
	Schedule reminderSchedule
}

var errNoWhen = errors.New(`I couldn't tell when. Try "tomorrow 8am take medicine", "every monday 7:00 trash" or "in 2 hours call grandma"`)

var (
	weekdayWords = map[string]int{
		"sun": 0, "sunday": 0, "sundays": 0,
		"mon": 1, "monday": 1, "mondays": 1,
		"tue": 2, "tues": 2, "tuesday": 2, "tuesdays": 2,
		"wed": 3, "weds": 3, "wednesday": 3, "wednesdays": 3,
		"thu": 4, "thur": 4, "thurs": 4, "thursday": 4, "thursdays": 4,
		"fri": 5, "friday": 5, "fridays": 5,
		"sat": 6, "saturday": 6, "saturdays": 6,
		// Indonesian ("minggu" is left out: it also means "week")
		"ahad": 0, "senin": 1, "selasa": 2, "rabu": 3, "kamis": 4, "jumat": 5, "jum'at": 5, "sabtu": 6,
	}
	monthWords = map[string]time.Month{
		"jan": 1, "january": 1, "feb": 2, "february": 2, "mar": 3, "march": 3, "apr": 4, "april": 4,
		"may": 5, "jun": 6, "june": 6, "jul": 7, "july": 7, "aug": 8, "august": 8,
		"sep": 9, "sept": 9, "september": 9, "oct": 10, "october": 10, "nov": 11, "november": 11, "dec": 12, "december": 12,
	}
	timeRe    = regexp.MustCompile(`^(\d{1,2})(?:[:.](\d{2}))?(am|pm|a\.m\.|p\.m\.)?$`)
	ordinalRe = regexp.MustCompile(`^(\d{1,2})(st|nd|rd|th)?$`)
	isoDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

func cleanWord(w string) string {
	return strings.Trim(strings.ToLower(w), ",;")
}

// parseTimeAt tries to read a time starting at words[i]. Returns hour,
// minute and the number of words consumed. A bare number ("8") only counts
// when allowBare (it followed "at").
func parseTimeAt(words []string, i int, allowBare bool) (int, int, int, bool) {
	if i >= len(words) {
		return 0, 0, 0, false
	}
	w := cleanWord(words[i])
	switch w {
	case "noon", "midday":
		return 12, 0, 1, true
	case "midnight":
		return 0, 0, 1, true
	}
	m := timeRe.FindStringSubmatch(w)
	if m == nil {
		return 0, 0, 0, false
	}
	h, _ := strconv.Atoi(m[1])
	mi := 0
	if m[2] != "" {
		mi, _ = strconv.Atoi(m[2])
	}
	suffix := m[3]
	used := 1
	if suffix == "" && i+1 < len(words) {
		switch cleanWord(words[i+1]) {
		case "am", "a.m.", "pm", "p.m.":
			suffix = cleanWord(words[i+1])
			used = 2
		}
	}
	if suffix == "" && m[2] == "" && !allowBare {
		return 0, 0, 0, false
	}
	if suffix != "" {
		if h < 1 || h > 12 {
			return 0, 0, 0, false
		}
		pm := strings.HasPrefix(suffix, "p")
		if h == 12 {
			h = 0
		}
		if pm {
			h += 12
		}
	}
	if h > 23 || mi > 59 {
		return 0, 0, 0, false
	}
	return h, mi, used, true
}

// repeatUnit normalizes an interval unit word (English or Indonesian).
func repeatUnit(w string) string {
	switch w {
	case "week", "weeks", "wk", "wks", "minggu", "pekan":
		return "week"
	case "month", "months", "mo", "mos", "bulan":
		return "month"
	case "day", "days", "hari":
		return "day"
	}
	return ""
}

// parseWeekdayList reads "[on] mon, wed and fri" at words[i]. With allowOn,
// a leading "on"/"pada" is accepted (and only consumed when weekdays follow).
// Returns the weekdays (deduplicated, in order) and the next index.
func parseWeekdayList(words []string, i int, allowOn bool) ([]int, int) {
	j := i
	if allowOn && j < len(words) {
		switch cleanWord(words[j]) {
		case "on", "pada", "hari", "setiap", "tiap":
			j++
		}
	}
	var days []int
	seen := map[int]bool{}
	for j < len(words) {
		cw := cleanWord(words[j])
		if cw == "and" || cw == "&" || cw == "dan" {
			j++
			continue
		}
		wd, ok := weekdayWords[cw]
		if !ok {
			break
		}
		if !seen[wd] {
			seen[wd] = true
			days = append(days, wd)
		}
		j++
	}
	if len(days) == 0 {
		return nil, i
	}
	return days, j
}

// parseMonthDay reads "[on] [the|day|tanggal] 5th" at words[i]. A bare
// number only counts when it can't be a time ("5 9am" -> 5, "9am" -> no).
func parseMonthDay(words []string, i int) (int, int, bool) {
	j := i
	for j < len(words) {
		switch cleanWord(words[j]) {
		case "on", "the", "pada", "tanggal", "tgl", "day":
			j++
			continue
		}
		break
	}
	if j >= len(words) {
		return 0, i, false
	}
	m := ordinalRe.FindStringSubmatch(cleanWord(words[j]))
	if m == nil || strings.ContainsAny(words[j], ":.") {
		return 0, i, false
	}
	if m[2] == "" && (j+1 >= len(words) || cleanWord(words[j+1]) == "am" || cleanWord(words[j+1]) == "pm") {
		return 0, i, false
	}
	d, _ := strconv.Atoi(m[1])
	if d < 1 || d > 31 {
		return 0, i, false
	}
	return d, j + 1, true
}

// timeAfter reads an optional "[at|jam|pukul] <time>" at words[i].
func timeAfter(words []string, i int) (int, int, int, bool) {
	if i < len(words) && (cleanWord(words[i]) == "at" || cleanWord(words[i]) == "jam" || cleanWord(words[i]) == "pukul") {
		if h, m, n, ok := parseTimeAt(words, i+1, true); ok {
			return h, m, n + 1, true
		}
		return 0, 0, 0, false
	}
	return parseTimeAt(words, i, false)
}

// parseDayMonth reads "dec 24" / "24 dec" / "december 24th" at words[i].
func parseDayMonth(words []string, i int) (time.Month, int, int, bool) {
	if i+1 >= len(words) {
		return 0, 0, 0, false
	}
	a, b := cleanWord(words[i]), cleanWord(words[i+1])
	if mon, ok := monthWords[a]; ok {
		if m := ordinalRe.FindStringSubmatch(b); m != nil {
			d, _ := strconv.Atoi(m[1])
			if d >= 1 && d <= 31 {
				return mon, d, 2, true
			}
		}
	}
	if mon, ok := monthWords[b]; ok {
		if m := ordinalRe.FindStringSubmatch(a); m != nil {
			d, _ := strconv.Atoi(m[1])
			if d >= 1 && d <= 31 {
				return mon, d, 2, true
			}
		}
	}
	return 0, 0, 0, false
}

func fmtHHMM(h, m int) string { return fmt.Sprintf("%02d:%02d", h, m) }

// parseReminderText parses chat text into a reminder. now is the current
// instant; loc the household zone.
func parseReminderText(text string, now time.Time, loc *time.Location) (parsedReminder, error) {
	if loc == nil {
		loc = time.UTC
	}
	var out parsedReminder
	var words []string
	for _, w := range strings.Fields(text) {
		if strings.HasPrefix(w, "@") && len(w) > 1 {
			if out.Assignee == "" {
				out.Assignee = strings.Trim(w[1:], ",.;:!?")
			}
			continue
		}
		words = append(words, w)
	}
	if len(words) == 0 {
		return out, errNoWhen
	}
	nowL := now.In(loc)
	today := dateOnly(nowL)
	i := 0
	first := cleanWord(words[0])

	at := func(d time.Time, h, m int) time.Time {
		return time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, loc).UTC()
	}
	once := func(t time.Time) {
		out.Schedule = reminderSchedule{Kind: "once", OnceAt: &t, StartDate: dateOnly(t.In(loc))}
	}

	switch {
	// in N minutes/hours/days
	case first == "in" && len(words) >= 3:
		nWord := cleanWord(words[1])
		n, err := strconv.Atoi(nWord)
		if nWord == "a" || nWord == "an" {
			n, err = 1, nil
		}
		if err != nil || n <= 0 || n > 10000 {
			return out, errNoWhen
		}
		var d time.Duration
		switch strings.TrimSuffix(cleanWord(words[2]), "s") {
		case "minute", "min", "m", "mn":
			d = time.Duration(n) * time.Minute
		case "hour", "hr", "h":
			d = time.Duration(n) * time.Hour
		case "day", "d":
			d = time.Duration(n) * 24 * time.Hour
		case "week", "wk", "w":
			d = time.Duration(n) * 7 * 24 * time.Hour
		default:
			return out, errNoWhen
		}
		once(now.Add(d).UTC().Truncate(time.Minute))
		i = 3

	// recurring
	case first == "every" || first == "tiap" || first == "setiap" || first == "daily" || first == "everyday":
		s := reminderSchedule{StartDate: today, TimeOfDay: "09:00"}
		i = 1
		if first == "daily" || first == "everyday" {
			s.Kind = "daily"
		} else {
			if len(words) < 2 {
				return out, errNoWhen
			}
			w := cleanWord(words[1])
			// "every 2 weeks", "every other week", "tiap 3 bulan"
			n := 0
			if w == "other" || w == "alternate" {
				n = 2
			} else if k, err := strconv.Atoi(w); err == nil {
				if k < 1 || k > maxReminderInterval {
					return out, fmt.Errorf("the repeat interval must be 1-%d", maxReminderInterval)
				}
				n = k
			}
			if n > 0 {
				if len(words) < 3 {
					return out, errNoWhen
				}
				i = 3
				switch repeatUnit(cleanWord(words[2])) {
				case "week":
					s.Kind = "weekly"
					var days []int
					days, i = parseWeekdayList(words, i, true)
					if len(days) == 0 {
						days = []int{int(today.Weekday())}
					}
					s.Weekdays = days
				case "month":
					s.Kind = "monthly"
					s.DayOfMonth = today.Day()
					if d, j, ok := parseMonthDay(words, i); ok {
						s.DayOfMonth, i = d, j
					}
				case "day":
					if n != 1 {
						return out, errors.New(`repeating every few days isn't supported yet. Try "every day", "every 2 weeks on fri" or "every 3 months"`)
					}
					s.Kind = "daily"
				default:
					return out, errNoWhen
				}
				if n > 1 {
					s.Interval = n
				}
			}
			switch {
			case n > 0: // handled above
			case w == "day" || w == "hari" || w == "morning" || w == "pagi" || w == "evening" || w == "sore" || w == "night" || w == "malam":
				s.Kind = "daily"
				i = 2
				switch w {
				case "morning", "pagi":
					s.TimeOfDay = "08:00"
				case "evening", "sore":
					s.TimeOfDay = "18:00"
				case "night", "malam":
					s.TimeOfDay = "20:00"
				}
			case w == "weekday" || w == "weekdays":
				s.Kind, s.Weekdays, i = "weekly", []int{1, 2, 3, 4, 5}, 2
			case w == "weekend" || w == "weekends":
				s.Kind, s.Weekdays, i = "weekly", []int{0, 6}, 2
			case w == "week" || w == "minggu":
				s.Kind = "weekly"
				var days []int
				days, i = parseWeekdayList(words, 2, true)
				if len(days) == 0 {
					days = []int{int(today.Weekday())}
				}
				s.Weekdays = days
			case w == "month" || w == "bulan":
				s.Kind = "monthly"
				i = 2
				s.DayOfMonth = today.Day()
				if d, j, ok := parseMonthDay(words, i); ok {
					s.DayOfMonth, i = d, j
				}
			case w == "year" || w == "tahun":
				s.Kind = "yearly"
				i = 2
				if i < len(words) && (cleanWord(words[i]) == "on" || cleanWord(words[i]) == "pada") {
					i++
				}
				mon, d, n, ok := parseDayMonth(words, i)
				if !ok {
					return out, errNoWhen
				}
				i += n
				s.StartDate, s.DayOfMonth = yearlyStart(today, mon, d), d
			default:
				if mon, d, n, ok := parseDayMonth(words, 1); ok { // every dec 24
					s.Kind = "yearly"
					s.StartDate, s.DayOfMonth = yearlyStart(today, mon, d), d
					i = 1 + n
					break
				}
				// every mon, wed and fri
				days, j := parseWeekdayList(words, 1, false)
				if len(days) == 0 {
					return out, errNoWhen
				}
				s.Kind, s.Weekdays, i = "weekly", days, j
			}
		}
		if h, m, n, ok := timeAfter(words, i); ok {
			s.TimeOfDay = fmtHHMM(h, m)
			i += n
		}
		out.Schedule = s

	// one-off on a day
	default:
		h, m, hasTime := 9, 0, false
		// time may come first: "8am tomorrow ..."
		if th, tm, n, ok := timeAfter(words, 0); ok {
			h, m, hasTime = th, tm, true
			i = n
		}
		var day time.Time
		hasDay := false
		if i < len(words) && cleanWord(words[i]) == "on" {
			i++
		}
		if i < len(words) {
			w := cleanWord(words[i])
			switch {
			case w == "today":
				day, hasDay = today, true
				i++
			case w == "tonight":
				day, hasDay = today, true
				if !hasTime {
					h, m = 20, 0
				}
				i++
			case w == "tomorrow" || w == "tmr" || w == "tmrw" || w == "tomorow":
				day, hasDay = today.AddDate(0, 0, 1), true
				i++
			case w == "next" && i+1 < len(words):
				if wd, ok := weekdayWords[cleanWord(words[i+1])]; ok {
					delta := (wd - int(today.Weekday()) + 7) % 7
					if delta == 0 {
						delta = 7
					}
					day, hasDay = today.AddDate(0, 0, delta), true
					i += 2
				} else if cleanWord(words[i+1]) == "week" {
					day, hasDay = today.AddDate(0, 0, 7), true
					i += 2
				}
			case isoDateRe.MatchString(w):
				if d, err := time.Parse("2006-01-02", w); err == nil {
					day, hasDay = d, true
					i++
				}
			default:
				if wd, ok := weekdayWords[w]; ok {
					delta := (wd - int(today.Weekday()) + 7) % 7
					day, hasDay = today.AddDate(0, 0, delta), true
					i++
					// "monday" said on a Monday after the time passed -> next week
					if delta == 0 {
						th, tm := h, m
						if !hasTime {
							if x, y, _, ok := timeAfter(words, i); ok {
								th, tm = x, y
							}
						}
						if !at(day, th, tm).After(now) {
							day = day.AddDate(0, 0, 7)
						}
					}
				} else if mon, d, n, ok := parseDayMonth(words, i); ok {
					day, hasDay = yearlyStart(today, mon, d), true
					if day.Day() != d { // clamped (e.g. feb 30) -> reject
						return out, errNoWhen
					}
					i += n
				}
			}
		}
		if !hasTime {
			if th, tm, n, ok := timeAfter(words, i); ok {
				h, m, hasTime = th, tm, true
				i += n
			}
		}
		if !hasDay && !hasTime {
			return out, errNoWhen
		}
		if !hasDay { // just a time: today, or tomorrow if it already passed
			day = today
			if !at(day, h, m).After(now) {
				day = today.AddDate(0, 0, 1)
			}
		}
		t := at(day, h, m)
		if !t.After(now) {
			return out, errors.New("that time has already passed")
		}
		once(t)
	}

	rest := words[i:]
	for len(rest) > 0 {
		switch cleanWord(rest[0]) {
		case "to", "that", "-", ":", "–", "about", "for", "untuk":
			rest = rest[1:]
			continue
		}
		break
	}
	out.Title = strings.TrimSpace(strings.Join(rest, " "))
	if out.Title == "" {
		return out, errors.New("what should I remind you about? e.g. \"tomorrow 8am take medicine\"")
	}
	if len(out.Title) > 200 {
		out.Title = out.Title[:200]
	}
	return out, nil
}

// yearlyStart is the next (today or later) local date with month/day,
// clamped to month end.
func yearlyStart(today time.Time, mon time.Month, d int) time.Time {
	y := today.Year()
	mk := func(y int) time.Time { return time.Date(y, mon, min(d, daysIn(y, mon)), 0, 0, 0, 0, time.UTC) }
	t := mk(y)
	if t.Before(today) {
		t = mk(y + 1)
	}
	return t
}
