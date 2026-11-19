import { H2, Callout, Code, Table } from '../components.jsx'

export const slug = 'levels'
export const title = 'Levels'
export const section = 'points-badges'
export const summary = 'Level 1 to 50: how points turn into levels and what each level is called.'

const TITLES = ['Getting Started', 'Tracker', 'Budget Rookie', 'Budget Buddy', 'Money Master', 'Savings Scout', 'Wealth Builder', 'Cash Commander', 'Finance Pro', 'Finance Wizard']
const start = n => 25 * n * (n - 1)
const fmt = n => n.toLocaleString('en-US')

export function Content() {
  const rows = TITLES.map((t, i) => [i + 1, t, fmt(start(i + 1))])
  return (
    <>
      <p>Your points decide your <strong>level</strong>, from level 1 up to level 50. Each level has a title that shows next to your name and in the points badge in the sidebar.</p>

      <H2>The formula</H2>
      <p>Level <em>n</em> starts at <code>25 × n × (n − 1)</code> points, so each level takes a little more than the one before.</p>
      <Code title="Examples">{`level 2  =  50 points
level 3  = 150 points
level 4  = 300 points
level 5  = 500 points   (Money Master)
level 10 = 2,250 points (Finance Wizard)`}</Code>

      <H2>Level titles</H2>
      <Table head={['Level', 'Title', 'Starts at (points)']} rows={rows} />
      <p>After level 10 the title keeps growing with you:</p>
      <Table head={['Levels', 'Title']} rows={[
        ['10 to 14', 'Finance Wizard'],
        ['15 to 19', 'Money Sage'],
        ['20 to 29', 'Wealth Guru'],
        ['30 and up (to 50)', 'Legend'],
      ]} />
      <Callout type="tip">
        <p>Reaching level 5 and level 10 also unlock the <strong>Money Master</strong> and <strong>Finance Wizard</strong> <a href="/docs/badges">badges</a>.</p>
      </Callout>
    </>
  )
}
