import { useState } from 'react'
import { api, formatRupiah } from '../api'
import { useApi, useSetup } from '../useApi'
import { useToast, useHiddenIds } from '../Toast.jsx'
import MoneyInput from '../MoneyInput.jsx'
import PageHeader from '../PageHeader.jsx'
import StatCard from '../StatCard.jsx'
import { SkeletonStats, SkeletonCard } from '../Skeleton.jsx'
import { IconClose, IconEdit } from '../icons.jsx'
import { TopGoalsCard } from './Goals.jsx'

// Module-level (not defined inside Assets) so its inputs aren't remounted on every render.
function AssetTable({ items, emptyLabel, onEditValue, onDelete }) {
  return (
    <div className="table-scroll">
      <table className="responsive-table">
        <thead><tr><th>Description</th><th>Account</th><th>Value</th><th><span className="sr-only">Actions</span></th></tr></thead>
        <tbody>
          {items.map(a => (
            <tr key={a.id}>
              <td data-label="Description">{a.description}</td>
              <td data-label="Account">{a.account_name || '-'}</td>
              <td data-label="Value"><MoneyInput value={a.value} aria-label={`Value of ${a.description}`} onBlur={v => { if ((Number(v) || 0) !== a.value) onEditValue(a.id, v) }} /></td>
              <td className="row-actions">
                <button className="icon-btn danger round" onClick={() => onDelete(a)} aria-label={`Delete asset "${a.description}"`} title="Delete">
                  <IconClose width={14} height={14} />
                </button>
              </td>
            </tr>
          ))}
          {items.length === 0 && <tr><td colSpan={4} className="muted">{emptyLabel}</td></tr>}
        </tbody>
      </table>
    </div>
  )
}

const emptyForm = { description: '', value: '', account_id: '', kind: 'liquid' }

export default function Assets() {
  const res = useApi(() => api.assets(), [])
  const { accounts: allAccounts } = useSetup()
  const toast = useToast()
  const [hidden, hide, unhide] = useHiddenIds()
  const [form, setForm] = useState(emptyForm)
  const [editingTarget, setEditingTarget] = useState(false)
  const [targetInput, setTargetInput] = useState('')

  const accounts = allAccounts.filter(a => a.kind === 'asset')
  const assets = (res.data?.assets || []).filter(a => !hidden.has(a.id))
  const target = res.data?.target
  const liquid = assets.filter(a => a.kind === 'liquid')
  const nonLiquid = assets.filter(a => a.kind === 'non_liquid')
  const liquidTotal = liquid.reduce((s, a) => s + a.value, 0)
  const nonLiquidTotal = nonLiquid.reduce((s, a) => s + a.value, 0)
  const total = liquidTotal + nonLiquidTotal
  const targetPct = target?.target_value > 0 ? Math.min(100, Math.round((total / target.target_value) * 100)) : 0

  async function handleAdd(e) {
    e.preventDefault()
    if (!form.description || form.value === '') return
    try {
      await api.addAsset({ ...form, value: Number(form.value), account_id: form.account_id || null })
      setForm(emptyForm)
      toast.success(`Added "${form.description}"`)
      res.reload()
    } catch (err) { toast.error(err.message) }
  }

  async function handleEditValue(id, value) {
    try {
      await api.updateAsset(id, { value: Number(value) || 0 })
      toast.success('Value updated')
      res.reload()
    } catch (err) { toast.error(err.message) }
  }

  function handleDelete(asset) {
    hide(asset.id)
    toast.undoable({
      message: `Deleted asset "${asset.description}"`,
      commit: () => api.deleteAsset(asset.id),
      onUndo: () => unhide(asset.id),
      onCommitted: res.reload,
    })
  }

  async function handleSaveTarget() {
    if (!targetInput) return
    try {
      await api.setAssetTarget({ target_value: Number(targetInput) })
      setEditingTarget(false)
      setTargetInput('')
      toast.success('Target saved')
      res.reload()
    } catch (e) { toast.error(e.message) }
  }

  const header = <PageHeader title="Assets" subtitle="Lifetime assets tracker" />

  if (!res.data) {
    return (
      <div>
        {header}
        {res.error ? <div className="error-box">{res.error}</div> : <><SkeletonStats count={4} /><SkeletonCard lines={4} /></>}
      </div>
    )
  }

  return (
    <div>
      {header}
      {res.error && <div className="error-box">{res.error}</div>}

      <div className="cards-row">
        <StatCard label="Total Assets Value" value={formatRupiah(total)} tone="savings" />
        <StatCard label="Liquid Assets" value={formatRupiah(liquidTotal)} tone="income" />
        <StatCard label="Non-Liquid Assets" value={formatRupiah(nonLiquidTotal)} tone="expense" />
        <div className="card stat">
          <div className="muted">
            Target {target?.last_update && <span>(update {target.last_update})</span>}
          </div>
          {editingTarget ? (
            <div className="inline-form" style={{ marginTop: '.4rem' }}>
              <MoneyInput placeholder="Target value" aria-label="Target value" value={targetInput} onChange={setTargetInput} />
              <button onClick={handleSaveTarget}>Save</button>
              <button className="secondary" onClick={() => setEditingTarget(false)}>Cancel</button>
            </div>
          ) : (
            <>
              <div className="stat-value">{target ? formatRupiah(target.target_value) : '-'}</div>
              <div className="progress-bar" role="progressbar" aria-valuenow={targetPct} aria-valuemin={0} aria-valuemax={100} aria-label="Progress to target">
                <div className="progress-fill" style={{ width: `${targetPct}%` }} />
              </div>
              <span className="progress-label">{targetPct}%</span>
              <div>
                <button className="icon-btn" onClick={() => { setEditingTarget(true); setTargetInput(target?.target_value || '') }}>
                  <IconEdit width={14} height={14} aria-hidden="true" /> edit target
                </button>
              </div>
            </>
          )}
        </div>
      </div>

      <TopGoalsCard />

      <form className="card inline-form" onSubmit={handleAdd}>
        <input placeholder="Description" aria-label="Asset description" value={form.description} onChange={e => setForm(f => ({ ...f, description: e.target.value }))} required />
        <MoneyInput placeholder="Value (e.g. 10m)" aria-label="Asset value" value={form.value} onChange={v => setForm(f => ({ ...f, value: v }))} required />
        <select aria-label="Account" value={form.account_id} onChange={e => setForm(f => ({ ...f, account_id: e.target.value }))}>
          <option value="">Account...</option>
          {accounts.map(a => <option key={a.id} value={a.id}>{a.name}</option>)}
        </select>
        <select aria-label="Kind" value={form.kind} onChange={e => setForm(f => ({ ...f, kind: e.target.value }))}>
          <option value="liquid">Liquid</option>
          <option value="non_liquid">Non-liquid</option>
        </select>
        <button type="submit">Add</button>
      </form>

      <div className="grid-2">
        <div className="card">
          <h2>Liquid Assets</h2>
          <AssetTable items={liquid} emptyLabel="No liquid assets yet." onEditValue={handleEditValue} onDelete={handleDelete} />
        </div>
        <div className="card">
          <h2>Non-Liquid Assets</h2>
          <AssetTable items={nonLiquid} emptyLabel="No non-liquid assets yet." onEditValue={handleEditValue} onDelete={handleDelete} />
        </div>
      </div>
    </div>
  )
}
