import { useEffect, useState } from 'react'
import { api, dataApi, formatRupiah, MONTHS, setCurrency } from '../api'
import { currentUTCPeriod } from '../YearContext.jsx'
import BarChart from '../BarChart.jsx'
import { Skeleton } from '../Skeleton.jsx'

const REFRESH_MS = 60_000 // auto-refresh for an always-on screen (TV/car head unit)

/**
 * Full-screen, no-sidebar, large-text dashboard meant for a TV or car head
 * unit: glanceable at a distance, no interaction needed, auto-refreshes.
 * Standalone route (/tv) outside the normal <Layout> - deliberately skips
 * auth chrome/nav since this is meant to just sit on a screen.
 * With a share token (/tv?k=..., see TVTokens.jsx) it loads everything from
 * the read-only GET /api/tv/data instead of the logged-in session.
 */
export default function TVView({ token }) {
  const [report, setReport] = useState(null)
  const [trend, setTrend] = useState([])
  const [error, setError] = useState('')
  // UTC "current month", re-evaluated on every refresh so an always-on screen
  // rolls over to the new month by itself.
  const [period, setPeriod] = useState(currentUTCPeriod)
  const { year, month } = period

  useEffect(() => {
    function load() {
      if (token) {
        dataApi.tvData(token)
          .then(d => {
            if (d.currency) setCurrency(d.currency)
            setReport(d.report); setTrend(d.trend); setPeriod({ year: d.year, month: d.month }); setError('')
          })
          .catch(e => {
            setError(e.status === 401 ? 'This TV link is invalid or was revoked. Create a new one under Data & devices.' : e.message)
            if (e.status === 401) setReport(null)
          })
        return
      }
      const p = currentUTCPeriod()
      Promise.all([api.report(p.year, p.month), api.trend(p.year, p.month, 6)])
        .then(([r, t]) => { setReport(r); setTrend(t); setPeriod(p); setError('') })
        .catch(e => setError(e.message))
    }
    load()
    const id = setInterval(load, REFRESH_MS)
    return () => clearInterval(id)
  }, [token])

  if (error && !report) return <div className="tv-view tv-error" role="alert">{error}</div>
  if (!report) {
    return (
      <div className="tv-view" aria-busy="true" aria-label="Loading">
        <Skeleton width={220} height={40} className="tv-skeleton" style={{ marginBottom: '2rem' }} />
        <div className="tv-cards">
          {[0, 1, 2].map(i => <div className="tv-card" key={i}><Skeleton height={60} className="tv-skeleton" /></div>)}
        </div>
      </div>
    )
  }

  const netFlow = report.total_income - report.budgeted_expenses
  const topExpenses = [...report.expenses_list]
    .filter(r => r.realization > 0)
    .sort((a, b) => b.realization - a.realization)
    .slice(0, 5)

  return (
    <div className="tv-view">
      <div className="tv-header">
        <div className="tv-brand">Kinkit</div>
        <div className="tv-date">{MONTHS[month - 1]} {year}</div>
      </div>

      <div className="tv-cards">
        <div className="tv-card">
          <div className="tv-label">Income</div>
          <div className="tv-value tv-positive">{formatRupiah(report.total_income)}</div>
        </div>
        <div className="tv-card">
          <div className="tv-label">Spending</div>
          <div className="tv-value tv-warning">{formatRupiah(report.budgeted_expenses)}</div>
        </div>
        <div className="tv-card">
          <div className="tv-label">Left</div>
          <div className={`tv-value ${netFlow >= 0 ? 'tv-positive' : 'tv-negative'}`}>{formatRupiah(report.left_for_budgeting)}</div>
        </div>
      </div>

      <div className="tv-row">
        <div className="tv-panel">
          <div className="tv-panel-title">6-Month Trend</div>
          {trend.length > 0 && <BarChart labels={trend.map(t => MONTHS[t.month - 1])} datasets={[
            { label: 'Income', data: trend.map(t => t.income), color: 'var(--success)' },
            { label: 'Expense', data: trend.map(t => t.expense), color: 'var(--warning)' },
          ]} height={320} />}
        </div>
        <div className="tv-panel">
          <div className="tv-panel-title">Top Expenses</div>
          {topExpenses.map((c, i) => (
            <div className="tv-expense-row" key={i}>
              <span>{c.category_name}</span>
              <span>{formatRupiah(c.realization)}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
