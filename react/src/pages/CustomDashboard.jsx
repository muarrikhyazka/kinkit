import { useEffect, useRef, useState } from 'react'
import { api, dataApi, MONTHS } from '../api'
import { useYearMonth } from '../YearContext.jsx'
import { useApi } from '../useApi'
import { Skeleton } from '../Skeleton.jsx'
import BarChart from '../BarChart.jsx'
import LineChart from '../LineChart.jsx'
import PieChart from '../PieChart.jsx'

const STORAGE_KEY = 'kinkit_custom_widgets'          // local cache (instant render / offline)
const MIGRATED_KEY = 'kinkit_custom_widgets_migrated' // set once the local layout went up to the server
const SAVE_DEBOUNCE_MS = 800

const byKind = (assets, kind) => assets.filter(a => (a.kind || 'liquid') === kind).reduce((s, a) => s + (a.value || 0), 0)

// Data sources: each knows how to fetch + shape itself into {labels, datasets}
// for Bar/Line, or {rows:[{category_name,total}]} for Pie. New sources just
// add an entry here - no other wiring needed.
const SOURCES = {
  year_trend: {
    label: 'This Year - Income vs Expense (by month)',
    charts: ['bar', 'line'],
    fetch: ({ year, valueMax }) => api.trend(year, 12, 12, valueMax),
    shape: (d) => ({
      labels: d.map(t => MONTHS[t.month - 1]),
      datasets: [
        { label: 'Income', data: d.map(t => t.income), color: 'var(--success)' },
        { label: 'Expense', data: d.map(t => t.expense), color: 'var(--warning)' },
      ],
    }),
  },
  category_breakdown: {
    label: 'This Year - Spending by Category',
    charts: ['pie', 'bar'],
    fetch: ({ year, valueMax }) => api.reportYear(year, valueMax),
    shape: (d) => ({ rows: d }),
    shapeBar: (d) => ({
      labels: d.map(c => c.category_name),
      datasets: [{ label: 'Total', data: d.map(c => c.total), color: 'var(--primary)' }],
    }),
  },
  years_comparison: {
    label: 'Year-over-Year Income vs Expense',
    charts: ['bar'],
    fetch: ({ valueMax }) => api.years(valueMax),
    shape: (d) => ({
      labels: d.map(y => String(y.year)),
      datasets: [
        { label: 'Income', data: d.map(y => y.income), color: 'var(--success)' },
        { label: 'Expense', data: d.map(y => y.expense), color: 'var(--warning)' },
      ],
    }),
  },
  month_categories: {
    label: 'This Month - Spending by Category',
    charts: ['pie', 'bar'],
    fetch: ({ year, month, valueMax }) => api.report(year, month, valueMax),
    shape: (d) => ({
      rows: (d.expenses_list || []).filter(c => c.realization > 0)
        .map(c => ({ category_name: c.category_name, total: c.realization }))
        .sort((a, b) => b.total - a.total),
    }),
    shapeBar: (d) => {
      const rows = (d.expenses_list || []).filter(c => c.realization > 0).sort((a, b) => b.realization - a.realization)
      return { labels: rows.map(c => c.category_name), datasets: [{ label: 'Spent', data: rows.map(c => c.realization), color: 'var(--warning)' }] }
    },
  },
  month_budget_vs_actual: {
    label: 'This Month - Budget vs Actual by Category',
    charts: ['bar'],
    fetch: ({ year, month, valueMax }) => api.report(year, month, valueMax),
    shape: (d) => {
      const rows = (d.expenses_list || []).filter(c => c.allocation > 0 || c.realization > 0)
      return {
        labels: rows.map(c => c.category_name),
        datasets: [
          { label: 'Budget', data: rows.map(c => c.allocation), color: 'var(--primary)' },
          { label: 'Actual', data: rows.map(c => c.realization), color: 'var(--warning)' },
        ],
      }
    },
  },
  six_month_trend: {
    label: 'Last 6 Months - Income vs Expense',
    charts: ['line', 'bar'],
    fetch: ({ year, month, valueMax }) => api.trend(year, month, 6, valueMax),
    shape: (d) => ({
      labels: d.map(t => `${MONTHS[t.month - 1]} ${String(t.year).slice(2)}`),
      datasets: [
        { label: 'Income', data: d.map(t => t.income), color: 'var(--success)' },
        { label: 'Expense', data: d.map(t => t.expense), color: 'var(--warning)' },
      ],
    }),
  },
  assets_totals: {
    label: 'Assets - Liquid vs Non-liquid',
    charts: ['pie', 'bar'],
    fetch: () => api.assets(),
    shape: (d) => ({
      rows: [
        { category_name: 'Liquid', total: byKind(d.assets || [], 'liquid') },
        { category_name: 'Non-liquid', total: byKind(d.assets || [], 'non_liquid') },
      ].filter(r => r.total > 0),
    }),
    shapeBar: (d) => {
      const assets = [...(d.assets || [])].sort((a, b) => b.value - a.value).slice(0, 12)
      const out = { labels: assets.map(a => a.description), datasets: [{ label: 'Value', data: assets.map(a => a.value), color: 'var(--success)' }] }
      if (d.target?.target_value) {
        out.labels.push('Target')
        out.datasets[0].data.push(d.target.target_value)
      }
      return out
    },
  },
  weekly_this_month: {
    label: 'This Month - Spending by Week',
    charts: ['bar'],
    fetch: ({ year, month, valueMax }) => api.weeklyReport(year, month, valueMax),
    shape: (d) => ({
      labels: d.map(w => `Week ${w.week}`),
      datasets: [{ label: 'Total', data: d.map(w => w.total), color: 'var(--warning)' }],
    }),
  },
}

