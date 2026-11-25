import { H2, Roles, Callout } from '../components.jsx'
import Shot from '../Shot.jsx'

export const slug = 'settings-members'
export const title = 'Members, allowances and passwords'
export const section = 'settings-data'
export const summary = 'Add family members, set allowances and change your own password.'

export function Content() {
  return (
    <>
      <p>Under <strong>Settings › Members</strong> everyone in the household can see who is in it and what role each person has. Under <strong>Settings › Your account</strong> you change your own password. Both tabs are open to every role.</p>
      <Shot name="settings-members" alt="Settings › Members: everyone in the household with their role and allowance." />

      <H2>Add and manage members</H2>
      <Roles who={['admin', 'parent']} />
      <p>Choose <strong>Add member</strong> and enter their email, a name, a role and a temporary password of at least 8 characters. Share the email and the temporary password with them. They can change it themselves under <strong>Settings › Your account</strong>.</p>
      <ul>
        <li>Use the edit button on a row to change a name or role, or to reset a password.</li>
        <li>Use the remove button to take someone out of the household. A notification with <strong>Undo</strong> appears for about 5 seconds.</li>
        <li>Parents cannot edit the family admin, and nobody can remove the family admin.</li>
        <li>Kids, parents and viewers see the list without the edit and remove buttons.</li>
      </ul>
      <p>See <a href="/docs/roles">Roles &amp; permissions</a> for what each role can do.</p>

      <H2>Allowances</H2>
      <Roles who={['admin', 'parent']} />
      <p>When you add or edit a kid, you can set a monthly <strong>allowance</strong>. The Allowances card shows how much of it each kid has spent this month. Money rewards for approved chores are added on top of the allowance. See <a href="/docs/chores">Chores</a>.</p>

      <H2>Change your password</H2>
      <p>Open <strong>Settings › Your account</strong>, enter your current password and a new one of at least 8 characters, then press <strong>Change password</strong>. The phone app has the same under Settings, in the <strong>Your account</strong> section.</p>
      <Callout type="note">
        <p>The phone app shows the members as a read-only list. Adding, editing and removing members and setting allowances happen on the web.</p>
      </Callout>
    </>
  )
}
