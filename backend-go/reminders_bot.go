package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Bot-token endpoints for reminders / Today. The chat user comes from
// X-Bot-User; an unlinked chat user in the household chat acts as an
// anonymous household member (may complete/snooze, can't create).

// botMember resolves the chat user (linked member) or an anonymous one.
func botMember(ctx context.Context, app *App, r *http.Request, aid int64) (memberInfo, bool) {
	if m, ok := botActor(ctx, app, r, aid); ok {
		return m, true
	}
	return memberInfo{AccountID: aid}, false
}

// GET /api/bot/today -> {text, date, timezone}
func botToday(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	ctx := r.Context()
	m, linked := botMember(ctx, app, r, aid)
	if !linked {
		m = householdView(aid)
	}
	now := time.Now().UTC()
	loc := householdLocation(ctx, app, aid)
	writeJSON(w, http.StatusOK, map[string]any{
		"text": app.digestText(ctx, aid, m, now), "date": householdToday(ctx, app, aid, now).Format("2006-01-02"), "timezone": loc.String(),
	})
}

// GET /api/bot/reminders -> {text, items}
func botListReminders(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	ctx := r.Context()
	m, linked := botMember(ctx, app, r, aid)
	if !linked {
		m = householdView(aid)
	}
	now := time.Now().UTC()
	if _, err := app.materialize(ctx, aid, now, materializeHorizon); err != nil {
		dbError(w, err)
		return
	}
	loc := householdLocation(ctx, app, aid)
	occ, err := app.queryOccurrences(ctx, aid, m,
		`o.status IN ('pending','sent','snoozed') AND COALESCE(CASE WHEN o.status='snoozed' THEN o.snoozed_until END, o.due_at) >= $2
		 AND COALESCE(CASE WHEN o.status='snoozed' THEN o.snoozed_until END, o.due_at) < $3`,
		now.Add(-12*time.Hour), now.Add(7*24*time.Hour))
	if err != nil {
		dbError(w, err)
		return
	}
	type item struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Due   string `json:"due"`
		Who   string `json:"who,omitempty"`
		// category id + human label (e.g. "laundry" / "Laundry")
		Category      string `json:"category"`
		CategoryLabel string `json:"category_label"`
	}
	items := []item{}
	var b strings.Builder
	b.WriteString("Upcoming reminders (7 days)")
	for i, o := range occ {
		if i == 15 {
			b.WriteString(fmt.Sprintf("\n…and %d more", len(occ)-i))
			break
		}
		when := o.effective().In(loc).Format("Mon 2 Jan 15:04")
		label := reminderCategoryLabel(o.Category)
		mark := "•"
		if o.Overdue {
			mark = "!"
		}
		line := "\n" + mark + " " + when + ": " + o.Title + " [" + label + "]"
		if o.AssigneeName != "" {
			line += " (" + o.AssigneeName + ")"
		}
		b.WriteString(line)
		items = append(items, item{o.ID, o.Title, o.effective().Format(time.RFC3339), o.AssigneeName, o.Category, label})
	}
	if len(occ) == 0 {
		b.Reset()
		b.WriteString("No reminders in the next 7 days. Add one with /remind tomorrow 8am take medicine")
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": b.String(), "items": items})
}

// findMemberByName matches "@name" against display names / email local parts.
func (app *App) findMemberByName(ctx context.Context, aid int64, name string) (int64, string, error) {
	rows, err := app.DB.Query(ctx, `SELECT id, COALESCE(NULLIF(display_name,''), split_part(email,'@',1)), split_part(email,'@',1) FROM members WHERE account_id=$1 ORDER BY id`, aid)
	if err != nil {
		return 0, "", err
	}
	defer rows.Close()
	type mem struct {
		id          int64
		name, local string
	}
	var all []mem
	for rows.Next() {
		var x mem
		if err := rows.Scan(&x.id, &x.name, &x.local); err != nil {
			return 0, "", err
		}
		all = append(all, x)
	}
	q := strings.ToLower(name)
	norm := func(s string) string { return strings.ToLower(strings.ReplaceAll(s, " ", "")) }
	for _, x := range all { // exact
		if norm(x.name) == q || strings.ToLower(x.local) == q || firstWord(x.name) == q {
			return x.id, x.name, nil
		}
	}
	var hit *mem
	for i, x := range all { // unique prefix
		if strings.HasPrefix(norm(x.name), q) {
			if hit != nil {
				return 0, "", fmt.Errorf("more than one member matches @%s", name)
			}
			hit = &all[i]
		}
	}
	if hit != nil {
		return hit.id, hit.name, nil
	}
	names := make([]string, 0, len(all))
	for _, x := range all {
		names = append(names, x.name)
	}
	return 0, "", fmt.Errorf("no member called @%s (members: %s)", name, strings.Join(names, ", "))
}

