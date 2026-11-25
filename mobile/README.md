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
# Gradle wrapper, launcher icons). Existing files, including lib/ and the
# hand-written android/ and ios/ files, are NOT overwritten.
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
# Verified: flutter analyze (no issues) + flutter test (124 passing, incl. a widget test that drives every tab and the Settings members list and password dialog against a fake backend, a check that a wrong current password does not log the user out, notification parser cases, the captured-expenses review and failure flows and the town map tones) and `flutter build apk --debug` (compiles the Kotlin capture code) on Flutter 3.44 (ghcr.io/cirruslabs/flutter:stable). The listener itself has not been run on a device.
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
- Launcher icons follow `brand/kinkit-logo.svg`: Android uses an adaptive vector icon
  (`res/drawable-anydpi-v26/ic_launcher.xml` + `drawable-v26/ic_launcher_{background,foreground}.xml`)
  with a vector fallback in `res/drawable/ic_launcher.xml`; iOS uses the single-size 1024px
  `ios/Runner/Assets.xcassets/AppIcon.appiconset`. In-app the same logo is drawn by
  `lib/widgets/kinkit_logo.dart` (splash + login).

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
`NSAllowsArbitraryLoads` / `NSAllowsLocalNetworking` on iOS) because the app can
connect to a backend on a local network during development. Remove those for
https-only builds.

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
  screens/               login, home shell (bottom nav), today, reminders, chores, lists, meals, calendar (one screen each),
                         list detail, money (dashboard/spending/income/budget/goals tabs),
                         leaderboard (family_screen.dart), more, settings, captured expenses (Android)
  capture/               Android notification capture: notification_parser.dart (pure Dart), capture_channel.dart
                         (MethodChannel `com.kinkit.mobile/capture`), capture_providers.dart
  game/                  Town map navigation (nav_mode.dart, town_map.dart, town_map_screen.dart)
  widgets/               async/empty/error states, month picker, stat cards, amount field,
                         quick-add sheet, gamification badge, app bar, chevron link, Kiki mascot
assets/mascot/           Kiki PNGs (brand/mascot*.svg rendered at 512px without the baked-in
                         contact shadow; widgets/mascot.dart draws and animates the shadow)
  util/                  money formatting & amount parsing, dates
