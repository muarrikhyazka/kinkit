import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { widgetsApi } from '../api'
import { useApi } from '../useApi'
import { useAuth, canManage } from '../AuthContext.jsx'
import { useToast } from '../Toast.jsx'
import PageHeader from '../PageHeader.jsx'
import { SkeletonCard } from '../Skeleton.jsx'
import { IconPlus } from '../icons.jsx'
import WidgetBoard from '../WidgetBoard.jsx'
import ColorPicker from '../ColorPicker.jsx'
import { backdropStyle, normalizeHex } from '../widgetColors.js'
import '../widgets.css'

const REFRESH_MS = 60_000
const DEVICES = [['any', 'Any device'], ['phone', 'Phone'], ['tv', 'TV'], ['car', 'Car screen'], ['monitor', 'Monitor']]

/**
 * Widgets: build boards of widgets (clock, calendar agenda, reminders, chores,
 * meals, budget, goals, ...) by drag and drop, then show them full-screen on a
 * phone, TV, car screen or monitor. Owners/parents edit; everyone can view.
 */
export default function Widgets() {
  const { user } = useAuth()
  const toast = useToast()
  const manager = canManage(user)
  const catalog = useApi(() => widgetsApi.catalog(), [])
  const boards = useApi(() => widgetsApi.boards(), [])
  const [activeId, setActiveId] = useState(null)
  const [draft, setDraft] = useState(null)   // widgets being edited (null = not editing)
  const [draftBg, setDraftBg] = useState('')  // board background being edited
  const [display, setDisplay] = useState(false)
  const [busy, setBusy] = useState(false)

  const list = boards.data?.boards || []
  const board = list.find(b => b.id === activeId) || list[0]
  const widgets = draft || board?.layout?.widgets || []
  const background = draft ? draftBg : normalizeHex(board?.layout?.background)
  const types = [...new Set(widgets.map(w => w.type))]
  const typesKey = types.join(',')

  const data = useApi(() => (board ? widgetsApi.data(types) : Promise.resolve(null)), [board?.id, typesKey])
  const reloadData = data.reload
  useEffect(() => {
    if (!board) return undefined
    const id = setInterval(reloadData, REFRESH_MS)
    return () => clearInterval(id)
  }, [board, reloadData])

  async function createBoard(payload) {
    setBusy(true)
    try {
      const b = await widgetsApi.createBoard(payload)
      await boards.reload()
      setActiveId(b.id)
      toast.success(`Board "${b.name}" created`)
    } catch (e) { toast.error(e.message) } finally { setBusy(false) }
  }
  async function saveLayout() {
    setBusy(true)
    try {
      await widgetsApi.updateBoard(board.id, { layout: { ...(board.layout || {}), widgets: draft, background: draftBg } })
      setDraft(null)
      boards.reload()
      toast.success('Layout saved')
    } catch (e) { toast.error(e.message) } finally { setBusy(false) }
  }
  async function patchBoard(p) {
    try { await widgetsApi.updateBoard(board.id, p); boards.reload() } catch (e) { toast.error(e.message) }
  }
  async function deleteBoard() {
    if (!window.confirm(`Delete the board "${board.name}"? TV links using it go back to the month summary.`)) return
    try { await widgetsApi.removeBoard(board.id); setActiveId(null); setDraft(null); boards.reload() } catch (e) { toast.error(e.message) }
  }
  function startEdit() { setDraftBg(normalizeHex(board.layout?.background)); setDraft(board.layout.widgets) }
  function addWidget(type) {
    const t = catalog.data?.widgets.find(w => w.type === type)
    if (!t) return
    setDraft([...(draft || widgets), { id: `w${Date.now().toString(36)}`, type, w: t.w, h: t.h, text: '' }])
  }

  if (boards.loading && !boards.data) return <div><PageHeader title="Widgets" /><SkeletonCard lines={5} /></div>

  // ----- empty state: pick a starting point -----
  if (!list.length) {
    return (
      <div>
        <PageHeader title="Widgets" subtitle="Your own screens: choose what to show and arrange it. Use them on a phone, TV, car screen or monitor." />
        {boards.error && <div className="error-box">{boards.error}</div>}
        {manager ? (
          <section className="card">
            <h2>Start from a template</h2>
            <p className="muted">You can add, remove and drag widgets around afterwards.</p>
            <div className="wd-presets">
              {(catalog.data?.presets || []).map(p => (
                <button key={p.id} type="button" className="wd-preset" disabled={busy} onClick={() => createBoard({ preset: p.id })}>
                  <strong>{p.name}</strong><small>{p.widgets.length} widgets</small>
                </button>
              ))}
              <button type="button" className="wd-preset" disabled={busy} onClick={() => createBoard({ name: 'My board', layout: { widgets: [] } })}>
                <strong>Blank board</strong><small>Pick every widget yourself</small>
              </button>
            </div>
          </section>
        ) : <p className="muted">No widget boards yet. Ask a parent or the family admin to create one.</p>}
      </div>
    )
  }

  const body = (
    <div className={`wb-stage ${background ? 'has-bg' : ''}`} style={backdropStyle(background)}>
      <WidgetBoard widgets={widgets} cards={data.data?.cards} tz={data.data?.timezone} catalog={catalog.data?.widgets}
        editing={!!draft} display={display} onChange={setDraft} />
    </div>
  )

  if (display) {
    return (
      <FullScreen name={board.name} family={data.data?.household_name} background={background} onExit={() => setDisplay(false)}>{body}</FullScreen>
    )
  }

  return (
    <div>
      <PageHeader title="Widgets" subtitle="Your own screens for a phone, TV, car or monitor. Drag widgets to rearrange them."
        actions={manager && !draft && <button onClick={() => createBoard({ name: 'New board', layout: { widgets: [] } })} disabled={busy}><IconPlus width={16} height={16} aria-hidden="true" /> New board</button>} />
      <div className="wd-boards" role="tablist" aria-label="Boards">
        {list.map(b => (
          <button key={b.id} role="tab" aria-selected={b.id === board.id} className={`chip ${b.id === board.id ? 'active' : ''}`}
            onClick={() => { if (!draft || window.confirm('Discard your unsaved layout changes?')) { setDraft(null); setActiveId(b.id) } }}>{b.name}</button>
        ))}
      </div>

      <div className="wd-toolbar">
        {!draft && <button onClick={() => setDisplay(true)}>Show full screen</button>}
        {manager && !draft && <button className="secondary" onClick={startEdit}>Edit layout</button>}
        {draft && (
          <>
            <select aria-label="Add a widget" value="" onChange={e => { if (e.target.value) addWidget(e.target.value) }}>
              <option value="">+ Add a widget…</option>
              {(catalog.data?.widgets || []).map(w => <option key={w.type} value={w.type}>{w.label}</option>)}
            </select>
            <ColorPicker value={draftBg} onChange={setDraftBg} label="Board background" buttonText="Background" className="secondary" />
            <button onClick={saveLayout} disabled={busy}>Save layout</button>
            <button className="secondary" onClick={() => setDraft(null)}>Cancel</button>
          </>
        )}
        <span className="spacer" />
        {manager && !draft && (
          <>
            <RenameInline board={board} onSave={name => patchBoard({ name })} />
            <select aria-label="Device" value={board.device} onChange={e => patchBoard({ device: e.target.value })}>
              {DEVICES.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
            </select>
            <button className="secondary danger-text" onClick={deleteBoard}>Delete board</button>
          </>
        )}
      </div>
      {draft && <p className="muted" style={{ marginBottom: 'var(--s-3)' }}>Drag a widget onto another to move it, or use the arrows. − / + change the width, ▁ / █ the height, the dot sets a widget's colour.</p>}
      {data.error && <div className="error-box">{data.error}</div>}
      {body}
      <p className="muted" style={{ marginTop: 'var(--s-4)' }}>
        Want this on a TV or a car screen without logging in? Create a link and choose this board under{' '}
        <Link to="/settings/data">Settings › Data &amp; devices</Link>.
      </p>
    </div>
  )
}

function RenameInline({ board, onSave }) {
  const [v, setV] = useState(board.name)
  useEffect(() => setV(board.name), [board.id, board.name])
  return (
    <input aria-label="Board name" value={v} maxLength={60} onChange={e => setV(e.target.value)}
      onBlur={() => { const t = v.trim(); if (t && t !== board.name) onSave(t); else setV(board.name) }}
      onKeyDown={e => { if (e.key === 'Enter') e.currentTarget.blur() }} style={{ width: '11rem' }} />
  )
}

/** Overlay that fills the screen (and asks the browser for real full screen). */
function FullScreen({ name, family, background, onExit, children }) {
  const ref = useRef(null)
  const exit = useCallback(() => { try { if (document.fullscreenElement) document.exitFullscreen() } catch { /* ignore */ } onExit() }, [onExit])
  useEffect(() => {
    try { ref.current?.requestFullscreen?.().catch(() => {}) } catch { /* ignore */ }
    const onFs = () => { if (!document.fullscreenElement) onExit() }
    const onKey = e => { if (e.key === 'Escape') exit() }
    document.addEventListener('fullscreenchange', onFs)
    window.addEventListener('keydown', onKey)
    return () => { document.removeEventListener('fullscreenchange', onFs); window.removeEventListener('keydown', onKey) }
  }, [exit, onExit])
  return (
    <div className="wb-fullscreen" ref={ref} style={backdropStyle(background)}>
      <div className="wb-fullscreen-bar">
        <strong>{family || 'Kinkit'}</strong><span>· {name}</span><span className="spacer" />
        <button className="secondary small" onClick={exit}>Exit full screen</button>
      </div>
      {children}
    </div>
  )
}
