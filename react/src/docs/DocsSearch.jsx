import { useEffect, useMemo, useRef, useState, Fragment } from 'react'
import { useNavigate } from 'react-router-dom'
import { DSearch, DClose } from './DocsIcons.jsx'
import { search, snippet } from './searchIndex.js'

function Highlight({ text, query }) {
  const tokens = query.toLowerCase().split(/\s+/).filter(Boolean).map(t => t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
  if (!tokens.length) return text
  const re = new RegExp(`(${tokens.join('|')})`, 'ig')
  return text.split(re).map((part, i) => (i % 2 ? <mark key={i}>{part}</mark> : <Fragment key={i}>{part}</Fragment>))
}

/** Search box with a results dropdown. Shortcut: "/" or Ctrl/Cmd+K. */
export default function DocsSearch() {
  const [q, setQ] = useState('')
  const [open, setOpen] = useState(false)       // results visible
  const [expanded, setExpanded] = useState(false) // mobile: field shown
  const [active, setActive] = useState(0)
  const inputRef = useRef(null)
  const boxRef = useRef(null)
  const navigate = useNavigate()
  const results = useMemo(() => (q.trim() ? search(q) : []), [q])

  useEffect(() => { setActive(0) }, [q])

  useEffect(() => {
    function onKey(e) {
      const tag = (e.target.tagName || '').toLowerCase()
      const typing = tag === 'input' || tag === 'textarea' || tag === 'select' || e.target.isContentEditable
      if ((e.key === '/' && !typing && !e.metaKey && !e.ctrlKey) || ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k')) {
        e.preventDefault()
        setExpanded(true)
        setTimeout(() => inputRef.current?.focus(), 0)
        inputRef.current?.focus()
        inputRef.current?.select()
        setOpen(true)
      }
    }
    function onDown(e) { if (boxRef.current && !boxRef.current.contains(e.target)) { setOpen(false) } }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onDown)
    return () => { document.removeEventListener('keydown', onKey); document.removeEventListener('mousedown', onDown) }
  }, [])

  function go(r) {
    navigate(r.href)
    setOpen(false); setExpanded(false); setQ('')
    inputRef.current?.blur()
  }

  function onKeyDown(e) {
    if (e.key === 'ArrowDown') { e.preventDefault(); setActive(a => Math.min(a + 1, Math.max(results.length - 1, 0))) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setActive(a => Math.max(a - 1, 0)) }
    else if (e.key === 'Enter' && results[active]) { e.preventDefault(); go(results[active]) }
    else if (e.key === 'Escape') {
      if (q) setQ(''); else { setOpen(false); setExpanded(false); inputRef.current?.blur() }
    }
  }

  const showList = open && q.trim().length > 0

  return (
    <div className={`dx-search ${expanded ? 'open' : ''}`} ref={boxRef} role="search">
      <button type="button" className="dx-icon-btn dx-search-toggle" aria-label="Search the docs" onClick={() => { setExpanded(true); setOpen(true); setTimeout(() => inputRef.current?.focus(), 0) }}>
        <DSearch />
      </button>
      <div className="dx-search-field">
        <DSearch width={16} height={16} />
        <input
          ref={inputRef}
          type="search"
          value={q}
          placeholder="Search the docs"
          aria-label="Search the docs"
          role="combobox"
          aria-expanded={showList}
          aria-controls="dx-results"
          aria-autocomplete="list"
          aria-activedescendant={showList && results[active] ? `dx-r-${active}` : undefined}
          autoComplete="off"
          spellCheck={false}
          onChange={e => { setQ(e.target.value); setOpen(true) }}
          onFocus={() => setOpen(true)}
          onKeyDown={onKeyDown}
        />
        <kbd aria-hidden="true">/</kbd>
        <button type="button" className="dx-search-close" aria-label="Close search" onClick={() => { setExpanded(false); setOpen(false); setQ('') }}><DClose width={16} height={16} /></button>
      </div>
      {showList && (
        <ul className="dx-results" id="dx-results" role="listbox" aria-label="Search results">
          {results.length === 0 && <li className="dx-results-empty" role="presentation">No results for &ldquo;{q}&rdquo;. Try a shorter word.</li>}
          {results.map((r, i) => (
            <li key={r.href + i} role="presentation">
              <a
                id={`dx-r-${i}`}
                role="option"
                aria-selected={i === active}
                className="dx-result"
                href={r.href}
                onMouseEnter={() => setActive(i)}
                onClick={e => { e.preventDefault(); go(r) }}
              >
                <div className="dx-result-path">{r.page.sectionTitle}{r.heading ? ` › ${r.page.title}` : ''}</div>
                <div className="dx-result-title"><Highlight text={r.heading || r.page.title} query={q} /></div>
                <div className="dx-result-snippet"><Highlight text={snippet(r, q)} query={q} /></div>
              </a>
            </li>
          ))}
          {results.length > 0 && <li className="dx-results-foot" role="presentation"><span>↑↓ to move</span><span>Enter to open</span><span>Esc to close</span></li>}
        </ul>
      )}
    </div>
  )
}