test/                    model parsing, money/date/URL tests, boot + full-shell widget tests
```

State management: **Riverpod** (`flutter_riverpod`). Feature data uses
`FutureProvider.autoDispose(.family)`, so pull-to-refresh is
`ref.refresh(provider.future)` and writes call `invalidateSpending(ref)` /
`invalidateIncome(ref)` (the native equivalent of the web's
`notifySpendingChanged()`).

Navigation mirrors the web nav (`react/src/routes.jsx`): bottom bar
**Today / Reminders / Chores / Money / More**. Reminders has Next 7 days, All and
Calendar segments; More has a *Family* group (Leaderboard, Chores, Lists, Meals, Widgets, Calendar), each
its own screen with its own app bar title; Money has Dashboard, Spending, Income, Budget and Goals tabs;
More holds profile, settings and logout. Menus are hidden by role the same way the web
does it (kids don't see income/budget/goals; viewers can't add; only
family admins/parents edit budgets, goals and income). The backend still enforces
permissions.

### Look and feel

Mirrors the web design system (`react/DESIGN_AUDIT.md`, tokens in `react/src/index.css`):
purple app bar (#4F46E5, dark theme #312E81) with the white logo tile, white title and the
family name in #E0E7FF, light status-bar icons; a white bottom bar with purple selection;
4/8/12/16/24/32 spacing, 6/10/14 radii (`KSpace`, `KRadius` in `theme.dart`); flat bordered
cards; tabular figures everywhere (`kTabular`); 40px+ touch targets; destructive actions in red.
Links are never underlined: "go somewhere" actions are rows or `ChevronLink` buttons with a
trailing chevron.

Kiki (`widgets/mascot.dart`) appears in a few places only: login (idle with a wave; "think" on
error), the Today header (small idle), "all clear" states (relax), errors and no-data states
(think) and reached goals (celebrate). Motion matches `react/src/mascot.css` (bob with
squash/stretch and a scaling shadow, wave wobble, celebrate hops) and stops when the OS asks
for reduced motion (`MediaQuery.disableAnimations`).

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
| Leaderboard (points, levels, badges) | More > Family > Leaderboard (web: `/family`) | `GET /api/gamification` |
| Members (read-only list) and change password | Settings > Members, Settings > Your account > Change password (web: `/settings/members`, `/settings/account`) | `GET /api/family`, `POST /api/me/password` `{current_password, new_password}` (a wrong current password is a 401 that the client shows inline instead of logging out) |
| Expense capture from notifications (Android only) | Settings > Expense capture, More > Captured expenses | Reads notifications on the phone (no endpoint). Each expense the user reviews and saves: `POST /api/spending` `{tx_date, description, category, amount, source: "notification", client_id: "notif-<hash>"}` |
| Town map (game-mode navigation) | Settings > Navigation > "Town map": the first bottom-bar tab is labelled "Town" (screen title "Town map") and replaces Today | Client-only, no endpoints (preference `kinkit.navMode`) |
| Points / streak / level / badges | App bar chip (`Lv N` + points + streak, thin progress to next level), Today + Leaderboard level card (hexagon, title, XP bar from `member.level_floor` to `next_level_at`, "N pts to Level X"), Leaderboard badges grouped by `category` (money / habits / home / levels) with `tier` medal colours (bronze / silver / gold), locked badges show `progress/target` | `GET /api/gamification` (`member.level_number` 1..50, `level_floor`, `next_level_at`; `badges[]` `{key,name,description,category,tier,target,progress,earned,earned_at}`; all fields optional so older servers still render) |
| Reminders (household categories, every N weeks/months, template gallery in the create sheet) | Reminders (bottom bar; web: `/reminders`) | `GET /api/reminders/upcoming?days=7`, `GET /api/reminders` (rows include `interval`), `PATCH /api/reminders/{id}` `{active}`, `DELETE /api/reminders/{id}`, `POST /api/reminders/occurrences/{id}/done`, `POST /api/reminders/occurrences/{id}/snooze` `{minutes}` or `{until: "tomorrow"}`, `POST /api/reminders` `{title, kind, start_date, time_of_day, category, notes, weekdays?, day_of_month?, interval, assignee_id?}`, `GET /api/family` (who-is-it-for picker) |
| Chores | Chores (bottom bar; More > Family > Chores; web: `/chores`) | `GET /api/chores/today`, `POST /api/chores/{instanceId}/complete` / `approve` / `reject` / `undo` `{note}` |
| Shopping lists | More > Family > Lists (web: `/lists`) | `GET /api/lists`, `POST /api/lists` `{name}` (emoji sent empty), `GET /api/lists/{id}`, `POST /api/lists/{id}/items` `{text}`, `PATCH /api/lists/{id}/items/{itemId}` `{checked}`, `DELETE /api/lists/{id}/items/{itemId}`, `POST /api/lists/{id}/clear-checked`, `POST /api/lists/{id}/finish` `{client_id, total_amount}` (finish shopping -> spending) |
| Meal plan + must-use ingredients | More > Family > Meals (web: `/meals`) | `GET /api/meal-plan` (`?week=this` for this week), `GET /api/meal-plan/ingredients`, `POST /api/meal-plan/ingredients` `{name}`, `DELETE /api/meal-plan/ingredients/{id}` |
| Calendar (activities + reminders overlay): month grid, selected-day list, 30-day agenda; owner/parent add, edit, delete (this one / series) | More > Family > Calendar (web: `/calendar`; the web also has week and day hour grids) | `GET /api/calendar?from&to[&include=activities\|reminders]`, `POST /api/activities` `{title, start_date, end_date, all_day, start_time, end_time, repeat, interval, weekdays, until_date, attendee_ids, location, notes, color}`, `PATCH /api/activities/{id}`, `DELETE /api/activities/{id}[?on=YYYY-MM-DD]`, `GET /api/family` (who-is-it-for picker) |
| Reminders as a month calendar | Reminders > Calendar segment | `GET /api/calendar?include=reminders`, then the reminder done/snooze endpoints above |
| Meal requests (anyone asks, owner/parent approve on a day or decline) | More > Family > Meals | `GET /api/meal-plan/proposals`, `POST /api/meal-plan/proposals` `{dish, note, wanted_date}`, `POST /api/meal-plan/proposals/{id}/approve` `{meal_date}`, `POST .../decline`, `DELETE /api/meal-plan/proposals/{id}`; approved dishes come back as `picks` on `GET /api/meal-plan` days |
| Widget boards (view, full screen, edit: reorder by long-press drag, resize, add, remove, colour each widget and the board background) | More > Family > Widgets (web: `/widgets`) | `GET /api/widgets/catalog`, `GET /api/widgets/boards`, `POST /api/widgets/boards` `{preset}` or `{name, layout}`, `PATCH /api/widgets/boards/{id}` `{name?, layout?}` (layout `{background?, widgets:[{..., color?}]}`, colours `#rrggbb`), `DELETE /api/widgets/boards/{id}`, `GET /api/widgets/data?types=` |

