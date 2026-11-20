package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// trend

func (app *App) handleTrend(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	year := queryIntDefault(r, "year", now.Year())
	month := queryIntDefault(r, "month", int(now.Month()))
	count := queryIntDefault(r, "count", 6)
	ctx := context.Background()

	type trendRow struct {
		Year         int   `json:"year"`
		Month        int   `json:"month"`
		Income       int64 `json:"income"`        // planned (budget_income)
		IncomeActual int64 `json:"income_actual"` // income_entries
		Expense      int64 `json:"expense"`
	}
	out := []trendRow{}
	y, m := year, month
	for i := 0; i < count; i++ {
		var income int64
		if err := app.DB.QueryRow(ctx, "SELECT COALESCE(SUM(amount),0) FROM budget_income WHERE account_id=$1 AND year=$2 AND month=$3", aid, y, m).Scan(&income); err != nil {
			dbError(w, err)
			return
		}
		q := "SELECT COALESCE(SUM(amount),0) FROM spending WHERE account_id=$1 AND EXTRACT(YEAR FROM tx_date)::int=$2 AND EXTRACT(MONTH FROM tx_date)::int=$3"
		args := []any{aid, y, m}
		q, args = appendCeiling(q, args, r, "amount")
		var expense int64
		if err := app.DB.QueryRow(ctx, q, args...).Scan(&expense); err != nil {
			dbError(w, err)
			return
		}
		incomeActual, err := app.actualIncome(ctx, aid, y, m)
		if err != nil {
			dbError(w, err)
			return
		}
		out = append(out, trendRow{Year: y, Month: m, Income: income, IncomeActual: incomeActual, Expense: expense})
		m--
		if m == 0 {
			m = 12
			y--
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *App) handleYearsOverview(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	ctx := context.Background()
	rows, err := app.DB.Query(ctx,
		"SELECT DISTINCT EXTRACT(YEAR FROM tx_date)::int AS y FROM spending WHERE account_id=$1 "+
			"UNION SELECT DISTINCT year FROM budget_income WHERE account_id=$1 "+
			"UNION SELECT DISTINCT EXTRACT(YEAR FROM tx_date)::int FROM income_entries WHERE account_id=$1 ORDER BY y", aid)
	if err != nil {
		dbError(w, err)
		return
	}
	years := []int{}
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		years = append(years, y)
	}
	rows.Close()

	type yearRow struct {
		Year         int   `json:"year"`
		Income       int64 `json:"income"` // planned
		IncomeActual int64 `json:"income_actual"`
		Expense      int64 `json:"expense"`
	}
	out := []yearRow{}
	for _, y := range years {
		var income int64
		if err := app.DB.QueryRow(ctx, "SELECT COALESCE(SUM(amount),0) FROM budget_income WHERE account_id=$1 AND year=$2", aid, y).Scan(&income); err != nil {
			dbError(w, err)
			return
		}
		q := "SELECT COALESCE(SUM(amount),0) FROM spending WHERE account_id=$1 AND EXTRACT(YEAR FROM tx_date)::int=$2"
		args := []any{aid, y}
		q, args = appendCeiling(q, args, r, "amount")
		var expense int64
		if err := app.DB.QueryRow(ctx, q, args...).Scan(&expense); err != nil {
			dbError(w, err)
			return
		}
		incomeActual, err := app.actualIncome(ctx, aid, y, 0)
		if err != nil {
			dbError(w, err)
			return
		}
		out = append(out, yearRow{Year: y, Income: income, IncomeActual: incomeActual, Expense: expense})
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *App) handleYearsByCategory(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	ctx := context.Background()
	q := `SELECT EXTRACT(YEAR FROM sp.tx_date)::int as year,
	             COALESCE(c.name, sp.category_name, 'Other') as category_name,
	             SUM(sp.amount) as total
	      FROM spending sp LEFT JOIN categories c ON c.id=sp.category_id
	      WHERE sp.account_id=$1`
	args := []any{aid}
	q, args = appendCeiling(q, args, r, "sp.amount")
	q += " GROUP BY 1, 2" // positional: bare category_name would bind to sp.category_name, not the alias
	rows, err := app.DB.Query(ctx, q, args...)
	if err != nil {
		dbError(w, err)
		return
	}
	type agg struct {
		year  int
		cat   string
		total int64
	}
	all := []agg{}
	yearSet := map[int]bool{}
	for rows.Next() {
		var a agg
		if err := rows.Scan(&a.year, &a.cat, &a.total); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		all = append(all, a)
		yearSet[a.year] = true
	}
	rows.Close()

	years := []int{}
	for y := range yearSet {
		years = append(years, y)
	}
	sort.Ints(years)

	cats := map[string]map[int]int64{}
	sums := map[string]int64{}
	for _, a := range all {
		if cats[a.cat] == nil {
			cats[a.cat] = map[int]int64{}
		}
		cats[a.cat][a.year] = a.total
		sums[a.cat] += a.total
	}
	names := []string{}
	for name := range cats {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return sums[names[i]] > sums[names[j]] })

	outRows := []map[string]any{}
	for _, name := range names {
		row := map[string]any{"category_name": name}
		for _, y := range years {
			row[strconv.Itoa(y)] = cats[name][y]
		}
		outRows = append(outRows, row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"years": years, "rows": outRows})
}

func (app *App) handleYearsMonthly(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	ctx := context.Background()
	q := `SELECT EXTRACT(YEAR FROM tx_date)::int as year, EXTRACT(MONTH FROM tx_date)::int as month, SUM(amount) as total
	      FROM spending WHERE account_id=$1`
	args := []any{aid}
	q, args = appendCeiling(q, args, r, "amount")
	q += " GROUP BY year, month"
	rows, err := app.DB.Query(ctx, q, args...)
	if err != nil {
		dbError(w, err)
		return
	}
	byYear := map[int][12]int64{}
	years := []int{}
	yearSeen := map[int]bool{}
	for rows.Next() {
		var y, m int
		var total int64
		if err := rows.Scan(&y, &m, &total); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		if !yearSeen[y] {
			yearSeen[y] = true
			years = append(years, y)
		}
		arr := byYear[y]
		arr[m-1] = total
		byYear[y] = arr
	}
	rows.Close()
	sort.Ints(years)
	outByYear := map[string][]int64{}
	for _, y := range years {
		arr := byYear[y]
		outByYear[strconv.Itoa(y)] = arr[:]
	}
	writeJSON(w, http.StatusOK, map[string]any{"years": years, "by_year": outByYear})
}

type txDateAmount struct {
	Date time.Time
	Amt  int64
}

func (app *App) fetchSpendingDates(ctx context.Context, aid int64, extraWhere string, extraArgs []any) ([]txDateAmount, error) {
	q := "SELECT tx_date, amount FROM spending WHERE account_id=$1" + extraWhere
	args := append([]any{aid}, extraArgs...)
	rows, err := app.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []txDateAmount{}
	for rows.Next() {
		var d time.Time
		var amt int64
		if err := rows.Scan(&d, &amt); err != nil {
			return nil, err
		}
		out = append(out, txDateAmount{Date: d, Amt: amt})
	}
	return out, rows.Err()
}

func (app *App) handleYearsDayOfMonthAvg(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	rows, err := app.fetchSpendingDates(context.Background(), aid, "", nil)
	if err != nil {
		dbError(w, err)
		return
	}
	ceiling := valueMaxCeiling(r)
	byDay := map[int]int64{}
	monthsWithDay := map[int]map[[2]int]bool{}
	for d := 1; d <= 31; d++ {
		monthsWithDay[d] = map[[2]int]bool{}
	}
	for _, row := range rows {
		if ceiling != nil && row.Amt > *ceiling {
			continue
		}
		day := row.Date.Day()
		byDay[day] += row.Amt
		monthsWithDay[day][[2]int{row.Date.Year(), int(row.Date.Month())}] = true
	}
	type outRow struct {
		Day int     `json:"day"`
		Avg float64 `json:"avg"`
	}
	out := []outRow{}
	for d := 1; d <= 31; d++ {
		if len(monthsWithDay[d]) == 0 {
			continue
		}
		out = append(out, outRow{Day: d, Avg: float64(byDay[d]) / float64(len(monthsWithDay[d]))})
	}
	writeJSON(w, http.StatusOK, out)
}

var monthAbbr = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

func (app *App) handleYearsDatemonthAvg(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	rows, err := app.fetchSpendingDates(context.Background(), aid, "", nil)
	if err != nil {
		dbError(w, err)
		return
	}
	ceiling := valueMaxCeiling(r)
	type md struct{ month, day int }
	byMD := map[md]int64{}
	yearsWithMD := map[md]map[int]bool{}
	for _, row := range rows {
		if ceiling != nil && row.Amt > *ceiling {
			continue
		}
		key := md{int(row.Date.Month()), row.Date.Day()}
		byMD[key] += row.Amt
		if yearsWithMD[key] == nil {
			yearsWithMD[key] = map[int]bool{}
		}
		yearsWithMD[key][row.Date.Year()] = true
	}
	type outRow struct {
		Month int     `json:"month"`
		Day   int     `json:"day"`
		Label string  `json:"label"`
		Avg   float64 `json:"avg"`
	}
	out := []outRow{}
	for month := 1; month <= 12; month++ {
		daysInMonth := 31
		if month == 2 {
			daysInMonth = 29
		} else if month == 4 || month == 6 || month == 9 || month == 11 {
			daysInMonth = 30
		}
		for day := 1; day <= daysInMonth; day++ {
			key := md{month, day}
			if years, ok := yearsWithMD[key]; ok {
				out = append(out, outRow{
					Month: month, Day: day,
					Label: fmt.Sprintf("%d %s", day, monthAbbr[month-1]),
					Avg:   float64(byMD[key]) / float64(len(years)),
				})
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *App) handleWeeklyReport(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	year, err1 := urlInt64(r, "year")
	month, err2 := urlInt64(r, "month")
	if err1 != nil || err2 != nil {
		jsonError(w, http.StatusBadRequest, "invalid year/month")
		return
	}
	q := `SELECT tx_date, amount, COALESCE(c.name, sp.category_name, 'Other') as category_name
	      FROM spending sp LEFT JOIN categories c ON c.id=sp.category_id
	      WHERE sp.account_id=$1 AND EXTRACT(YEAR FROM tx_date)::int=$2 AND EXTRACT(MONTH FROM tx_date)::int=$3`
	args := []any{aid, year, month}
	q, args = appendCeiling(q, args, r, "sp.amount")
	rows, err := app.DB.Query(context.Background(), q, args...)
	if err != nil {
		dbError(w, err)
		return
	}
	type week struct {
		Week       int   `json:"week"`
		Total      int64 `json:"total"`
		ByCategory map[string]int64
	}
	weeks := map[int]*week{}
	order := []int{}
	for rows.Next() {
		var d time.Time
		var amt int64
		var catName string
		if err := rows.Scan(&d, &amt, &catName); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		wk := (d.Day()-1)/7 + 1
		wrow, ok := weeks[wk]
		if !ok {
			wrow = &week{Week: wk, ByCategory: map[string]int64{}}
			weeks[wk] = wrow
			order = append(order, wk)
		}
		wrow.Total += amt
		wrow.ByCategory[catName] += amt
	}
	rows.Close()
	sort.Ints(order)
	type catTotal struct {
		CategoryName string `json:"category_name"`
		Total        int64  `json:"total"`
	}
	type outWeek struct {
		Week       int        `json:"week"`
		Total      int64      `json:"total"`
		ByCategory []catTotal `json:"by_category"`
	}
	out := []outWeek{}
	for _, wk := range order {
		wrow := weeks[wk]
		cats := []catTotal{}
		for k, v := range wrow.ByCategory {
			cats = append(cats, catTotal{k, v})
		}
		sort.Slice(cats, func(i, j int) bool { return cats[i].Total > cats[j].Total })
		out = append(out, outWeek{Week: wrow.Week, Total: wrow.Total, ByCategory: cats})
	}
	writeJSON(w, http.StatusOK, out)
}

// report / summary

func (app *App) handleReport(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	year, err1 := urlInt64(r, "year")
	month, err2 := urlInt64(r, "month")
	if err1 != nil || err2 != nil {
		jsonError(w, http.StatusBadRequest, "invalid year/month")
		return
	}
	ctx := context.Background()

	var incomeTotal, expenseBudget, savingsTotal int64
	if err := app.DB.QueryRow(ctx, "SELECT COALESCE(SUM(amount),0) FROM budget_income WHERE account_id=$1 AND year=$2 AND month=$3", aid, year, month).Scan(&incomeTotal); err != nil {
		dbError(w, err)
		return
	}
	if err := app.DB.QueryRow(ctx, "SELECT COALESCE(SUM(amount),0) FROM budget_expense WHERE account_id=$1 AND year=$2 AND month=$3", aid, year, month).Scan(&expenseBudget); err != nil {
		dbError(w, err)
		return
	}
	if err := app.DB.QueryRow(ctx, "SELECT COALESCE(SUM(amount),0) FROM savings_list WHERE account_id=$1 AND year=$2 AND month=$3", aid, year, month).Scan(&savingsTotal); err != nil {
		dbError(w, err)
		return
	}

	ceilingClause := ""
	ceilingArgs := []any{}
	if ceiling := valueMaxCeiling(r); ceiling != nil {
		ceilingArgs = append(ceilingArgs, *ceiling)
		ceilingClause = fmt.Sprintf(" AND sp.amount <= $%d", 4)
	}
	q := fmt.Sprintf(`SELECT c.name, c.main_category, COALESCE(be.amount,0) as allocation,
	            COALESCE((SELECT SUM(sp.amount) FROM spending sp
	                      WHERE sp.category_id=c.id AND sp.account_id=$1
	                      AND EXTRACT(YEAR FROM sp.tx_date)::int=$2 AND EXTRACT(MONTH FROM sp.tx_date)::int=$3%s),0) as realization
	       FROM categories c
	       LEFT JOIN budget_expense be ON be.category_id=c.id AND be.account_id=$1 AND be.year=$2 AND be.month=$3
	       WHERE c.account_id=$1
	       ORDER BY c.id`, ceilingClause)
	args := append([]any{aid, year, month}, ceilingArgs...)
	rows, err := app.DB.Query(ctx, q, args...)
	if err != nil {
		dbError(w, err)
		return
	}
	type expItem struct {
		CategoryName string `json:"category_name"`
		MainCategory string `json:"main_category"`
		Allocation   int64  `json:"allocation"`
		Realization  int64  `json:"realization"`
	}
	expensesList := []expItem{}
	for rows.Next() {
		var e expItem
		if err := rows.Scan(&e.CategoryName, &e.MainCategory, &e.Allocation, &e.Realization); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		expensesList = append(expensesList, e)
	}
	rows.Close()

	incomeActual, err := app.actualIncome(ctx, aid, int(year), int(month))
	if err != nil {
		dbError(w, err)
		return
	}
	eq := "SELECT COALESCE(SUM(amount),0) FROM spending WHERE account_id=$1 AND EXTRACT(YEAR FROM tx_date)::int=$2 AND EXTRACT(MONTH FROM tx_date)::int=$3"
	eargs := []any{aid, year, month}
	eq, eargs = appendCeiling(eq, eargs, r, "amount")
	var expenseActual int64
	if err := app.DB.QueryRow(ctx, eq, eargs...).Scan(&expenseActual); err != nil {
		dbError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total_income":       incomeTotal, // planned income (budget_income)
		"budgeted_expenses":  expenseBudget,
		"savings":            savingsTotal,
		"left_for_budgeting": incomeTotal - expenseBudget - savingsTotal,
		"expenses_list":      expensesList,
		"income_actual":      incomeActual,  // income_entries this month
		"expense_actual":     expenseActual, // all spending this month (honors value_max)
	})
}

func (app *App) handleReportYear(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	year, err := urlInt64(r, "year")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid year")
		return
	}
	q := `SELECT COALESCE(c.name, sp.category_name, 'Other') as category_name, SUM(sp.amount) as total
	      FROM spending sp LEFT JOIN categories c ON c.id=sp.category_id
	      WHERE sp.account_id=$1 AND EXTRACT(YEAR FROM sp.tx_date)::int=$2`
	args := []any{aid, year}
	q, args = appendCeiling(q, args, r, "sp.amount")
	q += " GROUP BY 1 ORDER BY total DESC" // positional, see handleYearsByCategory
	rows, err := app.DB.Query(context.Background(), q, args...)
	if err != nil {
		dbError(w, err)
		return
	}
	type outRow struct {
		CategoryName string `json:"category_name"`
		Total        int64  `json:"total"`
	}
	out := []outRow{}
	for rows.Next() {
		var o outRow
		if err := rows.Scan(&o.CategoryName, &o.Total); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		out = append(out, o)
	}
	rows.Close()
	writeJSON(w, http.StatusOK, out)
}

func (app *App) handleReportMonthCompare(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	year, err1 := urlInt64(r, "year")
	month, err2 := urlInt64(r, "month")
	if err1 != nil || err2 != nil {
		jsonError(w, http.StatusBadRequest, "invalid year/month")
		return
	}
	ctx := context.Background()
	expenseFor := func(y, m int64) (int64, error) {
		q := "SELECT COALESCE(SUM(amount),0) FROM spending WHERE account_id=$1 AND EXTRACT(YEAR FROM tx_date)::int=$2 AND EXTRACT(MONTH FROM tx_date)::int=$3"
		args := []any{aid, y, m}
		q, args = appendCeiling(q, args, r, "amount")
		var total int64
		err := app.DB.QueryRow(ctx, q, args...).Scan(&total)
		return total, err
	}
	current, err := expenseFor(year, month)
	if err != nil {
		dbError(w, err)
		return
	}
	prevY, prevM := year, month-1
	if month == 1 {
		prevY, prevM = year-1, 12
	}
	previous, err := expenseFor(prevY, prevM)
	if err != nil {
		dbError(w, err)
		return
	}
	lastYear, err := expenseFor(year-1, month)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"current": current, "previous_month": previous, "same_month_last_year": lastYear})
}

