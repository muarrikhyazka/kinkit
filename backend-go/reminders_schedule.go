package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Pure schedule math for reminders (no DB, unit tested).
//
// A schedule is human: "every Monday at 07:00" in the household's time zone.
// occurrencesBetween turns it into concrete UTC instants. DST: a local time
// that doesn't exist (spring-forward gap) fires at the shifted wall time Go
// picks (e.g. 02:30 -> 03:30); an ambiguous time (fall-back) fires once.
// Month-end: day_of_month 31 fires on the month's last day; a Feb 29 yearly
// date fires on Feb 28 in non-leap years.

type reminderSchedule struct {
	Kind       string     // once | daily | weekly | monthly | yearly
	TimeOfDay  string     // "HH:MM" local
	Weekdays   []int      // weekly: 0=Sunday..6
	DayOfMonth int        // monthly/yearly; 0 = start date's day
	StartDate  time.Time  // local calendar date (as UTC midnight)
	EndDate    *time.Time // inclusive local date, nil = forever
	OnceAt     *time.Time // kind once
	Interval   int        // weekly/monthly: every N weeks/months (0/1 = every); anchored on StartDate's week/month
}

// maxReminderInterval caps "every N weeks/months".
const maxReminderInterval = 52

// every returns the effective repeat interval (>= 1; only weekly/monthly use it).
func (s reminderSchedule) every() int {
	if (s.Kind == "weekly" || s.Kind == "monthly") && s.Interval > 1 {
		return s.Interval
	}
	return 1
}

// weekStart is the Sunday on or before local date d (weekdays are 0=Sunday).
func weekStart(d time.Time) time.Time {
	return dateOnly(d).AddDate(0, 0, -int(d.Weekday()))
}

var validReminderKinds = map[string]bool{"once": true, "daily": true, "weekly": true, "monthly": true, "yearly": true}

// parseHHMM parses "7:05" / "07:05" into hour, minute.
func parseHHMM(s string) (int, int, error) {
	h, m, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok {
		return 0, 0, fmt.Errorf("time must be HH:MM")
	}
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 || len(m) != 2 {
		return 0, 0, fmt.Errorf("time must be HH:MM")
	}
	return hh, mm, nil
}

