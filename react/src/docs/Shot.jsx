import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { DZoom, DClose } from './DocsIcons.jsx'

/** A screenshot of the real app (react/public/guide/<name>.jpg). Click to zoom. */
export default function Shot({ name, alt, phone = false }) {
  const [open, setOpen] = useState(false)
  useEffect(() => {
    if (!open) return
    const onKey = e => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open])
  const src = `/guide/${name}.jpg?v=3`
  return (
    <figure className={`dx-shot ${phone ? 'phone' : ''}`}>
      <button type="button" className="dx-shot-btn" onClick={() => setOpen(true)} aria-label={`Enlarge screenshot: ${alt}`}>
        <img src={src} alt={alt} loading="lazy" width={phone ? 390 : 1280} />
        <span className="dx-shot-zoom"><DZoom width={14} height={14} style={{ verticalAlign: '-2px' }} /> Click to zoom</span>
      </button>
      <figcaption>{alt}</figcaption>
      {open && typeof document !== 'undefined' && createPortal(
        <div className="dx dx-lightbox" role="dialog" aria-modal="true" aria-label={alt} onClick={() => setOpen(false)}>
          <button type="button" className="dx-lightbox-close" aria-label="Close" autoFocus onClick={() => setOpen(false)}><DClose /></button>
          <img src={src} alt={alt} />
          <div className="dx-lightbox-cap">{alt}</div>
        </div>,
        document.body,
      )}
    </figure>
  )
}