// POST /api/bot/reminders {text} -> {ok, message, id}
func botCreateReminder(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	ctx := r.Context()
	m, linked := botMember(ctx, app, r, aid)
	reply := func(ok bool, msg string, extra ...any) {
		out := map[string]any{"ok": ok, "message": msg}
		for i := 0; i+1 < len(extra); i += 2 {
			out[extra[i].(string)] = extra[i+1]
		}
		writeJSON(w, http.StatusOK, out)
	}
	if linked && !canManageFamily(m.Role) {
		reply(false, "Only parents can add reminders. Ask a parent, or add it in the app.")
		return
	}
	var d struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &d); err != nil || strings.TrimSpace(d.Text) == "" {
		reply(false, "Usage: /remind tomorrow 8am take medicine · /remind every tue and fri 19:00 take out the trash @budi · /remind every 2 weeks on fri 10:00 change bed sheets · /remind every 3 months on the 1st change car oil")
		return
	}
	loc := householdLocation(ctx, app, aid)
	now := time.Now().UTC()
	p, err := parseReminderText(d.Text, now, loc)
	if err != nil {
		reply(false, err.Error())
		return
	}
	in := reminderInput{Title: p.Title, Kind: p.Schedule.Kind, TimeOfDay: p.Schedule.TimeOfDay, Weekdays: p.Schedule.Weekdays,
		StartDate: p.Schedule.StartDate.Format("2006-01-02"), OnceAt: p.Schedule.OnceAt, Interval: p.Schedule.Interval,
		Category: guessReminderCategory(p.Title)}
	if p.Schedule.DayOfMonth > 0 {
		dom := p.Schedule.DayOfMonth
		in.DayOfMonth = &dom
	}
	who := "everyone"
	if p.Assignee != "" {
		if strings.EqualFold(p.Assignee, "me") && linked {
			in.AssigneeID = &m.ID
			who = app.memberDisplayName(ctx, m)
		} else {
			id, name, err := app.findMemberByName(ctx, aid, p.Assignee)
			if err != nil {
				reply(false, err.Error())
				return
			}
			in.AssigneeID, who = &id, name
		}
	}
	v, err := app.validateReminderInput(ctx, aid, in, loc, false)
	if err != nil {
		reply(false, err.Error())
		return
	}
	var createdBy *int64
	if linked {
		createdBy = &m.ID
	}
	id, err := app.insertReminder(ctx, aid, createdBy, v)
	if err != nil {
		dbError(w, err)
		return
	}
	reply(true, fmt.Sprintf("Reminder set: %s\n%s · for %s · %s", v.in.Title, describeSchedule(v.sched, loc), who, reminderCategoryLabel(v.in.Category)),
		"id", id, "category", v.in.Category, "interval", v.in.Interval, "description", describeSchedule(v.sched, loc))
}

// guessReminderCategory picks a category from keywords in the title.
func guessReminderCategory(title string) string {
	t := strings.ToLower(title)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(t, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("birthday", "bday", "ulang tahun", "anniversary"):
		return "birthday"
	case has("medicine", "pill", "vitamin", "obat", "meds", "insulin", "doctor", "dentist"):
		return "medicine"
	case has("pay", "bill", "rent", "invoice", "tax", "insurance", "bayar", "tagihan"):
		return "bill"
	case has("school", "homework", "pr ", "exam", "class", "sekolah"):
		return "school"
	case has("laundry", "curtain", "bed sheet", "bedsheet", "sheets", "towel", "sarung", "prayer mat", "sajadah", "ironing", "cuci baju", "seprai", "sprei", "handuk", "setrika"):
		return "laundry"
	case has("grocer", "supermarket", "market run", "belanja", "pasar"):
		return "groceries"
	case has("meal", "cook", "kitchen", "fridge", "recipe", "masak", "dapur", "kulkas"):
		return "cooking"
	case has("tyre", "tire", "oil change", "motorbike", "motor ", "car ", "vehicle", "service the car", "stnk", "ban ", "oli", "mobil"):
		return "vehicle"
	case has("water tank", "tandon", "toren", "filter", "aircon", " ac", "ac ", "plants", "water the", "garden", "gutter", "siram", "tanaman", "kuras"):
		return "home_maintenance"
	case has("self-care", "self care", "me time", "workout", "gym", "skincare", "haircut", "meditat", "olahraga"):
		return "self_care"
	case has("trash", "bins", "clean", "dishes", "vacuum", "mop", "sweep", "bathroom", "toilet", "sampah", "sapu", "pel", "bersih"):
		return "cleaning"
	}
	return "other"
}

func botOccurrenceDone(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	id, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := r.Context()
	m, _ := botMember(ctx, app, r, aid)
	o, already, err := app.markOccurrenceDone(ctx, aid, m, id)
	if err != nil {
		botOccError(w, err)
		return
	}
	msg := "Done: " + o.Title
	if n := app.memberDisplayName(ctx, m); n != "" {
		msg += ", by " + n
	}
	if already {
		msg = "Already done: " + o.Title
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg})
}

func botOccurrenceSnooze(w http.ResponseWriter, r *http.Request, app *App, aid int64) {
	id, ok := pathInt64(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var b snoozeBody
	if err := decodeJSON(r, &b); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx := r.Context()
	m, _ := botMember(ctx, app, r, aid)
	o, until, err := app.snoozeOccurrence(ctx, aid, m, id, b.Minutes, b.Until)
	if err != nil {
		botOccError(w, err)
		return
	}
	loc := householdLocation(ctx, app, aid)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Snoozed: " + o.Title + " until " + until.In(loc).Format("Mon 15:04")})
}

// botOccError: chat-friendly errors (200 + ok:false) for expected cases.
func botOccError(w http.ResponseWriter, err error) {
	var pe interface{ SQLState() string }
	if errors.As(err, &pe) {
		dbError(w, err)
		return
	}
	msg := err.Error()
	if strings.Contains(msg, "no rows") {
		msg = "That reminder no longer exists."
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": msg})
}

// reminderCategoryLabel is the human label for a category id.
func reminderCategoryLabel(c string) string {
	if l, ok := reminderCategoryLabels[c]; ok {
		return l
	}
	return "Other"
}

func firstWord(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return strings.ToLower(f[0])
	}
	return ""
}
