import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api, formatRupiah } from '../api'
import { useApi, useSetup } from '../useApi'
import { useToast, useHiddenIds } from '../Toast.jsx'
import Modal from '../Modal.jsx'
import MoneyInput from '../MoneyInput.jsx'
import PageHeader from '../PageHeader.jsx'
import { SkeletonCard } from '../Skeleton.jsx'
import { IconClose, IconEdit, IconPlus } from '../icons.jsx'
import { prettyDate } from './Recurring.jsx'

const COLORS = ['#4f46e5', '#16a34a', '#d97706', '#db2777', '#0891b2', '#7c3aed']

/** SVG progress ring. pct may exceed 100 (ring stays full). */
export function GoalRing({ pct, color = 'var(--primary)', size = 84, stroke = 8, label }) {
  const r = (size - stroke) / 2
  const c = 2 * Math.PI * r
  const clamped = Math.max(0, Math.min(100, pct || 0))
  return (
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} className="goal-ring" role="img" aria-label={`${Math.round(pct || 0)}% saved`}>
      <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--border)" strokeWidth={stroke} />
      <circle
        cx={size / 2} cy={size / 2} r={r} fill="none" stroke={color} strokeWidth={stroke} strokeLinecap="round"
        strokeDasharray={c} strokeDashoffset={c * (1 - clamped / 100)} transform={`rotate(-90 ${size / 2} ${size / 2})`}
        style={{ transition: 'stroke-dashoffset .4s' }}
      />
      <text x="50%" y="50%" dominantBaseline="central" textAnchor="middle" className="goal-ring-text">{label ?? `${Math.round(pct || 0)}%`}</text>
    </svg>
  )
}

function GoalModal({ initial, categories, assets, onClose, onSaved }) {
  const toast = useToast()
  const [form, setForm] = useState(() => ({
    name: initial?.name || '', emoji: '', color: initial?.color || COLORS[0],
    target_amount: initial?.target_amount ?? '', target_date: initial?.target_date || '',
    saved_amount: initial?.saved_amount ?? 0, category_id: initial?.category_id ?? '', asset_ids: initial?.asset_ids || [],
  }))
  const [error, setError] = useState('')
  const set = (k, v) => setForm(f => ({ ...f, [k]: v }))
  const savingsCats = categories.filter(c => c.main_category === 'Savings')
  const otherCats = categories.filter(c => c.main_category !== 'Savings')

  async function submit(e) {
    e.preventDefault()
    if (!form.name.trim() || !(Number(form.target_amount) > 0)) { setError('Name and a positive target are required.'); return }
    const payload = {
      ...form,
      name: form.name.trim(),
      target_amount: Number(form.target_amount),
      saved_amount: Number(form.saved_amount) || 0,
      target_date: form.target_date || null,
      category_id: form.category_id ? Number(form.category_id) : null,
    }
    try {
      if (initial) await api.updateGoal(initial.id, payload)
      else await api.addGoal(payload)
      toast.success(initial ? 'Goal saved' : `Goal "${payload.name}" created`)
      onSaved()
    } catch (err) { setError(err.message) }
  }

  function toggleAsset(id) {
    set('asset_ids', form.asset_ids.includes(id) ? form.asset_ids.filter(x => x !== id) : [...form.asset_ids, id])
  }

  return (
    <Modal title={initial ? 'Edit goal' : 'New savings goal'} onClose={onClose}>
      <form className="money-form" onSubmit={submit} noValidate>
        {error && <div className="error-box" role="alert">{error}</div>}
        <label className="span-2">Name
          <input data-autofocus value={form.name} onChange={e => set('name', e.target.value)} placeholder="Emergency fund, holiday, new laptop..." required />
        </label>
        <label>Target amount<MoneyInput value={form.target_amount} onChange={v => set('target_amount', v)} placeholder="e.g. 20jt" required /></label>
        <label>Target date <span className="muted">(optional)</span><input type="date" value={form.target_date} onChange={e => set('target_date', e.target.value)} /></label>
        <label>Already saved<MoneyInput value={form.saved_amount} onChange={v => set('saved_amount', v)} /></label>
        <label>Count spending in category
          <select value={form.category_id} onChange={e => set('category_id', e.target.value)}>
            <option value="">None</option>
            {savingsCats.length > 0 && <optgroup label="Savings">{savingsCats.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}</optgroup>}
            {otherCats.length > 0 && <optgroup label="Other">{otherCats.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}</optgroup>}
          </select>
        </label>
        {assets.length > 0 && (
          <fieldset className="span-2 goal-assets">
            <legend>Count these assets <span className="muted">(their current value adds to progress)</span></legend>
            {assets.map(a => (
              <label key={a.id} className="checkbox-label">
                <input type="checkbox" checked={form.asset_ids.includes(a.id)} onChange={() => toggleAsset(a.id)} /> {a.description} <span className="muted">{formatRupiah(a.value)}</span>
              </label>
            ))}
          </fieldset>
        )}
        <div className="span-2 goal-colors" role="radiogroup" aria-label="Color">
          {COLORS.map(c => (
            <button type="button" key={c} role="radio" aria-checked={form.color === c} aria-label={`Color ${c}`}
              className={`goal-color ${form.color === c ? 'active' : ''}`} style={{ background: c }} onClick={() => set('color', c)} />
          ))}
        </div>
        <p className="muted form-hint span-2">Progress = already saved + contributions + spending logged in the linked category since the goal was created + linked asset values.</p>
        <div className="money-form-actions span-2">
          <button type="button" className="secondary" onClick={onClose}>Cancel</button>
          <button type="submit">{initial ? 'Save' : 'Create goal'}</button>
        </div>
      </form>
    </Modal>
  )
}

