import { H2, Callout } from '../components.jsx'

export const slug = 'badges'
export const title = 'Badges'
export const section = 'points-badges'
export const summary = 'All 20 badges, what each one asks for, and their bronze, silver and gold tiers.'

const GROUPS = [
  ['Money', [
    ['First Log', 'bronze', 'Log your first spending.'],
    ['Financial Geek', 'silver', 'Log 100 expenses.'],
    ['Number Cruncher', 'gold', 'Log 500 expenses.'],
    ['First Budget Set', 'bronze', 'The household sets a monthly budget.'],
    ['Under Budget', 'silver', 'Close a month with Needs + Wants spending at or below budget.'],
    ['Saver', 'silver', 'Close a month with Savings at or above the savings budget.'],
    ['Goal Setter', 'bronze', 'The household creates a savings goal.'],
    ['Goal Crusher', 'gold', 'A savings goal reaches its target.'],
    ['Asset Tracker', 'bronze', 'Add an asset to your net worth.'],
    ['Income Insider', 'bronze', 'Record income.'],
  ]],
  ['Habits', [
    ['Warming Up', 'bronze', 'Log spending 3 days in a row.'],
    ['Streak 7', 'silver', 'Log spending 7 days in a row.'],
    ['Streak 30', 'gold', 'Log spending 30 days in a row.'],
    ['Never Forget', 'silver', 'Mark 10 reminders done.'],
    ['Planner', 'bronze', 'Add an activity to the family calendar.'],
  ]],
  ['Home', [
    ['Helping hand', 'bronze', 'Finish 10 chores.'],
    ['Chore champion', 'gold', 'Finish 50 chores.'],
    ['Menu Maker', 'bronze', 'Request a dish for the family menu.'],
    ["Chef's Choice", 'silver', 'A dish you requested makes the menu.'],
  ]],
  ['Levels', [
    ['Money Master', 'silver', 'Reach level 5 (500 points).'],
    ['Finance Wizard', 'gold', 'Reach level 10 (2,250 points).'],
  ]],
]
const TIER = { bronze: 'Bronze', silver: 'Silver', gold: 'Gold' }

export function Content() {
  return (
    <>
      <p>Badges reward good habits. There are <strong>20 badges</strong> in four categories: <strong>Money</strong>, <strong>Habits</strong>, <strong>Home</strong> and <strong>Levels</strong>. Each badge has a medal tier (bronze, silver or gold) that shows how big the achievement is. They appear on the <a href="/docs/family">Leaderboard</a> page under achievements.</p>
      <div className="dx-tiers" style={{ marginBottom: 16 }}>
        <span className="dx-tier bronze">Bronze</span>
        <span className="dx-tier silver">Silver</span>
        <span className="dx-tier gold">Gold</span>
      </div>
      {GROUPS.map(([name, list]) => (
        <section key={name}>
          <H2>{name}</H2>
          <div className="dx-badge-grid">
            {list.map(([n, tier, text]) => (
              <div className="dx-badge-card" key={n}>
                <h4>{n}</h4>
                <p>{text}</p>
                <div className="dx-tiers"><span className={`dx-tier ${tier}`}>{TIER[tier]}</span></div>
              </div>
            ))}
          </div>
        </section>
      ))}
      <Callout type="note">
        <p>Several badges depend on what the whole household does (for example setting a budget or creating a goal), so everyone shares them.</p>
      </Callout>
    </>
  )
}
