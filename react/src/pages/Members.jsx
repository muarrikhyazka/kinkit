import { useEffect, useState } from 'react'
import { api, formatRupiah, onSpendingChanged } from '../api'
import { useApi } from '../useApi'
import { useAuth, ROLE_LABELS } from '../AuthContext.jsx'
import { useToast, useHiddenIds } from '../Toast.jsx'
import Modal from '../Modal.jsx'
import PageHeader from '../PageHeader.jsx'
import MoneyInput from '../MoneyInput.jsx'
import { SkeletonCard } from '../Skeleton.jsx'
import { IconClose, IconEdit, IconPlus } from '../icons.jsx'

const ROLE_HELP = {
  parent: 'Full access to the household, can manage members',
  kid: 'Can view and log their own spending; has an optional monthly allowance',
  viewer: 'Read-only access',
}

export function RoleBadge({ role }) {
  return <span className={`badge role-badge role-${role}`}>{ROLE_LABELS[role] || role}</span>
}

// Effective allowance = monthly allowance + money earned from chores this
// month (shown as two parts).
function AllowanceBar({ member }) {
  const base = Number(member.allowance_monthly) || 0
  const earned = Number(member.chore_earned_month) || 0
  const allowance = base + earned
  if (!allowance) return null
  const pct = Math.min(100, (member.spent_this_month / allowance) * 100)
  const over = member.spent_this_month > allowance
  return (
    <div className="allowance">
      {earned > 0 && (
        <div className="allowance-parts muted">
          {base > 0 && <span>Allowance {formatRupiah(base)}</span>}
          {base > 0 && <span aria-hidden="true"> + </span>}
          <span className="allowance-earned">{formatRupiah(earned)} earned from chores</span>
          <span> = {formatRupiah(allowance)}</span>
        </div>
      )}
      <div className="progress-bar" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(pct)}
        aria-label={`${member.display_name} allowance used`}>
        <div className={`progress-fill ${over ? 'over' : ''}`} style={{ width: `${pct}%` }} />
      </div>
      <div className="muted">
        {formatRupiah(member.spent_this_month)} of {formatRupiah(allowance)} this month
        {over ? `, over by ${formatRupiah(member.spent_this_month - allowance)}` : `, ${formatRupiah(allowance - member.spent_this_month)} left`}
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
        toast.success(`Added ${form.display_name || form.email}. Share the email and temporary password with them.`)
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
          {!editing && <span className="muted">They can change it later under Settings › Your account.</span>}
        </label>
        {myRole === 'parent' && isOwnerRow && <p className="muted">Parents can't edit the family admin.</p>}
        <div className="family-form-actions">
          <button type="button" className="secondary" onClick={onClose}>Cancel</button>
          <button type="submit" disabled={busy}>{busy ? 'Saving...' : editing ? 'Save' : 'Add member'}</button>
        </div>
      </form>
    </Modal>
  )
}

/** Settings › Members: everyone in the household, their role and allowance. Owners and parents can change them. */
export default function Members() {
  const { user, refreshUser } = useAuth()
  const toast = useToast()
  const [hidden, hide, unhide] = useHiddenIds()
  const [modal, setModal] = useState(null) // null | 'add' | member
  const fam = useApi(() => api.family(), [])
  const { reload } = fam
  useEffect(() => onSpendingChanged(reload), [reload])

  const data = fam.data
  const myRole = data?.me?.role || user?.role
  const canManage = !!data?.can_manage
  const members = (data?.members || []).filter(m => !hidden.has(m.id))
  const kids = members.filter(m => m.role === 'kid')
  const kidsWithMoney = kids.filter(k => Number(k.allowance_monthly) > 0 || Number(k.chore_earned_month) > 0)

  function remove(m) {
    hide(m.id)
    toast.undoable({
      message: `Removed ${m.display_name}`,
      commit: () => api.deleteMember(m.id),
      onUndo: () => unhide(m.id),
      onCommitted: reload,
      onError: e => { unhide(m.id); toast.error(e.message) },
    })
  }

  const canEdit = m => canManage && !(m.role === 'owner' && myRole !== 'owner')
  const canRemove = m => canManage && m.role !== 'owner' && !m.is_me

  return (
    <div className="family-page">
      <PageHeader
        title="Members"
        subtitle={data ? `${data.household.name} · ${members.length} member${members.length === 1 ? '' : 's'}` : undefined}
        actions={canManage && <button onClick={() => setModal('add')}><IconPlus width={16} height={16} aria-hidden="true" /> Add member</button>}
      />

      {fam.error && !data && (
        <div className="error-box" role="alert">
          <p>Could not load the members list: {fam.error}</p>
          <button className="secondary" onClick={reload}>Try again</button>
        </div>
      )}
      {fam.error && data && <div className="error-box" role="alert">Could not refresh the members list: {fam.error} <button className="secondary" onClick={reload}>Try again</button></div>}

      {!data && !fam.error && <SkeletonCard lines={4} />}

      {data && (
        <>
          {!canManage && <p className="muted">Only the family admin and parents can add or change members.</p>}
          <div className="card">
            <h2>Household</h2>
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
                    {m.chores_done_week > 0 && <div className="muted">{m.chores_done_week} chore{m.chores_done_week === 1 ? '' : 's'} done this week</div>}
                    {m.role === 'kid' && <AllowanceBar member={m} />}
                  </div>
                  <div className="row-actions">
                    {canEdit(m) && (
                      <button className="icon-btn" onClick={() => setModal(m)} aria-label={`Edit ${m.display_name}`} title="Edit"><IconEdit width={15} height={15} aria-hidden="true" /></button>
                    )}
                    {canRemove(m) && (
                      <button className="icon-btn danger" onClick={() => remove(m)} aria-label={`Remove ${m.display_name}`} title="Remove"><IconClose width={14} height={14} aria-hidden="true" /></button>
                    )}
                  </div>
                </li>
              ))}
            </ul>
            {canManage && members.length === 1 && (
              <p className="muted">You are the only member so far. Use Add member to give your partner, kids or anyone else in the household their own login, points and badges.</p>
            )}
          </div>

          {kidsWithMoney.length > 0 && (
            <div className="card">
              <h2>Allowances ({data.month})</h2>
              {kidsWithMoney.map(k => (
                <div key={k.id} className="allowance-row">
                  <strong>{k.display_name}</strong>
                  <AllowanceBar member={k} />
                </div>
              ))}
            </div>
          )}
        </>
      )}

      {modal && (
        <MemberModal
          member={modal === 'add' ? null : modal}
          myRole={myRole}
          onClose={() => setModal(null)}
          onSaved={() => { reload(); refreshUser() }}
        />
      )}
    </div>
  )
}
