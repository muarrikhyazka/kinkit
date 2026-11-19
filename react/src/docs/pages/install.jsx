import { H2, Callout, Steps } from '../components.jsx'

export const slug = 'install'
export const title = 'Install and use offline'
export const section = 'getting-started'
export const summary = 'Add Kinkit to your home screen or desktop and keep working without a connection.'

export function Content() {
  return (
    <>
      <H2>Install as an app</H2>
      <p>Kinkit can be installed like a normal app, with its own icon and a full-screen window:</p>
      <ul>
        <li><strong>Android (Chrome):</strong> menu ⋮ → <strong>Install app</strong> / <strong>Add to Home screen</strong>.</li>
        <li><strong>iPhone / iPad (Safari):</strong> Share button → <strong>Add to Home Screen</strong>.</li>
        <li><strong>Desktop (Chrome / Edge):</strong> the install icon at the right of the address bar.</li>
      </ul>
      <Callout type="note">
        <p>Besides the installable web app there is a native phone app and a view-only TV app (see <a href="/docs/tv-links">TV &amp; car links</a>). An iPhone app is coming soon; until then, use the web app in Safari and add it to your Home Screen.</p>
      </Callout>

      <H2>Working offline</H2>
      <p>The installed app opens even without a connection and shows the data from your last visit. Spending you add offline is kept on the device (it survives closing the app) and syncs automatically once you are back online.</p>
      <ul>
        <li>The <strong>connection dot</strong> shows whether the server can be reached and how many entries are queued.</li>
        <li>If the server rejects a queued entry, it is listed at the top of the page so nothing disappears silently.</li>
        <li>Receipts cannot be attached while offline; add them later from the Spending list.</li>
      </ul>
    </>
  )
}
