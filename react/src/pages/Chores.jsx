import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, choresApi, formatRupiah, notifySpendingChanged } from '../api'
import { useApi } from '../useApi'
import { useAuth } from '../AuthContext.jsx'
import { useToast, useHiddenIds } from '../Toast.jsx'
import Modal from '../Modal.jsx'
import PageHeader from '../PageHeader.jsx'
import StatCard from '../StatCard.jsx'
import MoneyInput from '../MoneyInput.jsx'
import Celebration, { diffNewRewards } from '../Celebration.jsx'
import { SkeletonCard, SkeletonStats } from '../Skeleton.jsx'
import { IconBroom, IconCamera, IconCheck, IconChevronLeft, IconChevronRight, IconClose, IconEdit, IconPlus, IconSparkle } from '../icons.jsx'

// ---------- helpers ----------

const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const STATUS_LABEL = { todo: 'To do', submitted: 'Waiting for approval', approved: 'Done', rejected: 'Try again', missed: 'Missed' }

// Dates are household-local calendar days as YYYY-MM-DD; do the math in UTC.
const parseDay = s => new Date(`${s}T00:00:00Z`)
const fmtDay = d => d.toISOString().slice(0, 10)
const addDays = (s, n) => { const d = parseDay(s); d.setUTCDate(d.getUTCDate() + n); return fmtDay(d) }
const weekStartOf = s => { const d = parseDay(s); return addDays(s, -((d.getUTCDay() + 6) % 7)) }
const dayLabel = s => { const d = parseDay(s); return `${WEEKDAYS[d.getUTCDay()]} ${d.getUTCDate()}` }

function scheduleLabel(c) {
  let s
  switch (c.schedule) {
    case 'daily': s = 'Every day'; break
    case 'weekly': s = `Every ${(c.weekdays?.length ? c.weekdays : [parseDay(c.start_date).getUTCDay()]).map(d => WEEKDAYS[d]).join(', ')}`; break
    case 'monthly': s = `Monthly on day ${c.day_of_month || parseDay(c.start_date).getUTCDate()}`; break
    default: s = `Once on ${c.start_date}`
  }
  return c.due_time ? `${s} by ${c.due_time}` : s
}

function StatusPill({ status, projected }) {
  if (projected) return <span className="chore-pill chore-projected">Upcoming</span>
  return <span className={`chore-pill chore-${status}`}>{STATUS_LABEL[status] || status}</span>
}

// Celebrate chores a parent approved since this browser last looked.
const approvedKey = id => `kinkit:chores-approved:${id}`
function newlyApproved(items, memberId) {
  const mine = items.filter(i => i.status === 'approved' && i.requires_approval && i.member_id === memberId)
  let prev = null
  try { prev = JSON.parse(localStorage.getItem(approvedKey(memberId)) || 'null') } catch { prev = null }
  const ids = mine.map(i => i.id)
  try { localStorage.setItem(approvedKey(memberId), JSON.stringify([...new Set([...(prev || []), ...ids])].slice(-200))) } catch { return [] }
  if (!prev) return []
  const seen = new Set(prev)
  return mine.filter(i => !seen.has(i.id)).map(i => ({
    iconKey: 'chore_approved', title: `Approved: ${i.title}`, text: i.points ? `+${i.points} points` : 'Great job!',
  }))
}

