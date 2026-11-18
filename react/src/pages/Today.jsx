import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { remindersApi, choresApi, notifySpendingChanged } from '../api'
import { useApi } from '../useApi'
import { useAuth, canManage } from '../AuthContext.jsx'
import { useToast } from '../Toast.jsx'
import { SkeletonCard, SkeletonStats } from '../Skeleton.jsx'
import { formatDue } from './Reminders.jsx'
import Mascot from '../Mascot.jsx'
import { IconBell, IconBroom, IconCart, IconBudget, IconCalendar, IconCheck, IconClock } from '../icons.jsx'

const SECTION_ICONS = { reminders: IconBell, chores: IconBroom, lists: IconCart, budget: IconBudget, coming_up: IconCalendar }
const SectionIcon = ({ id }) => { const Icon = SECTION_ICONS[id] || IconCalendar; return <Icon width={18} height={18} aria-hidden="true" /> }
const DISMISS_TZ_KEY = 'kinkit_tz_prompt_dismissed'

/** One-time prompt for owners/parents while the household time zone is still UTC. */
function TimezoneBanner({ onSaved }) {
  const toast = useToast()
  const detected = Intl.DateTimeFormat().resolvedOptions().timeZone
  const [hidden, setHidden] = useState(() => { try { return localStorage.getItem(DISMISS_TZ_KEY) === '1' } catch { return false } })
  if (hidden || !detected || detected === 'UTC') return null
  const dismiss = () => { setHidden(true); try { localStorage.setItem(DISMISS_TZ_KEY, '1') } catch { /* ignore */ } }
  async function save() {
    try {
      await remindersApi.setTimezone(detected)
      toast.success(`Household time zone set to ${detected}`)
      dismiss()
      onSaved()
    } catch (err) { toast.error(err.message) }
  }
  return (
    <div className="notice-box tz-banner" role="status">
      <span>Reminders and the morning digest use the household time zone, which is still <strong>UTC</strong>. Use <strong>{detected}</strong> (this device)?</span>
      <span className="tz-banner-actions">
        <button onClick={save}>Use {detected}</button>
        <button className="secondary" onClick={dismiss}>Not now</button>
      </span>
    </div>
  )
}

function TodayItemRow({ item, timeZone, onChanged }) {
  const toast = useToast()
  const [busy, setBusy] = useState(false)

  async function act(action) {
    setBusy(true)
    try {
      if (item.kind === 'reminder') {
        if (action === 'done') await remindersApi.done(item.id)
        else await remindersApi.snooze(item.id, { minutes: 60 })
        toast.success(action === 'done' ? `Done: ${item.title}` : 'Snoozed for 1 hour')
      } else if (item.kind === 'chore') {
        const res = action === 'approve' ? await choresApi.approve(item.id) : await choresApi.complete(item.id)
        toast.success(res?.message || 'Nice work!')
        notifySpendingChanged() // refreshes the points badge
      }
      onChanged()
    } catch (err) { toast.error(err.message) } finally { setBusy(false) }
  }

  const actions = item.done ? [] : (item.actions || [])
  const due = item.due && item.due.length > 10 ? formatDue(item.due, timeZone) : item.due
  const title = item.link ? <Link to={item.link}>{item.title}</Link> : item.title

  return (
    <li className={`today-item ${item.done ? 'done' : ''} ${item.overdue ? 'overdue' : ''}`}>
      <span className="today-item-main">
        <strong>{title}</strong>
        {(item.detail || due || item.member_name) && (
          <span className="muted">{[due, item.member_name, item.detail].filter(Boolean).join(' · ')}</span>
        )}
      </span>
      {item.done && <span className="badge badge-savings">Done</span>}
      {actions.length > 0 && (
        <span className="occ-actions">
          {actions.includes('done') && <button className="small" disabled={busy} onClick={() => act('done')} aria-label={`Mark "${item.title}" done`}><IconCheck width={14} height={14} aria-hidden="true" /> Done</button>}
          {actions.includes('approve') && <button className="small" disabled={busy} onClick={() => act('approve')} aria-label={`Approve "${item.title}"`}>Approve</button>}
          {actions.includes('snooze') && <button className="secondary small" disabled={busy} onClick={() => act('snooze')} aria-label={`Snooze "${item.title}" for one hour`}><IconClock width={14} height={14} aria-hidden="true" /> 1h</button>}
        </span>
      )}
    </li>
  )
}

export default function Today() {
  const { user } = useAuth()
  const res = useApi(() => remindersApi.today(), [])
  const d = res.data

  // Keep the page fresh when it sits open (kitchen tablet) and on return.
  useEffect(() => {
    const id = setInterval(res.reload, 60000)
    const onVis = () => { if (document.visibilityState === 'visible') res.reload() }
    document.addEventListener('visibilitychange', onVis)
    return () => { clearInterval(id); document.removeEventListener('visibilitychange', onVis) }
  }, [res.reload])

  const dateLabel = d ? new Date(`${d.date}T12:00:00Z`).toLocaleDateString([], { weekday: 'long', day: 'numeric', month: 'long', timeZone: 'UTC' }) : ''
  const sections = (d?.sections || []).filter(s => s.items?.length || s.summary)

  return (
    <div>
      <div className="page-header">
        <div className="today-hello">
          <Mascot size={64} />
          <div>
            <h1>{d?.greeting || 'Today'}</h1>
            <p className="muted">{dateLabel}</p>
          </div>
        </div>
        {canManage(user) && (
          <div className="page-header-controls">
            <Link to="/reminders?new=1" className="button-link secondary">+ Reminder</Link>
          </div>
        )}
      </div>

      {d?.timezone_default && canManage(user) && <TimezoneBanner onSaved={res.reload} />}
      {res.error && <div className="error-box">{res.error}</div>}

      {!d ? <><SkeletonStats count={2} /><SkeletonCard lines={4} /></> : sections.length === 0 ? (
        <div className="card today-empty">
          <Mascot pose="relax" size={120} />
          <p><strong>All clear for today.</strong></p>
          <p className="muted">Add <Link to="/reminders">reminders</Link>, <Link to="/home/chores">chores</Link> or a <Link to="/home/lists">shopping list</Link> and they will show up here.</p>
        </div>
      ) : (
        <div className="today-grid">
          {sections.map(s => (
            <section key={s.key} className="card today-section" aria-labelledby={`today-${s.key}`}>
              <h2 id={`today-${s.key}`}><SectionIcon id={s.key} /> {s.title}</h2>
              {s.summary && <p className="muted">{s.summary}</p>}
              {s.items?.length > 0 && (
                <ul className="today-items">
                  {s.items.map((it, i) => <TodayItemRow key={`${it.kind}-${it.id || i}`} item={it} timeZone={d.timezone} onChanged={res.reload} />)}
                </ul>
              )}
            </section>
          ))}
        </div>
      )}
    </div>
  )
}
