import { H2, Roles, Callout } from '../components.jsx'
import Shot from '../Shot.jsx'

export const slug = 'chores'
export const title = 'Chores'
export const section = 'family-home'
export const summary = 'Points, photo proof, parent approval and turns.'

export function Content() {
  return (
    <>
      <p>Chores have points, optional photo proof and optional parent approval. Approved chores add to a kid&apos;s allowance. Find them in the sidebar under <strong>Family › Chores</strong>.</p>
      <Shot name="chores" alt="Chores with who has what today." />

      <H2>Add a chore</H2>
      <Roles who={['admin', 'parent']} />
      <p>Choose <strong>Add chore</strong>: a title, points, an optional money reward, and how often (once, every day, chosen weekdays or monthly, with an optional &ldquo;due by&rdquo; time).</p>
      <p><strong>Who does it:</strong> one person, <strong>take turns</strong> (each time the chore comes up the next person in the list gets it) or <strong>anyone</strong> (whoever does it first gets the credit).</p>

      <H2>Do a chore</H2>
      <Roles who={['kid', 'parent', 'admin']} />
      <p>Kids see big cards for today and tap <strong>Done!</strong>, optionally with a photo from the camera. If a chore needs a photo, the button opens the camera.</p>

      <H2>Approval</H2>
      <p>With <strong>approval</strong> switched on, a finished chore waits in the parent&apos;s <strong>Waiting for approval</strong> list (with the photo). <strong>Approve</strong> gives the points, or <strong>Send back</strong> with a note so they can try again.</p>

      <H2>Points, undo and the week view</H2>
      <ul>
        <li>Points land on the Leaderboard and count toward levels, the <strong>Helping hand</strong> (10 chores) and <strong>Chore champion</strong> (50 chores) badges and the weekly <strong>Finish all your chores</strong> challenge. Money rewards are added on top of the kid&apos;s monthly allowance, shown under <strong>Settings › Members</strong>.</li>
        <li>Made a mistake? Every action has <strong>Undo</strong>; undoing or sending back a chore also takes back its points and reward.</li>
        <li>The <strong>Week view</strong> shows everyone&apos;s chores per day (dashed means coming up). Chores that were not done stay overdue until the next one comes along, then count as missed.</li>
        <li>Linked chat accounts can list today&apos;s chores (<code>/chores</code>) and mark them done from the chat bot.</li>
      </ul>
      <Callout type="note">
        <p>Viewers cannot complete chores.</p>
      </Callout>
    </>
  )
}
