import { H2, Roles, Table, Callout } from '../components.jsx'

export const slug = 'roles'
export const title = 'Roles and permissions'
export const section = 'getting-started'
export const summary = 'Family admin, Parent, Kid and Viewer: who can do what.'

export function Content() {
  return (
    <>
      <p>A Kinkit account is a <strong>household</strong>. Whoever signed up is its <strong>Family Admin</strong>. On the <strong>Family</strong> page the family admin or a parent adds members with their email and a temporary password (they can change it later). Each member logs in with their own email.</p>

      <H2>The four roles</H2>
      <ul>
        <li><span className="dx-role admin">Family admin</span> everything, including setting the <strong>family name</strong> (Settings › Categories &amp; accounts), which shows under the Kinkit name in the sidebar, in the browser tab title and on the TV view. The family admin cannot be removed.</li>
        <li><span className="dx-role parent">Parent</span> full access to the household&apos;s money and can manage members (except the family admin), chores, lists, meals and widget boards.</li>
        <li><span className="dx-role kid">Kid</span> can see the household and log spending, and edit or delete only the entries they logged. A monthly <strong>allowance</strong> shows how much of it they have spent this month.</li>
        <li><span className="dx-role viewer">Viewer</span> read-only.</li>
      </ul>

      <H2>At a glance</H2>
      <Table head={['Feature', 'Family admin', 'Parent', 'Kid', 'Viewer']} rows={[
        ['Log spending', 'Yes', 'Yes', 'Own entries', 'No'],
        ['Income, Recurring, Budget, Savings', 'Yes', 'Yes', 'Hidden', 'View'],
        ['Add and edit reminders', 'Yes', 'Yes', 'No', 'No'],
        ['Mark a reminder done', 'Yes', 'Yes', 'Own / family ones', 'No'],
        ['Calendar activities', 'Create, edit', 'Create, edit', 'See own + family', 'View'],
        ['Chores: add / approve', 'Yes', 'Yes', 'Do them', 'View'],
        ['Shopping lists: add items', 'Yes', 'Yes', 'Yes', 'View'],
        ['Finish shopping, manage lists', 'Yes', 'Yes', 'No', 'No'],
        ['Meal requests: send', 'Yes', 'Yes', 'Yes', 'Yes'],
        ['Meal requests: approve / decline', 'Yes', 'Yes', 'No', 'No'],
        ['Widget boards: edit layout', 'Yes', 'Yes', 'No', 'No'],
        ['Settings (bots, data, categories)', 'Yes', 'Yes', 'Display only', 'Display only'],
      ]} />
      <Callout type="note">
        <p>The app hides menus you cannot use, and the server enforces the same rules, so a hidden button is never the only protection.</p>
      </Callout>
    </>
  )
}
