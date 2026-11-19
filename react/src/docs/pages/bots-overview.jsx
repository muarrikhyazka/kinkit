import { H2, Callout, Roles, Steps, Table } from '../components.jsx'

export const slug = 'bots-overview'
export const title = 'Chat bots overview'
export const section = 'chat-bots'
export const summary = 'Connect Telegram, Discord, Slack or Matrix, and link your own chat account.'

export function Content() {
  return (
    <>
      <p>Chat bots bring Kinkit into the chat your family already uses. They log spending from plain messages, deliver reminders with buttons, post the weekly menu and the morning digest, and send budget alerts.</p>

      <H2>Supported platforms</H2>
      <Table head={['Platform', 'What you need', 'Buttons']} rows={[
        [<a href="/docs/bots-telegram">Telegram</a>, 'Bot token from @BotFather and a chat ID', 'Tap buttons'],
        [<a href="/docs/bots-discord">Discord</a>, 'Bot token from the Developer Portal and a channel ID', 'Tap buttons'],
        [<a href="/docs/bots-slack">Slack</a>, 'Bot token (xoxb-) + app-level token (xapp-) and a channel ID', 'Tap buttons'],
        [<a href="/docs/bots-matrix">Matrix</a>, 'Homeserver URL, a bot user\'s access token and a room ID', 'Numbered choices'],
      ]} />

      <H2>Connect a bot</H2>
      <Roles who={['admin', 'parent']} />
      <Steps>
        <li>Open <strong>Settings › Bots</strong>.</li>
        <li>Pick the platform and paste the credentials shown on its page here.</li>
        <li>Enter the <strong>chat / channel ID</strong> of your household chat. It is optional, but when set the bot only answers in that chat.</li>
        <li>Press <strong>Save</strong> and keep <strong>Enabled</strong> ticked.</li>
      </Steps>
      <p>Each platform you connect gets its own secret token (the <code>X-Bot-Token</code> header) tied only to your account, so one shared bot process can serve every Kinkit household at once.</p>

      <H2 id="link-your-chat-account">Link your chat account</H2>
      <p>Link your own chat account once, on any platform, so reminders mention you and what you do in chat counts as you (for example chore points).</p>
      <Steps>
        <li>In Kinkit open <strong>Reminders › Link my chat account</strong> to get a code.</li>
        <li>Send <code>/link CODE</code> to the household bot. On Slack and Matrix use <code>!link CODE</code> (the <code>!</code> prefix works on every platform).</li>
      </Steps>

      <H2>Group chats</H2>
      <p>In a group chat the bot only answers in the household chat set under Settings › Bots. To log plain messages (not just commands) in a Telegram group, turn the bot&apos;s privacy mode off in BotFather (<code>/setprivacy</code> › Disable) or make it a group admin.</p>
      <Callout type="note">
        <p>Next: <a href="/docs/bot-commands">Chat commands and logging spending</a>.</p>
      </Callout>
    </>
  )
}