/** Shared actions: complete/undo/approve/reject with toasts + reward celebrations. */
function useChoreActions(reload) {
  const toast = useToast()
  const [celebrate, setCelebrate] = useState([])
  const [busy, setBusy] = useState(null)

  const afterChange = useCallback(() => {
    reload()
    notifySpendingChanged() // refreshes the sidebar points badge + Family page
    api.gamification().then(g => {
      const fresh = diffNewRewards(g)
      if (fresh.length) setCelebrate(c => [...c, ...fresh])
    }).catch(() => {})
  }, [reload])

  const undo = useCallback(async item => {
    try {
      await choresApi.undo(item.id)
      toast.info(`"${item.title}" is back on the list`)
      afterChange()
    } catch (e) { toast.error(e.message) }
  }, [afterChange, toast])

  const complete = useCallback(async (item, photo) => {
    setBusy(item.id)
    try {
      const res = await choresApi.complete(item.id, { photo })
      toast.show({ message: res.message, type: 'success', duration: 7000, action: { label: 'Undo', onClick: () => undo(item) } })
      afterChange()
    } catch (e) { toast.error(e.message) } finally { setBusy(null) }
  }, [afterChange, toast, undo])

  const approve = useCallback(async item => {
    setBusy(item.id)
    try {
      const res = await choresApi.approve(item.id)
      toast.show({ message: res.message, type: 'success', duration: 7000, action: { label: 'Undo', onClick: () => undo(item) } })
      afterChange()
    } catch (e) { toast.error(e.message) } finally { setBusy(null) }
  }, [afterChange, toast, undo])

  const reject = useCallback(async (item, note) => {
    setBusy(item.id)
    try {
      await choresApi.reject(item.id, note)
      toast.info(`Sent "${item.title}" back${item.member_name ? ` to ${item.member_name}` : ''}`)
      afterChange()
    } catch (e) { toast.error(e.message) } finally { setBusy(null) }
  }, [afterChange, toast])

  return { complete, undo, approve, reject, busy, celebrate, setCelebrate }
}

/** Camera / file picker styled as a button. */
function PhotoButton({ onPick, label, className = '', disabled }) {
  return (
    <label className={`chore-photo-btn ${className} ${disabled ? 'disabled' : ''}`}>
      {label}
      <input type="file" accept="image/*" capture="environment" disabled={disabled} className="sr-only"
        onChange={e => { const f = e.target.files?.[0]; e.target.value = ''; if (f) onPick(f) }} />
    </label>
  )
}

// ---------- kid view ----------

