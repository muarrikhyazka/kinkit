import { useEffect, useState } from 'react'
import { api, formatRupiah, onSpendingChanged } from '../api'
import { useApi } from '../useApi'
import { useAuth, ROLE_LABELS } from '../AuthContext.jsx'
import { useToast, useHiddenIds } from '../Toast.jsx'
import Modal from '../Modal.jsx'
import PageHeader from '../PageHeader.jsx'
import StatCard from '../StatCard.jsx'
import MoneyInput from '../MoneyInput.jsx'
import { SkeletonCard, SkeletonStats } from '../Skeleton.jsx'
import { IconClose, IconEdit, IconPlus } from '../icons.jsx'

// nav icon for routes.jsx (two people)
export const IconFamily = (p) => (
  <svg width={20} height={20} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinecap="round" strokeLinejoin="round" {...p}>
    <circle cx="9" cy="8" r="3.2" /><path d="M3 20c0-3.3 2.7-6 6-6s6 2.7 6 6" />
    <circle cx="17" cy="9.5" r="2.4" /><path d="M16 14.2c2.9-.4 5 1.8 5 4.8" />
  </svg>
)

const ROLE_HELP = {
  parent: 'Full access to the household, can manage members',
  kid: 'Can view and log their own spending; has an optional monthly allowance',
  viewer: 'Read-only access',
}

function RoleBadge({ role }) {
  return <span className={`badge role-badge role-${role}`}>{ROLE_LABELS[role] || role}</span>
}

function AllowanceBar({ member }) {
  const allowance = Number(member.allowance_monthly) || 0
  if (!allowance) return null
  const pct = Math.min(100, (member.spent_this_month / allowance) * 100)
  const over = member.spent_this_month > allowance
  return (
    <div className="allowance">
      <div className="progress-bar" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(pct)}
        aria-label={`${member.display_name} allowance used`}>
        <div className={`progress-fill ${over ? 'over' : ''}`} style={{ width: `${pct}%` }} />
      </div>
      <div className="muted">
        {formatRupiah(member.spent_this_month)} of {formatRupiah(allowance)} this month
        {over ? ` - over by ${formatRupiah(member.spent_this_month - allowance)}` : ` - ${formatRupiah(allowance - member.spent_this_month)} left`}
      </div>
    </div>
  )
}

