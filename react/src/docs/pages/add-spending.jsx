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
1.5k    ->  1.500
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
        <p>Even faster: type the expense in the family chat, for example <code>lunch 50k</code>. See <a href="/docs/bot-commands">Chat commands</a>.</p>
      </Callout>

      <H2>On Android: from notifications</H2>
      <Roles who={['admin', 'parent', 'kid']} />
      <p>The Android phone app can turn bank and e-wallet payment notifications into expenses. It is off by default and not available on iPhone, because iOS does not let an app read other apps&apos; notifications.</p>
      <Steps>
        <li>In the phone app open <strong>Settings › Expense capture</strong> and switch on <strong>Capture expenses from notifications</strong>.</li>
        <li>Press <strong>Open notification access</strong> and allow Kinkit in the Android list. If Android says the setting is restricted, open the Kinkit app info, tap the menu and choose <strong>Allow restricted settings</strong>.</li>
        <li>When a payment notification arrives, it shows up under <strong>More › Captured expenses</strong> with the app, time, amount and merchant.</li>
        <li>Tap one to review it. Fix the description, amount, category or date, then save. <strong>Dismiss</strong> removes it without saving.</li>
      </Steps>
      <ul>
        <li>The notification text stays on your phone. Only an expense you review and save is sent to Kinkit, marked as coming from a notification.</li>
        <li>Notifications about money coming in can only be dismissed. Amounts in a currency other than yours show a warning before you save.</li>
        <li>Viewers can read the list but cannot save expenses.</li>
      </ul>
    </>
  )
}
