import { useEffect, useRef, useState } from 'react'
import { MONTHS, MONTH_NAMES } from './api'
import { useYearMonth, currentUTCPeriod } from './YearContext.jsx'
import { IconChevronLeft, IconChevronRight } from './icons.jsx'

/**
 * Page title row with the period controls relevant to that page.
 *   <PageHeader title="Spending" subtitle="12 transactions" period="month" valueFilter actions={<button/>} />
 * period: 'month' (‹ Nov 2026 ›) | 'year' (‹ 2026 ›) | null/undefined (none)
 * valueFilter: show the "Hide large transactions" filter
 */
export default function PageHeader({ title, subtitle, period, valueFilter = false, actions }) {
  return (
    <div className="page-header">
      <div>
        <h1>{title}</h1>
        {subtitle && <p className="muted">{subtitle}</p>}
      </div>
      {(period || valueFilter || actions) && (
        <div className="page-header-controls">
          {period && <PeriodControl mode={period} />}
          {valueFilter && <ValueFilter />}
          {actions}
        </div>
      )}
    </div>
  )
}

/** ‹ Nov 2026 › stepper; clicking the label opens a month/year picker. */
export function PeriodControl({ mode = 'month' }) {
  const { year, setYear, month, setMonth } = useYearMonth()
  const [open, setOpen] = useState(false)
  const [pickYear, setPickYear] = useState(year)
  const wrapRef = useRef(null)
  const labelRef = useRef(null)
  const unit = mode === 'year' ? 'year' : 'month'

  useEffect(() => {
    if (!open) return
    const onDown = e => { if (!wrapRef.current?.contains(e.target)) setOpen(false) }
    const onKey = e => { if (e.key === 'Escape') { setOpen(false); labelRef.current?.focus() } }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => { document.removeEventListener('mousedown', onDown); document.removeEventListener('keydown', onKey) }
  }, [open])

  function step(d) {
    if (mode === 'year') { setYear(y => y + d); return }
    let m = month + d
    let y = year
    if (m < 1) { m = 12; y-- }
    if (m > 12) { m = 1; y++ }
    setMonth(m)
    setYear(y)
  }

  function choose(y, m) {
    setYear(y)
    if (m) setMonth(m)
    setOpen(false)
    labelRef.current?.focus()
  }

  const now = currentUTCPeriod()
  const label = mode === 'year' ? String(year) : `${MONTHS[month - 1]} ${year}`
  const fullLabel = mode === 'year' ? `Year ${year}` : `${MONTH_NAMES[month - 1]} ${year}`
  const yearBlock = Array.from({ length: 12 }, (_, i) => pickYear - 7 + i)

  return (
    <div className="period-control" ref={wrapRef}>
      <button className="period-step" onClick={() => step(-1)} aria-label={`Previous ${unit}`}><IconChevronLeft width={16} height={16} /></button>
      <button
        ref={labelRef}
        className="period-label"
        onClick={() => { setPickYear(year); setOpen(o => !o) }}
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-label={`${fullLabel} - choose ${unit}`}
      >
        {label}
      </button>
      <button className="period-step" onClick={() => step(1)} aria-label={`Next ${unit}`}><IconChevronRight width={16} height={16} /></button>

      {open && (
        <div className="period-picker" role="dialog" aria-label={`Choose ${unit}`}>
          <div className="period-picker-head">
            <button className="period-step" onClick={() => setPickYear(y => y - (mode === 'year' ? 12 : 1))} aria-label={mode === 'year' ? 'Earlier years' : 'Previous year'}>
              <IconChevronLeft width={14} height={14} />
            </button>
            <span>{mode === 'year' ? `${yearBlock[0]} - ${yearBlock[11]}` : pickYear}</span>
            <button className="period-step" onClick={() => setPickYear(y => y + (mode === 'year' ? 12 : 1))} aria-label={mode === 'year' ? 'Later years' : 'Next year'}>
              <IconChevronRight width={14} height={14} />
            </button>
          </div>
          <div className="period-picker-grid">
            {mode === 'year'
              ? yearBlock.map(y => (
                <button key={y} className={y === year ? 'active' : ''} aria-pressed={y === year} onClick={() => choose(y)}>{y}</button>
              ))
              : MONTHS.map((m, i) => {
                const active = pickYear === year && i + 1 === month
                return (
                  <button key={m} className={active ? 'active' : ''} aria-pressed={active} aria-label={`${MONTH_NAMES[i]} ${pickYear}`} onClick={() => choose(pickYear, i + 1)}>{m}</button>
                )
              })}
          </div>
          <button className="link-btn period-today" onClick={() => choose(now.year, mode === 'year' ? null : now.month)}>
            {mode === 'year' ? 'This year' : 'This month'}
          </button>
        </div>
      )}
    </div>
  )
}

const VALUE_TIP = 'Hide large transactions: only include transactions up to 10 juta / 100 juta'

/** "Hide large transactions" filter (value_max). */
export function ValueFilter() {
  const { valueMax, setValueMax } = useYearMonth()
  return (
    <label className={`value-filter ${valueMax !== 'all' ? 'active' : ''}`} title={VALUE_TIP}>
      <span>Hide large</span>
      <select value={valueMax} onChange={e => setValueMax(e.target.value)} aria-label={VALUE_TIP}>
        <option value="all">Off - show all</option>
        <option value="10jt">Only up to 10 juta</option>
        <option value="100jt">Only up to 100 juta</option>
      </select>
    </label>
  )
}
