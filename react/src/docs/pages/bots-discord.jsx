import { Steps, Roles, Callout } from '../components.jsx'

export const slug = 'bots-discord'
export const title = 'Discord'
export const section = 'chat-bots'
export const summary = 'Connect a Discord bot and your family channel.'

export function Content() {
  return (
    <>
      <Roles who={['admin', 'parent']} label="Who can connect:" />
      <Steps>
        <li>In the Discord Developer Portal create an application and a bot, and copy its <strong>bot token</strong>.</li>
        <li>Invite the bot to your server and let it read and post in your family channel.</li>
        <li>In Kinkit open <strong>Settings › Bots › Discord</strong>, paste the token and enter the <strong>channel ID</strong> (turn on Developer Mode in Discord, then right-click the channel › Copy ID). Leave it blank to allow any channel.</li>
        <li>Press <strong>Save</strong>, keep <strong>Enabled</strong> ticked, then <a href="/docs/bots-overview#link-your-chat-account">link your account</a> with <code>/link CODE</code>.</li>
      </Steps>
      <Callout type="tip">
        <p>Reminders and category questions come with tap buttons, just like on Telegram.</p>
      </Callout>
    </>
  )
}
