import { useState } from 'react'
import { api } from '../api'
import { useApi } from '../useApi'
import { useToast, useHiddenIds } from '../Toast.jsx'
import PageHeader from '../PageHeader.jsx'
import { SkeletonCard } from '../Skeleton.jsx'
import { IconTelegram, IconDiscord, IconLink, IconCopy, IconCheck, IconClose } from '../icons.jsx'

function CopyBlock({ text }) {
  const [copied, setCopied] = useState(false)
  function copy() {
    navigator.clipboard?.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }
  return (
    <div className="copy-block">
      <code>{text}</code>
      <button className="icon-btn" onClick={copy} title="Copy" aria-label={copied ? 'Copied' : 'Copy to clipboard'}>
        {copied ? <IconCheck width={13} height={13} /> : <IconCopy width={13} height={13} />}
      </button>
    </div>
  )
}

const PLATFORM_META = {
  telegram: { label: 'Telegram', Icon: IconTelegram, tokenHint: 'Bot token from @BotFather' },
  discord: { label: 'Discord', Icon: IconDiscord, tokenHint: 'Bot token from the Discord Developer Portal' },
}

function IntegrationCard({ integration, onUpdate, onDelete }) {
  const meta = PLATFORM_META[integration.platform]
  const toast = useToast()
  // The backend never returns the platform token in plain text - only a
  // masked hint. The input stays empty; we only send a token the user types.
  const [platformToken, setPlatformToken] = useState('')
  const [chatId, setChatId] = useState(integration.chat_id || '')
  const [saving, setSaving] = useState(false)

  async function save() {
    const payload = { chat_id: chatId }
    if (platformToken.trim()) payload.platform_token = platformToken.trim()
    setSaving(true)
    try {
      await onUpdate(integration.id, payload)
      setPlatformToken('')
      toast.success(`${meta.label} settings saved`)
    } catch (e) {
      toast.error(e.message)
    } finally {
      setSaving(false)
    }
  }

  async function toggleEnabled(enabled) {
    try { await onUpdate(integration.id, { enabled }) } catch (e) { toast.error(e.message) }
  }

  const tokenPlaceholder = integration.has_platform_token
    ? `${integration.platform_token_masked || 'saved'} - type to replace`
    : 'paste bot token'

  return (
    <div className="card">
      <h2 style={{ display: 'flex', alignItems: 'center', gap: '.5rem' }}>
        <meta.Icon width={18} height={18} aria-hidden="true" /> {meta.label}
        <span className={`badge ${integration.enabled ? 'badge-savings' : ''}`} style={{ marginLeft: 'auto' }}>
          {integration.enabled ? 'Enabled' : 'Disabled'}
        </span>
        <button className="icon-btn danger round" onClick={() => onDelete(integration)} title="Remove" aria-label={`Remove ${meta.label} integration`}><IconClose width={12} height={12} /></button>
      </h2>

      <label style={{ display: 'flex', flexDirection: 'column', gap: '.3rem', marginBottom: '.6rem' }}>
        <span>{meta.tokenHint}</span>
        <input value={platformToken} onChange={e => setPlatformToken(e.target.value)} placeholder={tokenPlaceholder} type="password" autoComplete="off" />
        <span className="muted">
          {integration.has_platform_token
            ? `A token is saved (${integration.platform_token_masked || 'hidden'}). Leave blank to keep it.`
            : 'No token saved yet.'}
        </span>
      </label>
      <label style={{ display: 'flex', flexDirection: 'column', gap: '.3rem', marginBottom: '.6rem' }}>
        <span>Allowed chat/channel ID (optional - leave blank to allow any)</span>
        <input value={chatId} onChange={e => setChatId(e.target.value)} placeholder="e.g. -1001234567890" />
      </label>
      <div style={{ display: 'flex', gap: '.6rem', alignItems: 'center' }}>
        <button onClick={save} disabled={saving}>{saving ? 'Saving...' : 'Save'}</button>
        <label style={{ display: 'flex', alignItems: 'center', gap: '.4rem' }}>
          <input type="checkbox" checked={!!integration.enabled} onChange={e => toggleEnabled(e.target.checked)} />
          Enabled
        </label>
      </div>

      <h3 style={{ marginTop: '1rem' }}>Kinkit-issued secret</h3>
      <p className="muted">Give this to your bot process (sent as the <code>X-Bot-Token</code> header) - it authenticates the bot's calls to your account, no separate login needed.</p>
      <CopyBlock text={integration.bot_token} />
    </div>
  )
}

