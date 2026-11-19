import { H2 } from '../components.jsx'
import Shot from '../Shot.jsx'

export const slug = 'today'
export const title = 'Today'
export const section = 'family-home'
export const summary = 'The start page: what needs attention today.'

export function Content() {
  return (
    <>
      <p>The start page. It greets you and collects what matters today: reminders due (and overdue), family activities, chores, shopping lists, meal requests, how much of this month&apos;s budget is used, and reminders coming up in the next days. Tick reminders and chores straight from here; the family admin and parents can approve chores kids have finished.</p>
      <Shot name="today" alt="The Today page: reminders, activities, chores, shopping lists, budget and meal requests." />

      <H2>Household time zone</H2>
      <p>Days follow the <strong>household time zone</strong> (Settings › Display). Data is stored in UTC; the time zone only decides when &ldquo;today&rdquo; starts and when &ldquo;every day at 08:00&rdquo; fires. The family admin and parents are asked once to switch from UTC to the time zone of their device.</p>

      <H2>Getting-started checklist</H2>
      <p>The family admin and parents also see a short <strong>Getting started</strong> checklist on Today: set up income sources and categories, set a budget, log your first expense and (optionally) connect a chat bot. You can dismiss it.</p>
    </>
  )
}
