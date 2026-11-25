# Kinkit landing page

The static marketing site for Kinkit. It is plain HTML, CSS and JS, with no build step, no dependencies and no external images or fonts.

```
landing/
├── index.html          # all sections; the copy is English only and lives here
├── privacy.html        # plain description of what the app does with data (facts from the code)
├── styles.css          # layout, light/dark via prefers-color-scheme, reduced motion
├── main.js             # app links (APP_URL), menu, scroll reveal, Kiki motion, FAQ (no copy lives here)
└── assets/
    ├── mockups.css     # CSS-built phone / laptop / TV mockups of the web app (white sidebar with a purple current-page marker, calm cards)
    ├── favicon.svg     # canonical logo (copy of brand/kinkit-logo.svg); same artwork as the inline favicon and #logo sprite in index.html
    └── og-image.svg    # 1200x630 social card
```

## Design

Direction lives in the repo-root `DESIGN.md` (identity, dials, motif). The page follows the web app's design system (`react/DESIGN_AUDIT.md`): spacing 4/8/12/16/24/32
(plus 48/64/96 for section rhythm), radii 6/10/14, two shadows, type 12/14/16/20/24/32 with a 48px
hero display size, purple only for primary actions and key accents, no gradients or glows. The top bar
is flat indigo with the white logo tile and white wordmark, like the phone app bar and the TV brand bar. The web app's own sidebar
is white, so the laptop mockup draws a white sidebar with a purple bar on the current page. Links are never
underlined; navigation in copy uses a `.link-chev` text button with the `#i-chevron` icon.

**Kiki (mascot)** appears three times, on purpose: waving in the hero (`mascot.svg`), celebrating next
to "Fun & motivation" (`mascot-celebrate.svg`) and relaxing in the final call to action
(`mascot-relax.svg`). The SVG bodies are copied verbatim from `brand/`; their shared gradients and
filters (`kk-*` ids) live once in the inline sprite. The motion CSS at the end of `styles.css` mirrors
`react/src/mascot.css` and only runs under `prefers-reduced-motion: no-preference`, and every loop ends within 5 seconds; `main.js` starts
the celebrate/relax motion when they scroll into view. If the art in `brand/` changes, re-copy it.

Bump the `?v=` query on `styles.css`, `assets/mockups.css` and `main.js` in `index.html` whenever
they change, because the CDN caches static files.

## Preview

```sh
cd landing
python3 -m http.server 8080
# open http://localhost:8080
```

You can also open `index.html` directly from disk, because the icons are an inline sprite.

The page is English only, like the app UI. There is no language toggle, and `main.js` holds no copy.

## Configure

All "Start free", "Sign in" and "Docs" links (on both pages) get their URL from a single constant at the top of `main.js`:

```js
var APP_URL = '';                                   // '' = same origin; or 'https://app.kinkit.app'
var APP_PATHS = { signup: '/login', login: '/login', docs: '/docs' };
```

> The React app currently shows sign-up as a toggle on `/login`, and there is no `/signup` route (unknown paths redirect to `/`, then to `/login`). Either add a `/signup` route that opens the form in sign-up mode, or set `APP_PATHS.signup = '/login'`.

Also update the `canonical`, `og:url` and `og:image` URLs in `index.html` (currently `https://kinkit.app/`). Some social networks ignore SVG `og:image`. If previews are blank, export `assets/og-image.svg` to a PNG.

**There is no pricing section.** The app gates no feature by plan, so the page only says "Free to sign up". Add a pricing section back only once real plans exist in the code.

**Privacy page:** `privacy.html` must stay a description of what the code does. It still needs a real contact address for privacy questions and deletion requests (see the `TODO(owner)` comment); until then it says so openly. The page deliberately has no invented address. There is also no Terms page yet: add one only once the owner supplies real terms.

## Deploy

The site is just files, so any static host works.

- **Static hosts** (Netlify, Cloudflare Pages, GitHub Pages, S3 + CloudFront): publish the `landing/` folder as-is.
- **nginx**, with the landing page on the apex domain and the app on a subdomain:
  ```nginx
  server {
    server_name kinkit.app;
    root /srv/kinkit/landing;
    location / { try_files $uri $uri/ =404; }
  }
  server {
    server_name app.kinkit.app;
    location / { proxy_pass http://127.0.0.1:5010; proxy_set_header Host $host; }
  }
  ```
  Then set `APP_URL = 'https://app.kinkit.app'`.
