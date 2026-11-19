import { useState } from 'react'
import { mealProposalsApi } from './api'
import { useApi } from './useApi'
import { useToast } from './Toast.jsx'
import { IconCheck, IconClose } from './icons.jsx'

const fmt = iso => { const [y, m, d] = iso.split('-').map(Number); return new Date(Date.UTC(y, m - 1, d)).toLocaleDateString(undefined, { timeZone: 'UTC', weekday: 'short', day: 'numeric', month: 'short' }) }

/**
 * Meal requests: everyone can ask for a dish; the family admin and parents
 * approve it (optionally on a day of the plan) or decline it.
 */
export default function MealRequests({ days = [], manager, onChanged }) {
  const toast = useToast()
  const list = useApi(() => mealProposalsApi.list(), [])
  const [dish, setDish] = useState('')
  const [note, setNote] = useState('')
  const [wanted, setWanted] = useState('')
  const [busy, setBusy] = useState(false)
  const [pick, setPick] = useState({}) // proposal id -> chosen day for approval

  async function run(fn, msg) {
    try { await fn(); if (msg) toast.success(msg); list.reload(); onChanged?.() } catch (e) { toast.error(e.message) }
  }
  async function submit(e) {
    e.preventDefault()
    if (!dish.trim()) return
    setBusy(true)
    await run(() => mealProposalsApi.propose({ dish: dish.trim(), note: note.trim(), wanted_date: wanted || null }), 'Request sent')
    setDish(''); setNote(''); setWanted(''); setBusy(false)
  }

  const items = list.data?.items || []
  const pending = items.filter(i => i.status === 'pending')
  const decided = items.filter(i => i.status !== 'pending').slice(0, 8)
  const dayOptions = days.map(d => <option key={d.date} value={d.date}>{d.day} {fmt(d.date)}</option>)

  return (
    <section className="card" aria-labelledby="meal-req-h">
      <h2 id="meal-req-h">Meal requests</h2>
      <p className="muted meal-note">
        Want something specific? Ask for it here. {manager ? 'You decide what goes on the menu.' : 'A parent or the family admin decides.'}
      </p>
      <form className="meal-add" onSubmit={submit}>
        <label className="sr-only" htmlFor="meal-req-dish">Dish</label>
        <input id="meal-req-dish" value={dish} onChange={e => setDish(e.target.value)} placeholder="What would you like to eat? e.g. Nasi goreng" maxLength={100} />
        <label className="sr-only" htmlFor="meal-req-note">Note</label>
        <input id="meal-req-note" value={note} onChange={e => setNote(e.target.value)} placeholder="Note (optional)" maxLength={300} />
        <label className="sr-only" htmlFor="meal-req-day">Day</label>
        <select id="meal-req-day" value={wanted} onChange={e => setWanted(e.target.value)}>
          <option value="">Any day</option>
          {dayOptions}
        </select>
        <button type="submit" disabled={busy || !dish.trim()}>Ask</button>
      </form>
      {list.error && <div className="error-box">{list.error}</div>}

      <h3 className="meal-req-h">Waiting for a decision</h3>
      {pending.length === 0 && <p className="muted">No requests waiting.</p>}
      <ul className="meal-req-list">
        {pending.map(p => (
          <li key={p.id} className="meal-req">
            <div className="meal-req-main">
              <strong>{p.dish}</strong>
              <span className="muted">
                {p.member_name || 'Someone'}{p.wanted_date ? ` · would like it on ${fmt(p.wanted_date)}` : ''}{p.note ? ` · ${p.note}` : ''}
              </span>
            </div>
            <div className="meal-req-actions">
              {p.can_decide && (
                <>
                  <select aria-label={`Day for ${p.dish}`} value={pick[p.id] ?? p.wanted_date ?? ''} onChange={e => setPick(s => ({ ...s, [p.id]: e.target.value }))}>
                    <option value="">No fixed day</option>
                    {dayOptions}
                  </select>
                  <button className="small" onClick={() => run(() => mealProposalsApi.approve(p.id, { meal_date: pick[p.id] ?? p.wanted_date ?? null }), `Added "${p.dish}"`)}>
                    <IconCheck width={14} height={14} aria-hidden="true" /> Approve
                  </button>
                  <button className="secondary small" onClick={() => run(() => mealProposalsApi.decline(p.id), 'Declined')}>Decline</button>
                </>
              )}
              {!p.can_decide && p.can_delete && (
                <button className="icon-btn" aria-label={`Withdraw ${p.dish}`} title="Withdraw" onClick={() => run(() => mealProposalsApi.remove(p.id), 'Request withdrawn')}><IconClose width={14} height={14} /></button>
              )}
            </div>
          </li>
        ))}
      </ul>

      {decided.length > 0 && (
        <>
          <h3 className="meal-req-h">Recently decided</h3>
          <ul className="meal-req-list">
            {decided.map(p => (
              <li key={p.id} className="meal-req">
                <div className="meal-req-main">
                  <strong>{p.dish}</strong>
                  <span className="muted">
                    {p.member_name || 'Someone'} · {p.status === 'approved' ? `approved${p.meal_date ? ` for ${fmt(p.meal_date)}` : ''}` : 'declined'}{p.decided_by_name ? ` by ${p.decided_by_name}` : ''}{p.decision_note ? ` · ${p.decision_note}` : ''}
                  </span>
                </div>
                <span className={`badge ${p.status === 'approved' ? 'badge-savings' : ''}`}>{p.status === 'approved' ? 'Approved' : 'Declined'}</span>
                {p.can_delete && <button className="icon-btn" aria-label={`Delete ${p.dish}`} onClick={() => run(() => mealProposalsApi.remove(p.id))}><IconClose width={14} height={14} /></button>}
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  )
}
