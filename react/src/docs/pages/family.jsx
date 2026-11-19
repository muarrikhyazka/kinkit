import { H2, Roles, Callout } from '../components.jsx'
import Shot from '../Shot.jsx'

export const slug = 'family'
export const title = 'Family members and allowances'
export const section = 'family-home'
export const summary = 'Add members, set allowances and see the leaderboard.'

export function Content() {
  return (
    <>
      <p>The <strong>Family</strong> page shows the members, their roles, allowances, the points leaderboard and achievements.</p>
      <Shot name="family" alt="Family members, allowances and achievements." />

      <H2>Add and manage members</H2>
      <Roles who={['admin', 'parent']} />
      <p>Add a member with their email and a temporary password. They can change it later on the Family page. Only the family admin and parents change members, and nobody can remove the family admin. See <a href="/docs/roles">Roles &amp; permissions</a>.</p>

      <H2>Allowances</H2>
      <p>Each kid has a monthly <strong>allowance</strong>. The page shows how much of it they have spent this month. Money rewards for approved chores are added on top of the allowance.</p>

      <H2>Leaderboard and achievements</H2>
      <p>The leaderboard shows the points everyone earned this month, and the family&apos;s badges and weekly challenges show under achievements. See <a href="/docs/points">Points and streaks</a> and <a href="/docs/badges">Badges</a>.</p>
      <Callout type="note">
        <p>The family name is set under Settings › Categories &amp; accounts and shows in the sidebar, the browser tab title and on the TV view.</p>
      </Callout>
    </>
  )
}
