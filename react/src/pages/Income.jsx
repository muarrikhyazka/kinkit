import { useEffect, useState } from 'react'
import { api, formatRupiah, todayISO, MONTH_NAMES } from '../api'
import { useYearMonth } from '../YearContext.jsx'
import { useApi, useSetup } from '../useApi'
import { useToast, useHiddenIds } from '../Toast.jsx'
import Modal from '../Modal.jsx'
import MoneyInput from '../MoneyInput.jsx'
import PageHeader from '../PageHeader.jsx'
import StatCard from '../StatCard.jsx'
import { SkeletonStats, SkeletonCard } from '../Skeleton.jsx'
import { IconClose, IconEdit } from '../icons.jsx'
import { prettyDate } from './Recurring.jsx'
import { useAuth, canManage } from '../AuthContext.jsx'

/** Default date for a new entry: today if viewing the current UTC month, else the 1st of the viewed month. */
function defaultDate(year, month) {
  const today = todayISO()
  const prefix = `${year}-${String(month).padStart(2, '0')}`
  return today.startsWith(prefix) ? today : `${prefix}-01`
}

function EditIncomeModal({ entry, sources, onClose, onSaved }) {
  const toast = useToast()
  const [form, setForm] = useState({ tx_date: entry.tx_date, source_id: entry.source_id ?? '', description: entry.description, amount: entry.amount })
  const [error, setError] = useState('')
  const set = (k, v) => setForm(f => ({ ...f, [k]: v }))

  async function submit(e) {
    e.preventDefault()
    try {
      await api.updateIncome(entry.id, { ...form, source_id: form.source_id ? Number(form.source_id) : null, amount: Number(form.amount) })
      toast.success('Income updated')
      onSaved()
    } catch (err) { setError(err.message) }
  }

  return (
    <Modal title="Edit income" onClose={onClose}>
      <form className="money-form" onSubmit={submit}>
        {error && <div className="error-box" role="alert">{error}</div>}
        <label>Date<input type="date" value={form.tx_date} onChange={e => set('tx_date', e.target.value)} required /></label>
        <label>Source
          <select value={form.source_id} onChange={e => set('source_id', e.target.value)}>
            <option value="">No source</option>
            {sources.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
        </label>
        <label>Description<input data-autofocus value={form.description} onChange={e => set('description', e.target.value)} /></label>
        <label>Amount<MoneyInput value={form.amount} onChange={v => set('amount', v)} required /></label>
        <div className="money-form-actions span-2">
          <button type="button" className="secondary" onClick={onClose}>Cancel</button>
          <button type="submit">Save</button>
        </div>
      </form>
    </Modal>
  )
}

// Income saved anywhere (page form or the quick-add modal) refreshes the Income page.
const INCOME_EVENT = 'kinkit:income-changed'
export const onIncomeChanged = fn => {
  window.addEventListener(INCOME_EVENT, fn)
  return () => window.removeEventListener(INCOME_EVENT, fn)
}

/** Add-income form, used on the Income page and in the quick-add modal. */
export function IncomeForm({ initialDate, onSaved, autoFocus, stacked }) {
  const { incomeSources } = useSetup()
  const toast = useToast()
  const [form, setForm] = useState({ tx_date: '', source_id: '', description: '', amount: '' })
  const [saving, setSaving] = useState(false)
  const set = (k, v) => setForm(f => ({ ...f, [k]: v }))
  const date = form.tx_date || initialDate || todayISO()

  async function add(e) {
    e.preventDefault()
    if (!(Number(form.amount) > 0)) return
    const payload = {
      tx_date: date,
      source_id: form.source_id ? Number(form.source_id) : null,
      description: form.description.trim(),
      amount: Number(form.amount),
      client_id: `web:${Date.now()}:${Math.random().toString(36).slice(2, 8)}`,
    }
    setSaving(true)
    try {
      await api.addIncome(payload)
      setForm(f => ({ ...f, description: '', amount: '' }))
      toast.success(`Added ${formatRupiah(payload.amount)} income`)
      window.dispatchEvent(new Event(INCOME_EVENT))
      onSaved && onSaved(payload)
    } catch (err) { toast.error(err.message) } finally { setSaving(false) }
  }

  return (
    <form className={stacked ? 'stacked-form' : 'card inline-form'} onSubmit={add} aria-label="Add income">
      <input type="date" aria-label="Date" value={date} onChange={e => set('tx_date', e.target.value)} />
      <select aria-label="Income source" value={form.source_id} onChange={e => set('source_id', e.target.value)} data-autofocus={autoFocus ? '' : undefined}>
        <option value="">Source...</option>
        {incomeSources.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
      </select>
      <input placeholder="Description (optional)" aria-label="Description" value={form.description} onChange={e => set('description', e.target.value)} />
      <MoneyInput placeholder="Amount (e.g. 10m)" aria-label="Amount" value={form.amount} onChange={v => set('amount', v)} required />
      <button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Add income'}</button>
    </form>
  )
}

export default function Income() {
  const { year, month } = useYearMonth()
  const { incomeSources } = useSetup()
  const toast = useToast()
  const { user } = useAuth()
  const res = useApi(() => api.income(year, month), [year, month])
  const [hidden, hide, unhide] = useHiddenIds()
  const [editing, setEditing] = useState(null)
  useEffect(() => onIncomeChanged(res.reload), [res.reload])

  const data = res.data
  const rows = (data?.rows || []).filter(r => !hidden.has(r.id))
  const total = rows.reduce((s, r) => s + r.amount, 0)
  const planned = data?.planned || 0
  const pct = planned > 0 ? Math.round((total / planned) * 100) : null

  function remove(r) {
    hide(r.id)
    toast.undoable({
      message: `Deleted income ${formatRupiah(r.amount)}`,
      commit: () => api.deleteIncome(r.id),
      onUndo: () => unhide(r.id),
      onCommitted: res.reload,
    })
  }

  const header = <PageHeader title="Income" subtitle={`Money actually received, ${MONTH_NAMES[month - 1]} ${year}`} period="month" />

  if (!data) {
    return <div>{header}{res.error ? <div className="error-box">{res.error}</div> : <><SkeletonStats count={3} /><SkeletonCard lines={4} /></>}</div>
  }

  return (
    <div>
      {header}
      {res.error && <div className="error-box">{res.error}</div>}
      <div className="cards-row">
        <StatCard label="Received" value={formatRupiah(total)} tone="income" sub={`${rows.length} entr${rows.length === 1 ? 'y' : 'ies'}`} />
        <StatCard label="Planned (Budgeting)" value={formatRupiah(planned)} sub={pct === null ? 'No income planned this month' : `${pct}% of plan received`} />
        <StatCard label="Difference" value={formatRupiah(total - planned)} tone={total - planned >= 0 ? 'positive' : 'negative'} sub={total >= planned ? 'At or above plan' : 'Still expected'} />
      </div>

      {canManage(user) && <IncomeForm initialDate={defaultDate(year, month)} />}

      <div className="grid-2">
        <div className="card table-scroll">
          <h2>Entries</h2>
          <table className="responsive-table">
            <thead><tr><th>Date</th><th>Source</th><th>Description</th><th>Amount</th><th><span className="sr-only">Actions</span></th></tr></thead>
            <tbody>
              {rows.map(r => (
                <tr key={r.id}>
                  <td data-label="Date">{prettyDate(r.tx_date)}</td>
                  <td data-label="Source">{r.source_name || <span className="muted">-</span>}</td>
                  <td data-label="Description">{r.description || <span className="muted">-</span>}</td>
                  <td data-label="Amount" className="num text-positive">{formatRupiah(r.amount)}</td>
                  <td className="row-actions">
                    <button className="icon-btn" onClick={() => setEditing(r)} aria-label={`Edit income of ${formatRupiah(r.amount)}`} title="Edit"><IconEdit width={14} height={14} /></button>
                    <button className="icon-btn danger round" onClick={() => remove(r)} aria-label={`Delete income of ${formatRupiah(r.amount)}`} title="Delete"><IconClose width={14} height={14} /></button>
                  </td>
                </tr>
              ))}
              {rows.length === 0 && <tr><td colSpan={5} className="muted">No income recorded for this month yet.</td></tr>}
            </tbody>
          </table>
        </div>
        <div className="card">
          <h2>By source</h2>
          {data.by_source.length === 0 && <div className="muted">Nothing yet.</div>}
          {data.by_source.map(s => (
            <div className="bar-row" key={s.source_id ?? 'none'}>
              <div className="bar-label">{s.source_name || 'No source'}</div>
              <div className="progress-bar"><div className="progress-fill" style={{ width: `${data.total ? Math.round((s.total / data.total) * 100) : 0}%`, background: 'var(--success)' }} /></div>
              <div className="bar-value">{formatRupiah(s.total)}</div>
            </div>
          ))}
        </div>
      </div>

      {editing && <EditIncomeModal entry={editing} sources={incomeSources} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); res.reload() }} />}
    </div>
  )
}
