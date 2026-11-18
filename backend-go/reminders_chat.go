package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Richer chat delivery for reminders: inline buttons (Telegram inline
// keyboard / Discord message components) and member mentions. alerts.go's
// plain sendChatMessage stays as is.

type chatButton struct {
	Label string // shown to the user, e.g. "✅ Done"
	Data  string // callback data / custom_id, e.g. "rem:12:done" (<=64 bytes)
}

type chatMessage struct {
	Text string
	// Mention is the platform user id to @-mention (prefixed to the text).
	Mention     string
	MentionName string
	Buttons     [][]chatButton // rows of buttons
}

// sendReminderChat is the delivery hook used by the reminders/digest worker.
// Tests swap it for a stub.
var sendReminderChat = sendChatMessageWithButtons

// sendChatMessageWithButtons posts msg to a Telegram chat or Discord channel.
// The bot token never appears in returned errors.
func sendChatMessageWithButtons(ctx context.Context, platform, token, chatID string, msg chatMessage) error {
	var req *http.Request
	var err error
	switch platform {
	case "telegram":
		body := map[string]any{"chat_id": chatID}
		if msg.Mention != "" {
			name := msg.MentionName
			if name == "" {
				name = "you"
			}
			body["text"] = fmt.Sprintf(`<a href="tg://user?id=%s">%s</a> %s`, html.EscapeString(msg.Mention), html.EscapeString(name), html.EscapeString(msg.Text))
			body["parse_mode"] = "HTML"
		} else {
			body["text"] = msg.Text
		}
		if len(msg.Buttons) > 0 {
			var rows [][]map[string]string
			for _, r := range msg.Buttons {
				var row []map[string]string
				for _, b := range r {
					row = append(row, map[string]string{"text": b.Label, "callback_data": b.Data})
				}
				rows = append(rows, row)
			}
			body["reply_markup"] = map[string]any{"inline_keyboard": rows}
		}
		raw, _ := json.Marshal(body)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, telegramAPIBase+"/bot"+token+"/sendMessage", bytes.NewReader(raw))
	case "discord":
		content := msg.Text
		allowed := map[string]any{"parse": []string{}}
		if msg.Mention != "" {
			content = "<@" + msg.Mention + "> " + content
			allowed["users"] = []string{msg.Mention}
		}
		body := map[string]any{"content": content, "allowed_mentions": allowed}
		if len(msg.Buttons) > 0 {
			var rows []map[string]any
			for _, r := range msg.Buttons {
				var comps []map[string]any
				for i, b := range r {
					style := 2 // secondary
					if i == 0 {
						style = 3 // success
					}
					comps = append(comps, map[string]any{"type": 2, "style": style, "label": b.Label, "custom_id": b.Data})
				}
				rows = append(rows, map[string]any{"type": 1, "components": comps})
			}
			body["components"] = rows
		}
		raw, _ := json.Marshal(body)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, discordAPIBase+"/channels/"+url.PathEscape(chatID)+"/messages", bytes.NewReader(raw))
		if req != nil {
			req.Header.Set("Authorization", "Bot "+token)
		}
	default:
		return fmt.Errorf("unsupported platform %q", platform)
	}
	if err != nil {
		return fmt.Errorf("%s: build request failed", platform)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := alertHTTPClient.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("%s: %v", platform, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s: HTTP %d", platform, resp.StatusCode)
	}
	return nil
}

// reminderButtons are the actions attached to a reminder message.
func reminderButtons(occID int64) [][]chatButton {
	id := fmt.Sprint(occID)
	return [][]chatButton{{
		{Label: "✅ Done", Data: "rem:" + id + ":done"},
		{Label: "⏰ Snooze 1h", Data: "rem:" + id + ":snooze60"},
		{Label: "🌅 Tomorrow", Data: "rem:" + id + ":snooze_tomorrow"},
	}}
}

// loadChatChannels lists the household's deliverable chat channels,
// optionally restricted to one platform ("" or "all" = every platform).
func (app *App) loadChatChannels(ctx context.Context, aid int64, platform string) ([]alertChannel, error) {
	rows, err := app.DB.Query(ctx, `SELECT platform, platform_token, chat_id FROM integrations
	  WHERE account_id=$1 AND enabled AND COALESCE(chat_id,'') <> '' AND COALESCE(platform_token,'') <> '' ORDER BY id`, aid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []alertChannel
	for rows.Next() {
		var c alertChannel
		if err := rows.Scan(&c.platform, &c.token, &c.chatID); err != nil {
			return nil, err
		}
		if platform == "" || platform == "all" || strings.EqualFold(platform, c.platform) {
			out = append(out, c)
		}
	}
	return out, rows.Err()
}
