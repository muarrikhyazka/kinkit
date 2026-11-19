import { useEffect, useMemo, useState } from 'react'
import { calendarApi } from './api'
import { useApi } from './useApi'
import Modal from './Modal.jsx'
import { SkeletonCard } from './Skeleton.jsx'
import { IconChevronLeft, IconChevronRight } from './icons.jsx'
import { MonthGrid, DayRows } from './CalendarViews.jsx'
import { addMonths, monthGridDays, monthTitle, dayTitle, toItems, itemTime } from './calendar.js'

/**
 * Month calendar of reminders (the "Calendar" view of the Reminders page).
 * `renderActions(occurrence, reload)` supplies the Done/Snooze buttons so this
 * file doesn't import the page that uses it.
 */
export default function RemindersCalendar({ renderActions, reloadKey }) {
  const [cursor, setCursor] = useState(null)
  const [selected, setSelected] = useState(null)
  const [open, setOpen] = useState(null)
  const first = useApi(() => calendarApi.range('2000-01-01', '2000-01-01', 'reminders'), [])
  const today = first.data?.today
  useEffect(() => { if (today && !cursor) { setCursor(today); setSelected(today) } }, [today, cursor])
  const grid = cursor ? monthGridDays(cursor) : null
  const cal = useApi(() => (grid ? calendarApi.range(grid[0], grid[41], 'reminders') : Promise.resolve(null)), [grid?.[0], reloadKey])
  const items = useMemo(() => (cal.data ? toItems(cal.data, { activities: false }) : []), [cal.data])

  if (!cursor) return <SkeletonCard lines={6} />
  const step = n => setCursor(c => addMonths(c, n))
  return (
    <section className="card" aria-labelledby="rem-cal-h">
      <div className="cal-toolbar">
        <h2 id="rem-cal-h" className="cal-title">{monthTitle(cursor)}</h2>
        <div className="cal-nav">
          <button className="icon-btn" onClick={() => step(-1)} aria-label="Previous month"><IconChevronLeft /></button>
          <button className="secondary" onClick={() => { setCursor(today); setSelected(today) }}>Today</button>
          <button className="icon-btn" onClick={() => step(1)} aria-label="Next month"><IconChevronRight /></button>
        </div>
      </div>
      {cal.error && <div className="error-box">{cal.error}</div>}
      <MonthGrid cursor={cursor} items={items} today={today} selected={selected} onSelect={setSelected} onOpen={setOpen} />
      <h3 style={{ margin: 'var(--s-4) 0 var(--s-2)' }}>{dayTitle(selected)}</h3>
      <DayRows day={selected} items={items} onOpen={setOpen} empty="No reminders on this day." />
      {open && (
        <Modal title={open.title} onClose={() => setOpen(null)}>
          <div className="cal-detail">
            <dl>
              <dt>When</dt><dd>{dayTitle(open.date)}<br /><span className="muted">{itemTime(open, open.date)} · {open.raw.status}</span></dd>
              <dt>Who</dt><dd>{open.raw.assignee_name || 'Whole family'}</dd>
              {open.raw.notes && <><dt>Notes</dt><dd>{open.raw.notes}</dd></>}
            </dl>
            <div className="actions">
              {open.raw.status !== 'done' && open.raw.can_act && renderActions(open.raw, () => { setOpen(null); cal.reload() })}
              <button className="secondary" onClick={() => setOpen(null)}>Close</button>
            </div>
          </div>
        </Modal>
      )}
    </section>
  )
}
