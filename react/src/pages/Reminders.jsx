import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, remindersApi, todayISO } from '../api'
import { useApi } from '../useApi'
import { useAuth, canManage } from '../AuthContext.jsx'
import { useToast, useHiddenIds } from '../Toast.jsx'
import Modal from '../Modal.jsx'
import PageHeader from '../PageHeader.jsx'
import { SkeletonCard } from '../Skeleton.jsx'
import {
  IconEdit, IconClose, IconPlus, IconCopy, IconCheck, IconClock, IconCalendar, IconRepeat, IconPause, IconPlay,
  IconUser, IconChat, IconBell, CategoryIcon,
} from '../icons.jsx'
import RemindersCalendar from '../RemindersCalendar.jsx'
import '../calendar.css'
import '../reminders.css'
import { ChevronLink } from '../ChevronLink.jsx'

// ---------- helpers (shared with Today.jsx) ----------

// Household categories first. Ids match the backend (reminderCategoryIDs).
export const REMINDER_CATEGORIES = [
  { id: 'cleaning', label: 'Cleaning' },
  { id: 'laundry', label: 'Laundry' },
  { id: 'cooking', label: 'Kitchen & meals' },
  { id: 'groceries', label: 'Groceries' },
  { id: 'home_maintenance', label: 'Home maintenance' },
  { id: 'vehicle', label: 'Vehicle' },
  { id: 'self_care', label: 'Self-care' },
  { id: 'chores', label: 'Chores' },
  { id: 'bill', label: 'Bills' },
  { id: 'medicine', label: 'Medicine' },
  { id: 'school', label: 'School' },
  { id: 'birthday', label: 'Birthdays' },
  { id: 'other', label: 'Other' },
]
export const categoryLabel = id => REMINDER_CATEGORIES.find(c => c.id === id)?.label || 'Other'
const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

