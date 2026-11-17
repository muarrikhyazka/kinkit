package main

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// RegisterGamificationRoutes wires the one read endpoint the frontend needs;
// points/streak are updated as a side effect of handleAddSpending (see
// awardPointsForLog below), not via their own write endpoint.
func RegisterGamificationRoutes(r chi.Router, app *App) {
	r.Get("/gamification", app.handleGetGamification)
}

const pointsPerLog = 10
const streakBonusPerDay = 5 // extra points per consecutive day, capped at streakBonusCap days
const streakBonusCap = 10

// badge thresholds, ordered low to high - first one not met is "next badge"
var badgeThresholds = []struct {
	Points int64
	Name   string
}{
	{0, "Getting Started"},
	{50, "Tracker"},
	{200, "Budget Buddy"},
	{500, "Money Master"},
	{1500, "Finance Wizard"},
}

type gamificationState struct {
	Points      int64   `json:"points"`
	StreakDays  int     `json:"streak_days"`
	LastLogDate *string `json:"last_log_date"` // YYYY-MM-DD (UTC), null if never logged
	Badge       string  `json:"badge"`
	NextBadge   string  `json:"next_badge,omitempty"`
	NextBadgeAt int64   `json:"next_badge_at,omitempty"`
}

const dateLayout = "2006-01-02"

