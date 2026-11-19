// Helpers for the /api/gamification member/level fields. Every field is optional so older
// servers (or a missing `member`) degrade to "no level info" instead of throwing.

/** Normalised level info for the logged-in member (falls back to household totals). */
export function levelInfo(state) {
  if (!state) return null
  const m = state.member
  const points = Number(m ? m.points : state.points) || 0
  const number = Number(m?.level_number ?? (m ? 0 : state.level)) || 0
  const title = (m ? m.level : state.badge) || ''
  const floor = Number(m?.level_floor ?? 0) || 0
  const next = (m ? m.next_level : state.next_badge) || ''
  const nextAt = Number((m ? m.next_level_at : state.next_badge_at) ?? 0) || 0
  const hasNext = !!next && nextAt > floor
  const span = hasNext ? nextAt - floor : 0
  const fraction = hasNext ? Math.min(1, Math.max(0, (points - floor) / span)) : 1
  return {
    points, number, title, floor, next, nextAt, hasNext, fraction,
    toNext: hasNext ? Math.max(0, nextAt - points) : 0,
    nextNumber: number ? number + 1 : 0,
  }
}

export const TIER_LABELS = { bronze: 'Bronze', silver: 'Silver', gold: 'Gold' }

export const BADGE_CATEGORIES = [
  { key: 'money', label: 'Money' },
  { key: 'habits', label: 'Habits' },
  { key: 'home', label: 'Home' },
  { key: 'levels', label: 'Levels' },
]

/** Groups badges by category in display order; unknown/missing categories land in "More". */
export function groupBadges(badges) {
  const list = badges || []
  const known = new Set(BADGE_CATEGORIES.map(c => c.key))
  const groups = BADGE_CATEGORIES.map(c => ({ ...c, badges: list.filter(b => b.category === c.key) }))
  const rest = list.filter(b => !known.has(b.category))
  if (rest.length) groups.push({ key: 'more', label: rest.length === list.length ? 'Badges' : 'More', badges: rest })
  return groups.filter(g => g.badges.length)
}