function MemberModal({ member, myRole, onClose, onSaved }) {
  const toast = useToast()
  const editing = !!member
  const isOwnerRow = member?.role === 'owner'
  const [form, setForm] = useState({
    email: member?.email || '',
    display_name: member?.display_name || '',
    role: member?.role || 'kid',
    allowance: member?.allowance_monthly ?? '',
    password: '',
  })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const set = (k, v) => setForm(f => ({ ...f, [k]: v }))

  async function submit(e) {
    e.preventDefault()
    setError('')
    if (!editing && form.password.length < 8) { setError('Temporary password must be at least 8 characters.'); return }
    if (editing && form.password && form.password.length < 8) { setError('New password must be at least 8 characters.'); return }
    const allowance = form.role === 'kid' ? (Number(form.allowance) || null) : null
    setBusy(true)
    try {
      if (editing) {
        const payload = { display_name: form.display_name }
        if (!isOwnerRow) { payload.role = form.role; payload.allowance_monthly = allowance }
        if (form.password) payload.password = form.password
        await api.updateMember(member.id, payload)
        toast.success(`Saved ${form.display_name || member.email}`)
      } else {
        await api.addMember({ email: form.email, display_name: form.display_name, role: form.role, password: form.password, allowance_monthly: allowance })
        toast.success(`Added ${form.display_name || form.email} - share the email and temporary password with them`)
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal title={editing ? `Edit ${member.display_name}` : 'Add family member'} onClose={onClose} className="family-modal">
      <form className="family-form" onSubmit={submit}>
        {error && <div className="error-box" role="alert">{error}</div>}
        {!editing && (
          <label>Email (their login)
            <input data-autofocus type="email" value={form.email} onChange={e => set('email', e.target.value)} required />
          </label>
        )}
        <label>Name
          <input data-autofocus={editing ? true : undefined} value={form.display_name} onChange={e => set('display_name', e.target.value)} placeholder="optional" />
        </label>
        {!isOwnerRow && (
          <label>Role
            <select value={form.role} onChange={e => set('role', e.target.value)} aria-describedby="role-help">
              <option value="parent">Parent</option>
              <option value="kid">Kid</option>
              <option value="viewer">Viewer</option>
            </select>
            <span id="role-help" className="muted">{ROLE_HELP[form.role]}</span>
          </label>
        )}
        {!isOwnerRow && form.role === 'kid' && (
          <label>Monthly allowance
            <MoneyInput value={form.allowance === null ? '' : String(form.allowance)} onChange={v => set('allowance', v)} placeholder="e.g. 200k (empty = none)" />
          </label>
        )}
        <label>{editing ? 'Reset password (optional)' : 'Temporary password'}
          <input type="text" autoComplete="new-password" value={form.password} onChange={e => set('password', e.target.value)}
            minLength={editing ? undefined : 8} required={!editing} placeholder="8+ characters" />
          {!editing && <span className="muted">They can change it later from the Family page.</span>}
        </label>
        {myRole === 'parent' && isOwnerRow && <p className="muted">Parents can't edit the owner.</p>}
        <div className="family-form-actions">
          <button type="button" className="secondary" onClick={onClose}>Cancel</button>
          <button type="submit" disabled={busy}>{busy ? 'Saving...' : editing ? 'Save' : 'Add member'}</button>
        </div>
      </form>
    </Modal>
  )
}

function ChangePassword() {
  const toast = useToast()
  const [cur, setCur] = useState('')
  const [next, setNext] = useState('')
  async function submit(e) {
    e.preventDefault()
    try {
      await api.changePassword({ current_password: cur, new_password: next })
      setCur(''); setNext('')
      toast.success('Password changed')
    } catch (err) { toast.error(err.message) }
  }
  return (
    <form className="inline-form" onSubmit={submit} aria-label="Change your password">
      <input type="password" autoComplete="current-password" placeholder="Current password" aria-label="Current password" value={cur} onChange={e => setCur(e.target.value)} required />
      <input type="password" autoComplete="new-password" placeholder="New password (8+)" aria-label="New password" minLength={8} value={next} onChange={e => setNext(e.target.value)} required />
      <button type="submit" className="secondary">Change password</button>
    </form>
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
                <span className="challenge-icon" aria-hidden="true">{c.icon}</span>
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
                          ? `${formatRupiah(c.progress)} of ${formatRupiah(c.target)} ${over ? '- over the limit' : 'so far'}`
                          : `${c.progress} / ${c.target} days`}
                        {c.last_week_done && ' · completed last week too'}
                      </div>
                    </>
                  ) : (
                    <div className="muted">Set a Wants budget for this month on the Budgeting page to unlock.</div>
                  )}
                </div>
              </li>
            )
          })}
        </ul>
      </div>

      <div className="card" id="badges">
        <h2>Your badges <span className="muted">({earned}/{g.badges.length})</span></h2>
        <ul className="badge-grid">
          {g.badges.map(b => (
            <li key={b.key} className={`badge-tile ${b.earned ? 'earned' : ''}`} title={b.description}>
              <span className="badge-tile-icon" aria-hidden="true">{b.icon}</span>
              <strong>{b.name}</strong>
              <span className="muted">{b.earned ? `Earned ${b.earned_at?.slice(0, 10)}` : b.description}</span>
              <span className="sr-only">{b.earned ? 'earned' : 'not earned yet'}</span>
            </li>
          ))}
        </ul>
      </div>
    </>
  )
}