func daysIn(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// validate checks a schedule is complete enough to compute.
func (s reminderSchedule) validate() error {
	if !validReminderKinds[s.Kind] {
		return fmt.Errorf("kind must be once, daily, weekly, monthly or yearly")
	}
	if s.Kind == "once" {
		if s.OnceAt == nil {
			return fmt.Errorf("a one-time reminder needs a date and time")
		}
		return nil
	}
	if _, _, err := parseHHMM(s.TimeOfDay); err != nil {
		return err
	}
	for _, w := range s.Weekdays {
		if w < 0 || w > 6 {
			return fmt.Errorf("weekdays must be 0 (Sunday) to 6 (Saturday)")
		}
	}
	if s.Interval < 0 || s.Interval > maxReminderInterval {
		return fmt.Errorf("interval must be 1-%d", maxReminderInterval)
	}
	if s.DayOfMonth < 0 || s.DayOfMonth > 31 {
		return fmt.Errorf("day_of_month must be 1-31")
	}
	if s.EndDate != nil && s.EndDate.Before(s.StartDate) {
		return fmt.Errorf("end date is before start date")
	}
	return nil
}

// matchesDay reports whether the schedule fires on local date d.
func (s reminderSchedule) matchesDay(d time.Time) bool {
	if d.Before(dateOnly(s.StartDate)) || (s.EndDate != nil && d.After(dateOnly(*s.EndDate))) {
		return false
	}
	dom := s.DayOfMonth
	if dom == 0 {
		dom = s.StartDate.Day()
	}
	switch s.Kind {
	case "daily":
		return true
	case "weekly":
		days := s.Weekdays
		if len(days) == 0 {
			days = []int{int(s.StartDate.Weekday())}
		}
		if n := s.every(); n > 1 {
			weeks := int(weekStart(d).Sub(weekStart(s.StartDate)).Hours()/24+0.5) / 7
			if weeks%n != 0 {
				return false
			}
		}
		for _, w := range days {
			if int(d.Weekday()) == w {
				return true
			}
		}
		return false
	case "monthly":
		if n := s.every(); n > 1 {
			months := (d.Year()*12 + int(d.Month())) - (s.StartDate.Year()*12 + int(s.StartDate.Month()))
			if months%n != 0 {
				return false
			}
		}
		return d.Day() == min(dom, daysIn(d.Year(), d.Month()))
	case "yearly":
		return d.Month() == s.StartDate.Month() && d.Day() == min(dom, daysIn(d.Year(), d.Month()))
	}
	return false
}

// occurrencesBetween returns the schedule's due instants t with from <= t < to,
// sorted, in UTC. At most maxOcc results.
func occurrencesBetween(s reminderSchedule, loc *time.Location, from, to time.Time) []time.Time {
	const maxOcc = 500
	if loc == nil {
		loc = time.UTC
	}
	if s.Kind == "once" {
		if s.OnceAt != nil && !s.OnceAt.Before(from) && s.OnceAt.Before(to) {
			return []time.Time{s.OnceAt.UTC()}
		}
		return nil
	}
	hh, mm, err := parseHHMM(s.TimeOfDay)
	if err != nil || !from.Before(to) {
		return nil
	}
	// Walk local calendar days covering [from, to) with a day of slack on
	// both sides (zone offsets), capped to keep runaway ranges cheap
	// (800 days: enough for a year-long lookahead of "every 6 months").
	first := dateOnly(from.In(loc)).AddDate(0, 0, -1)
	last := dateOnly(to.In(loc)).AddDate(0, 0, 1)
	if sd := dateOnly(s.StartDate); first.Before(sd) {
		first = sd
	}
	if s.EndDate != nil && last.After(dateOnly(*s.EndDate)) {
		last = dateOnly(*s.EndDate)
	}
	var out []time.Time
	for d, n := first, 0; !d.After(last) && n < 800; d, n = d.AddDate(0, 0, 1), n+1 {
		if !s.matchesDay(d) {
			continue
		}
		t := time.Date(d.Year(), d.Month(), d.Day(), hh, mm, 0, 0, loc).UTC()
		if t.Before(from) || !t.Before(to) {
			continue
		}
		out = append(out, t)
		if len(out) >= maxOcc {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

var weekdayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

func ordinalSuffix(n int) string {
	if n%100 >= 11 && n%100 <= 13 {
		return strconv.Itoa(n) + "th"
	}
	switch n % 10 {
	case 1:
		return strconv.Itoa(n) + "st"
	case 2:
		return strconv.Itoa(n) + "nd"
	case 3:
		return strconv.Itoa(n) + "rd"
	}
	return strconv.Itoa(n) + "th"
}

// describeSchedule renders "Every Monday at 07:00" (mirrors the frontend preview).
func describeSchedule(s reminderSchedule, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	dom := s.DayOfMonth
	if dom == 0 {
		dom = s.StartDate.Day()
	}
	at := " at " + s.TimeOfDay
	switch s.Kind {
	case "once":
		if s.OnceAt == nil {
			return "Once"
		}
		return s.OnceAt.In(loc).Format("Mon 2 Jan 2006 at 15:04")
	case "daily":
		return "Every day" + at
	case "weekly":
		days := append([]int(nil), s.Weekdays...)
		if len(days) == 0 {
			days = []int{int(s.StartDate.Weekday())}
		}
		sort.Ints(days)
		if n := s.every(); n > 1 {
			short := make([]string, len(days))
			for i, d := range days {
				short[i] = weekdayNames[d][:3]
			}
			return "Every " + strconv.Itoa(n) + " weeks on " + joinAnd(short) + at
		}
		if len(days) == 7 {
			return "Every day" + at
		}
		if fmt.Sprint(days) == "[1 2 3 4 5]" {
			return "Every weekday" + at
		}
		if fmt.Sprint(days) == "[0 6]" {
			return "Every weekend" + at
		}
		names := make([]string, len(days))
		for i, d := range days {
			names[i] = weekdayNames[d]
		}
		return "Every " + joinAnd(names) + at
	case "monthly":
		if n := s.every(); n > 1 {
			return "Every " + strconv.Itoa(n) + " months on the " + ordinalSuffix(dom) + at
		}
		return "Every month on the " + ordinalSuffix(dom) + at
	case "yearly":
		return "Every year on " + s.StartDate.Month().String()[:3] + " " + strconv.Itoa(dom) + at
	}
	return s.Kind
}

func joinAnd(xs []string) string {
	switch len(xs) {
	case 0:
		return ""
	case 1:
		return xs[0]
	}
	return strings.Join(xs[:len(xs)-1], ", ") + " and " + xs[len(xs)-1]
}

// localDayBounds returns the UTC instants of the start of the local day
// containing t and the start of the next local day (DST-aware: 23h/25h days).
func localDayBounds(t time.Time, loc *time.Location) (time.Time, time.Time) {
	y, m, d := t.In(loc).Date()
	start := time.Date(y, m, d, 0, 0, 0, 0, loc)
	end := time.Date(y, m, d+1, 0, 0, 0, 0, loc)
	return start.UTC(), end.UTC()
}

// sameLocalTimeNextDay: snooze "until tomorrow" keeps the wall-clock time
// of `due` on the local day after `now`.
func sameLocalTimeNextDay(due, now time.Time, loc *time.Location) time.Time {
	l := due.In(loc)
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d+1, l.Hour(), l.Minute(), 0, 0, loc).UTC()
}