export default function Integrations() {
  const res = useApi(() => api.integrations(), [])
  const toast = useToast()
  const [hidden, hide, unhide] = useHiddenIds()

  async function connect(platform) {
    try {
      await api.addIntegration({ platform })
      toast.success(`${PLATFORM_META[platform].label} connected`)
      res.reload()
    } catch (e) { toast.error(e.message) }
  }

  async function handleUpdate(id, payload) {
    await api.updateIntegration(id, payload)
    res.reload()
  }

  function handleDelete(integration) {
    hide(integration.id)
    toast.undoable({
      message: `Removed ${PLATFORM_META[integration.platform]?.label || integration.platform} - the bot stops working until reconnected`,
      commit: () => api.deleteIntegration(integration.id),
      onUndo: () => unhide(integration.id),
      onCommitted: res.reload,
    })
  }

  const header = <PageHeader title="Integrations" subtitle="Connect a chat platform's bot to log spending, get reminders, and manage your household - no server access needed, all configured here." />

  if (!res.data) {
    return <div>{header}{res.error ? <div className="error-box">{res.error}</div> : <><SkeletonCard lines={4} /><SkeletonCard lines={3} /></>}</div>
  }

  const items = res.data.filter(i => !hidden.has(i.id))
  const connected = new Set(res.data.map(i => i.platform)) // incl. pending-delete ones, so "Connect" waits for the delete

  return (
    <div>
      {header}
      {res.error && <div className="error-box">{res.error}</div>}

      {items.map(i => (
        <IntegrationCard key={i.id} integration={i} onUpdate={handleUpdate} onDelete={handleDelete} />
      ))}

      {!connected.has('telegram') && (
        <div className="card">
          <h2 style={{ display: 'flex', alignItems: 'center', gap: '.5rem' }}><IconTelegram width={18} height={18} /> Telegram</h2>
          <p className="muted">Not connected yet.</p>
          <button onClick={() => connect('telegram')}>Connect Telegram</button>
        </div>
      )}
      {!connected.has('discord') && (
        <div className="card">
          <h2 style={{ display: 'flex', alignItems: 'center', gap: '.5rem' }}><IconDiscord width={18} height={18} /> Discord</h2>
          <p className="muted">Not connected yet.</p>
          <button onClick={() => connect('discord')}>Connect Discord</button>
        </div>
      )}

      <div className="card">
        <h2 style={{ display: 'flex', alignItems: 'center', gap: '.5rem' }}>
          <IconLink width={18} height={18} /> How it works
        </h2>
        <p>Each platform you connect above gets its own secret token (<code>X-Bot-Token</code> header) tied only to your account. One shared bot process can serve every Kinkit user at once - it just looks up the right account from the token each message carries.</p>
        <table className="simple-table" style={{ marginTop: '.8rem' }}>
          <thead><tr><th>Action</th><th>Method & path</th><th>Body / params</th></tr></thead>
          <tbody>
            <tr><td>Bot config (reminders/keywords/categories)</td><td><code>GET /api/bot/config</code></td><td>-</td></tr>
            <tr><td>Add spending</td><td><code>POST /api/bot/spending</code></td><td><code>{'{ "tx_date", "description", "category", "amount" }'}</code></td></tr>
            <tr><td>Recent entries</td><td><code>GET /api/bot/spending/recent?n=5</code></td><td>-</td></tr>
            <tr><td>Search entries</td><td><code>GET /api/bot/spending/search?q=makan</code></td><td>-</td></tr>
            <tr><td>Update category</td><td><code>PATCH /api/bot/spending/&lt;id&gt;/category</code></td><td><code>{'{ "category" }'}</code></td></tr>
            <tr><td>Update date</td><td><code>PATCH /api/bot/spending/&lt;id&gt;/date</code></td><td><code>{'{ "tx_date" }'}</code></td></tr>
          </tbody>
        </table>
      </div>
    </div>
  )
}
