import { Fragment, useState, useRef } from 'react'
import { todayISO, notifySpendingChanged, dataApi, formatRupiah } from './api'
import { useOffline } from './OfflineContext.jsx'
import { useSetup } from './useApi'
import { useToast } from './Toast.jsx'
import { IconClose } from './icons.jsx'
import MoneyInput from './MoneyInput.jsx'
import CategorySelect from './CategorySelect.jsx'
import { IconPaperclip, IconCamera, RECEIPT_ACCEPT, checkReceiptFile } from './Receipts.jsx'
import { readAmountFromImage } from './receiptOcr.js'

const CAT_MEMORY_KEY = 'kinkit_desc_category'   // { "lunch": "Food", ... }
const RECENT_KEY = 'kinkit_recent_descriptions' // ["Lunch", "Grab", ...]
const RECENT_MAX = 6
const FIELDS = ['tx_date', 'description', 'category', 'amount']

function readJSON(key, fallback) {
  try { return JSON.parse(localStorage.getItem(key)) ?? fallback } catch { return fallback }
}
function writeJSON(key, value) {
  try { localStorage.setItem(key, JSON.stringify(value)) } catch { /* storage blocked */ }
}
const norm = d => d.trim().toLowerCase()

let rowSeq = 0
const emptyRow = (tx_date = todayISO()) => ({ key: ++rowSeq, tx_date, description: '', category: '', amount: '', autoCat: false, error: '', receipt: null, receiptUrl: '', ocr: '' })

/**
 * Multi-row "add spending" form shared by the quick-add modal and the Spending page.
 *   <SpendingForm autoFocus onSaved={({ saved, queued, failed }) => ...} />
 * - smart category: remembers the last category per description and pre-fills it
 * - recent descriptions as one-tap chips
 * - amount shorthand via MoneyInput (25k, 2rb, 1.5jt)
 * - keyboard: Enter = next field (submits from the last row's amount),
 *   Ctrl/Cmd+Enter = add a row. (Esc closing is handled by <Modal>.)
 * - rows the server rejects (4xx) stay in the form with their error.
 * - optional receipt photo per row (camera on mobile), uploaded after the
 *   row is saved; "Read amount" OCRs it on-device to prefill the amount.
 */
