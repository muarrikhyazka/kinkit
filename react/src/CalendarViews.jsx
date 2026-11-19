import { useEffect, useRef } from 'react'
import { IconBell } from './icons.jsx'
import {
  WEEKDAY_SHORT, monthGridDays, itemsOnDay, itemTime, layoutDay, dayShort, parseISO, fmtMin, addDays,
} from './calendar.js'
import './calendar.css'

const colorVar = i => (i.kind === 'reminder' ? 'var(--cal-reminder)' : `var(--cal-${i.color || 'indigo'})`)

/** One event/reminder as a compact chip. */
export function Chip({ item, day, onOpen, showTime = true, style }) {
  return (
    <button type="button" className={`cal-chip ${item.kind} ${item.done ? 'done' : ''}`} style={{ '--c': colorVar(item), ...style }}
      onClick={e => { e.stopPropagation(); onOpen?.(item) }} title={`${item.title} · ${itemTime(item, day)}`}>
      {item.kind === 'reminder' && <IconBell width={10} height={10} aria-hidden="true" />}
      {showTime && !item.allDay && <span className="t">{fmtMin(item.startMin)}</span>}
      <span className="ttl">{item.title}</span>
    </button>
  )
}

/** Month grid (Monday first). `cursor` is any date in the month. */
export function MonthGrid({ cursor, items, today, selected, onSelect, onOpen, maxChips = 3 }) {
  const days = monthGridDays(cursor)
  const month = cursor.slice(0, 7)
  return (
    <div className="cal-month" role="grid" aria-label="Month">
      {WEEKDAY_SHORT.map(d => <div key={d} className="cal-dow" role="columnheader">{d}</div>)}
      {days.map(d => {
        const list = itemsOnDay(items, d)
        const num = Number(d.slice(8))
        return (
          <div key={d} role="gridcell" tabIndex={0} aria-label={`${dayShort(d)}, ${list.length} item${list.length === 1 ? '' : 's'}`}
            className={`cal-cell ${d.slice(0, 7) !== month ? 'out' : ''} ${d === today ? 'today' : ''} ${d === selected ? 'sel' : ''}`}
            onClick={() => onSelect?.(d)} onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onSelect?.(d) } }}>
            <span className="cal-num">{num}</span>
            {list.slice(0, maxChips).map(i => <Chip key={i.key} item={i} day={d} onOpen={onOpen} />)}
            {list.length > maxChips && <span className="cal-more">+{list.length - maxChips} more</span>}
            {list.length > 0 && (
              <span className="cal-dots" aria-hidden="true">
                {list.slice(0, 4).map(i => <span key={i.key} className="cal-dot" style={{ '--c': colorVar(i) }} />)}
              </span>
            )}
          </div>
        )
      })}
    </div>
  )
}

const HH = 44 // px per hour

/** Week (7 days) or day (1 day) view with an hour grid. */
export function TimeGrid({ days, items, today, nowMin, onOpenDay, onOpen }) {
  const bodyRef = useRef(null)
  useEffect(() => { if (bodyRef.current) bodyRef.current.scrollTop = 7 * HH - 8 }, [days.length])
  const style = { '--days': days.length, '--hh': `${HH}px` }
  const perDay = days.map(d => {
    const list = itemsOnDay(items, d)
    return { d, allDay: list.filter(i => i.allDay), timed: list.filter(i => !i.allDay) }
  })
  return (
    <div className="cal-time" style={style}>
      <div className="cal-time-head">
        <div />
        {perDay.map(({ d }) => (
          <div key={d} className={d === today ? 'today' : ''}>
            <button type="button" onClick={() => onOpenDay?.(d)} aria-label={`Open ${dayShort(d)}`}>
              <span>{parseISO(d).toLocaleDateString(undefined, { weekday: 'short' })}</span>
              <span className="dn">{Number(d.slice(8))}</span>
            </button>
          </div>
        ))}
      </div>
      {perDay.some(p => p.allDay.length) && (
        <div className="cal-time-allday">
          <div>all-day</div>
          {perDay.map(({ d, allDay }) => <div key={d}>{allDay.map(i => <Chip key={i.key} item={i} day={d} onOpen={onOpen} />)}</div>)}
        </div>
      )}
      <div className="cal-time-body" ref={bodyRef}>
        <div className="cal-hours">
          {Array.from({ length: 24 }, (_, h) => <div key={h}>{h === 0 ? '' : fmtMin(h * 60)}</div>)}
        </div>
        {perDay.map(({ d, timed }) => (
          <div key={d} className="cal-col" style={{ height: 24 * HH }}>
            {d === today && nowMin != null && <div className="cal-now" style={{ top: nowMin / 60 * HH }} />}
            {layoutDay(timed).map(({ item, lane, lanes }) => (
              <Chip key={item.key} item={item} day={d} onOpen={onOpen} showTime={false}
                style={{
                  top: item.startMin / 60 * HH, height: Math.max((item.endMin - item.startMin) / 60 * HH - 2, 20),
                  left: `calc(${lane / lanes * 100}% + 1px)`, width: `calc(${100 / lanes}% - 3px)`,
                }} />
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}

/** Rows for one day (used under the month grid and in the agenda). */
export function DayRows({ day, items, onOpen, empty = 'Nothing planned.' }) {
  const list = itemsOnDay(items, day)
  if (!list.length) return <p className="cal-empty">{empty}</p>
  return (
    <ul className="cal-daylist">
      {list.map(i => (
        <li key={i.key}>
          <button type="button" className={`cal-row ${i.kind} ${i.done ? 'done' : ''}`} style={{ '--c': colorVar(i) }} onClick={() => onOpen?.(i)}>
            <span className="when">{itemTime(i, day)}</span>
            <span className="what">
              <strong>{i.kind === 'reminder' && <IconBell width={13} height={13} aria-hidden="true" />} {i.title}</strong>
              <small>
                {i.kind === 'reminder' ? `Reminder · ${i.raw.assignee_name || 'Whole family'}` : [i.raw.location, i.raw.attendee_names?.length ? i.raw.attendee_names.join(', ') : 'Whole family'].filter(Boolean).join(' · ')}
              </small>
            </span>
          </button>
        </li>
      ))}
    </ul>
  )
}

/** Next N days with something on them, grouped by date. */
export function Agenda({ from, days, items, today, onOpen }) {
  const dates = Array.from({ length: days }, (_, i) => addDays(from, i)).filter(d => itemsOnDay(items, d).length)
  if (!dates.length) return <p className="cal-empty">Nothing planned in this period.</p>
  return (
    <div className="cal-agenda">
      {dates.map(d => (
        <section key={d}>
          <h3 className={d === today ? 'today' : ''}>{d === today ? 'Today · ' : ''}{dayShort(d)}</h3>
          <DayRows day={d} items={items} onOpen={onOpen} />
        </section>
      ))}
    </div>
  )
}
