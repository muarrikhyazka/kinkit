import { useState } from 'react'
import { api } from '../api'
import { useAuth } from '../AuthContext.jsx'
import { useToast } from '../Toast.jsx'
import PageHeader from '../PageHeader.jsx'
import { RoleBadge } from './Members.jsx'

function ChangePassword() {
  const toast = useToast()
  const [cur, setCur] = useState('')
  const [next, setNext] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      await api.changePassword({ current_password: cur, new_password: next })
      setCur(''); setNext('')
      toast.success('Password changed')
    } catch (err) {
      setError(err.status === 401 ? 'The current password is wrong. Check it and try again.' : err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="family-form" onSubmit={submit} aria-label="Change your password">
      {error && <div className="error-box" role="alert">{error}</div>}
      <label>Current password
        <input type="password" autoComplete="current-password" value={cur} onChange={e => setCur(e.target.value)} required />
      </label>
      <label>New password
        <input type="password" autoComplete="new-password" minLength={8} value={next} onChange={e => setNext(e.target.value)} required />
        <span className="muted">At least 8 characters.</span>
      </label>
      <div className="family-form-actions">
        <button type="submit" disabled={busy}>{busy ? 'Saving...' : 'Change password'}</button>
      </div>
    </form>
  )
}

/** Settings › Your account: who you are signed in as, and your own password. Every role can open it. */
export default function Account() {
  const { user } = useAuth()
  return (
    <div className="family-page">
      <PageHeader title="Your account" />
      <div className="card">
        <p>
          Signed in as <strong>{user?.display_name}</strong> ({user?.username}) · <RoleBadge role={user?.role || 'owner'} />
        </p>
      </div>
      <div className="card">
        <h2>Change password</h2>
        <ChangePassword />
      </div>
    </div>
  )
}
