import { H2, Callout, Code, Roles, Steps } from '../components.jsx'

export const slug = 'add-spending'
export const title = 'Add spending'
export const section = 'money'
export const summary = 'Log an expense in seconds: shorthand amounts, smart categories and keyboard flow.'

export function Content() {
  return (
    <>
      <Roles who={['admin', 'parent', 'kid']} />
      <p>Click the <strong>Add</strong> button at the top of the sidebar (or the round <strong>+</strong> button on your phone) from any page.</p>
      <Steps>
        <li>Fill in the date, description, category and amount.</li>
        <li>Add several rows at once with <strong>+ Add Row</strong>.</li>
        <li>Save. Kinkit also awards points and updates your streak: see <a href="/docs/points">Points and streaks</a>.</li>
      </Steps>

      <H2>Amount shorthand</H2>
      <p>Type a short form and a small hint shows the resolved amount:</p>
      <Code title="Examples">{`25k     ->  25.000
2rb     ->  2.000
1.5jt   ->  1.500.000
1.5m    ->  1.500.000`}</Code>

      <H2>Smart category</H2>
      <p>Kinkit remembers the category you last used for each description and fills it in for you (marked with a dashed outline). You can always change it.</p>

      <H2>Recent chips</H2>
      <p>Your last few descriptions appear as one-tap chips above the form.</p>

      <H2>Keyboard</H2>
      <ul>
        <li><kbd>Enter</kbd> moves to the next field and saves from the last one.</li>
        <li><kbd>Ctrl</kbd>/<kbd>Cmd</kbd>+<kbd>Enter</kbd> adds a row.</li>
        <li><kbd>Esc</kbd> closes the dialog.</li>
      </ul>
      <Callout type="tip">
        <p>Even faster: type the expense in the family chat, for example <code>makan siang 50rb</code>. See <a href="/docs/bot-commands">Chat commands</a>.</p>
      </Callout>
    </>
  )
}
