import { useState } from 'react'
import { api, formatRupiah, MONTHS, MONTH_NAMES } from '../api'
import { useYearMonth } from '../YearContext.jsx'
import { useApi } from '../useApi'
import { categoryColor, seriesColor } from '../categoryColor'
import PieChart from '../PieChart.jsx'
import BarChart from '../BarChart.jsx'
import LineChart from '../LineChart.jsx'
import PageHeader from '../PageHeader.jsx'
import StatCard from '../StatCard.jsx'
import Collapsible from '../Collapsible.jsx'
import { SkeletonStats, SkeletonCard } from '../Skeleton.jsx'
import CustomDashboard from './CustomDashboard.jsx'
import { ChevronLink } from '../ChevronLink.jsx'

const TABS = ['Across Years', 'Year', 'Month', 'Week', 'Custom']
// Which period control each tab needs in the page header.
const TAB_PERIOD = { 'Across Years': null, Year: 'year', Month: 'month', Week: 'month', Custom: 'month' }
const TAB_KEY = 'kinkit_dashboard_tab'
const WEEKDAY_NAMES = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday']

const tabId = t => `dash-tab-${t.replace(/\s+/g, '-').toLowerCase()}`

export default function Dashboard() {
  const { year, month } = useYearMonth()
  const [tab, setTabState] = useState(() => {
    try { const t = localStorage.getItem(TAB_KEY); return TABS.includes(t) ? t : 'Month' } catch { return 'Month' }
  })
  function setTab(t) {
    setTabState(t)
    try { localStorage.setItem(TAB_KEY, t) } catch { /* ignore */ }
  }

  // Arrow-key navigation between tabs (WAI-ARIA tabs pattern).
  function onTabKey(e) {
    const i = TABS.indexOf(tab)
    let next = null
    if (e.key === 'ArrowRight') next = TABS[(i + 1) % TABS.length]
    if (e.key === 'ArrowLeft') next = TABS[(i - 1 + TABS.length) % TABS.length]
    if (next) {
      e.preventDefault()
      setTab(next)
      document.getElementById(tabId(next))?.focus()
    }
  }

  const period = TAB_PERIOD[tab]
  const subtitle = period === 'month' ? `${MONTH_NAMES[month - 1]} ${year}` : period === 'year' ? `Year ${year}` : 'All years'

  return (
    <div>
      <PageHeader
        title="Dashboard"
        subtitle={subtitle}
        period={period}
        valueFilter
        actions={<button className="export-pdf-btn secondary" onClick={() => window.print()}>Export PDF</button>}
      />

      <div className="tabs" role="tablist" aria-label="Dashboard views" onKeyDown={onTabKey}>
        {TABS.map(t => (
          <button
            key={t}
            id={tabId(t)}
            role="tab"
            aria-selected={tab === t}
            aria-controls="dash-panel"
            tabIndex={tab === t ? 0 : -1}
            className={`tab ${tab === t ? 'active' : ''}`}
            onClick={() => setTab(t)}
          >
            {t}
          </button>
        ))}
      </div>

      <div id="dash-panel" role="tabpanel" aria-labelledby={tabId(tab)}>
        {tab === 'Across Years' && <YearsTab />}
        {tab === 'Year' && <YearTab />}
        {tab === 'Month' && <MonthTab />}
        {tab === 'Week' && <WeekTab />}
        {tab === 'Custom' && <CustomDashboard />}
      </div>
    </div>
  )
}

function TabState({ error, stats = 4 }) {
  if (error) return <div className="error-box">{error}</div>
  return <><SkeletonStats count={stats} /><SkeletonCard chart lines={1} /></>
}

function CategoryBars({ rows, valueKey, emptyText = 'No data yet.' }) {
  const max = Math.max(1, ...rows.map(r => r[valueKey]))
  if (rows.length === 0) return <div className="muted">{emptyText}</div>
  return rows.map(c => {
    const color = categoryColor(c.category_name)
    return (
      <div className="bar-row" key={c.category_name}>
        <div className="bar-label"><span className="cat-dot" style={{ background: color }} />{c.category_name}</div>
        <div className="progress-bar">
          <div className="progress-fill" style={{ width: `${Math.round((c[valueKey] / max) * 100)}%`, background: color }} />
        </div>
        <div className="bar-value">{formatRupiah(c[valueKey])}</div>
      </div>
    )
  })
}

