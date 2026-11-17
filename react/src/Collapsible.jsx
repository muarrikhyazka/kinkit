import { useId, useState } from 'react'
import { IconChevronDown } from './icons.jsx'

/**
 * Collapsible section whose open/closed state is remembered in localStorage.
 *   <Collapsible title="More insights" storageKey="kinkit_dash_year_insights">...</Collapsible>
 * Children are only rendered (and so only fetch data) while open.
 */
export default function Collapsible({ title, storageKey, defaultOpen = false, children }) {
  const [open, setOpen] = useState(() => {
    try {
      const v = localStorage.getItem(storageKey)
      return v === null ? defaultOpen : v === '1'
    } catch { return defaultOpen }
  })
  const id = useId()

  function toggle() {
    setOpen(o => {
      try { localStorage.setItem(storageKey, o ? '0' : '1') } catch { /* ignore */ }
      return !o
    })
  }

  return (
    <section className={`collapsible ${open ? 'open' : ''}`}>
      <button className="collapsible-toggle" onClick={toggle} aria-expanded={open} aria-controls={id}>
        <IconChevronDown width={16} height={16} className="collapsible-chevron" />
        {title}
      </button>
      {open && <div id={id} className="collapsible-body">{children}</div>}
    </section>
  )
}
