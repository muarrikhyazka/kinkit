import { H2, H3, Callout, Table } from '../components.jsx'

export const slug = 'bot-commands'
export const title = 'Chat commands and logging spending'
export const section = 'chat-bots'
export const summary = 'Every command, plus how to log expenses and fix mistakes from chat.'

export function Content() {
  return (
    <>
      <Callout type="note" title="Command prefix">
        <p>On Telegram and Discord commands start with <code>/</code>. On Slack and Matrix use <code>!</code> instead (for example <code>!today</code>); <code>/</code> is swallowed by those apps. Link your account first: <a href="/docs/bots-overview#link-your-chat-account">Link your chat account</a>.</p>
      </Callout>

      <H2>Commands</H2>
      <Table head={['Command', 'What it does']} rows={[
        [<code key="1">/today</code>, 'Today\'s reminders, chores and budget'],
        [<code key="2">/remind tomorrow 8am take medicine</code>, <>Add a reminder (also <code>/remind every monday 7:00 trash</code>, <code>/remind in 2 hours call grandma @Mom</code>)</>],
        [<code key="3">/reminders</code>, 'What is coming up'],
        [<code key="4">/chores</code>, 'Today\'s chores, with Done buttons'],
        [<code key="5">/buy milk, 2 eggs</code>, 'Add to the shopping list'],
        [<code key="6">/list</code>, 'Show the shopping list'],
        [<code key="7">/done_shopping 250k</code>, 'Finish shopping with the total'],
        [<><code>/recent</code> (or <code>/recent 10</code>)</>, 'Last expenses'],
        [<><code>/menu</code> / <code>/menu this</code></>, 'Weekly meal plan (next week / this week)'],
        [<code key="10">/ingredients</code>, <>Must-use ingredients; <code>/addingredient chicken</code> and <code>/removeingredient chicken</code> change them</>],
        [<code key="11">/schedule</code>, <>When the meal plan is posted, plus upcoming reminders. <code>/schedule meal fri 06:00</code> or <code>/schedule meal off</code> to change it (parents)</>],
        [<code key="12">/feedback weekly report please</code>, 'Send a feature request'],
        [<><code>/about</code>, <code>/help</code></>, 'About Kinkit and the command list'],
        [<code key="14">/link CODE</code>, 'Link your chat account'],
      ]} />

      <H2>Log spending from chat</H2>
      <ul>
        <li>Type the expense with the amount first or last, one per line: <code>lunch 50k</code>, <code>fuel 20000</code>, <code>50k lunch</code>. <code>k</code> means thousand, so <code>50k</code> is 50,000.</li>
        <li>If a word in the description is a known keyword the expense is saved right away. Otherwise the bot shows category choices (and a <strong>Not an expense</strong> option); after you pick one it remembers the words, so next time it is automatic. Edit keywords in <strong>Settings › Chat keywords</strong>.</li>
      </ul>

      <H3>Fix a mistake</H3>
      <ul>
        <li>Wrong category? <code>/fix lunch Food</code> (or <code>/fix 2 Food</code> for the second most recent) shows the matching expenses as choices.</li>
        <li>Wrong day? <code>/fixdate lunch 2026-10-20</code>.</li>
        <li>Catching up on old receipts? <code>/date 2026-10-20</code> logs the next expenses on that day until <code>/date reset</code>.</li>
      </ul>
    </>
  )
}
