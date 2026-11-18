package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Today page: GET /api/today runs every registered TodayProvider (see
// home_contract.go) with a timeout each; a failing provider is logged and
// skipped so the page still renders. The morning digest and the bots'
// /today reuse the same sections.

const todayProviderTimeout = 4 * time.Second

func init() {
	registerTodayProvider("reminders", 10, todayRemindersProvider)
	registerTodayProvider("budget", 50, todayBudgetProvider)
	registerTodayProvider("coming_up", 90, todayComingUpProvider)
}

// runTodayProviders builds all sections concurrently, preserving provider
// order. Sections with no items and no summary are dropped.
func runTodayProviders(ctx context.Context, app *App, aid int64, m memberInfo, day time.Time) []TodaySection {
	provs := listTodayProviders()
	results := make([]*TodaySection, len(provs))
	done := make(chan struct{}, len(provs))
	for i, p := range provs {
		go func(i int, p namedTodayProvider) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("today: provider %s panic: %v", p.key, rec)
				}
				done <- struct{}{}
			}()
			pctx, cancel := context.WithTimeout(ctx, todayProviderTimeout)
			defer cancel()
			ch := make(chan *TodaySection, 1)
			go func() {
				defer func() {
					if rec := recover(); rec != nil {
						log.Printf("today: provider %s panic: %v", p.key, rec)
						ch <- nil
					}
				}()
				s, err := p.fn(pctx, app, aid, m, day)
				if err != nil {
					log.Printf("today: provider %s (account %d): %v", p.key, aid, err)
					s = nil
				}
				ch <- s
			}()
			select {
			case s := <-ch:
				results[i] = s
			case <-pctx.Done():
				log.Printf("today: provider %s (account %d) timed out", p.key, aid)
			}
		}(i, p)
	}
	for range provs {
		<-done
	}
	out := []TodaySection{}
	for i, s := range results {
		if s == nil || (len(s.Items) == 0 && s.Summary == "") {
			continue
		}
		if s.Key == "" {
			s.Key = provs[i].key
		}
		if s.Items == nil {
			s.Items = []TodayItem{}
		}
		s.Order = provs[i].order
		out = append(out, *s)
	}
	return out
}

func greetingFor(t time.Time) string {
	switch h := t.Hour(); {
	case h >= 5 && h < 12:
		return "Good morning"
	case h >= 12 && h < 17:
		return "Good afternoon"
	case h >= 17 && h < 22:
		return "Good evening"
	}
	return "Hello"
}

func (app *App) memberDisplayName(ctx context.Context, m memberInfo) string {
	if m.ID == 0 {
		return ""
	}
	var n string
	_ = app.DB.QueryRow(ctx, "SELECT COALESCE(NULLIF(display_name,''), split_part(email,'@',1)) FROM members WHERE id=$1", m.ID).Scan(&n)
	return n
}

func (app *App) handleToday(w http.ResponseWriter, r *http.Request) {
	m, ok := requireMember(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	aid := m.AccountID
	now := time.Now().UTC()
	loc := householdLocation(ctx, app, aid)
	day := householdToday(ctx, app, aid, now)
	name := app.memberDisplayName(ctx, m)
	greeting := greetingFor(now.In(loc))
	if name != "" {
		greeting += ", " + name
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date":             day.Format("2006-01-02"),
		"timezone":         loc.String(),
		"timezone_default": loc.String() == "UTC",
		"greeting":         greeting,
		"member_name":      name,
		"role":             m.Role,
		"sections":         runTodayProviders(ctx, app, aid, m, day),
	})
}

// ---------- providers ----------

func localDayFromDate(day time.Time, loc *time.Location) (time.Time, time.Time) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	end := time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, loc)
	return start.UTC(), end.UTC()
}

