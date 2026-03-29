# DraftRight Mobile App Update — Design Spec

**Date:** 2026-03-29
**Status:** Approved
**Sub-project:** 3 of 4 (Mobile App Update)

## Overview

Update the existing Flutter mobile app and native keyboard extensions (iOS/Android) to authenticate with the DraftRight backend and proxy rewrite requests through it, instead of calling OpenAI directly. Adds login/register screens and subscription status display.

## Changes Summary

| Component | Change |
|---|---|
| Flutter app | Replace API key settings with login/register + subscription status |
| Android keyboard (`DraftRightIME`) | Call backend `/rewrite` instead of OpenAI directly |
| iOS keyboard extension | Call backend `/rewrite` instead of OpenAI directly |
| macOS app | Call backend `/rewrite` instead of OpenAI directly |

## Flutter App Changes

### New Screens

**Login Screen** (`lib/screens/login_screen.dart`)
- Email + password fields
- Login button → `POST /auth/login`
- "Create account" link → Register screen
- Stores JWT (access + refresh tokens) in flutter_secure_storage
- On success → navigates to home

**Register Screen** (`lib/screens/register_screen.dart`)
- Name + email + password fields
- Register button → `POST /auth/register`
- Stores JWT, navigates to home

**Subscription Screen** (`lib/screens/subscription_screen.dart`)
- Shows: plan name, daily limit, usage today, billing period, expires at
- Calls `GET /subscription`
- "Upgrade" button (future: links to app store)

### Modified Screens

**Settings Screen** (`lib/screens/settings_screen.dart`)
- Remove: AI Provider picker, API Key field, Server URL, Model, Temperature
- Keep: Translation Language picker
- Add: Backend server URL field (for self-hosted users)
- Add: Logout button
- Add: Link to Subscription screen

**Onboarding Screen** (`lib/screens/onboarding_screen.dart`)
- Add login/register step after keyboard enable instructions

### New Services

**Auth Service** (`lib/services/auth_service.dart`)
- `login(email, password)` → stores tokens
- `register(email, password, name)` → stores tokens
- `logout()` → clears tokens
- `getAccessToken()` → returns token, auto-refreshes if expired
- `isLoggedIn` → bool
- Stores tokens in flutter_secure_storage
- Syncs access token to SharedPreferences for keyboard extension access

**Backend Client** (`lib/services/backend_client.dart`)
- `rewrite(text, tone, targetLanguage?)` → calls `POST /rewrite`
- `getSubscription()` → calls `GET /subscription`
- Uses auth_service for JWT headers
- Configurable base URL (default: production server, overridable for self-hosted)

### Modified Services

**Settings Service** (`lib/services/settings_service.dart`)
- Remove: apiKey, aiProvider, endpoint, model, temperature
- Keep: translateLanguage
- Add: backendUrl (default: production URL)
- Sync backendUrl to SharedPreferences for keyboard extension

### Storage for Keyboard Extension Access

The keyboard extension runs in a separate process and needs:
- **JWT access token** → synced to SharedPreferences (key: `flutter.draftright.accessToken`)
- **Backend URL** → synced to SharedPreferences (key: `flutter.draftright.backendUrl`)
- **Translation language** → already in SharedPreferences

## Android Keyboard Extension Changes

### `SharedSettings.kt`
- Remove: `apiKey`, `aiProvider`, `endpoint`, `model`, `temperature`
- Add: `accessToken` (reads from SharedPreferences)
- Add: `backendUrl` (reads from SharedPreferences)

### `OpenAIClient.kt` → rename to `BackendClient.kt`
- Change endpoint from OpenAI URL to `backendUrl + "/rewrite"`
- Change request body from OpenAI format to: `{ text, tone, target_language? }`
- Change auth header from `Bearer <api_key>` to `Bearer <accessToken>`
- Change response parsing from `choices[0].message.content` to `rewritten_text`

### `DraftRightIME.kt`
- Update to use `BackendClient` instead of `OpenAIClient`
- Show "Please login in DraftRight app" instead of "set up API key" when no token

## iOS Keyboard Extension Changes

Same pattern as Android:

### `SharedSettings.swift`
- Remove API key fields
- Add `accessToken` and `backendUrl` from App Group UserDefaults

### `OpenAIClient.swift` → rename to `BackendClient.swift`
- Call backend `/rewrite` endpoint
- Use JWT access token for auth

### `KeyboardViewController.swift`
- Update to use `BackendClient`

## macOS App Changes

### `AppModel.swift`
- Remove: `apiKey` from Keychain
- Add: `accessToken`, `refreshToken` in Keychain
- Add: `backendUrl` in UserDefaults
- Add: login/logout methods

### `OpenAIClient.swift` → update to `BackendClient.swift`
- Call backend `/rewrite` instead of OpenAI directly

### `SettingsView.swift`
- Replace API key field with login/logout
- Add backend URL field
- Show subscription status

### `ServiceProvider.swift`
- Update to use BackendClient

## File Structure Changes

```
DraftRightMobile/lib/
├── screens/
│   ├── login_screen.dart          # NEW
│   ├── register_screen.dart       # NEW
│   ├── subscription_screen.dart   # NEW
│   ├── settings_screen.dart       # MODIFIED
│   ├── onboarding_screen.dart     # MODIFIED
│   └── playground_screen.dart     # MODIFIED (use backend)
├── services/
│   ├── auth_service.dart          # NEW
│   ├── backend_client.dart        # NEW
│   ├── settings_service.dart      # MODIFIED
│   └── openai_client.dart         # REMOVED
└── models/
    └── tone.dart                  # UNCHANGED

DraftRightMobile/android/.../keyboard/
├── BackendClient.kt               # NEW (replaces OpenAIClient.kt)
├── SharedSettings.kt              # MODIFIED
├── DraftRightIME.kt               # MODIFIED
├── OpenAIClient.kt                # REMOVED
└── (rest unchanged)

DraftRightMobile/ios/DraftRightKeyboard/
├── BackendClient.swift             # NEW (replaces OpenAIClient.swift)
├── SharedSettings.swift            # MODIFIED
├── KeyboardViewController.swift    # MODIFIED
├── OpenAIClient.swift              # REMOVED
└── (rest unchanged)

DraftRight/ (macOS app)
├── AI/BackendClient.swift          # NEW (replaces OpenAIClient.swift)
├── AppModel.swift                  # MODIFIED
├── Services/ServiceProvider.swift  # MODIFIED
├── UI/SettingsView.swift           # MODIFIED
└── AI/OpenAIClient.swift           # REMOVED
```
