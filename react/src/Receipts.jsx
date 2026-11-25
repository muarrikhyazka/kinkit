import { useRef, useState } from 'react'
import { dataApi, formatRupiah } from './api'
import { useToast } from './Toast.jsx'
import Modal from './Modal.jsx'
import { ChevronLink } from './ChevronLink.jsx'

// Icons for the data/receipt features (same style as icons.jsx).
const base = { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 1.8, strokeLinecap: 'round', strokeLinejoin: 'round' }
export const IconPaperclip = (p) => (
  <svg {...base} {...p}><path d="M21 11.5l-8.6 8.6a5 5 0 01-7.1-7.1l8.6-8.6a3.3 3.3 0 014.7 4.7l-8.6 8.6a1.7 1.7 0 01-2.4-2.4l7.9-7.9" /></svg>
)
export const IconCamera = (p) => (
  <svg {...base} {...p}><path d="M4 8h3l2-3h6l2 3h3a1 1 0 011 1v9a1 1 0 01-1 1H4a1 1 0 01-1-1V9a1 1 0 011-1z" /><circle cx="12" cy="13" r="3.5" /></svg>
)
export const IconDownload = (p) => (
  <svg {...base} {...p}><path d="M12 4v11M7 10l5 5 5-5M5 20h14" /></svg>
)
export const IconDevices = (p) => (
  <svg {...base} {...p}><rect x="2" y="4" width="14" height="10" rx="1.5" /><path d="M6 18h6M9 14v4" /><rect x="17" y="8" width="5" height="12" rx="1" /></svg>
)

export const RECEIPT_ACCEPT = 'image/jpeg,image/png,image/webp,image/heic,image/heif,image/*'
export const MAX_RECEIPT_BYTES = 5 * 1024 * 1024

export function checkReceiptFile(file) {
  if (!file) return 'No file selected.'
  if (file.size > MAX_RECEIPT_BYTES) return 'Receipt photo is too large (max 5 MB).'
  if (file.type && !file.type.startsWith('image/')) return 'Please choose an image (JPEG, PNG, WebP or HEIC).'
  return ''
}

/**
 * Paperclip button for a spending row: opens the lightbox when the row has a
 * receipt, otherwise picks a photo and uploads it.
 */
export function ReceiptButton({ row, onChanged }) {
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const inputRef = useRef(null)
  const toast = useToast()

  async function attach(file) {
    const err = checkReceiptFile(file)
    if (err) { toast.error(err); return }
    setBusy(true)
    try {
      await dataApi.uploadReceipt(row.id, file)
      toast.success('Receipt attached')
      onChanged && onChanged()
    } catch (e) { toast.error(e.message) } finally { setBusy(false) }
  }

  return (
    <>
      <button
        className={`icon-btn round receipt-btn ${row.has_receipt ? 'has-receipt' : ''}`}
        onClick={() => (row.has_receipt ? setOpen(true) : inputRef.current?.click())}
        disabled={busy}
        aria-label={row.has_receipt ? `View receipt for "${row.description}"` : `Attach receipt to "${row.description}"`}
        title={row.has_receipt ? 'View receipt' : 'Attach receipt photo'}
      >
        <IconPaperclip width={14} height={14} />
      </button>
      <input ref={inputRef} type="file" accept={RECEIPT_ACCEPT} hidden
        onChange={e => { const f = e.target.files?.[0]; e.target.value = ''; if (f) attach(f) }} />
      {open && <ReceiptLightbox row={row} onClose={() => setOpen(false)} onChanged={onChanged} />}
    </>
  )
}

export function ReceiptLightbox({ row, onClose, onChanged }) {
  const toast = useToast()
  const [version, setVersion] = useState(() => String(Date.now()))
  const [failed, setFailed] = useState(false)
  const [busy, setBusy] = useState(false)
  const inputRef = useRef(null)
  const src = dataApi.receiptUrl(row.id, version)

  async function replace(file) {
    const err = checkReceiptFile(file)
    if (err) { toast.error(err); return }
    setBusy(true)
    try {
      await dataApi.uploadReceipt(row.id, file)
      setFailed(false)
      setVersion(String(Date.now()))
      toast.success('Receipt replaced')
      onChanged && onChanged()
    } catch (e) { toast.error(e.message) } finally { setBusy(false) }
  }

  async function remove() {
    if (!window.confirm('Remove this receipt photo?')) return
    setBusy(true)
    try {
      await dataApi.deleteReceipt(row.id)
      toast.success('Receipt removed')
      onChanged && onChanged()
      onClose()
    } catch (e) { toast.error(e.message); setBusy(false) }
  }

  return (
    <Modal title={`Receipt: ${row.description}`} onClose={onClose} className="receipt-modal">
      <p className="muted">{row.tx_date} · {row.category_name || 'Other'} · {formatRupiah(row.amount)}</p>
      <div className="receipt-view">
        {failed ? (
          <div>
            <p className="muted">This browser can't preview the image (HEIC photos only preview in Safari). Download it instead.</p>
            <ChevronLink href={src} download className="standalone">Download receipt</ChevronLink>
          </div>
        ) : (
          <a href={src} target="_blank" rel="noopener" title="Open full size">
            <img src={src} alt={`Receipt for ${row.description}`} onError={() => setFailed(true)} />
          </a>
        )}
      </div>
      <div className="quick-add-actions">
        <button className="secondary danger-text" onClick={remove} disabled={busy}>Remove</button>
        <button className="secondary" onClick={() => inputRef.current?.click()} disabled={busy}>Replace photo</button>
        <input ref={inputRef} type="file" accept={RECEIPT_ACCEPT} hidden
          onChange={e => { const f = e.target.files?.[0]; e.target.value = ''; if (f) replace(f) }} />
      </div>
    </Modal>
  )
}
