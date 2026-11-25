import { H2, Callout, Roles } from '../components.jsx'
import Shot from '../Shot.jsx'

export const slug = 'meals'
export const title = 'Meal plan'
export const section = 'family-home'
export const summary = 'A weekly menu with must-use ingredients and posts to the family chat.'

export function Content() {
  return (
    <>
      <p>Under <strong>Family › Meals</strong> you get a weekly menu with one main dish and one side per day (everyday home cooking), at most two fried dishes a week. It stays the same all week; switch between <strong>This week</strong> and <strong>Next week</strong> at the top.</p>
      <Shot name="meals" alt="The weekly menu with meal requests waiting for a decision." />

      <H2>Must-use ingredients</H2>
      <Roles who={['admin', 'parent']} />
      <p>Add something left over or that you want to cook, and the menu gets a dish with it. The chat bot can do the same with <code>/addingredient chicken</code> and <code>/removeingredient chicken</code>; <code>/ingredients</code> lists them.</p>

      <H2>Post to the family chat</H2>
      <p>Parents can have next week&apos;s menu posted to the connected chat (Telegram, Discord, Slack or Matrix) on chosen days and time, for example Friday 06:00. In chat, <code>/menu</code> shows next week and <code>/menu this</code> shows this week; <code>/schedule</code> shows and changes when it is posted.</p>

      <H2>Grocery promos</H2>
      <p>When the server is set up to read grocery promos (Super Indo and Astro in Indonesia), dishes whose main ingredient is on promo are preferred and the promo is shown next to the day. The promo is listed with the store&apos;s own product name.</p>

      <Callout type="tip">
        <p>Want a specific dish? Ask for it: see <a href="/docs/meal-requests">Meal requests</a>.</p>
      </Callout>
    </>
  )
}