// utcDate truncates t to its UTC calendar date (midnight UTC).
func utcDate(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// todayUTC is the current UTC calendar date. All streak math is in UTC so
// users worldwide share one well-defined day boundary.
func todayUTC() time.Time { return utcDate(time.Now().UTC()) }

// effectiveStreak is the streak to *display* on `today`: the stored streak
// survives only while the last log was today or yesterday; once a full UTC
// day has been missed it is reported as 0 (the row is only rewritten on the
// next log).
func effectiveStreak(lastLog *time.Time, stored int, today time.Time) int {
	if lastLog == nil {
		return 0
	}
	last, t := utcDate(*lastLog), utcDate(today)
	if last.Equal(t) || last.AddDate(0, 0, 1).Equal(t) {
		return stored
	}
	return 0
}

// nextStreak computes the streak after logging on `today`, plus whether this
// is the first log of the day (which earns points). Same day -> unchanged;
// exactly the day after last log -> +1; anything else (gap, never logged,
// clock skew into the past) -> 1.
func nextStreak(lastLog *time.Time, stored int, today time.Time) (streak int, firstToday bool) {
	if lastLog == nil {
		return 1, true
	}
	last, t := utcDate(*lastLog), utcDate(today)
	switch {
	case last.Equal(t):
		if stored < 1 {
			stored = 1
		}
		return stored, false
	case last.AddDate(0, 0, 1).Equal(t):
		return stored + 1, true
	default:
		return 1, true
	}
}

func currentBadge(points int64) (badge, next string, nextAt int64) {
	badge = badgeThresholds[0].Name
	for i, b := range badgeThresholds {
		if points >= b.Points {
			badge = b.Name
		}
		if points < b.Points && next == "" {
			next = b.Name
			nextAt = b.Points
		}
		_ = i
	}
	return
}

func (app *App) handleGetGamification(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	now := time.Now().UTC()
	// Challenges first: a completion bonus lands in the household total
	// shown by loadGamificationState.
	challenges := app.evaluateChallenges(ctx, aid, now)
	resp := gamificationResponse{
		gamificationState: app.loadGamificationState(ctx, aid),
		Challenges:        challenges,
		Leaderboard:       app.loadLeaderboard(ctx, aid, now),
		Badges:            []badgeView{},
	}
	if m, ok := currentMember(r); ok && m.ID != 0 {
		resp.Member = app.loadMemberGamification(ctx, aid, m.ID, now)
		resp.Badges = app.evaluateBadges(ctx, aid, m.ID, now)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (app *App) loadGamificationState(ctx context.Context, aid int64) gamificationState {
	var points int64
	var streak int
	var lastLog *time.Time
	err := app.DB.QueryRow(ctx, "SELECT points, streak_days, last_log_date FROM user_points WHERE account_id=$1", aid).Scan(&points, &streak, &lastLog)
	if err != nil {
		points, streak, lastLog = 0, 0, nil
	}
	var lastLogStr *string
	if lastLog != nil {
		v := utcDate(*lastLog).Format(dateLayout)
		lastLogStr = &v
	}
	badge, next, nextAt := currentBadge(points)
	return gamificationState{
		Points:      points,
		StreakDays:  effectiveStreak(lastLog, streak, todayUTC()),
		LastLogDate: lastLogStr,
		Badge:       badge,
		NextBadge:   next,
		NextBadgeAt: nextAt,
	}
}

// awardPointsForLog is called right after a spending row is successfully
// inserted by a path that has no acting member (bots). Points go to the
// household total and to the household owner's member score.
func (app *App) awardPointsForLog(ctx context.Context, aid int64) {
	app.awardPointsForMember(ctx, aid, nil)
}

// awardPointsForMember: household streak/points (user_points) as before,
// plus the same rules applied to the acting member's own streak
// (member_points) with the earned points recorded in point_events.
// Streak rules live in nextStreak; dates are UTC calendar dates. Base
// points + streak bonus are earned once per UTC day.
func (app *App) awardPointsForMember(ctx context.Context, aid int64, memberID *int64) {
	today := todayUTC()
	var lastLog *time.Time
	var streak int
	err := app.DB.QueryRow(ctx, "SELECT last_log_date, streak_days FROM user_points WHERE account_id=$1", aid).Scan(&lastLog, &streak)
	exists := err == nil
	if !exists {
		lastLog, streak = nil, 0
	}

	newStreak, firstToday := nextStreak(lastLog, streak, today)
	earned := int64(0)
	if firstToday {
		earned = int64(pointsPerLog + streakBonusPerDay*min(newStreak, streakBonusCap))
	}

	_, _ = app.DB.Exec(ctx,
		"INSERT INTO user_points (account_id, points, streak_days, last_log_date) VALUES ($1,$2,$3,$4::date) "+
			"ON CONFLICT (account_id) DO UPDATE SET points = user_points.points + EXCLUDED.points, "+
			"streak_days = EXCLUDED.streak_days, last_log_date = EXCLUDED.last_log_date",
		aid, earned, newStreak, today.Format(dateLayout))

	// --- per member ---
	var mid int64
	if memberID != nil {
		mid = *memberID
	} else if err := app.DB.QueryRow(ctx, "SELECT id FROM members WHERE account_id=$1 AND role='owner'", aid).Scan(&mid); err != nil {
		return
	}
	var mLast *time.Time
	var mStreak int
	if err := app.DB.QueryRow(ctx, "SELECT last_log_date, streak_days FROM member_points WHERE member_id=$1", mid).Scan(&mLast, &mStreak); err != nil {
		mLast, mStreak = nil, 0
	}
	mNew, mFirst := nextStreak(mLast, mStreak, today)
	mEarned := int64(0)
	if mFirst {
		mEarned = int64(pointsPerLog + streakBonusPerDay*min(mNew, streakBonusCap))
	}
	_, _ = app.DB.Exec(ctx,
		"INSERT INTO member_points (member_id, account_id, points, streak_days, best_streak, last_log_date) VALUES ($1,$2,$3,$4,$4,$5::date) "+
			"ON CONFLICT (member_id) DO UPDATE SET points = member_points.points + EXCLUDED.points, "+
			"streak_days = EXCLUDED.streak_days, best_streak = GREATEST(member_points.best_streak, EXCLUDED.streak_days), "+
			"last_log_date = EXCLUDED.last_log_date",
		mid, aid, mEarned, mNew, today.Format(dateLayout))
	if mEarned > 0 {
		_, _ = app.DB.Exec(ctx, "INSERT INTO point_events (account_id, member_id, points, reason) VALUES ($1,$2,$3,'log')", aid, mid, mEarned)
	}
}
