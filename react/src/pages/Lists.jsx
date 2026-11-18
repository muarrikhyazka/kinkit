import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import PageHeader from '../PageHeader.jsx'
import Modal from '../Modal.jsx'
import MoneyInput from '../MoneyInput.jsx'
import CategorySelect from '../CategorySelect.jsx'
import { SkeletonCard } from '../Skeleton.jsx'
import { useToast } from '../Toast.jsx'
import { useSetup, useDebounced } from '../useApi'
import { useAuth, canManage } from '../AuthContext.jsx'
import { listsApi, formatRupiah, isRetryableError, notifySpendingChanged } from '../api'
import { IconPlus, IconClose, IconEdit, IconCheck } from '../icons.jsx'

/**
 * Home › Lists: shared shopping lists, built for use in the store on a phone.
 * - chips switch lists (?list=<id>, also what Today links to)
 * - one big add box: "eggs, 2 milk, bread x3" or a pasted multi-line list,
 *   with suggestions from what the household bought before
 * - tap an item to put it in the cart; swipe left (or ×) to delete with undo
 * - "Finish shopping" logs ONE spending row for the trip and clears the cart
 * - polls every 15s while visible so the family sees each other's changes
 * - ticking items works offline: failed toggles are retried on the next poll
 */
const POLL_MS = 15000
const LAST_LIST_KEY = 'kinkit:lists:last'

