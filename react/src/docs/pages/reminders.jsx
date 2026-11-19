import { H2, Callout, Roles } from '../components.jsx'
import Shot from '../Shot.jsx'

export const slug = 'reminders'
export const title = 'Reminders'
export const section = 'family-home'
export const summary = 'Routines, bills and birthdays as a list or a calendar, delivered to your chat.'

export function Content() {
  return (
    <>
      <H2>Create reminders</H2>
      <Roles who={['admin', 'parent']} label="Who can add:" />
      <ul>
        <li>Add reminders for bills, medicine, school, birthdays, chores or anything else from the <strong>Quick add</strong> templates or <strong>New reminder</strong>: once, daily, weekly (pick days), monthly (day 31 means the last day of shorter months) or yearly.</li>
        <li>Assign a reminder to one person or to the whole family. Optionally pick someone to tell if nobody marks it done within a number of minutes.</li>
        <li>Reminders arrive in the family&apos;s chat (Telegram, Discord, Slack or Matrix) with <strong>Done</strong> and <strong>Snooze</strong> buttons, and show on <a href="/docs/today">Today</a>. Everyone except viewers can mark their own (or whole-family) reminders done.</li>
      </ul>

      <H2 id="list-and-calendar">List and calendar</H2>
      <p><strong>Reminders</strong> has a <strong>List | Calendar</strong> switch.</p>
      <ul>
        <li><strong>List</strong> shows what is due in the next 7 days plus every reminder grouped by category.</li>
        <li><strong>Calendar</strong> shows the whole month: tap a day to see its reminders, tap one to mark it <strong>Done</strong> or snooze it.</li>
      </ul>
      <Shot name="reminders-calendar" alt="Reminders as a month calendar, with the selected day listed underneath." />
      <p>Reminders also appear as a layer on the <a href="/docs/calendar">family calendar</a>.</p>

      <H2>Morning digest</H2>
      <p>Every morning (07:00 by default, change it in Settings › Display) the chat gets a short summary of the day.</p>

      <H2>Add reminders from chat</H2>
      <p>Once you have <a href="/docs/bots-overview#link-your-chat-account">linked your chat account</a>, you can create reminders by typing, for example:</p>
      <ul>
        <li><code>/remind tomorrow 8am take medicine</code></li>
        <li><code>/remind every monday 7:00 trash</code></li>
        <li><code>/remind in 2 hours call grandma @Mom</code></li>
      </ul>
      <Callout type="tip">
        <p>Reminders you finish count toward the <strong>Never Forget</strong> badge (10 reminders done). See <a href="/docs/badges">Badges</a>.</p>
      </Callout>
    </>
  )
}
