# Kinkit landing page

The static marketing site for Kinkit. It is plain HTML, CSS and JS, with no build step, no dependencies and no external images.
The page loads one Google Font, **DM Serif Display**, for headlines (`fonts.googleapis.com`, `display=swap`); if it is blocked the
headlines fall back to Georgia/serif. Body text uses the system sans stack.

Brand palette: Persian Blue `#16284C` (nav, hero, final CTA, footer), Cream `#EFE8DA` (body sections; cards `#FBF8F1`,
borders `#DDD3C0`) and Amber `#FFB92E` (highlights, primary CTA with navy text, icons, tags). Dark mode uses `#0D1830`
backgrounds with `#16284C` cards. Amber is never used as text on cream; amber-family text there uses `#8A5A00` for AA contrast.

```
landing/
├── index.html          # all sections; English copy lives here
├── styles.css          # layout, light/dark via prefers-color-scheme, reduced motion
├── main.js             # app links (APP_URL), menu, scroll reveal, FAQ, EN/ID toggle + Indonesian strings
└── assets/
    ├── mockups.css     # CSS-built phone / laptop / TV device mockups of the dashboard
    ├── favicon.svg     # same artwork as the inline data-URI favicon in index.html
    └── og-image.svg    # 1200x630 social card
```

## Preview

```sh
cd landing
python3 -m http.server 8080
# open http://localhost:8080
```

You can also open `index.html` directly from disk, because the icons are an inline sprite.

Language: English is the default, matching the app UI. Use the EN/ID toggle, or open `?lang=id`. The visitor's choice is remembered in `localStorage`.

## Configure

All "Start free" and "Sign in" links get their URL from a single constant at the top of `main.js`:

```js
var APP_URL = '';                                   // '' = same origin; or 'https://app.kinkit.app'
var APP_PATHS = { signup: '/signup', login: '/login' };
```

> The React app currently shows sign-up as a toggle on `/login`, and there is no `/signup` route (unknown paths redirect to `/`, then to `/login`). Either add a `/signup` route that opens the form in sign-up mode, or set `APP_PATHS.signup = '/login'`.

Also update the `canonical`, `og:url` and `og:image` URLs in `index.html` (currently `https://kinkit.app/`). Some social networks ignore SVG `og:image`. If previews are blank, export `assets/og-image.svg` to a PNG.

**Pricing is a placeholder.** The tier names, prices and feature split are marked with an HTML comment in the `#pricing` section. The app does not gate any feature by plan today. When you change the English copy, also edit the `price.*` keys in `main.js`.

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

When you add copy, give the element a `data-i18n="some.key"` (or `data-i18n-aria` for an `aria-label`) and add `'some.key'` to the `ID` dictionary in `main.js`.

## Feature claims → source of truth

The page should only claim what the code does. When a feature changes, find its row here, check the source and update the copy in **both** `index.html` and the `ID` strings in `main.js`.