function newClientId() {
  try { if (crypto?.randomUUID) return crypto.randomUUID() } catch { /* insecure context */ }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

function readLastList() {
  try { return Number(localStorage.getItem(LAST_LIST_KEY)) || null } catch { return null }
}
function writeLastList(id) {
  try { localStorage.setItem(LAST_LIST_KEY, String(id)) } catch { /* storage blocked */ }
}

// Client-side mirror of the backend's default trip category (lists.go
// defaultShoppingCategory) so the modal shows what will be used.
function defaultCategory(categories) {
  const tests = [/grocer/i, /belanja/i, /supermarket/i, /^food$/i, /food|makan/i, /^supplies$/i]
  for (const re of tests) {
    const c = categories.find(c => re.test(c.name))
    if (c) return c.name
  }
  return (categories.find(c => c.main_category === 'Needs') || categories[0])?.name || ''
}

export default function Lists() {
  const { user } = useAuth()
  const manager = canManage(user)
  const readOnly = user?.role === 'viewer'
  const toast = useToast()
  const { categories } = useSetup()
  const [params, setParams] = useSearchParams()

  const [lists, setLists] = useState(null)
  const [data, setData] = useState(null) // { list, items }
  const [error, setError] = useState('')
  const [hidden, setHidden] = useState(() => new Set())
  const [modal, setModal] = useState(null) // 'new-list' | 'edit-list' | 'finish' | {item}
  const pending = useRef(new Map()) // itemId -> checked (toggles not yet saved)

  const paramId = Number(params.get('list')) || null
  const activeId = useMemo(() => {
    if (!lists?.length) return null
    const want = paramId || readLastList()
    return (lists.find(l => l.id === want) || lists[0]).id
  }, [lists, paramId])

  const selectList = useCallback(id => {
    writeLastList(id)
    setParams(p => { const n = new URLSearchParams(p); n.set('list', String(id)); return n }, { replace: true })
  }, [setParams])

  const loadLists = useCallback(async () => {
    const d = await listsApi.lists()
    setLists(d.lists)
    return d.lists
  }, [])

  // Re-send toggles that failed (offline); keep them applied locally meanwhile.
  const flushPending = useCallback(async listId => {
    for (const [itemId, checked] of [...pending.current]) {
      try {
        await listsApi.updateItem(listId, itemId, { checked })
        pending.current.delete(itemId)
      } catch (e) {
        if (!isRetryableError(e)) pending.current.delete(itemId)
        else break
      }
    }
  }, [])

  const loadActive = useCallback(async (id = activeId) => {
    if (!id) return
    await flushPending(id)
    const d = await listsApi.list(id)
    const items = d.items.map(it => (pending.current.has(it.id) ? { ...it, checked: pending.current.get(it.id) } : it))
    setData({ list: d.list, items })
    setLists(ls => ls?.map(l => (l.id === d.list.id ? d.list : l)))
    if (d.staples_added?.length) toast.info(`Added from staples: ${d.staples_added.join(', ')}`)
    setError('')
  }, [activeId, flushPending, toast])

  const refresh = useCallback(async () => {
    try {
      await loadLists()
      await loadActive()
    } catch (e) {
      setError(isRetryableError(e) ? 'Offline - showing the last loaded list. Ticks are saved when you reconnect.' : e.message)
    }
  }, [loadLists, loadActive])

  useEffect(() => { loadLists().catch(e => setError(e.message)) }, [loadLists])
  useEffect(() => {
    if (!activeId) return
    if (data?.list?.id !== activeId) setData(null)
    loadActive(activeId).catch(e => setError(e.message))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeId])

  // poll while the page is visible; refresh right away when it comes back
  useEffect(() => {
    const tick = () => { if (document.visibilityState === 'visible') refresh() }
    const t = setInterval(tick, POLL_MS)
    document.addEventListener('visibilitychange', tick)
    window.addEventListener('online', tick)
    return () => {
      clearInterval(t)
      document.removeEventListener('visibilitychange', tick)
      window.removeEventListener('online', tick)
    }
  }, [refresh])

  const patchItemLocal = (id, patch) => setData(d => d && ({ ...d, items: d.items.map(it => (it.id === id ? { ...it, ...patch } : it)) }))

  async function toggle(item) {
    if (readOnly) return
    const checked = !item.checked
    patchItemLocal(item.id, { checked })
    try {
      const updated = await listsApi.updateItem(data.list.id, item.id, { checked })
      pending.current.delete(item.id)
      patchItemLocal(item.id, updated)
    } catch (e) {
      if (isRetryableError(e)) {
        pending.current.set(item.id, checked)
      } else {
        patchItemLocal(item.id, { checked: item.checked })
        toast.error(e.message)
      }
    }
  }

  function remove(item) {
    const listId = data.list.id
    setHidden(s => new Set(s).add(item.id))
    const unhide = () => setHidden(s => { const n = new Set(s); n.delete(item.id); return n })
    toast.undoable({
      message: `Removed "${item.name}"`,
      commit: () => listsApi.deleteItem(listId, item.id),
      onUndo: unhide,
      onCommitted: () => { setData(d => d && ({ ...d, items: d.items.filter(i => i.id !== item.id) })); unhide() },
    })
  }

  async function add(payload) {
    const d = await listsApi.addItems(data.list.id, payload)
    await loadActive()
    return d
  }

  async function clearChecked() {
    try {
      const r = await listsApi.clearChecked(data.list.id)
      toast.success(`Cleared ${r.removed} item${r.removed === 1 ? '' : 's'}`)
      loadActive()
    } catch (e) { toast.error(e.message) }
  }

  const visible = data?.items.filter(i => !hidden.has(i.id)) || []
  const toBuy = visible.filter(i => !i.checked)
  const inCart = visible.filter(i => i.checked)
  const groups = useMemo(() => {
    const m = new Map()
    for (const it of toBuy) {
      const k = it.category_name || 'Other'
      if (!m.has(k)) m.set(k, [])
      m.get(k).push(it)
    }
    return [...m.entries()].sort((a, b) => (a[0] === 'Other') - (b[0] === 'Other') || a[0].localeCompare(b[0]))
  }, [toBuy])

  return (
    <div className={`lists-page ${inCart.length ? 'has-finishbar' : ''}`}>
      <PageHeader title="Lists" subtitle={data ? `${toBuy.length} to buy${inCart.length ? ` · ${inCart.length} in cart` : ''}` : undefined} />

      <div className="list-chips" role="tablist" aria-label="Shopping lists">
        {lists?.map(l => (
          <button key={l.id} role="tab" aria-selected={l.id === activeId}
            className={`list-chip ${l.id === activeId ? 'active' : ''}`} onClick={() => selectList(l.id)}>
            {l.emoji && <span aria-hidden="true">{l.emoji}</span>} {l.name}
            {l.open_count > 0 && <span className="list-chip-count">{l.open_count}</span>}
          </button>
        ))}
        {manager && (
          <button className="list-chip add" onClick={() => setModal('new-list')} aria-label="New list">
            <IconPlus width={14} height={14} /> New list
          </button>
        )}
        {manager && data && (
          <button className="list-chip ghost" onClick={() => setModal('edit-list')} aria-label={`Edit list ${data.list.name}`}>
            <IconEdit width={14} height={14} />
          </button>
        )}
      </div>

      {error && <div className="error-box" role="alert">{error}</div>}
      {!data && !error && <SkeletonCard lines={5} />}

      {data && (
        <>
          {data.list.store && <p className="muted list-store">At {data.list.store}</p>}
          {!readOnly && <AddBox listId={data.list.id} onAdd={add} />}

          {toBuy.length === 0 && inCart.length === 0 && (
            <div className="card muted list-empty">Nothing on this list yet. Add items above - try “eggs, 2 milk, bread x3”.</div>
          )}

          {groups.map(([cat, items]) => (
            <section key={cat} className="list-group" aria-label={cat}>
              <h2 className="list-group-title">{cat}</h2>
              <ul className="list-items">
                {items.map(it => (
                  <ItemRow key={it.id} item={it} readOnly={readOnly} manager={manager}
                    onToggle={() => toggle(it)} onDelete={() => remove(it)} onEdit={() => setModal({ item: it })} />
                ))}
              </ul>
            </section>
          ))}

          {inCart.length > 0 && (
            <section className="list-group in-cart" aria-label="In cart">
              <div className="list-group-title-row">
                <h2 className="list-group-title">In cart ({inCart.length})</h2>
                {manager && <button className="link-btn" onClick={clearChecked}>Clear without logging</button>}
              </div>
              <ul className="list-items">
                {inCart.map(it => (
                  <ItemRow key={it.id} item={it} readOnly={readOnly} manager={manager}
                    onToggle={() => toggle(it)} onDelete={() => remove(it)} onEdit={() => setModal({ item: it })} />
                ))}
              </ul>
            </section>
          )}

          <Staples list={data.list} lists={lists || []} manager={manager} onChanged={() => loadActive()} />

          {inCart.length > 0 && (
            <div className="list-finishbar" role="region" aria-label="Finish shopping">
              <span><strong>{inCart.length}</strong> in cart</span>
              {manager
                ? <button onClick={() => setModal('finish')}>Finish shopping</button>
                : <span className="muted">A parent logs the total</span>}
            </div>
          )}
        </>
      )}

      {modal === 'new-list' && (
        <ListModal onClose={() => setModal(null)} onSaved={async l => { setModal(null); await loadLists(); selectList(l.id) }} />
      )}
      {modal === 'edit-list' && data && (
        <ListModal list={data.list} onClose={() => setModal(null)}
          onSaved={async () => { setModal(null); await refresh() }}
          onDeleted={async () => { setModal(null); const ls = await loadLists(); if (ls[0]) selectList(ls[0].id) }} />
      )}
      {modal === 'finish' && data && (
        <FinishModal list={data.list} items={inCart} categories={categories}
          onClose={() => setModal(null)}
          onDone={res => {
            setModal(null)
            toast.success(`Logged ${formatRupiah(res.amount)} in ${res.category}${res.removed ? ` · ${res.removed} item${res.removed === 1 ? '' : 's'} cleared` : ''}`)
            notifySpendingChanged()
            loadActive()
          }} />
      )}
      {modal?.item && (
        <ItemModal item={modal.item} listId={data.list.id} categories={categories}
          onClose={() => setModal(null)} onSaved={() => { setModal(null); loadActive() }} />
      )}
    </div>
  )
}

// ---------- add box ----------

function AddBox({ listId, onAdd }) {
  const toast = useToast()
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const [suggestions, setSuggestions] = useState([])
  const [focused, setFocused] = useState(false)
  const inputRef = useRef(null)
  const q = useDebounced(text.split(',').pop().trim(), 200)

  useEffect(() => {
    if (!focused) return
    let alive = true
    listsApi.suggestions(q, listId).then(d => { if (alive) setSuggestions(d.suggestions) }, () => {})
    return () => { alive = false }
  }, [q, listId, focused])

  async function submit(value = text) {
    const v = value.trim()
    if (!v || busy) return
    setBusy(true)
    try {
      const d = await onAdd({ text: v })
      setText('')
      if (d.added.length > 1) toast.success(`Added ${d.added.length} items`)
    } catch (e) {
      toast.error(isRetryableError(e) ? 'You are offline - adding items needs a connection.' : e.message)
    } finally {
      setBusy(false)
      inputRef.current?.focus()
    }
  }

  function pick(name) {
    // replace the part after the last comma with the suggestion
    const parts = text.split(',')
    parts[parts.length - 1] = ` ${name}`
    const next = parts.join(',').trim()
    if (parts.length === 1) submit(next)
    else { setText(`${next}, `); inputRef.current?.focus() }
  }

  function onPaste(e) {
    const pasted = e.clipboardData.getData('text')
    if (/\n/.test(pasted.trim())) {
      e.preventDefault()
      submit(text ? `${text}\n${pasted}` : pasted)
    }
  }

  return (
    <div className="list-addbox">
      <form onSubmit={e => { e.preventDefault(); submit() }} className="list-addbox-row">
        <input ref={inputRef} value={text} onChange={e => setText(e.target.value)} onPaste={onPaste}
          onFocus={() => setFocused(true)} onBlur={() => setTimeout(() => setFocused(false), 150)}
          placeholder="Add items: eggs, 2 milk, bread x3" aria-label="Add items (comma separated)"
          enterKeyHint="done" autoComplete="off" />
        <button type="submit" disabled={busy || !text.trim()} aria-label="Add"><IconPlus width={18} height={18} /></button>
      </form>
      {focused && suggestions.length > 0 && (
        <div className="list-suggestions" role="group" aria-label="Suggestions">
          {suggestions.map(s => (
            <button type="button" key={s.name} className="chip" onMouseDown={e => e.preventDefault()} onClick={() => pick(s.name)}>
              + {s.name}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

// ---------- item row (tap to check, swipe left to delete) ----------

function ItemRow({ item, readOnly, manager, onToggle, onDelete, onEdit }) {
  const start = useRef(null)
  const [dx, setDx] = useState(0)
  const canDelete = manager

  function onTouchStart(e) { if (canDelete) start.current = { x: e.touches[0].clientX, y: e.touches[0].clientY } }
  function onTouchMove(e) {
    if (!start.current) return
    const mx = e.touches[0].clientX - start.current.x
    const my = e.touches[0].clientY - start.current.y
    if (Math.abs(my) > Math.abs(mx)) { start.current = null; setDx(0); return }
    setDx(Math.min(0, mx))
  }
  function onTouchEnd() {
    if (start.current && dx < -90) onDelete()
    start.current = null
    setDx(0)
  }

  const who = item.checked ? item.checked_by_name : item.added_by_name
  return (
    <li className={`list-item ${item.checked ? 'checked' : ''}`} style={dx ? { transform: `translateX(${dx}px)` } : undefined}
      onTouchStart={onTouchStart} onTouchMove={onTouchMove} onTouchEnd={onTouchEnd}>
      <button className="list-item-main" onClick={onToggle} disabled={readOnly} aria-pressed={item.checked}
        aria-label={`${item.name}${item.qty ? `, ${item.qty}` : ''}${item.checked ? ' (in cart)' : ''}`}>
        <span className="list-check" aria-hidden="true">{item.checked && <IconCheck width={16} height={16} />}</span>
        <span className="list-item-text">
          <span className="list-item-name">{item.qty && <span className="list-qty">{item.qty}</span>}{item.name}</span>
          {(item.note || item.est_price || who) && (
            <span className="list-item-meta">
              {[item.note, item.est_price ? `~${formatRupiah(item.est_price)}` : '', who ? `${item.checked ? '✓' : '+'} ${who}` : '', item.staple_id ? 'staple' : '']
                .filter(Boolean).join(' · ')}
            </span>
          )}
        </span>
      </button>
      {manager && (
        <button className="icon-btn" onClick={onEdit} aria-label={`Edit ${item.name}`}><IconEdit width={15} height={15} /></button>
      )}
      {canDelete && (
        <button className="icon-btn danger" onClick={onDelete} aria-label={`Delete ${item.name}`}><IconClose width={15} height={15} /></button>
      )}
    </li>
  )
}

// ---------- modals ----------

function ListModal({ list, onClose, onSaved, onDeleted }) {
  const toast = useToast()
  const [name, setName] = useState(list?.name || '')
  const [emoji, setEmoji] = useState(list?.emoji || '')
  const [store, setStore] = useState(list?.store || '')
  const [busy, setBusy] = useState(false)

  async function save(e) {
    e.preventDefault()
    if (!name.trim()) return
    setBusy(true)
    try {
      const l = list ? await listsApi.updateList(list.id, { name, emoji, store }) : await listsApi.createList({ name, emoji, store })
      onSaved(l)
    } catch (err) { toast.error(err.message) } finally { setBusy(false) }
  }
  async function del() {
    if (!window.confirm(`Delete "${list.name}" and all its items and staples?`)) return
    try { await listsApi.deleteList(list.id); toast.success('List deleted'); onDeleted() } catch (err) { toast.error(err.message) }
  }
  async function archive() {
    try { await listsApi.updateList(list.id, { archived: true }); toast.success('List archived'); onDeleted() } catch (err) { toast.error(err.message) }
  }

  return (
    <Modal title={list ? 'Edit list' : 'New list'} onClose={onClose}>
      <form className="list-form" onSubmit={save}>
        <label>Name<input data-autofocus value={name} onChange={e => setName(e.target.value)} placeholder="Groceries" required maxLength={60} /></label>
        <div className="list-form-row">
          <label className="narrow">Emoji<input value={emoji} onChange={e => setEmoji(e.target.value)} placeholder="🛒" maxLength={8} /></label>
          <label>Store / place<input value={store} onChange={e => setStore(e.target.value)} placeholder="Supermarket, market, pharmacy…" maxLength={80} /></label>
        </div>
        <div className="list-form-actions">
          {list && <button type="button" className="danger" onClick={del}>Delete</button>}
          {list && <button type="button" className="secondary" onClick={archive}>Archive</button>}
          <button type="submit" disabled={busy || !name.trim()}>{list ? 'Save' : 'Create list'}</button>
        </div>
      </form>
    </Modal>
  )
}

function FinishModal({ list, items, categories, onClose, onDone }) {
  const toast = useToast()
  const estimate = items.reduce((s, i) => s + (i.est_price || 0), 0)
  const [total, setTotal] = useState(estimate || '')
  const [category, setCategory] = useState('')
  const [description, setDescription] = useState('')
  const [busy, setBusy] = useState(false)
  const clientId = useRef(newClientId()) // same id on retry -> no duplicate spending
  const fallback = defaultCategory(categories)
  useEffect(() => { if (!category && fallback) setCategory(fallback) }, [fallback, category])

  async function submit(e) {
    e.preventDefault()
    if (!(Number(total) > 0)) { toast.error('Enter the total you paid'); return }
    setBusy(true)
    try {
      const res = await listsApi.finish(list.id, {
        client_id: clientId.current, total_amount: Number(total), category: category || undefined,
        description: description.trim() || undefined, items: items.map(i => i.id),
      })
      onDone(res)
    } catch (err) {
      toast.error(isRetryableError(err) ? 'Could not reach the server - try again when you are back online (it will not be logged twice).' : err.message)
    } finally { setBusy(false) }
  }

  return (
    <Modal title="Finish shopping" onClose={onClose}>
      <form className="list-form" onSubmit={submit}>
        <p className="muted">{items.length} item{items.length === 1 ? '' : 's'} from {list.name}: {items.slice(0, 6).map(i => i.name).join(', ')}{items.length > 6 ? '…' : ''}</p>
        <label>Total paid
          <MoneyInput data-autofocus value={total} onChange={setTotal} placeholder="e.g. 250k" aria-label="Total paid" className="list-total" />
        </label>
        {estimate > 0 && <span className="muted">Estimated from item prices: {formatRupiah(estimate)}</span>}
        <label>Category<CategorySelect value={category} onChange={setCategory} categories={categories} /></label>
        <label>Description <span className="muted">(optional)</span>
          <input value={description} onChange={e => setDescription(e.target.value)} placeholder={`${list.name} (${items.length} items)`} maxLength={200} />
        </label>
        <div className="list-form-actions">
          <button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Log spending & clear cart'}</button>
        </div>
      </form>
    </Modal>
  )
}

function ItemModal({ item, listId, categories, onClose, onSaved }) {
  const toast = useToast()
  const [name, setName] = useState(item.name)
  const [qty, setQty] = useState(item.qty)
  const [note, setNote] = useState(item.note)
  const [catId, setCatId] = useState(item.category_id ? String(item.category_id) : '')
  const [price, setPrice] = useState(item.est_price ?? '')
  const [busy, setBusy] = useState(false)

  async function save(e) {
    e.preventDefault()
    setBusy(true)
    try {
      await listsApi.updateItem(listId, item.id, {
        name, qty, note, category_id: catId ? Number(catId) : null, est_price: price === '' ? null : Number(price),
      })
      onSaved()
    } catch (err) { toast.error(err.message) } finally { setBusy(false) }
  }

  return (
    <Modal title="Edit item" onClose={onClose}>
      <form className="list-form" onSubmit={save}>
        <div className="list-form-row">
          <label>Item<input data-autofocus value={name} onChange={e => setName(e.target.value)} required maxLength={120} /></label>
          <label className="narrow">Qty<input value={qty} onChange={e => setQty(e.target.value)} placeholder="2, 1kg" maxLength={40} /></label>
        </div>
        <label>Note<input value={note} onChange={e => setNote(e.target.value)} placeholder="brand, size…" maxLength={200} /></label>
        <label>Aisle / category
          <select value={catId} onChange={e => setCatId(e.target.value)}>
            <option value="">Other</option>
            {categories.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select>
        </label>
        <label>Estimated price <span className="muted">(optional)</span><MoneyInput value={price} onChange={setPrice} placeholder="e.g. 25k" /></label>
        <div className="list-form-actions"><button type="submit" disabled={busy || !name.trim()}>Save</button></div>
      </form>
    </Modal>
  )
}

// ---------- staples ----------

function Staples({ list, lists, manager, onChanged }) {
  const toast = useToast()
  const [open, setOpen] = useState(false)
  const [staples, setStaples] = useState(null)
  const [name, setName] = useState('')
  const [qty, setQty] = useState('')
  const [every, setEvery] = useState(7)

  const load = useCallback(() => listsApi.staples().then(d => setStaples(d.staples), e => toast.error(e.message)), [toast])
  useEffect(() => { if (open) load() }, [open, load])

  const mine = staples?.filter(s => s.list_id === list.id) || []

  async function add(e) {
    e.preventDefault()
    if (!name.trim()) return
    try {
      await listsApi.createStaple({ list_id: list.id, name, qty, every_days: Number(every) || 7 })
      setName(''); setQty('')
      await load()
      onChanged()
    } catch (err) { toast.error(err.message) }
  }
  async function update(s, patch) {
    try { await listsApi.updateStaple(s.id, patch); load() } catch (err) { toast.error(err.message) }
  }
  async function del(s) {
    try { await listsApi.deleteStaple(s.id); load() } catch (err) { toast.error(err.message) }
  }

  return (
    <section className="card list-staples">
      <button className="list-staples-toggle" onClick={() => setOpen(o => !o)} aria-expanded={open}>
        <span>Staples <span className="muted">- items that come back on their own</span></span>
        <span aria-hidden="true">{open ? '−' : '+'}</span>
      </button>
      {open && (
        <>
          {staples === null && <p className="muted">Loading…</p>}
          {staples && mine.length === 0 && <p className="muted">No staples for {list.name} yet{manager ? ' - e.g. “Rice, 5kg, every 14 days”.' : '.'}</p>}
          <ul className="staple-rows">
            {mine.map(s => (
              <li key={s.id} className={s.active ? '' : 'paused'}>
                <span className="staple-name">{s.qty && <span className="list-qty">{s.qty}</span>}{s.name}</span>
                <span className="muted">every {s.every_days} day{s.every_days === 1 ? '' : 's'} · next {s.active ? s.next_due : 'paused'}</span>
                {manager && (
                  <span className="staple-actions">
                    <button className="link-btn" onClick={() => update(s, { active: !s.active })}>{s.active ? 'Pause' : 'Resume'}</button>
                    {lists.length > 1 && (
                      <select aria-label={`Move ${s.name} to list`} value={s.list_id} onChange={e => update(s, { list_id: Number(e.target.value) })}>
                        {lists.map(l => <option key={l.id} value={l.id}>{l.name}</option>)}
                      </select>
                    )}
                    <button className="icon-btn danger" onClick={() => del(s)} aria-label={`Delete staple ${s.name}`}><IconClose width={14} height={14} /></button>
                  </span>
                )}
              </li>
            ))}
          </ul>
          {manager && (
            <form className="staple-form" onSubmit={add}>
              <input value={name} onChange={e => setName(e.target.value)} placeholder="Rice" aria-label="Staple item" maxLength={120} />
              <input value={qty} onChange={e => setQty(e.target.value)} placeholder="5kg" aria-label="Quantity" className="narrow" maxLength={40} />
              <label className="staple-every">every
                <input type="number" min={1} max={366} value={every} onChange={e => setEvery(e.target.value)} aria-label="Every how many days" />
                days</label>
              <button type="submit" disabled={!name.trim()}>Add staple</button>
            </form>
          )}
        </>
      )}
    </section>
  )
}