- **Same host as the app**: serve the landing page under a path such as `/welcome/` (with nginx `location /welcome/ { alias /srv/kinkit/landing/; }`) and keep `APP_URL = ''`. The Go server's SPA handler (`backend-go/spa.go`) treats unknown paths as app routes, so if you want the Go binary itself to serve the page, add a dedicated route/`http.FileServer` for the landing directory *before* the SPA fallback.

## Checks

```sh
# HTML well-formedness (stdlib only)
python3 - <<'EOF'
from html.parser import HTMLParser
VOID={'area','base','br','col','embed','hr','img','input','link','meta','source','track','wbr','use','path','circle','rect','stop'}
class P(HTMLParser):
    def __init__(s): super().__init__(); s.st=[]
    def handle_starttag(s,t,a):
        if t not in VOID: s.st.append(t)
    def handle_endtag(s,t):
        if t in VOID: return
        assert s.st and s.st.pop()==t, f"mismatch at {s.getpos()}"
p=P(); p.feed(open('landing/index.html').read()); assert not p.st, p.st; print('HTML OK')
EOF
node --check landing/main.js   # if Node is installed
```

When you add copy, write it in the HTML in English. Don't advertise Indonesian input in copy (the parsers still accept it; see `CLAUDE.md`).

## Feature claims → source of truth

The page should only claim what the code does. When a feature changes, find its row here, check the source and update the copy in `index.html` (and `privacy.html` where it is mentioned).

