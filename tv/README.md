# Kinkit TV

View-only big-screen apps for Kinkit. They show the current month's income,
budget, spending, savings, budget-vs-actual per category and a 6-month trend,
refreshing every 60 seconds. They **cannot log in or change anything**: the only
account data they read comes from the token-based, read-only
`GET /api/tv/data?k=<token>` endpoint (backend-go/tvshare.go).

| Folder | Targets |
|---|---|
| `flutter/` | Android TV, Google TV, Fire TV (and any Android device) |
| `web/` | Samsung Tizen, LG webOS, and any TV / kiosk browser (plain HTML/CSS/ES5 JS, no build step) |

## 1. Get a TV token

1. Sign in to the Kinkit web app as a family admin or parent.
2. Open **Settings > Data & devices > TV / car display links**.
3. Enter a name (e.g. "Living room TV") and press **Create link**. The link is copied:
   `https://your-kinkit-server/tv?k=<TOKEN>`
4. On the TV, either enter the **server** (`https://your-kinkit-server`) and the
   **token** (the part after `?k=`), or paste/type the **whole link** into either
   field: the apps split it for you.

Revoking the link in the web app stops the TV within a minute. The TV then goes
back to the pairing screen with a "link was revoked" message.

Tokens are about 43 characters long. Typing one with the remote works but is slow.
Easier options:
- a USB/Bluetooth keyboard, or a remote app with a keyboard (Google TV app, Fire TV app, LG ThinQ, SmartThings);
- Android: `adb shell input text 'https://your-server/tv?k=TOKEN'` while the token field is selected;
- web build: open `index.html?server=https://your-server&k=TOKEN` once (it pairs itself and remembers).

## 2. Remote controls (both apps)

| Key | Action |
|---|---|
| Left / Right (also CH+/CH-, rewind/fast-forward) | previous / next page |
| OK / Enter (short press) | pause / resume the auto-rotation (every 15 s) |
| Play/Pause | pause / resume |
| Hold BACK or hold OK (0.7 s), MENU, INFO, RED, or `M`/`S` on a keyboard | open Settings (refresh, change server/token, unpair, exit) |
| BACK (short press) on the dashboard | exit the app |

On the pairing screen, the D-pad moves between the fields, the on-screen keyboard
and the buttons. Moving onto a field makes it the one you are typing into. A hardware
keyboard types straight into the active field. **System keyboard** opens the TV's own
keyboard (IME).

When the server can't be reached, the last data stays on screen with an
**OFFLINE - hh:mm** badge, and the app keeps retrying every minute.

## 3. Flutter (Android TV / Google TV / Fire TV)

Needs Flutter 3.22+ (Dart 3.3+), the Android SDK and JDK 17.

```bash
cd tv/flutter
flutter create . --platforms=android --org com.kinkit --project-name kinkit_tv  # fills in missing boilerplate (gradlew, wrapper jar, local.properties); keeps existing files
flutter pub get
flutter test                      # unit tests: JSON parsing, URL/token parsing, money formatting
flutter build apk --release       # build/app/outputs/flutter-apk/app-release.apk
```

`flutter create .` doesn't overwrite files that already exist (the manifest, MainActivity,
gradle files, lib/). If it asks about the `android/` folder, keep the committed versions.

Sideload:
- **Android TV / Google TV**: turn on Developer options (Settings > System > About > tap *Build* 7 times),
  turn on USB/network debugging, then `adb connect <tv-ip>:5555 && adb install -r app-release.apk`.
- **Fire TV**: Settings > My Fire TV > Developer options > ADB debugging = on (and "Apps from unknown sources"),
  then `adb connect <firetv-ip>:5555 && adb install -r app-release.apk`. You can also use the *Downloader* app with a URL to the APK.

Notes:
- The manifest declares `LEANBACK_LAUNCHER`, `android.software.leanback` / `android.hardware.touchscreen`
  as `required="false"`, `INTERNET`, `usesCleartextTraffic="true"` (LAN servers over plain http), and
  `FLAG_KEEP_SCREEN_ON` so the screen doesn't sleep.
- `res/drawable/banner.xml` (320x180dp) and `res/drawable/ic_launcher.xml` are placeholder vector art.
  For Play Store / Amazon Appstore, add a 320x180 `res/drawable-xhdpi/banner.png` with the wordmark
  and proper mipmap launcher icons, and set up a real release `signingConfig` (sideload builds use the debug key).
- The layout is drawn on a fixed 960x540dp canvas and scaled to the screen (`TvCanvas`), with
  48dp/27dp overscan margins, so it looks the same on 720p, 1080p and 4K boxes.

## 4. Web build (Samsung Tizen / LG webOS / any browser)

`tv/web/` is the whole app: `index.html`, `styles.css`, `app.js` (ES5, no dependencies),
`config.xml` (Tizen), `appinfo.json` (webOS), and placeholder icons. It's laid out
on a 1920x1080 stage that scales to the window, with a 5% overscan margin.

**Quick test in any browser:** serve the folder (`python3 -m http.server -d tv/web 8080`) and open
`http://localhost:8080/?server=https://your-kinkit-server&k=TOKEN`. Arrow keys, Enter, Esc/Backspace
act as the remote. The backend allows cross-origin reads of the two TV endpoints (see section 6).

