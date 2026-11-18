import { useMemo, useState } from 'react'
import { api, formatRupiah, MONTHS, MONTH_NAMES } from '../api'
import { useYearMonth } from '../YearContext.jsx'
import { useApi, useSetup } from '../useApi'
import { useToast } from '../Toast.jsx'
import { categoryColor } from '../categoryColor'
import MoneyInput from '../MoneyInput.jsx'
import PageHeader from '../PageHeader.jsx'
import { SkeletonCard } from '../Skeleton.jsx'
import { IconCheck, IconAlert } from '../icons.jsx'
import { ChevronLink } from '../ChevronLink.jsx'

/**
 * One editable budget cell with its own save state:
 * saving spinner -> check mark (fades) | error marker (hover/aria for message).
 * Saves on blur, only when the value actually changed.
 */
function BudgetCell({ value, label, onSave }) {
  const [status, setStatus] = useState('idle') // idle | saving | saved | error
  const [err, setErr] = useState('')

  async function handleBlur(v) {
    const next = Number(v) || 0
    if (status !== 'error' && next === (Number(value) || 0)) return
    setStatus('saving')
    try {
      await onSave(next)
      setStatus('saved')
      setTimeout(() => setStatus(s => (s === 'saved' ? 'idle' : s)), 1500)
    } catch (e) {
      setErr(e.message)
      setStatus('error')
    }
  }

  return (
    <div className={`budget-cell ${status}`} title={status === 'error' ? `Not saved: ${err}` : undefined}>
      <MoneyInput value={value ?? ''} onBlur={handleBlur} aria-label={label} aria-invalid={status === 'error'} />
      <span className="budget-cell-status" aria-live="polite">
        {status === 'saving' && <span className="spinner" role="img" aria-label="Saving" />}
        {status === 'saved' && <IconCheck width={12} height={12} role="img" aria-label="Saved" />}
        {status === 'error' && <IconAlert width={13} height={13} role="img" aria-label={`Not saved: ${err}`} />}
      </span>
    </div>
  )
}

/** Budget-vs-actual state for an expense cell: '' | ok | warn (>=80%) | over (>100%, or spending with no budget). */
function budgetState(budget, actual) {
  if (!actual) return ''
  if (!budget) return 'over'
  const pct = actual / budget
  return pct > 1 ? 'over' : pct >= 0.8 ? 'warn' : 'ok'
}

function ActualNote({ kind, actual, planned }) {
  if (!actual) return null
  const pct = planned > 0 ? Math.round((actual / planned) * 100) : null
  const verb = kind === 'income' ? 'Got' : 'Spent'
  return (
    <div className="bva-note" title={`${verb} ${formatRupiah(actual)}${pct !== null ? ` (${pct}% of plan)` : ' (nothing planned)'}`}>
      {verb} {formatRupiah(actual)}{pct !== null && <span className="bva-pct"> · {pct}%</span>}
    </div>
  )
}

