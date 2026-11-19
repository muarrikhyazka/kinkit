import { Roles } from '../components.jsx'
import Shot from '../Shot.jsx'

export const slug = 'goals'
export const title = 'Savings goals'
export const section = 'money'
export const summary = 'Targets with progress rings and the monthly amount you need.'

export function Content() {
  return (
    <>
      <Roles who={['admin', 'parent', 'viewer']} label="Who can see this page:" />
      <p>Under <strong>Savings › Goals</strong> create savings goals with a target amount and an optional target date. Each goal shows a progress ring and, if it has a date, how much to save per month to get there.</p>
      <Shot name="goals" alt="Savings goals with progress." />
      <ul>
        <li>Use <strong>Add</strong> on a goal to record a contribution (a negative amount withdraws).</li>
        <li>You can link a category (for example a Savings category: spending logged there since the goal was created counts) and assets (their current value counts).</li>
        <li>Your top goals also appear on the <a href="/docs/assets">Assets</a> page and as a widget on your boards.</li>
      </ul>
    </>
  )
}
