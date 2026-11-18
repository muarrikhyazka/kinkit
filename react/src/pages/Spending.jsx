import { useEffect, useState } from 'react'
import { api, formatRupiah, MONTHS, onSpendingChanged } from '../api'
import { useYearMonth } from '../YearContext.jsx'
import { useApi, useDebounced, useSetup } from '../useApi'
import { useToast, useHiddenIds } from '../Toast.jsx'
import { categoryColor } from '../categoryColor'
import MoneyInput from '../MoneyInput.jsx'
import { IconClose, IconEdit, IconChevronLeft, IconChevronRight } from '../icons.jsx'
import CategorySelect from '../CategorySelect.jsx'
import SpendingForm from '../SpendingForm.jsx'
import PageHeader from '../PageHeader.jsx'
import SortableTh from '../SortableTh.jsx'
import { SkeletonTable } from '../Skeleton.jsx'
import { ReceiptButton } from '../Receipts.jsx'
import { ChevronLink } from '../ChevronLink.jsx'

const PAGE_SIZES = [10, 20, 50, 100]

export default function Spending() {
  const { year, month, valueMax } = useYearMonth()
  const { categories } = useSetup()
  const toast = useToast()
  const [page, setPage] = useState(1)
  const [perPage, setPerPage] = useState(20)
  const [search, setSearch] = useState('')
  const [categoryFilter, setCategoryFilter] = useState('')
  const [sort, setSort] = useState('tx_date')
  const [order, setOrder] = useState('desc')
  const [editingId, setEditingId] = useState(null)
  const [editRow, setEditRow] = useState(null)
  const [hidden, hide, unhide] = useHiddenIds()
  const debouncedSearch = useDebounced(search)

  const list = useApi(
    () => api.spending(year, month, page, perPage, { search: debouncedSearch, category: categoryFilter, sort, order, valueMax }),
    [year, month, page, perPage, debouncedSearch, categoryFilter, sort, order, valueMax],
  )
  const { reload } = list

  // back to page 1 whenever the result set changes shape
  useEffect(() => { setPage(1) }, [year, month, perPage, debouncedSearch, categoryFilter, sort, order, valueMax])
  // refresh after a save from the quick-add modal / offline sync
  useEffect(() => onSpendingChanged(reload), [reload])

  const rows = (list.data?.rows || []).filter(r => !hidden.has(r.id))
  // Show who logged each row once more than one household member appears.
  const showMember = new Set(rows.map(r => r.member_name).filter(Boolean)).size > 1
  const total = Math.max(0, (list.data?.total || 0) - ((list.data?.rows || []).length - rows.length))
  const totalPages = Math.max(1, Math.ceil(total / perPage))

  function toggleSort(col) {
    if (sort === col) setOrder(o => (o === 'asc' ? 'desc' : 'asc'))
    else { setSort(col); setOrder('asc') }
  }

  function handleDelete(row) {
    hide(row.id)
    toast.undoable({
      message: `Deleted "${row.description}"`,
      commit: () => api.deleteSpending(row.id),
      onUndo: () => unhide(row.id),
      onCommitted: reload, // id stays hidden; it's gone server-side now
    })
  }

  async function handleCheck(row) {
    try {
      await api.updateSpending(row.id, { checked: row.checked ? 0 : 1 })
      reload()
    } catch (err) { toast.error(err.message) }
  }

  function startEdit(row) {
    setEditingId(row.id)
    setEditRow({ tx_date: row.tx_date, description: row.description, category: row.category_name || '', amount: row.amount })
  }
  function cancelEdit() {
    setEditingId(null)
    setEditRow(null)
  }
  async function saveEdit(id) {
    try {
      await api.updateSpending(id, { ...editRow, amount: Number(editRow.amount) })
      cancelEdit()
      toast.success('Transaction updated')
      reload()
    } catch (err) { toast.error(err.message) }
  }
  function editKeyDown(e, id) {
    if (e.key === 'Escape') { e.preventDefault(); cancelEdit() }
    if (e.key === 'Enter' && e.target.tagName === 'INPUT' && e.target.type !== 'checkbox') { e.preventDefault(); saveEdit(id) }
  }

  return (
    <div>
      <PageHeader
        title="Spending"
        subtitle={`${total} transactions - ${MONTHS[month - 1]} ${year}`}
        period="month"
        valueFilter
        actions={<ChevronLink to="/settings/data" title="Import a bank CSV or export your data">Import / export</ChevronLink>}
      />

      {list.error && <div className="error-box">{list.error}</div>}

      <div className="card">
        <SpendingForm />
      </div>

      <div className="card inline-form filters-bar">
        <input type="search" placeholder="Search description..." aria-label="Search description" value={search} onChange={e => setSearch(e.target.value)} />
        <CategorySelect value={categoryFilter} onChange={setCategoryFilter} categories={categories} placeholder="All categories" aria-label="Filter by category" />
      </div>

      <div className="card table-scroll">
        {list.loading && !list.data ? <SkeletonTable rows={6} cols={5} /> : (
          <table className="responsive-table">
            <thead>
              <tr>
                <SortableTh col="tx_date" sort={sort} order={order} onSort={toggleSort}>Date</SortableTh>
                <SortableTh col="description" sort={sort} order={order} onSort={toggleSort}>Description</SortableTh>
                <SortableTh col="category_name" sort={sort} order={order} onSort={toggleSort}>Category</SortableTh>
                <SortableTh col="amount" sort={sort} order={order} onSort={toggleSort} className="num">Amount</SortableTh>
                <th>Checked</th>
                <th><span className="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              {rows.map(r => editingId === r.id ? (
                <tr key={r.id} className="editing" onKeyDown={e => editKeyDown(e, r.id)}>
                  <td data-label="Date"><input type="date" aria-label="Date" value={editRow.tx_date} onChange={e => setEditRow(v => ({ ...v, tx_date: e.target.value }))} /></td>
                  <td data-label="Description"><input aria-label="Description" value={editRow.description} onChange={e => setEditRow(v => ({ ...v, description: e.target.value }))} autoFocus /></td>
                  <td data-label="Category">
                    <CategorySelect value={editRow.category} onChange={v => setEditRow(row => ({ ...row, category: v }))} categories={categories} />
                  </td>
                  <td data-label="Amount"><MoneyInput aria-label="Amount" value={editRow.amount} onChange={v => setEditRow(row => ({ ...row, amount: v }))} /></td>
                  <td data-label="Checked"><input type="checkbox" aria-label="Checked" checked={!!r.checked} onChange={() => handleCheck(r)} /></td>
                  <td className="row-actions">
                    <button onClick={() => saveEdit(r.id)}>Save</button>
                    <button className="secondary" onClick={cancelEdit}>Cancel</button>
                  </td>
                </tr>
              ) : (
                <tr key={r.id}>
                  <td data-label="Date">{r.tx_date}</td>
                  <td data-label="Description">
                    {r.description}
                    {showMember && r.member_name && <div className="muted logged-by">by {r.member_name}</div>}
                  </td>
                  <td data-label="Category"><span className="tag" style={{ background: categoryColor(r.category_name) + '22', color: categoryColor(r.category_name) }}>{r.category_name || 'Other'}</span></td>
                  <td data-label="Amount" className="num">{formatRupiah(r.amount)}</td>
                  <td data-label="Checked"><input type="checkbox" aria-label={`Mark "${r.description}" as checked`} checked={!!r.checked} onChange={() => handleCheck(r)} /></td>
                  <td className="row-actions">
                    <ReceiptButton row={r} onChanged={reload} />
                    <button className="icon-btn round" onClick={() => startEdit(r)} aria-label={`Edit "${r.description}"`} title="Edit"><IconEdit width={14} height={14} /></button>
                    <button className="icon-btn danger round" onClick={() => handleDelete(r)} aria-label={`Delete "${r.description}"`} title="Delete"><IconClose width={14} height={14} /></button>
                  </td>
                </tr>
              ))}
              {rows.length === 0 && <tr><td colSpan={6} className="muted table-empty">{search || categoryFilter ? 'No transactions match this search.' : 'No transactions this month yet. Add one with the form above.'}</td></tr>}
            </tbody>
          </table>
        )}

        <div className="table-footer">
          <label className="field-inline">
            <span className="muted">Rows:</span>
            <select value={perPage} onChange={e => setPerPage(Number(e.target.value))}>
              {PAGE_SIZES.map(n => <option key={n} value={n}>{n}</option>)}
            </select>
          </label>

          {totalPages > 1 && (
            <div className="pagination">
              <button className="secondary" disabled={page <= 1} onClick={() => setPage(p => p - 1)}><IconChevronLeft width={16} height={16} aria-hidden="true" /> Prev</button>
              <span className="muted">Page {page} / {totalPages}</span>
              <button className="secondary" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>Next <IconChevronRight width={16} height={16} aria-hidden="true" /></button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
