import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, calendarApi } from '../api'
import { useApi } from '../useApi'
import { useAuth, canManage } from '../AuthContext.jsx'
import { useToast } from '../Toast.jsx'
import Modal from '../Modal.jsx'
import PageHeader from '../PageHeader.jsx'
import { SkeletonCard } from '../Skeleton.jsx'
import { IconPlus, IconChevronLeft, IconChevronRight, IconBell } from '../icons.jsx'
import { MonthGrid, TimeGrid, DayRows, Agenda } from '../CalendarViews.jsx'
import {
  ACTIVITY_COLORS, WEEKDAY_SHORT, WEEKDAY_NUMS, addDays, addMonths, startOfWeek, monthGridDays, toItems, monthTitle, dayTitle,
  parseISO, toISO, zoned, itemTime,
} from '../calendar.js'
import '../calendar.css'

const VIEW_KEY = 'kinkit_calendar_view'
const VIEWS = [['month', 'Month'], ['week', 'Week'], ['day', 'Day'], ['agenda', 'Agenda']]
const REPEATS = [['none', 'Does not repeat'], ['daily', 'Daily'], ['weekly', 'Weekly'], ['monthly', 'Monthly'], ['yearly', 'Yearly']]
const UNIT = { daily: 'day(s)', weekly: 'week(s)', monthly: 'month(s)', yearly: 'year(s)' }

function rangeFor(view, cursor) {
  if (view === 'month') { const g = monthGridDays(cursor); return [g[0], g[41]] }
  if (view === 'week') { const s = startOfWeek(cursor); return [s, addDays(s, 6)] }
  if (view === 'day') return [cursor, cursor]
  return [cursor, addDays(cursor, 29)]
}

function titleFor(view, cursor) {
  if (view === 'month') return monthTitle(cursor)
  if (view === 'day') return dayTitle(cursor)
  const [a, b] = rangeFor(view, cursor)
  const f = d => parseISO(d).toLocaleDateString(undefined, { day: 'numeric', month: 'short' })
  return `${f(a)} – ${f(b)} ${parseISO(b).getFullYear()}`
}

