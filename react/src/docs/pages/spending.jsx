import { H2, H3, Callout } from '../components.jsx'
import Shot from '../Shot.jsx'
import { IconEdit } from '../../icons.jsx'
import { IconPaperclip } from '../../Receipts.jsx'

export const slug = 'spending'
export const title = 'Spending and receipts'
export const section = 'money'
export const summary = 'The transaction list, editing, reconciling and receipt photos.'

export function Content() {
  return (
    <>
      <p>Under <strong>Transactions › Spending</strong> you see the full transaction list for the selected month. Search by description, filter by category, and click any column header to sort.</p>
      <Shot name="spending" alt="The spending list for a month." />

      <H2>Edit and reconcile</H2>
      <ul>
        <li>Click the <IconEdit width={13} height={13} role="img" aria-label="edit" /> edit icon on a row to change its date, description, category or amount. <kbd>Enter</kbd> saves, <kbd>Esc</kbd> cancels.</li>
        <li>Tick <strong>Checked</strong> once you have reconciled a transaction.</li>
        <li>Deleting shows an <strong>Undo</strong> for a few seconds instead of asking first.</li>
        <li>Kids can edit or delete only the entries they logged.</li>
      </ul>

      <H2>Receipts</H2>
      <p>Keep the photo of a receipt with its transaction. When adding spending, tap <strong>Attach receipt</strong> (or <strong>Camera</strong> on your phone) under a row.</p>
      <H3>Read amount from receipt</H3>
      <p><strong>Read amount from receipt</strong> reads the photo on your device and fills in the total it finds. It downloads a text-recognition helper the first time, so it needs a connection.</p>
      <Callout type="warning">
        <p>Always double-check an amount read from a receipt photo.</p>
      </Callout>
      <H3>In the list</H3>
      <p>In the Spending list, the <IconPaperclip width={13} height={13} role="img" aria-label="paperclip" /> paperclip on a row attaches a photo. Once a row has one, the paperclip is highlighted and opens the photo, where you can replace or remove it.</p>
      <p>Photos up to 5 MB (JPEG, PNG, WebP or HEIC) are stored privately: only your household can see them. Receipts cannot be attached while offline; add them later from the Spending list.</p>
    </>
  )
}
