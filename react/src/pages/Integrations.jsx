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

// Slack and Matrix marks (kept here; the shared icon set only has Telegram/Discord).
const IconSlack = (p) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" {...p}><path d="M9.5 3v11M14.5 10v11M3 9.5h11M10 14.5h11" /></svg>
)
const IconMatrix = (p) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M6 3H4v18h2M18 3h2v18h-2" /><path d="M8.5 16V8l3.5 4 3.5-4v8" /></svg>
)

// Per-platform form fields and setup steps. `extra` is a second credential kept
// in the integration's "extra" column: Slack's App-Level token (write-only,
// like the bot token) or Matrix's homeserver URL (not secret, shown back).
const PLATFORM_META = {
  telegram: {
    label: 'Telegram', Icon: IconTelegram,
    tokenLabel: 'Bot token from @BotFather',
    chatLabel: 'Allowed chat/channel ID (optional - leave blank to allow any)', chatPlaceholder: 'e.g. -1001234567890',
    steps: [
      <>Open <strong>@BotFather</strong> in Telegram, send <code>/newbot</code> and copy the bot token it gives you.</>,
      <>Add the bot to your family group (or just message it), then paste the token here.</>,
      <>Optional: put the group's chat ID above so the bot only answers there. In groups, turn off the bot's privacy mode if you want plain messages like "50k lunch" logged.</>,
    ],
  },
  discord: {
    label: 'Discord', Icon: IconDiscord,
    tokenLabel: 'Bot token from the Discord Developer Portal',
    chatLabel: 'Allowed chat/channel ID (optional - leave blank to allow any)', chatPlaceholder: 'e.g. 123456789012345678',
    steps: [
      <>Create an application at <strong>discord.com/developers</strong>, add a Bot and copy its token.</>,
      <>Under Bot, enable the <strong>Message Content Intent</strong>, then invite the bot to your server with the <code>bot</code> scope.</>,
      <>Optional: paste the channel ID above so the bot only answers in that channel.</>,
    ],
  },
  slack: {
    label: 'Slack', Icon: IconSlack,
    tokenLabel: 'Bot User OAuth token (xoxb-...)', tokenPlaceholder: 'xoxb-...',
    extra: { label: 'App-Level token (xapp-..., scope connections:write)', placeholder: 'xapp-...', secret: true },
    chatLabel: 'Channel ID where reminders, alerts and digests are posted', chatPlaceholder: 'C0123456789', chatRequired: true,
    note: 'In Slack, type commands with ! instead of / (for example !today) - Slack reserves / for its own slash commands. Buttons work as usual.',
    steps: [
      <>Create an app at <strong>api.slack.com/apps</strong> (From scratch) for your workspace.</>,
      <>Open <strong>Socket Mode</strong> and enable it. Create an <strong>App-Level token</strong> with the <code>connections:write</code> scope (starts with <code>xapp-</code>).</>,
      <>Under <strong>OAuth &amp; Permissions</strong> add the bot scopes <code>chat:write</code>, <code>channels:history</code>, <code>groups:history</code>, <code>im:history</code> and <code>users:read</code>.</>,
      <>Under <strong>Event Subscriptions</strong> enable events and subscribe the bot to <code>message.channels</code>, <code>message.groups</code> and <code>message.im</code>.</>,
      <>Under <strong>Interactivity &amp; Shortcuts</strong> make sure Interactivity is on (needed for the Done / Snooze buttons).</>,
      <><strong>Install to Workspace</strong> and copy the <strong>Bot User OAuth token</strong> (<code>xoxb-</code>).</>,
      <>In Slack, invite the bot to your household channel (<code>/invite @YourBot</code>), then copy the channel ID (channel name &gt; About, at the bottom).</>,
    ],
  },
  matrix: {
    label: 'Matrix', Icon: IconMatrix,
    tokenLabel: 'Bot access token', tokenPlaceholder: 'syt_...',
    extra: { label: 'Homeserver URL', placeholder: 'https://matrix.example.org', secret: false, required: true },
    chatLabel: 'Room ID', chatPlaceholder: '!abc123:example.org', chatRequired: true,
    note: 'In Matrix, type commands with ! instead of / (for example !today). Matrix messages have no buttons: replies show a numbered list - answer with !1, !2 ... - and reminders are answered with !rem <id> done | snooze | tomorrow.',
    steps: [
      <>Create a normal account for the bot on your homeserver (for example <code>@kinkit:example.org</code>).</>,
      <>Get its access token: in Element, log in as the bot, then <strong>Settings &gt; Help &amp; About &gt; Access Token</strong>. Treat it like a password.</>,
      <>Create a room for the household (leave encryption <strong>off</strong> - the bot cannot read encrypted rooms) and invite the bot. It joins by itself.</>,
      <>Copy the room ID from <strong>Room settings &gt; Advanced</strong> (it looks like <code>!abc123:example.org</code>), not the room alias.</>,
      <>Fill in the three fields here, using your homeserver's URL.</>,
    ],
  },
}
const PLATFORMS = ['telegram', 'discord', 'slack', 'matrix']

