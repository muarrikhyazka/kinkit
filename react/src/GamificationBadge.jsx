import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from './api'
import Celebration, { diffNewRewards } from './Celebration.jsx'
import { IconFlame, IconSteps, IconChevronRight, LevelBadge } from './icons.jsx'
import { levelInfo } from './gamification.js'
import './gamification.css'

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
  const info = levelInfo(state)
  const m = state.member
  const { points, number, title: level, next, nextAt, hasNext, fraction, toNext } = info
  const streak = m ? m.streak_days : state.streak_days
  const lvLabel = number ? `Level ${number}` : ''
  const tip = `${lvLabel ? `${lvLabel} · ` : ''}${level} · ${points} pts · ${streak} day streak${m ? ` · household total ${state.points} pts` : ''}`
  const celebration = celebrate.length > 0 && <Celebration items={celebrate} onClose={() => setCelebrate([])} />

  if (collapsed) {
    return (
      <>
        <Link to="/family" className="gami-badge-collapsed" title={tip} aria-label={tip}>
          {number ? <LevelBadge level={number} size={28} aria-hidden="true" /> : <IconSteps width={14} height={14} className="gami-ico" aria-hidden="true" />}
          <span><IconFlame width={12} height={12} className="gami-ico" />{streak}</span>
        </Link>
        {celebration}
      </>
    )
  }

  return (
    <>
      <Link to="/family" className="gami-badge" title={`${tip}. Open the Leaderboard.`}>
        <div className="gami-level">
          {number > 0 && <LevelBadge level={number} size={34} aria-hidden="true" />}
          <div className="gami-level-text">
            {number > 0 && <span className="gami-level-no">Lv {number}</span>}
            <strong>{level}</strong>
          </div>
          <IconChevronRight className="chev" width={14} height={14} aria-hidden="true" />
        </div>
        <div className="gami-row">
          <span className="gami-points"><IconSteps width={13} height={13} className="gami-ico" aria-hidden="true" /> {points} pts</span>
          <span className="gami-streak"><IconFlame width={13} height={13} className="gami-ico" /> {streak}d</span>
        </div>
        {hasNext && (
          <>
            <div className="progress-bar" title={`${toNext} pts to ${next}`}>
              <div className="progress-fill" style={{ width: `${fraction * 100}%` }} />
            </div>
            <div className="gami-next">{toNext} pts to {number ? `Lv ${number + 1}` : next}</div>
          </>
        )}
      </Link>
      {celebration}
    </>
  )
}
