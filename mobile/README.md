# kinkit mobile (Flutter)

Native iOS / Android client for the kinkit family finance & home app. It talks
to the same Go backend (`backend-go/`) as the web app, using the same
session-cookie auth.

## Setup

Requirements: Flutter 3.27+ (Dart 3.5+), Android Studio / Android SDK for
Android, Xcode 15+ and CocoaPods for iOS.

```bash
cd mobile
# Generate any missing platform boilerplate (Xcode project, storyboards,
# Gradle wrapper, launcher icons). Existing files - including lib/ and the
# hand-written android/ and ios/ files - are NOT overwritten.
flutter create . --platforms=android,ios --org com.kinkit --project-name kinkit_mobile
flutter pub get
```

The repo contains a minimal hand-written `android/` and `ios/` scaffold
(Gradle Kotlin DSL files, manifest, `MainActivity.kt`, `Info.plist`,
`AppDelegate.swift`, `Podfile`). `flutter create .` fills in what is missing
(e.g. `ios/Runner.xcodeproj`, `Main.storyboard` / `LaunchScreen.storyboard`,
`Assets.xcassets`, `android/gradlew`) without touching what is there.

## Run

```bash
flutter run                                   # default server: http://10.0.2.2:5000 (Android emulator -> host)
flutter run --dart-define=KINKIT_BASE_URL=https://kinkit.example.com
flutter test                                  # unit tests (models, money formatting, parsing)
flutter analyze
# Verified: flutter analyze (no issues) + flutter test (29 passing, incl. a widget test that drives every tab against a fake backend) on Flutter 3.44 (ghcr.io/cirruslabs/flutter:stable).
```

Running the backend locally for development: `docker compose up` in the repo
root, or `go run .` in `backend-go/` with `APP_ENV=development`.

## Build

```bash
flutter build apk --release --dart-define=KINKIT_BASE_URL=https://kinkit.example.com
flutter build appbundle --release --dart-define=KINKIT_BASE_URL=https://kinkit.example.com
flutter build ipa --release --dart-define=KINKIT_BASE_URL=https://kinkit.example.com
```

- Android release signing: create `android/key.properties` with
  `storeFile`, `storePassword`, `keyAlias`, `keyPassword`. Without it, release
  builds are signed with the debug key (OK for testing, not for Play).
- iOS: open `ios/Runner.xcworkspace` once to set your Team / bundle id
  (`PRODUCT_BUNDLE_IDENTIFIER`) before `flutter build ipa`.
- The launcher icon is a placeholder vector (`android/app/src/main/res/drawable/ic_launcher.xml`).
  Use `flutter_launcher_icons` for real icons on both platforms.

## Configuring the server URL

1. **Build-time default**: `--dart-define=KINKIT_BASE_URL=https://your-server`
   (falls back to `http://10.0.2.2:5000`).
2. **At runtime**: on the login screen tap *Server: ...*, enter the address
   you use in the browser (e.g. `https://kinkit.example.com` or
   `http://192.168.1.10:5000`) and *Save & test connection*. It is persisted
   (shared_preferences). Logged-in users can change it in *More > Settings*.

The app normalises the input (adds `https://` when no scheme is given, strips
trailing `/` and `/api`).

Plain-http servers are allowed (`usesCleartextTraffic` on Android,
`NSAllowsArbitraryLoads` / `NSAllowsLocalNetworking` on iOS) because kinkit is
often self-hosted on a LAN. Remove those for https-only builds.

### Auth & cookies

- `POST /api/login` sets the `accountamily` session cookie (HttpOnly,
  SameSite=Lax, Max-Age 30 days). The app stores it in a `PersistCookieJar`
  (`cookie_jar` + `dio_cookie_manager`) in the app's private support
  directory, so you stay logged in across restarts. Logout clears the jar.
- SameSite does not affect native clients (there is no cross-site context).
  The backend has no CSRF / Origin check, so no backend change is needed.
- **Secure flag**: the backend marks the cookie `Secure` when
  `COOKIE_SECURE=true` or `APP_BASE_URL` starts with `https://`. A Secure
  cookie is never sent over http, so if the server is configured that way
  you must connect to it over **https** (the app shows a hint when login
  succeeds but the session is not kept).
- Login is rate-limited server-side (5 attempts / 5 min per IP + email).

## Architecture

```
lib/
  main.dart              bootstrap: SharedPreferences + PersistCookieJar, ProviderScope
  app.dart               MaterialApp, light/dark theme, AuthGate (login vs app)
  config.dart            --dart-define default server URL, URL normalisation
  theme.dart             palette from react/src/index.css (light + dark), ThemeData
  api/
    api_client.dart      KinkitApi (dio): typed methods for every endpoint used, ApiException
    models.dart          lenient JSON models mirroring backend-go structs
  state/
    providers.dart       Riverpod: prefs, base URL, theme mode, API client, auth, money formatter, selected month
    data_providers.dart  Riverpod FutureProviders per feature + invalidation helpers
  screens/               login, home shell (bottom nav), today, household (reminders/chores/lists),
                         list detail, money (dashboard/spending/income/budget/goals tabs),
                         family, more, settings
  widgets/               async/empty/error states, month picker, stat cards, amount field,
                         quick-add sheet, gamification badge, app bar
  util/                  money formatting & amount parsing, dates
test/                    model parsing, money/date/URL tests, boot + full-shell widget tests
```

