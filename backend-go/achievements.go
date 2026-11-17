package main

import (
	"context"
	"math"
	"time"
)

// Achievements: per-member levels, habit badges, weekly household
// challenges and the family leaderboard - all served by GET /api/gamification
// (see handleGetGamification). Everything is evaluated lazily on that GET
// with a handful of cheap indexed queries; dates are UTC.

type gamificationResponse struct {
	gamificationState                     // household totals (backward compatible fields)
	Member            *memberGamification `json:"member,omitempty"`
	Badges            []badgeView         `json:"badges"`
	Challenges        []challengeView     `json:"challenges"`
	Leaderboard       []leaderRow         `json:"leaderboard"`
}

type memberGamification struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Points      int64  `json:"points"`
	PointsMonth int64  `json:"points_month"`
	StreakDays  int    `json:"streak_days"`
	BestStreak  int    `json:"best_streak"`
	Level       string `json:"level"` // points-threshold badge name (badgeThresholds)
	LevelIndex  int    `json:"level_index"`
	NextLevel   string `json:"next_level,omitempty"`
	NextLevelAt int64  `json:"next_level_at,omitempty"`
}

type leaderRow struct {
	MemberID    int64  `json:"member_id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	PointsMonth int64  `json:"points_month"`
}

func levelIndex(points int64) int {
	idx := 0
	for i, b := range badgeThresholds {
		if points >= b.Points {
			idx = i
		}
	}
	return idx
}

func (app *App) loadMemberGamification(ctx context.Context, aid, mid int64, now time.Time) *memberGamification {
	mg := &memberGamification{ID: mid}
	var display *string
	if err := app.DB.QueryRow(ctx, "SELECT display_name, email, role FROM members WHERE id=$1 AND account_id=$2", mid, aid).
		Scan(&display, &mg.DisplayName, &mg.Role); err != nil {
		return nil
	}
	if display != nil && *display != "" {
		mg.DisplayName = *display
	}
	var stored int
	var last *time.Time
	if err := app.DB.QueryRow(ctx, "SELECT points, streak_days, best_streak, last_log_date FROM member_points WHERE member_id=$1", mid).
		Scan(&mg.Points, &stored, &mg.BestStreak, &last); err == nil {
		mg.StreakDays = effectiveStreak(last, stored, now)
	}
	start, _ := utcMonthBounds(now)
	_ = app.DB.QueryRow(ctx, "SELECT COALESCE(SUM(points),0)::bigint FROM point_events WHERE member_id=$1 AND created_at >= $2", mid, start).Scan(&mg.PointsMonth)
	mg.Level, mg.NextLevel, mg.NextLevelAt = currentBadge(mg.Points)
	mg.LevelIndex = levelIndex(mg.Points)
	return mg
}

func (app *App) loadLeaderboard(ctx context.Context, aid int64, now time.Time) []leaderRow {
	start, _ := utcMonthBounds(now)
	out := []leaderRow{}
	rows, err := app.DB.Query(ctx, `
		SELECT m.id, COALESCE(NULLIF(m.display_name,''), split_part(m.email,'@',1)), m.role,
		       COALESCE((SELECT SUM(points) FROM point_events pe WHERE pe.member_id=m.id AND pe.created_at >= $2), 0)::bigint AS pts
		FROM members m WHERE m.account_id=$1 ORDER BY pts DESC, m.id`, aid, start)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var lr leaderRow
		if rows.Scan(&lr.MemberID, &lr.DisplayName, &lr.Role, &lr.PointsMonth) == nil {
			out = append(out, lr)
		}
	}
	return out
}

// ---------- habit badges ----------

type badgeDef struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

var badgeCatalog = []badgeDef{
	{"first_log", "First Log", "Logged your first spending", "📝"},
	{"first_budget", "First Budget Set", "The household set a monthly budget", "🗂️"},
	{"streak_7", "Streak 7", "Logged spending 7 days in a row", "🔥"},
	{"streak_30", "Streak 30", "Logged spending 30 days in a row", "🏆"},
	{"under_budget", "Under Budget", "Closed a month with Needs + Wants spending at or below budget", "🎯"},
	{"saver", "Saver", "Closed a month with Savings at or above the savings budget", "🐷"},
}

type badgeView struct {
	badgeDef
	Earned   bool       `json:"earned"`
	EarnedAt *time.Time `json:"earned_at"`
}

// badgeFacts are the inputs to the pure badge rules (unit tested). The
// "Prev*" fields describe the last closed UTC month.
type badgeFacts struct {
	HasLogged          bool
	BestStreak         int
	HasBudget          bool
	PrevExpenseBudget  int64 // budgeted Needs + Wants
	PrevExpenseSpent   int64 // actual non-Savings spending
	PrevTxCount        int64
	PrevSavingsBudget  int64
	PrevSavingsRealize int64
}

// earnedBadges returns the badge keys whose conditions hold.
func earnedBadges(f badgeFacts) []string {
	var out []string
	if f.HasLogged {
		out = append(out, "first_log")
	}
	if f.HasBudget {
		out = append(out, "first_budget")
	}
	if f.BestStreak >= 7 {
		out = append(out, "streak_7")
	}
	if f.BestStreak >= 30 {
		out = append(out, "streak_30")
	}
	// a month with no logging at all doesn't count as "under budget"
	if f.PrevExpenseBudget > 0 && f.PrevTxCount > 0 && f.PrevExpenseSpent <= f.PrevExpenseBudget {
		out = append(out, "under_budget")
	}
	if f.PrevSavingsBudget > 0 && f.PrevSavingsRealize >= f.PrevSavingsBudget {
		out = append(out, "saver")
	}
	return out
}

func (app *App) loadBadgeFacts(ctx context.Context, aid, mid int64, now time.Time) badgeFacts {
	var f badgeFacts
	var best int
	var stored int
	var last *time.Time
	if err := app.DB.QueryRow(ctx, "SELECT best_streak, streak_days, last_log_date FROM member_points WHERE member_id=$1", mid).
		Scan(&best, &stored, &last); err == nil {
		f.HasLogged = last != nil
		f.BestStreak = max(best, effectiveStreak(last, stored, now))
	}
	_ = app.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM budget_expense WHERE account_id=$1 AND amount > 0)", aid).Scan(&f.HasBudget)

	thisMonth, _ := utcMonthBounds(now)
	prevStart := thisMonth.AddDate(0, -1, 0)
	_ = app.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(CASE WHEN c.main_category IN ('Needs','Wants') THEN be.amount END),0)::bigint,
		       COALESCE(SUM(CASE WHEN c.main_category = 'Savings' THEN be.amount END),0)::bigint
		FROM budget_expense be JOIN categories c ON c.id = be.category_id
		WHERE be.account_id=$1 AND be.year=$2 AND be.month=$3`,
		aid, prevStart.Year(), int(prevStart.Month())).Scan(&f.PrevExpenseBudget, &f.PrevSavingsBudget)
	if f.PrevExpenseBudget > 0 || f.PrevSavingsBudget > 0 {
		_ = app.DB.QueryRow(ctx, `
			SELECT COUNT(*),
			       COALESCE(SUM(CASE WHEN COALESCE(c.main_category,'') <> 'Savings' THEN sp.amount END),0)::bigint,
			       COALESCE(SUM(CASE WHEN c.main_category = 'Savings' THEN sp.amount END),0)::bigint
			FROM spending sp LEFT JOIN categories c ON c.id = sp.category_id
			WHERE sp.account_id=$1 AND sp.tx_date >= $2 AND sp.tx_date < $3`,
			aid, prevStart, thisMonth).Scan(&f.PrevTxCount, &f.PrevExpenseSpent, &f.PrevSavingsRealize)
	}
	return f
}