| Claim on the page | Where it's implemented |
|---|---|
| Quick add from any page; amount shorthand `25k`, `2rb`, `1.5jt`; smart category memory; recent chips | `react/src/QuickAddSpending.jsx`, `react/src/SpendingForm.jsx`, `react/src/MoneyInput.jsx`; described in `react/src/pages/Guide.jsx` ("Add Spending") |
| Budgets per category grouped Needs / Wants / Savings; budget vs actual tint (green, amber ≥80%, red over) | `react/src/pages/Budgeting.jsx`, `backend-go/budgeting.go` |
| Budget alerts at 80% and >100% via Telegram/Discord | `backend-go/alerts.go`, toggle card in `react/src/pages/Budgeting.jsx` |
| Income received vs planned (month / year / across years) | `react/src/pages/Income.jsx`, `backend-go/income.go`, `react/src/pages/Dashboard.jsx` |
| Recurring payments (monthly / weekly / yearly), auto-added, pause/resume | `react/src/pages/Recurring.jsx`, `backend-go/recurring.go` |
| Savings goals with progress ring and per-month amount; assets and net-worth target | `react/src/pages/Goals.jsx`, `backend-go/goals.go`, `react/src/pages/Assets.jsx`, `backend-go/assets.go` |
| Receipt photos (private to household, ≤5 MB) and on-device "Read amount from receipt" | `react/src/Receipts.jsx`, `react/src/receiptOcr.js` (tesseract.js runs in the browser and is loaded from a CDN on first use), `backend-go/receipts.go` |
| Bank CSV import: BCA, Mandiri, generic; preview; no duplicates; undo import | `backend-go/csvimport.go`, `react/src/pages/ImportExport.jsx` |
| Dashboards: Week / Month / Year / Across Years; Custom tab with chosen charts, drag-to-reorder; "Hide large" | `react/src/pages/Dashboard.jsx`, `react/src/pages/CustomDashboard.jsx`, `backend-go/dashboard_layout.go`, `backend-go/reports.go` |
| Export to PDF | `react/src/pages/Dashboard.jsx` ("Export PDF" uses `window.print()` with print CSS in `react/src/index.css`) |
| Multi-currency display: IDR, USD, EUR, GBP, SGD, MYR, JPY, AUD, INR; daily rates; stored in IDR | `CURRENCY_OPTIONS` in `react/src/pages/Setup.jsx`, `backend-go/fx.go` |
| Household, members sign in with their own email | `react/src/pages/Family.jsx`, `backend-go/family.go`, `backend-go/auth.go` |
| Roles: Family Admin, Parent, Kid (own entries only), Viewer (read-only) | `backend-go/family.go`, role gating in `react/src/routes.jsx` |
| Kid monthly allowance with spent-this-month meter | `react/src/pages/Family.jsx` (`allowance_monthly`), `backend-go/family.go` |
| Undo instead of confirmation on delete | `react/src/Toast.jsx`; described in `Guide.jsx` ("Deleting and undo") |
| Starter categories / income sources on signup | `seedDefaults` in `backend-go/auth.go` |
| Today page (what's due for the household) | `react/src/pages/Today.jsx`, `backend-go/today.go` (`GET /api/today`) |
| Chores with points, approval, photo proof, allowance earnings | `react/src/pages/Chores.jsx`, `backend-go/chores*.go`, allowance in `backend-go/family.go` |
| Household reminders (cleaning/laundry/kitchen/groceries/home maintenance/vehicle/self-care plus bill/medicine/school/birthday), every N weeks/months, household templates, chat nudges | `react/src/pages/Reminders.jsx` (`REMINDER_TEMPLATES`), `backend-go/reminders*.go` (`reminderCategoryIDs`, `repeat_interval`) |
| Shared shopping lists, bulk add, "finish trip" → spending | `react/src/pages/Lists.jsx`, `backend-go/lists*.go` |
| Bot commands `/today` `/remind` `/buy` `/chores` | `bots/telegram_bot.py`, `bots/discord_bot.py`, `backend-go/*_bot.go` |
| Points (10/day + streak bonus) and levels Getting Started → Tracker → Budget Buddy → Money Master → Finance Wizard | `backend-go/gamification.go` (`badgeThresholds`), `react/src/GamificationBadge.jsx` |
| Badges: First Log, First Budget Set, Streak 7, Streak 30, Under Budget, Saver | `badgeCatalog` in `backend-go/achievements.go` |
| Weekly family challenges (log every day, no-spend weekend, Wants under a week's share) | `backend-go/achievements.go` |
| Monthly leaderboard | `react/src/pages/Family.jsx`, `backend-go/achievements.go` |
| Confetti celebration on new badges / level-ups (skipped with reduced motion) | `react/src/Celebration.jsx` |
| Themes: Light, Dark, Playful (kids), Senior (high-contrast) | `react/src/ThemeContext.jsx`, `[data-theme=…]` blocks in `react/src/index.css`, `react/src/pages/DisplaySettings.jsx` |
| Installable web app (PWA) | `react/index.html` (manifest), service worker registration in `react/src/main.jsx` |
| Works offline: queue, sync, rejected entries listed | `react/src/OfflineContext.jsx` |
| TV / car display links: no login, refresh every minute, revocable | `react/src/pages/TVView.jsx` (`REFRESH_MS = 60_000`), `react/src/TVTokens.jsx`, `backend-go/tvshare.go` |
| Telegram & Discord bots: "50k makan siang", keyword categories, `/recent` (`!recent` on Discord), bring your own bot | `bots/telegram_bot.py`, `bots/discord_bot.py`, `backend-go/bot.go`, `backend-go/integrations.go`, `react/src/pages/Integrations.jsx` |
| Export spending CSV (date range), income CSV, full JSON backup | `backend-go/importexport.go` (`/export/all.json` etc.), `react/src/pages/ImportExport.jsx` |
| Self-host with docker-compose (Postgres, app on :5010, Telegram & Discord bots) | `docker-compose.yml`, `backend-go/Dockerfile`, `bots/Dockerfile`, `.env.example` |
| Android phone app (APK download) | `mobile/` (Flutter); built APK served at `downloads/kinkit-android.apk` |
| Smart TV viewer: Android TV / Google TV / Fire TV APK, LG webOS IPK, Tizen project zip | `tv/flutter/`, `tv/web/`; served at `downloads/kinkit-tv-android.apk`, `kinkit-tv-webos.ipk`, `kinkit-tv-tizen-webos-source.zip` |
| **Coming soon:** native iPhone app | `mobile/ios` exists but needs a Mac/Xcode to build and sign |

The `downloads/` folder isn't in the repo; the deploy puts the built files next to `index.html`.
Each download card hides its size when its file is missing, but the button still shows, so only
deploy the page together with the files. Set `<meta name="kinkit-app-url">` to the app's base URL
when the landing page and the app are on different hosts.

Mockup numbers and names (e.g. "Rp 6,1jt", "The Wijaya family") are illustrative sample data, not real users.
