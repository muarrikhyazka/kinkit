# Kinkit web: design and UX audit

Scope: `react/src/**`. I judged it from the code and the built CSS. There was no live login.
Not touched because other people own them: `Login.jsx`, `Today.jsx`, `Celebration.jsx`, `Mascot.jsx`, `mascot.css`, `KinkitLogo`.

## Findings: what was wrong and why it matters

| # | Finding | Where | Why it matters |
|---|---|---|---|
| 1 | There were no tokens beyond colours. I counted about 40 font sizes (.6–1.5rem), radii of 4/6/8/9/10/12/14/18/20/22/50%, and paddings picked by eye. | `index.css` throughout | The lack of rhythm is what makes it look generated. Every component looked slightly different from the next. |
| 2 | Gradients were used as decoration: a navy sidebar, a gradient-clipped "Kinkit" wordmark, a green-gradient "Add" CTA, a green-gradient FAB with a coloured glow, and a gradient gamification bar. | `.sidebar`, `.brand`, `.nav-cta`, `.fab`, `.gami-badge .progress-fill`, `.tv-brand` | These are the classic "AI slop" tells. The green Add also broke the colour logic, because green means income or good elsewhere in the app. |
| 3 | Coloured 4px side or top borders were the main way to show hierarchy. | `.card.stat-*`, `.goal-card`, `.budget-progress`, `.onboarding`, `.chore-approvals`, toasts | Every block shouted with the same weight. Colour bars repeated what the numbers already said. |
| 4 | Every table row had a filled blue Edit circle next to a filled red Delete circle. | `.icon-btn.round`, `.icon-btn.danger.round` (Spending, Goals, Integrations) | The loudest thing on the screen was the destructive action, repeated on every row. The targets were also only 24px. |
| 5 | The secondary button was a solid grey (#6b7280) and disabled buttons had no style. Hover was `opacity:.88` everywhere. | `button.secondary`, `.button-link.secondary` | Secondary looked like a disabled primary. There was no clear primary, secondary, ghost hierarchy. |
| 6 | Links relied on browser-default underlines. Inline links sat in the middle of sentences ("plan it on the Budget page"). | Dashboard, Budgeting, Goals, Reminders, Receipts, MealPlan, TVTokens, Spending header | These were hard to hit on touch and didn't match the button language. This was the user's explicit request. |
| 7 | Money was not tabular. It used proportional figures, was left-aligned in tables, and `.bar-value` was muted grey. | all tables, `.stat-value`, `.bar-value` | You couldn't compare amounts down a column, and the key numbers were low contrast. |
| 8 | `--muted` #6b7280 scored 4.8:1, and less on `--bg`. Inputs used `--border` #e5e7eb (1.2:1), so the fields almost disappeared. | `:root`, `input` | This is the low-contrast grey-on-grey problem. Form fields failed the WCAG 1.4.11 non-text contrast check. |
| 9 | Dark theme set `--primary:#818cf8` with white text (2.9:1). Badges, pills and the error box kept their light-theme tints. | `[data-theme=dark]`, `.badge-*`, `.chore-*`, `.error-box` | Primary buttons were unreadable in dark mode, and the tints looked like stickers. |
| 10 | The senior theme's `button {padding:.7rem 1.3rem}` also hit icon buttons, so the icons got squeezed. | `[data-theme=senior] button` | The accessibility theme broke its own controls. |
| 11 | Touch targets: icon buttons were 24–28px, the period stepper about 30px, and links had zero padding. | `.icon-btn`, `.period-step`, `.link-btn` | Phones are a primary surface for this app. |
| 12 | Empty states were bare: "No transactions.", "No goals yet. Create one…" written as an inline link. | Spending, Dashboard Week, Goals top card | They gave no next step. |
| 13 | Skeletons shimmered purple (`--primary-light`). The modal and popovers each had their own heavy shadow (.18/.25/.3). | `.skeleton`, `.modal`, `.period-picker`, `.toast` | The loading state looked branded, and the depth language was inconsistent. |
| 14 | There was no type hierarchy. h1 was 24px and h2 was 16.8px, but the stat values (24px/800) were bolder than the page title. Tables were 14.08px. | base | The eye had no obvious starting point. |

## The system (tokens in `index.css` `:root`)

- **Spacing:** `--s-1..6` = 4 / 8 / 12 / 16 / 24 / 32.
- **Radii:** `--r-sm` 6 for icon buttons, tags and link hover. `--r-md` 10 for buttons, inputs, rows and small cards. `--r-lg` 14 for cards, modals and popovers. `--r-pill` for chips and progress bars. The playful theme only redefines `--r-md` and `--r-lg`.
- **Elevation:** `--shadow-1` is for resting cards. `--shadow-2` is for floating layers only: modal, popover, toast and the list finish bar.
- **Type:** `--fs-xs/sm/md/lg/xl/2xl` = 12 / 14 / 16 / 20 / 24 / 32, in rem so the senior theme scales them.
  - A script mapped every old rem font size in `index.css`, `mealplan.css` and `reminders.css` onto this scale. TV-view sizes were left alone on purpose.
  - Weights are 500, 600 and 700 only. Nothing uses 800 any more.
- **Colour:**
  - Neutrals: `--bg`, `--card`, `--surface-2`, `--border`, `--border-strong`, `--text`, `--muted` (#5d6475, 5.9:1).
  - Brand: `--primary` and `--primary-hover`, plus `--on-primary`, which is dark text on the light indigo in dark mode.
  - Semantic: `--success`, `--danger`, `--danger-light`, `--warning` (darkened to #b45309 so it works as text).
- **Chrome:**
  - `--chrome-*` drives the sidebar and the mobile top bar.
  - Light: flat #4F46E5, with a white wordmark and #E0E7FF for secondary text. The active nav item is a white pill with purple text.
  - Dark: flat #312E81, with a translucent white active state.
  - Senior: flat #312E81, white text, and an active item outlined in white.
- **Numbers:** `td` and every money element use `font-variant-numeric: tabular-nums`. Add `.num` to a th/td to right-align it.
- **Focus:** inputs get a 3px `--focus-ring`. Buttons and links get a 2px outline on `:focus-visible`, in white inside the purple chrome.

## What changed

**Chrome**
- **Sidebar and mobile top bar (`.sidebar`, `.mobile-topbar`):** purple, solid white "Kinkit", and nav items in white text with 40px rows.
- **Nav states:** hover is `rgba(255,255,255,.12)`. Active is a white pill with purple text.
- **Logo tile:** has a hairline and a soft shadow so a white tile reads cleanly on purple.
- **Add CTA:** now a white button with purple text instead of the green gradient. The FAB is solid primary.
- **Gamification badge:** a translucent white card with a white progress bar and a trailing chevron, because it links to Family.

**Links (point 2 of the request)**
- **No underlines:** the global `a` rule removes them, `:hover` included.
- **New `ChevronLink.jsx`:**
  - `ChevronLink` is a compact text button with a trailing `IconChevronRight` that nudges right on hover.
  - `RowLink` makes a whole card-row clickable, with a chevron. It's available for future settings rows.
- **Converted to chevron links:**
  - Dashboard: the no-budget state.
  - Budgeting: "no chat connected" and the received-income hint.
  - Goals: "All goals" and an empty state.
  - Spending: the "Import / export" header link.
  - Reminders: "Connect a chat bot".
  - Receipts: "Download receipt".
  - MealPlan: "View promo".
  - TVTokens: "Open".
  - Guide: a new "go to" row under Tips.
- **Whole-row links with a chevron:** the Getting-started checklist steps (`OnboardingChecklist.jsx`) and the mobile "More" sheet links (`Layout.jsx`).
- **In-place text actions** (Undo, Pause, Remove, This month, Read amount) stay `.link-btn` with a tinted hover and no chevron. A chevron means "go somewhere", and these don't navigate.

**Components**
- **Buttons:**
  - The default is primary.
  - `.secondary` is an outlined surface. `.ghost` is a new class.
  - `.danger` is solid red. `.secondary.danger-text` is an outlined red for destructive actions; TV "Revoke" now uses it.
  - Disabled shows at 50% opacity with a not-allowed cursor. Hover is `brightness(.93)` instead of fading the opacity.
- **Icon buttons:** quiet, 32px squares (40px on touch and in senior). Delete only turns red on hover or focus.
- **Stat tiles:**
  - Each has an uppercase 12px label with a small tone square and a 24px/700 tabular value. There are no side borders.
  - Negative values are red, so the colour carries meaning.
  - `.cards-row` is now a grid instead of flex-wrap.
- **Removed decoration:** the side and top borders on goal cards, budget progress, onboarding and chore approvals. The toast side borders became a small status dot.
- **Tables:** 14px text, 12px uppercase headers, a `--surface-2` hover, and no border on the last row.
  - Money columns are right-aligned with `.num` in Dashboard (all tables), Spending, Income, Recurring and the Budgeting totals.
- **Tabs:** 2px underline. Hover is a neutral underline instead of a purple fill.
- **Segmented control:** an iOS-style white thumb instead of a solid purple fill.
- **Period stepper:** 32px buttons (40 on touch) with tabular labels.
- **Collapsible:** solid border instead of dashed.
- **Skeletons:** neutral shimmer.
- **Modal:** `--shadow-2`, a 20px title, and a 10vh top offset.
- **Forms:** inputs use `--border-strong` and a focus ring, and the placeholder colour is set.
- **Empty states:** `.empty-hint` has a bold first line, a 44ch measure and an action. Spending's empty row now says what to do and tells "no match" apart from "nothing yet".
- **Spending:** inline-edit Save is now primary, Cancel stays secondary, and pagination uses chevron icons.
- **Dark theme:** `--on-primary` fixes button contrast. Badges, chore pills, import badges, the error and notice boxes and danger buttons all have dark tints.
- **Senior theme:** the big button padding no longer hits icon, link, tab or sort buttons. Inputs and rows get 2px borders.
- **Touch and small screens:**
  - `@media (pointer: coarse)` sets a 40px minimum on buttons, inputs, selects, icon buttons and link buttons.
  - A ≤400px block tightens the gutters and hides the "Hide large" text label. Its select keeps an aria-label.
- **Guide copy:** "the green Add button" became "the Add button at the top of the sidebar", to match the new design.

## Remaining recommendations

1. ~~**Mascot and Login owners:** the login hero still used a gradient.~~ Done on 2026-11-20: the login hero, chrome and TV brand bars are flat, and Kiki's loops end within 5 seconds (see `anti-slop/report-001-2026-11-20.md`).
2. **Spacing tokens are partial:** they're applied to the shell, cards, tables, buttons, forms, chrome and the shared components. Feature blocks (`.list-*`, `.chore-*`, `.meal-*`, `.rem-*`) still use rem paddings that sit near the scale. Convert those as each feature is next touched.
3. **Dashboard density:** the Month tab stacks 4 stats, budget progress, a trend chart, 2 cards and a big table. Consider putting "Budget vs Actual by Category" behind the existing `Collapsible`, as Year already does for patterns.
4. **Charts:** `IncomeExpenseChart` hardcodes `#86efac` for planned income, and `seriesColor` and `categoryColor` aren't checked for contrast in dark mode. Move the chart colours into tokens.
5. **Inline styles:** about 60 `style={{}}` remain, mostly layout in `Dashboard.jsx`, `Setup.jsx` and `Integrations.jsx`. Move them to classes.
6. **Category tags:** `categoryColor + '22'` tag tints give arbitrary rainbow chips in Spending and Recent transactions. A neutral tag with a coloured dot (`.cat-dot`) would calm the list.
7. **Page-header pattern:** `PageHeader` is consistent, but pages inside tabs hide the h1 and keep the subtitle. Check that every tab page's subtitle is a single short line.
8. **Bundle size:** the JS bundle is 668 kB. Lazy-load the admin, import and chart-heavy pages.
