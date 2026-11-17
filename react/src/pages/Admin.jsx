import { useState } from 'react'
import { api } from '../api'
import { useApi } from '../useApi'
import { useToast, useHiddenIds } from '../Toast.jsx'
import PageHeader from '../PageHeader.jsx'
import { SkeletonTable } from '../Skeleton.jsx'
import { IconClose } from '../icons.jsx'

export default function Admin() {
  const res = useApi(() => api.adminAccounts(), [])
  const toast = useToast()
  const [hidden, hide, unhide] = useHiddenIds()
  const [resetPending, setResetPending] = useState(false)

  // Destructive + irreversible, so it's deferred behind an Undo toast too.
  function resetDemoData() {
    setResetPending(true)
    toast.undoable({
      message: 'Resetting demo1/demo2 sample data...',
      commit: async () => {
        const d = await api.adminResetDemoData()
        toast.success(`Reset: ${d.accounts_reset.map(a => a.email).join(', ')}`)
      },
      onUndo: () => setResetPending(false),
      onCommitted: () => { setResetPending(false); res.reload() },
      onError: e => { setResetPending(false); toast.error(e.message) },
    })
  }

  async function update(row, payload) {
    try {
      await api.adminUpdateAccount(row.id, payload)
      res.reload()
    } catch (e) { toast.error(e.message) }
  }

  function remove(row) {
    hide(row.id)
    toast.undoable({
      message: `Deleted account "${row.email}" and all its data`,
      commit: () => api.adminDeleteAccount(row.id),
      onUndo: () => unhide(row.id),
      onCommitted: res.reload,
    })
  }

  const rows = (res.data || []).filter(r => !hidden.has(r.id))

  return (
    <div>
      <PageHeader title="Super Admin" subtitle="All accounts on this instance. Visible only to admins." />
      {res.error && <div className="error-box">{res.error}</div>}

      <div className="card" style={{ marginBottom: '1rem' }}>
        <button onClick={resetDemoData} disabled={resetPending}>
          {resetPending ? 'Reset scheduled...' : 'Reset Demo Data'}
        </button>
        <span className="muted" style={{ marginLeft: '0.75rem' }}>Wipes and re-seeds demo1/demo2 with fresh randomized values.</span>
      </div>

      <div className="card table-scroll">
        {!res.data ? <SkeletonTable rows={5} cols={6} /> : (
          <table className="simple-table responsive-table">
            <thead>
              <tr><th>Email</th><th>Name</th><th>Currency</th><th>Transactions</th><th>Members</th><th>Integrations</th><th>Admin</th><th>Status</th><th>Created</th><th><span className="sr-only">Actions</span></th></tr>
            </thead>
            <tbody>
              {rows.map(r => (
                <tr key={r.id}>
                  <td data-label="Email">{r.email}</td>
                  <td data-label="Name">{r.display_name}</td>
                  <td data-label="Currency">{r.currency}</td>
                  <td data-label="Transactions">{r.spending_count}</td>
                  <td data-label="Members">{r.member_count ?? 1}</td>
                  <td data-label="Integrations">{r.integration_count}</td>
                  <td data-label="Admin"><input type="checkbox" aria-label={`Admin: ${r.email}`} checked={!!r.is_admin} onChange={() => update(r, { is_admin: !r.is_admin })} /></td>
                  <td data-label="Status">
                    <button className={r.is_disabled ? 'secondary' : ''} onClick={() => update(r, { is_disabled: !r.is_disabled })} aria-label={`${r.is_disabled ? 'Enable' : 'Disable'} ${r.email}`}>
                      {r.is_disabled ? 'Disabled' : 'Active'}
                    </button>
                  </td>
                  <td data-label="Created">{r.created_at}</td>
                  <td className="row-actions"><button className="icon-btn danger round" onClick={() => remove(r)} aria-label={`Delete account ${r.email}`} title="Delete"><IconClose width={12} height={12} /></button></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
