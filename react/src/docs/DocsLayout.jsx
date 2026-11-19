import { useCallback, useEffect, useState } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom'
import { useTheme } from '../ThemeContext.jsx'
import { useAuth } from '../AuthContext.jsx'
import { KinkitLogo, IconSun, IconMoon } from '../icons.jsx'
import { SECTIONS, PAGES, getPage, pagePath, sectionOf, FIRST_SLUG } from './index.js'
import DocsSearch from './DocsSearch.jsx'
import { DMenu, DClose, DChevR, DArrowL, DArrowR } from './DocsIcons.jsx'
import './docs.css'

function Sidebar({ current, open, onClose }) {
  const sec = current && sectionOf(current)
  const [openMap, setOpenMap] = useState({}) // user choices; default = only the current section is open
  // moving to a page opens its section
  useEffect(() => { if (sec) setOpenMap(m => ({ ...m, [sec.id]: true })) }, [sec?.id]) // eslint-disable-line react-hooks/exhaustive-deps
  return (
    <nav className={`dx-side ${open ? 'open' : ''}`} id="dx-sidebar" aria-label="Documentation">
      <div className="dx-side-head">
        <span>Documentation</span>
        <button type="button" className="dx-icon-btn" onClick={onClose} aria-label="Close menu"><DClose /></button>
      </div>
      {SECTIONS.map(s => {
        const isOpen = openMap[s.id] ?? sec?.id === s.id
        const listId = `dx-nav-${s.id}`
        return (
          <div className="dx-nav-section" key={s.id}>
            <button type="button" className="dx-nav-head" aria-expanded={isOpen} aria-controls={listId}
              onClick={() => setOpenMap(m => ({ ...m, [s.id]: !isOpen }))}>
              <span className="dx-nav-title"><s.Icon width={16} height={16} />{s.title}</span>
              <DChevR className="dx-chev" width={16} height={16} />
            </button>
            <ul className="dx-nav-list" id={listId} hidden={!isOpen}>
              {s.pages.map(p => (
                <li key={p.slug}>
                  <Link to={pagePath(p.slug)} aria-current={current?.slug === p.slug ? 'page' : undefined} onClick={onClose}>{p.title}</Link>
                </li>
              ))}
            </ul>
          </div>
        )
      })}
      <div className="dx-side-foot">Kinkit docs. Press <kbd>/</kbd> to search.</div>
    </nav>
  )
}