State management: **Riverpod** (`flutter_riverpod`). Feature data uses
`FutureProvider.autoDispose(.family)`, so pull-to-refresh is
`ref.refresh(provider.future)` and writes call `invalidateSpending(ref)` /
`invalidateIncome(ref)` (the native equivalent of the web's
`notifySpendingChanged()`).

Navigation mirrors the web nav (`react/src/routes.jsx`): bottom bar
**Today / Home / Money / Family / More**. Home has Reminders, Chores and
Lists tabs; Money has Dashboard, Spending, Income, Budget and Goals tabs;
More holds profile, settings and logout. Menus are hidden by role the same way the web
does it (kids don't see income/budget/goals; viewers can't add; only
owners/parents edit budgets, goals and income). The backend still enforces
permissions.

### Money

Amounts are stored by the backend as **whole IDR integers** (no minor units).
Like the web's `formatRupiah`, the app converts IDR to the household currency
(`currency` from `/api/me`) with the cached FX table from `/api/fx-rates` and
prints `symbol + grouped digits` using that currency's locale via `intl`
(`Rp1.250.000`, `$75`, `€1.234`). Unknown rates fall back to showing IDR.
Amount inputs are entered in IDR (with web-style shorthand: `25k`, `2rb`,
`1.5jt`).

## Feature / API endpoint map

| Feature | Screen | Endpoints |
|---|---|---|
| Log in / sign up / log out | Login, More | `POST /api/login` `{username, password}`, `POST /api/signup` `{email, password, display_name}`, `POST /api/logout`, `GET /api/me` |
| Server check | Login | `GET /api/ping` |
| Currency formatting | all | `GET /api/me` (currency), `GET /api/fx-rates` |
| Categories / income sources | Quick add, Budget | `GET /api/setup` |
| Today | Today | `GET /api/today` (greeting + sections; item actions call the reminder/chore endpoints below), `GET /api/spending?year&month&per_page`, `GET /api/report/{y}/{m}`, `GET /api/gamification` |
| Dashboard (month summary) | Money > Dashboard | `GET /api/report/{y}/{m}`, `GET /api/report/month-compare/{y}/{m}`, `GET /api/spending?year&month&page&per_page` |
| Spending list / delete / check | Money > Spending | `GET /api/spending`, `DELETE /api/spending/{id}`, `PATCH /api/spending/{id}` `{checked}` |
| Quick add spending | FAB sheet | `POST /api/spending` `{tx_date, description, category, amount, source: "mobile", client_id}` |
| Income list / add / delete | Money > Income, FAB sheet | `GET /api/income?year&month`, `POST /api/income` `{tx_date, amount, source_id, description, client_id}`, `DELETE /api/income/{id}` |
| Budget (read + edit) | Money > Budget | `GET /api/budgeting/{year}`, `GET /api/budgeting/{year}/actuals`, `POST /api/budgeting/income` `{year, month, source_id, amount}`, `POST /api/budgeting/expense` `{year, month, category_id, amount}` |
| Goals + contribute | Money > Goals | `GET /api/goals`, `POST /api/goals`, `POST /api/goals/{id}/contribute` `{amount}` (negative = withdraw), `DELETE /api/goals/{id}` |
| Family members | Family | `GET /api/family` |
| Points / streak / badges | App bar chip, Today, Family | `GET /api/gamification` |
| Reminders | Home > Reminders | `GET /api/reminders/upcoming?days=7`, `GET /api/reminders`, `PATCH /api/reminders/{id}` `{active}`, `DELETE /api/reminders/{id}`, `POST /api/reminders/occurrences/{id}/done`, `POST /api/reminders/occurrences/{id}/snooze` `{minutes}`, `POST /api/reminders` `{title, kind, start_date, time_of_day, category, notes, weekdays?, day_of_month?}` |
| Chores | Home > Chores | `GET /api/chores/today`, `POST /api/chores/{instanceId}/complete` / `approve` / `reject` / `undo` `{note}` |
| Shopping lists | Home > Lists | `GET /api/lists`, `POST /api/lists` `{name, emoji}`, `GET /api/lists/{id}`, `POST /api/lists/{id}/items` `{text}`, `PATCH /api/lists/{id}/items/{itemId}` `{checked}`, `DELETE /api/lists/{id}/items/{itemId}`, `POST /api/lists/{id}/clear-checked`, `POST /api/lists/{id}/finish` `{client_id, total_amount}` (finish shopping -> spending) |

### Web-only for now (intentionally not on the phone yet)

- Creating/editing chore definitions and schedules, chore photo proof
  (chores that require a photo are shown with a camera hint), staples,
  list reordering, item suggestions,
  editing a reminder's schedule (pause/resume/delete are supported) and chat
  linking, household time zone / digest.
- Editing spending rows, receipts, recurring, assets, categories, member
  management, imports and bots.
