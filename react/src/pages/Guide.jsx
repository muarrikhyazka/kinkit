import { IconCalendar, IconWidgets } from '../icons.jsx'
import { IconDashboard, IconSpending, IconBudget, IconAssets, IconSetup, IconPlus, IconEdit, IconLink, IconMore } from '../icons.jsx'
import { IconHome, IconToday, IconBell } from '../icons.jsx' // --- home ---
import PageHeader from '../PageHeader.jsx'
import { ChevronLink } from '../ChevronLink.jsx'
import { IconDevices, IconPaperclip, IconDownload } from '../Receipts.jsx' // --- data ---
import { IconRepeat, IconIncome, IconGoal } from '../moneyIcons.jsx' // --- money ---
import { IconFamily } from './Family.jsx' // --- family ---

function Section({ Icon, title, children }) {
  return (
    <div className="card">
      <h2 className="card-title-icon">
        <Icon width={18} height={18} aria-hidden="true" /> {title}
      </h2>
      {children}
    </div>
  )
}

/** A screenshot of the real app (react/public/guide/<name>.jpg), shown under its section. */
function Shot({ name, alt, phone = false }) {
  return (
    <figure className={`guide-shot ${phone ? 'phone' : ''}`}>
      <img src={`/guide/${name}.jpg?v=2`} alt={alt} loading="lazy" width={phone ? 390 : 1280} />
      <figcaption className="muted">{alt}</figcaption>
    </figure>
  )
}