func (app *App) handlePatternsYear(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	year, err := urlInt64(r, "year")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid year")
		return
	}
	rows, err := app.fetchSpendingDates(context.Background(), aid, " AND EXTRACT(YEAR FROM tx_date)::int=$2", []any{year})
	if err != nil {
		dbError(w, err)
		return
	}
	ceiling := valueMaxCeiling(r)

	byDate := map[int]int64{}
	byWeek := map[int]int64{}
	byWeekday := map[int]int64{}
	byWeekInMonth := map[int]int64{}
	monthsPresentSet := map[[2]int]bool{}
	for _, row := range rows {
		if ceiling != nil && row.Amt > *ceiling {
			continue
		}
		d := row.Date
		byDate[d.Day()] += row.Amt
		_, wk := d.ISOWeek()
		byWeek[wk] += row.Amt
		pyWeekday := (int(d.Weekday()) + 6) % 7
		byWeekday[pyWeekday] += row.Amt
		wim := (d.Day()-1)/7 + 1
		byWeekInMonth[wim] += row.Amt
		monthsPresentSet[[2]int{d.Year(), int(d.Month())}] = true
	}

	dayCounts := map[int]int{}
	for ym := range monthsPresentSet {
		daysInMonth := time.Date(ym[0], time.Month(ym[1]+1), 0, 0, 0, 0, 0, time.UTC).Day()
		for dom := 1; dom <= daysInMonth; dom++ {
			dayCounts[dom]++
		}
	}
	type dayPattern struct {
		Day int     `json:"day"`
		Avg float64 `json:"avg"`
	}
	datePattern := []dayPattern{}
	for dom := 1; dom <= 31; dom++ {
		if dayCounts[dom] > 0 {
			datePattern = append(datePattern, dayPattern{Day: dom, Avg: float64(byDate[dom]) / float64(dayCounts[dom])})
		}
	}

	weekKeys := []int{}
	for wk := range byWeek {
		weekKeys = append(weekKeys, wk)
	}
	sort.Ints(weekKeys)
	type weekPattern struct {
		Week int     `json:"week"`
		Avg  float64 `json:"avg"`
	}
	weekPatternOut := []weekPattern{}
	for _, wk := range weekKeys {
		weekPatternOut = append(weekPatternOut, weekPattern{Week: wk, Avg: float64(byWeek[wk])})
	}

	occ := map[int]int{}
	d0 := time.Date(int(year), 1, 1, 0, 0, 0, 0, time.UTC)
	d1 := time.Date(int(year), 12, 31, 0, 0, 0, 0, time.UTC)
	for cur := d0; !cur.After(d1); cur = cur.AddDate(0, 0, 1) {
		pyWeekday := (int(cur.Weekday()) + 6) % 7
		occ[pyWeekday]++
	}
	type weekdayPattern struct {
		Weekday int     `json:"weekday"`
		Avg     float64 `json:"avg"`
	}
	weekdayPatternOut := []weekdayPattern{}
	for wd := 0; wd < 7; wd++ {
		avg := 0.0
		if occ[wd] > 0 {
			avg = float64(byWeekday[wd]) / float64(occ[wd])
		}
		weekdayPatternOut = append(weekdayPatternOut, weekdayPattern{Weekday: wd, Avg: avg})
	}

	monthsCount := len(monthsPresentSet)
	if monthsCount == 0 {
		monthsCount = 1
	}
	type weekInMonthPattern struct {
		Week int     `json:"week"`
		Avg  float64 `json:"avg"`
	}
	weekInMonthOut := []weekInMonthPattern{}
	for wim := 1; wim <= 5; wim++ {
		weekInMonthOut = append(weekInMonthOut, weekInMonthPattern{Week: wim, Avg: float64(byWeekInMonth[wim]) / float64(monthsCount)})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"by_date":          datePattern,
		"by_week":          weekPatternOut,
		"by_weekday":       weekdayPatternOut,
		"by_week_in_month": weekInMonthOut,
	})
}

