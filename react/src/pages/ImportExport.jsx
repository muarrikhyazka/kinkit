import { useState } from 'react'
import { dataApi, formatRupiah, notifySpendingChanged } from '../api'
import { useApi, useSetup } from '../useApi'
import { useToast } from '../Toast.jsx'
import PageHeader from '../PageHeader.jsx'
import CategorySelect from '../CategorySelect.jsx'
import TVTokens from '../TVTokens.jsx'
import { IconDownload } from '../Receipts.jsx'

const MAP_FIELDS = [
  ['date', 'Date'],
  ['description', 'Description'],
  ['amount', 'Amount (one column)'],
  ['debit', 'Debit / money out'],
  ['credit', 'Credit / money in'],
  ['drcr', 'DB/CR marker'],
  ['category', 'Category'],
]
const PREVIEW_LIMIT = 300
const STATUS_LABEL = { ok: 'Import', skip: 'Skip', error: 'Error', duplicate: 'Already imported' }

function ExportCard() {
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  return (
    <div className="card">
      <h2 className="card-title-icon"><IconDownload width={18} height={18} aria-hidden="true" /> Export</h2>
      <p className="muted">Your data is yours. Download it any time. CSV and Excel open in any spreadsheet app. The full backup has everything (categories, budgets, assets, spending) as JSON or as an Excel workbook with one sheet per table.</p>
      <div className="inline-form">
        <label className="field-inline"><span className="muted">From</span><input type="date" value={from} onChange={e => setFrom(e.target.value)} aria-label="Export from date" /></label>
        <label className="field-inline"><span className="muted">To</span><input type="date" value={to} onChange={e => setTo(e.target.value)} aria-label="Export to date" /></label>
        <a className="button-link" href={dataApi.exportSpendingUrl(from, to)} download>Spending CSV</a>
        <a className="button-link" href={dataApi.exportSpendingUrl(from, to, 'xlsx')} download>Spending Excel</a>
      </div>
      <div className="export-actions">
        <a className="button-link secondary" href={dataApi.exportIncomeUrl()} download>Income CSV</a>
        <a className="button-link secondary" href={dataApi.exportIncomeUrl('xlsx')} download>Income Excel</a>
        <a className="button-link secondary" href={dataApi.exportAllUrl()} download>Full backup (JSON)</a>
        <a className="button-link secondary" href={dataApi.exportAllUrl('xlsx')} download>Full backup (Excel)</a>
      </div>
    </div>
  )
}