function KidView({ data, actions, memberId }) {
  const { complete, undo, busy } = actions
  const items = data.items
  const week = data.week
  const todo = items.filter(i => !i.done)
  const done = items.filter(i => i.done)
  return (
    <>
      <div className="cards-row">
        <StatCard label="Points this week" value={String(week.points)} tone="positive" sub={`${week.done} chore${week.done === 1 ? '' : 's'} done this week`} />
        <StatCard label="Today" value={`${done.length} / ${items.length}`} sub={todo.length ? `${todo.length} to go` : items.length ? 'All done - amazing!' : 'Nothing today'} />
        {week.earned_month > 0 && <StatCard label="Earned this month" value={formatRupiah(week.earned_month)} tone="savings" sub="from chores, on top of your allowance" />}
      </div>
      {items.length === 0 && (
        <div className="card empty-hint"><p className="chore-big-emoji" aria-hidden="true"><IconSparkle width={44} height={44} /></p><p>No chores for you today.</p></div>
      )}
      <ul className="chore-kid-grid">
        {todo.map(i => (
          <li key={i.id} className={`chore-kid-card ${i.overdue ? 'overdue' : ''} ${i.status === 'rejected' ? 'rejected' : ''}`}>
            <span className="chore-big-emoji" aria-hidden="true"><IconBroom width={44} height={44} /></span>
            <strong className="chore-kid-title">{i.title}</strong>
            <div className="chore-kid-meta">
              {i.points > 0 && <span className="chip">+{i.points} pts</span>}
              {i.reward_amount > 0 && <span className="chip">+{formatRupiah(i.reward_amount)}</span>}
              {i.overdue && <span className="chore-pill chore-missed">From {dayLabel(i.due_date)}</span>}
              {i.due_time && <span className="muted">by {i.due_time}</span>}
              {i.member_id == null && <span className="muted">anyone</span>}
            </div>
            {i.status === 'rejected' && <div className="chore-note">Try again{i.note ? `: ${i.note}` : ''}</div>}
            {i.requires_photo ? (
              <PhotoButton className="chore-done-btn" label={busy === i.id ? 'Sending...' : <><IconCamera width={18} height={18} aria-hidden="true" /> Take photo & finish</>} disabled={busy === i.id} onPick={f => complete(i, f)} />
            ) : (
              <>
                <button className="chore-done-btn" disabled={busy === i.id} onClick={() => complete(i)}>{busy === i.id ? 'Saving...' : <><IconCheck width={18} height={18} aria-hidden="true" /> Done!</>}</button>
                <PhotoButton className="link-btn" label={<><IconCamera width={15} height={15} aria-hidden="true" /> Done with a photo</>} disabled={busy === i.id} onPick={f => complete(i, f)} />
              </>
            )}
          </li>
        ))}
      </ul>
      {done.length > 0 && (
        <div className="card">
          <h2>Done today</h2>
          <ul className="chore-list">
            {done.map(i => (
              <li key={i.id} className="chore-row">
                <span className="chore-emoji" aria-hidden="true"><IconBroom width={20} height={20} /></span>
                <span className="chore-row-main"><strong>{i.title}</strong> <StatusPill status={i.status} /></span>
                {(i.status === 'submitted' || !i.requires_approval) && i.member_id === memberId && (
                  <button className="link-btn" onClick={() => undo(i)}>Undo</button>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </>
  )
}

// ---------- parent / viewer view ----------

function RejectModal({ item, onClose, onReject }) {
  const [note, setNote] = useState('')
  return (
    <Modal title={`Send back "${item.title}"?`} onClose={onClose}>
      <form className="family-form" onSubmit={e => { e.preventDefault(); onReject(note); onClose() }}>
        <label style={{ gridColumn: '1 / -1' }}>What needs fixing? (optional)
          <input data-autofocus value={note} onChange={e => setNote(e.target.value)} maxLength={500} placeholder="e.g. the bed isn't made yet" />
        </label>
        <div className="family-form-actions">
          <button type="button" className="secondary" onClick={onClose}>Cancel</button>
          <button type="submit" className="danger">Send back</button>
        </div>
      </form>
    </Modal>
  )
}

function ApprovalQueue({ waiting, actions }) {
  const [rejecting, setRejecting] = useState(null)
  const [photo, setPhoto] = useState(null)
  if (!waiting.length) return null
  return (
    <div className="card chore-approvals">
      <h2>Waiting for approval <span className="badge chore-count">{waiting.length}</span></h2>
      <ul className="chore-list">
        {waiting.map(i => (
          <li key={i.id} className="chore-row">
            {i.has_photo ? (
              <button className="chore-thumb" onClick={() => setPhoto(i)} aria-label={`View photo for ${i.title}`}>
                <img src={choresApi.photoUrl(i.id)} alt="" loading="lazy" />
              </button>
            ) : <span className="chore-emoji" aria-hidden="true"><IconBroom width={20} height={20} /></span>}
            <span className="chore-row-main">
              <strong>{i.title}</strong>
              <span className="muted"> · {i.member_name || 'someone'} · {dayLabel(i.due_date)}{i.points ? ` · +${i.points} pts` : ''}</span>
              {i.note && <div className="chore-note">"{i.note}"</div>}
            </span>
            <span className="row-actions">
              <button disabled={actions.busy === i.id} onClick={() => actions.approve(i)}>Approve</button>
              <button className="secondary" disabled={actions.busy === i.id} onClick={() => setRejecting(i)}>Send back</button>
            </span>
          </li>
        ))}
      </ul>
      {rejecting && <RejectModal item={rejecting} onClose={() => setRejecting(null)} onReject={note => actions.reject(rejecting, note)} />}
      {photo && (
        <Modal title={`${photo.title} - ${photo.member_name || ''}`} onClose={() => setPhoto(null)} className="chore-photo-modal">
          <img className="chore-photo-full" src={choresApi.photoUrl(photo.id)} alt={`Photo for ${photo.title}`} />
          <div className="family-form-actions">
            <button className="secondary" onClick={() => { setRejecting(photo); setPhoto(null) }}>Send back</button>
            <button data-autofocus onClick={() => { actions.approve(photo); setPhoto(null) }}>Approve</button>
          </div>
        </Modal>
      )}
    </div>
  )
}

function TodayBoard({ data, actions, canManage }) {
  const groups = useMemo(() => {
    const byKey = new Map()
    for (const m of data.members) byKey.set(m.id, { key: m.id, name: m.name, items: [] })
    byKey.set('anyone', { key: 'anyone', name: 'Anyone', items: [] })
    for (const i of data.items) {
      const g = byKey.get(i.member_id ?? 'anyone') || byKey.get('anyone')
      g.items.push(i)
    }
    return [...byKey.values()].filter(g => g.items.length)
  }, [data])
  return (
    <div className="card">
      <h2>Today's board</h2>
      {groups.length === 0 && <p className="muted">No chores due today.</p>}
      <div className="chore-board">
        {groups.map(g => {
          const doneN = g.items.filter(i => i.done).length
          return (
            <section key={g.key} className="chore-board-col" aria-label={`${g.name}'s chores`}>
              <h3>{g.name} <span className="muted">{doneN}/{g.items.length}</span></h3>
              <ul className="chore-list">
                {g.items.map(i => (
                  <li key={i.id} className={`chore-row ${i.done ? 'is-done' : ''}`}>
                    <span className="chore-emoji" aria-hidden="true"><IconBroom width={20} height={20} /></span>
                    <span className="chore-row-main">
                      <strong>{i.title}</strong>
                      <span className="chore-row-sub"><StatusPill status={i.status} />{i.overdue && <span className="muted"> from {dayLabel(i.due_date)}</span>}</span>
                    </span>
                    {canManage && (
                      <span className="row-actions">
                        {!i.done && <button className="secondary" disabled={actions.busy === i.id} onClick={() => actions.complete(i)} title="Mark done (counts as approved)">Done</button>}
                        {i.status === 'submitted' && <button disabled={actions.busy === i.id} onClick={() => actions.approve(i)}>Approve</button>}
                        {i.done && <button className="link-btn" onClick={() => actions.undo(i)}>Undo</button>}
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            </section>
          )
        })}
      </div>
    </div>
  )
}

function WeekGrid({ today, nonce }) {
  const [start, setStart] = useState(() => weekStartOf(today))
  const end = addDays(start, 6)
  const week = useApi(() => choresApi.instances(start, end), [start, nonce])
  const days = Array.from({ length: 7 }, (_, i) => addDays(start, i))
  const d = week.data
  const rows = useMemo(() => {
    if (!d) return []
    const list = d.members.map(m => ({ id: m.id, name: m.name }))
    if (d.instances.some(i => i.member_id == null)) list.push({ id: null, name: 'Anyone' })
    return list.filter(r => d.instances.some(i => (i.member_id ?? null) === r.id))
  }, [d])
  return (
    <div className="card">
      <div className="chore-week-head">
        <h2>Week view</h2>
        <div className="period-control">
          <button className="period-step" onClick={() => setStart(s => addDays(s, -7))} aria-label="Previous week"><IconChevronLeft width={16} height={16} /></button>
          <button className="period-label" onClick={() => setStart(weekStartOf(today))} title="Back to this week">{dayLabel(start)} - {dayLabel(end)}</button>
          <button className="period-step" onClick={() => setStart(s => addDays(s, 7))} aria-label="Next week"><IconChevronRight width={16} height={16} /></button>
        </div>
      </div>
      {!d ? <SkeletonCard lines={3} /> : rows.length === 0 ? <p className="muted">No chores this week.</p> : (
        <div className="chore-week-scroll">
          <table className="chore-week">
            <thead>
              <tr><th scope="col">Who</th>{days.map(day => <th key={day} scope="col" className={day === today ? 'today' : ''}>{dayLabel(day)}</th>)}</tr>
            </thead>
            <tbody>
              {rows.map(r => (
                <tr key={r.id ?? 'anyone'}>
                  <th scope="row">{r.name}</th>
                  {days.map(day => {
                    const cell = d.instances.filter(i => i.due_date === day && (i.member_id ?? null) === r.id)
                    return (
                      <td key={day} className={day === today ? 'today' : ''}>
                        {cell.map((i, k) => (
                          <span key={i.id || `p${i.chore_id}-${k}`} className={`chore-cell ${i.projected ? 'chore-projected' : `chore-${i.status}`}`}
                            title={`${i.title} - ${i.projected ? 'upcoming' : STATUS_LABEL[i.status]}`}>
                            <span aria-hidden="true">•</span><span className="sr-only">{i.title}: {i.projected ? 'upcoming' : STATUS_LABEL[i.status]}</span>
                          </span>
                        ))}
                      </td>
                    )
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <div className="chore-legend" aria-hidden="true">
        {['todo', 'submitted', 'approved', 'rejected', 'missed'].map(s => <span key={s} className={`chore-cell chore-${s}`}>{STATUS_LABEL[s]}</span>)}
        <span className="chore-cell chore-projected">Upcoming</span>
      </div>
    </div>
  )
}

// ---------- manage ----------

function ChoreModal({ chore, members, today, onClose, onSaved }) {
  const toast = useToast()
  const editing = !!chore
  const initialMode = chore?.rotation_member_ids?.length > 1 ? 'rotation' : chore?.member_id ? 'fixed' : (editing ? 'anyone' : 'fixed')
  const kids = members.filter(m => m.role === 'kid')
  const [form, setForm] = useState({
    title: chore?.title || '',
    description: chore?.description || '',
    points: chore?.points ?? 10,
    reward: chore?.reward_amount ?? '',
    mode: initialMode,
    member: chore?.member_id ?? (kids[0]?.id || members[0]?.id || ''),
    rotation: chore?.rotation_member_ids?.length > 1 ? chore.rotation_member_ids : kids.map(k => k.id),
    schedule: chore?.schedule || 'daily',
    weekdays: chore?.weekdays?.length ? chore.weekdays : [parseDay(today).getUTCDay()],
    dom: chore?.day_of_month ?? '',
    start: chore?.start_date || today,
    end: chore?.end_date || '',
    due: chore?.due_time || '',
    approval: chore?.requires_approval ?? true,
    photo: chore?.requires_photo ?? false,
    active: chore?.active ?? true,
  })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const set = (k, v) => setForm(f => ({ ...f, [k]: v }))
  const toggle = (k, v) => setForm(f => ({ ...f, [k]: f[k].includes(v) ? f[k].filter(x => x !== v) : [...f[k], v] }))

  async function submit(e) {
    e.preventDefault()
    setError('')
    if (form.mode === 'rotation' && form.rotation.length < 2) { setError('Pick at least two people to rotate between.'); return }
    if (form.schedule === 'weekly' && form.weekdays.length === 0) { setError('Pick at least one weekday.'); return }
    const payload = {
      title: form.title, emoji: '', description: form.description,
      points: Number(form.points) || 0,
      reward_amount: Number(form.reward) || null,
      member_id: form.mode === 'fixed' ? Number(form.member) || null : null,
      rotation_member_ids: form.mode === 'rotation' ? form.rotation : [],
      schedule: form.schedule,
      weekdays: form.schedule === 'weekly' ? form.weekdays : [],
      day_of_month: form.schedule === 'monthly' && form.dom ? Number(form.dom) : null,
      start_date: form.start, end_date: form.end || null, due_time: form.due || null,
      requires_approval: form.approval, requires_photo: form.photo, active: form.active,
    }
    setBusy(true)
    try {
      if (editing) await choresApi.update(chore.id, payload)
      else await choresApi.create(payload)
      toast.success(editing ? `Saved "${form.title}"` : `Added "${form.title}"`)
      onSaved()
      onClose()
    } catch (err) { setError(err.message) } finally { setBusy(false) }
  }

  return (
    <Modal title={editing ? `Edit ${chore.title}` : 'Add chore'} onClose={onClose} className="family-modal">
      <form className="money-form chore-form" onSubmit={submit}>
        {error && <div className="error-box" role="alert">{error}</div>}
        <label className="span-2">Chore
          <input data-autofocus value={form.title} onChange={e => set('title', e.target.value)} required maxLength={120} placeholder="e.g. Feed the cat" />
        </label>
        <label>Points
          <input type="number" min={0} max={10000} value={form.points} onChange={e => set('points', e.target.value)} />
        </label>
        <label>Money reward (optional)
          <MoneyInput value={form.reward === null ? '' : String(form.reward)} onChange={v => set('reward', v)} placeholder="e.g. 5k" />
        </label>
        <fieldset className="span-2 chore-fieldset">
          <legend>Who does it?</legend>
          <div className="chore-seg" role="radiogroup">
            {[['fixed', 'One person'], ['rotation', 'Take turns'], ['anyone', 'Anyone']].map(([v, l]) => (
              <label key={v} className={form.mode === v ? 'active' : ''}>
                <input type="radio" name="mode" className="sr-only" checked={form.mode === v} onChange={() => set('mode', v)} />{l}
              </label>
            ))}
          </div>
          {form.mode === 'fixed' && (
            <select aria-label="Assignee" value={form.member} onChange={e => set('member', e.target.value)}>
              {members.map(m => <option key={m.id} value={m.id}>{m.name}</option>)}
            </select>
          )}
          {form.mode === 'rotation' && (
            <div className="chore-checks">
              {members.map(m => (
                <label key={m.id}><input type="checkbox" checked={form.rotation.includes(m.id)} onChange={() => toggle('rotation', m.id)} /> {m.name}</label>
              ))}
              <span className="muted form-hint">Each time the chore comes up, the next person in this list gets it.</span>
            </div>
          )}
          {form.mode === 'anyone' && <span className="muted form-hint">Whoever does it first gets the points.</span>}
        </fieldset>
        <label>How often
          <select value={form.schedule} onChange={e => set('schedule', e.target.value)}>
            <option value="once">Once</option>
            <option value="daily">Every day</option>
            <option value="weekly">Weekly</option>
            <option value="monthly">Monthly</option>
          </select>
        </label>
        <label>{form.schedule === 'once' ? 'On' : 'Starting'}
          <input type="date" value={form.start} onChange={e => set('start', e.target.value)} required />
        </label>
        {form.schedule === 'weekly' && (
          <div className="span-2 chore-emoji-pick chore-days" role="group" aria-label="Weekdays">
            {[1, 2, 3, 4, 5, 6, 0].map(d => (
              <button type="button" key={d} className={form.weekdays.includes(d) ? 'active' : ''} aria-pressed={form.weekdays.includes(d)} onClick={() => toggle('weekdays', d)}>{WEEKDAYS[d]}</button>
            ))}
          </div>
        )}
        {form.schedule === 'monthly' && (
          <label>Day of month
            <input type="number" min={1} max={31} value={form.dom} onChange={e => set('dom', e.target.value)} placeholder={String(parseDay(form.start).getUTCDate())} />
          </label>
        )}
        <label>Due by (optional)
          <input type="time" value={form.due} onChange={e => set('due', e.target.value)} />
        </label>
        {form.schedule !== 'once' && (
          <label>Ends (optional)
            <input type="date" value={form.end} min={form.start} onChange={e => set('end', e.target.value)} />
          </label>
        )}
        <div className="span-2 chore-checks">
          <label><input type="checkbox" checked={form.approval} onChange={e => set('approval', e.target.checked)} /> A parent approves it before points are given</label>
          <label><input type="checkbox" checked={form.photo} onChange={e => set('photo', e.target.checked)} /> Needs a photo as proof</label>
          {editing && <label><input type="checkbox" checked={form.active} onChange={e => set('active', e.target.checked)} /> Active</label>}
        </div>
        <div className="span-2 money-form-actions">
          <button type="button" className="secondary" onClick={onClose}>Cancel</button>
          <button type="submit" disabled={busy}>{busy ? 'Saving...' : editing ? 'Save' : 'Add chore'}</button>
        </div>
      </form>
    </Modal>
  )
}

function ManageChores({ today, nonce, onChanged }) {
  const toast = useToast()
  const [hidden, hide, unhide] = useHiddenIds()
  const [modal, setModal] = useState(null)
  const list = useApi(() => choresApi.list(), [nonce])
  const d = list.data
  const names = useMemo(() => Object.fromEntries((d?.members || []).map(m => [m.id, m.name])), [d])
  const who = c => c.rotation_member_ids.length > 1
    ? `Turns: ${c.rotation_member_ids.map(id => names[id] || '?').join(' → ')}`
    : c.member_id ? names[c.member_id] || 'Removed member' : 'Anyone'

  function remove(c) {
    hide(c.id)
    toast.undoable({
      message: `Deleted "${c.title}"`,
      commit: () => choresApi.remove(c.id),
      onUndo: () => unhide(c.id),
      onCommitted: () => { list.reload(); onChanged() },
      onError: e => { unhide(c.id); toast.error(e.message) },
    })
  }

  const chores = (d?.chores || []).filter(c => !hidden.has(c.id))
  return (
    <div className="card">
      <div className="chore-week-head">
        <h2>Chores</h2>
        <button onClick={() => setModal('add')}><IconPlus width={16} height={16} aria-hidden="true" /> Add chore</button>
      </div>
      {list.error && <div className="error-box" role="alert">{list.error}</div>}
      {!d ? <SkeletonCard lines={3} /> : chores.length === 0 ? (
        <div className="empty-hint">
          <p className="chore-big-emoji" aria-hidden="true"><IconBroom width={44} height={44} /></p>
          <p>No chores yet. Add one - like "Make your bed" every day, or "Take out the trash" taking turns.</p>
        </div>
      ) : (
        <ul className="chore-list">
          {chores.map(c => (
            <li key={c.id} className={`chore-row ${c.active ? '' : 'inactive'}`}>
              <span className="chore-emoji" aria-hidden="true"><IconBroom width={20} height={20} /></span>
              <span className="chore-row-main">
                <strong>{c.title}</strong>{!c.active && <span className="muted"> (paused)</span>}
                <span className="chore-row-sub muted">
                  {scheduleLabel(c)} · {who(c)} · {c.points} pts{c.reward_amount ? ` + ${formatRupiah(c.reward_amount)}` : ''}
                  {c.requires_approval ? ' · approval' : ''}{c.requires_photo ? ' · photo' : ''}
                </span>
              </span>
              <span className="row-actions">
                <button className="icon-btn" onClick={() => setModal(c)} aria-label={`Edit ${c.title}`} title="Edit"><IconEdit width={15} height={15} /></button>
                <button className="icon-btn danger" onClick={() => remove(c)} aria-label={`Delete ${c.title}`} title="Delete"><IconClose width={14} height={14} /></button>
              </span>
            </li>
          ))}
        </ul>
      )}
      {modal && d && (
        <ChoreModal chore={modal === 'add' ? null : modal} members={d.members} today={today}
          onClose={() => setModal(null)} onSaved={() => { list.reload(); onChanged() }} />
      )}
    </div>
  )
}

// ---------- page ----------

export default function Chores() {
  const { user } = useAuth()
  const [nonce, setNonce] = useState(0)
  const today = useApi(() => choresApi.today(), [nonce])
  const reload = useCallback(() => setNonce(n => n + 1), [])
  const actions = useChoreActions(reload)
  const { setCelebrate } = actions
  const data = today.data
  const role = data?.me?.role || user?.role || 'owner'
  const isKid = role === 'kid'
  const canManage = !!data?.me?.can_manage

  useEffect(() => {
    if (!data || !isKid) return
    const fresh = newlyApproved(data.items, data.me.id)
    if (fresh.length) setCelebrate(c => [...c, ...fresh])
  }, [data, isKid, setCelebrate])

  const subtitle = data ? (isKid ? "Your chores for today" : `${data.items.filter(i => i.done).length} of ${data.items.length} done today${data.waiting.length ? ` · ${data.waiting.length} waiting for approval` : ''}`) : undefined

  return (
    <div className="chores-page">
      <PageHeader title="Chores" subtitle={subtitle} />
      {today.error && <div className="error-box" role="alert">{today.error}</div>}
      {!data ? (<><SkeletonStats count={2} /><SkeletonCard lines={4} /></>) : isKid ? (
        <>
          <KidView data={data} actions={actions} memberId={data.me.id} />
          <WeekGrid today={data.date} nonce={nonce} />
        </>
      ) : (
        <>
          {canManage && <ApprovalQueue waiting={data.waiting} actions={actions} />}
          <TodayBoard data={data} actions={actions} canManage={canManage} />
          <WeekGrid today={data.date} nonce={nonce} />
          {canManage && <ManageChores today={data.date} nonce={nonce} onChanged={reload} />}
        </>
      )}
      {actions.celebrate.length > 0 && <Celebration items={actions.celebrate} onClose={() => setCelebrate([])} />}
    </div>
  )
}