func todayRemindersProvider(ctx context.Context, app *App, aid int64, m memberInfo, day time.Time) (*TodaySection, error) {
	now := time.Now().UTC()
	if _, err := app.materialize(ctx, aid, now, materializeHorizon); err != nil {
		return nil, err
	}
	loc := householdLocation(ctx, app, aid)
	start, end := localDayFromDate(day, loc)
	occ, err := app.queryOccurrences(ctx, aid, m,
		`(o.due_at >= $2 AND o.due_at < $3) OR (o.status='snoozed' AND o.snoozed_until >= $2 AND o.snoozed_until < $3)`, start, end)
	if err != nil {
		return nil, err
	}
	sec := &TodaySection{Key: "reminders", Title: "Reminders", Items: []TodayItem{}}
	open := 0
	for _, o := range occ {
		it := TodayItem{Kind: "reminder", ID: o.ID, Title: o.Title, Due: o.effective().Format(time.RFC3339), MemberID: o.AssigneeID,
			MemberName: o.AssigneeName, Done: o.Status == "done", Overdue: o.Overdue, Link: "/home/reminders"}
		it.Detail = o.DueAt.In(loc).Format("15:04")
		switch o.Status {
		case "snoozed":
			it.Detail = "snoozed until " + o.SnoozedUntil.In(loc).Format("15:04")
		case "missed":
			it.Detail += " · missed"
		case "done":
			if o.DoneByName != "" {
				it.Detail += " · done by " + o.DoneByName
			}
		}
		if o.open() {
			open++
			if o.CanAct {
				it.Actions = []string{"done", "snooze"}
			}
		}
		sec.Items = append(sec.Items, it)
	}
	if len(sec.Items) == 0 {
		return nil, nil
	}
	if open == 0 {
		sec.Summary = "All reminders done. Nice!"
	} else {
		sec.Summary = fmt.Sprintf("%d reminder%s left today", open, plural(open))
	}
	return sec, nil
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func todayComingUpProvider(ctx context.Context, app *App, aid int64, m memberInfo, day time.Time) (*TodaySection, error) {
	now := time.Now().UTC()
	if _, err := app.materialize(ctx, aid, now, 31*24*time.Hour); err != nil {
		return nil, err
	}
	loc := householdLocation(ctx, app, aid)
	_, tomorrow := localDayFromDate(day, loc)
	weekEnd := time.Date(day.Year(), day.Month(), day.Day()+8, 0, 0, 0, 0, loc).UTC()
	monthEnd := time.Date(day.Year(), day.Month(), day.Day()+31, 0, 0, 0, 0, loc).UTC()
	occ, err := app.queryOccurrences(ctx, aid, m,
		`o.status IN ('pending','snoozed') AND o.due_at >= $2 AND (o.due_at < $3 OR (r.category='birthday' AND o.due_at < $4))`, tomorrow, weekEnd, monthEnd)
	if err != nil {
		return nil, err
	}
	sec := &TodaySection{Key: "coming_up", Title: "Coming up", Items: []TodayItem{}}
	seen := map[int64]bool{} // daily reminders would flood the list: show each rule once
	for _, o := range occ {
		if seen[o.ReminderID] {
			continue
		}
		seen[o.ReminderID] = true
		title := o.Title
		if o.Category == "birthday" && !strings.Contains(strings.ToLower(title), "birthday") {
			title = "Birthday: " + title
		}
		sec.Items = append(sec.Items, TodayItem{Kind: "reminder", ID: o.ID, Title: title, Due: o.DueAt.Format(time.RFC3339),
			Detail: o.DueAt.In(loc).Format("Mon 2 Jan · 15:04"), MemberID: o.AssigneeID, MemberName: o.AssigneeName, Link: "/home/reminders"})
		if len(sec.Items) >= 8 {
			break
		}
	}
	if len(sec.Items) == 0 {
		return nil, nil
	}
	return sec, nil
}

// todayBudgetProvider: spent vs budget for the household-local month and
// the category furthest over budget. Not shown to kids.
func todayBudgetProvider(ctx context.Context, app *App, aid int64, m memberInfo, day time.Time) (*TodaySection, error) {
	if m.Role == roleKid {
		return nil, nil
	}
	year, month := day.Year(), int(day.Month())
	from, to := monthRange(year, month)
	var budget, spent int64
	err := app.DB.QueryRow(ctx, `SELECT
	    COALESCE((SELECT SUM(be.amount) FROM budget_expense be JOIN categories c ON c.id=be.category_id
	              WHERE be.account_id=$1 AND be.year=$2 AND be.month=$3 AND c.main_category<>'Savings'),0),
	    COALESCE((SELECT SUM(sp.amount) FROM spending sp LEFT JOIN categories c ON c.id=sp.category_id
	              WHERE sp.account_id=$1 AND sp.tx_date >= $4 AND sp.tx_date < $5 AND COALESCE(c.main_category,'')<>'Savings'),0)`,
		aid, year, month, from, to).Scan(&budget, &spent)
	if err != nil {
		return nil, err
	}
	if budget == 0 && spent == 0 {
		return nil, nil
	}
	var currency string
	_ = app.DB.QueryRow(ctx, "SELECT COALESCE(currency,'IDR') FROM users WHERE id=$1", aid).Scan(&currency)
	var rates map[string]float64
	if c := loadFxCacheFile(fxCachePath()); c != nil {
		rates = c.Rates
	}
	f := func(v int64) string { return formatAlertAmount(v, currency, rates) }
	sec := &TodaySection{Key: "budget", Title: "Budget", Items: []TodayItem{}}
	if budget > 0 {
		pct := spent * 100 / budget
		sec.Summary = fmt.Sprintf("Spent %s of %s this month (%d%%)", f(spent), f(budget), pct)
		left := budget - spent
		detail := f(left) + " left"
		if left < 0 {
			detail = f(-left) + " over"
		}
		sec.Items = append(sec.Items, TodayItem{Kind: "budget", Title: fmt.Sprintf("%d%% of this month's budget used", pct), Detail: detail, Link: "/budget", Overdue: left < 0})
	} else {
		sec.Summary = fmt.Sprintf("Spent %s this month (no budget set)", f(spent))
		sec.Items = append(sec.Items, TodayItem{Kind: "budget", Title: "Spent " + f(spent) + " this month", Detail: "Set a budget to track it", Link: "/budget"})
	}
	var cat string
	var cb, cs int64
	err = app.DB.QueryRow(ctx, `SELECT c.name, be.amount,
	    COALESCE((SELECT SUM(sp.amount) FROM spending sp WHERE sp.account_id=$1 AND sp.category_id=c.id AND sp.tx_date >= $4 AND sp.tx_date < $5),0) AS s
	  FROM budget_expense be JOIN categories c ON c.id=be.category_id
	  WHERE be.account_id=$1 AND be.year=$2 AND be.month=$3 AND be.amount > 0
	  ORDER BY s - be.amount DESC LIMIT 1`, aid, year, month, from, to).Scan(&cat, &cb, &cs)
	if err == nil && cs > cb {
		sec.Items = append(sec.Items, TodayItem{Kind: "budget", Title: cat + " is over budget", Detail: fmt.Sprintf("%s of %s", f(cs), f(cb)), Overdue: true, Link: "/budget"})
	}
	return sec, nil
}
