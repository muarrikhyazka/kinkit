import { useState } from 'react'
import { api, formatRupiah, todayISO, notifySpendingChanged } from '../api'
import { useApi, useSetup } from '../useApi'
import { useToast, useHiddenIds } from '../Toast.jsx'
import Modal from '../Modal.jsx'
import MoneyInput from '../MoneyInput.jsx'
import PageHeader from '../PageHeader.jsx'
import { SkeletonCard } from '../Skeleton.jsx'
import { IconClose, IconEdit, IconPlus } from '../icons.jsx'
import { IconPause, IconPlay } from '../moneyIcons.jsx'

const WEEKDAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
const MONTH_NAMES_SHORT = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

function ordinal(n) {
  const s = ['th', 'st', 'nd', 'rd']
  const v = n % 100
  return n + (s[(v - 20) % 10] || s[v] || s[0])
}

export function describeSchedule(r) {
  if (r.frequency === 'weekly') return `Every ${WEEKDAYS[r.weekday ?? 0]}`
  if (r.frequency === 'yearly') {
    const m = Number(r.start_date.slice(5, 7))
    return `Every year on ${MONTH_NAMES_SHORT[m - 1]} ${r.day_of_month}`
  }
  return `Monthly on the ${ordinal(r.day_of_month)}${r.day_of_month > 28 ? ' (or last day)' : ''}`
}

/** Dates are UTC ISO strings; render without local-timezone shifts. */
export function prettyDate(iso) {
  if (!iso) return '-'
  const [y, m, d] = iso.split('-').map(Number)
  return `${d} ${MONTH_NAMES_SHORT[m - 1]} ${y}`
}

function emptyForm() {
  const today = todayISO()
  return {
    description: '', category_id: '', amount: '', frequency: 'monthly',
    day_of_month: Number(today.slice(8, 10)), weekday: new Date().getUTCDay(), start_date: today, end_date: '',
  }
}

function RecurringModal({ initial, categories, onClose, onSaved }) {
  const toast = useToast()
  const [form, setForm] = useState(() => (initial
    ? {
        description: initial.description, category_id: initial.category_id ?? '', amount: initial.amount,
        frequency: initial.frequency, day_of_month: initial.day_of_month ?? 1, weekday: initial.weekday ?? 1,
        start_date: initial.start_date, end_date: initial.end_date ?? '',
      }
    : emptyForm()))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const set = (k, v) => setForm(f => ({ ...f, [k]: v }))

  async function submit(e) {
    e.preventDefault()
    if (!form.description.trim() || !(Number(form.amount) > 0)) { setError('Description and a positive amount are required.'); return }
    const payload = {
      description: form.description.trim(),
      category_id: form.category_id ? Number(form.category_id) : null,
      amount: Number(form.amount),
      frequency: form.frequency,
      start_date: form.start_date,
      end_date: form.end_date || null,
      ...(form.frequency === 'weekly' ? { weekday: Number(form.weekday) } : { day_of_month: Number(form.day_of_month) }),
    }
    setSaving(true)
    setError('')
    try {
      const res = initial ? await api.updateRecurring(initial.id, payload) : await api.addRecurring(payload)
      const n = res?.generated || 0
      toast.success(`${initial ? 'Saved' : 'Added'} "${payload.description}"${n ? `. ${n} transaction${n === 1 ? '' : 's'} created` : ''}`)
      if (n) notifySpendingChanged()
      onSaved()
    } catch (err) {
      setError(err.message)
      setSaving(false)
    }
  }

  return (
    <Modal title={initial ? 'Edit recurring transaction' : 'New recurring transaction'} onClose={onClose}>
      <form className="money-form" onSubmit={submit} noValidate>
        {error && <div className="error-box" role="alert">{error}</div>}
        <label className="span-2">Description
          <input data-autofocus value={form.description} onChange={e => set('description', e.target.value)} placeholder="Rent, Netflix, gym..." required />
        </label>
        <label>Amount
          <MoneyInput value={form.amount} onChange={v => set('amount', v)} placeholder="e.g. 1.5m" required />
        </label>
        <label>Category
          <select value={form.category_id} onChange={e => set('category_id', e.target.value)}>
            <option value="">No category</option>
            {categories.map(c => <option key={c.id} value={c.id}>{c.name} ({c.main_category})</option>)}
          </select>
        </label>
        <label>Repeats
          <select value={form.frequency} onChange={e => set('frequency', e.target.value)}>
            <option value="monthly">Monthly</option>
            <option value="weekly">Weekly</option>
            <option value="yearly">Yearly</option>
          </select>
        </label>
        {form.frequency === 'weekly' ? (
          <label>On
            <select value={form.weekday} onChange={e => set('weekday', e.target.value)}>
              {WEEKDAYS.map((w, i) => <option key={w} value={i}>{w}</option>)}
            </select>
          </label>
        ) : (
          <label>Day of month
            <input type="number" min={1} max={31} value={form.day_of_month} onChange={e => set('day_of_month', e.target.value)} />
          </label>
        )}
        <label>Starts
          <input type="date" value={form.start_date} onChange={e => set('start_date', e.target.value)} required />
        </label>
        <label>Ends <span className="muted">(optional)</span>
          <input type="date" value={form.end_date} min={form.start_date} onChange={e => set('end_date', e.target.value)} />
        </label>
        <p className="muted form-hint span-2">
          {form.frequency === 'yearly' && 'Yearly repeats in the start date\'s month. '}
          Days past the end of a month (e.g. 31) use the month's last day. Past occurrences since the start date are added right away. Dates are UTC.
        </p>
        <div className="money-form-actions span-2">
          <button type="button" className="secondary" onClick={onClose}>Cancel</button>
          <button type="submit" disabled={saving}>{saving ? 'Saving...' : initial ? 'Save' : 'Add'}</button>
        </div>
      </form>
    </Modal>
  )
}

