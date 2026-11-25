import { useEffect } from 'react'
import { api, formatRupiah, onSpendingChanged } from '../api'
import { useApi } from '../useApi'
import { useAuth } from '../AuthContext.jsx'
import PageHeader from '../PageHeader.jsx'
import StatCard from '../StatCard.jsx'
import { RoleBadge } from './Members.jsx'
import { SkeletonCard, SkeletonStats } from '../Skeleton.jsx'
import { BadgeIcon, LevelBadge } from '../icons.jsx'
import { levelInfo, groupBadges, TIER_LABELS } from '../gamification.js'
import '../gamification.css'

function LevelCard({ g }) {
  const li = levelInfo(g)
  if (!li || !li.number) return null
  return (
    <div className="card level-card" id="level">
      <LevelBadge level={li.number} size={84} />
      <div className="level-card-main">
        <div className="level-card-head">
          <h2>{li.title}</h2>
          <span className="level-card-no">Level {li.number}</span>
        </div>
        <div className="level-xp">
          <div className="progress-bar" role="progressbar" aria-label="Progress to next level" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(li.fraction * 100)}>
            <div className="progress-fill" style={{ width: `${li.fraction * 100}%` }} />
          </div>
          <div className="level-xp-labels">
            {li.hasNext
              ? <><span><strong>{li.points}</strong> / {li.nextAt} pts</span><span><strong>{li.toNext} pts</strong> to Level {li.nextNumber}{li.next ? ` · ${li.next}` : ''}</span></>
              : <span><strong>{li.points} pts</strong> · max level reached</span>}
          </div>
        </div>
      </div>
    </div>
  )
}

function Achievements({ g }) {
  if (!g) return <SkeletonCard lines={4} />
  const earned = g.badges.filter(b => b.earned).length
  return (
    <>
      <div className="card" id="challenges">
        <h2>This week's family challenges</h2>
        <p className="muted">Weeks run Monday to Sunday (UTC). Completing one adds bonus points to the household total, once per week.</p>
        <ul className="challenge-list">
          {g.challenges.map(c => {
            const pct = c.target ? Math.min(100, (c.progress / c.target) * 100) : 0
            const over = c.lower_is_better && c.progress > c.target
            return (
              <li key={c.key} className={`challenge ${c.completed ? 'done' : ''} ${!c.available ? 'locked' : ''}`}>
                <span className="challenge-icon" aria-hidden="true"><BadgeIcon id={c.key || c.id} width={28} height={28} /></span>
                <div className="challenge-body">
                  <div className="challenge-head">
                    <strong>{c.name}</strong>
                    <span className="chip">{c.completed ? 'Done' : `+${c.bonus} pts`}</span>
                  </div>
                  <div className="muted">{c.description}</div>
                  {c.available ? (
                    <>
                      <div className="progress-bar" aria-hidden="true">
                        <div className={`progress-fill ${over ? 'over' : ''}`} style={{ width: `${pct}%` }} />
                      </div>
                      <div className="muted">
                        {c.unit === 'amount'
                          ? `${formatRupiah(c.progress)} of ${formatRupiah(c.target)} ${over ? '(over the limit)' : 'so far'}`
                          : `${c.progress} / ${c.target} ${c.unit || 'days'}`}
                        {c.last_week_done && ' · completed last week too'}
                      </div>
                    </>
                  ) : (
                    <div className="muted">{c.key === 'all_chores'
                      ? 'Add some chores on the Chores page to unlock.'
                      : 'Set a Wants budget for this month on the Budgeting page to unlock.'}</div>
                  )}
                </div>
              </li>
            )
          })}
        </ul>
      </div>

      <div className="card badge-groups" id="badges">
        <h2>Your badges <span className="muted">({earned}/{g.badges.length})</span></h2>
        {groupBadges(g.badges).map(grp => (
          <section key={grp.key} className="badge-group" aria-label={`${grp.label} badges`}>
            <h3 className="badge-group-title">{grp.label} <span className="muted">{grp.badges.filter(b => b.earned).length}/{grp.badges.length}</span></h3>
            <ul className="badge-grid">
              {grp.badges.map(b => {
                const tier = TIER_LABELS[b.tier] ? b.tier : 'bronze'
                const target = Number(b.target) || 0
                const progress = Math.min(Number(b.progress) || 0, target)
                return (
                  <li key={b.key} className={`badge-tile tier-${tier} ${b.earned ? 'earned' : ''}`} title={b.description}>
                    <span className="badge-tile-icon" aria-hidden="true"><BadgeIcon id={b.key} width={28} height={28} /></span>
                    <strong>{b.name}</strong>
                    {b.tier && <span className="badge-tier">{TIER_LABELS[tier]}</span>}
                    <span className="badge-desc">{b.description}</span>
                    {b.earned ? (
                      <span className="badge-earned-on">Earned{b.earned_at ? ` ${b.earned_at.slice(0, 10)}` : ''}</span>
                    ) : target > 0 && (
                      <div className="badge-progress">
                        <div className="progress-bar" aria-hidden="true"><div className="progress-fill" style={{ width: `${(progress / target) * 100}%` }} /></div>
                        <span className="badge-progress-num">{progress}/{target}</span>
                      </div>
                    )}
                    <span className="sr-only">{b.earned ? 'earned' : `not earned yet${target > 0 ? `, ${progress} of ${target}` : ''}`}</span>
                  </li>
                )
              })}
            </ul>
          </section>
        ))}
      </div>
    </>
  )
}


/** Leaderboard: your level, points this month and badges. Members and passwords live under Settings. */
export default function Family() {
  const { user } = useAuth()
  const gami = useApi(() => api.gamification(), [])
  const { reload } = gami
  useEffect(() => onSpendingChanged(reload), [reload])

  const g = gami.data
  const failed = !!gami.error && !g

  return (
    <div className="family-page">
      <PageHeader
        title="Leaderboard"
        subtitle={user?.household_name || undefined}
      />

      {gami.error && (
        <div className="error-box" role="alert">
          <p>{failed ? 'Could not load points and badges' : 'Could not refresh points and badges'}: {gami.error}</p>
          <button className="secondary" onClick={reload}>Try again</button>
        </div>
      )}

      {g && <LevelCard g={g} />}
      {!g && !failed && <SkeletonStats count={3} />}
      {g && (
        <div className="cards-row">
          <StatCard label="Your points" value={`${g.member?.points ?? 0} pts`} sub={`${g.member?.points_month ?? 0} this month`} />
          <StatCard label="Your streak" value={`${g.member?.streak_days ?? 0} days`} tone="positive" sub={`Best: ${g.member?.best_streak ?? 0} days`} />
          <StatCard label="Household total" value={`${g.points} pts`} tone="savings" sub={`${g.streak_days}-day household streak`} />
        </div>
      )}

      {!failed && (
        <div className="card">
          <h2>Points this month</h2>
          {!g ? <SkeletonCard lines={3} /> : (
            <ol className="leaderboard">
              {g.leaderboard.map((r, i) => (
                <li key={r.member_id} className={r.member_id === g.member?.id ? 'me' : ''}>
                  <span className={`leader-rank${i < 3 ? ` rank-${i + 1}` : ''}`} aria-hidden="true">{i + 1}</span>
                  <span className="leader-name">{r.display_name} <RoleBadge role={r.role} /></span>
                  <span className="leader-points">{r.points_month} pts</span>
                </li>
              ))}
            </ol>
          )}
          <p className="muted">Log spending every day to earn points (10 per day plus a streak bonus), and finish chores for extra points.</p>
        </div>
      )}

      {!failed && <Achievements g={g} />}
    </div>
  )
}