/** Local YYYY-MM-DD for a Date. */
const localISO = d => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`

/** Next local date (today or later) whose day of month is `day` (clamped to short months). */
function nextDayOfMonth(day) {
  const now = new Date()
  const clamp = (y, m) => new Date(y, m, Math.min(day, new Date(y, m + 1, 0).getDate()))
  let d = clamp(now.getFullYear(), now.getMonth())
  if (localISO(d) < localISO(now)) d = clamp(now.getFullYear(), now.getMonth() + 1)
  return localISO(d)
}

/**
 * One-click household templates. Each opens the create form prefilled.
 * kind/weekdays/day_of_month/interval use the same fields as the API.
 */
export const REMINDER_TEMPLATES = [
  { key: 'sheets', title: 'Change bed sheets & towels', category: 'laundry', kind: 'weekly', interval: 2, weekdays: [5], time_of_day: '10:00' },
  { key: 'prayer_mats', title: 'Wash prayer mats & sarung', category: 'laundry', kind: 'weekly', interval: 2, weekdays: [5], time_of_day: '09:00' },
  { key: 'trash', title: 'Take out the trash', category: 'cleaning', kind: 'weekly', weekdays: [2, 5], time_of_day: '19:00' },
  { key: 'plants', title: 'Water the plants', category: 'home_maintenance', kind: 'daily', time_of_day: '07:00' },
  { key: 'bathroom', title: 'Clean the bathroom', category: 'cleaning', kind: 'weekly', weekdays: [6], time_of_day: '09:00' },
  { key: 'meals', title: "Plan next week's meals", category: 'cooking', kind: 'weekly', weekdays: [4], time_of_day: '20:30' },
  { key: 'groceries', title: 'Weekly grocery run', category: 'groceries', kind: 'weekly', weekdays: [6], time_of_day: '08:00' },
  { key: 'tyres', title: 'Check tyre pressure', category: 'vehicle', kind: 'weekly', interval: 2, weekdays: [5], time_of_day: '11:00' },
  { key: 'oil', title: 'Change motorbike oil', category: 'vehicle', kind: 'monthly', interval: 3, day_of_month: 1, time_of_day: '09:00' },
  { key: 'water_tank', title: 'Clean the water tank', category: 'home_maintenance', kind: 'monthly', interval: 3, time_of_day: '09:00' },
  { key: 'ac', title: 'Service the AC', category: 'home_maintenance', kind: 'monthly', interval: 6, time_of_day: '10:00' },
  { key: 'self_care', title: 'Self-care Saturday', category: 'self_care', kind: 'weekly', weekdays: [6], time_of_day: '08:00' },
  { key: 'electricity', title: 'Pay electricity bill', category: 'bill', kind: 'monthly', day_of_month: 20, time_of_day: '09:00' },
]

/** "Today 08:00", "Tomorrow 07:30", "Mon 12 Dec 09:00" in the household time zone. */
export function formatDue(iso, timeZone) {
  if (!iso) return ''
  const d = new Date(iso)
  const opts = { timeZone: timeZone || undefined }
  const day = d.toLocaleDateString('en-CA', opts) // YYYY-MM-DD
  const now = new Date()
  const today = now.toLocaleDateString('en-CA', opts)
  const tomorrow = new Date(now.getTime() + 86400000).toLocaleDateString('en-CA', opts)
  const time = d.toLocaleTimeString([], { ...opts, hour: '2-digit', minute: '2-digit', hour12: false })
  if (day === today) return `Today ${time}`
  if (day === tomorrow) return `Tomorrow ${time}`
  return `${d.toLocaleDateString([], { ...opts, weekday: 'short', day: 'numeric', month: 'short' })} ${time}`
}

/** Done / snooze buttons for one reminder occurrence. */
export function OccurrenceActions({ occurrenceId, title, onChanged }) {
  const toast = useToast()
  const [busy, setBusy] = useState(false)
  async function run(fn, msg) {
    setBusy(true)
    try { await fn(); toast.success(msg); onChanged && onChanged() } catch (err) { toast.error(err.message) } finally { setBusy(false) }
  }
  return (
    <span className="occ-actions">
      <button className="secondary small" disabled={busy} onClick={() => run(() => remindersApi.done(occurrenceId), `Done: ${title}`)} aria-label={`Mark "${title}" done`}><IconCheck width={14} height={14} aria-hidden="true" /> Done</button>
      <button className="secondary small" disabled={busy} onClick={() => run(() => remindersApi.snooze(occurrenceId, { minutes: 60 }), 'Snoozed for 1 hour')} aria-label={`Snooze "${title}" for one hour`}><IconClock width={14} height={14} aria-hidden="true" /> 1h</button>
      <button className="secondary small" disabled={busy} onClick={() => run(() => remindersApi.snooze(occurrenceId, { until: 'tomorrow' }), 'Snoozed until tomorrow')} aria-label={`Snooze "${title}" until tomorrow`}>Tomorrow</button>
    </span>
  )
}

function emptyForm() {
  return {
    title: '', notes: '', category: 'cleaning', assignee_id: '', kind: 'weekly', interval: 1, time_of_day: '08:00',
    weekdays: [new Date().getDay()], day_of_month: new Date().getDate(), start_date: todayISO(), end_date: '',
    escalate_to: '', escalate_after_minutes: 30, active: true,
  }
}

function fromRow(r) {
  return {
    ...emptyForm(),
    title: r.title, notes: r.notes || '', category: r.category, assignee_id: r.assignee_id || '',
    kind: r.kind, interval: r.interval || 1, time_of_day: r.time_of_day, weekdays: r.weekdays?.length ? r.weekdays : [1],
    day_of_month: r.day_of_month || 1, start_date: r.start_date, end_date: r.end_date || '',
    escalate_to: r.escalate_to || '', escalate_after_minutes: r.escalate_after_minutes || 30, active: r.active,
  }
}

function fromTemplate(t) {
  const f = emptyForm()
  const today = localISO(new Date())
  return {
    ...f,
    title: t.title, category: t.category, kind: t.kind, interval: t.interval || 1, time_of_day: t.time_of_day,
    weekdays: t.weekdays || f.weekdays,
    day_of_month: t.day_of_month || new Date().getDate(),
    // a fixed day of month starts on its next occurrence so "every 3 months on the 1st" begins on the coming 1st
    start_date: t.kind === 'monthly' && t.day_of_month ? nextDayOfMonth(t.day_of_month) : today,
  }
}

/** Friendly schedule text, mirroring the backend's `description`. */
export function previewSchedule(f) {
  const at = `at ${f.time_of_day}`
  const n = Number(f.interval) || 1
  switch (f.kind) {
    case 'once': return `Once on ${f.start_date} ${at}`
    case 'daily': return `Every day ${at}`
    case 'weekly': {
      const days = [...f.weekdays].sort((a, b) => a - b).map(d => WEEKDAYS[d])
      if (!days.length) return 'Pick at least one day'
      const list = days.length > 1 ? `${days.slice(0, -1).join(', ')} and ${days[days.length - 1]}` : days[0]
      return n > 1 ? `Every ${n} weeks on ${list} ${at}` : `Every ${list} ${at}`
    }
    case 'monthly': {
      const every = n > 1 ? `Every ${n} months` : 'Every month'
      return `${every} on day ${f.day_of_month} ${at}${Number(f.day_of_month) > 28 ? ' (last day in shorter months)' : ''}`
    }
    case 'yearly': return `Every year on ${f.start_date.slice(5)} ${at}`
    default: return ''
  }
}

function ReminderModal({ initial, template, members, chatHint, onClose, onSaved }) {
  const toast = useToast()
  const [f, setF] = useState(initial ? fromRow(initial) : template ? fromTemplate(template) : emptyForm())
  const [saving, setSaving] = useState(false)
  const set = (k, v) => setF(x => ({ ...x, [k]: v }))
  const toggleDay = d => set('weekdays', f.weekdays.includes(d) ? f.weekdays.filter(x => x !== d) : [...f.weekdays, d])
  const repeats = f.kind === 'weekly' || f.kind === 'monthly'

  async function save(e) {
    e.preventDefault()
    if (f.kind === 'weekly' && f.weekdays.length === 0) { toast.error('Pick at least one day'); return }
    const payload = {
      title: f.title.trim(), notes: f.notes, category: f.category,
      assignee_id: f.assignee_id ? Number(f.assignee_id) : null,
      kind: f.kind, time_of_day: f.time_of_day,
      interval: repeats ? Math.max(1, Math.min(52, Number(f.interval) || 1)) : 1,
      weekdays: f.kind === 'weekly' ? f.weekdays : [],
      day_of_month: f.kind === 'monthly' ? Number(f.day_of_month) : null,
      start_date: f.start_date, end_date: f.end_date || null,
      escalate_to: f.escalate_to ? Number(f.escalate_to) : null,
      escalate_after_minutes: f.escalate_to ? Number(f.escalate_after_minutes) : null,
      active: f.active,
    }
    setSaving(true)
    try {
      if (initial) await remindersApi.update(initial.id, payload)
      else await remindersApi.create(payload)
      toast.success(initial ? 'Reminder updated' : 'Reminder added')
      onSaved()
    } catch (err) { toast.error(err.message) } finally { setSaving(false) }
  }

  return (
    <Modal title={initial ? 'Edit reminder' : template ? `New reminder: ${template.title}` : 'New reminder'} onClose={onClose}>
      <form className="stacked-form" onSubmit={save}>
        <label>What
          <input data-autofocus value={f.title} onChange={e => set('title', e.target.value)} placeholder="e.g. Change bed sheets" required maxLength={200} />
        </label>
        <div className="form-row">
          <label>Category
            <select value={f.category} onChange={e => set('category', e.target.value)}>
              {REMINDER_CATEGORIES.map(c => <option key={c.id} value={c.id}>{c.label}</option>)}
            </select>
          </label>
          <label>Who is it for
            <select value={f.assignee_id} onChange={e => set('assignee_id', e.target.value)}>
              <option value="">Whole family</option>
              {members.map(m => <option key={m.id} value={m.id}>{m.display_name || m.email}</option>)}
            </select>
          </label>
        </div>
        <div className="form-row">
          <label>Repeat
            <select value={f.kind} onChange={e => set('kind', e.target.value)}>
              <option value="once">Once</option>
              <option value="daily">Every day</option>
              <option value="weekly">Weekly</option>
              <option value="monthly">Monthly</option>
              <option value="yearly">Every year</option>
            </select>
          </label>
          <label>Time
            <input type="time" value={f.time_of_day} onChange={e => set('time_of_day', e.target.value)} required />
          </label>
        </div>
        {repeats && (
          <div className="rem-interval">
            <label htmlFor="rem-interval">Repeat every</label>
            <input id="rem-interval" type="number" min={1} max={52} inputMode="numeric" value={f.interval}
              onChange={e => set('interval', e.target.value)} aria-describedby="rem-interval-unit" />
            <span id="rem-interval-unit">{f.kind === 'weekly' ? (Number(f.interval) === 1 ? 'week' : 'weeks') : (Number(f.interval) === 1 ? 'month' : 'months')}</span>
          </div>
        )}
        {f.kind === 'weekly' && (
          <div className="weekday-picker" role="group" aria-label="Days of the week">
            {WEEKDAYS.map((d, i) => (
              <button type="button" key={d} className={`chip ${f.weekdays.includes(i) ? 'active' : ''}`} aria-pressed={f.weekdays.includes(i)} onClick={() => toggleDay(i)}>{d}</button>
            ))}
          </div>
        )}
        {f.kind === 'monthly' && (
          <label>Day of month
            <input type="number" min={1} max={31} value={f.day_of_month} onChange={e => set('day_of_month', e.target.value)} />
          </label>
        )}
        <div className="form-row">
          <label>{f.kind === 'once' || f.kind === 'yearly' ? 'Date' : 'Starts'}
            <input type="date" value={f.start_date} onChange={e => set('start_date', e.target.value)} required />
          </label>
          {f.kind !== 'once' && (
            <label>Ends (optional)
              <input type="date" value={f.end_date} onChange={e => set('end_date', e.target.value)} />
            </label>
          )}
        </div>
        <p className="schedule-preview"><IconCalendar width={15} height={15} aria-hidden="true" /> {previewSchedule(f)}</p>
        {repeats && Number(f.interval) > 1 && (
          <p className="rem-form-hint">Counted from the {f.kind === 'weekly' ? 'week' : 'month'} of the start date.</p>
        )}
        <div className="form-row">
          <label>If nobody marks it done, tell
            <select value={f.escalate_to} onChange={e => set('escalate_to', e.target.value)}>
              <option value="">Nobody</option>
              {members.map(m => <option key={m.id} value={m.id}>{m.display_name || m.email}</option>)}
            </select>
          </label>
          {f.escalate_to && (
            <label>after (minutes)
              <input type="number" min={5} max={1440} value={f.escalate_after_minutes} onChange={e => set('escalate_after_minutes', e.target.value)} />
            </label>
          )}
        </div>
        <label>Notes (optional)
          <textarea rows={2} value={f.notes} onChange={e => set('notes', e.target.value)} maxLength={2000} />
        </label>
        <p className="rem-form-hint">{chatHint}</p>
        <div className="modal-actions">
          <button type="button" className="secondary" onClick={onClose}>Cancel</button>
          <button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</button>
        </div>
      </form>
    </Modal>
  )
}

/** Link this member's Telegram/Discord account so reminders can mention them. */
function ChatLinkCard({ status }) {
  const toast = useToast()
  const [code, setCode] = useState(null)
  const s = status.data
  if (!s || !s.can_link || s.bots.length === 0) return null

  async function makeCode() {
    try { setCode(await remindersApi.createLinkCode()) } catch (err) { toast.error(err.message) }
  }
  async function unlink(platform) {
    try { await remindersApi.unlinkChat(platform); toast.success('Unlinked'); status.reload() } catch (err) { toast.error(err.message) }
  }
  const linked = [s.telegram_linked && 'telegram', s.discord_linked && 'discord'].filter(Boolean)

  return (
    <div className="card">
      <h2>Link your chat account</h2>
      <p className="muted">Reminders for you then mention you, and tapping Done in chat counts as you.</p>
      {linked.length > 0 && (
        <p>Linked: {linked.map(p => (
          <span key={p} className="badge badge-savings" style={{ marginRight: '.4rem' }}>
            {p} <button className="icon-btn" onClick={() => unlink(p)} aria-label={`Unlink ${p}`}><IconClose width={10} height={10} /></button>
          </span>
        ))}</p>
      )}
      {code ? (
        <div>
          <p>Send this to the household bot within 15 minutes:</p>
          <div className="copy-block">
            <code>{code.command}</code>
            <button className="icon-btn" onClick={() => { navigator.clipboard?.writeText(code.command); toast.success('Copied') }} aria-label="Copy command"><IconCopy width={13} height={13} /></button>
          </div>
        </div>
      ) : (
        <button className="secondary" onClick={makeCode}>Link my chat account</button>
      )}
    </div>
  )
}

const platformName = p => (p === 'telegram' ? 'Telegram' : p === 'discord' ? 'Discord' : p)

export default function Reminders() {
  const { user } = useAuth()
  const toast = useToast()
  const manager = canManage(user) // only the family admin and parents create or edit reminders
  const upcoming = useApi(() => remindersApi.upcoming(7), [])
  const rules = useApi(() => remindersApi.list(), [])
  const family = useApi(() => api.family(), [])
  const chat = useApi(() => remindersApi.chatLink(), [])
  const [hidden, hide, unhide] = useHiddenIds()
  const [editing, setEditing] = useState(null) // null | {row} | {template} | {}
  const [catFilter, setCatFilter] = useState('all')
  const [mode, setMode] = useState(() => { try { return localStorage.getItem('kinkit_reminders_view') || 'list' } catch { return 'list' } })
  const [params, setParams] = useSearchParams()
  const members = family.data?.members || []
  const tz = upcoming.data?.timezone || rules.data?.timezone

  // "/reminders?new=1" (e.g. from Today) opens the create form
  useEffect(() => {
    if (params.get('new') && manager) {
      setEditing({})
      params.delete('new')
      setParams(params, { replace: true })
    }
  }, [params, setParams, manager])

  const reloadAll = () => { upcoming.reload(); rules.reload(); setCalKey(k => k + 1) }
  const [calKey, setCalKey] = useState(0)
  const chooseMode = m => { setMode(m); try { localStorage.setItem('kinkit_reminders_view', m) } catch { /* ignore */ } }
  useEffect(() => {
    const id = setInterval(upcoming.reload, 60000) // keep "due" state fresh
    return () => clearInterval(id)
  }, [upcoming.reload])

  const bots = chat.data?.bots || []
  const chatHint = bots.length > 0
    ? `Sent to your family's ${bots.map(platformName).join(' and ')} chat with Done and Snooze buttons, and shown on Today.`
    : 'Shown here and on Today. Connect Telegram or Discord to get them in your family chat too.'

  const items = upcoming.data?.items || []
  const todayKey = upcoming.data?.today
  const groups = useMemo(() => {
    const dayOf = o => new Date(o.snoozed_until && o.status === 'snoozed' ? o.snoozed_until : o.due_at).toLocaleDateString('en-CA', { timeZone: tz })
    const open = items.filter(o => o.status !== 'done' && o.status !== 'missed')
    return {
      overdue: open.filter(o => o.overdue),
      today: items.filter(o => !o.overdue && o.status !== 'missed' && dayOf(o) === todayKey),
      later: open.filter(o => !o.overdue && dayOf(o) !== todayKey),
    }
  }, [items, todayKey, tz])

  const ruleRows = (rules.data?.reminders || []).filter(r => !hidden.has(r.id))
  const existingTitles = useMemo(() => new Set(ruleRows.map(r => r.title.trim().toLowerCase())), [ruleRows])
  const byCategory = useMemo(() => {
    const out = []
    for (const c of REMINDER_CATEGORIES) {
      const list = ruleRows.filter(r => (REMINDER_CATEGORIES.some(x => x.id === r.category) ? r.category : 'other') === c.id)
      if (list.length) out.push({ ...c, list })
    }
    return out
  }, [ruleRows])
  const shownGroups = catFilter === 'all' ? byCategory : byCategory.filter(g => g.id === catFilter)

  function remove(r) {
    hide(r.id)
    toast.undoable({
      message: `Deleted reminder "${r.title}"`,
      commit: () => remindersApi.remove(r.id),
      onUndo: () => unhide(r.id),
      onCommitted: reloadAll,
    })
  }
  async function toggleActive(r) {
    try {
      await remindersApi.update(r.id, { active: !r.active })
      toast.success(r.active ? `Paused "${r.title}"` : `Resumed "${r.title}"`)
      reloadAll()
    } catch (err) { toast.error(err.message) }
  }

  const occList = (list, emptyText) => list.length === 0 ? (emptyText ? <p className="muted">{emptyText}</p> : null) : (
    <ul className="occ-list">
      {list.map(o => (
        <li key={o.id} className={`occ ${o.status === 'done' ? 'done' : ''} ${o.overdue ? 'overdue' : ''}`}>
          <span className={`rem-cat-ico cat-${o.category}`} aria-hidden="true"><CategoryIcon id={o.category} width={18} height={18} /></span>
          <span className="occ-main">
            <strong>{o.title}</strong>
            <span className="muted">
              {formatDue(o.status === 'snoozed' && o.snoozed_until ? o.snoozed_until : o.due_at, tz)}
              {o.status === 'snoozed' && ' · snoozed'}
              {' · '}{o.assignee_name || 'Whole family'}
              {o.status === 'done' && o.done_by_name && ` · done by ${o.done_by_name}`}
            </span>
          </span>
          {o.status === 'done' ? <span className="badge badge-savings">Done</span>
            : o.can_act && <OccurrenceActions occurrenceId={o.id} title={o.title} onChanged={upcoming.reload} />}
        </li>
      ))}
    </ul>
  )

  return (
    <div className="reminders-page">
      <PageHeader
        title="Reminders"
        subtitle={`Household routines, bills and anything the family should not forget.${tz ? ` Times in ${tz}.` : ''}`}
        actions={manager && <button onClick={() => setEditing({})}><IconPlus width={16} height={16} aria-hidden="true" /> New reminder</button>}
      />
      <p className="rem-help">
        <IconBell width={15} height={15} aria-hidden="true" />
        <span>
          {manager ? 'Pick a household template or add your own. ' : 'Mark your reminders done or snooze them here or in chat. '}
          In the family chat you can also type <code>/remind every 2 weeks on fri 10:00 change bed sheets</code>.
        </span>
      </p>
      <div className={`rem-delivery ${bots.length ? 'on' : ''}`} role="note">
        <IconChat width={16} height={16} aria-hidden="true" />
        <span>{chatHint}{' '}
          {!bots.length && manager && <ChevronLink to="/settings/bots">Connect a chat bot</ChevronLink>}
        </span>
      </div>
      {(upcoming.error || rules.error) && <div className="error-box">{upcoming.error || rules.error}</div>}

      <div className="cal-seg" role="group" aria-label="Reminders view" style={{ marginBottom: 'var(--s-3)' }}>
        <button type="button" aria-pressed={mode === 'list'} onClick={() => chooseMode('list')}>List</button>
        <button type="button" aria-pressed={mode === 'calendar'} onClick={() => chooseMode('calendar')}>Calendar</button>
      </div>
      {mode === 'calendar' && (
        <RemindersCalendar reloadKey={calKey}
          renderActions={(o, done) => <OccurrenceActions occurrenceId={o.id} title={o.title} onChanged={() => { reloadAll(); done() }} />} />
      )}

      {mode === 'list' && <section className="card" aria-labelledby="rem-upcoming-h">
        <h2 id="rem-upcoming-h">Upcoming <span className="muted rem-h-note">next 7 days</span></h2>
        {!upcoming.data ? <SkeletonCard lines={3} /> : (
          <>
            {groups.overdue.length > 0 && <><h3 className="rem-subh text-negative">Overdue</h3>{occList(groups.overdue)}</>}
            <h3 className="rem-subh">Today</h3>
            {occList(groups.today, 'Nothing due today.')}
            <h3 className="rem-subh">Later this week</h3>
            {occList(groups.later, 'Nothing else coming up this week.')}
          </>
        )}
      </section>}

      {manager && (
        <section className="card" aria-labelledby="rem-quick-h">
          <h2 id="rem-quick-h">Quick add</h2>
          <p className="muted rem-quick-sub">Common household routines. Tap one to review and save it.</p>
          <ul className="rem-templates">
            {REMINDER_TEMPLATES.map(t => {
              const added = existingTitles.has(t.title.toLowerCase())
              return (
                <li key={t.key}>
                  <button type="button" className={`rem-template ${added ? 'added' : ''}`} onClick={() => setEditing({ template: t })}
                    aria-label={`Add reminder: ${t.title}, ${previewSchedule(fromTemplate(t))}${added ? ' (already added)' : ''}`}>
                    <span className={`rem-cat-ico cat-${t.category}`} aria-hidden="true"><CategoryIcon id={t.category} width={18} height={18} /></span>
                    <span className="rem-template-text">
                      <strong>{t.title}</strong>
                      <span>{previewSchedule(fromTemplate(t))}</span>
                    </span>
                    {added ? <IconCheck width={15} height={15} aria-hidden="true" className="rem-template-mark" /> : <IconPlus width={15} height={15} aria-hidden="true" className="rem-template-mark" />}
                  </button>
                </li>
              )
            })}
          </ul>
        </section>
      )}

      <section className="card" aria-labelledby="rem-all-h">
        <h2 id="rem-all-h">All reminders</h2>
        {byCategory.length > 1 && (
          <div className="rem-filter" role="group" aria-label="Filter by category">
            <button type="button" className={`chip ${catFilter === 'all' ? 'active' : ''}`} aria-pressed={catFilter === 'all'} onClick={() => setCatFilter('all')}>All</button>
            {byCategory.map(g => (
              <button type="button" key={g.id} className={`chip ${catFilter === g.id ? 'active' : ''}`} aria-pressed={catFilter === g.id} onClick={() => setCatFilter(g.id)}>
                {g.label} ({g.list.length})
              </button>
            ))}
          </div>
        )}
        {!rules.data ? <SkeletonCard lines={3} /> : ruleRows.length === 0 ? (
          <p className="muted">No reminders yet. {manager && 'Start with a template from Quick add above.'}</p>
        ) : shownGroups.map(g => (
          <div key={g.id} className="rem-group">
            <h3 className="rem-group-h">
              <span className={`rem-cat-ico cat-${g.id}`} aria-hidden="true"><CategoryIcon id={g.id} width={16} height={16} /></span>
              {g.label} <span className="muted">({g.list.length})</span>
            </h3>
            <ul className="rem-rules">
              {g.list.map(r => (
                <li key={r.id} className={`rem-rule ${r.active ? '' : 'paused'}`}>
                  <div className="rem-rule-main">
                    <strong>{r.title}{!r.active && <span className="badge rem-paused">Paused</span>}</strong>
                    <span className="rem-meta">
                      <span><IconRepeat width={13} height={13} aria-hidden="true" /> {r.description}</span>
                      <span><IconUser width={13} height={13} aria-hidden="true" /> {r.assignee_name || 'Whole family'}</span>
                      {r.active && r.next_due && <span><IconClock width={13} height={13} aria-hidden="true" /> Next: {formatDue(r.next_due, tz)}</span>}
                    </span>
                  </div>
                  {manager && (
                    <div className="rem-rule-actions">
                      <button className="icon-btn round" onClick={() => toggleActive(r)} aria-label={`${r.active ? 'Pause' : 'Resume'} ${r.title}`} title={r.active ? 'Pause' : 'Resume'}>
                        {r.active ? <IconPause width={14} height={14} /> : <IconPlay width={14} height={14} />}
                      </button>
                      <button className="icon-btn round" onClick={() => setEditing({ row: r })} aria-label={`Edit ${r.title}`} title="Edit"><IconEdit width={14} height={14} /></button>
                      <button className="icon-btn danger round" onClick={() => remove(r)} aria-label={`Delete ${r.title}`} title="Delete"><IconClose width={14} height={14} /></button>
                    </div>
                  )}
                </li>
              ))}
            </ul>
          </div>
        ))}
      </section>

      <ChatLinkCard status={chat} />

      {editing && (
        <ReminderModal
          initial={editing.row || null}
          template={editing.template || null}
          members={members}
          chatHint={chatHint}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); reloadAll() }}
        />
      )}
    </div>
  )
}
