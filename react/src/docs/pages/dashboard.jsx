import { H2 } from '../components.jsx'
import Shot from '../Shot.jsx'

export const slug = 'dashboard'
export const title = 'Dashboard'
export const section = 'money'
export const summary = 'Charts of your money across years, months and weeks.'

export function Content() {
  return (
    <>
      <p>An overview of your money across the <strong>Across Years</strong>, <strong>Year</strong>, <strong>Month</strong>, <strong>Week</strong> and <strong>Custom</strong> tabs. Each tab starts with the key numbers and one main chart.</p>
      <Shot name="dashboard" alt="The money dashboard." />

      <H2>More insights</H2>
      <p>Extra pattern charts live under <strong>More insights</strong> (Year and Across Years). It remembers whether you left it open.</p>

      <H2>Custom tab</H2>
      <p>In <strong>Custom</strong> you pick which charts to show and drag them (or use the arrow buttons) to reorder.</p>

      <H2>Budget bar on the Month tab</H2>
      <p>The <strong>Month</strong> tab starts with a &ldquo;Spent X of Y budgeted this month&rdquo; bar and lists the categories that need attention. See <a href="/docs/budget">Budget</a>.</p>
      <p>The Dashboard also shows received vs planned income in the Month, Year and Across Years views (see <a href="/docs/income">Income</a>).</p>
    </>
  )
}
