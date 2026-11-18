package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// Morning digest: once per household-local day, at digest_settings.time_of_day
// (default 07:00), a plain-text summary built from the Today providers is
// sent to the household's chat channels. Idempotency: a digest_sends row
// (account, local date) is claimed before sending and released if every
// send fails. Not sent more than 3h late (e.g. after downtime).

const digestMaxItems = 6
const digestLateWindow = 3 * time.Hour

// buildDigestText renders sections as a concise chat message (pure).
func buildDigestText(greeting string, day time.Time, sections []TodaySection) string {
	var b strings.Builder
	b.WriteString(greeting + "! " + day.Format("Monday, 2 Jan"))
	if len(sections) == 0 {
		b.WriteString("\n\nNothing scheduled today. Enjoy!")
		return b.String()
	}
	for _, s := range sections {
		b.WriteString("\n\n" + s.Title)
		if s.Key == "budget" && s.Summary != "" {
			// one line is enough for money
			b.WriteString(": " + s.Summary)
			for _, it := range s.Items {
				if it.Overdue && strings.Contains(it.Title, "over budget") {
					b.WriteString("\n  ! " + it.Title + " (" + it.Detail + ")")
				}
			}
			continue
		}
		shown := 0
		for _, it := range s.Items {
			if shown == digestMaxItems {
				break
			}
			mark := "•"
			if it.Done {
				mark = "[x]"
			} else if it.Overdue {
				mark = "!"
			}
			line := "\n" + mark + " " + it.Title
			var extra []string
			if it.Detail != "" {
				extra = append(extra, it.Detail)
			}
			if it.MemberName != "" {
				extra = append(extra, it.MemberName)
			}
			if len(extra) > 0 {
				line += " (" + strings.Join(extra, ", ") + ")"
			}
			b.WriteString(line)
			shown++
		}
		if rest := len(s.Items) - shown; rest > 0 {
			b.WriteString(fmt.Sprintf("\n…and %d more", rest))
		}
		if len(s.Items) == 0 && s.Summary != "" {
			b.WriteString("\n" + s.Summary)
		}
	}
	return b.String()
}

// householdView is the member used for chat-wide messages: sees everything.
func householdView(aid int64) memberInfo { return memberInfo{AccountID: aid, Role: roleOwner} }

// digestText builds the digest for a household as seen by m.
func (app *App) digestText(ctx context.Context, aid int64, m memberInfo, now time.Time) string {
	loc := householdLocation(ctx, app, aid)
	day := householdToday(ctx, app, aid, now)
	greeting := greetingFor(now.In(loc))
	if n := app.memberDisplayName(ctx, m); n != "" {
		greeting += ", " + n
	}
	return buildDigestText(greeting, day, runTodayProviders(ctx, app, aid, m, day))
}

// digestDue: should a digest at local "HH:MM" be sent at local time t?
func digestDue(timeOfDay string, t time.Time) bool {
	hh, mm, err := parseHHMM(timeOfDay)
	if err != nil {
		return false
	}
	at := time.Date(t.Year(), t.Month(), t.Day(), hh, mm, 0, 0, t.Location())
	return !t.Before(at) && t.Sub(at) < digestLateWindow
}

// sendDigests sends today's digest to every household that is due.
func (app *App) sendDigests(ctx context.Context, now time.Time) (int, error) {
	rows, err := app.DB.Query(ctx, `SELECT u.id, u.timezone, COALESCE(d.enabled, TRUE), COALESCE(d.time_of_day,'07:00'), COALESCE(d.channel,'all')
	  FROM users u LEFT JOIN digest_settings d ON d.account_id=u.id
	  WHERE NOT u.is_disabled AND COALESCE(d.enabled, TRUE) AND EXISTS (
	    SELECT 1 FROM integrations i WHERE i.account_id=u.id AND i.enabled
	      AND COALESCE(i.chat_id,'') <> '' AND COALESCE(i.platform_token,'') <> '')`)
	if err != nil {
		return 0, err
	}
	type acct struct {
		id              int64
		tz, at, channel string
		enabled         bool
	}
	var list []acct
	for rows.Next() {
		var a acct
		if err := rows.Scan(&a.id, &a.tz, &a.enabled, &a.at, &a.channel); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	sent := 0
	for _, a := range list {
		local := now.In(loadLoc(a.tz))
		if !digestDue(a.at, local) {
			continue
		}
		localDate := dateOnly(local)
		tag, err := app.DB.Exec(ctx, "INSERT INTO digest_sends (account_id, local_date) VALUES ($1,$2) ON CONFLICT DO NOTHING", a.id, localDate)
		if err != nil {
			return sent, err
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		chs, err := app.loadChatChannels(ctx, a.id, a.channel)
		if err != nil {
			return sent, err
		}
		text := app.digestText(ctx, a.id, householdView(a.id), now)
		if app.deliver(ctx, chs, func(string) chatMessage { return chatMessage{Text: text} }, fmt.Sprintf("digest account %d", a.id)) == 0 {
			log.Printf("digest: account %d: no channel delivered, will retry", a.id)
			_, _ = app.DB.Exec(ctx, "DELETE FROM digest_sends WHERE account_id=$1 AND local_date=$2", a.id, localDate)
			continue
		}
		sent++
	}
	return sent, nil
}
