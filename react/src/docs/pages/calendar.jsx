import { H2, H3, Callout, Roles, Table } from '../components.jsx'
import Shot from '../Shot.jsx'

export const slug = 'calendar'
export const title = 'Calendar and activities'
export const section = 'family-home'
export const summary = 'A shared family calendar with repeats, attendees and reminder layers.'

export function Content() {
  return (
    <>
      <p>The family calendar works like Google Calendar. Switch between <strong>Month</strong>, <strong>Week</strong>, <strong>Day</strong> and <strong>Agenda</strong>, step with the arrows and jump back with <strong>Today</strong>.</p>
      <Shot name="calendar-month" alt="Month view with activities (solid) and reminders (dotted)." />
      <Shot name="calendar-week" alt="Week view with an hour grid and a red line for the current time." />

      <H2>Layers</H2>
      <p>The calendar draws two layers on top of each other. Untick <strong>Reminders</strong> or <strong>Activities</strong> to focus on one. Activities are solid; reminders are dotted.</p>

      <H2>Add an activity</H2>
      <Roles who={['admin', 'parent']} />
      <p>Choose <strong>New activity</strong>, or <strong>Add on this day</strong> under the month grid. Set:</p>
      <ul>
        <li>the <strong>title</strong>;</li>
        <li><strong>all-day</strong>, or a start and end time;</li>
        <li>the <strong>attendees</strong>: who it is for (nobody ticked means the whole family);</li>
        <li>where, notes and a color.</li>
      </ul>

      <H2>Repeat rules</H2>
      <p>An activity can repeat:</p>
      <Table head={['Repeat', 'Options']} rows={[
        ['Daily', 'Every N days'],
        ['Weekly', 'Every N weeks, on the weekdays you pick'],
        ['Monthly', 'Every N months'],
        ['Yearly', 'Every N years'],
      ]} />
      <p>Add an optional end date to stop the series. Click an activity and use <strong>Delete this one</strong> to skip a single day, or <strong>Delete series</strong> to remove every occurrence.</p>

      <H2>Who sees what</H2>
      <ul>
        <li>Kids only see activities that include them or the whole family.</li>
        <li>Viewers can look but not change anything.</li>
      </ul>

      <H2>On a phone</H2>
      <Callout type="note">
        <p>The phone app shows the <strong>Month</strong> and <strong>Agenda</strong> views. On the web app on a small screen the month shows dots; tap a day to see its list underneath.</p>
      </Callout>
      <H3>Reminders on the calendar</H3>
      <p>Your reminders appear as dotted entries on the same grid. You can also see them on their own month calendar: see <a href="/docs/reminders#list-and-calendar">Reminders</a>.</p>
    </>
  )
}