function GoalCard({ goal, onEdit, onDelete, onContribute }) {
  const [amount, setAmount] = useState('')
  const done = goal.remaining <= 0
  return (
    <div className="card goal-card" style={{ borderTopColor: goal.color || 'var(--primary)' }}>
      <div className="goal-card-top">
        <GoalRing pct={goal.pct} color={goal.color || 'var(--primary)'} />
        <div className="goal-card-info">
          <h2>{goal.name}</h2>
          <div className="goal-amounts"><strong>{formatRupiah(goal.progress)}</strong> <span className="muted">of {formatRupiah(goal.target_amount)}</span></div>
          {done ? (
            <div className="text-positive">Goal reached!</div>
          ) : goal.monthly_needed != null ? (
            <div className="muted">Save <strong>{formatRupiah(goal.monthly_needed)}</strong>/month for {goal.months_left} month{goal.months_left === 1 ? '' : 's'} to reach it by {prettyDate(goal.target_date)}</div>
          ) : (
            <div className="muted">{formatRupiah(goal.remaining)} to go · no target date</div>
          )}
          {(goal.category_name || goal.asset_ids.length > 0) && (
            <div className="muted goal-links">
              {goal.category_name && <>Includes "{goal.category_name}" spending</>}
              {goal.category_name && goal.asset_ids.length > 0 && ' · '}
              {goal.asset_ids.length > 0 && <>{goal.asset_ids.length} linked asset{goal.asset_ids.length === 1 ? '' : 's'}</>}
              {goal.linked_amount > 0 && <> ({formatRupiah(goal.linked_amount)})</>}
            </div>
          )}
        </div>
      </div>
      <form className="inline-form goal-contribute" onSubmit={e => { e.preventDefault(); if (Number(amount)) { onContribute(goal, Number(amount)); setAmount('') } }}>
        <MoneyInput value={amount} onChange={setAmount} placeholder="Add money (e.g. 500k)" aria-label={`Contribute to ${goal.name}`} />
        <button type="submit" disabled={!Number(amount)}>Add</button>
        <span className="goal-card-actions">
          <button type="button" className="icon-btn" onClick={() => onEdit(goal)} aria-label={`Edit goal ${goal.name}`} title="Edit"><IconEdit width={14} height={14} /></button>
          <button type="button" className="icon-btn danger round" onClick={() => onDelete(goal)} aria-label={`Delete goal ${goal.name}`} title="Delete"><IconClose width={14} height={14} /></button>
        </span>
      </form>
    </div>
  )
}