function PatternChart({ title, desc, labels, data, label = 'Average' }) {
  if (!data.length) return null
  return (
    <div className="card">
      <h2>{title}</h2>
      {desc && <p className="muted">{desc}</p>}
      <LineChart labels={labels} datasets={[{ label, data, color: 'var(--primary)' }]} />
    </div>
  )
}

// Tab: Across Years
function YearsTab() {
  const { valueMax } = useYearMonth()
  const { data, error } = useApi(
    () => Promise.all([api.years(valueMax), api.yearsByCategory(valueMax), api.yearsMonthly(valueMax)]),
    [valueMax],
  )
  if (!data) return <TabState error={error} />
  const [years, byCategory, monthly] = data

  const income = years.reduce((s, y) => s + y.income, 0)
  const expense = years.reduce((s, y) => s + y.expense, 0)
  const avgExpense = years.length ? expense / years.length : 0

  return (
    <div>
      {error && <div className="error-box">{error}</div>}
      <div className="cards-row">
        <StatCard label="All-time Income" value={formatRupiah(income)} tone="income" />
        <StatCard label="All-time Expense" value={formatRupiah(expense)} tone="expense" />
        <StatCard label="All-time Net" value={formatRupiah(income - expense)} tone={income - expense >= 0 ? 'positive' : 'negative'} />
        <StatCard label="Avg Expense / Year" value={formatRupiah(avgExpense)} sub={`${years.length} year${years.length === 1 ? '' : 's'} of data`} />
      </div>

      <div className="card">
        <h2>Year-over-Year Comparison</h2>
        {years.length === 0 && <div className="muted">No data yet.</div>}
        {years.length > 0 && (
          <BarChart
            labels={years.map(y => String(y.year))}
            datasets={[
              { label: 'Income', data: years.map(y => y.income), color: 'var(--success)' },
              { label: 'Expense', data: years.map(y => y.expense), color: 'var(--warning)' },
            ]}
          />
        )}
      </div>

      <div className="card table-scroll">
        <table>
          <thead><tr><th>Year</th><th className="num">Income (planned)</th><th className="num">Income (received)</th><th className="num">Expense</th><th className="num">Net</th></tr></thead>
          <tbody>
            {years.map(y => (
              <tr key={y.year}>
                <td>{y.year}</td>
                <td className="num">{formatRupiah(y.income)}</td>
                <td className="num">{formatRupiah(y.income_actual || 0)}</td>
                <td className="num">{formatRupiah(y.expense)}</td>
                <td className={`num ${y.income - y.expense >= 0 ? 'text-positive' : 'text-negative'}`}>{formatRupiah(y.income - y.expense)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {monthly.years.length > 0 && (
        <div className="card">
          <h2>Monthly Spending - Across Years</h2>
          <LineChart
            labels={MONTHS}
            datasets={monthly.years.map((y, i) => ({ label: String(y), data: (monthly.by_year[y] || []), color: seriesColor(i) }))}
          />
        </div>
      )}

      <div className="card">
        <h2>Spending by Category - Across Years</h2>
        {byCategory.rows.length === 0 && <div className="muted">No data yet.</div>}
        {byCategory.rows.length > 0 && (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Category</th>
                  {byCategory.years.map(y => <th key={y} className="num">{y}</th>)}
                </tr>
              </thead>
              <tbody>
                {byCategory.rows.map(row => (
                  <tr key={row.category_name}>
                    <td><span className="cat-dot" style={{ background: categoryColor(row.category_name) }} />{row.category_name}</td>
                    {byCategory.years.map(y => <td key={y} className="num">{formatRupiah(row[String(y)])}</td>)}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <Collapsible title="More insights - spending patterns across all years" storageKey="kinkit_dash_years_insights">
        <YearsInsights valueMax={valueMax} monthly={monthly} />
      </Collapsible>
    </div>
  )
}

// Only mounted when "More insights" is open, so these requests are lazy.
function YearsInsights({ valueMax, monthly }) {
  const { data, error } = useApi(
    () => Promise.all([api.yearsDayOfMonthAvg(valueMax), api.yearsDatemonthAvg(valueMax)]),
    [valueMax],
  )
  // average expense per month-name, across all years that have data for that month
  const monthNameAvg = MONTHS.map((_, i) => {
    const vals = monthly.years.map(y => monthly.by_year[y]?.[i] || 0).filter(v => v > 0)
    return vals.length ? vals.reduce((s, v) => s + v, 0) / vals.length : 0
  })
  if (error) return <div className="error-box">{error}</div>
  if (!data) return <SkeletonCard chart lines={0} />
  const [dayOfMonthAvg, datemonthAvg] = data

  return (
    <>
      {monthNameAvg.some(v => v > 0) && (
        <PatternChart title="Average Expense per Month (Pattern Across All Years)" labels={MONTHS} data={monthNameAvg} />
      )}
      <PatternChart
        title="Average Expense per Date of Month (Pattern Across All Years)"
        labels={dayOfMonthAvg.map(d => String(d.day))}
        data={dayOfMonthAvg.map(d => d.avg)}
      />
      {datemonthAvg.length > 0 && (
        <div className="card">
          <h2>Average Expense per Date (Jan 1 - Dec 31, Pattern Across All Years)</h2>
          <div style={{ overflowX: 'auto' }}>
            <div style={{ minWidth: `${datemonthAvg.length * 24}px` }}>
              <LineChart
                labels={datemonthAvg.map(d => d.label)}
                datasets={[{ label: 'Average', data: datemonthAvg.map(d => d.avg), color: 'var(--primary)' }]}
              />
            </div>
          </div>
        </div>
      )}
    </>
  )
}

// Tab: Year (12-month view of selected year)
function YearTab() {
  const { year, valueMax } = useYearMonth()
  const { data, error } = useApi(
    () => Promise.all([api.trend(year, 12, 12, valueMax), api.reportYear(year, valueMax)]),
    [year, valueMax],
  )
  if (!data) return <TabState error={error} />
  const [trend, catBreakdown] = data

  const totalIncome = trend.reduce((s, t) => s + t.income, 0)
  const totalIncomeActual = trend.reduce((s, t) => s + (t.income_actual || 0), 0)
  const totalExpense = trend.reduce((s, t) => s + t.expense, 0)
  const activeMonths = trend.filter(t => t.expense > 0).length

  return (
    <div>
      {error && <div className="error-box">{error}</div>}
      <div className="cards-row">
        <StatCard label={`Income Received ${year}`} value={formatRupiah(totalIncomeActual)} tone="income" sub={`Planned: ${formatRupiah(totalIncome)}`} />
        <StatCard label={`Total Expense ${year}`} value={formatRupiah(totalExpense)} tone="expense" />
        <StatCard label={`Net ${year}`} value={formatRupiah(totalIncome - totalExpense)} tone={totalIncome - totalExpense >= 0 ? 'positive' : 'negative'} />
        <StatCard label="Avg Expense / Month" value={formatRupiah(activeMonths ? totalExpense / activeMonths : 0)} sub={`over ${activeMonths} month${activeMonths === 1 ? '' : 's'} with spending`} />
      </div>

      <div className="card">
        <h2>12 Months - {year}</h2>
        {trend.length > 0 && <IncomeExpenseChart trend={trend} />}
      </div>

      <div className="grid-2">
        <div className="card">
          <h2>Breakdown by Category - {year}</h2>
          <CategoryBars rows={catBreakdown} valueKey="total" />
        </div>
        <div className="card">
          <h2>Category Breakdown - {year}</h2>
          <PieChart data={catBreakdown} />
        </div>
      </div>

      <Collapsible title={`More insights - spending patterns in ${year}`} storageKey="kinkit_dash_year_insights">
        <YearPatterns year={year} valueMax={valueMax} />
      </Collapsible>
    </div>
  )
}

function YearPatterns({ year, valueMax }) {
  const { data: p, error } = useApi(() => api.patternsYear(year, valueMax), [year, valueMax])
  if (error) return <div className="error-box">{error}</div>
  if (!p) return <SkeletonCard chart lines={0} />
  return (
    <>
      <PatternChart title={`Pattern by Date of Month - ${year}`} desc="Average expense for each date (1-31), averaged across the whole year"
        labels={p.by_date.map(d => String(d.day))} data={p.by_date.map(d => d.avg)} />
      <PatternChart title={`Pattern by Week of Year - ${year}`} desc={`Total expense per ISO week across ${year}`} label="Total"
        labels={p.by_week.map(w => `W${w.week}`)} data={p.by_week.map(w => w.avg)} />
      <PatternChart title={`Pattern by Week in Month - ${year}`} desc="Average expense for week 1-5 within a month, averaged across the whole year"
        labels={p.by_week_in_month.map(w => `Week ${w.week}`)} data={p.by_week_in_month.map(w => w.avg)} />
      <PatternChart title={`Pattern by Day of Week - ${year}`} desc="Average expense per weekday, averaged across the whole year"
        labels={WEEKDAY_NAMES} data={p.by_weekday.map(w => w.avg)} />
    </>
  )
}

// --- money --- planned vs received income next to expense.
function IncomeExpenseChart({ trend }) {
  const hasActual = trend.some(t => t.income_actual > 0)
  return (
    <BarChart
      labels={trend.map(t => MONTHS[t.month - 1])}
      datasets={[
        { label: hasActual ? 'Income (planned)' : 'Income', data: trend.map(t => t.income), color: hasActual ? '#86efac' : 'var(--success)' },
        ...(hasActual ? [{ label: 'Income (received)', data: trend.map(t => t.income_actual || 0), color: 'var(--success)' }] : []),
        { label: 'Expense', data: trend.map(t => t.expense), color: 'var(--warning)' },
      ]}
    />
  )
}

// "Spent X of Y budgeted" + categories at/over budget.
function BudgetProgress({ report, spent }) {
  const budgeted = report.budgeted_expenses
  const pct = budgeted > 0 ? (spent / budgeted) * 100 : null
  const state = pct === null ? '' : pct > 100 ? 'over' : pct >= 80 ? 'warn' : 'ok'
  const flagged = report.expenses_list
    .filter(r => r.realization > 0 && (r.allocation === 0 || r.realization >= r.allocation * 0.8))
    .map(r => ({ ...r, pct: r.allocation > 0 ? (r.realization / r.allocation) * 100 : null }))
    .sort((a, b) => (b.pct ?? Infinity) - (a.pct ?? Infinity))
  return (
    <div className={`card budget-progress ${state}`}>
      <div className="budget-progress-head">
        <h2>
          Spent {formatRupiah(spent)} <span className="muted">of {formatRupiah(budgeted)} budgeted this month</span>
        </h2>
        {pct !== null && <span className={`budget-progress-pct ${state}`}>{Math.round(pct)}%</span>}
      </div>
      {pct === null ? (
        <>
          <p className="muted">No budget set for this month yet.</p>
          <ChevronLink to="/budget" className="standalone">Plan it on the Budget page</ChevronLink>
        </>
      ) : (
        <>
          <div className="progress-bar progress-lg" role="progressbar" aria-valuenow={Math.round(pct)} aria-valuemin={0} aria-valuemax={100} aria-label="Budget used">
            <div className={`progress-fill bva-fill-${state}`} style={{ width: `${Math.min(100, pct)}%` }} />
          </div>
          <p className="muted">
            {spent <= budgeted ? `${formatRupiah(budgeted - spent)} left to spend` : `${formatRupiah(spent - budgeted)} over budget`}
          </p>
        </>
      )}
      {flagged.length > 0 && (
        <div className="over-list">
          <h3>Needs attention</h3>
          {flagged.map(r => (
            <div className="bar-row" key={r.category_name}>
              <div className="bar-label"><span className="cat-dot" style={{ background: categoryColor(r.category_name) }} />{r.category_name}</div>
              <div className="progress-bar">
                <div className={`progress-fill bva-fill-${r.pct === null || r.pct > 100 ? 'over' : 'warn'}`} style={{ width: `${Math.min(100, r.pct ?? 100)}%` }} />
              </div>
              <div className="bar-value">
                {r.pct === null ? <span className="text-negative">No budget</span> : <span className={r.pct > 100 ? 'text-negative' : 'bva-warn-text'}>{Math.round(r.pct)}%</span>}
                {' '}{r.realization > r.allocation ? `+${formatRupiah(r.realization - r.allocation)}` : formatRupiah(r.realization)}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// Tab: Month
const pctChange = (curr, base) => (base > 0 ? ((curr - base) / base) * 100 : (curr > 0 ? null : 0))
function PctText({ pct, label }) {
  if (pct === null) return <span>N/A {label}</span>
  // more spending = bad (red), less = good (green)
  const cls = pct > 0 ? 'text-negative' : pct < 0 ? 'text-positive' : ''
  return <span><span className={cls}>{pct >= 0 ? '+' : ''}{pct.toFixed(1)}%</span> {label}</span>
}

function MonthTab() {
  const { year, month, valueMax } = useYearMonth()
  const { data, error } = useApi(
    () => Promise.all([
      api.report(year, month, valueMax),
      api.trend(year, month, 6, valueMax),
      api.spending(year, month, 1, 6, { valueMax }),
      api.reportMonthCompare(year, month, valueMax),
    ]),
    [year, month, valueMax],
  )
  if (!data) return <TabState error={error} />
  const [report, trend, recentRes, compare] = data
  const recent = recentRes.rows

  const topCategories = [...report.expenses_list].filter(r => r.realization > 0).sort((a, b) => b.realization - a.realization).slice(0, 10)
  const byCategorySorted = [...report.expenses_list].sort((a, b) => {
    const pctA = a.allocation > 0 ? a.realization / a.allocation : (a.realization > 0 ? Infinity : 0)
    const pctB = b.allocation > 0 ? b.realization / b.allocation : (b.realization > 0 ? Infinity : 0)
    return pctB - pctA
  })
  const netFlow = report.total_income - report.budgeted_expenses
  const incomeActual = report.income_actual || 0
  const vsPrevMonth = pctChange(compare.current, compare.previous_month)
  const vsLastYear = pctChange(compare.current, compare.same_month_last_year)

  return (
    <div>
      {error && <div className="error-box">{error}</div>}

      <div className="cards-row">
        <StatCard
          label={`Spending ${MONTHS[month - 1]}`}
          value={formatRupiah(compare.current)}
          tone="expense"
          sub={<><PctText pct={vsPrevMonth} label="vs last month" /> · <PctText pct={vsLastYear} label="vs last year" /></>}
        />
        <StatCard
          label="Income Received"
          value={formatRupiah(incomeActual)}
          tone="income"
          sub={`Planned: ${formatRupiah(report.total_income)}${report.total_income > 0 ? ` (${Math.round((incomeActual / report.total_income) * 100)}%)` : ''}`}
        />
        <StatCard label="Savings" value={formatRupiah(report.savings)} tone="savings" />
        <StatCard
          label="Left for Budgeting"
          value={formatRupiah(report.left_for_budgeting)}
          tone={netFlow >= 0 ? 'positive' : 'negative'}
          sub={`Budgeted expenses: ${formatRupiah(report.budgeted_expenses)}`}
        />
      </div>

      <BudgetProgress report={report} spent={report.expense_actual ?? compare.current} />

      <div className="card">
        <h2>6-Month Trend</h2>
        {trend.length > 0 && <IncomeExpenseChart trend={trend} />}
      </div>

      <div className="grid-2">
        <div className="card">
          <h2>Top 10 Expenses</h2>
          <CategoryBars rows={topCategories} valueKey="realization" emptyText="No expenses this month yet." />
        </div>

        <div className="card">
          <h2>Recent Transactions</h2>
          {recent.length === 0 && <div className="muted">No transactions yet.</div>}
          {recent.length > 0 && (
            <div className="table-scroll">
              <table className="responsive-table">
                <thead><tr><th>Date</th><th>Description</th><th>Category</th><th className="num">Amount</th></tr></thead>
                <tbody>
                  {recent.map(s => (
                    <tr key={s.id}>
                      <td data-label="Date">{s.tx_date}</td>
                      <td data-label="Description">{s.description}</td>
                      <td data-label="Category"><span className="tag" style={{ background: categoryColor(s.category_name) + '22', color: categoryColor(s.category_name) }}>{s.category_name || 'Other'}</span></td>
                      <td data-label="Amount" className="num">{formatRupiah(s.amount)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>

      <div className="card table-scroll">
        <h2>Budget vs Actual by Category</h2>
        <table>
          <thead>
            <tr><th>Category</th><th className="num">Allocation</th><th className="num">Realization</th><th>Progress</th></tr>
          </thead>
          <tbody>
            {byCategorySorted.map((r, i) => {
              const pctRaw = r.allocation > 0 ? Math.round((r.realization / r.allocation) * 100) : 0
              const over = r.realization > r.allocation
              const color = categoryColor(r.category_name)
              return (
                <tr key={i}>
                  <td><span className="cat-dot" style={{ background: color }} />{r.category_name}</td>
                  <td className="num">{formatRupiah(r.allocation)}</td>
                  <td className="num">{formatRupiah(r.realization)}</td>
                  <td className="progress-cell">
                    <div className="progress-bar">
                      <div className={`progress-fill ${over ? 'over' : ''}`} style={{ width: `${Math.min(100, pctRaw)}%`, background: over ? undefined : color }} />
                    </div>
                    <span className="progress-label">{pctRaw}%</span>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}

// Tab: Week (weekly breakdown within selected month)
function WeekTab() {
  const { year, month, valueMax } = useYearMonth()
  const { data: weeks, error } = useApi(() => api.weeklyReport(year, month, valueMax), [year, month, valueMax])
  if (!weeks) return <TabState error={error} stats={3} />

  const total = weeks.reduce((s, w) => s + w.total, 0)
  const maxTotal = Math.max(1, ...weeks.map(w => w.total))
  const busiest = weeks.reduce((b, w) => (!b || w.total > b.total ? w : b), null)

  // grouped bar: categories on x-axis, one series per week
  const catTotal = cat => weeks.reduce((s, w) => s + (w.by_category.find(c => c.category_name === cat)?.total || 0), 0)
  const allCats = [...new Set(weeks.flatMap(w => w.by_category.map(c => c.category_name)))].sort((a, b) => catTotal(b) - catTotal(a))

  return (
    <div>
      {error && <div className="error-box">{error}</div>}
      <div className="cards-row">
        <StatCard label={`Total ${MONTHS[month - 1]}`} value={formatRupiah(total)} tone="expense" />
        <StatCard label="Average / Week" value={formatRupiah(weeks.length ? total / weeks.length : 0)} />
        <StatCard label="Busiest Week" value={busiest ? formatRupiah(busiest.total) : '-'} tone="negative" sub={busiest ? `Week ${busiest.week}` : undefined} />
      </div>

      {weeks.length === 0 && <div className="card empty-hint"><p>No transactions this month yet.</p></div>}

      {weeks.length > 0 && (
        <div className="card">
          <h2>Total Spending per Week</h2>
          <BarChart
            labels={weeks.map(w => `Week ${w.week}`)}
            datasets={[{ label: 'Total', data: weeks.map(w => w.total), color: 'var(--warning)' }]}
          />
        </div>
      )}

      {allCats.length > 0 && (
        <div className="card">
          <h2>By Category - Across Weeks</h2>
          <BarChart
            labels={allCats}
            datasets={weeks.map((w, i) => ({
              label: `Week ${w.week}`,
              data: allCats.map(cat => w.by_category.find(c => c.category_name === cat)?.total || 0),
              color: seriesColor(i),
            }))}
          />
        </div>
      )}

      {weeks.map(w => (
        <div className="card" key={w.week}>
          <h2 className="card-title-row">Week {w.week} <span className="muted num">{formatRupiah(w.total)}</span></h2>
          <div className="progress-bar" style={{ marginBottom: '.8rem' }}>
            <div className="progress-fill" style={{ width: `${(w.total / maxTotal) * 100}%` }} />
          </div>
          {w.by_category.map(c => {
            const color = categoryColor(c.category_name)
            return (
              <div className="bar-row" key={c.category_name}>
                <div className="bar-label"><span className="cat-dot" style={{ background: color }} />{c.category_name}</div>
                <div className="progress-bar">
                  <div className="progress-fill" style={{ width: `${Math.round((c.total / w.total) * 100)}%`, background: color }} />
                </div>
                <div className="bar-value">{formatRupiah(c.total)}</div>
              </div>
            )
          })}
        </div>
      ))}
    </div>
  )
}