**Same-origin hosting (no CORS needed):** if `tv/web/` is served by the Kinkit server itself
(e.g. at `/tv-app/`), opening `/tv-app/?k=TOKEN` pairs itself using the page's own origin as the server.

### Samsung Tizen (2017+ TVs, Tizen 3.0+)
1. Install Tizen Studio with the *TV Extensions* and *Samsung Certificate Extension*.
2. Create a Samsung certificate profile (Tools > Certificate Manager). For your own TV, a *Samsung* author + distributor certificate that includes the TV's DUID.
3. On the TV: Apps > type `12345` on the remote > Developer mode **On**, enter your PC's IP, reboot the TV.
4. Package and install:
   ```bash
   cd tv/web
   tizen package -t wgt -s <certificate-profile> -- .
   sdb connect <tv-ip>
   tizen install -n KinkitTV.wgt -t <device-name-from-'sdb devices'>
   ```
   (Or in Tizen Studio: File > Import > Tizen > Tizen Project from this folder, then Run As > Tizen Web Application.)
- `config.xml` grants `internet` and `tv.inputdevice` (to register Play/Pause, CH+/-, RED, INFO keys),
  `<access origin="*">` and a CSP with `connect-src *` so it can reach a self-hosted server.
  Change the `tizen:application id` / `package` (10 characters) to your own.
- BACK is key code **10009**. A short press on the dashboard exits via `tizen.application.getCurrentApplication().exit()`.

### LG webOS (webOS 3.0+)
1. Install the webOS TV CLI (`npm install -g @webos-tools/cli`, or the older `ares-cli` from the webOS TV SDK).
2. On the TV: install the **Developer Mode** app from the LG Content Store, sign in with an LG developer account, turn on Dev Mode + Key Server, then reboot.
3. Package and install:
   ```bash
   cd tv
   ares-setup-device                     # add the TV (IP, port 9922, passphrase from the Developer Mode app)
   ares-package web                      # -> app.kinkit.tv_1.0.0_all.ipk
   ares-install --device <tv> app.kinkit.tv_1.0.0_all.ipk
   ares-launch --device <tv> app.kinkit.tv
   ```
- `appinfo.json` sets `"disableBackHistoryAPI": true`, so BACK arrives as key code **461** and the app handles it.
  A short press on the dashboard calls `PalmSystem.platformBack()` to leave the app.
- Dev Mode sessions expire after 50 hours unless extended in the Developer Mode app.

## 5. API contract: fields consumed from `GET /api/tv/data?k=<token>`

No cookies and no custom headers; the token goes in the query string. `401` means the token is invalid or revoked (the apps unpair and show the pairing screen). Any other failure counts as offline (the apps keep showing the last cached payload).

```jsonc
{
  "year": 2026,                    // int, current UTC year
  "month": 11,                     // int 1-12, current UTC month
  "currency": "IDR",               // account display currency (IDR, USD, EUR, GBP, SGD, MYR, JPY, AUD, INR)
  "display_name": "John Smith",    // string or null - owner's name; header fallback when household_name is ""
  "household_name": "The Smiths",  // string, family name ("" when unset) - shown next to the logo
  "report": {
    "total_income": 25000000,      // int, IDR      -> "Income" card
    "budgeted_expenses": 15000000, // int, IDR      -> "Budget" card, % of budget
    "savings": 4000000,            // int, IDR      -> "Savings" card
    "left_for_budgeting": 6000000, // int, IDR      -> "Left to budget"
    "expenses_list": [             // one row per category
      {
        "category_name": "Groceries",  // string
        "main_category": "Needs",      // string (parsed, not displayed yet)
        "allocation": 4000000,         // int, IDR budgeted this month
        "realization": 4350000         // int, IDR spent this month
      }
    ]
  },
  "trend": [                       // 6 months, oldest first, ending at year/month
    { "year": 2026, "month": 6, "income": 22000000, "expense": 12000000 }
  ]
}
```

Derived on the client: **Spent** = sum of `realization` across categories; **Budget remaining** = `budgeted_expenses - Spent`; the budget pages show categories with `allocation > 0 || realization > 0`, most-used-of-budget first; the top-5 expenses are sorted by `realization`.

Money: every amount is in **IDR**. Like the web app (`react/src/api.js` `formatRupiah`), the TV apps convert to `currency` using the public, read-only `GET /api/fx-rates` (`{"base":"IDR","rates":{"USD":0.000061,...}}`, no session, no account data), cached for 6 hours. Without a rate they show IDR rather than a wrong number.

## 6. Backend notes

- **CORS:** `GET /api/tv/data` and `GET /api/fx-rates` (only those) send `Access-Control-Allow-Origin: *` (`publicCORS` in `backend-go/tvshare.go`), so the web build works from any origin, including `file://` in packaged Tizen/webOS apps. No credentials are allowed; the token is in the URL and the apps send only simple GETs, so no preflight is needed.
- **Default server:** build with `--dart-define=KINKIT_SERVER=https://your-server` (Flutter) or set `<html data-default-server="https://your-server">` in `tv/web/index.html` to pre-fill the server field on first pairing.
- **Signing (Flutter):** `android/key.properties` (storeFile, storePassword, keyAlias, keyPassword) switches release builds from the debug key to your keystore. Keep the same keystore so new APKs install as updates.
- **Mixed content:** an https-hosted web build can't call an http-only server. Use https, or host the web build on http too.
