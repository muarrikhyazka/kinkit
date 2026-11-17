import { IconDashboard, IconSpending, IconBudget, IconAssets, IconSetup, IconPlus, IconEdit, IconLink, IconMore } from '../icons.jsx'
import PageHeader from '../PageHeader.jsx'
import { IconDevices, IconPaperclip, IconDownload } from '../Receipts.jsx' // --- data ---
import { IconRepeat, IconIncome, IconGoal } from '../moneyIcons.jsx' // --- money ---
import { IconFamily } from './Family.jsx' // --- family ---

function Section({ Icon, title, children }) {
  return (
    <div className="card">
      <h2 style={{ display: 'flex', alignItems: 'center', gap: '.5rem' }}>
        <Icon width={18} height={18} aria-hidden="true" /> {title}
      </h2>
      {children}
    </div>
  )
}

export default function Guide() {
  return (
    <div>
      <PageHeader title="Guide" subtitle="How to use Kinkit, page by page." />

      <Section Icon={IconPlus} title="Add Spending">
        <p>
          Click the green <strong>Add Spending</strong> button in the sidebar (or the round <strong>+</strong> button on your phone) from any page.
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

      <Section Icon={IconSpending} title="Spending">
        <p>
          Full transaction list for the selected month. Search by description, filter by category, and click any column header to sort.
          Click the <IconEdit width={13} height={13} role="img" aria-label="edit" /> edit icon on a row to change its date, description, category (via the category select) or amount - <kbd>Enter</kbd> saves, <kbd>Esc</kbd> cancels.
          Tick <strong>Checked</strong> once you've reconciled a transaction.
        </p>
      </Section>

      <Section Icon={IconBudget} title="Budgeting">
        <p>
          Set your planned income per source and planned expense per category for the selected year. Values save automatically when you leave a cell:
          a spinner shows while saving, a check mark when saved, and a red marker if it failed (hover it to see why - just edit and leave the cell again to retry).
        </p>
      </Section>

      <Section Icon={IconAssets} title="Assets">
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
          <strong>Budget alerts:</strong> with Telegram or Discord connected on the Integrations page (bot token and chat id filled in), Kinkit sends a chat message when a category reaches 80% of its monthly budget and again when it goes over 100% - once each per category per month.
          Checks run every few minutes for the current month (UTC). Turn alerts on or off in the <strong>Budget alerts</strong> card at the bottom of the Budgeting page.
        </p>
      </Section>

      <Section Icon={IconIncome} title="Income">
        <p>
          Budgeting holds the income you <em>plan</em>; the <strong>Income</strong> page records money you actually <em>received</em>. Pick a date, a source and an amount.
          The page compares what came in against the plan for the month, and the Dashboard shows received vs planned income in the Month, Year and Across Years views.
        </p>
      </Section>

      <Section Icon={IconRepeat} title="Recurring">
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

      <Section Icon={IconGoal} title="Goals">
        <p>
          Create savings goals with a target amount and optional target date. Each goal shows a progress ring and, if it has a date, how much to save per month to get there.
          Use <strong>Add</strong> on a goal to record a contribution (a negative amount withdraws).
          You can also link a category (e.g. a Savings category - spending logged there since the goal was created counts) and assets (their current value counts).
          Your top goals also appear on the Assets page.
        </p>
      </Section>
      {/* --- end money --- */}

      <Section Icon={IconSetup} title="Setup">
        <p>
          Manage master data: income sources, asset accounts, and spending categories. Categories are grouped under Needs / Wants / Savings - this grouping drives the budgeting breakdown.
          Your display currency converts amounts using daily exchange rates (stored values stay in IDR).
          If a category or income source is still in use, Kinkit offers a <strong>Force delete</strong> (existing transactions keep the old name as text).
        </p>
      </Section>

      {/* --- family --- */}
      <Section Icon={IconFamily} title="Family & achievements">
        <p>
          A Kinkit account is a <strong>household</strong>. Whoever signed up is its <strong>Owner</strong>; on the <strong>Family</strong> page the owner or a parent
          can add members with their email and a temporary password (they can change it later on the Family page). Each member logs in with their own email.
        </p>
        <ul className="guide-list">
          <li><strong>Owner</strong> - everything, including renaming the household (Setup). The owner can't be removed.</li>
          <li><strong>Parent</strong> - full access to the household's money and can manage members (except the owner).</li>
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

      <Section Icon={IconMore} title="Deleting and undo">
        <p>
          Deleting a transaction, asset, setup item or integration doesn't ask for confirmation - instead a notification appears at the bottom with an <strong>Undo</strong> button.
          The delete only happens after about 5 seconds, so you have time to change your mind.
        </p>
      </Section>

      <Section Icon={IconLink} title="On your phone">
        <p>
          On small screens the sidebar is replaced by a bottom tab bar: <strong>Dashboard</strong>, <strong>Spending</strong>, <strong>Budgeting</strong>, <strong>Assets</strong> and <strong>More</strong>.
          <strong> More</strong> opens a sheet with Setup, Integrations, this Guide, the theme picker, TV view and logout. The round <strong>+</strong> button adds spending from anywhere.
        </p>
      </Section>

      {/* --- data --- */}
      <Section Icon={IconDevices} title="Data & devices (import, export, TV links)">
        <p>Open <strong>Data & devices</strong> in the sidebar (or the <strong>More</strong> sheet on your phone).</p>
        <ul className="guide-list">
          <li><IconDownload width={13} height={13} aria-hidden="true" /> <strong>Export:</strong> download your spending as CSV (optionally for a date range), your income as CSV, or a <strong>full backup</strong> of everything as JSON. Your data is always yours to take.</li>
          <li><strong>Import a bank CSV:</strong> pick the format (BCA, Mandiri, generic debit/credit or generic date/description/amount) and upload the file. Nothing is saved yet - you get a preview first.
            Check the column mapping, date order and decimal separator, map categories from the file to yours, and change the category of any row or untick rows you don't want. Then press <strong>Import</strong>.</li>
          <li>Only money going <em>out</em> is imported as spending; income/credit rows are shown as skipped. Importing the same file twice never creates duplicates.</li>
          <li>Made a mistake? Press <strong>Undo this import</strong> right after, or use <strong>Undo</strong> next to it under <strong>Recent imports</strong> - it removes exactly the transactions from that import.</li>
          <li><strong>TV / car display links:</strong> create a link, then open it on a TV browser, wall tablet or car head unit. It shows this month's numbers without logging in and refreshes every minute.
            Anyone with the link can see those numbers - <strong>Revoke</strong> a link to switch it off instantly.</li>
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
          <li>Themes: Light, Dark, Playful (kids) and Senior (high-contrast) - pick one in the sidebar footer or the More sheet; saved per browser.</li>
          <li>Need to log spending from chat instead of opening the app? See the <strong>Integrations</strong> page to connect Telegram or Discord.</li>
          <li>TV view shows a big, auto-refreshing summary of the current month - handy for a TV or car head unit.</li>
        </ul>
      </div>
    </div>
  )
}
