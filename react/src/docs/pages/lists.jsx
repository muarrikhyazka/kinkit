import { H2, Code, Callout } from '../components.jsx'

export const slug = 'lists'
export const title = 'Shopping lists'
export const section = 'family-home'
export const summary = 'Shared lists, quick item entry, in-store ticking and one spending entry per trip.'

export function Content() {
  return (
    <>
      <p>Shared shopping lists for the whole household, under <strong>Family › Lists</strong>. A <strong>Groceries</strong> list is ready to go; add more (hardware, pharmacy…) with <strong>New list</strong> and switch between them with the chips at the top.</p>

      <H2>Add items</H2>
      <p>Type several items at once; quantities are picked up automatically. Pasting a list with one item per line works too.</p>
      <Code title="Example">{`eggs, 2 milk, bread x3, 1kg rice`}</Code>
      <p>Suggestions come from what your household bought before, and an item keeps the aisle/category you gave it last time.</p>

      <H2>In the store</H2>
      <ul>
        <li>Tap an item to put it <strong>in the cart</strong> (it is crossed out and moves down).</li>
        <li>Swipe left or tap × to remove an item. You have a few seconds to undo.</li>
        <li>Ticking items works with a bad connection: ticks are saved when you are back online.</li>
        <li>Everyone sees changes within about 15 seconds, so you can split up in the store.</li>
      </ul>

      <H2>Finish shopping</H2>
      <p><strong>Finish shopping</strong> asks for the total you paid (shorthand like <kbd>250k</kbd> works) and a category, then logs <em>one</em> spending entry for the trip (it counts for points and streaks) and clears the cart. Tapping it twice never logs twice.</p>

      <H2>Staples</H2>
      <p><strong>Staples</strong> come back on their own, for example &ldquo;Rice, 5kg, every 14 days&rdquo;: it is added to the list when due and shows up on Today.</p>

      <H2>Who can do what</H2>
      <ul>
        <li>Kids can add items and tick them off.</li>
        <li>Parents finish the trip and manage lists.</li>
        <li>Viewers can only look.</li>
      </ul>
      <Callout type="tip">
        <p>With a chat bot connected you can add items (&ldquo;eggs, milk&rdquo;), see the list and finish shopping from chat: <code>/buy milk, 2 eggs</code>, <code>/list</code>, <code>/done_shopping 250k</code>.</p>
      </Callout>
    </>
  )
}
