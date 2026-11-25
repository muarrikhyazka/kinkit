import { H2, Callout } from '../components.jsx'
import Shot from '../Shot.jsx'

export const slug = 'family'
export const title = 'Leaderboard'
export const section = 'family-home'
export const summary = 'Points this month, your level, badges and weekly challenges.'

export function Content() {
  return (
    <>
      <p>Open <strong>Family › Leaderboard</strong> in the sidebar. It shows your level, the points everyone earned this month, and the household&apos;s badges and weekly challenges.</p>
      <Shot name="family" alt="The Leaderboard: your level, points this month and badges." />

      <H2>Your level and stats</H2>
      <p>The level card shows your level and how far you are to the next one. Next to it are your points, your streak and the household total. See <a href="/docs/levels">Levels</a>.</p>

      <H2>Points this month</H2>
      <p>Everyone in the household is ranked by the points they earned this month. See <a href="/docs/points">Points and streaks</a> for what earns points.</p>

      <H2>Achievements</H2>
      <p>Badges and the weekly family challenges show under achievements. See <a href="/docs/badges">Badges</a>.</p>

      <Callout type="note">
        <p>Adding members, allowances and passwords moved to Settings. See <a href="/docs/settings-members">Members, allowances and passwords</a>.</p>
      </Callout>
    </>
  )
}
