package main

// Shared contracts for the household features (Today page, reminders,
// chores, shopping lists). Feature files plug into these from their own
// init() functions so they don't need to edit each other's code.

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"
	_ "time/tzdata" // embed zone database so household time zones work in any image
)

// ---------- Today page / daily digest ----------

// TodayItem is one line on the Today page and in the morning digest message.
type TodayItem struct {
	Kind       string   `json:"kind"`                  // "reminder" | "chore" | "list" | "budget" | ...
	ID         int64    `json:"id,omitempty"`          // id within its kind (e.g. reminder occurrence id)
	Title      string   `json:"title"`                 // short, human readable
	Detail     string   `json:"detail,omitempty"`      // secondary text
	Due        string   `json:"due,omitempty"`         // RFC3339 UTC instant or YYYY-MM-DD
	MemberID   *int64   `json:"member_id,omitempty"`   // assignee, if any
	MemberName string   `json:"member_name,omitempty"` // assignee display name
	Done       bool     `json:"done"`
	Overdue    bool     `json:"overdue,omitempty"`
	Link       string   `json:"link,omitempty"`    // in-app path, e.g. "/home/chores"
	Actions    []string `json:"actions,omitempty"` // UI/bot actions supported: "done", "snooze", "approve"
}

// TodaySection groups items from one provider ("Reminders", "Chores", ...).
type TodaySection struct {
	Key   string      `json:"key"`
	Title string      `json:"title"`
	Order int         `json:"-"`
	Items []TodayItem `json:"items"`
	// Summary is an optional one-liner used in the digest when there are no
	// items worth listing (e.g. "Budget: 2.1jt left this month").
	Summary string `json:"summary,omitempty"`
}

// TodayProvider builds one section for a household on a given day. `day` is
// the household's local calendar day expressed as a UTC midnight time.Time
// (see householdToday). `member` is the viewer: providers may tailor items
// (kids see only their own chores). Return nil for "nothing to show".
type TodayProvider func(ctx context.Context, app *App, aid int64, member memberInfo, day time.Time) (*TodaySection, error)

type namedTodayProvider struct {
	key   string
	order int
	fn    TodayProvider
}

var (
	todayProvidersMu sync.Mutex
	todayProviders   []namedTodayProvider
)

// registerTodayProvider is called from feature files' init().
// Lower order sorts first.
func registerTodayProvider(key string, order int, fn TodayProvider) {
	todayProvidersMu.Lock()
	defer todayProvidersMu.Unlock()
	todayProviders = append(todayProviders, namedTodayProvider{key, order, fn})
	sort.SliceStable(todayProviders, func(i, j int) bool { return todayProviders[i].order < todayProviders[j].order })
}

func listTodayProviders() []namedTodayProvider {
	todayProvidersMu.Lock()
	defer todayProvidersMu.Unlock()
	return append([]namedTodayProvider(nil), todayProviders...)
}

// ---------- Kid permissions ----------

// KidRule may allow a non-GET request for the kid role. Return ok=false to
// pass. Decision roleAllow lets the handler run (the handler must then check
// ownership itself, e.g. "only chores assigned to me").
type KidRule func(method, path string) (decision roleDecision, id int64, ok bool)

var kidRules []KidRule

// registerKidRule is called from feature files' init().
func registerKidRule(rule KidRule) { kidRules = append(kidRules, rule) }

// ---------- Bot actor (which household member is talking to the bot) ----------

// Bots forward the chat user as header X-Bot-User: "<platform>:<user id>",
// e.g. "telegram:123456". Members link their chat account once (reminders
// feature owns the linking flow and the members.telegram_user_id /
// members.discord_user_id columns, see schema.sql).
func botActor(ctx context.Context, app *App, r *http.Request, aid int64) (memberInfo, bool) {
	platform, uid, ok := splitBotUser(r.Header.Get("X-Bot-User"))
	if !ok {
		return memberInfo{}, false
	}
	col := map[string]string{"telegram": "telegram_user_id", "discord": "discord_user_id"}[platform]
	if col == "" {
		return memberInfo{}, false
	}
	var m memberInfo
	err := app.DB.QueryRow(ctx,
		"SELECT id, account_id, role FROM members WHERE account_id=$1 AND "+col+"=$2", aid, uid).
		Scan(&m.ID, &m.AccountID, &m.Role)
	if err != nil {
		return memberInfo{}, false
	}
	return m, true
}

func splitBotUser(v string) (platform, uid string, ok bool) {
	for i := 0; i < len(v); i++ {
		if v[i] == ':' {
			if i == 0 || i == len(v)-1 {
				return "", "", false
			}
			return v[:i], v[i+1:], true
		}
	}
	return "", "", false
}

// ---------- Household time zone ----------

// householdLocation returns the household's time zone (UTC on any problem).
func householdLocation(ctx context.Context, app *App, aid int64) *time.Location {
	var tz string
	if err := app.DB.QueryRow(ctx, "SELECT timezone FROM users WHERE id=$1", aid).Scan(&tz); err != nil || tz == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

// householdToday is the household's current local calendar day, returned as
// midnight UTC of that date (so it compares cleanly with DATE columns).
func householdToday(ctx context.Context, app *App, aid int64, now time.Time) time.Time {
	y, m, d := now.In(householdLocation(ctx, app, aid)).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
