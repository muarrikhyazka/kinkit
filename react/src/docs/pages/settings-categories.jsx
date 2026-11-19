import { Roles, Callout } from '../components.jsx'

export const slug = 'settings-categories'
export const title = 'Categories and accounts'
export const section = 'settings-data'
export const summary = 'Income sources, asset accounts, spending categories, currency and the family name.'

export function Content() {
  return (
    <>
      <Roles who={['admin', 'parent']} />
      <p>Under <strong>Settings › Categories &amp; accounts</strong> you manage master data: income sources, asset accounts and spending categories, plus the <strong>family name</strong>.</p>
      <ul>
        <li>Categories are grouped under <strong>Needs / Wants / Savings</strong>. This grouping drives the budget breakdown.</li>
        <li>Your display currency converts amounts using daily exchange rates. Stored values stay in IDR.</li>
        <li>If a category or income source is still in use, Kinkit offers a <strong>Force delete</strong>. Existing transactions keep the old name as text.</li>
        <li>The family name shows under the Kinkit name in the sidebar, in the browser tab title and on the TV view.</li>
      </ul>
      <Callout type="note">
        <p>Chat keywords (which words map to which category when you log from chat) are edited under <strong>Settings › Chat keywords</strong>. See <a href="/docs/bot-commands">Chat commands</a>.</p>
      </Callout>
    </>
  )
}