function SetupSteps({ meta }) {
  return (
    <details className="field-gap">
      <summary style={{ cursor: 'pointer' }}>How to set up {meta.label}</summary>
      <ol style={{ margin: '.5rem 0 0', paddingLeft: '1.2rem' }}>
        {meta.steps.map((st, i) => <li key={i} style={{ marginBottom: '.3rem' }}>{st}</li>)}
      </ol>
    </details>
  )
}

function IntegrationCard({ integration, onUpdate, onDelete }) {
  const meta = PLATFORM_META[integration.platform]
  const toast = useToast()
  // The backend never returns the platform token in plain text - only a
  // masked hint. The input stays empty; we only send a token the user types.
  const [platformToken, setPlatformToken] = useState('')
  const [chatId, setChatId] = useState(integration.chat_id || '')
  // Slack's app token is write-only; Matrix's homeserver URL comes back from the API.
  const [extra, setExtra] = useState(meta.extra?.secret ? '' : integration.extra || '')
  const [saving, setSaving] = useState(false)

  async function save() {
    const payload = { chat_id: chatId }
    if (platformToken.trim()) payload.platform_token = platformToken.trim()
    if (meta.extra && extra.trim() && extra.trim() !== (meta.extra.secret ? '' : integration.extra || '')) payload.extra = extra.trim()
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
    : meta.tokenPlaceholder || 'paste bot token'
  const incomplete = meta.extra && (!integration.has_platform_token || !integration.has_extra || (meta.chatRequired && !integration.chat_id))

  return (
    <div className="card">
      <h2 className="card-title-icon">
        <meta.Icon width={18} height={18} aria-hidden="true" /> {meta.label}
        <span className={`badge ${integration.enabled ? 'badge-savings' : ''}`} style={{ marginLeft: 'auto' }}>
          {integration.enabled ? 'Enabled' : 'Disabled'}
        </span>
        <button className="icon-btn danger round" onClick={() => onDelete(integration)} title="Remove" aria-label={`Remove ${meta.label} integration`}><IconClose width={12} height={12} /></button>
      </h2>

      <label className="field-stack field-gap">
        <span>{meta.tokenLabel}</span>
        <input value={platformToken} onChange={e => setPlatformToken(e.target.value)} placeholder={tokenPlaceholder} type="password" autoComplete="off" />
        <span className="muted">
          {integration.has_platform_token
            ? `A token is saved (${integration.platform_token_masked || 'hidden'}). Leave blank to keep it.`
            : 'No token saved yet.'}
        </span>
      </label>
      {meta.extra && (
        <label className="field-stack field-gap">
          <span>{meta.extra.label}</span>
          <input value={extra} onChange={e => setExtra(e.target.value)} placeholder={meta.extra.secret && integration.has_extra ? 'saved - type to replace' : meta.extra.placeholder}
            type={meta.extra.secret ? 'password' : 'text'} autoComplete="off" />
          {meta.extra.secret && (
            <span className="muted">{integration.has_extra ? 'An app-level token is saved. Leave blank to keep it.' : 'No app-level token saved yet.'}</span>
          )}
        </label>
      )}
      <label className="field-stack field-gap">
        <span>{meta.chatLabel}</span>
        <input value={chatId} onChange={e => setChatId(e.target.value)} placeholder={meta.chatPlaceholder} />
      </label>
      {incomplete && <p className="muted">The {meta.label} bot starts once the token, the {meta.extra.secret ? 'app-level token' : 'homeserver URL'} and the {meta.chatRequired && meta.label === 'Matrix' ? 'room' : 'channel'} ID are all saved.</p>}
      <div style={{ display: 'flex', gap: '.6rem', alignItems: 'center' }}>
        <button onClick={save} disabled={saving}>{saving ? 'Saving...' : 'Save'}</button>
        <label style={{ display: 'flex', alignItems: 'center', gap: '.4rem' }}>
          <input type="checkbox" checked={!!integration.enabled} onChange={e => toggleEnabled(e.target.checked)} />
          Enabled
        </label>
      </div>

      {meta.note && <p className="muted">{meta.note}</p>}
      <SetupSteps meta={meta} />

      <h3 style={{ marginTop: '1rem' }}>Kinkit-issued secret</h3>
      <p className="muted">Give this to your bot process (sent as the <code>X-Bot-Token</code> header) - it authenticates the bot's calls to your account, no separate login needed.</p>
      <CopyBlock text={integration.bot_token} />
    </div>
  )
}

/** Slack and Matrix need all their credentials up front (Matrix is rejected without a homeserver URL). */
function ConnectForm({ platform, onConnect }) {
  const meta = PLATFORM_META[platform]
  const [token, setToken] = useState('')
  const [extra, setExtra] = useState('')
  const [chatId, setChatId] = useState('')
  const [busy, setBusy] = useState(false)
  const ready = token.trim() && extra.trim() && chatId.trim()

  async function submit(e) {
    e.preventDefault()
    setBusy(true)
    try { await onConnect(platform, { platform_token: token.trim(), extra: extra.trim(), chat_id: chatId.trim() }) } finally { setBusy(false) }
  }

  return (
    <div className="card">
      <h2 className="card-title-icon"><meta.Icon width={18} height={18} aria-hidden="true" /> {meta.label}</h2>
      <p className="muted">Not connected yet. Fill in all three fields to connect.</p>
      <form onSubmit={submit}>
        <label className="field-stack field-gap">
          <span>{meta.tokenLabel}</span>
          <input value={token} onChange={e => setToken(e.target.value)} placeholder={meta.tokenPlaceholder} type="password" autoComplete="off" />
        </label>
        <label className="field-stack field-gap">
          <span>{meta.extra.label}</span>
          <input value={extra} onChange={e => setExtra(e.target.value)} placeholder={meta.extra.placeholder} type={meta.extra.secret ? 'password' : 'text'} autoComplete="off" />
        </label>
        <label className="field-stack field-gap">
          <span>{meta.chatLabel}</span>
          <input value={chatId} onChange={e => setChatId(e.target.value)} placeholder={meta.chatPlaceholder} />
        </label>
        <button type="submit" disabled={busy || !ready}>{busy ? 'Connecting...' : `Connect ${meta.label}`}</button>
      </form>
      {meta.note && <p className="muted">{meta.note}</p>}
      <SetupSteps meta={meta} />
    </div>
  )
}

export default function Integrations() {
  const res = useApi(() => api.integrations(), [])
  const toast = useToast()
  const [hidden, hide, unhide] = useHiddenIds()

  async function connect(platform, fields = {}) {
    try {
      await api.addIntegration({ platform, ...fields })
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

      {PLATFORMS.filter(p => !connected.has(p)).map(p => {
        const meta = PLATFORM_META[p]
        if (meta.extra) return <ConnectForm key={p} platform={p} onConnect={connect} />
        return (
          <div className="card" key={p}>
            <h2 className="card-title-icon"><meta.Icon width={18} height={18} /> {meta.label}</h2>
            <p className="muted">Not connected yet.</p>
            <button onClick={() => connect(p)}>Connect {meta.label}</button>
            <SetupSteps meta={meta} />
          </div>
        )
      })}

      <div className="card">
        <h2 className="card-title-icon">
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
