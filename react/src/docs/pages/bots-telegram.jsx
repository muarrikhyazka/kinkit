import { H2, Steps, Roles, Callout, Code } from '../components.jsx'

export const slug = 'bots-telegram'
export const title = 'Telegram'
export const section = 'chat-bots'
export const summary = 'Connect a Telegram bot and your family group.'

export function Content() {
  return (
    <>
      <Roles who={['admin', 'parent']} label="Who can connect:" />
      <Steps>
        <li>In Telegram, talk to <strong>@BotFather</strong>, create a bot and copy its <strong>bot token</strong>.</li>
        <li>Add the bot to your family group (or chat with it directly).</li>
        <li>In Kinkit open <strong>Settings › Bots › Telegram</strong>, paste the token and enter the <strong>chat ID</strong>, for example <code>-1001234567890</code> for a group. Leave the chat ID blank to allow any chat.</li>
        <li>Press <strong>Save</strong>, keep <strong>Enabled</strong> ticked, then <a href="/docs/bots-overview#link-your-chat-account">link your account</a> with <code>/link CODE</code>.</li>
      </Steps>
      <H2>Read plain messages in a group</H2>
      <p>By default Telegram bots in groups only see commands. To log spending from plain messages, turn privacy mode off or make the bot a group admin:</p>
      <Code title="BotFather">{`/setprivacy
(choose your bot)
Disable`}</Code>
      <Callout type="tip">
        <p>Reminders and category questions come with tap buttons: <strong>Done</strong>, <strong>Snooze</strong>, category names and <strong>Not an expense</strong>.</p>
      </Callout>
    </>
  )
}
