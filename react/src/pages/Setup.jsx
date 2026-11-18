import { useState } from 'react'
import { api } from '../api'
import { useSetup } from '../useApi'
import { useToast, useHiddenIds } from '../Toast.jsx'
import { categoryColor } from '../categoryColor'
import { IconClose, IconEdit } from '../icons.jsx'
import { useAuth, canManage } from '../AuthContext.jsx'
import PageHeader from '../PageHeader.jsx'
import { SkeletonCard } from '../Skeleton.jsx'

const MAIN_CATS = ['Needs', 'Wants', 'Savings']
const CURRENCY_OPTIONS = ['IDR', 'USD', 'EUR', 'GBP', 'SGD', 'MYR', 'JPY', 'AUD', 'INR']

const isInUse = e => e?.status === 409 || /in use/i.test(e?.message || '')

/**
 * Undo-able delete for master data. Waits 5s (Undo cancels); if the server
 * then says the item is in use, offer "Force delete" from the error toast.
 */
function useUndoableDelete({ toast, hide, unhide, onDelete, onChanged, canForce = true }) {
  return function remove(item, label = item.name) {
    hide(item.id)
    toast.undoable({
      message: `Deleted "${label}"`,
      commit: () => onDelete(item.id, false),
      onUndo: () => unhide(item.id),
      onCommitted: onChanged,
      onError: e => {
        unhide(item.id)
        if (!isInUse(e)) { toast.error(e.message); return }
        if (!canForce) { toast.error(`"${label}" is in use and can't be deleted.`); return }
        toast.show({
          type: 'error',
          duration: 10000,
          message: `"${label}" is in use - existing records keep it as text if you force delete.`,
          action: {
            label: 'Force delete',
            onClick: async () => {
              try { await onDelete(item.id, true); toast.success(`Deleted "${label}"`); onChanged() } catch (e2) { toast.error(e2.message) }
            },
          },
        })
      },
    })
  }
}

function AccountSettings() {
  const { user, setUserCurrency, refreshUser } = useAuth()
  const toast = useToast()
  const [currency, setCurrencyLocal] = useState(user?.currency || 'IDR')
  const [household, setHousehold] = useState(user?.household_name || '')
  const isOwner = user?.role === 'owner'
  const manager = canManage(user)

  async function saveHousehold(e) {
    e.preventDefault()
    try {
      const d = await api.updateHousehold({ name: household.trim() })
      setHousehold(d.name)
      await refreshUser()
      toast.success('Household name saved')
    } catch (err) { toast.error(err.message) }
  }

  async function handleChange(e) {
    const code = e.target.value
    const prev = currency
    setCurrencyLocal(code)
    try {
      await api.updateSettings({ currency: code })
      setUserCurrency(code)
      toast.success(`Display currency set to ${code}`)
    } catch (err) {
      setCurrencyLocal(prev)
      toast.error(err.message)
    }
  }

  return (
    <div className="card">
      <h2>Account Settings</h2>
      <p className="muted">
        Display currency - amounts are stored in IDR and converted to the selected currency for display
        using daily exchange rates, so shown values are approximate. Your stored data never changes.
      </p>
      {/* --- family: household name (owner only) --- */}
      <form className="inline-form" onSubmit={saveHousehold} style={{ marginBottom: '1rem' }}>
        <label style={{ display: 'flex', flexDirection: 'column', gap: '.3rem', flex: '1 1 240px' }}>
          <span>Household name</span>
          <input value={household} onChange={e => setHousehold(e.target.value)} disabled={!isOwner} maxLength={80}
            placeholder="e.g. The Smiths" title={isOwner ? undefined : 'Only the family admin can rename the household'} />
        </label>
        {isOwner && <button type="submit" style={{ alignSelf: 'flex-end' }}>Save</button>}
      </form>
      <label style={{ display: 'flex', flexDirection: 'column', gap: '.3rem', maxWidth: '220px' }}>
        <span>Currency</span>
        <select value={currency} onChange={handleChange} disabled={!manager} title={manager ? undefined : 'Only the family admin and parents can change the household currency'}>
          {CURRENCY_OPTIONS.map(c => <option key={c} value={c}>{c}</option>)}
        </select>
      </label>
    </div>
  )
}

