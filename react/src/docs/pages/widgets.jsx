import { H2, H3, Callout, Roles, Steps, Table } from '../components.jsx'
import Shot from '../Shot.jsx'

export const slug = 'widgets'
export const title = 'Widget boards'
export const section = 'screens-devices'
export const summary = 'Build your own screens with drag and drop, colors and a background.'

export function Content() {
  return (
    <>
      <p><strong>Widgets</strong> (Family › Widgets) lets you build a screen with just what you want to see. Start from a template or a blank board, arrange the widgets, and show the board on a phone, a TV, a car screen or a monitor.</p>

      <H2>Templates</H2>
      <Table head={['Template', 'Starts with']} rows={[
        ['Phone', 'Clock, agenda, reminders, chores, meals, lists'],
        ['Living-room TV', 'Clock, agenda, reminders, chores, meals, budget, goals, family points'],
        ['Car screen', 'Clock, agenda, reminders, lists'],
        ['Desk monitor', 'Clock, agenda, reminders, chores, lists, budget, goals, meals, family points'],
      ]} />
      <p>Or pick a <strong>blank board</strong> and add only what you need.</p>

      <H2>Available widgets</H2>
      <Table head={['Widget', 'Shows']} rows={[
        ['Clock & date', 'A large clock with the date, in the family\'s time zone'],
        ['Calendar agenda', 'The next 7 days of family activities'],
        ['Reminders today', 'What is due today and what is overdue'],
        ['Chores today', 'Chores to do today and who has them'],
        ['Shopping lists', 'Open shopping lists and what is on them'],
        ['Today\'s meals', 'Dishes planned for today, including approved requests'],
        ['Meal requests', 'Dishes family members asked for, waiting for a decision'],
        ['Budget this month', 'Money left to spend and the busiest categories'],
        ['Savings goals', 'Progress on every goal'],
        ['Family points', 'The household\'s points, streak and badge'],
        ['Note', 'A short message you write yourself, such as the Wi-Fi password or a house rule'],
      ]} />

      <H2>Edit the layout</H2>
      <Roles who={['admin', 'parent']} />
      <Steps>
        <li>Open a board and press <strong>Edit layout</strong>.</li>
        <li><strong>Drag a widget onto another</strong> to move it, or use the arrows.</li>
        <li><strong>− / +</strong> change the width, <strong>▁ / █</strong> the height, <strong>✕</strong> removes the widget.</li>
        <li>Pick <strong>+ Add a widget</strong> to add more.</li>
        <li>Press <strong>Save layout</strong>.</li>
      </Steps>
      <Shot name="widgets-edit" alt="Editing a board: drag widgets, resize them, add more." />

      <H2>Colors and background</H2>
      <Roles who={['admin', 'parent']} />
      <p>While editing you can make a board your own:</p>
      <ul>
        <li><strong>Color each widget.</strong> Pick a colour swatch on a widget, or choose a <strong>custom</strong> colour. The text colour switches between light and dark automatically so it stays readable. Choose the default swatch to go back to the theme colour.</li>
        <li><strong>Board background.</strong> Set a background colour for the whole board.</li>
      </ul>
      <Callout type="tip">
        <p>Use colour to sort a TV screen at a glance: chores in one colour, money in another, the clock in a calm neutral.</p>
      </Callout>

      <H2>Show full screen</H2>
      <p><strong>Show full screen</strong> fills the screen for an always-on phone, tablet or monitor and refreshes every minute. Press <kbd>Esc</kbd> to leave.</p>

      <H2>Link a TV or car display to a board</H2>
      <p>A TV or car link can show a board without anyone logging in. Under <strong>Settings › Data &amp; devices</strong> create a TV / car link and choose the board it shows in the <strong>Shows</strong> list. See <a href="/docs/tv-links">TV &amp; car links</a>.</p>

      <H3>Who sees what</H3>
      <ul>
        <li>Everyone can view boards.</li>
        <li>Kids do not get money widgets (budget, goals).</li>
        <li>Only the family admin and parents edit layouts.</li>
      </ul>
    </>
  )
}