### Expense capture (Android only)

A hand-written Kotlin `NotificationListenerService` (`ExpenseNotificationListener.kt`) watches
posted notifications and keeps the ones that mention an amount of money in the app's private
`SharedPreferences` (`CaptureStore.kt`, at most 100, newest first), so nothing is lost while
the Flutter engine is not running. `MainActivity.kt` exposes the queue over the
`com.kinkit.mobile/capture` MethodChannel (`status`, `openSettings`, `setEnabled`, `pending`,
`remove`, `clear`). The Dart side parses each item (`lib/capture/notification_parser.dart`),
and the user reviews it in *More > Captured expenses* before it is saved. Nothing is ever
saved automatically.

- Off until the user switches it on in *Settings > Expense capture* **and** allows
  "Notification access" for Kinkit in Android settings. If Android calls the setting
  restricted (sideloaded APKs): App info > menu > Allow restricted settings.
- Notification text stays on the phone. Only an expense the user saves is sent, as a normal
  `POST /api/spending` with `source: "notification"` and `client_id: "notif-<hash>"`
  (retrying a save never duplicates it).
- One-time codes (OTP, "kode OTP / verifikasi", "one-time") and promotions or vouchers with no
  payment wording are deleted from the phone. A payment that merely mentions a promo still
  parses. A notification that has a currency amount the parser cannot read is kept and listed as
  "Amount unclear"; its review sheet opens with an empty amount. Money-in notifications can only
  be dismissed. A notification in another currency (SGD, MYR, USD, EUR, GBP) is shown with a
  warning and the amount field starts empty, because amounts are entered in IDR.
- If the Android side fails (status, list, dismiss, switch), the screens say so with a Retry or
  next step instead of showing an empty list or "Dismissed".
- Logging out switches capture off and deletes the queued notifications on this phone.
- `android:allowBackup="false"`, so the queue is never copied to cloud backup or a device transfer.
- iOS does not allow an app to read other apps' notifications, so capture is Android only and
  hidden on iPhone (every entry point checks `captureSupportedPlatform`).
- Adds no Flutter package and no `QUERY_ALL_PACKAGES` permission. The app name comes from the
  notification's own `android.appInfo`, then a PackageManager lookup, then the package name.

### Web-only for now (intentionally not on the phone yet)

- Exports (CSV, Excel, JSON backup: `GET /api/export/*`); the phone app points people to the web.
- Meal plan chat-post schedule (`GET/PUT /api/meal-plan/schedule`) and the chat bot keyword list (Settings > Chat keywords, `/api/bot-keywords`): parent settings, web only
- Creating/editing chore definitions and schedules, chore photo proof
  (chores that require a photo are shown with a camera hint), staples,
  list reordering, item suggestions,
  editing an existing reminder's schedule (create, pause/resume/delete are supported) and chat
  linking, household time zone / digest.
- Editing spending rows, receipts, recurring, assets, categories, adding, editing and removing members and
  editing allowances (Settings > Members on the web), imports and bots.
