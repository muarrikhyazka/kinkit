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
	Level       string `json:"level"` // title of the current level (levelTitle)
	LevelIndex  int    `json:"level_index"`
	LevelNumber int    `json:"level_number"` // 1..maxLevel
	LevelFloor  int64  `json:"level_floor"`  // points where this level began
	NextLevel   string `json:"next_level,omitempty"`
	NextLevelAt int64  `json:"next_level_at,omitempty"` // points where the next level begins
}

type leaderRow struct {
	MemberID    int64  `json:"member_id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	PointsMonth int64  `json:"points_month"`
}

func levelIndex(points int64) int { return levelFor(points) - 1 }

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
	mg.LevelNumber = mg.LevelIndex + 1
	mg.LevelFloor = levelThreshold(mg.LevelNumber)
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

// badges

// badgeDef describes one badge. Category groups them in the UI (money, habits,
// home, levels); Tier (bronze/silver/gold) sets the medal colour; Target is
// the value of the badge's metric needed to earn it (1 for one-off badges).
type badgeDef struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
	Tier        string `json:"tier"`
	Target      int64  `json:"target"`
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
	ChoresDone         int64 // approved chore instances credited to the member
	SpendingCount      int64 // expenses the member logged
	Points             int64 // member's points (for level badges)
	GoalsCreated       int64 // household savings goals
	GoalsReached       int64 // ... that hit their target
	AssetsCount        int64
	IncomeCount        int64
	RemindersDone      int64 // reminders the member marked done
	ProposalsMade      int64 // dishes the member requested
	ProposalsApproved  int64
	ActivitiesCreated  int64
}

type badgeRule struct {
	badgeDef
	metric func(f badgeFacts) int64 // progress toward Target
}

func flag(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

var badgeRules = []badgeRule{
	// --- money ---
	{badgeDef{"first_log", "First Log", "Logged your first spending", "", "money", "bronze", 1}, func(f badgeFacts) int64 { return flag(f.HasLogged || f.SpendingCount > 0) }},
	{badgeDef{"financial_geek", "Financial Geek", "Logged 100 expenses", "", "money", "silver", 100}, func(f badgeFacts) int64 { return f.SpendingCount }},
	{badgeDef{"number_cruncher", "Number Cruncher", "Logged 500 expenses", "", "money", "gold", 500}, func(f badgeFacts) int64 { return f.SpendingCount }},
	{badgeDef{"first_budget", "First Budget Set", "The household set a monthly budget", "", "money", "bronze", 1}, func(f badgeFacts) int64 { return flag(f.HasBudget) }},
	{badgeDef{"under_budget", "Under Budget", "Closed a month with Needs + Wants spending at or below budget", "", "money", "silver", 1}, func(f badgeFacts) int64 {
		// a month with no logging at all doesn't count as "under budget"
		return flag(f.PrevExpenseBudget > 0 && f.PrevTxCount > 0 && f.PrevExpenseSpent <= f.PrevExpenseBudget)
	}},
	{badgeDef{"saver", "Saver", "Closed a month with Savings at or above the savings budget", "", "money", "silver", 1}, func(f badgeFacts) int64 {
		return flag(f.PrevSavingsBudget > 0 && f.PrevSavingsRealize >= f.PrevSavingsBudget)
	}},
	{badgeDef{"goal_setter", "Goal Setter", "The household created a savings goal", "", "money", "bronze", 1}, func(f badgeFacts) int64 { return f.GoalsCreated }},
	{badgeDef{"goal_crusher", "Goal Crusher", "A savings goal reached its target", "", "money", "gold", 1}, func(f badgeFacts) int64 { return f.GoalsReached }},
	{badgeDef{"asset_tracker", "Asset Tracker", "Added an asset to your net worth", "", "money", "bronze", 1}, func(f badgeFacts) int64 { return f.AssetsCount }},
	{badgeDef{"income_insider", "Income Insider", "Recorded income", "", "money", "bronze", 1}, func(f badgeFacts) int64 { return f.IncomeCount }},
	// --- habits ---
	{badgeDef{"streak_3", "Warming Up", "Logged spending 3 days in a row", "", "habits", "bronze", 3}, func(f badgeFacts) int64 { return int64(f.BestStreak) }},
	{badgeDef{"streak_7", "Streak 7", "Logged spending 7 days in a row", "", "habits", "silver", 7}, func(f badgeFacts) int64 { return int64(f.BestStreak) }},
	{badgeDef{"streak_30", "Streak 30", "Logged spending 30 days in a row", "", "habits", "gold", 30}, func(f badgeFacts) int64 { return int64(f.BestStreak) }},
	{badgeDef{"never_forget", "Never Forget", "Marked 10 reminders done", "", "habits", "silver", 10}, func(f badgeFacts) int64 { return f.RemindersDone }},
	{badgeDef{"planner", "Planner", "Added an activity to the family calendar", "", "habits", "bronze", 1}, func(f badgeFacts) int64 { return f.ActivitiesCreated }},
	// --- home ---
	{badgeDef{"chores_10", "Helping hand", "Finished 10 chores", "", "home", "bronze", choresBadgeHelping}, func(f badgeFacts) int64 { return f.ChoresDone }},
	{badgeDef{"chores_50", "Chore champion", "Finished 50 chores", "", "home", "gold", choresBadgeChampion}, func(f badgeFacts) int64 { return f.ChoresDone }},
	{badgeDef{"menu_maker", "Menu Maker", "Requested a dish for the family menu", "", "home", "bronze", 1}, func(f badgeFacts) int64 { return f.ProposalsMade }},
	{badgeDef{"chefs_choice", "Chef's Choice", "A dish you requested made the menu", "", "home", "silver", 1}, func(f badgeFacts) int64 { return f.ProposalsApproved }},
	// --- levels ---
	{badgeDef{"level_5", "Money Master", "Reached level 5", "", "levels", "silver", 5}, func(f badgeFacts) int64 { return int64(levelFor(f.Points)) }},
	{badgeDef{"level_10", "Finance Wizard", "Reached level 10", "", "levels", "gold", 10}, func(f badgeFacts) int64 { return int64(levelFor(f.Points)) }},
}

const (
	choresBadgeHelping  = 10
	choresBadgeChampion = 50
)

var badgeCatalog = func() []badgeDef {
	out := make([]badgeDef, len(badgeRules))
	for i, r := range badgeRules {
		out[i] = r.badgeDef
	}
	return out
}()

type badgeView struct {
	badgeDef
	Earned   bool       `json:"earned"`
	EarnedAt *time.Time `json:"earned_at"`
	Progress int64      `json:"progress"` // metric so far, capped at Target
}

// earnedBadges returns the badge keys whose conditions hold.
func earnedBadges(f badgeFacts) []string {
	var out []string
	for _, r := range badgeRules {
		if r.metric(f) >= r.Target {
			out = append(out, r.Key)
		}
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
	_ = app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM chore_instances WHERE account_id=$1 AND member_id=$2 AND status='approved'", aid, mid).Scan(&f.ChoresDone)
	_ = app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM spending WHERE account_id=$1 AND member_id=$2", aid, mid).Scan(&f.SpendingCount)
	_ = app.DB.QueryRow(ctx, "SELECT COALESCE(points,0) FROM member_points WHERE member_id=$1", mid).Scan(&f.Points)
	_ = app.DB.QueryRow(ctx, "SELECT COUNT(*), COUNT(*) FILTER (WHERE saved_amount >= target_amount AND target_amount > 0) FROM savings_goals WHERE account_id=$1", aid).Scan(&f.GoalsCreated, &f.GoalsReached)
	_ = app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM assets WHERE account_id=$1", aid).Scan(&f.AssetsCount)
	_ = app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM income_entries WHERE account_id=$1", aid).Scan(&f.IncomeCount)
	_ = app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM reminder_occurrences WHERE account_id=$1 AND done_by=$2", aid, mid).Scan(&f.RemindersDone)
	_ = app.DB.QueryRow(ctx, "SELECT COUNT(*), COUNT(*) FILTER (WHERE status='approved') FROM meal_proposals WHERE account_id=$1 AND member_id=$2", aid, mid).Scan(&f.ProposalsMade, &f.ProposalsApproved)
	_ = app.DB.QueryRow(ctx, "SELECT COUNT(*) FROM activities WHERE account_id=$1 AND created_by=$2", aid, mid).Scan(&f.ActivitiesCreated)

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
	facts := app.loadBadgeFacts(ctx, aid, mid, now)
	for _, key := range earnedBadges(facts) {
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
	out := make([]badgeView, 0, len(badgeRules))
	for _, r := range badgeRules {
		v := badgeView{badgeDef: r.badgeDef, Progress: min(r.metric(facts), r.Target)}
		if t, ok := earned[r.Key]; ok {
			v.Earned, v.EarnedAt = true, &t
			v.Progress = r.Target
		}
		out = append(out, v)
	}
	return out
}

// weekly household challenges

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
	bonusAllChores     = 40
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
	WeekIsEligible bool  // household existed before the week began
	ChoresTotal    int64 // --- chores --- chore occurrences due this week
	ChoresDone     int64 // ... of which approved
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
	case "all_chores":
		// every chore occurrence due this week approved, final once Sunday is over
		if s.ChoresTotal == 0 {
			return 0, 0, false, false
		}
		return s.ChoresDone, s.ChoresTotal, true, s.Ended && s.ChoresDone == s.ChoresTotal
	}
	return 0, 0, false, false
}

var challengeDefs = []struct {
	Key, Name, Description, Icon, Unit string
	Bonus                              int64
	LowerIsBetter                      bool
}{
	{"log_every_day", "Log every day this week", "Log at least one spending entry on each of the 7 days (Mon-Sun, UTC).", "", "days", bonusLogEveryDay, false},
	{"no_spend_weekend", "No-spend weekend", "Spend nothing on Saturday and Sunday (log something during the week so we know you're tracking).", "", "days", bonusNoSpendWkend, false},
	{"wants_under", "Keep Wants under budget", "Keep this week's Wants spending under a week's share of your monthly Wants budget.", "", "amount", bonusWantsUnderCap, true},
	{"all_chores", "Finish all your chores this week", "Get every chore due this week done (and approved) by Sunday.", "", "chores", bonusAllChores, false},
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
	_ = app.DB.QueryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE status='approved') FROM chore_instances
		WHERE account_id=$1 AND due_date >= $2 AND due_date < $3`, aid, weekStart, end).Scan(&s.ChoresTotal, &s.ChoresDone)
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