func (app *App) handleSummary(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	year := queryIntDefault(r, "year", now.Year())
	month := queryIntDefault(r, "month", int(now.Month()))
	ctx := context.Background()

	type mainCatTotal struct {
		MainCategory string `json:"main_category"`
		Total        int64  `json:"total"`
	}
	byMain := []mainCatTotal{}
	rows, err := app.DB.Query(ctx,
		`SELECT c.main_category, COALESCE(SUM(sp.amount),0) as total
		 FROM spending sp JOIN categories c ON c.id=sp.category_id
		 WHERE sp.account_id=$1 AND EXTRACT(YEAR FROM sp.tx_date)::int=$2 AND EXTRACT(MONTH FROM sp.tx_date)::int=$3
		 GROUP BY c.main_category`, aid, year, month)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var m mainCatTotal
		if err := rows.Scan(&m.MainCategory, &m.Total); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		byMain = append(byMain, m)
	}
	rows.Close()

	type catTotal struct {
		CategoryName string `json:"category_name"`
		Total        int64  `json:"total"`
	}
	byCategory := []catTotal{}
	rows, err = app.DB.Query(ctx,
		`SELECT COALESCE(c.name, sp.category_name, 'Other') as category_name, COALESCE(SUM(sp.amount),0) as total
		 FROM spending sp LEFT JOIN categories c ON c.id=sp.category_id
		 WHERE sp.account_id=$1 AND EXTRACT(YEAR FROM sp.tx_date)::int=$2 AND EXTRACT(MONTH FROM sp.tx_date)::int=$3
		 GROUP BY 1 ORDER BY total DESC`, aid, year, month)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var c catTotal
		if err := rows.Scan(&c.CategoryName, &c.Total); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		byCategory = append(byCategory, c)
	}
	rows.Close()

	accounts := []accountRow{}
	rows, err = app.DB.Query(ctx, "SELECT id, account_id, name, kind FROM accounts WHERE account_id=$1 AND kind='bank' ORDER BY id", aid)
	if err != nil {
		dbError(w, err)
		return
	}
	for rows.Next() {
		var a accountRow
		if err := rows.Scan(&a.ID, &a.AccountID, &a.Name, &a.Kind); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		accounts = append(accounts, a)
	}
	rows.Close()

	writeJSON(w, http.StatusOK, map[string]any{"by_main_category": byMain, "by_category": byCategory, "accounts": accounts})
}

// actualIncome sums income_entries for year/month (month 0 = whole year).
func (app *App) actualIncome(ctx context.Context, aid int64, year, month int) (int64, error) {
	from, to := monthRange(year, month)
	var total int64
	err := app.DB.QueryRow(ctx,
		"SELECT COALESCE(SUM(amount),0) FROM income_entries WHERE account_id=$1 AND tx_date >= $2 AND tx_date < $3",
		aid, from, to).Scan(&total)
	return total, err
}
