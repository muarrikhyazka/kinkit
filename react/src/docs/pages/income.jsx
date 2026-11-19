import { Roles } from '../components.jsx'

export const slug = 'income'
export const title = 'Income'
export const section = 'money'
export const summary = 'Record the money you actually received and compare it with your plan.'

export function Content() {
  return (
    <>
      <Roles who={['admin', 'parent']} label="Who can add:" />
      <p><a href="/docs/budget">Budget</a> holds the income you <em>plan</em>; the <strong>Income</strong> page (Transactions › Income) records money you actually <em>received</em>. Pick a date, a source and an amount.</p>
      <p>The page compares what came in against the plan for the month, and the Dashboard shows received vs planned income in the Month, Year and Across Years views.</p>
      <p>Income sources are managed under <a href="/docs/settings-categories">Settings › Categories &amp; accounts</a>. On a phone the round <strong>+</strong> button offers income as well as spending for the family admin and parents.</p>
    </>
  )
}
