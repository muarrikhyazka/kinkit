import { Link } from 'react-router-dom'
import { H2, Callout, CardGrid, Roles, ALL_ROLES } from '../components.jsx'
import Shot from '../Shot.jsx'
import { DKey, DWallet, DHome, DScreen, DChat, DSteps } from '../DocsIcons.jsx'

export const slug = 'introduction'
export const title = 'Welcome to Kinkit'
export const section = 'getting-started'
export const summary = 'What Kinkit is, who it is for and where to start.'

export function Content() {
  return (
    <>
      <div className="dx-hero">
        <h2>Money and home, for the whole family.</h2>
        <p>Kinkit keeps budgets, spending, savings, chores, shopping lists, reminders, the family calendar and the weekly menu in one place, for parents and kids alike.</p>
        <Link to="/docs/login">Create your household</Link>
      </div>

      <H2>What you can do</H2>
      <CardGrid items={[
        { to: 'login', title: 'Getting started', text: 'Create your household, log in and find your way around.', Icon: DKey },
        { to: 'add-spending', title: 'Money', text: 'Add spending, set budgets, track income, goals and assets.', Icon: DWallet },
        { to: 'today', title: 'Family & home', text: 'Today, calendar, reminders, chores, lists and the meal plan.', Icon: DHome },
        { to: 'widgets', title: 'Screens & devices', text: 'Widget boards for a phone, TV, car or monitor.', Icon: DScreen },
        { to: 'bots-overview', title: 'Chat bots', text: 'Log spending and get reminders in Telegram, Discord, Slack or Matrix.', Icon: DChat },
        { to: 'points', title: 'Points, levels & badges', text: 'Small rewards that make good habits stick.', Icon: DSteps },
      ]} />

      <H2>Finding your way</H2>
      <ul>
        <li><strong>Today</strong> (the start page): everything that needs attention today: reminders, activities, chores, shopping lists, meal requests, budget and what is coming up.</li>
        <li><Link to="/docs/calendar"><strong>Calendar</strong></Link>: family activities in month, week, day and agenda views, with your reminders on top.</li>
        <li><Link to="/docs/reminders"><strong>Reminders</strong></Link>: routines, bills and birthdays, as a list or on a calendar.</li>
        <li><strong>Money</strong> group: <strong>Dashboard</strong>, <strong>Transactions</strong> (Spending, Income, Recurring), <strong>Budget</strong> and <strong>Savings</strong> (Goals, Assets).</li>
        <li><strong>Family</strong> group: <strong>Family</strong>, <strong>Chores</strong>, <strong>Lists</strong>, <strong>Meals</strong> and <strong>Widgets</strong>, each its own page.</li>
        <li><strong>Settings</strong> (bottom of the sidebar): <strong>Categories &amp; accounts</strong>, <strong>Bots</strong>, <strong>Chat keywords</strong>, <strong>Data &amp; devices</strong> and <strong>Display</strong>.</li>
      </ul>
      <Shot name="today" alt="The Today page: reminders, activities, chores, shopping lists, budget and meal requests." />

      <H2>Who sees what</H2>
      <p>Every member of the household has a role. The role decides what they can change. See <Link to="/docs/roles">Roles &amp; permissions</Link> for the full list.</p>
      <Roles who={ALL_ROLES} label="Roles used in these docs:" />
      <Callout type="tip" title="Role badges">
        <p>Pages that only some people can use start with a row of role badges, like the one above, so you can tell at a glance whether a feature is yours.</p>
      </Callout>
    </>
  )
}
