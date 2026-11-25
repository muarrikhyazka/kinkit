import { H2, Callout, Table } from '../components.jsx'

export const slug = 'settings-display'
export const title = 'Display, themes and time zone'
export const section = 'settings-data'
export const summary = 'Light and dark mode, Playful and Senior themes, household time zone and morning digest.'

export function Content() {
  return (
    <>
      <p><strong>Settings › Display</strong> is available to every role. It holds the theme for this device, plus the household time zone and morning digest.</p>

      <H2>Themes</H2>
      <p>The sun/moon button at the bottom of the sidebar (and in the docs top bar) switches between <strong>light and dark</strong>: the same colors and fonts, with white swapped for black. Two more themes live under Settings › Display (or the <strong>More</strong> sheet on your phone). The choice is saved per device.</p>
      <Table head={['Theme', 'For']} rows={[
        ['Light', 'The default'],
        ['Dark', 'Low light'],
        ['Playful (kids)', 'A warm, friendly look for kids'],
        ['Senior (high-contrast)', 'High contrast and larger text'],
      ]} />

      <H2>Navigation</H2>
      <p>The <strong>Navigation</strong> card chooses how you move between pages on this device: <strong>Sidebar</strong> lists every page, <strong>Game (town map)</strong> lets you walk Kiki around a small town where each building opens a page. See <a href="/docs/navigation#game-mode">Game mode</a>.</p>

      <H2>Household time zone</H2>
      <p>Everything is stored in UTC. The household time zone decides when &ldquo;today&rdquo; starts and when &ldquo;every day at 08:00&rdquo; fires, for the whole family. The family admin and parents can change it; they are asked once to switch from UTC to the time zone of their device.</p>

      <H2>Morning digest</H2>
      <p>Tick <strong>Send a morning digest to the family chat</strong>, choose the time (07:00 by default) and the chat platform. Every morning the chat gets a short summary of the day.</p>
      <Callout type="note">
        <p>The digest needs a connected chat bot: see <a href="/docs/bots-overview">Chat bots overview</a>.</p>
      </Callout>
    </>
  )
}
