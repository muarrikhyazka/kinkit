import { Link } from 'react-router-dom'
import { H2, H3, Callout, Table } from '../components.jsx'

export const slug = 'navigation'
export const title = 'Navigation and shortcuts'
export const section = 'getting-started'
export const summary = 'The sidebar, the phone tab bar, undo and the period picker.'

export function Content() {
  return (
    <>
      <H2>The sidebar</H2>
      <p>On a computer or tablet the sidebar lists every page. The <strong>Add</strong> button at the top records spending from anywhere, and the family name shows under the Kinkit name.</p>
      <ul>
        <li><strong>Fold it to icons.</strong> The <strong>«</strong> button at the top collapses the sidebar to a slim icon rail; the same button expands it again.</li>
        <li><strong>Hide it completely.</strong> A second button hides the sidebar fully for the widest working area, and shows it again. The shortcut is <kbd>Ctrl</kbd>+<kbd>B</kbd> (<kbd>Cmd</kbd>+<kbd>B</kbd> on a Mac).</li>
        <li><strong>Foldable groups.</strong> The <strong>Money</strong> and <strong>Family</strong> headings fold their group of pages so you only see what you use.</li>
        <li>Everything above is remembered on this device.</li>
      </ul>
      <Callout type="note">
        <p>The <strong>Family</strong> group has five pages: <strong>Family</strong>, <strong>Chores</strong>, <strong>Lists</strong>, <strong>Meals</strong> and <strong>Widgets</strong>. They used to be tabs under a single <em>Home</em> page; old links such as <code>/home/chores</code> still open the right page.</p>
      </Callout>

      <H2>On your phone</H2>
      <p>On small screens the sidebar becomes a bottom tab bar: <strong>Today</strong>, <strong>Reminders</strong>, <strong>Chores</strong>, <strong>Transactions</strong> and <strong>More</strong>. <strong>More</strong> opens a sheet with Calendar, Dashboard, Budget, Savings, Family, Lists, Meals, Widgets, Settings, Help, the theme picker and logout. The round <strong>+</strong> button adds spending (or income, for the family admin and parents) from anywhere.</p>

      <H2>Keyboard shortcuts</H2>
      <Table head={['Shortcut', 'Where', 'What it does']} rows={[
        [<><kbd>Ctrl</kbd>/<kbd>Cmd</kbd>+<kbd>B</kbd></>, 'App', 'Hide or show the sidebar'],
        [<kbd>Enter</kbd>, 'Add spending', 'Next field; saves from the last field'],
        [<><kbd>Ctrl</kbd>/<kbd>Cmd</kbd>+<kbd>Enter</kbd></>, 'Add spending', 'Add another row'],
        [<kbd>Esc</kbd>, 'Dialogs, full-screen widgets', 'Close the dialog or leave full screen'],
        [<><kbd>/</kbd> or <kbd>Ctrl</kbd>+<kbd>K</kbd></>, 'These docs', 'Focus the search box'],
      ]} />

      <H2>Period and filters</H2>
      <p>Pages that depend on a period show a stepper in the page header: <strong>‹ Nov 2026 ›</strong>. Use the arrows to go back or forward a month (or a year on Budget), or click the label to jump to any month or year. Dates are in UTC.</p>
      <p><strong>Hide large</strong> leaves out big one-off transactions so they do not distort your charts: choose &ldquo;Only up to 10 juta&rdquo; or &ldquo;Only up to 100 juta&rdquo; to include only transactions up to that amount.</p>

      <H2>Deleting and undo</H2>
      <p>Deleting a transaction, asset, setup item or integration does not ask for confirmation. Instead a notification appears at the bottom with an <strong>Undo</strong> button, and the delete only happens after about 5 seconds, so you have time to change your mind.</p>

      <H2>Old links keep working</H2>
      <p>Links such as <code>/spending</code>, <code>/setup</code> or <code>/guide</code> still work and open the new place. The old in-app Help page now lives here at <Link to="/docs">/docs</Link>, and the <strong>?</strong> button in the app opens it.</p>

      <H3>Kids and viewers</H3>
      <p>Kids do not see Budget, Savings, Income, Recurring or most of Settings; everything else is there for them (Display is under Settings). Viewers see everything except Settings (apart from Display). See <Link to="/docs/roles">Roles &amp; permissions</Link>.</p>
    </>
  )
}
