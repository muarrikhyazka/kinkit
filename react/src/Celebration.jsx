import { useMemo } from 'react'
import { createPortal } from 'react-dom'
import Modal from './Modal.jsx'
import { BadgeIcon, LevelBadge } from './icons.jsx'
import Mascot from './Mascot.jsx'

const COLORS = ['#f43f5e', '#f59e0b', '#10b981', '#3b82f6', '#8b5cf6', '#ec4899', '#facc15']

export const prefersReducedMotion = () => {
  try { return window.matchMedia('(prefers-reduced-motion: reduce)').matches } catch { return false }
}

/** Pure CSS confetti burst (no deps). Skipped entirely for reduced motion. */
function Confetti({ count = 60 }) {
  const pieces = useMemo(() => Array.from({ length: count }, (_, i) => ({
    left: Math.random() * 100,
    delay: Math.random() * 0.6,
    duration: 2.2 + Math.random() * 1.6,
    color: COLORS[i % COLORS.length],
    size: 6 + Math.random() * 6,
    drift: (Math.random() - 0.5) * 160,
    round: i % 3 === 0,
  })), [count])
  return (
    <div className="confetti" aria-hidden="true">
      {pieces.map((p, i) => (
        <span key={i} style={{
          left: `${p.left}%`, width: p.size, height: p.size * (p.round ? 1 : 0.45),
          background: p.color, borderRadius: p.round ? '50%' : 2,
          animationDelay: `${p.delay}s`, animationDuration: `${p.duration}s`, '--drift': `${p.drift}px`,
        }} />
      ))}
    </div>
  )
}

/**
 * Celebration dialog for newly earned badges / level-ups.
 *   <Celebration items={[{ iconKey, title, text, level? }]} onClose={...} />
 * iconKey is a badge/challenge key (see BadgeIcon in icons.jsx).
 * Rendered in a portal so it works even when the opener is in a hidden
 * container (e.g. the desktop sidebar on a phone).
 */
export default function Celebration({ items, onClose }) {
  if (!items?.length) return null
  const reduced = prefersReducedMotion()
  return createPortal(
    <>
      {!reduced && <Confetti />}
      <Modal title={items.length > 1 ? 'You earned new rewards!' : 'Nice work!'} onClose={onClose} className="celebration">
        <div className="celebration-mascot"><Mascot pose="celebrate" size={128} animate={!reduced} /></div>
        <ul className="celebration-list">
          {items.map((it, i) => (
            <li key={i}>
              <span className="celebration-icon" aria-hidden="true">{it.level ? <LevelBadge level={it.level} size={40} /> : <BadgeIcon id={it.iconKey} width={28} height={28} />}</span>
              <div>
                <strong>{it.title}</strong>
                {it.text && <div className="muted">{it.text}</div>}
              </div>
            </li>
          ))}
        </ul>
        <div className="modal-actions">
          <button onClick={onClose} data-autofocus>Yay!</button>
        </div>
      </Modal>
    </>,
    document.body,
  )
}

// ---------- "what's new since last seen" (per member, per browser) ----------

const seenKey = memberId => `kinkit:gami-seen:${memberId}`

/**
 * Compares a /gamification response against what this browser last showed
 * for the member; returns celebration items for anything new and records
 * the new state. The very first time (nothing stored) it only records, so
 * existing badges aren't celebrated on a new device.
 */
export function diffNewRewards(state) {
  const m = state?.member
  if (!m) return []
  const earned = (state.badges || []).filter(b => b.earned)
  // `lv` = 1-based level number (new schema). Older stored records only had a 0-based `level` index
  // on a different scale, so they are never compared against `lv`.
  const lv = Number(m.level_number ?? 0) || 0
  const current = { lv, badges: earned.map(b => b.key) }
  let prev = null
  try { prev = JSON.parse(localStorage.getItem(seenKey(m.id)) || 'null') } catch { prev = null }
  try { localStorage.setItem(seenKey(m.id), JSON.stringify(current)) } catch { /* storage blocked: no celebrations */ return [] }
  if (!prev) return []
  const items = []
  if (lv > 0 && typeof prev.lv === 'number' && lv > prev.lv) {
    items.push({
      iconKey: 'level_up', level: lv, title: `Level up: Level ${lv}${m.level ? ` - ${m.level}` : ''}`,
      text: `${m.points} points${m.next_level ? ` - next: ${m.next_level} at ${m.next_level_at}` : ''}`,
    })
  }
  const seen = new Set(prev.badges || [])
  earned.filter(b => !seen.has(b.key)).forEach(b => items.push({ iconKey: b.key, title: `Badge: ${b.name}`, text: b.description }))
  return items
}
