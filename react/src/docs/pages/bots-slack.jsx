import { H2, Steps, Roles, Callout, Table, Code } from '../components.jsx'

export const slug = 'bots-slack'
export const title = 'Slack'
export const section = 'chat-bots'
export const summary = 'Connect a Slack app using Socket Mode: two tokens, a few scopes, one channel.'

export function Content() {
  return (
    <>
      <Roles who={['admin', 'parent']} label="Who can connect:" />
      <p>The Slack bot uses <strong>Socket Mode</strong>, so Slack connects out to Kinkit and no public URL is needed. You need two tokens from your Slack app.</p>

      <H2>What you need</H2>
      <Table head={['Item', 'Looks like', 'Where to get it']} rows={[
        ['Bot User OAuth token', <code key="a">xoxb-…</code>, 'OAuth & Permissions, after installing the app to your workspace'],
        ['App-level token', <code key="b">xapp-…</code>, 'Basic Information › App-Level Tokens, with the connections:write scope'],
        ['Channel ID', <code key="c">C0123456789</code>, 'Channel details › bottom of the About tab'],
      ]} />

      <H2>Set up the Slack app</H2>
      <Steps>
        <li>Create a Slack app at api.slack.com/apps and turn on <strong>Socket Mode</strong>.</li>
        <li>Create an <strong>App-Level token</strong> with the <code>connections:write</code> scope. This is the <code>xapp-…</code> token.</li>
        <li>Under <strong>OAuth &amp; Permissions</strong> add these <strong>bot token scopes</strong>:
          <Code title="Bot scopes">{`chat:write
channels:history
groups:history
im:history
users:read`}</Code>
        </li>
        <li>Under <strong>Event Subscriptions</strong> subscribe to <code>message.channels</code>, <code>message.groups</code> and <code>message.im</code>.</li>
        <li><strong>Install</strong> the app to your workspace and copy the <strong>Bot User OAuth token</strong> (<code>xoxb-…</code>).</li>
        <li>In Slack, <strong>invite the bot</strong> to your family channel (<code>/invite @YourBot</code>) and copy the <strong>Channel ID</strong>.</li>
        <li>In Kinkit open <strong>Settings › Bots › Slack</strong>, paste the <code>xoxb-…</code> token, the <code>xapp-…</code> token and the Channel ID, then press <strong>Save</strong> and keep <strong>Enabled</strong> ticked.</li>
        <li><a href="/docs/bots-overview#link-your-chat-account">Link your account</a> with <code>!link CODE</code>.</li>
      </Steps>

      <H2>Using it</H2>
      <ul>
        <li>Commands are plain messages with a <code>!</code> prefix: <code>!today</code>, <code>!remind …</code>, <code>!buy milk</code>. Slack swallows <code>/…</code> typed in the message box as slash commands, so use <code>!</code>.</li>
        <li>Anything else in the household channel that looks like <code>50k lunch</code> is logged as spending.</li>
        <li>Reminders and questions arrive with real buttons (<strong>Done</strong>, <strong>Snooze</strong>, categories).</li>
        <li>Outside the household channel (and in direct messages) the bot only answers <code>!help</code>, <code>!link</code> and <code>!about</code>.</li>
      </ul>
      <Callout type="warning" title="Invite the bot">
        <p>If the bot never answers, check that it is a member of the channel and that all three <code>*:history</code> scopes and the <code>message.*</code> event subscriptions are in place. Reinstall the app after changing scopes.</p>
      </Callout>
    </>
  )
}
