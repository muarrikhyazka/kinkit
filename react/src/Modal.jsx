import { useEffect, useId, useRef } from 'react'
import { IconClose } from './icons.jsx'

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

/**
 * Accessible modal dialog: role="dialog" + aria-modal, focus trap, Esc to
 * close, click-outside to close, focus returns to the opener on close.
 * Put data-autofocus on the element that should get initial focus.
 *   <Modal title="Add Spending" onClose={...} className="quick-add">...</Modal>
 * className "sheet" renders it as a bottom sheet (mobile "More" menu).
 */
export default function Modal({ title, onClose, children, className = '' }) {
  const ref = useRef(null)
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose
  const titleId = useId()

  useEffect(() => {
    const node = ref.current
    const opener = document.activeElement
    const focusables = () => [...node.querySelectorAll(FOCUSABLE)].filter(el => el.offsetParent !== null)
    const first = node.querySelector('[data-autofocus]') || focusables()[0] || node
    first.focus()

    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    function onKey(e) {
      if (e.key === 'Escape') {
        e.stopPropagation()
        onCloseRef.current()
      } else if (e.key === 'Tab') {
        const els = focusables()
        if (els.length === 0) { e.preventDefault(); return }
        const firstEl = els[0]
        const lastEl = els[els.length - 1]
        if (e.shiftKey && (document.activeElement === firstEl || !node.contains(document.activeElement))) {
          e.preventDefault(); lastEl.focus()
        } else if (!e.shiftKey && document.activeElement === lastEl) {
          e.preventDefault(); firstEl.focus()
        }
      }
    }
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prevOverflow
      if (opener && typeof opener.focus === 'function') opener.focus()
    }
  }, [])

  return (
    <div className={`modal-backdrop ${className.includes('sheet') ? 'sheet-backdrop' : ''}`} onMouseDown={e => { if (e.target === e.currentTarget) onClose() }}>
      <div ref={ref} className={`modal ${className}`} role="dialog" aria-modal="true" aria-labelledby={titleId} tabIndex={-1}>
        <div className="modal-header">
          <h2 id={titleId}>{title}</h2>
          <button className="icon-btn" onClick={onClose} aria-label="Close dialog"><IconClose width={16} height={16} /></button>
        </div>
        {children}
      </div>
    </div>
  )
}