export default function SpendingForm({ onSaved, autoFocus = false }) {
  const { categories } = useSetup()
  const { addSpendingResilient } = useOffline()
  const toast = useToast()
  const [rows, setRows] = useState(() => [emptyRow()])
  const [recent, setRecent] = useState(() => readJSON(RECENT_KEY, []))
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const formRef = useRef(null)
  const activeRow = useRef(0)

  function focusField(i, field) {
    setTimeout(() => formRef.current?.querySelector(`[data-row="${i}"][data-field="${field}"]`)?.focus(), 0)
  }

  function suggestCategory(desc) {
    const cat = readJSON(CAT_MEMORY_KEY, {})[norm(desc)]
    return cat && categories.some(c => c.name === cat) ? cat : ''
  }

  function updateRow(i, patch) {
    setRows(rs => rs.map((r, idx) => (idx === i ? { ...r, ...patch, error: '' } : r)))
  }

  function setDescription(i, description) {
    const suggestion = suggestCategory(description)
    setRows(rs => rs.map((r, idx) => {
      if (idx !== i) return r
      const next = { ...r, description, error: '' }
      // only auto-fill when the user hasn't picked a category themselves
      if (!r.category || r.autoCat) {
        next.category = suggestion || (r.autoCat ? '' : r.category)
        next.autoCat = !!suggestion
      }
      return next
    }))
  }

  function pickRecent(desc) {
    let i = Math.min(activeRow.current, rows.length - 1)
    if (rows[i].description) {
      const empty = rows.findIndex(r => !r.description)
      if (empty >= 0) i = empty
    }
    setDescription(i, desc)
    focusField(i, 'amount')
  }

  function addRow() {
    const idx = rows.length
    setRows(rs => [...rs, emptyRow(rs[rs.length - 1]?.tx_date)])
    focusField(idx, 'description')
  }

  function removeRow(i) {
    if (rows.length > 1 && rows[i]?.receiptUrl) URL.revokeObjectURL(rows[i].receiptUrl)
    setRows(rs => (rs.length === 1 ? rs : rs.filter((_, idx) => idx !== i)))
  }

  function setReceipt(i, file) {
    if (file) {
      const err = checkReceiptFile(file)
      if (err) { toast.error(err); return }
    }
    setRows(rs => rs.map((r, idx) => {
      if (idx !== i) return r
      if (r.receiptUrl) URL.revokeObjectURL(r.receiptUrl)
      return { ...r, receipt: file || null, receiptUrl: file ? URL.createObjectURL(file) : '', ocr: '' }
    }))
  }

  async function readAmount(i) {
    const file = rows[i]?.receipt
    if (!file) return
    const key = rows[i].key
    const patchByKey = patch => setRows(rs => rs.map(r => (r.key === key ? { ...r, ...patch } : r)))
    patchByKey({ ocr: 'Loading OCR...' })
    try {
      const { amount } = await readAmountFromImage(file, p => patchByKey({ ocr: `Reading... ${Math.round(p * 100)}%` }))
      if (amount) {
        patchByKey({ amount, ocr: '', error: '' })
        toast.info(`Amount read from receipt: ${formatRupiah(amount)} - please double-check it.`)
      } else {
        patchByKey({ ocr: '' })
        toast.error("Couldn't find a total on this receipt - type the amount instead.")
      }
    } catch (e) {
      patchByKey({ ocr: '' })
      toast.error(e.message)
    }
  }

  function handleKeyDown(e) {
    if (e.key !== 'Enter') return
    if (e.ctrlKey || e.metaKey) { e.preventDefault(); addRow(); return }
    const field = e.target.dataset?.field
    if (!field || e.shiftKey) return
    const i = Number(e.target.dataset.row)
    if (i === rows.length - 1 && field === 'amount') return // native implicit submit
    e.preventDefault()
    const fi = FIELDS.indexOf(field)
    if (fi < FIELDS.length - 1) focusField(i, FIELDS[fi + 1])
    else focusField(i + 1, 'description')
  }

  function remember(desc, category) {
    const mem = readJSON(CAT_MEMORY_KEY, {})
    mem[norm(desc)] = category
    writeJSON(CAT_MEMORY_KEY, mem)
    setRecent(prev => {
      const next = [desc, ...prev.filter(d => norm(d) !== norm(desc))].slice(0, RECENT_MAX)
      writeJSON(RECENT_KEY, next)
      return next
    })
  }

  async function handleSubmit(e) {
    e.preventDefault()
    const filled = rows.filter(r => r.description.trim() || r.amount !== '' || r.category)
    if (filled.length === 0) { setError('Fill in at least one row.'); focusField(0, 'description'); return }
    const incomplete = filled.filter(r => !(r.description.trim() && r.category && r.amount))
    if (incomplete.length) {
      setRows(rs => rs.map(r => (incomplete.includes(r) ? { ...r, error: 'Description, category and amount are required.' } : r)))
      setError('')
      return
    }

    setSaving(true)
    setError('')
    let saved = 0
    let queued = 0
    let receiptsSkipped = 0
    const remaining = []
    for (const r of filled) {
      try {
        const res = await addSpendingResilient({ tx_date: r.tx_date, description: r.description.trim(), category: r.category, amount: Number(r.amount) })
        if (res?.queued) queued++; else saved++
        remember(r.description.trim(), r.category)
        if (r.receipt) {
          if (res?.id && !res.queued) {
            try { await dataApi.uploadReceipt(res.id, r.receipt) } catch (err) { toast.error(`Saved "${r.description.trim()}" but the receipt upload failed: ${err.message}`) }
          } else {
            receiptsSkipped++
          }
          if (r.receiptUrl) URL.revokeObjectURL(r.receiptUrl)
        }
      } catch (err) {
        remaining.push({ ...r, error: err.message })
      }
    }
    if (receiptsSkipped) toast.info('Receipt photos are not stored offline - attach them from the Spending list once you are back online.')
    setRows(remaining.length ? remaining : [emptyRow()])
    setSaving(false)

    if (saved) {
      toast.success(`${saved} transaction${saved > 1 ? 's' : ''} saved`)
      notifySpendingChanged()
    }
    if (queued) toast.info(`${queued} transaction${queued > 1 ? 's' : ''} saved offline - will sync automatically.`)
    if (remaining.length) setError(`${remaining.length} row${remaining.length > 1 ? 's' : ''} could not be saved - see below.`)
    else focusField(0, 'description')
    if (saved + queued > 0) onSaved && onSaved({ saved, queued, failed: remaining.length })
  }

  return (
    <form ref={formRef} className="spending-form" onSubmit={handleSubmit} onKeyDown={handleKeyDown} noValidate>
      {error && <div className="error-box" role="alert">{error}</div>}

      {recent.length > 0 && (
        <div className="recent-chips" role="group" aria-label="Recent descriptions">
          <span className="muted">Recent:</span>
          {recent.map(d => (
            <button type="button" key={d} className="chip" onClick={() => pickRecent(d)}>{d}</button>
          ))}
        </div>
      )}

      <div className="quick-add-rows">
        {rows.map((r, i) => (
          <Fragment key={r.key}>
            <div className={`quick-add-row ${r.error ? 'has-error' : ''}`} onFocus={() => { activeRow.current = i }}>
              <input type="date" aria-label="Date" data-row={i} data-field="tx_date" value={r.tx_date}
                onChange={e => updateRow(i, { tx_date: e.target.value })} />
              <input aria-label="Description" placeholder="Description" data-row={i} data-field="description" value={r.description}
                onChange={e => setDescription(i, e.target.value)} data-autofocus={autoFocus && i === 0 ? '' : undefined}
                aria-invalid={!!r.error && !r.description.trim()} />
              <CategorySelect data-row={i} data-field="category" value={r.category} categories={categories}
                onChange={v => updateRow(i, { category: v, autoCat: false })} suggested={r.autoCat && !!r.category}
                aria-invalid={!!r.error && !r.category} />
              <MoneyInput aria-label="Amount" placeholder="Amount (e.g. 25k)" inputMode="text" data-row={i} data-field="amount" value={r.amount}
                onChange={v => updateRow(i, { amount: v })} aria-invalid={!!r.error && r.amount === ''} />
              <button type="button" className="icon-btn danger round" onClick={() => removeRow(i)} disabled={rows.length === 1} aria-label={`Remove row ${i + 1}`}>
                <IconClose width={12} height={12} />
              </button>
            </div>
            <div className="receipt-line">
              {r.receipt ? (
                <>
                  <img className="receipt-thumb" src={r.receiptUrl} alt="" aria-hidden="true" />
                  <span className="muted receipt-name" title={r.receipt.name}>{r.receipt.name || 'Receipt photo'}</span>
                  <button type="button" className="link-btn" onClick={() => readAmount(i)} disabled={!!r.ocr}>
                    {r.ocr || 'Read amount from receipt'}
                  </button>
                  <button type="button" className="link-btn danger-text" onClick={() => setReceipt(i, null)} aria-label={`Remove receipt from row ${i + 1}`}>Remove</button>
                </>
              ) : (
                <>
                  <label className="link-btn receipt-pick">
                    <IconPaperclip width={13} height={13} aria-hidden="true" /> Attach receipt
                    <input type="file" accept={RECEIPT_ACCEPT} className="sr-only"
                      onChange={e => { const f = e.target.files?.[0]; e.target.value = ''; if (f) setReceipt(i, f) }} />
                  </label>
                  <label className="link-btn receipt-pick receipt-camera">
                    <IconCamera width={13} height={13} aria-hidden="true" /> Camera
                    <input type="file" accept="image/*" capture="environment" className="sr-only"
                      onChange={e => { const f = e.target.files?.[0]; e.target.value = ''; if (f) setReceipt(i, f) }} />
                  </label>
                </>
              )}
            </div>
            {r.error && <div className="row-error" role="alert">{r.error}</div>}
          </Fragment>
        ))}
      </div>

      <p className="form-hint muted">
        Amount shorthand: <kbd>25k</kbd> <kbd>2rb</kbd> <kbd>1.5jt</kbd> · <kbd>Enter</kbd> next field / save · <kbd>Ctrl</kbd>+<kbd>Enter</kbd> add row
      </p>

      <div className="quick-add-actions">
        <button type="button" className="secondary" onClick={addRow}>+ Add Row</button>
        <button type="submit" disabled={saving}>{saving ? 'Saving...' : rows.length > 1 ? 'Save All' : 'Save'}</button>
      </div>
    </form>
  )
}