export default function Family() {
  const { user, refreshUser } = useAuth()
  const toast = useToast()
  const [hidden, hide, unhide] = useHiddenIds()
  const [modal, setModal] = useState(null) // null | 'add' | member
  const fam = useApi(() => api.family(), [])
  const gami = useApi(() => api.gamification(), [])
  const { reload: reloadFam } = fam
  const { reload: reloadGami } = gami
  useEffect(() => onSpendingChanged(() => { reloadFam(); reloadGami() }), [reloadFam, reloadGami])

  const data = fam.data
  const g = gami.data
  const myRole = data?.me?.role || user?.role
  const canManage = !!data?.can_manage
  const members = (data?.members || []).filter(m => !hidden.has(m.id))
  const kids = members.filter(m => m.role === 'kid')
  const me = members.find(m => m.is_me)

  function remove(m) {
    hide(m.id)
    toast.undoable({
      message: `Removed ${m.display_name}`,
      commit: () => api.deleteMember(m.id),
      onUndo: () => unhide(m.id),
      onCommitted: () => { reloadFam(); reloadGami() },
      onError: e => { unhide(m.id); toast.error(e.message) },
    })
  }

  const canEdit = m => canManage && !(m.role === 'owner' && myRole !== 'owner')
  const canRemove = m => canManage && m.role !== 'owner' && !m.is_me

  return (
    <div className="family-page">
      <PageHeader
        title="Family"
        subtitle={data ? `${data.household.name} · ${members.length} member${members.length === 1 ? '' : 's'}` : undefined}
        actions={canManage && <button onClick={() => setModal('add')}><IconPlus width={16} height={16} aria-hidden="true" /> Add member</button>}
      />
      {fam.error && <div className="error-box" role="alert">{fam.error}</div>}

      {!g ? <SkeletonStats count={3} /> : (
        <div className="cards-row">
          <StatCard label="Your points" value={`${g.member?.points ?? 0} pts`} sub={`${g.member?.points_month ?? 0} this month · ${g.member?.level || ''}`} />
          <StatCard label="Your streak" value={`${g.member?.streak_days ?? 0} days`} tone="positive" sub={`Best: ${g.member?.best_streak ?? 0} days`} />
          <StatCard label="Household total" value={`${g.points} pts`} tone="savings" sub={`${g.streak_days}-day household streak`} />
        </div>
      )}

      {!data ? <SkeletonCard lines={4} /> : (
        <div className="card">
          <h2>Members</h2>
          <ul className="member-list">
            {members.map(m => (
              <li key={m.id} className="member-row">
                <div className="member-avatar" aria-hidden="true">{(m.display_name || m.email).slice(0, 1).toUpperCase()}</div>
                <div className="member-main">
                  <div className="member-name">
                    <strong>{m.display_name}</strong> <RoleBadge role={m.role} />
                    {m.is_me && <span className="muted"> (you)</span>}
                  </div>
                  <div className="muted">{m.email}</div>
                  {m.role === 'kid' && <AllowanceBar member={m} />}
                </div>
                <div className="row-actions">
                  {canEdit(m) && (
                    <button className="icon-btn" onClick={() => setModal(m)} aria-label={`Edit ${m.display_name}`} title="Edit"><IconEdit width={15} height={15} /></button>
                  )}
                  {canRemove(m) && (
                    <button className="icon-btn danger" onClick={() => remove(m)} aria-label={`Remove ${m.display_name}`} title="Remove"><IconClose width={14} height={14} /></button>
                  )}
                </div>
              </li>
            ))}
          </ul>
          {canManage && members.length === 1 && (
            <p className="muted">Add your partner, kids or anyone else in the household. Each person gets their own login, points and badges.</p>
          )}
        </div>
      )}

      {kids.length > 0 && kids.some(k => Number(k.allowance_monthly) > 0) && (
        <div className="card">
          <h2>Allowances ({data.month})</h2>
          {kids.filter(k => Number(k.allowance_monthly) > 0).map(k => (
            <div key={k.id} className="allowance-row">
              <strong>{k.display_name}</strong>
              <AllowanceBar member={k} />
            </div>
          ))}
        </div>
      )}

      <div className="card">
        <h2>Leaderboard <span className="muted">(points this month)</span></h2>
        {!g ? <SkeletonCard lines={3} /> : (
          <ol className="leaderboard">
            {g.leaderboard.map((r, i) => (
              <li key={r.member_id} className={r.member_id === g.member?.id ? 'me' : ''}>
                <span className="leader-rank" aria-hidden="true">{['🥇', '🥈', '🥉'][i] || i + 1}</span>
                <span className="leader-name">{r.display_name} <RoleBadge role={r.role} /></span>
                <span className="leader-points">{r.points_month} pts</span>
              </li>
            ))}
          </ol>
        )}
        <p className="muted">Log spending every day to earn points: 10 per day plus a streak bonus.</p>
      </div>

      <Achievements g={g} />

      <div className="card">
        <h2>Your login</h2>
        <p className="muted">
          Signed in as <strong>{me?.display_name || user?.display_name}</strong> ({me?.email || user?.username}) · <RoleBadge role={myRole} />
        </p>
        <ChangePassword />
      </div>

      {modal && (
        <MemberModal
          member={modal === 'add' ? null : modal}
          myRole={myRole}
          onClose={() => setModal(null)}
          onSaved={() => { reloadFam(); reloadGami(); refreshUser() }}
        />
      )}
    </div>
  )
}