/** Compact "top goals" card for other pages (Assets). */
export function TopGoalsCard({ limit = 3 }) {
  const { data, error } = useApi(() => api.goals(), [])
  if (error) return null
  const goals = [...(data || [])].sort((a, b) => (a.remaining <= 0) - (b.remaining <= 0) || b.pct - a.pct).slice(0, limit)
  return (
    <div className="card">
      <h2 className="card-title-row">Savings goals <Link to="/savings/goals" className="link-btn">All goals</Link></h2>
      {!data && <div className="muted">Loading...</div>}
      {data && goals.length === 0 && <div className="muted">No goals yet. <Link to="/savings/goals">Create one</Link> to track what you're saving for.</div>}
      <div className="top-goals">
        {goals.map(g => (
          <div className="top-goal" key={g.id}>
            <GoalRing pct={g.pct} color={g.color || 'var(--primary)'} size={56} stroke={6} />
            <div>
              <div><strong>{g.name}</strong></div>
              <div className="muted">{formatRupiah(g.progress)} of {formatRupiah(g.target_amount)}</div>
              {g.monthly_needed != null && g.remaining > 0 && <div className="muted">{formatRupiah(g.monthly_needed)}/month</div>}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

export default function Goals() {
  const res = useApi(() => api.goals(), [])
  const assetsRes = useApi(() => api.assets(), [])
  const { categories } = useSetup()
  const toast = useToast()
  const [hidden, hide, unhide] = useHiddenIds()
  const [editing, setEditing] = useState(null)

  const goals = (res.data || []).filter(g => !hidden.has(g.id))
  const totalTarget = goals.reduce((s, g) => s + g.target_amount, 0)
  const totalProgress = goals.reduce((s, g) => s + Math.min(g.progress, g.target_amount), 0)

  async function contribute(goal, amount) {
    try {
      await api.contributeGoal(goal.id, amount)
      toast.success(`${amount > 0 ? 'Added' : 'Withdrew'} ${formatRupiah(Math.abs(amount))} ${amount > 0 ? 'to' : 'from'} "${goal.name}"`)
      res.reload()
    } catch (e) { toast.error(e.message) }
  }

  function remove(goal) {
    hide(goal.id)
    toast.undoable({
      message: `Deleted goal "${goal.name}"`,
      commit: () => api.deleteGoal(goal.id),
      onUndo: () => unhide(goal.id),
      onCommitted: res.reload,
    })
  }

  return (
    <div>
      <PageHeader
        title="Goals"
        subtitle={goals.length ? `${formatRupiah(totalProgress)} saved of ${formatRupiah(totalTarget)} across ${goals.length} goal${goals.length === 1 ? '' : 's'}` : 'What are you saving for?'}
        actions={<button onClick={() => setEditing('new')}><IconPlus width={14} height={14} aria-hidden="true" /> New goal</button>}
      />
      {res.error && <div className="error-box">{res.error}</div>}
      {!res.data && !res.error && <div className="goal-grid"><SkeletonCard lines={3} /><SkeletonCard lines={3} /></div>}
      {res.data && goals.length === 0 && (
        <div className="card empty-hint">
          <p>No savings goals yet.</p>
          <p className="muted">Set a target (and optionally a date) and Kinkit shows how much to put aside each month.</p>
          <button onClick={() => setEditing('new')}>Create a goal</button>
        </div>
      )}
      <div className="goal-grid">
        {goals.map(g => <GoalCard key={g.id} goal={g} onEdit={setEditing} onDelete={remove} onContribute={contribute} />)}
      </div>
      {editing && (
        <GoalModal
          initial={editing === 'new' ? null : editing}
          categories={categories}
          assets={assetsRes.data?.assets || []}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); res.reload() }}
        />
      )}
    </div>
  )
}