function EditableList({ title, items, onAdd, onUpdate, onDelete, onChanged, extraLabel, dotColor, canForce = true }) {
  const toast = useToast()
  const [hidden, hide, unhide] = useHiddenIds()
  const [name, setName] = useState('')
  const [editing, setEditing] = useState(null)
  const remove = useUndoableDelete({ toast, hide, unhide, onDelete, onChanged, canForce })

  async function handleAdd(e) {
    e.preventDefault()
    if (!name.trim()) return
    try {
      await onAdd(name.trim())
      toast.success(`Added "${name.trim()}"`)
      setName('')
      onChanged()
    } catch (e) { toast.error(e.message) }
  }

  async function handleSave() {
    try {
      await onUpdate(editing.id, editing.name)
      setEditing(null)
      toast.success('Saved')
      onChanged()
    } catch (e) { toast.error(e.message) }
  }

  const visible = items.filter(i => !hidden.has(i.id))

  return (
    <div className="card">
      <h2>{title}</h2>
      {extraLabel && <p className="muted">{extraLabel}</p>}
      <form className="inline-form" onSubmit={handleAdd}>
        <input placeholder="New name..." aria-label={`New ${title} name`} value={name} onChange={e => setName(e.target.value)} />
        <button type="submit">Add</button>
      </form>
      <div className="category-list" style={{ marginTop: '.9rem' }}>
        {visible.map(item => (
          <div className="category-chip" key={item.id}>
            {dotColor && <span className="cat-dot" style={{ background: dotColor(item.name) }} />}
            {editing?.id === item.id ? (
              <>
                <input aria-label="Name" value={editing.name} onChange={e => setEditing({ ...editing, name: e.target.value })} autoFocus
                  onKeyDown={e => { if (e.key === 'Enter') handleSave(); if (e.key === 'Escape') setEditing(null) }} />
                <button onClick={handleSave}>Save</button>
                <button className="secondary" onClick={() => setEditing(null)}>Cancel</button>
              </>
            ) : (
              <>
                <span>{item.name}</span>
                <button className="icon-btn" onClick={() => setEditing({ id: item.id, name: item.name })} aria-label={`Rename "${item.name}"`} title="Rename"><IconEdit width={13} height={13} /></button>
                <button className="icon-btn danger round" onClick={() => remove(item)} aria-label={`Delete "${item.name}"`} title="Delete"><IconClose width={12} height={12} /></button>
              </>
            )}
          </div>
        ))}
        {visible.length === 0 && <div className="muted">No data yet.</div>}
      </div>
    </div>
  )
}

