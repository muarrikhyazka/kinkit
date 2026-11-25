import { Faq, Callout } from '../components.jsx'

export const slug = 'faq'
export const title = 'FAQ and troubleshooting'
export const section = 'faq'
export const summary = 'Quick answers to common questions.'

export function Content() {
  return (
    <>
      <Faq q="I signed up but I do not see my family's data.">
        <p>Signing up creates a <em>new</em> household. To join an existing family, ask the family admin or a parent to add you under <strong>Settings › Members</strong> and log in with the email and temporary password they set. See <a href="/docs/login">Log in and sign up</a>.</p>
      </Faq>
      <Faq q="Where did Home, and the Help page, go?">
        <p>Chores, Lists and Meals are separate pages in the <strong>Family</strong> group now, next to Leaderboard and Widgets. Old links such as <code>/home/chores</code>, <code>/help</code> and <code>/guide</code> still work. The Help page is this documentation site.</p>
      </Faq>
      <Faq q="I deleted something by accident.">
        <p>A notification with an <strong>Undo</strong> button appears at the bottom for about 5 seconds. For a bank import use <strong>Undo this import</strong>. For chores, undo also takes back the points.</p>
      </Faq>
      <Faq q="The sidebar is gone.">
        <p>Press <kbd>Ctrl</kbd>/<kbd>Cmd</kbd>+<kbd>B</kbd> or use the show button to bring it back. See <a href="/docs/navigation">Navigation and shortcuts</a>.</p>
      </Faq>
      <Faq q="My chat bot does not answer.">
        <p>Check that <strong>Enabled</strong> is ticked under Settings › Bots, that the chat/channel ID matches the household chat, and that you have linked your account with <code>/link CODE</code> (<code>!link CODE</code> on Slack and Matrix). For Telegram groups, turn privacy mode off to log plain messages. On Slack make sure the bot was invited to the channel; on Matrix use an unencrypted room. See <a href="/docs/bots-overview">Chat bots overview</a>.</p>
      </Faq>
      <Faq q="Spending I added offline is missing.">
        <p>Offline entries are kept on the device and sync when the server is reachable again (watch the connection dot). If the server rejects one it is listed at the top of the page. See <a href="/docs/install">Install and use offline</a>.</p>
      </Faq>
      <Faq q="Why is my category tinted red on the Budget page?">
        <p>You spent more than planned, or spent in a category that has no budget. Green is under 80%, amber 80 to 100%. See <a href="/docs/budget">Budget and alerts</a>.</p>
      </Faq>
      <Faq q="Can kids see money pages?">
        <p>Kids see the household and can log spending, but Budget, Savings, Income and Recurring are hidden, and money widgets are not shown to them. See <a href="/docs/roles">Roles and permissions</a>.</p>
      </Faq>
      <Faq q="Is there an iPhone app?">
        <p>An iPhone app is coming soon. Until then use the web app in Safari and choose <strong>Add to Home Screen</strong>. See <a href="/docs/install">Install and use offline</a>.</p>
      </Faq>
      <Callout type="tip">
        <p>Can&apos;t find something? Press <kbd>/</kbd> to search these docs.</p>
      </Callout>
    </>
  )
}
