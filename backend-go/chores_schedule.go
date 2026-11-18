package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Pure chore math (unit tested in chores_test.go): which household-local
// dates a chore is due on, who gets each occurrence, and how many points an
// instance should carry for its status. Dates are calendar dates expressed
// as midnight UTC (same convention as householdToday).

const (
	choreOnce    = "once"
	choreDaily   = "daily"
	choreWeekly  = "weekly"
	choreMonthly = "monthly"

	choreTodo      = "todo"
	choreSubmitted = "submitted"
	choreApproved  = "approved"
	choreRejected  = "rejected"
	choreMissed    = "missed"
)

// maxBackfillDays: when the generator has been idle (no reads, server down)
// it only catches up this many past days, so a forgotten daily chore doesn't
// produce a month of "missed" rows.
const maxBackfillDays = 7

// maxGenerateDates caps one dueDates call (a year of daily occurrences).
const maxGenerateDates = 400

type choreSchedule struct {
	Kind       string
	Weekdays   []int // weekly: 0=Sunday..6; empty = Start's weekday
	DayOfMonth int   // monthly: 1..31 (0 = Start's day); clamped to the month's last day
	Start      time.Time
	End        *time.Time
}

func choreDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func daysInMonth(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// dueOn reports whether an occurrence falls on calendar day d.
func (s choreSchedule) dueOn(d time.Time) bool {
	d, start := choreDay(d), choreDay(s.Start)
	if d.Before(start) || (s.End != nil && d.After(choreDay(*s.End))) {
		return false
	}
	switch s.Kind {
	case choreOnce:
		return d.Equal(start)
	case choreDaily:
		return true
	case choreWeekly:
		days := s.Weekdays
		if len(days) == 0 {
			days = []int{int(start.Weekday())}
		}
		return slices.Contains(days, int(d.Weekday()))
	case choreMonthly:
		dom := s.DayOfMonth
		if dom <= 0 {
			dom = start.Day()
		}
		return d.Day() == min(dom, daysInMonth(d.Year(), d.Month()))
	}
	return false
}

// dueDates lists occurrences in [from, to] (inclusive), oldest first.
func (s choreSchedule) dueDates(from, to time.Time) []time.Time {
	from, to = choreDay(from), choreDay(to)
	if start := choreDay(s.Start); from.Before(start) {
		from = start
	}
	var out []time.Time
	for d := from; !d.After(to) && len(out) < maxGenerateDates; d = d.AddDate(0, 0, 1) {
		if s.dueOn(d) {
			out = append(out, d)
		}
	}
	return out
}

// generationWindow: the dates the generator should (re)visit for a chore,
// given its last generated day and the household's today. ok=false when
// there's nothing to do.
func generationWindow(start time.Time, lastGenerated *time.Time, today time.Time) (from, to time.Time, ok bool) {
	today = choreDay(today)
	from = choreDay(start)
	if lastGenerated != nil {
		if next := choreDay(*lastGenerated).AddDate(0, 0, 1); next.After(from) {
			from = next
		}
	}
	if floor := today.AddDate(0, 0, -maxBackfillDays); from.Before(floor) {
		from = floor
	}
	return from, today, !from.After(today)
}

// assignOccurrences returns the assignee of each of n consecutive
// occurrences plus the advanced cursor. Rotation wins over a fixed member;
// with neither, occurrences are unassigned (anyone may do them).
func assignOccurrences(fixed *int64, rotation []int64, cursor, n int) ([]*int64, int) {
	out := make([]*int64, n)
	for i := range out {
		if len(rotation) > 0 {
			id := rotation[((cursor%len(rotation))+len(rotation))%len(rotation)]
			out[i] = &id
			cursor++
		} else if fixed != nil {
			id := *fixed
			out[i] = &id
		}
	}
	return out, cursor
}

// choreRewardTarget: points an instance should carry for a status. Only
// approved instances (parent-approved, or completed on a chore that needs no
// approval) earn points.
func choreRewardTarget(status string, points int) int {
	if status == choreApproved {
		return points
	}
	return 0
}

// choreRewardDelta is the ledger change needed to move an instance from
// what it currently carries (awarded) to its status' target. Applying it
// repeatedly is a no-op, which makes approve/undo/reject idempotent.
func choreRewardDelta(awarded int, status string, points int) int {
	return choreRewardTarget(status, points) - awarded
}

// choreIsDone: the kid has done their part (approval may still be pending).
func choreIsDone(status string) bool { return status == choreSubmitted || status == choreApproved }

// parseDueTime validates a local "HH:MM".
func parseDueTime(s string) (h, m int, ok bool) {
	hs, ms, found := strings.Cut(strings.TrimSpace(s), ":")
	if !found || len(hs) == 0 || len(hs) > 2 || len(ms) != 2 {
		return 0, 0, false
	}
	h, err1 := strconv.Atoi(hs)
	m, err2 := strconv.Atoi(ms)
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

// dueInstant turns a local date + "HH:MM" into the UTC instant (RFC3339).
func dueInstant(day time.Time, dueTime string, loc *time.Location) string {
	h, m, ok := parseDueTime(dueTime)
	if !ok {
		return day.Format(dateLayout)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, loc).UTC().Format(time.RFC3339)
}

// localWeekStart: Monday of the week containing local date d.
func localWeekStart(d time.Time) time.Time {
	d = choreDay(d)
	return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
}

// choreDoneMessage is the friendly bot/app reply after completing a chore.
func choreDoneMessage(name string, forOther bool, points int, status string) string {
	var b strings.Builder
	if forOther {
		fmt.Fprintf(&b, "Marked done for %s!", name)
	} else {
		fmt.Fprintf(&b, "Nice work %s!", name)
	}
	if points > 0 {
		fmt.Fprintf(&b, " +%d points ⭐", points)
	} else {
		b.WriteString(" ✅")
	}
	if status == choreSubmitted {
		b.WriteString(" (waiting for approval)")
	}
	return b.String()
}
