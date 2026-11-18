import { useEffect, useMemo, useState } from 'react'
import { api, remindersApi, todayISO } from '../api'
import { useApi } from '../useApi'
import { useAuth, canManage } from '../AuthContext.jsx'
import { useToast, useHiddenIds } from '../Toast.jsx'
import Modal from '../Modal.jsx'
import PageHeader from '../PageHeader.jsx'
import { SkeletonCard } from '../Skeleton.jsx'
import { IconEdit, IconClose, IconPlus, IconCopy, IconCheck, IconClock, IconCalendar, CategoryIcon } from '../icons.jsx'

// ---------- helpers (shared with Today.jsx) ----------

export const REMINDER_CATEGORIES = [
  { id: 'bill', label: 'Bill' },
  { id: 'medicine', label: 'Medicine' },
  { id: 'school', label: 'School' },
  { id: 'birthday', label: 'Birthday' },
  { id: 'chores', label: 'Chores' },
  { id: 'other', label: 'Other' },
]
const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

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
    </span>
  )
}

function emptyForm() {
  return {
    title: '', notes: '', category: 'other', assignee_id: '', kind: 'daily', time_of_day: '08:00',
    weekdays: [1], day_of_month: 1, start_date: todayISO(), end_date: '',
    escalate_to: '', escalate_after_minutes: 30, active: true,
  }
}

function fromRow(r) {
  return {
    ...emptyForm(),
    title: r.title, notes: r.notes || '', category: r.category, assignee_id: r.assignee_id || '',
    kind: r.kind, time_of_day: r.time_of_day, weekdays: r.weekdays?.length ? r.weekdays : [1],
    day_of_month: r.day_of_month || 1, start_date: r.start_date, end_date: r.end_date || '',
    escalate_to: r.escalate_to || '', escalate_after_minutes: r.escalate_after_minutes || 30, active: r.active,
  }
}

/** Friendly preview of the schedule being built ("Every Monday and Thursday at 07:00"). */
function previewSchedule(f) {
  const at = `at ${f.time_of_day}`
  switch (f.kind) {
    case 'once': return `Once on ${f.start_date} ${at}`
    case 'daily': return `Every day ${at}`
    case 'weekly': {
      const days = [...f.weekdays].sort().map(d => WEEKDAYS[d])
      return days.length ? `Every ${days.join(', ')} ${at}` : 'Pick at least one day'
    }
    case 'monthly': return `Every month on day ${f.day_of_month} ${at} (last day in shorter months)`
    case 'yearly': return `Every year on ${f.start_date.slice(5)} ${at}`
    default: return ''
  }
}

