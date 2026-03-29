# DraftRight Changelog

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
