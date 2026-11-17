import { useState } from 'react'
import { dataApi } from './api'
import { useApi } from './useApi'
import { useToast } from './Toast.jsx'
import { IconCopy, IconCheck, IconTv } from './icons.jsx'

const tvUrl = token => `${window.location.origin}/tv?k=${encodeURIComponent(token)}`
const when = iso => (iso ? new Date(iso).toISOString().slice(0, 16).replace('T', ' ') + ' UTC' : 'never')

async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    // older browsers / insecure context: fall back to a temporary textarea
    const ta = document.createElement('textarea')
    ta.value = text
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    ta.remove()
    return ok
  }
}

/**
 * "TV / car display links": read-only share links for /tv that work without
 * logging in (e.g. a TV browser or car head unit). Revoking kills the link.
 */
export default function TVTokens() {
  const toast = useToast()
  const tokens = useApi(() => dataApi.tvTokens(), [])
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const [copied, setCopied] = useState(null)

  async function create(e) {
    e.preventDefault()
    setBusy(true)
    try {
      const t = await dataApi.createTvToken(name.trim() || 'TV display')
      setName('')
      tokens.reload()
      if (await copyText(tvUrl(t.token))) { setCopied(t.id); toast.success('Link created and copied') } else toast.success('Link created')
    } catch (err) { toast.error(err.message) } finally { setBusy(false) }
  }

  async function copy(t) {
    if (await copyText(tvUrl(t.token))) {
      setCopied(t.id)
      setTimeout(() => setCopied(c => (c === t.id ? null : c)), 2000)
    } else toast.error('Could not copy - select the link and copy it manually.')
  }

  async function revoke(t) {
    if (!window.confirm(`Revoke "${t.name}"? Screens using this link will stop updating.`)) return
    try {
      await dataApi.revokeTvToken(t.id)
      toast.success('Link revoked')
      tokens.reload()
    } catch (err) { toast.error(err.message) }
  }

  const list = tokens.data || []
  const active = list.filter(t => !t.revoked)
  const revoked = list.filter(t => t.revoked)

  return (
    <div className="card">
      <h2 className="card-title-icon"><IconTv width={18} height={18} aria-hidden="true" /> TV / car display links</h2>
      <p className="muted">
        A link that opens the big-screen view (this month's income, spending and trend) <strong>without logging in</strong> -
        for a TV browser, tablet on the wall or car head unit. It is read-only and refreshes every minute.
        Anyone with the link can see these numbers, so revoke links you no longer use.
      </p>
      <form className="inline-form" onSubmit={create}>
        <input placeholder="Name, e.g. Living room TV" aria-label="Link name" value={name} maxLength={60} onChange={e => setName(e.target.value)} />
        <button type="submit" disabled={busy}>{busy ? 'Creating...' : 'Create link'}</button>
      </form>
      {tokens.error && <div className="error-box">{tokens.error}</div>}
      {active.length > 0 && (
        <ul className="tv-token-list">
          {active.map(t => (
            <li key={t.id}>
              <div className="tv-token-head">
                <strong>{t.name}</strong>
                <span className="muted">created {when(t.created_at)} · last used {when(t.last_used_at)}</span>
              </div>
              <div className="tv-token-url">
                <input readOnly value={tvUrl(t.token)} aria-label={`Link for ${t.name}`} onFocus={e => e.target.select()} />
                <button type="button" className="icon-btn" onClick={() => copy(t)} aria-label={`Copy link for ${t.name}`} title="Copy link">
                  {copied === t.id ? <IconCheck width={16} height={16} /> : <IconCopy width={16} height={16} />}
                </button>
                <a className="link-btn" href={tvUrl(t.token)} target="_blank" rel="noopener">Open</a>
                <button type="button" className="secondary" onClick={() => revoke(t)}>Revoke</button>
              </div>
            </li>
          ))}
        </ul>
      )}
      {!tokens.loading && active.length === 0 && <p className="muted">No active links yet.</p>}
      {revoked.length > 0 && <p className="muted">{revoked.length} revoked link{revoked.length > 1 ? 's' : ''} (no longer work).</p>}
    </div>
  )
}