export default function Guide() {
  return (
    <div>
      <PageHeader title="Help" subtitle="How to use Kinkit, page by page." />

      <Section Icon={IconMore} title="Finding your way">
        <ul className="guide-list">
          <li><strong>Today</strong> (the start page) - everything that needs attention today: reminders, activities, chores, shopping lists, meal requests, budget and what's coming up.</li>
          <li><strong>Calendar</strong> - family activities in month, week, day and agenda views, with your reminders on top.</li>
          <li><strong>Reminders</strong> - routines, bills and birthdays, as a list or on a calendar.</li>
          <li><strong>Home</strong> - tabs for <strong>Chores</strong>, <strong>Lists</strong> (shopping) and <strong>Meals</strong> (weekly menu and meal requests).</li>
          <li><strong>Money</strong> group: <strong>Dashboard</strong> (overview charts), <strong>Transactions</strong> (Spending, Income, Recurring), <strong>Budget</strong> (budget vs actual) and <strong>Savings</strong> (Goals, Assets).</li>
          <li><strong>Family</strong> group: <strong>Family</strong> (members, allowances, leaderboard, achievements) and <strong>Widgets</strong> (your own screens for a phone, TV, car or monitor).</li>
          <li><strong>Settings</strong> (bottom of the sidebar) - <strong>Categories &amp; accounts</strong> (including the family name), <strong>Bots</strong> (Telegram/Discord), <strong>Data &amp; devices</strong> (import/export, TV links) and <strong>Display</strong> (theme, household time zone, morning digest).</li>
          <li><strong>Sidebar:</strong> the « button at the top folds it down to icons, and the <strong>Money</strong> / <strong>Family</strong> headings fold their group. Both are remembered on this device.</li>
          <li>This page is behind the <strong>?</strong> button. Old links such as /spending or /setup still work and open the new place.</li>
          <li>Kids see Today, Calendar, Home, Dashboard, their own spending and Family; viewers see everything except Settings (apart from Display).</li>
        </ul>
        <Shot name="today" alt="The Today page: reminders, activities, chores, shopping lists, budget and meal requests." />
      </Section>

      <Section Icon={IconCalendar} title="Calendar and activities">
        <p>
          The family calendar works like Google Calendar. Switch between <strong>Month</strong>, <strong>Week</strong>, <strong>Day</strong> and <strong>Agenda</strong>,
          step with the arrows and jump back with <strong>Today</strong>. Untick <strong>Reminders</strong> or <strong>Activities</strong> to focus on one.
        </p>
        <ul className="guide-list">
          <li><strong>Add an activity</strong> (family admin and parents): <strong>New activity</strong>, or <strong>Add on this day</strong> under the month grid. Set the title, all-day or start/end time, who it is for (nobody ticked = the whole family), where, notes and a color.</li>
          <li><strong>Repeat</strong> daily, weekly (pick the weekdays), monthly or yearly, every N days/weeks/months, with an optional end date. Click an activity and use <strong>Delete this one</strong> to skip a single day, or <strong>Delete series</strong>.</li>
          <li>Kids only see activities that include them or the whole family. Viewers can look but not change anything.</li>
          <li>On a phone the month shows dots; tap a day to see its list underneath.</li>
        </ul>
        <Shot name="calendar-month" alt="Month view with activities (solid) and reminders (dotted)." />
        <Shot name="calendar-week" alt="Week view with an hour grid and a red line for the current time." />
      </Section>

      <Section Icon={IconBell} title="Reminders: list and calendar">
        <p>
          <strong>Reminders</strong> has a <strong>List | Calendar</strong> switch. <strong>List</strong> shows what is due in the next 7 days plus every reminder grouped by category.
          <strong> Calendar</strong> shows the whole month: tap a day to see its reminders, tap one to mark it <strong>Done</strong> or snooze it.
          Family admin and parents add reminders from the Quick add templates or <strong>New reminder</strong>.
        </p>
        <Shot name="reminders-calendar" alt="Reminders as a month calendar, with the selected day listed underneath." />
      </Section>

      <Section Icon={IconHome} title="Home: chores, lists and meals">
        <ul className="guide-list">
          <li><strong>Chores</strong> - points, optional photo proof and parent approval. Approved chores add to a kid's allowance.</li>
          <li><strong>Lists</strong> - shopping lists: type <kbd>eggs, 2 milk, bread x3</kbd> and finish the trip into one spending entry.</li>
          <li><strong>Meals</strong> - the weekly menu. <strong>Everyone can request a dish</strong> under <em>Meal requests</em> (what it is, a note, and the day you would like it). The <strong>family admin and parents decide</strong>: <strong>Approve</strong> (optionally on a day) puts it on that day of the menu, <strong>Decline</strong> turns it down with the request kept in the history. You can withdraw your own request while it is waiting.</li>
        </ul>
        <Shot name="chores" alt="Chores with who has what today." />
        <Shot name="meals" alt="The weekly menu with meal requests waiting for a decision." />
      </Section>

      <Section Icon={IconWidgets} title="Widgets: your own screens">
        <p>
          <strong>Widgets</strong> lets you build a screen with just what you want to see: a clock, the calendar agenda, reminders, chores, shopping lists, today's meals, meal requests, budget, goals, family points and a note of your own.
          Start from a template (Phone, Living-room TV, Car screen, Desk monitor) or a blank board.
        </p>
        <ul className="guide-list">
          <li><strong>Edit layout</strong> (family admin and parents): <strong>drag a widget onto another</strong> to move it, or use the arrows. <strong>− / +</strong> change the width, <strong>▁ / █</strong> the height, <strong>✕</strong> removes it. Pick <strong>+ Add a widget</strong> to add more, then <strong>Save layout</strong>.</li>
          <li><strong>Show full screen</strong> fills the screen for an always-on phone, tablet or monitor and refreshes every minute. Press <kbd>Esc</kbd> to leave.</li>
          <li><strong>TV or car without logging in:</strong> under <strong>Settings › Data &amp; devices</strong> create a TV / car link and choose the board it shows. The Android TV, Samsung and LG apps and any TV browser show that board.</li>
          <li>Kids don't get money widgets (budget, goals). Everyone can view boards.</li>
        </ul>
        <Shot name="widgets-edit" alt="Editing a board: drag widgets, resize them, add more." />
        <Shot name="tv-board" alt="The same board on a TV link." />
      </Section>

      <Section Icon={IconPlus} title="Add Spending">
        <p>
          Click the <strong>Add</strong> button at the top of the sidebar (or the round <strong>+</strong> button on your phone) from any page.
          Fill date, description, category and amount, then save. Add several rows at once with <strong>+ Add Row</strong>.
        </p>
        <ul className="guide-list">
          <li><strong>Amount shorthand:</strong> type <kbd>25k</kbd> for 25.000, <kbd>2rb</kbd> for 2.000, <kbd>1.5jt</kbd> or <kbd>1.5m</kbd> for 1.500.000 - a small hint shows the resolved amount.</li>
          <li><strong>Smart category:</strong> Kinkit remembers the category you last used for each description and fills it in for you (marked with a dashed outline). You can always change it.</li>
          <li><strong>Recent chips:</strong> your last few descriptions appear as one-tap chips above the form.</li>
          <li><strong>Keyboard:</strong> <kbd>Enter</kbd> moves to the next field and saves from the last one, <kbd>Ctrl</kbd>/<kbd>Cmd</kbd>+<kbd>Enter</kbd> adds a row, <kbd>Esc</kbd> closes the dialog.</li>
        </ul>
      </Section>

      <Section Icon={IconDashboard} title="Dashboard">
        <p>
          Overview of your money across the <strong>Across Years</strong>, <strong>Year</strong>, <strong>Month</strong>, <strong>Week</strong> and <strong>Custom</strong> tabs.
          Each tab starts with the key numbers and one main chart. Extra pattern charts live under <strong>More insights</strong> (Year and Across Years) - it remembers whether you left it open.
          In <strong>Custom</strong> you pick which charts to show and drag (or use the arrow buttons) to reorder them.
        </p>
        <Shot name="dashboard" alt="The money dashboard." />
      </Section>

      <Section Icon={IconSpending} title="Period and filters">
        <p>
          Pages that depend on a period show a stepper in the page header: <strong>‹ Nov 2026 ›</strong>. Use the arrows to go back or forward a month (or a year on Budgeting),
          or click the label to jump to any month/year. Dates are in UTC.
        </p>
        <p>
          <strong>Hide large</strong> leaves out big one-off transactions so they don't distort your charts: choose "Only up to 10 juta" or "Only up to 100 juta" to include only transactions up to that amount.
        </p>
      </Section>

      <Section Icon={IconSpending} title="Transactions › Spending">
        <p>
          Full transaction list for the selected month. Search by description, filter by category, and click any column header to sort.
          Click the <IconEdit width={13} height={13} role="img" aria-label="edit" /> edit icon on a row to change its date, description, category (via the category select) or amount - <kbd>Enter</kbd> saves, <kbd>Esc</kbd> cancels.
          Tick <strong>Checked</strong> once you've reconciled a transaction.
        </p>
        <Shot name="spending" alt="The spending list for a month." />
      </Section>

      <Section Icon={IconBudget} title="Budget">
        <p>
          Set your planned income per source and planned expense per category for the selected year. Values save automatically when you leave a cell:
          a spinner shows while saving, a check mark when saved, and a red marker if it failed (hover it to see why - just edit and leave the cell again to retry).
        </p>
        <Shot name="budget" alt="Budget: planned income and expenses per category." />
      </Section>

      <Section Icon={IconGoal} title="Savings and family">
        <p><strong>Savings</strong> tracks goals (with monthly amount needed) and assets. <strong>Family</strong> shows the members, roles, allowances, leaderboard and achievements. Only the family admin and parents change members.</p>
        <Shot name="goals" alt="Savings goals with progress." />
        <Shot name="family" alt="Family members, allowances and achievements." />
      </Section>

      <Section Icon={IconAssets} title="Savings › Assets">
        <p>Track balances across accounts (liquid cash, savings, investments). Set a target net-worth goal and see progress toward it.</p>
      </Section>

      {/* --- money --- */}
      <Section Icon={IconBudget} title="Budget vs actual and alerts">
        <p>
          On <strong>Budgeting</strong>, every cell also shows what actually happened under the input: <em>Spent</em> for categories and <em>Got</em> for income sources.
          Expense cells are tinted green under 80% of the plan, amber from 80% to 100%, and red when over budget (or when you spent in a category with no budget).
        </p>
        <p>
          The Dashboard <strong>Month</strong> tab starts with a "Spent X of Y budgeted this month" bar and lists the categories that need attention.
        </p>
        <p>
          <strong>Budget alerts:</strong> with Telegram or Discord connected under Settings › Bots (bot token and chat id filled in), Kinkit sends a chat message when a category reaches 80% of its monthly budget and again when it goes over 100% - once each per category per month.
          Checks run every few minutes for the current month (UTC). Turn alerts on or off in the <strong>Budget alerts</strong> card at the bottom of the Budget page.
        </p>
      </Section>

      <Section Icon={IconIncome} title="Transactions › Income">
        <p>
          Budgeting holds the income you <em>plan</em>; the <strong>Income</strong> page records money you actually <em>received</em>. Pick a date, a source and an amount.
          The page compares what came in against the plan for the month, and the Dashboard shows received vs planned income in the Month, Year and Across Years views.
        </p>
      </Section>

      <Section Icon={IconRepeat} title="Transactions › Recurring">
        <p>
          Add rent, subscriptions, loan payments and other repeating spending once on the <strong>Recurring</strong> page: monthly (on a day of the month), weekly (on a weekday) or yearly.
          Kinkit adds each occurrence to Spending automatically (source "recurring"), including any past dates since the start date. Day 29-31 falls on the last day in shorter months.
        </p>
        <ul className="guide-list">
          <li>The list shows the <strong>next date</strong> each rule will run.</li>
          <li><strong>Pause</strong> stops new transactions; <strong>Resume</strong> continues from today without backfilling the paused period.</li>
          <li>Deleting a rule (with Undo) keeps the transactions it already created.</li>
        </ul>
      </Section>

      <Section Icon={IconGoal} title="Savings › Goals">
        <p>
          Create savings goals with a target amount and optional target date. Each goal shows a progress ring and, if it has a date, how much to save per month to get there.
          Use <strong>Add</strong> on a goal to record a contribution (a negative amount withdraws).
          You can also link a category (e.g. a Savings category - spending logged there since the goal was created counts) and assets (their current value counts).
          Your top goals also appear on the Assets page.
        </p>
      </Section>
      {/* --- end money --- */}

      <Section Icon={IconSetup} title="Settings › Categories & accounts">
        <p>
          Manage master data: income sources, asset accounts, and spending categories. Categories are grouped under Needs / Wants / Savings - this grouping drives the budgeting breakdown.
          Your display currency converts amounts using daily exchange rates (stored values stay in IDR).
          If a category or income source is still in use, Kinkit offers a <strong>Force delete</strong> (existing transactions keep the old name as text).
        </p>
      </Section>

      {/* --- family --- */}
      <Section Icon={IconFamily} title="Family & achievements">
        <p>
          A Kinkit account is a <strong>household</strong>. Whoever signed up is its <strong>Family Admin</strong>; on the <strong>Family</strong> page the family admin or a parent
          can add members with their email and a temporary password (they can change it later on the Family page). Each member logs in with their own email.
        </p>
        <ul className="guide-list">
          <li><strong>Family Admin</strong> - everything, including setting the <strong>family name</strong> (Settings › Categories &amp; accounts), which shows under the Kinkit name in the sidebar, in the browser tab title and on the TV view. The family admin can't be removed.</li>
          <li><strong>Parent</strong> - full access to the household's money and can manage members (except the family admin).</li>
          <li><strong>Kid</strong> - can see the household and log spending, and edit or delete only the entries they logged. A monthly <strong>allowance</strong> shows how much of it they have spent this month.</li>
          <li><strong>Viewer</strong> - read-only.</li>
        </ul>
        <p>
          Everyone earns <strong>points</strong> for logging spending (10 a day plus a streak bonus) and climbs levels. <strong>Badges</strong> reward good habits:
          a 7- or 30-day streak, setting your first budget, closing a month under budget (Needs + Wants), and hitting your Savings budget.
          Weekly <strong>family challenges</strong> (log every day, no-spend weekend, keep Wants under a week's share of the budget) add bonus points to the household total once per week.
          The leaderboard shows points earned this month. Days and weeks follow UTC (weeks run Monday to Sunday).
        </p>
      </Section>
      {/* --- end family --- */}

      {/* --- lists --- */}
      {/* --- chores --- */}
      <Section Icon={IconToday} title="Today">
        <p>
          The start page. It greets you and collects what matters today: reminders due (and overdue), chores, shopping lists,
          how much of this month's budget is used, and reminders coming up in the next days. Tick reminders and chores
          straight from here; the family admin and parents can approve chores kids have finished.
        </p>
        <p>
          Days follow the <strong>household time zone</strong> (Settings › Display). Data is stored in UTC, the time zone only
          decides when "today" starts and when "every day at 08:00" fires. The family admin and parents are asked once to switch from UTC to
          the time zone of their device.
        </p>
      </Section>

      <Section Icon={IconBell} title="Reminders">
        <ul className="guide-list">
          <li>The family admin and parents add reminders for bills, medicine, school, birthdays, chores or anything else: once, daily, weekly (pick days), monthly (day 31 means the last day of shorter months) or yearly.</li>
          <li>Assign a reminder to one person or to the whole family. Optionally pick someone to tell if nobody marks it done within a number of minutes.</li>
          <li>Reminders arrive in the family's Telegram or Discord chat with <strong>Done</strong> and <strong>Snooze</strong> buttons, and show on Today. Everyone except viewers can mark their own (or whole-family) reminders done.</li>
          <li><strong>Morning digest</strong>: every morning (07:00 by default, change it in Settings › Display) the chat gets a short summary of the day.</li>
        </ul>
      </Section>

      <Section Icon={IconLink} title="Chat commands and linking">
        <p>
          Link your own chat account once: <strong>Reminders › Link my chat account</strong> gives a code, then send
          <kbd>/link CODE</kbd> to the household bot. Reminders for you then mention you, and what you do in chat counts as you
          (for example chore points).
        </p>
        <ul className="guide-list">
          <li><kbd>/today</kbd> - today's reminders, chores and budget</li>
          <li><kbd>/remind tomorrow 8am take medicine</kbd>, <kbd>/remind every monday 7:00 trash</kbd>, <kbd>/remind in 2 hours call grandma @Mom</kbd> - add a reminder</li>
          <li><kbd>/reminders</kbd> - what's coming up</li>
          <li><kbd>/chores</kbd> - today's chores, with Done buttons</li>
          <li><kbd>/buy milk, 2 eggs</kbd>, <kbd>/list</kbd>, <kbd>/done_shopping 250k</kbd> - shopping list</li>
          <li><kbd>/recent</kbd> (or <kbd>/recent 10</kbd>) - last expenses</li>
          <li><kbd>/menu</kbd> (next week) or <kbd>/menu this</kbd> - weekly meal plan; <kbd>/ingredients</kbd>, <kbd>/addingredient chicken</kbd>, <kbd>/removeingredient chicken</kbd> - must-use ingredients</li>
          <li><kbd>/schedule</kbd> - when the meal plan is posted, plus upcoming reminders; <kbd>/schedule meal fri 06:00</kbd> or <kbd>/schedule meal off</kbd> to change it (parents)</li>
          <li><kbd>/feedback weekly report please</kbd> - send a feature request; <kbd>/about</kbd>, <kbd>/help</kbd></li>
          <li>Indonesian command names work too: <kbd>/bantuan</kbd>, <kbd>/hariini</kbd>, <kbd>/ingatkan</kbd>, <kbd>/pengingat</kbd>, <kbd>/last</kbd>, <kbd>/koreksi</kbd>, <kbd>/koreksitgl</kbd>, <kbd>/tanggal</kbd>, <kbd>/bahan</kbd>, <kbd>/tambahbahan</kbd>, <kbd>/hapusbahan</kbd>, <kbd>/jadwal</kbd>, <kbd>/fitur</kbd>, <kbd>/pengeluaran</kbd>.</li>
        </ul>
        <p><strong>Logging spending from chat</strong></p>
        <ul className="guide-list">
          <li>Type the expense with the amount first or last, one per line: <kbd>makan siang 50rb</kbd>, <kbd>bensin rp20.000</kbd>, <kbd>50k lunch</kbd>. <kbd>k</kbd>/<kbd>rb</kbd> mean thousand, <kbd>jt</kbd>/<kbd>juta</kbd> million.</li>
          <li>If a word in the description is a known keyword the expense is saved right away. Otherwise the bot shows category buttons (and a <strong>Not an expense</strong> button); after you pick one it remembers the words, so next time it is automatic. Keywords can be edited in <strong>Settings › Chat keywords</strong>.</li>
          <li>Wrong category? <kbd>/fix bakso Food</kbd> (or <kbd>/fix 2 Food</kbd> for the second most recent) shows the matching expenses as buttons. Wrong day? <kbd>/fixdate bakso 2026-10-20</kbd>.</li>
          <li>Catching up on old receipts? <kbd>/date 2026-10-20</kbd> logs the next expenses on that day until <kbd>/date reset</kbd>.</li>
          <li>In a group chat the bot only answers in the household chat set under Settings › Bots. To log plain messages (not just commands) in a Telegram group, turn the bot's privacy mode off in BotFather (<kbd>/setprivacy</kbd> › Disable) or make it a group admin.</li>
        </ul>
      </Section>

      <Section Icon={IconHome} title="Home › Chores">
        <ul className="guide-list">
          <li><strong>Parents</strong> add chores with <strong>Add chore</strong>: a title, points, an optional money reward, and how often (once, every day, chosen weekdays or monthly, with an optional "due by" time).</li>
          <li><strong>Who does it</strong>: one person, <strong>take turns</strong> (each time the chore comes up the next person in the list gets it) or <strong>anyone</strong> (whoever does it first gets the credit).</li>
          <li><strong>Kids</strong> see big cards for today and tap <strong>Done!</strong> - optionally with a photo from the camera. If a chore needs a photo, the button opens the camera.</li>
          <li>With <strong>approval</strong> switched on, a finished chore waits in the parent's <strong>Waiting for approval</strong> list (with the photo). Approve to give the points, or <strong>Send back</strong> with a note so they can try again.</li>
          <li>Points land on the Family leaderboard and count toward levels, the <strong>Helping hand</strong> (10 chores) and <strong>Chore champion</strong> (50 chores) badges and the weekly <strong>Finish all your chores</strong> challenge. Money rewards are added on top of the kid's monthly allowance on the Family page.</li>
          <li>Made a mistake? Every action has <strong>Undo</strong>; undoing or sending back a chore also takes back its points and reward.</li>
          <li>The <strong>Week view</strong> shows everyone's chores per day (dashed = coming up). Chores that weren't done stay overdue until the next one comes along, then count as missed.</li>
          <li>Linked Telegram/Discord accounts can list today's chores and mark them done from the chat bot.</li>
        </ul>
      </Section>

      <Section Icon={IconHome} title="Home › Lists (shopping)">
        <ul className="guide-list">
          <li>Shared shopping lists for the whole household. A <strong>Groceries</strong> list is ready to go; add more (hardware, pharmacy…) with <strong>New list</strong> and switch between them with the chips at the top.</li>
          <li>Type several items at once: <em>eggs, 2 milk, bread x3, 1kg rice</em> - quantities are picked up automatically. Pasting a list with one item per line works too. Suggestions come from what your household bought before, and an item keeps the aisle/category you gave it last time.</li>
          <li>In the store, tap an item to put it <strong>in the cart</strong> (it is crossed out and moves down). Swipe left or tap × to remove an item - you have a few seconds to undo. Ticking items works with a bad connection: ticks are saved when you are back online.</li>
          <li>Everyone sees changes within about 15 seconds, so you can split up in the store.</li>
          <li><strong>Finish shopping</strong> asks for the total you paid (shorthand like 250k works) and a category, then logs <em>one</em> spending entry for the trip (counts for points and streaks) and clears the cart. Tapping it twice never logs twice.</li>
          <li><strong>Staples</strong> come back on their own, e.g. “Rice, 5kg, every 14 days” - it is added to the list when due and shows up on Today.</li>
          <li>Kids can add items and tick them off; parents finish the trip and manage lists. Viewers can only look.</li>
          <li>Chat bots: if a bot is connected you can add items (“eggs, milk”), see the list and finish shopping from Telegram/Discord.</li>
        </ul>
      </Section>
      {/* --- end lists --- */}

      <Section Icon={IconHome} title="Home › Meals">
        <ul className="guide-list">
          <li>A weekly menu with one main dish and one side per day (Indonesian home cooking), at most two fried dishes a week. It stays the same all week; switch between <strong>This week</strong> and <strong>Next week</strong> at the top.</li>
          <li><strong>Must-use ingredients</strong>: add something left over or that you want to cook (parents) and the menu gets a dish with it. The chat bot can do the same with <kbd>/addingredient</kbd> and <kbd>/removeingredient</kbd>.</li>
          <li><strong>Post to the family chat</strong>: parents can have next week's menu posted to the connected Telegram/Discord chat on chosen days and time (for example Friday 06:00).</li>
          <li>When the server is set up to read grocery promos, dishes whose main ingredient is on promo are preferred and the promo is shown next to the day.</li>
        </ul>
      </Section>

      <Section Icon={IconMore} title="Deleting and undo">
        <p>
          Deleting a transaction, asset, setup item or integration doesn't ask for confirmation - instead a notification appears at the bottom with an <strong>Undo</strong> button.
          The delete only happens after about 5 seconds, so you have time to change your mind.
        </p>
      </Section>

      <Section Icon={IconLink} title="On your phone">
        <p>
          On small screens the sidebar is replaced by a bottom tab bar: <strong>Today</strong>, <strong>Home</strong>, <strong>Transactions</strong> and <strong>More</strong>.
          <strong> More</strong> opens a sheet with Dashboard, Budget, Savings, Family, Settings, Help, the theme picker and logout. The round <strong>+</strong> button adds spending (or income, for the family admin and parents) from anywhere.
        </p>
      </Section>

      {/* --- data --- */}
      <Section Icon={IconDevices} title="Data & devices (import, export, TV links)">
        <p>Open <strong>Settings › Data & devices</strong> in the sidebar (or the <strong>More</strong> sheet on your phone).</p>
        <ul className="guide-list">
          <li><IconDownload width={13} height={13} aria-hidden="true" /> <strong>Export:</strong> download your spending as CSV (optionally for a date range), your income as CSV, or a <strong>full backup</strong> of everything as JSON. Your data is always yours to take.</li>
          <li><strong>Import a bank CSV:</strong> pick the format (BCA, Mandiri, generic debit/credit or generic date/description/amount) and upload the file. Nothing is saved yet - you get a preview first.
            Check the column mapping, date order and decimal separator, map categories from the file to yours, and change the category of any row or untick rows you don't want. Then press <strong>Import</strong>.</li>
          <li>Only money going <em>out</em> is imported as spending; income/credit rows are shown as skipped. Importing the same file twice never creates duplicates.</li>
          <li>Made a mistake? Press <strong>Undo this import</strong> right after, or use <strong>Undo</strong> next to it under <strong>Recent imports</strong> - it removes exactly the transactions from that import.</li>
          <li><strong>TV / car display links:</strong> create a link, then open it on a TV browser, wall tablet or car head unit. It shows this month's numbers without logging in and refreshes every minute.
            Anyone with the link can see those numbers - <strong>Revoke</strong> a link to switch it off instantly.</li>
          <li>Each link can show the month summary or one of your <strong>widget boards</strong> - pick it in the "Shows" list.</li>
        </ul>
      </Section>

      <Section Icon={IconPaperclip} title="Receipts">
        <p>
          Keep the photo of a receipt with its transaction. When adding spending, tap <strong>Attach receipt</strong> (or <strong>Camera</strong> on your phone) under a row.
          <strong> Read amount from receipt</strong> reads the photo on your device and fills in the total it finds - always double-check it. It downloads a text-recognition helper the first time, so it needs a connection.
        </p>
        <p>
          In the Spending list, the <IconPaperclip width={13} height={13} role="img" aria-label="paperclip" /> paperclip on a row attaches a photo; once a row has one, the paperclip is highlighted and opens the photo, where you can replace or remove it.
          Photos up to 5 MB (JPEG, PNG, WebP or HEIC) are stored privately - only your household can see them. Receipts can't be attached while offline; add them later from the Spending list.
        </p>
      </Section>

      <Section Icon={IconDevices} title="Install as an app">
        <p>Kinkit can be installed like a normal app, with its own icon and full-screen window:</p>
        <ul className="guide-list">
          <li><strong>Android (Chrome):</strong> menu ⋮ → <strong>Install app</strong> / <strong>Add to Home screen</strong>.</li>
          <li><strong>iPhone / iPad (Safari):</strong> Share button → <strong>Add to Home Screen</strong>.</li>
          <li><strong>Desktop (Chrome / Edge):</strong> the install icon at the right of the address bar.</li>
        </ul>
        <p>
          The installed app opens even without a connection and shows the data from your last visit. Spending you add offline is kept on the device (it survives closing the app) and syncs automatically once you're back online.
        </p>
      </Section>
      {/* --- end data --- */}

      <div className="card">
        <h2>Tips</h2>
        <ul className="guide-list">
          <li>Works offline: spending added while offline queues locally and syncs once the server is reachable again (see the connection dot). If the server rejects a queued entry, it's listed at the top of the page so nothing disappears silently.</li>
          <li>Themes: the sun/moon button at the bottom of the sidebar switches between <strong>light and dark</strong> (same colors and fonts, white swapped for black). <strong>Playful</strong> (kids) and <strong>Senior</strong> (high-contrast, larger text) are under Settings › Display (or the More sheet on your phone). Saved per device.</li>
          <li>Need to log spending from chat instead of opening the app? Connect Telegram or Discord under <strong>Settings › Bots</strong>.</li>
          <li>The TV view shows a big, auto-refreshing screen: the current month summary, or a widget board you built - handy for a TV or car head unit.</li>
        </ul>
        <div className="guide-goto">
          <ChevronLink to="/settings/bots">Settings › Bots</ChevronLink>
          <ChevronLink to="/settings/display">Settings › Display</ChevronLink>
        </div>
      </div>
    </div>
  )
}
