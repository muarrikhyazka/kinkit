import { H2, Roles, Steps, Callout } from '../components.jsx'

export const slug = 'meal-requests'
export const title = 'Meal requests'
export const section = 'family-home'
export const summary = 'Anyone can ask for a dish; parents decide and approved dishes land on the menu.'

export function Content() {
  return (
    <>
      <p>Meal requests let the whole family have a say in the weekly menu. They live on the <a href="/docs/meals">Meals</a> page under <strong>Meal requests</strong> and also show on <a href="/docs/today">Today</a>.</p>

      <H2>Request a dish</H2>
      <Roles who={['admin', 'parent', 'kid', 'viewer']} label="Who can request:" />
      <Steps>
        <li>Open <strong>Family › Meals</strong> and find <strong>Meal requests</strong>.</li>
        <li>Say what the dish is, add an optional note and the day you would like it.</li>
        <li>Send it. It waits for a decision, and you can <strong>withdraw</strong> your own request while it is waiting.</li>
      </Steps>

      <H2>Decide</H2>
      <Roles who={['admin', 'parent']} label="Who can decide:" />
      <ul>
        <li><strong>Approve</strong> (optionally on a day) puts the dish on that day of the menu.</li>
        <li><strong>Decline</strong> turns it down. The request is kept in the history.</li>
      </ul>
      <Callout type="note">
        <p>Approved dishes show on the menu, and meal requests can also be shown as a widget on your <a href="/docs/widgets">widget boards</a>.</p>
      </Callout>
    </>
  )
}
