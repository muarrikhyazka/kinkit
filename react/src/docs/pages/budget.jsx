import { H2, Callout, Table, Roles } from '../components.jsx'
import Shot from '../Shot.jsx'

export const slug = 'budget'
export const title = 'Budget and alerts'
export const section = 'money'
export const summary = 'Plan income and expenses, see actual vs plan, get chat alerts.'

export function Content() {
  return (
    <>
      <Roles who={['admin', 'parent', 'viewer']} label="Who can see this page:" />
      <p>Set your planned income per source and planned expense per category for the selected year. Values save automatically when you leave a cell: a spinner shows while saving, a check mark when saved, and a red marker if it failed (hover it to see why; just edit and leave the cell again to retry).</p>
      <Shot name="budget" alt="Budget: planned income and expenses per category." />

      <H2>Budget vs actual</H2>
      <p>Every cell also shows what actually happened under the input: <em>Spent</em> for categories and <em>Got</em> for income sources. Expense cells are tinted:</p>
      <Table head={['Tint', 'Meaning']} rows={[
        ['Green', 'Under 80% of the plan'],
        ['Amber', '80% to 100% of the plan'],
        ['Red', 'Over budget, or you spent in a category with no budget'],
      ]} />
      <p>The Dashboard <strong>Month</strong> tab starts with a &ldquo;Spent X of Y budgeted this month&rdquo; bar and lists the categories that need attention.</p>

      <H2>Budget alerts</H2>
      <p>With a chat bot connected under <strong>Settings › Bots</strong> (bot token and chat id filled in), Kinkit sends a chat message when a category reaches 80% of its monthly budget and again when it goes over 100%, once each per category per month. Checks run every few minutes for the current month (UTC).</p>
      <p>Turn alerts on or off in the <strong>Budget alerts</strong> card at the bottom of the Budget page.</p>
      <Callout type="note">
        <p>Categories are grouped under Needs / Wants / Savings. That grouping drives the budget breakdown and the &ldquo;closed a month under budget&rdquo; badge. See <a href="/docs/settings-categories">Categories &amp; accounts</a>.</p>
      </Callout>
    </>
  )
}
