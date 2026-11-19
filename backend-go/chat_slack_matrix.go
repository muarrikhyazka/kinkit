package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Outbound delivery for the Slack and Matrix integrations (alerts, reminders,
// digests). Inbound chat (commands, spending) is handled by bots/slack_bot.py
// and bots/matrix_bot.py. Both platforms are optional: a household only gets
// them when it connects a bot under Settings > Bots.

// Overridable in tests.
var slackAPIBase = "https://slack.com/api"

// sendSlackOrMatrix delivers msg. For Matrix the stored token is
// "<homeserver URL> <access token>" (see loadChatChannels).
func sendSlackOrMatrix(ctx context.Context, platform, token, chatID string, msg chatMessage) error {
	var req *http.Request
	var err error
	switch platform {
	case "slack":
		text := msg.Text
		if msg.Mention != "" {
			text = "<@" + msg.Mention + "> " + text
		}
		body := map[string]any{"channel": chatID, "text": text}
		if len(msg.Buttons) > 0 {
			var els []map[string]any
			for _, row := range msg.Buttons {
				for _, b := range row {
					els = append(els, map[string]any{
						"type": "button", "text": map[string]any{"type": "plain_text", "text": b.Label},
						"action_id": b.Data, "value": b.Data,
					})
				}
			}
			body["blocks"] = []map[string]any{
				{"type": "section", "text": map[string]any{"type": "mrkdwn", "text": text}},
				{"type": "actions", "elements": els},
			}
		}
		raw, _ := json.Marshal(body)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, slackAPIBase+"/chat.postMessage", bytes.NewReader(raw))
		if req != nil {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	case "matrix":
		server, access, ok := strings.Cut(token, " ")
		server = strings.TrimRight(server, "/")
		if !ok || !validHomeserver(server) || access == "" {
			return fmt.Errorf("matrix: homeserver or access token missing")
		}
		text := msg.Text
		if msg.Mention != "" {
			name := msg.MentionName
			if name == "" {
				name = msg.Mention
			}
			text = name + ": " + text
		}
		// Matrix has no tap buttons: tell people how to answer a reminder.
		if len(msg.Buttons) > 0 && len(msg.Buttons[0]) > 0 {
			if parts := strings.Split(msg.Buttons[0][0].Data, ":"); len(parts) == 3 && parts[0] == "rem" {
				text += "\nReply: !rem " + parts[1] + " done | snooze | tomorrow"
			}
		}
		raw, _ := json.Marshal(map[string]any{"msgtype": "m.text", "body": text})
		txn := strconv.FormatInt(time.Now().UnixNano(), 36)
		req, err = http.NewRequestWithContext(ctx, http.MethodPut,
			server+"/_matrix/client/v3/rooms/"+url.PathEscape(chatID)+"/send/m.room.message/"+txn, bytes.NewReader(raw))
		if req != nil {
			req.Header.Set("Authorization", "Bearer "+access)
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
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s: HTTP %d", platform, resp.StatusCode)
	}
	if platform == "slack" { // Slack answers 200 with {"ok":false,"error":"..."}
		var out struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &out) == nil && !out.OK {
			return fmt.Errorf("slack: %s", out.Error)
		}
	}
	return nil
}