export default function Setup() {
  const { setup, error, reload } = useSetup()
  const toast = useToast()
  const [hiddenCats, hideCat, unhideCat] = useHiddenIds()
  const [catName, setCatName] = useState('')
  const [catMain, setCatMain] = useState('Needs')
  const [catEditing, setCatEditing] = useState(null)
  const removeCategory = useUndoableDelete({
    toast, hide: hideCat, unhide: unhideCat,
    onDelete: (id, force) => api.deleteCategory(id, force),
    onChanged: reload,
  })

  async function handleAddCategory(e) {
    e.preventDefault()
    if (!catName.trim()) return
    try {
      await api.addCategory({ name: catName.trim(), main_category: catMain })
      toast.success(`Added category "${catName.trim()}"`)
      setCatName('')
      reload()
    } catch (e) { toast.error(e.message) }
  }

  async function handleSaveCategory() {
    try {
      await api.updateCategory(catEditing.id, { name: catEditing.name, main_category: catEditing.main_category })
      setCatEditing(null)
      toast.success('Category saved')
      reload()
    } catch (e) { toast.error(e.message) }
  }

  const header = <PageHeader title="Setup" subtitle="Manage master data: income sources, asset accounts, and spending categories." />

  if (!setup) {
    return (
      <div>
        {header}
        {error ? <div className="error-box">{error}</div> : <><SkeletonCard lines={2} /><SkeletonCard lines={3} /><SkeletonCard lines={5} /></>}
      </div>
    )
  }

  const assetAccounts = setup.accounts.filter(a => a.kind === 'asset')
  const groupedCats = MAIN_CATS.map(mc => ({ mc, items: setup.categories.filter(c => c.main_category === mc && !hiddenCats.has(c.id)) }))

  return (
    <div>
      {header}
      {error && <div className="error-box">{error}</div>}

      <AccountSettings />

      <EditableList
        title="Income Sources (Budgeting)"
        extraLabel="Salary/income sources for the Budgeting page"
        items={setup.income_sources}
        onAdd={(name) => api.addIncomeSource({ name })}
        onUpdate={(id, name) => api.updateIncomeSource(id, { name })}
        onDelete={(id, force) => api.deleteIncomeSource(id, force)}
        onChanged={reload}
      />

      <EditableList
        title="Asset Accounts (Assets)"
        extraLabel="Used to tag sources on the Assets page"
        items={assetAccounts}
        onAdd={(name) => api.addAccount({ name, kind: 'asset' })}
        onUpdate={(id, name) => api.updateAccount(id, { name })}
        onDelete={(id) => api.deleteAccount(id)}
        onChanged={reload}
        canForce={false}
      />

      <div className="card">
        <h2>Spending Categories</h2>
        <p className="muted">Add/remove your own categories - colors stay consistent across all pages.</p>
        <form className="inline-form" onSubmit={handleAddCategory}>
          <input placeholder="Category name" aria-label="Category name" value={catName} onChange={e => setCatName(e.target.value)} />
          <select aria-label="Group" value={catMain} onChange={e => setCatMain(e.target.value)}>
            {MAIN_CATS.map(m => <option key={m} value={m}>{m}</option>)}
          </select>
          <button type="submit">Add</button>
        </form>

        {groupedCats.map(({ mc, items }) => (
          <div key={mc} style={{ marginTop: '1rem' }}>
            <span className={`badge badge-${mc.toLowerCase()}`}>{mc}</span>
            <div className="category-list" style={{ marginTop: '.6rem' }}>
              {items.map(cat => (
                <div className="category-chip" key={cat.id}>
                  <span className="cat-dot" style={{ background: categoryColor(cat.name) }} />
                  {catEditing?.id === cat.id ? (
                    <>
                      <input aria-label="Category name" value={catEditing.name} onChange={e => setCatEditing({ ...catEditing, name: e.target.value })} autoFocus
                        onKeyDown={e => { if (e.key === 'Enter') handleSaveCategory(); if (e.key === 'Escape') setCatEditing(null) }} />
                      <select aria-label="Group" value={catEditing.main_category} onChange={e => setCatEditing({ ...catEditing, main_category: e.target.value })}>
                        {MAIN_CATS.map(m => <option key={m} value={m}>{m}</option>)}
                      </select>
                      <button onClick={handleSaveCategory}>Save</button>
                      <button className="secondary" onClick={() => setCatEditing(null)}>Cancel</button>
                    </>
                  ) : (
                    <>
                      <span>{cat.name}</span>
                      <button className="icon-btn" onClick={() => setCatEditing({ id: cat.id, name: cat.name, main_category: cat.main_category })} aria-label={`Edit category "${cat.name}"`} title="Edit"><IconEdit width={13} height={13} /></button>
                      <button className="icon-btn danger round" onClick={() => removeCategory(cat)} aria-label={`Delete category "${cat.name}"`} title="Delete"><IconClose width={12} height={12} /></button>
                    </>
                  )}
                </div>
              ))}
              {items.length === 0 && <div className="muted">No {mc} categories yet.</div>}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