/** Calendar: family activities (Google-Calendar style) + the reminders overlay. */
export default function Calendar() {
  const { user } = useAuth()
  const toast = useToast()
  const manager = canManage(user)
  const [view, setView] = useState(() => { try { return localStorage.getItem(VIEW_KEY) || 'month' } catch { return 'month' } })
  const [cursor, setCursor] = useState(null)
  const [selected, setSelected] = useState(null)
  const [showActs, setShowActs] = useState(true)
  const [showRems, setShowRems] = useState(true)
  const [open, setOpen] = useState(null)      // item shown in the detail modal
  const [editing, setEditing] = useState(null) // null | {row?, date?}
  const family = useApi(() => api.family(), [])

  // first load: ask the server for "today" in the household's time zone
  const first = useApi(() => calendarApi.range('2000-01-01', '2000-01-01', 'activities'), [])
  const today = first.data?.today
  useEffect(() => { if (today && !cursor) { setCursor(today); setSelected(today) } }, [today, cursor])
  useEffect(() => { try { localStorage.setItem(VIEW_KEY, view) } catch { /* ignore */ } }, [view])

  const [from, to] = cursor ? rangeFor(view, cursor) : ['', '']
  const cal = useApi(() => (cursor ? calendarApi.range(from, to) : Promise.resolve(null)), [from, to, cursor])
  const items = useMemo(() => (cal.data ? toItems(cal.data, { activities: showActs, reminders: showRems }) : []), [cal.data, showActs, showRems])
  const tz = cal.data?.timezone || first.data?.timezone
  const nowMin = useMemo(() => (tz ? zoned(new Date().toISOString(), tz).minutes : null), [tz, cal.data])

  const step = n => {
    if (view === 'month') setCursor(c => addMonths(c, n))
    else if (view === 'week') setCursor(c => addDays(c, 7 * n))
    else if (view === 'day') setCursor(c => addDays(c, n))
    else setCursor(c => addDays(c, 30 * n))
  }
  const goToday = () => { setCursor(today); setSelected(today) }
  const openDay = d => { setCursor(d); setSelected(d); setView('day') }

  if (!cursor) return <div className="cal-page"><PageHeader title="Calendar" /><SkeletonCard lines={6} /></div>
  const members = family.data?.members || []
  const weekDays = Array.from({ length: 7 }, (_, i) => addDays(startOfWeek(cursor), i))

  return (
    <div className="cal-page">
      <PageHeader
        title="Calendar"
        subtitle="Family activities, plus reminders, in one place."
        actions={manager && <button onClick={() => setEditing({ date: selected || cursor })}><IconPlus width={16} height={16} aria-hidden="true" /> New activity</button>}
      />
      <div className="cal-toolbar">
        <div className="cal-nav">
          <button className="icon-btn" onClick={() => step(-1)} aria-label="Previous"><IconChevronLeft /></button>
          <button className="secondary" onClick={goToday}>Today</button>
          <button className="icon-btn" onClick={() => step(1)} aria-label="Next"><IconChevronRight /></button>
        </div>
        <h2 className="cal-title" aria-live="polite">{titleFor(view, cursor)}</h2>
        <div className="cal-seg" role="group" aria-label="View">
          {VIEWS.map(([id, label]) => <button key={id} type="button" aria-pressed={view === id} onClick={() => setView(id)}>{label}</button>)}
        </div>
      </div>
      <div className="cal-layers">
        <label><input type="checkbox" checked={showActs} onChange={e => setShowActs(e.target.checked)} /> Activities</label>
        <label><input type="checkbox" checked={showRems} onChange={e => setShowRems(e.target.checked)} /> <IconBell width={13} height={13} aria-hidden="true" /> Reminders</label>
      </div>
      {cal.error && <div className="error-box">{cal.error}</div>}
      {!cal.data && !cal.error ? <SkeletonCard lines={8} /> : (
        <>
          {view === 'month' && (
            <>
              <MonthGrid cursor={cursor} items={items} today={today} selected={selected} onSelect={setSelected} onOpen={setOpen} />
              <section className="card" style={{ marginTop: 'var(--s-4)' }}>
                <h2>{dayTitle(selected)}</h2>
                <DayRows day={selected} items={items} onOpen={setOpen} />
                <p style={{ marginTop: 'var(--s-3)', display: 'flex', gap: 'var(--s-2)', flexWrap: 'wrap' }}>
                  <button className="secondary" onClick={() => openDay(selected)}>Open day view</button>
                  {manager && <button className="secondary" onClick={() => setEditing({ date: selected })}><IconPlus width={14} height={14} aria-hidden="true" /> Add on this day</button>}
                </p>
              </section>
            </>
          )}
          {view === 'week' && <TimeGrid days={weekDays} items={items} today={today} nowMin={nowMin} onOpenDay={openDay} onOpen={setOpen} />}
          {view === 'day' && <TimeGrid days={[cursor]} items={items} today={today} nowMin={nowMin} onOpen={setOpen} />}
          {view === 'agenda' && <div className="card"><Agenda from={cursor} days={30} items={items} today={today} onOpen={setOpen} /></div>}
        </>
      )}

      {open && (
        <EventDetail item={open} manager={manager} onClose={() => setOpen(null)}
          onEdit={() => { setEditing({ row: open.raw }); setOpen(null) }}
          onDeleted={() => { setOpen(null); cal.reload() }} toast={toast} />
      )}
      {editing && (
        <ActivityForm initial={editing.row} date={editing.date} members={members} onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); cal.reload() }} toast={toast} />
      )}
    </div>
  )
}

