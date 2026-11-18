import { Link } from 'react-router-dom'
import { IconChevronRight } from './icons.jsx'

/**
 * Text link that reads as a control: no underline, a trailing chevron that
 * nudges right on hover. Use for "View all" / "Go to X" style navigation.
 *   <ChevronLink to="/budget">Set a budget</ChevronLink>
 *   <ChevronLink href={url} external>Open</ChevronLink>
 */
export function ChevronLink({ to, href, external, children, className = '', ...rest }) {
  const cls = `link-btn link-chevron ${className}`.trim()
  const body = <>{children}<IconChevronRight className="chev" width={16} height={16} aria-hidden="true" /></>
  if (to) return <Link to={to} className={cls} {...rest}>{body}</Link>
  return (
    <a href={href} className={cls} {...(external ? { target: '_blank', rel: 'noopener noreferrer' } : {})} {...rest}>
      {body}
    </a>
  )
}

/**
 * A whole row that navigates, card-style, with a trailing chevron.
 *   <RowLink to="/settings/bots" title="Connect a chat bot" desc="Log spending from Telegram or Discord" />
 */
export function RowLink({ to, href, icon, title, desc, children, className = '', ...rest }) {
  const body = (
    <>
      {icon && <span className="row-link-icon" aria-hidden="true">{icon}</span>}
      <span className="row-link-text">
        <span className="row-link-title">{title}</span>
        {desc && <span className="row-link-desc">{desc}</span>}
        {children}
      </span>
      <IconChevronRight className="chev" width={18} height={18} aria-hidden="true" />
    </>
  )
  const cls = `row-link ${className}`.trim()
  if (to) return <Link to={to} className={cls} {...rest}>{body}</Link>
  return <a href={href} className={cls} {...rest}>{body}</a>
}
