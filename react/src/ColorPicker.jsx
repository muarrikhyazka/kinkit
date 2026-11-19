import { useEffect, useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { PALETTE, normalizeHex } from './widgetColors.js'

/**
 * Swatch button that opens a small popover: a palette of preset colours
 * (including "Default" = no colour) plus a native colour input for a custom one.
 * value: '#rrggbb' or ''. onChange(newValue).
 */
export default function ColorPicker({ value = '', onChange, label, buttonText, className = '' }) {
  const [open, setOpen] = useState(false)
  const [pos, setPos] = useState({ top: 0, left: 0 })
  const btn = useRef(null)
  const pop = useRef(null)
  const id = useId()
  const current = normalizeHex(value)
  const preset = PALETTE.find(p => p.hex === current)

  useEffect(() => {
    if (!open) return undefined
    const r = btn.current?.getBoundingClientRect()
    if (r) setPos({ top: Math.min(r.bottom + 6, window.innerHeight - 250), left: Math.max(8, Math.min(r.left, window.innerWidth - 236)) })
    const first = pop.current?.querySelector('button[aria-pressed="true"]') || pop.current?.querySelector('button')
    first?.focus()
    const onKey = e => { if (e.key === 'Escape') { e.stopPropagation(); setOpen(false); btn.current?.focus() } }
    const onDown = e => { if (!pop.current?.contains(e.target) && !btn.current?.contains(e.target)) setOpen(false) }
    const close = () => setOpen(false)
    document.addEventListener('keydown', onKey, true)
    document.addEventListener('mousedown', onDown)
    window.addEventListener('resize', close)
    window.addEventListener('scroll', close, true)
    return () => {
      document.removeEventListener('keydown', onKey, true)
      document.removeEventListener('mousedown', onDown)
      window.removeEventListener('resize', close)
      window.removeEventListener('scroll', close, true)
    }
  }, [open])

  const pick = hex => { onChange(hex); setOpen(false); btn.current?.focus() }

  return (
    <>
      <button type="button" ref={btn} className={`cp-btn ${buttonText ? 'with-text' : 'icon-btn'} ${className}`} aria-haspopup="dialog" aria-expanded={open}
        aria-label={`${label}: ${preset ? preset.name : current || 'Default'}`} title={label} onClick={() => setOpen(o => !o)} draggable={false}>
        <span className={`cp-swatch ${current ? '' : 'none'}`} style={current ? { background: current } : undefined} aria-hidden="true" />
        {buttonText && <span>{buttonText}</span>}
      </button>
      {open && createPortal(
        <div ref={pop} className="cp-pop" role="dialog" aria-label={label} style={{ top: pos.top, left: pos.left }}>
          <div className="cp-grid">
            {PALETTE.map(p => (
              <button key={p.name} type="button" className={`cp-opt ${p.hex === current ? 'sel' : ''}`} aria-pressed={p.hex === current}
                aria-label={p.name} title={p.name} onClick={() => pick(p.hex)}>
                <span className={`cp-swatch big ${p.hex ? '' : 'none'}`} style={p.hex ? { background: p.hex } : undefined} />
              </button>
            ))}
          </div>
          <label className="cp-custom" htmlFor={id}>
            <span>Custom</span>
            <input id={id} type="color" value={current || '#4f46e5'} onChange={e => onChange(normalizeHex(e.target.value))} aria-label={`${label}: custom colour`} />
          </label>
        </div>,
        document.body,
      )}
    </>
  )
}