// evaluateBadges stores newly earned badges (once, with earned_at) and
// returns the full catalog with earned state for this member.
func (app *App) evaluateBadges(ctx context.Context, aid, mid int64, now time.Time) []badgeView {
	for _, key := range earnedBadges(app.loadBadgeFacts(ctx, aid, mid, now)) {
		_, _ = app.DB.Exec(ctx, "INSERT INTO member_badges (member_id, badge_key, earned_at) VALUES ($1,$2,$3) ON CONFLICT (member_id, badge_key) DO NOTHING", mid, key, now)
	}
	earned := map[string]time.Time{}
	if rows, err := app.DB.Query(ctx, "SELECT badge_key, earned_at FROM member_badges WHERE member_id=$1", mid); err == nil {
		for rows.Next() {
			var k string
			var t time.Time
			if rows.Scan(&k, &t) == nil {
				earned[k] = t.UTC()
			}
		}
		rows.Close()
	}
	out := make([]badgeView, 0, len(badgeCatalog))
	for _, b := range badgeCatalog {
		v := badgeView{badgeDef: b}
		if t, ok := earned[b.Key]; ok {
			v.Earned, v.EarnedAt = true, &t
		}
		out = append(out, v)
	}
	return out
}

// ---------- weekly household challenges ----------

// weekStartUTC is the Monday (00:00 UTC) of t's ISO week.
func weekStartUTC(t time.Time) time.Time {
	d := utcDate(t)
	offset := (int(d.Weekday()) + 6) % 7 // Monday=0 ... Sunday=6
	return d.AddDate(0, 0, -offset)
}

