import { Callout, Roles } from '../components.jsx'

export const slug = 'recurring'
export const title = 'Recurring spending'
export const section = 'money'
export const summary = 'Rent, subscriptions and loans added automatically on schedule.'

export function Content() {
  return (
    <>
      <Roles who={['admin', 'parent']} label="Who can manage:" />
      <p>Add rent, subscriptions, loan payments and other repeating spending once on the <strong>Recurring</strong> page (Transactions › Recurring): monthly (on a day of the month), weekly (on a weekday) or yearly.</p>
      <p>Kinkit adds each occurrence to Spending automatically (source &ldquo;recurring&rdquo;), including any past dates since the start date. Day 29 to 31 falls on the last day in shorter months.</p>
      <ul>
        <li>The list shows the <strong>next date</strong> each rule will run.</li>
        <li><strong>Pause</strong> stops new transactions; <strong>Resume</strong> continues from today without backfilling the paused period.</li>
        <li>Deleting a rule (with Undo) keeps the transactions it already created.</li>
      </ul>
      <Callout type="tip">
        <p>Recurring entries count toward your budget just like ones you type in.</p>
      </Callout>
    </>
  )
}
