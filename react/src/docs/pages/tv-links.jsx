import { H2, Callout, Roles, Steps } from '../components.jsx'
import Shot from '../Shot.jsx'

export const slug = 'tv-links'
export const title = 'TV and car links'
export const section = 'screens-devices'
export const summary = 'Show a board or this month\'s numbers on a TV, wall tablet or car head unit without logging in.'

export function Content() {
  return (
    <>
      <p>A TV / car display link opens in a TV browser, on a wall tablet or on a car head unit. It shows without logging in and refreshes every minute.</p>
      <Shot name="tv-board" alt="The same board on a TV link." />

      <H2>Create a link</H2>
      <Roles who={['admin', 'parent']} />
      <Steps>
        <li>Open <strong>Settings › Data &amp; devices</strong>.</li>
        <li>Create a TV / car link.</li>
        <li>Choose what it <strong>Shows</strong>: the month summary or one of your <a href="/docs/widgets">widget boards</a>.</li>
        <li>Open the link on the TV or car screen.</li>
      </Steps>
      <p>The Android TV, Samsung and LG apps and any TV browser show that board. The TV view is a big, auto-refreshing screen: the current month summary, or a widget board you built.</p>

      <Callout type="warning" title="Anyone with the link can see it">
        <p>Anyone with the link can see those numbers. <strong>Revoke</strong> a link to switch it off instantly.</p>
      </Callout>

      <H2>View only</H2>
      <p>TV links are read-only: the screen can show your household but never change anything, and it never asks for a login.</p>
    </>
  )
}