type challengeView struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Unit        string `json:"unit"` // days | amount
	Progress    int64  `json:"progress"`
	Target      int64  `json:"target"`
	Bonus       int64  `json:"bonus"`
	Available   bool   `json:"available"` // false: needs setup (e.g. no Wants budget)
	Completed   bool   `json:"completed"` // bonus already awarded for this week
	// For "stay under" challenges success is only final when the week ends.
	LowerIsBetter bool   `json:"lower_is_better,omitempty"`
	WeekStart     string `json:"week_start"`
	WeekEnd       string `json:"week_end"`
	LastWeekDone  bool   `json:"last_week_done"`
}

const (
	bonusLogEveryDay   = 50
	bonusNoSpendWkend  = 30
	bonusWantsUnderCap = 40
)

// weekStats are the raw numbers for one household week (pure inputs).
type weekStats struct {
	DaysLogged     int64 // distinct tx_date values in the week
	WeekdayTx      int64 // rows Mon-Fri
	WeekendTx      int64 // rows Sat-Sun
	WeekendPassed  int64 // weekend days already fully over (0..2)
	WantsSpent     int64
	WantsCap       int64 // weekly Wants allowance derived from the monthly budget; 0 = none
	Ended          bool  // the week is over
	TotalTx        int64
	WeekIsEligible bool // household existed before the week began
}

// challengeOutcome is the pure challenge rule set (unit tested):
// progress/target for display and whether it's complete (bonus-worthy).
func challengeOutcome(key string, s weekStats) (progress, target int64, available, done bool) {
	switch key {
	case "log_every_day":
		return s.DaysLogged, 7, true, s.DaysLogged >= 7
	case "no_spend_weekend":
		// zero weekend spending, finalised once Sunday is over; must have
		// tracked something during the week so inactivity doesn't count.
		prog := int64(0)
		if s.WeekendTx == 0 {
			prog = s.WeekendPassed
		}
		return prog, 2, true, s.Ended && s.WeekendTx == 0 && s.WeekdayTx > 0
	case "wants_under":
		if s.WantsCap <= 0 {
			return s.WantsSpent, 0, false, false
		}
		return s.WantsSpent, s.WantsCap, true, s.Ended && s.TotalTx > 0 && s.WantsSpent <= s.WantsCap
	}
	return 0, 0, false, false
}

var challengeDefs = []struct {
	Key, Name, Description, Icon, Unit string
	Bonus                              int64
	LowerIsBetter                      bool
}{
	{"log_every_day", "Log every day this week", "Log at least one spending entry on each of the 7 days (Mon-Sun, UTC).", "📅", "days", bonusLogEveryDay, false},
	{"no_spend_weekend", "No-spend weekend", "Spend nothing on Saturday and Sunday (log something during the week so we know you're tracking).", "🛋️", "days", bonusNoSpendWkend, false},
	{"wants_under", "Keep Wants under budget", "Keep this week's Wants spending under a week's share of your monthly Wants budget.", "🧺", "amount", bonusWantsUnderCap, true},
}

