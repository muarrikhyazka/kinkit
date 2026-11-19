import { Link } from 'react-router-dom'
import { H2, Callout, Steps } from '../components.jsx'

export const slug = 'login'
export const title = 'Log in and sign up'
export const section = 'getting-started'
export const summary = 'Create a household, or join one the family admin set up for you.'

export function Content() {
  return (
    <>
      <p>The login screen is the front door of Kinkit. It shows the Kinkit mascot, an <strong>Email</strong> field and a <strong>Password</strong> field, and one button that switches between logging in and signing up.</p>

      <H2>Create a new household</H2>
      <Steps>
        <li>Open the app and choose <strong>Don&apos;t have an account? Sign up</strong>.</li>
        <li>Enter your email, an optional <strong>Name</strong> and a password of at least 8 characters.</li>
        <li>Press <strong>Create account</strong>. You are logged in and become the <strong>Family Admin</strong> of a brand-new household.</li>
        <li>Follow the <strong>Getting started</strong> checklist on the <strong>Today</strong> page: set up categories, set a budget, log a first expense and invite the family on the Family page.</li>
      </Steps>

      <H2>Join an existing family</H2>
      <p>Do <strong>not</strong> sign up. Ask the family admin or a parent to add you on the <strong>Family</strong> page with your email and a temporary password. Then <strong>Login</strong> with that email and password and change the password on the Family page.</p>
      <Callout type="note">
        <p>Signing up always creates a <em>new</em> household. If you sign up by mistake you end up alone in an empty household instead of in your family&apos;s.</p>
      </Callout>

      <H2>Logging in</H2>
      <p>Enter your email and password and press <strong>Login</strong>. Every member logs in with their own email, so points, chores and reminders are always attributed to the right person.</p>
      <ul>
        <li>The sidebar footer (or <strong>More</strong> on a phone) has <strong>Logout</strong>.</li>
        <li>Chat bots do not use your password. You link a chat account with a one-time code: see <Link to="/docs/bots-overview#link-your-chat-account">Link your chat account</Link>.</li>
        <li>TV and car displays use a share link instead of logging in: see <Link to="/docs/tv-links">TV &amp; car links</Link>.</li>
      </ul>
    </>
  )
}
