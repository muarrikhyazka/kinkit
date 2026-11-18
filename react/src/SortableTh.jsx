import { IconSortUp, IconSortDown } from './icons.jsx'

/**
 * Keyboard-reachable sortable column header with aria-sort.
 *   <SortableTh col="amount" sort={sort} order={order} onSort={toggleSort}>Amount</SortableTh>
 */
export default function SortableTh({ col, sort, order, onSort, children, className }) {
  const active = sort === col
  return (
    <th className={className} aria-sort={active ? (order === 'asc' ? 'ascending' : 'descending') : 'none'}>
      <button type="button" className="th-sort" onClick={() => onSort(col)}>
        {children}
        {active && (order === 'asc' ? <IconSortUp width={11} height={11} aria-hidden="true" /> : <IconSortDown width={11} height={11} aria-hidden="true" />)}
      </button>
    </th>
  )
}