func (app *App) loadWeekStats(ctx context.Context, aid int64, weekStart, now time.Time, householdCreated time.Time) weekStats {
	end := weekStart.AddDate(0, 0, 7)
	today := utcDate(now)
	s := weekStats{Ended: !today.Before(end), WeekIsEligible: householdCreated.Before(weekStart)}
	_ = app.DB.QueryRow(ctx, `
		SELECT COUNT(DISTINCT sp.tx_date),
		       COUNT(*) FILTER (WHERE EXTRACT(ISODOW FROM sp.tx_date) <= 5),
		       COUNT(*) FILTER (WHERE EXTRACT(ISODOW FROM sp.tx_date) >= 6),
		       COALESCE(SUM(sp.amount) FILTER (WHERE c.main_category = 'Wants'),0)::bigint,
		       COUNT(*)
		FROM spending sp LEFT JOIN categories c ON c.id = sp.category_id
		WHERE sp.account_id=$1 AND sp.tx_date >= $2 AND sp.tx_date < $3`,
		aid, weekStart, end).Scan(&s.DaysLogged, &s.WeekdayTx, &s.WeekendTx, &s.WantsSpent, &s.TotalTx)
	sat, sun := weekStart.AddDate(0, 0, 5), weekStart.AddDate(0, 0, 6)
	if today.After(sat) {
		s.WeekendPassed++
	}
	if today.After(sun) {
		s.WeekendPassed++
	}
	// weekly Wants cap: monthly Wants budget of the week's Monday month,
	// scaled to 7 days of that month.
	var monthly int64
	_ = app.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(be.amount),0)::bigint FROM budget_expense be JOIN categories c ON c.id = be.category_id
		WHERE be.account_id=$1 AND be.year=$2 AND be.month=$3 AND c.main_category='Wants'`,
		aid, weekStart.Year(), int(weekStart.Month())).Scan(&monthly)
	if monthly > 0 {
		ms, me := utcMonthBounds(weekStart)
		days := me.Sub(ms).Hours() / 24
		s.WantsCap = int64(math.Round(float64(monthly) * 7 / days))
	}
	return s
}

// awardChallenge records a completion once per (household, challenge, week)
// and adds the bonus to the household total. Returns true if newly awarded.
func (app *App) awardChallenge(ctx context.Context, aid int64, key string, weekStart time.Time, bonus int64) bool {
	var id int64
	err := app.DB.QueryRow(ctx,
		"INSERT INTO challenge_completions (account_id, challenge_key, period_start, bonus) VALUES ($1,$2,$3::date,$4) "+
			"ON CONFLICT (account_id, challenge_key, period_start) DO NOTHING RETURNING id",
		aid, key, weekStart.Format(dateLayout), bonus).Scan(&id)
	if err != nil {
		return false // already awarded (no row returned) or db error
	}
	_, _ = app.DB.Exec(ctx,
		"INSERT INTO user_points (account_id, points, streak_days) VALUES ($1,$2,0) "+
			"ON CONFLICT (account_id) DO UPDATE SET points = user_points.points + EXCLUDED.points", aid, bonus)
	_, _ = app.DB.Exec(ctx, "INSERT INTO point_events (account_id, member_id, points, reason) VALUES ($1,NULL,$2,$3)", aid, bonus, "challenge:"+key)
	return true
}

// evaluateChallenges awards any completed challenge for last week and this
// week (weeks that began before the household existed are never eligible,
// so a brand-new account doesn't get a free "no-spend weekend"), then
// returns this week's challenges with progress.
func (app *App) evaluateChallenges(ctx context.Context, aid int64, now time.Time) []challengeView {
	var created time.Time
	if err := app.DB.QueryRow(ctx, "SELECT created_at FROM users WHERE id=$1", aid).Scan(&created); err != nil {
		return []challengeView{}
	}
	thisWeek := weekStartUTC(now)
	lastWeek := thisWeek.AddDate(0, 0, -7)
	cur := app.loadWeekStats(ctx, aid, thisWeek, now, created)
	prev := app.loadWeekStats(ctx, aid, lastWeek, now, created)

	completed := map[string]bool{}
	if rows, err := app.DB.Query(ctx, "SELECT challenge_key, period_start FROM challenge_completions WHERE account_id=$1 AND period_start >= $2", aid, lastWeek); err == nil {
		for rows.Next() {
			var k string
			var p time.Time
			if rows.Scan(&k, &p) == nil {
				completed[k+"@"+utcDate(p).Format(dateLayout)] = true
			}
		}
		rows.Close()
	}
	out := make([]challengeView, 0, len(challengeDefs))
	for _, def := range challengeDefs {
		curKey := def.Key + "@" + thisWeek.Format(dateLayout)
		prevKey := def.Key + "@" + lastWeek.Format(dateLayout)
		if _, _, _, done := challengeOutcome(def.Key, prev); done && prev.WeekIsEligible && !completed[prevKey] {
			completed[prevKey] = app.awardChallenge(ctx, aid, def.Key, lastWeek, def.Bonus) || completed[prevKey]
		}
		progress, target, available, done := challengeOutcome(def.Key, cur)
		if done && cur.WeekIsEligible && !completed[curKey] {
			completed[curKey] = app.awardChallenge(ctx, aid, def.Key, thisWeek, def.Bonus)
		}
		out = append(out, challengeView{
			Key: def.Key, Name: def.Name, Description: def.Description, Icon: def.Icon, Unit: def.Unit,
			Progress: progress, Target: target, Bonus: def.Bonus, Available: available,
			Completed: completed[curKey], LowerIsBetter: def.LowerIsBetter,
			WeekStart: thisWeek.Format(dateLayout), WeekEnd: thisWeek.AddDate(0, 0, 6).Format(dateLayout),
			LastWeekDone: completed[prevKey],
		})
	}
	return out
}