function ImportWizard({ onImported }) {
  const toast = useToast()
  const { categories } = useSetup()
  const [file, setFile] = useState(null)
  const [preset, setPreset] = useState('generic')
  const [year, setYear] = useState(() => new Date().getUTCFullYear())
  const [dateOrder, setDateOrder] = useState('')
  const [decimal, setDecimal] = useState('')
  const [userMapping, setUserMapping] = useState(null) // null = auto-detect
  const [catMap, setCatMap] = useState({})
  const [overrides, setOverrides] = useState({})
  const [preview, setPreview] = useState(null)
  const [result, setResult] = useState(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const presets = useApi(() => dataApi.importPresets(), [])

  function options(over = {}) {
    const o = { preset, default_year: Number(year) || undefined, date_order: dateOrder || undefined, decimal: decimal || undefined, ...over }
    const m = over.mapping !== undefined ? over.mapping : userMapping
    if (m) o.mapping = m
    else delete o.mapping
    return o
  }

  async function runPreview(f = file, over = {}, cm = catMap) {
    if (!f) return
    setBusy(true)
    setError('')
    try {
      const res = await dataApi.importSpending(f, { options: options(over), category_map: cm }, true)
      setPreview(res)
    } catch (e) {
      setError(e.message)
    } finally { setBusy(false) }
  }

  function pickFile(f) {
    if (!f) return
    if (f.size > 5 * 1024 * 1024) { setError('File too large (max 5 MB).'); return }
    setFile(f)
    setUserMapping(null)
    setOverrides({})
    setCatMap({})
    setResult(null)
    runPreview(f, { mapping: null }, {})
  }

  function changePreset(id) {
    setPreset(id)
    setUserMapping(null)
    const p = (presets.data || []).find(x => x.id === id)
    setDateOrder('')
    setDecimal('')
    if (file) runPreview(file, { preset: id, mapping: null, date_order: p?.date_order, decimal: undefined })
  }

  function changeMapping(field, value) {
    const base = userMapping || { ...(preview?.mapping || {}) }
    const next = { ...base, [field]: Number(value) }
    setUserMapping(next)
    runPreview(file, { mapping: next })
  }

  function changeCatMap(src, cat) {
    const next = { ...catMap, [src]: cat }
    setCatMap(next)
    runPreview(file, {}, next)
  }

  function setRowCategory(row, cat) {
    setOverrides(o => ({ ...o, [row]: { ...o[row], category: cat || 'Other', skip: false } }))
  }
  function toggleSkip(row, skip) {
    setOverrides(o => ({ ...o, [row]: { ...o[row], skip } }))
  }

  async function commit() {
    setBusy(true)
    setError('')
    try {
      const res = await dataApi.importSpending(file, { options: options(), category_map: catMap, overrides }, false)
      setResult(res)
      setPreview(null)
      if (res.imported) { notifySpendingChanged(); onImported && onImported() }
    } catch (e) { setError(e.message) } finally { setBusy(false) }
  }

  async function undo() {
    if (!result?.batch) return
    try {
      const r = await dataApi.undoImport(result.batch)
      toast.success(`Import undone. ${r.deleted} transaction${r.deleted === 1 ? '' : 's'} removed`)
      setResult(null)
      notifySpendingChanged()
      onImported && onImported()
    } catch (e) { toast.error(e.message) }
  }

  function reset() {
    setFile(null); setPreview(null); setResult(null); setUserMapping(null); setOverrides({}); setCatMap({}); setError('')
  }

  const rows = preview?.rows || []
  const willImport = rows.filter(r => r.status === 'ok' && !overrides[r.row]?.skip).length
  const headers = preview?.headers || []
  const mapping = userMapping || preview?.mapping || {}
  const presetInfo = (presets.data || []).find(p => p.id === preset)

  return (
    <div className="card">
      <h2>Import spending from CSV</h2>
      <p className="muted">Upload a bank statement or spreadsheet export. You'll see a preview before anything is saved, and you can undo an import afterwards. Re-importing the same file never creates duplicates.</p>
      {error && <div className="error-box" role="alert">{error}</div>}

      {result ? (
        <div className="import-result" role="status">
          <p><strong>{result.imported}</strong> transaction{result.imported === 1 ? '' : 's'} imported{result.total_amount ? ` (${formatRupiah(result.total_amount)})` : ''}.</p>
          <ul className="guide-list">
            {!!result.summary?.duplicate && <li>{result.summary.duplicate} already imported before, skipped</li>}
            {!!result.summary?.skip && <li>{result.summary.skip} skipped (income, blank or excluded rows)</li>}
            {!!result.summary?.error && <li>{result.summary.error} row{result.summary.error === 1 ? '' : 's'} could not be read</li>}
          </ul>
          <div className="quick-add-actions">
            {result.batch && result.imported > 0 && <button className="secondary" onClick={undo}>Undo this import</button>}
            <button onClick={reset}>Import another file</button>
          </div>
        </div>
      ) : (
        <>
          <div className="import-controls">
            <label className="field-stack">
              <span className="muted">Format</span>
              <select value={preset} onChange={e => changePreset(e.target.value)}>
                {(presets.data || [{ id: 'generic', label: 'Generic' }]).map(p => <option key={p.id} value={p.id}>{p.label}</option>)}
              </select>
            </label>
            <label className="field-stack">
              <span className="muted">CSV file</span>
              <input type="file" accept=".csv,text/csv,text/plain,.txt" onChange={e => pickFile(e.target.files?.[0])} />
            </label>
            <label className="field-stack">
              <span className="muted">Year (for dates without one)</span>
              <input type="number" min="1970" max="2200" value={year} onChange={e => setYear(e.target.value)} onBlur={() => file && runPreview()} />
            </label>
          </div>
          {presetInfo?.note && <p className="muted form-hint">{presetInfo.note}</p>}
        </>
      )}

      {busy && <p className="muted" aria-live="polite">Reading file...</p>}

      {preview && !result && (
        <div className="import-preview">
          {preview.errors?.length > 0 && (
            <div className="error-box">{preview.errors.map((e, i) => <div key={i}>{e}</div>)}</div>
          )}

          <details className="import-mapping" open={!!preview.errors?.length}>
            <summary>Column mapping ({headers.length} columns, header on line {preview.header_line}, separator "{preview.delimiter === '\t' ? 'tab' : preview.delimiter}")</summary>
            <div className="import-controls">
              {MAP_FIELDS.map(([field, label]) => (
                <label key={field} className="field-stack">
                  <span className="muted">{label}</span>
                  <select value={mapping[field] ?? -1} onChange={e => changeMapping(field, e.target.value)}>
                    <option value={-1}>- not used -</option>
                    {headers.map((h, i) => <option key={i} value={i}>{h || `(column ${i + 1})`}</option>)}
                  </select>
                </label>
              ))}
              <label className="field-stack">
                <span className="muted">Date order</span>
                <select value={dateOrder || preview.date_order} onChange={e => { setDateOrder(e.target.value); runPreview(file, { date_order: e.target.value }) }}>
                  <option value="dmy">Day / month / year</option>
                  <option value="mdy">Month / day / year</option>
                  <option value="ymd">Year / month / day</option>
                </select>
              </label>
              <label className="field-stack">
                <span className="muted">Decimal separator</span>
                <select value={decimal || preview.decimal || ''} onChange={e => { setDecimal(e.target.value); runPreview(file, { decimal: e.target.value || undefined }) }}>
                  <option value="">Auto-detect</option>
                  <option value=".">Point (1,234.56)</option>
                  <option value=",">Comma (1.234,56)</option>
                </select>
              </label>
            </div>
          </details>

          {preview.unknown_categories?.length > 0 && (
            <div className="import-catmap">
              <p className="muted">These categories from the file don't exist in Kinkit. Map them (or leave as Other):</p>
              {preview.unknown_categories.map(src => (
                <label key={src} className="field-inline">
                  <span>{src}</span>
                  <CategorySelect value={catMap[src] || ''} onChange={v => changeCatMap(src, v)} categories={categories} placeholder="Other" aria-label={`Category for "${src}"`} />
                </label>
              ))}
            </div>
          )}

          <p className="import-summary">
            <span className="badge import-ok">{willImport} to import</span>
            {!!preview.summary?.duplicate && <span className="badge import-duplicate">{preview.summary.duplicate} already imported</span>}
            {!!preview.summary?.skip && <span className="badge import-skip">{preview.summary.skip} skipped</span>}
            {!!preview.summary?.error && <span className="badge import-error">{preview.summary.error} errors</span>}
          </p>

          <div className="table-scroll">
            <table className="responsive-table import-table">
              <thead>
                <tr><th>#</th><th>Date</th><th>Description</th><th>Amount</th><th>Category</th><th>Status</th><th>Import?</th></tr>
              </thead>
              <tbody>
                {rows.slice(0, PREVIEW_LIMIT).map(r => {
                  const ov = overrides[r.row] || {}
                  const importable = r.status === 'ok'
                  const cat = ov.category ?? r.category
                  return (
                    <tr key={r.row} className={`import-row-${importable && ov.skip ? 'skip' : r.status}`}>
                      <td data-label="#">{r.row}</td>
                      <td data-label="Date">{r.tx_date || '-'}</td>
                      <td data-label="Description">{r.description}{r.source_category ? <span className="muted"> · {r.source_category}</span> : null}</td>
                      <td data-label="Amount">{r.amount ? formatRupiah(r.amount) : '-'}</td>
                      <td data-label="Category">
                        {importable ? (
                          <CategorySelect value={cat === 'Other' ? '' : cat} onChange={v => setRowCategory(r.row, v)} categories={categories} placeholder="Other" aria-label={`Category for row ${r.row}`} disabled={ov.skip} />
                        ) : <span className="muted">-</span>}
                      </td>
                      <td data-label="Status"><span title={r.reason || ''}>{STATUS_LABEL[r.status] || r.status}</span>{r.reason && r.status !== 'ok' ? <div className="muted import-reason">{r.reason}</div> : null}</td>
                      <td data-label="Import?">
                        {importable && <input type="checkbox" checked={!ov.skip} onChange={e => toggleSkip(r.row, !e.target.checked)} aria-label={`Import row ${r.row}`} />}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
          {rows.length > PREVIEW_LIMIT && <p className="muted">Showing the first {PREVIEW_LIMIT} of {rows.length} rows. The rest are imported with their detected category.</p>}

          <div className="quick-add-actions">
            <button className="secondary" onClick={reset}>Cancel</button>
            <button onClick={commit} disabled={busy || willImport === 0}>Import {willImport} transaction{willImport === 1 ? '' : 's'}</button>
          </div>
        </div>
      )}
    </div>
  )
}

function RecentImports({ refreshKey }) {
  const toast = useToast()
  const batches = useApi(() => dataApi.importBatches(), [refreshKey])
  if (!batches.data?.length) return null
  async function undo(b) {
    if (!window.confirm(`Remove all ${b.count} transactions from this import?`)) return
    try {
      await dataApi.undoImport(b.batch)
      toast.success('Import undone')
      batches.reload()
      notifySpendingChanged()
    } catch (e) { toast.error(e.message) }
  }
  return (
    <div className="card">
      <h2>Recent imports</h2>
      <ul className="recent-imports">
        {batches.data.map(b => (
          <li key={b.batch}>
            <span>{b.imported_at.slice(0, 16).replace('T', ' ')} UTC · {b.count} transactions · {formatRupiah(b.total)} · {b.from} to {b.to}</span>
            <button className="secondary" onClick={() => undo(b)}>Undo</button>
          </li>
        ))}
      </ul>
    </div>
  )
}

export default function ImportExport() {
  const [refresh, setRefresh] = useState(0)
  return (
    <div>
      <PageHeader title="Data & devices" subtitle="Import and export your data, and share a read-only view with a TV or car screen." />
      <ExportCard />
      <ImportWizard onImported={() => setRefresh(k => k + 1)} />
      <RecentImports refreshKey={refresh} />
      <TVTokens />
    </div>
  )
}
