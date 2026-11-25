# Kinkit: project guide for Claude

Kinkit is a family money + home app (budgets, spending, income, goals, assets, chores, lists,
reminders, family calendar, meal plan + requests, widget screens, family roles/allowances, gamification). One Go backend serves several clients.

## Repository layout

| Path | What | Stack |
|---|---|---|
| `backend-go/` | REST API under `/api`, also serves the built React SPA | Go 1.26, chi, pgx (Postgres), gorilla/sessions |
| `react/` | Main web app (source of truth for features and UX) | React 18 + Vite, Chart.js |
| `mobile/` | Phone app for iOS + Android (full read/write client) | Flutter / Dart |
| `tv/flutter/` | Smart-TV viewer for Android TV / Google TV / Fire TV, **view only** | Flutter / Dart |
| `tv/web/` | Same TV viewer for Samsung Tizen / LG webOS / TV browsers, **view only** | Plain ES5 HTML/CSS/JS |
| `landing/` | Marketing landing page (sells + explains the product) | Static HTML/CSS/JS, English |
| `bots/` | Telegram, Discord, Slack and Matrix bots (rumahbot was merged in: spending log, keyword learning, meal plan, must-use ingredients) | Python |
| `docker-compose.yml` | postgres + app + bots | |

## Cross-surface rule (most important)

**Any change to a feature, API, or data shape must be applied to every surface it touches in the
same change.** Before finishing a task, walk this checklist:

1. **Backend** (`backend-go/`): route, handler, schema (`schema.sql`), tests (`*_test.go`).
2. **Web** (`react/src/api.js` + the page in `react/src/pages/`).
3. **Phone app** (`mobile/lib/api/` client + models, `mobile/lib/screens/`). If the web gains or
   changes a user-facing feature, the phone app gets it too. Update `mobile/README.md`'s
   feature ↔ endpoint table.
4. **TV viewer** (`tv/flutter/lib/src/` **and** `tv/web/app.js`, keep the two in sync). Only
   relevant when the change affects what's shown on the TV, i.e. `GET /api/tv/data`
   (`backend-go/tvshare.go`), `/api/fx-rates`, or the web's `TVView.jsx`. The TV apps must stay
   read-only: token auth via `?k=`, never login, never write calls. Update the field list in
   `tv/README.md`.
5. **Landing page** (`landing/index.html`, plus `landing/privacy.html` when data handling changes). Update it
   when a feature is added, removed, renamed, or leaves "Coming soon". Only claim features that
   exist in the code; keep the "Feature claims → source of truth" table in `landing/README.md`
   accurate.
6. **Bots** (`bots/`), if the change affects chat commands or the `/api/bot/*` / internal routes.
7. **In-app docs** (`react/src/docs/pages/*.jsx`, the docs site at `/docs`) for user-visible behaviour changes.

If a surface intentionally doesn't get a change (e.g. an admin-only feature isn't put on the phone
app), say so explicitly in your summary instead of skipping it silently.

## UI work

Read `DESIGN.md` first: it holds the identity, the dials (ENERGY / RHYTHM / MOTION) and the "never" list for every surface. Treat the antislop rules as the filter on top, and keep landing claims in step with `landing/README.md`.

## Commands

```sh
# Backend
cd backend-go && go build ./... && go test ./...
# Web
cd react && npm install && npm run dev      # build: npm run build (output react/dist, copied into the Go image)
# Full stack
docker compose up -d --build                # app on :5010
# Phone app (Flutter not installed on this host; use a Flutter SDK or the ghcr.io/cirruslabs/flutter image)
cd mobile && flutter pub get && flutter analyze && flutter test
# TV app
cd tv/flutter && flutter pub get && flutter analyze && flutter test
cd tv/web && python3 -m http.server 8081
# Landing
cd landing && python3 -m http.server 8080
```

If `android/`/`ios/` boilerplate is missing (gradle wrapper, Xcode project), run
`flutter create . --platforms=android,ios` (mobile) or `flutter create . --platforms=android` (tv).
This fills in the missing files and leaves `lib/` alone.

## Conventions

- Money amounts are integers. Format them with the user's currency (`/api/me`); FX rates come from `/api/fx-rates`.
- Auth: session cookie for web and phone; `X-Bot-Token` / `X-Service-Token` for bots and internal calls;
  TV share tokens (`/api/tv-tokens`) for the TV viewers.
- Logo: `brand/kinkit-logo.svg` (white tile with hairline #E0E7FF border, purple #4F46E5 house) is the single source. Web favicons/PWA icons,
  the phone launcher icon, TV launcher/banner and tv/web icons, and the landing favicon/nav logo are all
  renders of it; change it there first and re-export everywhere.
- Dark mode text and focus primary is #818CF8 on every surface; filled controls keep #4F46E5 with white text.
- Keep visual identity consistent: the palette lives in `react/src/index.css`. Mirror changes in
  `mobile/lib/theme.dart`, `tv/flutter/lib/src/theme.dart`, `tv/web/styles.css`, and `landing/styles.css`.
- The web UI, phone app, bots and landing page copy are English only (no language toggle). Parsers still
  accept Indonesian input such as 25rb, 1,5jt and reminder phrases; don't advertise it in copy.
- Navigation has two modes on web and phone: the sidebar (tabs on the phone) and game mode, a town map
  you walk with arrow keys or a D-pad (`react/src/game/`, `mobile/lib/game/`). A new page needs a sidebar
  entry and, if it should be reachable in game mode, a building on the map. The TV viewers have neither.
- Don't commit secrets (`.env`). Use `.env.example` for new config keys.

## Known gaps / TODO

- `/api/tv/data` has no FX rate and no "spent" total; the TV apps compute or fetch these.
- The canonical domain `https://kinkit.app/` is assumed (landing canonical link).
- The privacy contact address and a Terms page await the owner (`landing/privacy.html` says so openly).
