import { H2, Callout, Steps, Roles } from '../components.jsx'
import { IconDownload } from '../../Receipts.jsx'

export const slug = 'import-export'
export const title = 'Import and export data'
export const section = 'settings-data'
export const summary = 'Download your data, import a bank CSV with a preview and undo an import.'

export function Content() {
  return (
    <>
      <Roles who={['admin', 'parent']} />
      <p>Open <strong>Settings › Data &amp; devices</strong> in the sidebar (or the <strong>More</strong> sheet on your phone).</p>

      <H2>Export</H2>
      <p><IconDownload width={14} height={14} aria-hidden="true" /> Download your spending as CSV (optionally for a date range), your income as CSV, or a <strong>full backup</strong> of everything as JSON. Your data is always yours to take.</p>

      <H2>Import a bank CSV</H2>
      <Steps>
        <li>Pick the format (BCA, Mandiri, generic debit/credit or generic date/description/amount) and upload the file. Nothing is saved yet: you get a preview first.</li>
        <li>Check the column mapping, date order and decimal separator.</li>
        <li>Map categories from the file to yours, and change the category of any row or untick rows you do not want.</li>
        <li>Press <strong>Import</strong>.</li>
      </Steps>
      <ul>
        <li>Only money going <em>out</em> is imported as spending; income/credit rows are shown as skipped.</li>
        <li>Importing the same file twice never creates duplicates.</li>
      </ul>
      <Callout type="tip" title="Made a mistake?">
        <p>Press <strong>Undo this import</strong> right after, or use <strong>Undo</strong> next to it under <strong>Recent imports</strong>. It removes exactly the transactions from that import.</p>
      </Callout>

      <H2>TV and car display links</H2>
      <p>The same page creates the links for TV and car displays. See <a href="/docs/tv-links">TV &amp; car links</a>.</p>
    </>
  )
}
