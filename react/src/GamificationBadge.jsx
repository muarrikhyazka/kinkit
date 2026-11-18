import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from './api'
import Celebration, { diffNewRewards } from './Celebration.jsx'
import { IconFlame, IconStar, IconChevronRight } from './icons.jsx'

/**
 * Compact points/streak/level strip for the sidebar. Shows the logged-in
 * member's own score (household total in the tooltip). refreshKey bump
 * (e.g. after saving a spending entry) re-fetches so points update
 * immediately, and newly earned badges / level-ups trigger a celebration.
 */
export default function GamificationBadge({ collapsed, refreshKey }) {
  const [state, setState] = useState(null)
  const [celebrate, setCelebrate] = useState([])

  useEffect(() => {
    let alive = true
    api.gamification().then(s => {
      if (!alive) return
      setState(s)
      const fresh = diffNewRewards(s)
      if (fresh.length) setCelebrate(fresh)
    }).catch(() => {})
    return () => { alive = false }
  }, [refreshKey])

  if (!state) return null
  const m = state.member
  const points = m ? m.points : state.points
  const streak = m ? m.streak_days : state.streak_days
  const level = m ? m.level : state.badge
  const next = m ? m.next_level : state.next_badge
  const nextAt = m ? m.next_level_at : state.next_badge_at
  const title = `${points} pts · ${streak} day streak · ${level}${m ? ` · household total ${state.points} pts` : ''}`
  const celebration = celebrate.length > 0 && <Celebration items={celebrate} onClose={() => setCelebrate([])} />

  if (collapsed) {
    return (
      <>
        <Link to="/family" className="gami-badge-collapsed" title={title} aria-label={title}><IconFlame width={14} height={14} className="gami-ico" />{streak}</Link>
        {celebration}
      </>
    )
  }

  return (
    <>
      <Link to="/family" className="gami-badge" title={`${title} - see achievements`}>
        <div className="gami-row">
          <span className="gami-points"><IconStar width={13} height={13} className="gami-ico" /> {points} pts</span>
          <span className="gami-streak"><IconFlame width={13} height={13} className="gami-ico" /> {streak}d</span>
        </div>
        <div className="gami-badge-name"><span>{level}</span><IconChevronRight className="chev" width={14} height={14} aria-hidden="true" /></div>
        {next && (
          <div className="progress-bar" title={`${nextAt - points} pts to ${next}`}>
            <div className="progress-fill" style={{ width: `${Math.min(100, (points / nextAt) * 100)}%` }} />
          </div>
        )}
      </Link>
      {celebration}
    </>
  )
}