function EventDetail({ item, manager, onClose, onEdit, onDeleted, toast }) {
  const a = item.raw
  const isAct = item.kind === 'activity'
  async function del(onlyThis) {
    const msg = onlyThis ? `Remove only the ${a.occurs_on} occurrence of "${a.title}"?` : `Delete "${a.title}"${a.repeating ? ' and all its repeats' : ''}?`
    if (!window.confirm(msg)) return
    try {
      await calendarApi.remove(a.id, onlyThis ? a.occurs_on : undefined)
      toast.success('Deleted')
      onDeleted()
    } catch (e) { toast.error(e.message) }
  }
  return (
    <Modal title={item.title} onClose={onClose}>
      <div className="cal-detail">
        {isAct ? (
          <dl>
            <dt>When</dt>
            <dd>{a.occurs_on === a.occurs_until ? dayTitle(a.occurs_on) : `${dayTitle(a.occurs_on)} → ${dayTitle(a.occurs_until)}`}<br /><span className="muted">{itemTime(item, a.occurs_on)}</span></dd>
            {a.repeating && <><dt>Repeats</dt><dd>{describeRepeat(a)}</dd></>}
            {a.location && <><dt>Where</dt><dd>{a.location}</dd></>}
            <dt>Who</dt><dd>{a.attendee_names?.length ? a.attendee_names.join(', ') : 'Whole family'}</dd>
            {a.notes && <><dt>Notes</dt><dd style={{ whiteSpace: 'pre-wrap' }}>{a.notes}</dd></>}
          </dl>
        ) : (
          <dl>
            <dt>When</dt><dd>{dayTitle(item.date)}<br /><span className="muted">{itemTime(item, item.date)} · {a.status}</span></dd>
            <dt>Who</dt><dd>{a.assignee_name || 'Whole family'}</dd>
            {a.notes && <><dt>Notes</dt><dd>{a.notes}</dd></>}
          </dl>
        )}
        <div className="actions">
          {isAct && manager && <button onClick={onEdit}>Edit</button>}
          {isAct && manager && a.repeating && <button className="secondary" onClick={() => del(true)}>Delete this one</button>}
          {isAct && manager && <button className="danger" onClick={() => del(false)}>{a.repeating ? 'Delete series' : 'Delete'}</button>}
          {!isAct && <Link className="button-link secondary" to="/reminders">Open reminders</Link>}
          <button className="secondary" onClick={onClose}>Close</button>
        </div>
      </div>
    </Modal>
  )
}

function describeRepeat(a) {
  const n = a.interval > 1 ? `every ${a.interval} ` : 'every '
  const unit = { daily: 'day', weekly: 'week', monthly: 'month', yearly: 'year' }[a.repeat]
  let s = a.interval > 1 ? `${n}${unit}s` : `${n}${unit}`
  if (a.repeat === 'weekly' && a.weekdays?.length) s += ` on ${WEEKDAY_NUMS.filter(d => a.weekdays.includes(d)).map(d => WEEKDAY_SHORT[WEEKDAY_NUMS.indexOf(d)]).join(', ')}`
  if (a.until_date) s += `, until ${a.until_date}`
  return s
}