/** Small card: budget alerts via Telegram/Discord on/off + recent alerts. */
function AlertsCard() {
  const toast = useToast()
  const res = useApi(() => api.alertSettings(), [])
  const s = res.data
  if (!s) return null
  const ready = s.channels.filter(c => c.ready)

  async function toggle(e) {
    const next = e.target.checked
    res.setData(d => ({ ...d, alerts_enabled: next }))
    try {
      await api.updateAlertSettings({ alerts_enabled: next })
      toast.success(next ? 'Budget alerts on' : 'Budget alerts off')
    } catch (err) {
      res.setData(d => ({ ...d, alerts_enabled: !next }))
      toast.error(err.message)
    }
  }

  return (
    <div className="card alerts-card">
      <div className="alerts-card-head">
        <div>
          <h2>Budget alerts</h2>
          <p className="muted">
            Get a chat message when a category reaches 80% and when it goes over 100% of this month's budget (once each).
          </p>
        </div>
        <label className="switch">
          <input type="checkbox" checked={s.alerts_enabled} onChange={toggle} aria-label="Send budget alerts" />
          <span>{s.alerts_enabled ? 'On' : 'Off'}</span>
        </label>
      </div>
      {ready.length === 0 ? (
        <>
          <p className="muted">No chat connected yet. Connect Telegram or Discord (bot token + chat id) to receive alerts.</p>
          <ChevronLink to="/settings/bots" className="standalone">Connect Telegram or Discord</ChevronLink>
        </>
      ) : (
        <p className="muted">Sending to: {ready.map(c => c.platform[0].toUpperCase() + c.platform.slice(1)).join(', ')}</p>
      )}
      {s.recent.length > 0 && (
        <ul className="alerts-recent">
          {s.recent.slice(0, 5).map((a, i) => (
            <li key={i}>
              <span className={a.threshold >= 100 ? 'text-negative' : 'bva-warn-text'}>{a.threshold >= 100 ? 'Over budget' : '80% used'}</span>
              {' '}{a.category_name} <span className="muted">· {MONTHS[a.month - 1]} {a.year} · {formatRupiah(a.spent)} of {formatRupiah(a.budget)}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function historyStats(historyMap, id) {
  const years = Object.keys(historyMap?.[id] || {}).map(Number).sort((a, b) => a - b)
  if (years.length === 0) return { lastYear: 0, avg: 0 }
  const lastYear = historyMap[id][years[years.length - 1]]
  const avg = years.reduce((s, y) => s + historyMap[id][y], 0) / years.length
  return { lastYear, avg }
}

export default function Budgeting() {
  const { year } = useYearMonth()
  const { setup, error: setupError } = useSetup()
  const toast = useToast()
  const budget = useApi(() => api.budgeting(year), [year])
  const actuals = useApi(() => api.budgetActuals(year), [year])
  const data = budget.data

  // Actual spending per category-month and actual income per source-month.
  const actualMaps = useMemo(() => {
    const expense = {}
    const income = {}
    const incomeMonth = Array(12).fill(0)
    actuals.data?.expense.forEach(r => { expense[`${r.category_id}-${r.month}`] = r.amount })
    actuals.data?.income.forEach(r => {
      income[`${r.source_id ?? 'none'}-${r.month}`] = r.amount
      incomeMonth[r.month - 1] += r.amount
    })
    return { expense, income, incomeMonth }
  }, [actuals.data])

  const incomeMap = useMemo(() => {
    const m = {}
    data?.income.forEach(r => { m[`${r.source_id}-${r.month}`] = r.amount })
    return m
  }, [data])

  const expenseMap = useMemo(() => {
    const m = {}
    data?.expense.forEach(r => { m[`${r.category_id}-${r.month}`] = r.amount })
    return m
  }, [data])

  function refreshLocal(key, idField, idVal, month, amount) {
    budget.setData(d => {
      const list = d[key].filter(r => !(r[idField] === idVal && r.month === month))
      list.push({ [idField]: idVal, month, year, amount })
      return { ...d, [key]: list }
    })
  }

  // Both throw on failure so BudgetCell can show the error state.
  async function saveIncome(sourceId, month, amount) {
    try {
      await api.setBudgetIncome({ year, month, source_id: sourceId, amount })
    } catch (e) {
      toast.error(`Couldn't save ${MONTH_NAMES[month - 1]} income: ${e.message}`)
      throw e
    }
    refreshLocal('income', 'source_id', sourceId, month, amount)
  }

  async function saveExpense(categoryId, month, amount) {
    try {
      await api.setBudgetExpense({ year, month, category_id: categoryId, amount })
    } catch (e) {
      toast.error(`Couldn't save ${MONTH_NAMES[month - 1]} budget: ${e.message}`)
      throw e
    }
    refreshLocal('expense', 'category_id', categoryId, month, amount)
  }

  const categoriesByMain = useMemo(() => {
    const groups = { Needs: [], Wants: [], Savings: [] }
    setup?.categories.forEach(c => { (groups[c.main_category] ||= []).push(c) })
    return groups
  }, [setup])

  const monthTotals = useMemo(() => {
    const totals = Array(12).fill(0)
    data?.income.forEach(r => { totals[r.month - 1] += r.amount })
    return totals
  }, [data])

  const error = budget.error || setupError
  const header = <PageHeader title="Budgeting" subtitle={`Plan income and spending per month - ${year}`} period="year" />

  if (!setup || !data || budget.loading) { // also on year change: never show last year's cells under the new year
    return (
      <div>
        {header}
        {error ? <div className="error-box">{error}</div> : <><SkeletonCard lines={4} /><SkeletonCard lines={6} /></>}
      </div>
    )
  }

  return (
    <div>
      {header}
      {error && <div className="error-box">{error}</div>}
      <p className="muted form-hint">Values save automatically when you leave a cell. Shorthand works here too (e.g. <kbd>5jt</kbd>).</p>
      <p className="muted form-hint bva-legend">
        Under each cell: what actually happened.
        <span className="bva-swatch ok" /> under 80% <span className="bva-swatch warn" /> 80-100% <span className="bva-swatch over" /> over budget
      </p>
      <p className="form-hint"><ChevronLink to="/transactions/income">Log received money in Transactions › Income</ChevronLink></p>

      <div className="card table-scroll">
        <table className="budget-table">
          <thead>
            <tr>
              <th>Income</th>
              {MONTHS.map(m => <th key={m}>{m}</th>)}
              <th className="num">Total</th>
              <th className="num">Last Year</th>
              <th className="num">Avg Past Years</th>
            </tr>
          </thead>
          <tbody>
            {setup.income_sources.map(src => {
              const hist = historyStats(data.income_history, src.id)
              const rowTotal = MONTHS.reduce((s, _, i) => s + (incomeMap[`${src.id}-${i + 1}`] || 0), 0)
              return (
                <tr key={src.id}>
                  <td>{src.name}</td>
                  {MONTHS.map((_, i) => {
                    const month = i + 1
                    const planned = incomeMap[`${src.id}-${month}`] || 0
                    const actual = actualMaps.income[`${src.id}-${month}`] || 0
                    return (
                      <td key={month} className={actual ? (actual >= planned ? 'bva-ok' : '') : ''}>
                        <BudgetCell
                          value={incomeMap[`${src.id}-${month}`]}
                          label={`${src.name} income, ${MONTH_NAMES[i]} ${year}`}
                          onSave={v => saveIncome(src.id, month, v)}
                        />
                        <ActualNote kind="income" actual={actual} planned={planned} />
                      </td>
                    )
                  })}
                  <td className="num">{formatRupiah(rowTotal)}</td>
                  <td className="num muted">{formatRupiah(hist.lastYear)}</td>
                  <td className="num muted">{formatRupiah(hist.avg)}</td>
                </tr>
              )
            })}
            <tr className="total-row">
              <td>Total Income</td>
              {monthTotals.map((t, i) => (
                <td key={i}>
                  {formatRupiah(t)}
                  {actualMaps.incomeMonth[i] > 0 && <div className="bva-note">Got {formatRupiah(actualMaps.incomeMonth[i])}</div>}
                </td>
              ))}
              <td className="num">{formatRupiah(monthTotals.reduce((s, t) => s + t, 0))}</td>
              <td></td>
              <td></td>
            </tr>
          </tbody>
        </table>
      </div>

      {Object.entries(categoriesByMain).map(([main, cats]) => (
        <div className="card table-scroll" key={main}>
          <h2>{main}</h2>
          <table className="budget-table">
            <thead>
              <tr>
                <th>Category</th>
                {MONTHS.map(m => <th key={m}>{m}</th>)}
                <th className="num">Total</th>
                <th className="num">Last Year</th>
                <th className="num">Avg Past Years</th>
              </tr>
            </thead>
            <tbody>
              {cats.map(cat => {
                const hist = historyStats(data.expense_history, cat.id)
                const rowTotal = MONTHS.reduce((s, _, i) => s + (expenseMap[`${cat.id}-${i + 1}`] || 0), 0)
                return (
                  <tr key={cat.id}>
                    <td><span className="cat-dot" style={{ background: categoryColor(cat.name) }} />{cat.name}</td>
                    {MONTHS.map((_, i) => {
                      const month = i + 1
                      const planned = expenseMap[`${cat.id}-${month}`] || 0
                      const actual = actualMaps.expense[`${cat.id}-${month}`] || 0
                      const state = budgetState(planned, actual)
                      return (
                        <td key={month} className={state ? `bva-${state}` : ''}>
                          <BudgetCell
                            value={expenseMap[`${cat.id}-${month}`]}
                            label={`${cat.name} budget, ${MONTH_NAMES[i]} ${year}`}
                            onSave={v => saveExpense(cat.id, month, v)}
                          />
                          <ActualNote kind="expense" actual={actual} planned={planned} />
                        </td>
                      )
                    })}
                    <td className="num">{formatRupiah(rowTotal)}</td>
                    <td className="num muted">{formatRupiah(hist.lastYear)}</td>
                    <td className="num muted">{formatRupiah(hist.avg)}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      ))}

      <AlertsCard />
    </div>
  )
}