function ReminderModal({ initial, members, onClose, onSaved }) {
  const toast = useToast()
  const [f, setF] = useState(initial ? fromRow(initial) : emptyForm())
  const [saving, setSaving] = useState(false)
  const set = (k, v) => setF(x => ({ ...x, [k]: v }))
  const toggleDay = d => set('weekdays', f.weekdays.includes(d) ? f.weekdays.filter(x => x !== d) : [...f.weekdays, d])

  async function save(e) {
    e.preventDefault()
    if (f.kind === 'weekly' && f.weekdays.length === 0) { toast.error('Pick at least one day'); return }
    const payload = {
      title: f.title.trim(), notes: f.notes, category: f.category,
      assignee_id: f.assignee_id ? Number(f.assignee_id) : null,
      kind: f.kind, time_of_day: f.time_of_day,
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
    <Modal title={initial ? 'Edit reminder' : 'New reminder'} onClose={onClose}>
      <form className="stacked-form" onSubmit={save}>
        <label>What
          <input data-autofocus value={f.title} onChange={e => set('title', e.target.value)} placeholder="e.g. Pay electricity bill" required maxLength={200} />
        </label>
        <div className="form-row">
          <label>Category
            <select value={f.category} onChange={e => set('category', e.target.value)}>
              {REMINDER_CATEGORIES.map(c => <option key={c.id} value={c.id}>{c.label}</option>)}
            </select>
          </label>
          <label>Who
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
              <option value="weekly">Every week</option>
              <option value="monthly">Every month</option>
              <option value="yearly">Every year</option>
            </select>
          </label>
          <label>Time
            <input type="time" value={f.time_of_day} onChange={e => set('time_of_day', e.target.value)} required />
          </label>
        </div>
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
        <div className="modal-actions">
          <button type="button" className="secondary" onClick={onClose}>Cancel</button>
          <button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</button>
        </div>
      </form>
    </Modal>
  )
}

/** Link this member's Telegram/Discord account so reminders can mention them. */
function ChatLinkCard() {
  const toast = useToast()
  const status = useApi(() => remindersApi.chatLink(), [])
  const [code, setCode] = useState(null)
  const s = status.data
  if (!s || !s.can_link) return null

  async function makeCode() {
    try { setCode(await remindersApi.createLinkCode()) } catch (err) { toast.error(err.message) }
  }
  async function unlink(platform) {
    try { await remindersApi.unlinkChat(platform); toast.success('Unlinked'); status.reload() } catch (err) { toast.error(err.message) }
  }
  const linked = [s.telegram_linked && 'telegram', s.discord_linked && 'discord'].filter(Boolean)

  return (
    <div className="card">
      <h2>Get reminders in chat</h2>
      {s.bots.length === 0 ? (
        <p className="muted">No chat bot is connected for your household yet. The family admin or a parent can connect Telegram or Discord under Settings › Bots.</p>
      ) : (
        <>
          <p className="muted">Link your own chat account so reminders for you mention you, and so tapping Done in chat counts as you.</p>
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
        </>
      )}
    </div>
  )
}

export default function Reminders() {
  const { user } = useAuth()
  const toast = useToast()
  const manager = canManage(user)
  const readOnly = !manager // only owners/parents create or edit reminders
  const upcoming = useApi(() => remindersApi.upcoming(7), [])
  const rules = useApi(() => remindersApi.list(), [])
  const family = useApi(() => api.family(), [])
  const [hidden, hide, unhide] = useHiddenIds()
  const [editing, setEditing] = useState(null) // null | 'new' | reminder row
  const members = family.data?.members || []
  const tz = upcoming.data?.timezone || rules.data?.timezone

  const reloadAll = () => { upcoming.reload(); rules.reload() }
  useEffect(() => {
    const id = setInterval(upcoming.reload, 60000) // keep "due" state fresh
    return () => clearInterval(id)
  }, [upcoming.reload])

  const items = upcoming.data?.items || []
  const todayKey = upcoming.data?.today
  const groups = useMemo(() => {
    const dayOf = o => new Date(o.snoozed_until && o.status === 'snoozed' ? o.snoozed_until : o.due_at).toLocaleDateString('en-CA', { timeZone: tz })
    const open = items.filter(o => o.status !== 'done' && o.status !== 'missed')
    return {
      overdue: open.filter(o => o.overdue),
      today: items.filter(o => !o.overdue && dayOf(o) === todayKey),
      later: open.filter(o => !o.overdue && dayOf(o) !== todayKey),
    }
  }, [items, todayKey, tz])

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
    try { await remindersApi.update(r.id, { active: !r.active }); reloadAll() } catch (err) { toast.error(err.message) }
  }

  const occList = (list, emptyText) => list.length === 0 ? <p className="muted">{emptyText}</p> : (
    <ul className="occ-list">
      {list.map(o => (
        <li key={o.id} className={`occ ${o.status === 'done' ? 'done' : ''} ${o.overdue ? 'overdue' : ''}`}>
          <span className="occ-emoji" aria-hidden="true"><CategoryIcon id={o.category} width={20} height={20} /></span>
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

  const ruleRows = (rules.data?.reminders || []).filter(r => !hidden.has(r.id))

  return (
    <div>
      <PageHeader
        title="Reminders"
        subtitle={tz ? `Times shown in ${tz}` : 'Bills, medicine, school and anything else the family should not forget.'}
        actions={!readOnly && <button onClick={() => setEditing('new')}><IconPlus width={16} height={16} aria-hidden="true" /> New reminder</button>}
      />
      {(upcoming.error || rules.error) && <div className="error-box">{upcoming.error || rules.error}</div>}

      {!upcoming.data ? <SkeletonCard lines={4} /> : (
        <div className="grid-2">
          <div className="card">
            <h2>Today</h2>
            {groups.overdue.length > 0 && <><h3 className="text-negative">Overdue</h3>{occList(groups.overdue, '')}</>}
            {occList(groups.today, 'Nothing due today.')}
          </div>
          <div className="card">
            <h2>Next 7 days</h2>
            {occList(groups.later, 'Nothing coming up this week.')}
          </div>
        </div>
      )}

      <div className="card">
        <h2>All reminders</h2>
        {!rules.data ? <SkeletonCard lines={3} /> : ruleRows.length === 0 ? (
          <p className="muted">No reminders yet. {!readOnly && 'Add one for a bill, a medicine or the weekly trash day.'}</p>
        ) : (
          <div className="table-scroll">
            <table className="responsive-table">
              <thead><tr><th>Reminder</th><th>When</th><th>Who</th><th>Next</th><th><span className="sr-only">Actions</span></th></tr></thead>
              <tbody>
                {ruleRows.map(r => (
                  <tr key={r.id} className={r.active ? '' : 'muted'}>
                    <td data-label="Reminder"><CategoryIcon id={r.category} width={15} height={15} aria-hidden="true" className="inline-ico" /> {r.title}{!r.active && <span className="badge" style={{ marginLeft: '.4rem' }}>Paused</span>}</td>
                    <td data-label="When">{r.description}</td>
                    <td data-label="Who">{r.assignee_name || 'Whole family'}</td>
                    <td data-label="Next">{r.active && r.next_due ? formatDue(r.next_due, tz) : '-'}</td>
                    <td className="row-actions">
                      {manager && (
                        <>
                          <button className="secondary small" onClick={() => toggleActive(r)}>{r.active ? 'Pause' : 'Resume'}</button>
                          <button className="icon-btn round" onClick={() => setEditing(r)} aria-label={`Edit ${r.title}`}><IconEdit width={14} height={14} /></button>
                          <button className="icon-btn danger round" onClick={() => remove(r)} aria-label={`Delete ${r.title}`}><IconClose width={14} height={14} /></button>
                        </>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <ChatLinkCard />

      {editing && (
        <ReminderModal
          initial={editing === 'new' ? null : editing}
          members={members}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); reloadAll() }}
        />
      )}
    </div>
  )
}
