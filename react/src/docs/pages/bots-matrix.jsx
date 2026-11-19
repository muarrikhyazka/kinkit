import { H2, Steps, Roles, Callout, Table } from '../components.jsx'

export const slug = 'bots-matrix'
export const title = 'Matrix'
export const section = 'chat-bots'
export const summary = 'Connect a Matrix bot user and a room. Unencrypted rooms only; replies list numbered choices.'

export function Content() {
  return (
    <>
      <Roles who={['admin', 'parent']} label="Who can connect:" />

      <H2>What you need</H2>
      <Table head={['Item', 'Example']} rows={[
        ['Homeserver URL', <code key="a">https://matrix.example.org</code>],
        ['Access token of a bot user', 'Create a separate user for the bot and copy its access token'],
        ['Room ID', <code key="b">!abc:example.org</code>],
      ]} />

      <H2>Connect it</H2>
      <Steps>
        <li>Create a Matrix user for the bot and get its <strong>access token</strong> (in Element: Settings › Help &amp; About › Advanced › Access Token).</li>
        <li>Create a room for your family, or pick an existing one, and <strong>invite the bot</strong> to it. The bot joins the invite to the configured room automatically.</li>
        <li>Copy the <strong>room ID</strong> (Room settings › Advanced). It looks like <code>!abc:example.org</code>, not the readable <code>#alias</code>.</li>
        <li>In Kinkit open <strong>Settings › Bots › Matrix</strong> and paste the homeserver URL, the access token and the room ID. Press <strong>Save</strong> and keep <strong>Enabled</strong> ticked.</li>
        <li><a href="/docs/bots-overview#link-your-chat-account">Link your account</a> with <code>!link CODE</code>.</li>
      </Steps>
      <Callout type="warning" title="Unencrypted rooms only">
        <p>The Matrix bot does not support end-to-end encryption. Use a room without encryption, or the bot will not be able to read your messages.</p>
      </Callout>

      <H2>Numbered choices instead of buttons</H2>
      <p>Matrix messages have no tap buttons. When Kinkit would show buttons, the reply lists numbered choices instead:</p>
      <ul>
        <li>Answer <code>!1</code>, <code>!2</code>… to pick one (a bare <code>1</code> also works while a list is waiting).</li>
        <li>Reminders posted by Kinkit are plain text. Answer them with <code>!rem &lt;id&gt; done</code>, <code>!rem &lt;id&gt; snooze</code> or <code>!rem &lt;id&gt; tomorrow</code>. The id is in the reminder message.</li>
      </ul>

      <H2>Using it</H2>
      <ul>
        <li>Commands are plain messages with a <code>!</code> prefix (<code>!today</code>, <code>!buy milk</code>), because Element treats <code>/…</code> as its own commands.</li>
        <li>Anything else in the household room that looks like <code>50k lunch</code> is logged as spending.</li>
        <li>Only the configured room is served.</li>
      </ul>
    </>
  )
}
