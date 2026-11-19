import { Roles } from '../components.jsx'

export const slug = 'assets'
export const title = 'Assets and net worth'
export const section = 'money'
export const summary = 'Balances across accounts and progress toward a net-worth target.'

export function Content() {
  return (
    <>
      <Roles who={['admin', 'parent', 'viewer']} label="Who can see this page:" />
      <p>Under <strong>Savings › Assets</strong>, track balances across accounts (liquid cash, savings, investments). Set a target net-worth goal and see progress toward it.</p>
      <p>Asset accounts are created under <a href="/docs/settings-categories">Settings › Categories &amp; accounts</a>. Your top <a href="/docs/goals">goals</a> also appear on this page.</p>
    </>
  )
}