| Claim on the page | Where it's implemented |
|---|---|
| Quick add from any page; amount shorthand `25k` and `1.5m`; smart category memory; recent chips | `react/src/QuickAddSpending.jsx`, `react/src/SpendingForm.jsx`, `react/src/MoneyInput.jsx`; described in `react/src/docs/pages/add-spending.jsx` |
| Budgets per category grouped Needs / Wants / Savings; budget vs actual tint (green, amber ≥80%, red over) | `react/src/pages/Budgeting.jsx`, `backend-go/budgeting.go` |
| Budget alerts at 80% and >100% via Telegram/Discord/Slack/Matrix, sent by a check every few minutes; the "See it work" bubble quotes `alertText` verbatim | `backend-go/alerts.go`, `backend-go/money.go` (worker), toggle card in `react/src/pages/Budgeting.jsx` |
| Income received vs planned (month / year / across years) | `react/src/pages/Income.jsx`, `backend-go/income.go`, `react/src/pages/Dashboard.jsx` |
| Recurring payments (monthly / weekly / yearly), auto-added, pause/resume | `react/src/pages/Recurring.jsx`, `backend-go/recurring.go` |
| Savings goals with progress ring and per-month amount; assets and net-worth target | `react/src/pages/Goals.jsx`, `backend-go/goals.go`, `react/src/pages/Assets.jsx`, `backend-go/assets.go` |
| Receipt photos (private to household, ≤5 MB) and on-device "Read amount from receipt" | `react/src/Receipts.jsx`, `react/src/receiptOcr.js` (tesseract.js runs in the browser and is loaded from a CDN on first use), `backend-go/receipts.go` |
| Bank CSV import: BCA, Mandiri, generic; preview; no duplicates; undo import | `backend-go/csvimport.go`, `react/src/pages/ImportExport.jsx` |
| Dashboards: Week / Month / Year / Across Years; Custom tab with chosen charts, drag-to-reorder; "Hide large" | `react/src/pages/Dashboard.jsx`, `react/src/pages/CustomDashboard.jsx`, `backend-go/dashboard_layout.go`, `backend-go/reports.go` |
| Export to PDF | `react/src/pages/Dashboard.jsx` ("Export PDF" uses `window.print()` with print CSS in `react/src/index.css`) |
| Multi-currency display: IDR, USD, EUR, GBP, SGD, MYR, JPY, AUD, INR; daily rates; stored in IDR | `CURRENCY_OPTIONS` in `react/src/pages/Setup.jsx`, `backend-go/fx.go` |
| Household, members sign in with their own email; the family admin or a parent adds, edits and removes members under Settings › Members | `react/src/pages/Members.jsx`, `mobile/lib/screens/settings_screen.dart` (read-only list), `backend-go/family.go`, `backend-go/auth.go` |
| Roles: Family Admin, Parent, Kid (own entries only), Viewer (read-only) | `backend-go/family.go`, role gating in `react/src/routes.jsx` |
| Kid monthly allowance with spent-this-month meter (set under Settings › Members) | `react/src/pages/Members.jsx` (`allowance_monthly`), `mobile/lib/screens/settings_screen.dart`, `backend-go/family.go` |
| Undo for everyday deletes; big removals (a whole list, an import, a widget board, a TV link, a receipt, a repeating activity) still ask first | `react/src/Toast.jsx`; `window.confirm` in `Lists.jsx`, `ImportExport.jsx`, `Widgets.jsx`, `TVTokens.jsx`, `Receipts.jsx`, `Calendar.jsx` |
| Starter categories / income sources on signup | `seedDefaults` in `backend-go/auth.go` |
| Today page (what's due for the household) | `react/src/pages/Today.jsx`, `backend-go/today.go` (`GET /api/today`) |
| Chores with points, approval, photo proof, allowance earnings | `react/src/pages/Chores.jsx`, `backend-go/chores*.go`, allowance in `backend-go/family.go` |
| Family calendar: month/week/day/agenda, repeating activities, per-person schedules, reminders overlay; Reminders calendar view | `react/src/pages/Calendar.jsx`, `react/src/RemindersCalendar.jsx`, `backend-go/activities.go` (`GET /api/calendar`, `/api/activities`) |
| Meal requests: anyone proposes, parents/admin approve on a day or decline | `react/src/MealRequests.jsx`, `backend-go/meal_proposals.go` |
| Widget screens: pick widgets, colour each widget, drag to arrange, full screen for phone/TV/car/monitor, templates; TV links can show a board | `react/src/pages/Widgets.jsx`, `react/src/WidgetBoard.jsx`, `react/src/widgetColors.js`, `react/src/ColorPicker.jsx`, `backend-go/widgets.go`, `backend-go/tvshare.go` (`board_id`) |
| Household reminders (cleaning/laundry/kitchen/groceries/home maintenance/vehicle/self-care plus bill/medicine/school/birthday), every N weeks/months, household templates, chat nudges | `react/src/pages/Reminders.jsx` (`REMINDER_TEMPLATES`), `backend-go/reminders*.go` (`reminderCategoryIDs`, `repeat_interval`) |
| Shared shopping lists, bulk add, "finish trip" → spending | `react/src/pages/Lists.jsx`, `backend-go/lists*.go` |
| Bot commands `/today` `/remind` `/buy` `/chores` `/menu` | `bots/telegram_bot.py`, `bots/discord_bot.py`, `bots/slack_bot.py`, `bots/matrix_bot.py`, `bots/kinkit_common.py`, `backend-go/*_bot.go`, `backend-go/meal_plan.go` |
| "See it work" bot reply ("Logged: lunch, Rp25.000 (Eating out)") | `log_spending` / `format_rp` in `bots/kinkit_common.py` |
| Bot category buttons for new words, learned keywords | `bots/kinkit_common.py` (`log_spending`, `spc:` callbacks), `backend-go/bot_extras.go` (`/api/bot/keywords/learn`) |
| Points (10/day + streak bonus) and Levels 1 to 50 with titles from Getting Started to Legend (Budget Buddy, Money Master, Finance Wizard on the way) | `backend-go/gamification.go` (`maxLevel`, `levelThreshold`, `levelTitles`, `levelTitle`), `react/src/GamificationBadge.jsx` |
| 20+ badges in bronze / silver / gold (Financial Geek, Number Cruncher, Goal Crusher, Streak 30, Chore champion, Chef's Choice, ...) | `badgeRules` / `badgeCatalog` in `backend-go/achievements.go` (count them before changing "20+") |
| Weekly family challenges (log every day, no-spend weekend, Wants under a week's share) | `backend-go/achievements.go` |
| Leaderboard (points this month; page named Leaderboard in the web nav and the phone More tab) | `react/src/pages/Family.jsx`, `react/src/routes.jsx`, `mobile/lib/screens/family_screen.dart`, `backend-go/achievements.go` |
| Game mode: walk Kiki through a small town, each building opens a page; arrow keys or an on-screen pad; switch in Display settings (web) or Navigation (phone); web and phone only, not the TV viewers | `react/src/game/` (`navMode.js`, `worldMap.js`, `GameWorld.jsx`), `react/src/pages/DisplaySettings.jsx`, `mobile/lib/game/` (`nav_mode.dart`, `town_map_screen.dart`), `mobile/lib/screens/settings_screen.dart` |
| Expenses from Android notifications: off by default, Android only, the user reviews each one before saving, notification text stays on the phone (privacy page paragraph says the same) | `mobile/lib/capture/` (`notification_parser.dart`, `capture_channel.dart`), `mobile/lib/screens/captured_expenses_screen.dart`, `mobile/android/app/src/main/kotlin/com/kinkit/mobile/ExpenseNotificationListener.kt`, saved through `POST /api/spending` with `source: "notification"` (`backend-go/spending.go`) |
| Confetti celebration on new badges / level-ups (skipped with reduced motion) | `react/src/Celebration.jsx` |
| Themes: Light, Dark (same design, white swapped for black; sidebar toggle flips only light/dark), Playful (kids), Senior (high-contrast) | `react/src/ThemeContext.jsx`, `[data-theme=…]` blocks in `react/src/index.css`, `react/src/pages/DisplaySettings.jsx` |
| Installable web app (PWA) | `react/index.html` (manifest), service worker registration in `react/src/main.jsx` |
| Works offline: queue, sync, rejected entries listed | `react/src/OfflineContext.jsx` |
| TV / car display links: no login, refresh every minute, revocable | `react/src/pages/TVView.jsx` (`REFRESH_MS = 60_000`), `react/src/TVTokens.jsx`, `backend-go/tvshare.go` |
| Telegram, Discord, Slack & Matrix bots: "50k lunch", keyword categories, `/recent` (`!recent` on Discord), bring your own bot (Matrix also needs a homeserver URL); budget alerts and reminders go to all four | `bots/telegram_bot.py`, `bots/discord_bot.py`, `bots/slack_bot.py`, `bots/matrix_bot.py`, `backend-go/bot.go`, `backend-go/integrations.go`, `backend-go/chat_slack_matrix.go`, `react/src/pages/Integrations.jsx` |
| Privacy page facts: hashed passwords, no analytics, receipts on the server, tesseract.js from jsDelivr, rates from open.er-api.com, one session cookie, no self-serve account delete | `backend-go/auth.go`, `backend-go/receipts.go`, `react/src/receiptOcr.js`, `backend-go/fx.go`, `backend-go/admin.go` |
| Export spending (date range) and income as CSV or Excel, full backup as JSON or Excel (one sheet per table) | `backend-go/importexport.go` (`/export/spending.csv`, `/export/spending.xlsx`, `/export/income.xlsx`, `/export/all.json`, `/export/all.xlsx`), `backend-go/xlsx.go`, `react/src/pages/ImportExport.jsx` |
| Docs link (nav + footer) opens the in-app documentation at `<app-url>/docs` (relative `/docs` when `kinkit-app-url` is empty) | `react/src/docs`, route wired in `react/src/routes.jsx`; link built by `APP_PATHS.docs` in `main.js` |
| Role-based access (kids edit only their own entries, viewers read-only) shown under "Your data" | `backend-go/family.go`, role gating in `react/src/routes.jsx` |
| Android phone app (APK download) | `mobile/` (Flutter); built APK served at `downloads/kinkit-android.apk` |
| Smart TV viewer: Android TV / Google TV / Fire TV APK, LG webOS IPK, Tizen project zip | `tv/flutter/`, `tv/web/`; served at `downloads/kinkit-tv-android.apk`, `kinkit-tv-webos.ipk`, `kinkit-tv-tizen-webos-source.zip` |
| **Coming soon:** native iPhone app | `mobile/ios` exists but needs a Mac/Xcode to build and sign |

The `downloads/` folder isn't in the repo; the deploy puts the built files next to `index.html`.
Each download card hides its size when its file is missing, but the button still shows, so only
deploy the page together with the files. Set `<meta name="kinkit-app-url">` to the app's base URL
when the landing page and the app are on different hosts.

Mockup numbers and names (e.g. "Rp 6.1M", "The Carter family": Sarah, Tom, Leo, Mia and Grandma Ruth) are illustrative sample data, not real users. The hero shows a visible caption under the laptop (`.mock-note` in `index.html`: "Sample data: the Carter family and their numbers are made up."). Keep it whenever the hero mockup keeps made-up numbers, and keep the sample household in step with the docs screenshots (`react/public/guide/`).
