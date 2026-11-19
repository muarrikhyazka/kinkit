import { useState } from 'react'
import { Link } from 'react-router-dom'
import { DInfo, DTip, DWarn, DCopy, DCheck } from './DocsIcons.jsx'

/** Flatten React children to plain text (used to build heading ids). */
export function textOf(node) {
  if (node == null || typeof node === 'boolean') return ''
  if (typeof node === 'string' || typeof node === 'number') return String(node)
  if (Array.isArray(node)) return node.map(textOf).join('')
  return textOf(node.props?.children)
}

export const slugify = s => s.toLowerCase().replace(/&/g, ' and ').replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')

function Heading({ as: Tag, id, children }) {
  const hid = id || slugify(textOf(children))
  return (
    <Tag id={hid}>
      {children}
      <a className="dx-anchor" href={`#${hid}`} aria-label={`Link to section: ${textOf(children)}`}>#</a>
    </Tag>
  )
}
/** Section heading with a deep-link anchor; appears in "On this page". */
export const H2 = p => <Heading as="h2" {...p} />
export const H3 = p => <Heading as="h3" {...p} />

const CALLOUT = {
  note: { Icon: DInfo, title: 'Note' },
  tip: { Icon: DTip, title: 'Tip' },
  warning: { Icon: DWarn, title: 'Warning' },
}
/** <Callout type="tip|note|warning" title?> */
export function Callout({ type = 'note', title, children }) {
  const { Icon, title: t } = CALLOUT[type]
  return (
    <aside className={`dx-callout ${type}`} role="note">
      <Icon className="dx-callout-ico" width={20} height={20} />
      <div className="dx-callout-body">
        <div className="dx-callout-title">{title || t}</div>
        {children}
      </div>
    </aside>
  )
}

async function copyText(text) {
  try { await navigator.clipboard.writeText(text); return true } catch { /* fall back */ }
  try {
    const ta = document.createElement('textarea')
    ta.value = text; ta.style.position = 'fixed'; ta.style.opacity = '0'
    document.body.appendChild(ta); ta.select()
    const ok = document.execCommand('copy'); ta.remove(); return ok
  } catch { return false }
}

/** Code / command block with a copy button. */
export function Code({ title = 'Command', children }) {
  const [done, setDone] = useState(false)
  const text = String(children).replace(/^\n/, '').replace(/\n$/, '')
  async function copy() {
    if (await copyText(text)) { setDone(true); setTimeout(() => setDone(false), 1600) }
  }
  return (
    <div className="dx-code">
      <div className="dx-code-head">
        <span>{title}</span>
        <button type="button" className={`dx-copy ${done ? 'done' : ''}`} onClick={copy} aria-label={done ? 'Copied' : `Copy ${title}`}>
          {done ? <DCheck width={14} height={14} /> : <DCopy width={14} height={14} />}
          <span aria-live="polite">{done ? 'Copied' : 'Copy'}</span>
        </button>
      </div>
      <pre><code>{text}</code></pre>
    </div>
  )
}

const ROLE = {
  admin: 'Family admin',
  parent: 'Parent',
  kid: 'Kid',
  viewer: 'Viewer',
}
/** Who can do this. <Roles who={['admin','parent']} /> */
export function Roles({ who, label = 'Who can do this:' }) {
  return (
    <div className="dx-roles">
      <span>{label}</span>
      {who.map(w => <span key={w} className={`dx-role ${w}`}>{ROLE[w]}</span>)}
    </div>
  )
}
export const ALL_ROLES = ['admin', 'parent', 'kid', 'viewer']

/** Numbered step list: <Steps><li>...</li></Steps> */
export const Steps = ({ children }) => <ol className="dx-steps">{children}</ol>

/** Link to another docs page (optionally to a heading). */
export function DocLink({ to, children }) {
  return <Link to={`/docs/${to}`}>{children}</Link>
}

export function Table({ head, rows }) {
  return (
    <div className="dx-table-wrap">
      <table>
        <thead><tr>{head.map(h => <th key={h} scope="col">{h}</th>)}</tr></thead>
        <tbody>{rows.map((r, i) => <tr key={i}>{r.map((c, j) => <td key={j}>{c}</td>)}</tr>)}</tbody>
      </table>
    </div>
  )
}

export function CardGrid({ items }) {
  return (
    <div className="dx-card-grid">
      {items.map(({ to, title, text, Icon }) => (
        <Link key={to} to={`/docs/${to}`} className="dx-card-link">
          {Icon && <span className="dx-card-ico"><Icon width={20} height={20} /></span>}
          <strong>{title}</strong>
          <span>{text}</span>
        </Link>
      ))}
    </div>
  )
}

export function Faq({ q, children }) {
  return (
    <details className="dx-faq">
      <summary>{q}</summary>
      <div>{children}</div>
    </details>
  )
}
