import { H2, Callout, Table } from '../components.jsx'

export const slug = 'points'
export const title = 'Points and streaks'
export const section = 'points-badges'
export const summary = 'How points are earned, streaks, weekly family challenges and the leaderboard.'

export function Content() {
  return (
    <>
      <p>Everyone in the household earns <strong>points</strong> for the habits that keep a family running. Points add up to a <a href="/docs/levels">level</a>, unlock <a href="/docs/badges">badges</a> and fill the leaderboard on the Family page.</p>

      <H2>How to earn points</H2>
      <Table head={['Action', 'Points']} rows={[
        ['Log spending (once per day)', '10, plus a streak bonus of 5 per day in your streak (up to 10 days, so up to 60 in a day)'],
        ['Finish a chore', 'The points set on the chore, once approved when approval is on'],
        ['Weekly family challenge completed', 'A bonus added to the household total once per week'],
      ]} />
      <Callout type="note">
        <p>Taking a chore back with <strong>Undo</strong> or <strong>Send back</strong> also takes its points back.</p>
      </Callout>

      <H2>Streaks</H2>
      <p>Log at least one spending a day and your streak grows. Miss a day and it starts again. Streaks unlock the <strong>Warming Up</strong> (3 days), <strong>Streak 7</strong> and <strong>Streak 30</strong> badges. Days follow UTC.</p>

      <H2>Weekly family challenges</H2>
      <p>Each week has challenges for the household, such as logging every day, a no-spend weekend, keeping Wants under a week&apos;s share of the budget, or finishing all your chores. Completing one adds bonus points to the household total, once per week. Weeks run Monday to Sunday (UTC).</p>

      <H2>Leaderboard</H2>
      <p>The leaderboard on the <a href="/docs/family">Family</a> page shows the points everyone earned this month.</p>
    </>
  )
}
