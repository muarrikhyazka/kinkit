import { useEffect, useState } from 'react'
import { formatRupiah } from './api'
import './widgets.css'

/**
 * Renders a widget board: a 12-column flow grid of widgets. Used by the
 * Widgets page (view + edit), full-screen display mode and the TV link view.
 * Data comes from GET /api/widgets/data (or /api/tv/data) as generic "cards".
 *
 *   widgets: [{id, type, w, h, text?}]   cards: {type: {title, big, big_amount, caption, rows[], empty}}
 *   editing: show drag handles + controls; onChange(newWidgets) on every edit
 */
export default function WidgetBoard({ widgets, cards, tz, catalog, editing = false, display = false, onChange }) {
  const [dragId, setDragId] = useState(null)
  const [overId, setOverId] = useState(null)
  const label = type => catalog?.find(c => c.type === type)?.label || type

  const move = (id, to) => {
    const i = widgets.findIndex(w => w.id === id)
    const j = Math.max(0, Math.min(widgets.length - 1, to))
    if (i < 0 || i === j) return
    const next = [...widgets]
    const [x] = next.splice(i, 1)
    next.splice(j, 0, x)
    onChange(next)
  }
  const patch = (id, p) => onChange(widgets.map(w => (w.id === id ? { ...w, ...p } : w)))
  const remove = id => onChange(widgets.filter(w => w.id !== id))

  if (!widgets.length) return <p className="muted wb-empty">This board is empty.{editing ? ' Add a widget above.' : ''}</p>

  return (
    <div className={`wb ${display ? 'display' : ''} ${editing ? 'editing' : ''}`}>
      {widgets.map((w, i) => (
        <div
          key={w.id}
          className={`wb-item ${dragId === w.id ? 'dragging' : ''} ${overId === w.id && dragId !== w.id ? 'over' : ''}`}
          style={{ '--w': w.w, '--h': w.h }}
          draggable={editing}
          onDragStart={e => { setDragId(w.id); e.dataTransfer.effectAllowed = 'move'; try { e.dataTransfer.setData('text/plain', w.id) } catch { /* ignore */ } }}
          onDragOver={e => { if (editing && dragId) { e.preventDefault(); setOverId(w.id) } }}
          onDragLeave={() => setOverId(o => (o === w.id ? null : o))}
          onDrop={e => { e.preventDefault(); if (dragId && dragId !== w.id) move(dragId, i); setDragId(null); setOverId(null) }}
          onDragEnd={() => { setDragId(null); setOverId(null) }}
        >
          {editing && (
            <div className="wb-controls" role="group" aria-label={`${label(w.type)} controls`}>
              <span className="wb-grip" aria-hidden="true" title="Drag to move">⠿</span>
              <strong className="wb-name">{label(w.type)}</strong>
              <button type="button" className="icon-btn" onClick={() => move(w.id, i - 1)} disabled={i === 0} aria-label={`Move ${label(w.type)} earlier`}>◀</button>
              <button type="button" className="icon-btn" onClick={() => move(w.id, i + 1)} disabled={i === widgets.length - 1} aria-label={`Move ${label(w.type)} later`}>▶</button>
              <button type="button" className="icon-btn" onClick={() => patch(w.id, { w: Math.max(2, w.w - 1) })} aria-label="Narrower" title="Narrower">−</button>
              <button type="button" className="icon-btn" onClick={() => patch(w.id, { w: Math.min(12, w.w + 1) })} aria-label="Wider" title="Wider">+</button>
              <button type="button" className="icon-btn" onClick={() => patch(w.id, { h: Math.max(1, w.h - 1) })} aria-label="Shorter" title="Shorter">▁</button>
              <button type="button" className="icon-btn" onClick={() => patch(w.id, { h: Math.min(4, w.h + 1) })} aria-label="Taller" title="Taller">█</button>
              <button type="button" className="icon-btn danger" onClick={() => remove(w.id)} aria-label={`Remove ${label(w.type)}`} title="Remove">✕</button>
            </div>
          )}
          <div className="wb-body">
            <WidgetContent widget={w} card={cards?.[w.type]} tz={tz} editing={editing} onText={text => patch(w.id, { text })} />
          </div>
        </div>
      ))}
    </div>
  )
}

function useClock(tz) {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => { const id = setInterval(() => setNow(new Date()), 1000); return () => clearInterval(id) }, [])
  return now
}

function Clock({ tz }) {
  const now = useClock(tz)
  const opt = tz ? { timeZone: tz } : {}
  return (
    <div className="wb-clock">
      <div className="wb-clock-time">{now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hourCycle: 'h23', ...opt })}</div>
      <div className="wb-clock-date">{now.toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long', ...opt })}</div>
    </div>
  )
}

function WidgetContent({ widget, card, tz, editing, onText }) {
  if (widget.type === 'clock') return <Clock tz={tz} />
  if (widget.type === 'note') {
    return editing ? (
      <textarea className="wb-note-edit" value={widget.text || ''} onChange={e => onText(e.target.value)} maxLength={400} placeholder="Type a note, e.g. Wi-Fi password" aria-label="Note text" />
    ) : <p className="wb-note">{widget.text || <span className="muted">Empty note</span>}</p>
  }
  if (!card) return <p className="muted">No data</p>
  const money = n => formatRupiah(n)
  return (
    <div className="wb-card">
      <h3 className="wb-title">{card.title}</h3>
      {(card.big_amount != null || card.big) && (
        <div className={`wb-big ${card.big_amount != null && card.big_amount < 0 ? 'neg' : ''}`}>{card.big_amount != null ? money(card.big_amount) : card.big}</div>
      )}
      {card.caption && <div className="wb-caption">{card.caption}</div>}
      {card.rows?.length > 0 ? (
        <ul className="wb-rows">
          {card.rows.map((r, i) => (
            <li key={i} className={`${r.done ? 'done' : ''} ${r.alert ? 'alert' : ''}`}>
              <div className="wb-row-main">
                <span className="wb-row-title">{r.title}</span>
                {r.detail && <span className="wb-row-detail">{r.detail}</span>}
              </div>
              {r.amount != null && <span className="wb-row-amount">{money(r.amount)}</span>}
              {r.pct != null && <div className="wb-bar" aria-hidden="true"><div style={{ width: `${Math.max(0, Math.min(100, r.pct))}%` }} /></div>}
            </li>
          ))}
        </ul>
      ) : (card.big_amount == null && !card.big && <p className="muted">{card.empty || 'Nothing to show'}</p>)}
    </div>
  )
}