export default function Recurring() {
  const res = useApi(() => api.recurring(), [])
  const { categories } = useSetup()
  const toast = useToast()
  const [hidden, hide, unhide] = useHiddenIds()
  const [editing, setEditing] = useState(null) // null | 'new' | row

  const rows = (res.data || []).filter(r => !hidden.has(r.id))
  const active = rows.filter(r => r.active)
  const monthlyTotal = active.reduce((s, r) => s + (r.frequency === 'weekly' ? (r.amount * 52) / 12 : r.frequency === 'yearly' ? r.amount / 12 : r.amount), 0)

  async function toggle(r) {
    try {
      const out = await api.updateRecurring(r.id, { active: !r.active })
      toast.success(r.active ? `Paused "${r.description}"` : `Resumed "${r.description}"`)
      if (out?.generated) notifySpendingChanged()
      res.reload()
    } catch (e) { toast.error(e.message) }
  }

  function remove(r) {
    hide(r.id)
    toast.undoable({
      message: `Deleted "${r.description}" (past transactions are kept)`,
      commit: () => api.deleteRecurring(r.id),
      onUndo: () => unhide(r.id),
      onCommitted: res.reload,
    })
  }

  const header = (
    <PageHeader
      title="Recurring"
      subtitle={res.data ? `${active.length} active · about ${formatRupiah(monthlyTotal)} / month` : 'Bills and subscriptions that repeat'}
      actions={<button onClick={() => setEditing('new')}><IconPlus width={14} height={14} aria-hidden="true" /> New</button>}
    />
  )

  return (
    <div>
      {header}
      {res.error && <div className="error-box">{res.error}</div>}
      {!res.data && !res.error && <SkeletonCard lines={4} />}
      {res.data && (
        <div className="card table-scroll">
          {rows.length === 0 ? (
            <div className="empty-hint">
              <p>No recurring transactions yet.</p>
              <p className="muted">Add rent, subscriptions or loan payments once and Kinkit logs them as spending on schedule.</p>
              <button onClick={() => setEditing('new')}>Add your first one</button>
            </div>
          ) : (
            <table className="responsive-table">
              <thead><tr><th>Description</th><th>Schedule</th><th>Amount</th><th>Next</th><th><span className="sr-only">Actions</span></th></tr></thead>
              <tbody>
                {rows.map(r => (
                  <tr key={r.id} className={r.active ? '' : 'row-paused'}>
                    <td data-label="Description">
                      {r.description}
                      <div className="muted">{r.category_name || 'No category'}</div>
                    </td>
                    <td data-label="Schedule">
                      {describeSchedule(r)}
                      {r.end_date && <div className="muted">until {prettyDate(r.end_date)}</div>}
                    </td>
                    <td data-label="Amount" className="num">{formatRupiah(r.amount)}</td>
                    <td data-label="Next">{r.active ? (r.next_date ? prettyDate(r.next_date) : <span className="muted">Ended</span>) : <span className="tag">Paused</span>}</td>
                    <td className="row-actions">
                      <button className="icon-btn" onClick={() => toggle(r)} aria-label={r.active ? `Pause "${r.description}"` : `Resume "${r.description}"`} title={r.active ? 'Pause' : 'Resume'}>
                        {r.active ? <IconPause width={14} height={14} /> : <IconPlay width={14} height={14} />}
                      </button>
                      <button className="icon-btn" onClick={() => setEditing(r)} aria-label={`Edit "${r.description}"`} title="Edit">
                        <IconEdit width={14} height={14} />
                      </button>
                      <button className="icon-btn danger round" onClick={() => remove(r)} aria-label={`Delete "${r.description}"`} title="Delete">
                        <IconClose width={14} height={14} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
      {res.data && rows.length > 0 && (
        <p className="muted form-hint">Generated transactions appear in Spending with source "recurring". Pausing stops new ones; resuming doesn't backfill the paused period.</p>
      )}
      {editing && (
        <RecurringModal
          initial={editing === 'new' ? null : editing}
          categories={categories}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); res.reload() }}
        />
      )}
    </div>
  )
}