function useToc(slug) {
  const [items, setItems] = useState([])
  const [active, setActive] = useState('')
  useEffect(() => {
    // wait one frame so the article has rendered
    const raf = requestAnimationFrame(() => {
      const hs = [...document.querySelectorAll('.dx-prose h2[id], .dx-prose h3[id]')]
      setItems(hs.map(h => ({ id: h.id, text: h.textContent.replace(/#$/, ''), level: h.tagName === 'H3' ? 3 : 2 })))
    })
    return () => cancelAnimationFrame(raf)
  }, [slug])
  useEffect(() => {
    if (!items.length) { setActive(''); return }
    let ticking = false
    const compute = () => {
      ticking = false
      let cur = ''
      for (const it of items) {
        const el = document.getElementById(it.id)
        if (el && el.getBoundingClientRect().top <= 110) cur = it.id
        else if (el) break
      }
      // at the very bottom the last heading wins
      if (window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 4) cur = items[items.length - 1].id
      setActive(cur)
    }
    const onScroll = () => { if (!ticking) { ticking = true; requestAnimationFrame(compute) } }
    compute()
    window.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('resize', onScroll)
    return () => { window.removeEventListener('scroll', onScroll); window.removeEventListener('resize', onScroll) }
  }, [items])
  return { items, active }
}

function NotFound({ slug }) {
  return (
    <div className="dx-404">
      <img src="/mascot/mascot-think-256.png" alt="" width={120} height={120} />
      <h1>Page not found</h1>
      <p>There is no docs page called <code>{slug}</code>. It may have moved or been renamed.</p>
      <p><Link to="/docs">Back to the docs home</Link> or press <kbd>/</kbd> to search.</p>
    </div>
  )
}

export default function DocsLayout() {
  const { slug } = useParams()
  const { hash } = useLocation()
  const navigate = useNavigate()
  const { theme, toggle } = useTheme()
  const { user } = useAuth()
  const [menu, setMenu] = useState(false)
  const page = getPage(slug)
  const { items, active } = useToc(slug)
  const dark = theme === 'dark'

  // /docs/introduction is the same page as /docs
  useEffect(() => { if (slug === FIRST_SLUG) navigate('/docs' + hash, { replace: true }) }, [slug, hash, navigate])

  useEffect(() => {
    const prev = document.title
    document.title = page ? `${page.title} | Kinkit Docs` : 'Page not found | Kinkit Docs'
    return () => { document.title = prev }
  }, [page])

  // scroll to the heading in the URL (deep links), else to the top on page change
  useEffect(() => {
    setMenu(false)
    const id = decodeURIComponent((hash || '').slice(1))
    const t = setTimeout(() => {
      const el = id && document.getElementById(id)
      if (el) el.scrollIntoView({ block: 'start' })
      else window.scrollTo(0, 0)
    }, 60)
    return () => clearTimeout(t)
  }, [slug, hash])

  // drawer: Esc closes, lock body scroll
  useEffect(() => {
    if (!menu) return
    const onKey = e => { if (e.key === 'Escape') setMenu(false) }
    document.addEventListener('keydown', onKey)
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.removeEventListener('keydown', onKey); document.body.style.overflow = prev }
  }, [menu])

  // links inside articles written as plain <a href="/docs/..."> navigate client-side
  const onArticleClick = useCallback(e => {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
    const a = e.target.closest?.('a[href]')
    if (!a || a.target) return
    const href = a.getAttribute('href')
    if (href && (href === '/docs' || href.startsWith('/docs/') || href.startsWith('/docs#'))) {
      e.preventDefault()
      navigate(href)
    }
  }, [navigate])

  const idx = page ? PAGES.findIndex(p => p.slug === page.slug) : -1
  const prev = idx > 0 ? PAGES[idx - 1] : null
  const next = idx >= 0 && idx < PAGES.length - 1 ? PAGES[idx + 1] : null
  const sec = page && sectionOf(page)
  const Content = page?.Content

  return (
    <div className="dx">
      <a className="dx-skip" href="#dx-main">Skip to content</a>
      <header className="dx-top">
        <button type="button" className="dx-menu-btn" aria-label="Open menu" aria-expanded={menu} aria-controls="dx-sidebar" onClick={() => setMenu(true)}><DMenu /></button>
        <Link to="/docs" className="dx-brand" aria-label="Kinkit docs home">
          <KinkitLogo size={30} aria-hidden="true" />
          <span>Kinkit</span>
          <span className="dx-brand-docs">Docs</span>
        </Link>
        <div className="dx-top-spacer" />
        <DocsSearch />
        <div className="dx-top-actions">
          <button type="button" className="dx-icon-btn" onClick={toggle} aria-label={dark ? 'Switch to light mode' : 'Switch to dark mode'} title={dark ? 'Light mode' : 'Dark mode'}>
            {dark ? <IconSun width={20} height={20} /> : <IconMoon width={20} height={20} />}
          </button>
          <Link to={user ? '/' : '/login'} className="dx-open-app">{user ? 'Open app' : 'Log in'}</Link>
        </div>
      </header>

      <div className="dx-shell">
        <Sidebar current={page} open={menu} onClose={() => setMenu(false)} />
        <div className={`dx-backdrop ${menu ? 'open' : ''}`} onClick={() => setMenu(false)} aria-hidden="true" />

        <main className="dx-main" id="dx-main" tabIndex={-1}>
          <article className="dx-article" onClick={onArticleClick}>
            {!page && <NotFound slug={slug} />}
            {page && (
              <>
                <nav className="dx-crumbs" aria-label="Breadcrumb">
                  <Link to="/docs">Docs</Link>
                  <DChevR width={14} height={14} />
                  <span>{sec?.title}</span>
                  {page.slug !== FIRST_SLUG && <><DChevR width={14} height={14} /><span aria-current="page">{page.title}</span></>}
                </nav>
                <h1 className="dx-title">{page.title}</h1>
                <p className="dx-lede">{page.summary}</p>
                {items.length > 2 && (
                  <details className="dx-toc-inline">
                    <summary>On this page</summary>
                    <ul>{items.map(it => <li key={it.id} className={`l${it.level}`}><a href={`#${it.id}`}>{it.text}</a></li>)}</ul>
                  </details>
                )}
                <div className="dx-prose"><Content /></div>

                <nav className="dx-pager" aria-label="Previous and next page">
                  {prev && <Link to={pagePath(prev.slug)} className="prev"><small><DArrowL width={12} height={12} /> Previous</small><strong>{prev.title}</strong></Link>}
                  {next && <Link to={pagePath(next.slug)} className="next"><small>Next <DArrowR width={12} height={12} /></small><strong>{next.title}</strong></Link>}
                </nav>
                <div className="dx-foot">
                  <span>Kinkit docs</span>
                  <span><a href="#dx-main" onClick={e => { e.preventDefault(); window.scrollTo({ top: 0, behavior: 'smooth' }) }}>Back to top</a></span>
                </div>
              </>
            )}
          </article>

          {page && items.length > 0 && (
            <aside className="dx-toc" aria-label="On this page">
              <p className="dx-toc-title">On this page</p>
              <ul>
                {items.map(it => (
                  <li key={it.id}>
                    <a href={`#${it.id}`} className={`l${it.level} ${active === it.id ? 'active' : ''}`} aria-current={active === it.id ? 'location' : undefined}>{it.text}</a>
                  </li>
                ))}
              </ul>
            </aside>
          )}
        </main>
      </div>
    </div>
  )
}