const validWidgets = list => (Array.isArray(list) ? list.filter(w => w && w.id && SOURCES[w.source]).map(w => ({
  ...w, chart: SOURCES[w.source].charts.includes(w.chart) ? w.chart : SOURCES[w.source].charts[0],
})) : [])

function readLocal() {
  try { return validWidgets(JSON.parse(localStorage.getItem(STORAGE_KEY))) } catch { return [] }
}

function loadWidgets() {
  const saved = readLocal()
  if (saved.length) return saved
  return [
    { id: 'w1', source: 'year_trend', chart: 'bar' },
    { id: 'w2', source: 'category_breakdown', chart: 'pie' },
  ]
}

export default function CustomDashboard() {
  const { year, month, valueMax } = useYearMonth()
  const [widgets, setWidgets] = useState(loadWidgets)
  const [dragIndex, setDragIndex] = useState(null)
  const [syncState, setSyncState] = useState('loading') // loading | synced | saving | offline
  const loaded = useRef(false)     // server layout fetched (or failed) - saving allowed after this
  const skipSave = useRef(false)   // don't echo a just-loaded server layout back

  // Server is the source of truth (same layout on every device). The local
  // copy renders instantly and covers offline; a layout that only exists
  // locally (from before server sync) is migrated up once.
  useEffect(() => {
    let alive = true
    dataApi.dashboardLayout().then(d => {
      if (!alive) return
      let migrated = false
      try { migrated = localStorage.getItem(MIGRATED_KEY) === '1' } catch { /* ignore */ }
      if (Array.isArray(d?.layout)) {
        skipSave.current = true
        setWidgets(validWidgets(d.layout))
        setSyncState('synced')
      } else {
        // nothing on the server yet: push the pre-sync local layout up (once per device)
        const local = readLocal()
        if (local.length && !migrated) dataApi.saveDashboardLayout(local).then(() => setSyncState('synced'), () => setSyncState('offline'))
        else setSyncState('synced')
      }
      try { localStorage.setItem(MIGRATED_KEY, '1') } catch { /* ignore */ }
      loaded.current = true
    }, () => { if (alive) { loaded.current = true; setSyncState('offline') } })
    return () => { alive = false }
  }, [])

  useEffect(() => {
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(widgets)) } catch { /* ignore */ }
    if (!loaded.current) return
    if (skipSave.current) { skipSave.current = false; return }
    setSyncState('saving')
    const t = setTimeout(() => {
      dataApi.saveDashboardLayout(widgets).then(() => setSyncState('synced'), () => setSyncState('offline'))
    }, SAVE_DEBOUNCE_MS)
    return () => clearTimeout(t)
  }, [widgets])

  function addWidget() {
    const id = 'w' + Date.now()
    setWidgets(w => [...w, { id, source: 'year_trend', chart: 'bar' }])
  }
  function removeWidget(id) { setWidgets(w => w.filter(x => x.id !== id)) }
  function updateWidget(id, patch) {
    setWidgets(w => w.map(x => x.id === id ? { ...x, ...patch } : x))
  }
  // keyboard-friendly alternative to drag-and-drop
  function moveWidget(i, d) {
    setWidgets(w => {
      const next = [...w]
      const [moved] = next.splice(i, 1)
      next.splice(i + d, 0, moved)
      return next
    })
  }
  function onDrop(targetIndex) {
    if (dragIndex === null || dragIndex === targetIndex) return
    setWidgets(w => {
      const next = [...w]
      const [moved] = next.splice(dragIndex, 1)
      next.splice(targetIndex, 0, moved)
      return next
    })
    setDragIndex(null)
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>Custom Layout</h2>
          <p className="muted">
            Drag cards to reorder. Pick what data and chart each one shows.{' '}
            <span className="sync-note" aria-live="polite">
              {syncState === 'saving' ? 'Saving...' : syncState === 'offline' ? 'Saved on this device only (offline).' : syncState === 'synced' ? 'Synced across your devices.' : ''}
            </span>
          </p>
        </div>
        <button onClick={addWidget}>+ Add widget</button>
      </div>

      {widgets.map((w, i) => (
        <div
          key={w.id}
          className="card custom-widget"
          draggable
          onDragStart={() => setDragIndex(i)}
          onDragOver={(e) => e.preventDefault()}
          onDrop={() => onDrop(i)}
        >
          <div className="custom-widget-controls">
            <span className="drag-handle" title="Drag to reorder" aria-hidden="true">⠿</span>
            <select aria-label="Data source" value={w.source} onChange={e => {
              const src = e.target.value
              updateWidget(w.id, { source: src, chart: SOURCES[src].charts[0] })
            }}>
              {Object.entries(SOURCES).map(([key, s]) => <option key={key} value={key}>{s.label}</option>)}
            </select>
            <select aria-label="Chart type" value={w.chart} onChange={e => updateWidget(w.id, { chart: e.target.value })}>
              {SOURCES[w.source].charts.map(c => <option key={c} value={c}>{c[0].toUpperCase() + c.slice(1)} chart</option>)}
            </select>
            <button className="icon-btn" onClick={() => moveWidget(i, -1)} disabled={i === 0} aria-label="Move widget up" title="Move up">↑</button>
            <button className="icon-btn" onClick={() => moveWidget(i, 1)} disabled={i === widgets.length - 1} aria-label="Move widget down" title="Move down">↓</button>
            <button className="icon-btn danger" onClick={() => removeWidget(w.id)} title="Remove" aria-label="Remove widget">×</button>
          </div>
          <WidgetBody key={w.source} widget={w} ctx={{ year, month, valueMax }} />
        </div>
      ))}
    </div>
  )
}

function WidgetBody({ widget, ctx }) {
  const src = SOURCES[widget.source]
  const { data, error, loading } = useApi(() => src.fetch(ctx), [widget.source, ctx.year, ctx.month, ctx.valueMax])

  if (error) return <div className="error-box">{error}</div>
  if (!data || loading) return <Skeleton height={240} radius={10} />

  if (widget.chart === 'pie') {
    const { rows } = src.shape(data)
    return <PieChart data={rows} />
  }
  if (widget.chart === 'bar') {
    const shaped = src.shapeBar ? src.shapeBar(data) : src.shape(data)
    return <BarChart labels={shaped.labels} datasets={shaped.datasets} />
  }
  if (widget.chart === 'line') {
    const shaped = src.shape(data)
    return <LineChart labels={shaped.labels} datasets={shaped.datasets} />
  }
  return null
}
