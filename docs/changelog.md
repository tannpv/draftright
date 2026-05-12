# DraftRight Changelog

## 2026-05-12

### Desktop auto-update overhaul (macOS + Windows → 2.2.4)
- Persistent "Update X.Y.Z available" affordance: Windows tray menu item + Settings→Advanced→Updates link; macOS menu-bar item + Settings→Updates button. No longer have to click "Check for Updates" to learn about a release.
- Silent background pre-download: when a check finds an update, the installer/DMG downloads quietly in the background (3 retries, cache-busting, per-attempt timeout — a stalled socket can't hang the progress window anymore). Once staged, the affordance becomes "ready — restart & install" and the install is instant. Install + restart still requires one user click; nothing auto-restarts.
- Both clients prefer the `platforms.<platform>.{version,url}` map from `/updates/latest` over the legacy top-level fields (macOS fully; Windows TODO). Fixes "Windows-only release invisible to Windows clients" and "macOS downloads a stale dmg".
- Backend: `/updates/latest` top-level `version` is now the highest version across all platforms (was hardcoded to macOS's) — defense-in-depth for older clients.
- Known gotcha documented: missing files under `draftright.info/downloads/` return HTTP 200 + a ~28 KB HTML page (not 404) — always verify `content-length`/`content-type` after `release-publish.sh`.
- Added an iOS bug-report integration test (`DraftRightMobile/integration_test/bug_report_test.dart`), runs on the iOS simulator.

## 2026-03-29

### V2 Backend + Admin Portal
- Built NestJS backend API with PostgreSQL (auth, rewrite proxy, subscriptions, usage limits)
- Built React admin portal with Modernize dark theme (dashboard, users, plans, providers, analytics, transactions)
- Updated mobile app + keyboard extensions to use backend instead of direct OpenAI
- Updated macOS app to use backend
- Added Windows/Linux desktop support (Flutter Desktop with system tray + hotkey)
- Renamed to "DraftRight V2" with new bundle IDs for side-by-side install

### V1 Android Keyboard Fix
- Added full QWERTY keyboard to Android IME (was toolbar-only)
- Fixed blank key labels (emoji icons for toolbar + special keys)
- Fixed API key sync between Flutter app and keyboard extension
- Fixed temperature type casting crash

## 2026-03-28

### V1 Initial Release (tag: v1.0)
- macOS menu bar app (Swift/SwiftUI) with NSServices, floating diff panel
- Flutter mobile app with onboarding, settings, test playground
- iOS keyboard extension (Swift)
- Android keyboard extension (Kotlin)
- 6 tones: Simple, Natural, Polished, Concise, Technical, Translate
- Direct OpenAI API integration (user provides own key)
