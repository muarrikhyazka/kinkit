package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"
)

// Reminders background worker: every minute it
//  1. materializes upcoming occurrences (idempotent: UNIQUE(reminder_id, due_at)),
//  2. marks open occurrences missed once their local day is over,
//  3. sends due occurrences to chat (claim first, release if every send fails),
//  4. escalates unacknowledged ones to the reminder's escalate_to member,
//  5. sends the morning digest (digest.go).

const (
	remindersInterval  = time.Minute
	materializeHorizon = 8 * 24 * time.Hour
)

func (app *App) startRemindersWorker(ctx context.Context) {
	go func() {
		t := time.NewTicker(remindersInterval)
		defer t.Stop()
		for {
			app.remindersPass(ctx, time.Now().UTC())
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

// remindersPass runs one full worker pass; each step is panic-safe and logs.
func (app *App) remindersPass(ctx context.Context, now time.Time) {
	step := func(name string, fn func(context.Context) error) {
		defer func() {
			if p := recover(); p != nil {
				log.Printf("reminders worker: %s panic: %v", name, p)
			}
		}()
		pctx, cancel := context.WithTimeout(ctx, 50*time.Second)
		defer cancel()
		if err := fn(pctx); err != nil {
			log.Printf("reminders worker: %s: %v", name, err)
		}
	}
	step("materialize", func(c context.Context) error { _, err := app.materialize(c, 0, now, materializeHorizon); return err })
	step("missed", func(c context.Context) error { _, err := app.markMissed(c, now); return err })
	step("send", func(c context.Context) error { _, err := app.sendDueReminders(c, now); return err })
	step("escalate", func(c context.Context) error { _, err := app.escalateReminders(c, now); return err })
	step("digest", func(c context.Context) error { _, err := app.sendDigests(c, now); return err })
}

// materialize creates occurrences in [schedule_from-ish, now+horizon) for
// active reminders of one household (aid>0) or all (aid=0). Returns the
// number of new rows.
func (app *App) materialize(ctx context.Context, aid int64, now time.Time, horizon time.Duration) (int, error) {
	to := now.Add(horizon).UTC().Truncate(time.Minute)
	q := `SELECT ` + reminderCols + `, r.account_id, u.timezone, r.schedule_from, r.materialized_until
	  FROM reminders r JOIN users u ON u.id=r.account_id
	  WHERE r.active AND NOT u.is_disabled AND (r.materialized_until IS NULL OR r.materialized_until < $1)`
	args := []any{to}
	if aid > 0 {
		q += " AND r.account_id=$2"
		args = append(args, aid)
	}
	rows, err := app.DB.Query(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	type job struct {
		r        reminderRow
		aid      int64
		tz       string
		from     time.Time
		matUntil *time.Time
	}
	var jobs []job
	for rows.Next() {
		var j job
		var wd []int32
		var dom, esc *int32
		var start time.Time
		var end *time.Time
		x := &j.r
		if err := rows.Scan(&x.ID, &x.Title, &x.Notes, &x.AssigneeID, &x.CreatedBy, &x.Kind, &x.TimeOfDay, &wd,
			&dom, &start, &end, &x.OnceAt, &x.EscalateTo, &esc, &x.Channels, &x.Active, &x.Category, &x.Interval,
			&j.aid, &j.tz, &j.from, &j.matUntil); err != nil {
			rows.Close()
			return 0, err
		}
		for _, v := range wd {
			x.Weekdays = append(x.Weekdays, int(v))
		}
		if dom != nil {
			v := int(*dom)
			x.DayOfMonth = &v
		}
		x.StartDate = start.Format("2006-01-02")
		if end != nil {
			s := end.Format("2006-01-02")
			x.EndDate = &s
		}
		jobs = append(jobs, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	locs := map[string]*time.Location{}
	total := 0
	for _, j := range jobs {
		loc, ok := locs[j.tz]
		if !ok {
			loc, err = time.LoadLocation(j.tz)
			if err != nil {
				loc = time.UTC
			}
			locs[j.tz] = loc
		}
		from := j.from
		if j.matUntil != nil && j.matUntil.After(from) {
			from = *j.matUntil
		}
		times := occurrencesBetween(j.r.schedule(), loc, from, to)
		if len(times) > 0 {
			tag, err := app.DB.Exec(ctx, `INSERT INTO reminder_occurrences (reminder_id, account_id, due_at)
			  SELECT $1, $2, unnest($3::timestamptz[]) ON CONFLICT (reminder_id, due_at) DO NOTHING`, j.r.ID, j.aid, times)
			if err != nil {
				return total, err
			}
			total += int(tag.RowsAffected())
		}
		if _, err := app.DB.Exec(ctx, "UPDATE reminders SET materialized_until=GREATEST(COALESCE(materialized_until, $1), $1) WHERE id=$2", to, j.r.ID); err != nil {
			return total, err
		}
	}
	return total, nil
}

// markMissed flips open occurrences to 'missed' once the household-local day
// of their (snoozed) due time has ended.
func (app *App) markMissed(ctx context.Context, now time.Time) (int, error) {
	rows, err := app.DB.Query(ctx, `SELECT o.id, COALESCE(CASE WHEN o.status='snoozed' THEN o.snoozed_until END, o.due_at), u.timezone
	  FROM reminder_occurrences o JOIN users u ON u.id=o.account_id
	  WHERE o.status IN ('pending','sent','snoozed') AND COALESCE(CASE WHEN o.status='snoozed' THEN o.snoozed_until END, o.due_at) < $1`, now)
	if err != nil {
		return 0, err
	}
	var ids []int64
	locs := map[string]*time.Location{}
	for rows.Next() {
		var id int64
		var eff time.Time
		var tz string
		if err := rows.Scan(&id, &eff, &tz); err != nil {
			rows.Close()
			return 0, err
		}
		loc, ok := locs[tz]
		if !ok {
			if loc, err = time.LoadLocation(tz); err != nil {
				loc = time.UTC
			}
			locs[tz] = loc
		}
		if _, dayEnd := localDayBounds(eff, loc); !now.Before(dayEnd) {
			ids = append(ids, id)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(ids) == 0 {
		return 0, err
	}
	tag, err := app.DB.Exec(ctx, "UPDATE reminder_occurrences SET status='missed' WHERE id = ANY($1) AND status IN ('pending','sent','snoozed')", ids)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

type dueOcc struct {
	id, aid       int64
	status        string
	prevSentAt    *time.Time
	dueAt         time.Time
	title, notes  string
	assignee      *int64
	escalateTo    *int64
	channels      []string
	tz            string
	tgUser, dcUsr *string // assignee's linked chat ids (or escalate_to's, for escalation)
	who           string
}

func (d dueOcc) mention(platform string) string {
	p := d.tgUser
	if platform == "discord" {
		p = d.dcUsr
	}
	if p == nil {
		return ""
	}
	return *p
}

func loadLoc(tz string) *time.Location {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

// deliver sends msg (with per-platform mention) to every channel; returns
// how many succeeded.
func (app *App) deliver(ctx context.Context, channels []alertChannel, mk func(platform string) chatMessage, what string) int {
	ok := 0
	for _, ch := range channels {
		sctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := sendReminderChat(sctx, ch.platform, ch.token, ch.chatID, mk(ch.platform))
		cancel()
		if err != nil {
			log.Printf("reminders: %s: %v", what, err)
			continue
		}
		ok++
	}
	return ok
}

// sendDueReminders delivers occurrences that became due (or whose snooze
// ended). Returns how many were marked sent.
func (app *App) sendDueReminders(ctx context.Context, now time.Time) (int, error) {
	rows, err := app.DB.Query(ctx, `SELECT o.id, o.account_id, o.status, o.sent_at, o.due_at, r.title, r.notes, r.assignee_id, r.channels, u.timezone,
	    m.telegram_user_id, m.discord_user_id, COALESCE(NULLIF(m.display_name,''), split_part(m.email,'@',1), '')
	  FROM reminder_occurrences o JOIN reminders r ON r.id=o.reminder_id JOIN users u ON u.id=o.account_id
	  LEFT JOIN members m ON m.id=r.assignee_id
	  WHERE r.active AND ((o.status='pending' AND o.due_at <= $1) OR (o.status='snoozed' AND o.snoozed_until <= $1))
	  ORDER BY o.due_at LIMIT 500`, now)
	if err != nil {
		return 0, err
	}
	var due []dueOcc
	for rows.Next() {
		var d dueOcc
		if err := rows.Scan(&d.id, &d.aid, &d.status, &d.prevSentAt, &d.dueAt, &d.title, &d.notes, &d.assignee, &d.channels, &d.tz,
			&d.tgUser, &d.dcUsr, &d.who); err != nil {
			rows.Close()
			return 0, err
		}
		due = append(due, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	chanCache := map[int64][]alertChannel{}
	sent := 0
	for _, d := range due {
		tag, err := app.DB.Exec(ctx, "UPDATE reminder_occurrences SET status='sent', sent_at=$1 WHERE id=$2 AND status=$3", now, d.id, d.status)
		if err != nil {
			return sent, err
		}
		if tag.RowsAffected() == 0 {
			continue // someone else claimed / acted on it
		}
		if slices.Contains(d.channels, "chat") {
			chs, ok := chanCache[d.aid]
			if !ok {
				if chs, err = app.loadChatChannels(ctx, d.aid, ""); err != nil {
					return sent, err
				}
				chanCache[d.aid] = chs
			}
			if len(chs) > 0 {
				text := reminderText(d, loadLoc(d.tz))
				n := app.deliver(ctx, chs, func(p string) chatMessage {
					return chatMessage{Text: text, Mention: d.mention(p), MentionName: d.who, Buttons: reminderButtons(d.id)}
				}, fmt.Sprintf("occurrence %d", d.id))
				if n == 0 { // release the claim; retried next minute
					_, _ = app.DB.Exec(ctx, "UPDATE reminder_occurrences SET status=$1, sent_at=$2 WHERE id=$3 AND status='sent'", d.status, d.prevSentAt, d.id)
					continue
				}
			}
		}
		sent++
	}
	return sent, nil
}

func reminderText(d dueOcc, loc *time.Location) string {
	var b strings.Builder
	b.WriteString("Reminder: " + d.title)
	if d.notes != "" {
		b.WriteString("\n" + d.notes)
	}
	who := "everyone"
	if d.assignee != nil && d.who != "" {
		who = d.who
	}
	b.WriteString("\nFor " + who + " · " + d.dueAt.In(loc).Format("15:04"))
	return b.String()
}

// escalateReminders re-notifies the escalate_to member when an occurrence
// stays unacknowledged escalate_after_minutes after it was sent.
func (app *App) escalateReminders(ctx context.Context, now time.Time) (int, error) {
	rows, err := app.DB.Query(ctx, `SELECT o.id, o.account_id, o.due_at, r.title, r.assignee_id, r.escalate_to, u.timezone,
	    e.telegram_user_id, e.discord_user_id, COALESCE(NULLIF(e.display_name,''), split_part(e.email,'@',1)),
	    COALESCE(NULLIF(a.display_name,''), split_part(a.email,'@',1), '')
	  FROM reminder_occurrences o JOIN reminders r ON r.id=o.reminder_id JOIN users u ON u.id=o.account_id
	  JOIN members e ON e.id=r.escalate_to
	  LEFT JOIN members a ON a.id=r.assignee_id
	  WHERE o.status='sent' AND o.escalated_at IS NULL AND r.escalate_after_minutes IS NOT NULL
	    AND o.sent_at + make_interval(mins => r.escalate_after_minutes) <= $1
	  ORDER BY o.due_at LIMIT 500`, now)
	if err != nil {
		return 0, err
	}
	type esc struct {
		d        dueOcc
		escName  string
		assignee string
	}
	var list []esc
	for rows.Next() {
		var e esc
		if err := rows.Scan(&e.d.id, &e.d.aid, &e.d.dueAt, &e.d.title, &e.d.assignee, &e.d.escalateTo, &e.d.tz,
			&e.d.tgUser, &e.d.dcUsr, &e.escName, &e.assignee); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	n := 0
	for _, e := range list {
		tag, err := app.DB.Exec(ctx, "UPDATE reminder_occurrences SET escalated_at=$1 WHERE id=$2 AND escalated_at IS NULL AND status='sent'", now, e.d.id)
		if err != nil {
			return n, err
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		chs, err := app.loadChatChannels(ctx, e.d.aid, "")
		if err != nil {
			return n, err
		}
		who := "everyone"
		if e.assignee != "" {
			who = e.assignee
		}
		text := fmt.Sprintf("Not done yet: %s (for %s, due %s)", e.d.title, who, e.d.dueAt.In(loadLoc(e.d.tz)).Format("15:04"))
		if len(chs) > 0 && app.deliver(ctx, chs, func(p string) chatMessage {
			return chatMessage{Text: text, Mention: e.d.mention(p), MentionName: e.escName, Buttons: reminderButtons(e.d.id)}
		}, fmt.Sprintf("escalation %d", e.d.id)) == 0 {
			_, _ = app.DB.Exec(ctx, "UPDATE reminder_occurrences SET escalated_at=NULL WHERE id=$1", e.d.id)
			continue
		}
		n++
	}
	return n, nil
}