function ActivityForm({ initial, date, members, onClose, onSaved, toast }) {
  const edit = !!initial
  const d0 = initial?.start_date || date
  const [f, setF] = useState({
    title: initial?.title || '', notes: initial?.notes || '', location: initial?.location || '', color: initial?.color || 'indigo',
    all_day: initial?.all_day || false, start_date: d0, start_time: initial?.start_time || '09:00',
    end_date: initial?.end_date || d0, end_time: initial?.end_time || '10:00',
    repeat: initial?.repeat || 'none', interval: initial?.interval || 1, weekdays: initial?.weekdays || [],
    until_date: initial?.until_date || '', attendee_ids: initial?.attendee_ids || [],
  })
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const set = (k, v) => setF(s => ({ ...s, [k]: v }))
  const toggle = (k, v) => setF(s => ({ ...s, [k]: s[k].includes(v) ? s[k].filter(x => x !== v) : [...s[k], v] }))

  async function submit(e) {
    e.preventDefault()
    setBusy(true); setErr('')
    const body = { ...f, title: f.title.trim(), interval: Number(f.interval) || 1, until_date: f.repeat === 'none' ? '' : f.until_date }
    if (body.all_day) { delete body.start_time; delete body.end_time }
    if (body.repeat === 'weekly' && body.weekdays.length === 0) body.weekdays = [parseISO(f.start_date).getDay()]
    try {
      if (edit) await calendarApi.update(initial.id, body)
      else await calendarApi.create(body)
      toast.success(edit ? 'Activity updated' : 'Activity added')
      onSaved()
    } catch (e2) { setErr(e2.message) } finally { setBusy(false) }
  }

  return (
    <Modal title={edit ? 'Edit activity' : 'New activity'} onClose={onClose}>
      <form className="cal-form" onSubmit={submit}>
        <label>Title<input data-autofocus value={f.title} onChange={e => set('title', e.target.value)} maxLength={120} required placeholder="e.g. Football practice" /></label>
        <label className="check"><input type="checkbox" checked={f.all_day} onChange={e => set('all_day', e.target.checked)} /> All day</label>
        <div className="row">
          <label>Starts<input type="date" value={f.start_date} required onChange={e => { set('start_date', e.target.value); if (f.end_date < e.target.value) set('end_date', e.target.value) }} /></label>
          {!f.all_day && <label>At<input type="time" value={f.start_time} onChange={e => set('start_time', e.target.value)} required /></label>}
        </div>
        <div className="row">
          <label>Ends<input type="date" value={f.end_date} min={f.start_date} required onChange={e => set('end_date', e.target.value)} /></label>
          {!f.all_day && <label>At<input type="time" value={f.end_time} onChange={e => set('end_time', e.target.value)} required /></label>}
        </div>
        <div className="row">
          <label>Repeat
            <select value={f.repeat} onChange={e => set('repeat', e.target.value)}>
              {REPEATS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
            </select>
          </label>
          {f.repeat !== 'none' && <label>Every<span style={{ display: 'flex', gap: '.4rem', alignItems: 'center' }}>
            <input type="number" min="1" max="99" value={f.interval} onChange={e => set('interval', e.target.value)} style={{ width: '4.5rem' }} /> {UNIT[f.repeat]}</span></label>}
        </div>
        {f.repeat === 'weekly' && (
          <fieldset aria-label="Repeat on">
            {WEEKDAY_NUMS.map((n, i) => (
              <button type="button" key={n} className={`chip ${f.weekdays.includes(n) ? 'active' : ''}`} aria-pressed={f.weekdays.includes(n)} onClick={() => toggle('weekdays', n)}>{WEEKDAY_SHORT[i]}</button>
            ))}
          </fieldset>
        )}
        {f.repeat !== 'none' && <label>Until (optional)<input type="date" value={f.until_date} min={f.start_date} onChange={e => set('until_date', e.target.value)} /></label>}
        <fieldset aria-label="Who">
          <legend className="muted" style={{ width: '100%', fontSize: 'var(--fs-sm)' }}>Who is it for? (none selected = whole family)</legend>
          {members.map(m => (
            <button type="button" key={m.id} className={`chip ${f.attendee_ids.includes(m.id) ? 'active' : ''}`} aria-pressed={f.attendee_ids.includes(m.id)} onClick={() => toggle('attendee_ids', m.id)}>{m.display_name || m.email}</button>
          ))}
        </fieldset>
        <label>Location<input value={f.location} onChange={e => set('location', e.target.value)} maxLength={200} /></label>
        <label>Notes<textarea rows={2} value={f.notes} onChange={e => set('notes', e.target.value)} maxLength={2000} /></label>
        <div className="cal-swatches" role="group" aria-label="Color">
          {ACTIVITY_COLORS.map(c => <button type="button" key={c} className="cal-swatch" style={{ '--c': `var(--cal-${c})` }} aria-pressed={f.color === c} aria-label={c} onClick={() => set('color', c)} />)}
        </div>
        {err && <div className="error-box">{err}</div>}
        <div className="actions" style={{ display: 'flex', gap: 'var(--s-2)', justifyContent: 'flex-end' }}>
          <button type="button" className="secondary" onClick={onClose}>Cancel</button>
          <button type="submit" disabled={busy || !f.title.trim()}>{busy ? 'Saving…' : 'Save'}</button>
        </div>
      </form>
    </Modal>
  )
}
