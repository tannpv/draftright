# DraftRight Mobile — Design Spec

**Date:** 2026-03-28
**Status:** Approved
**Platform:** iOS + Android (Flutter + native keyboard extensions)

## Overview

DraftRight Mobile is a Flutter app with native keyboard extensions for iOS and Android. It adds an AI-powered rewrite toolbar above the system keyboard. Users type on their normal keyboard, tap a tone icon on the toolbar, preview the rewrite in a bottom sheet diff view, then replace or copy.

## Architecture

```
┌──────────────────────────────────────────────┐
│  Flutter App (main app)                      │
│  - Onboarding (enable keyboard instructions) │
│  - Settings (provider, API key, language)    │
│  - Test Playground                           │
└──────────────────────────────────────────────┘

┌──────────────────────────────────────────────┐
│  Native Keyboard Extension (per platform)     │
│                                              │
│  ┌─────────────────────────────────────────┐ │
│  │ System Keyboard (user's normal keyboard) │ │
│  ├─────────────────────────────────────────┤ │
│  │ DraftRight Toolbar (above keyboard)      │ │
│  │ [✎][💬][✨][⊘][🔧][🌐▾]    [↻ Undo]   │ │
│  └─────────────────────────────────────────┘ │
│                                              │
│  On tone tap:                                │
│  1. Read text from input field               │
│  2. Call OpenAI / Ollama API                 │
│  3. Show bottom sheet with diff preview       │
│  4. Replace / Copy / Cancel                   │
└──────────────────────────────────────────────┘
```

### Communication

- Flutter app writes settings to shared storage (App Group on iOS, SharedPreferences on Android)
- Keyboard extension reads settings from shared storage
- Keyboard extension calls AI API directly (or via backend proxy)
- No real-time communication between the app and extension needed

## Keyboard Toolbar

Sits as an Input Accessory View (iOS) / candidate view area (Android) above the system keyboard. Always visible when enabled.

### Layout

```
┌──────────────────────────────────────────────────┐
│ [✎][💬][✨][⊘][🔧][🌐▾]              [↻ Undo]  │
└──────────────────────────────────────────────────┘
  Simple Natural Polished Concise Technical Translate
```

### Tone Options

| Icon | Tone | Prompt Behavior |
|---|---|---|
| ✎ | Simple | Easy-to-understand language, short sentences, common words |
| 💬 | More Natural | Conversational, smooth flow, remove awkward phrasing |
| ✨ | More Polished | Professional, refined word choice, workplace-appropriate |
| ⊘ | Concise | Remove filler, redundancy, keep it tight |
| 🔧 | Technical | Spec-style, precise, unambiguous documentation language |
| 🌐 | Translate | Translate to selected language; shows language picker dropdown on tap |

### Behavior

- **Tone icons** — horizontally scrollable if screen is narrow. Tap to trigger rewrite.
- **Translate button** — has dropdown arrow. Tap shows language picker popup (30+ languages) before translating.
- **Undo button** — appears for 5 seconds after Replace, lets user revert to original text.
- **Loading state** — tapped icon shows spinner, toolbar disabled until result arrives.
- **No API key** — tapping any tone shows banner: "Open DraftRight app to set up API key"
- **Toolbar height** — ~44px (standard iOS accessory view height)

### Reading/Writing Text

- **iOS**: `UITextDocumentProxy` — reads full text, replaces selected or all text
- **Android**: `InputConnection` — `getExtractedText()` reads, `commitText()` writes back

## Bottom Sheet Diff Preview

When user taps a tone, after the API returns, a bottom sheet slides up showing the diff.

### Layout

```
┌──────────────────────────────────────────────┐
│  ── drag handle ──                           │
│                                              │
│  ┌─────────────────┬───────────────────────┐ │
│  │ Original        │ Rewritten             │ │
│  │                 │                       │ │
│  │ Selected text   │ Improved text with    │ │
│  │ with deletions  │ additions in green    │ │
│  │ in red          │                       │ │
│  └─────────────────┴───────────────────────┘ │
│                                              │
│  [ Cancel ]    [ Copy ]    [ Replace ]       │
└──────────────────────────────────────────────┘
```

### Behavior

- Slides up over the keyboard
- Draggable — expand to full screen for long texts
- Side-by-side word-level diff (red deletions, green additions)
- Replace — writes rewritten text back via UITextDocumentProxy / InputConnection
- Copy — copies to clipboard, dismisses sheet
- Cancel — dismisses sheet, original text stays
- Both panels scroll independently

### Constraints

- iOS keyboard extensions have 30MB memory limit — keep UI lightweight
- Android has no hard limit but keep lean for performance

## Flutter Main App

### Screens

**1. Onboarding (first launch)**
- Welcome screen explaining DraftRight
- Step-by-step guide to enable keyboard:
  - iOS: Settings → General → Keyboard → Keyboards → Add → DraftRight
  - Android: Settings → Language & Input → Manage Keyboards → Enable DraftRight
- "Allow Full Access" explanation (iOS requires this for network calls)

**2. Settings**

| Setting | Control | Default |
|---|---|---|
| AI Provider | Picker: OpenAI / Custom Server | OpenAI |
| API Key | Secure text field (optional for Custom Server) | (empty) |
| Server URL | Text field | `https://api.openai.com/v1/chat/completions` |
| Model | Text field | `gpt-4o-mini` |
| Temperature | Slider 0–1 | 0.3 |
| Translation Language | Picker (30+ languages) | Vietnamese |

Custom Server auto-fills Ollama defaults (`http://server:11434/v1/chat/completions`, model `llama3`). API key optional for custom servers.

**3. Test Playground**
- Text field to test the rewrite flow in-app
- Verifies API key works and demonstrates tones

### Storage

- API key → Flutter Secure Storage (Keychain on iOS, EncryptedSharedPreferences on Android)
- All other settings → SharedPreferences (accessible by keyboard extension)
- iOS: App Group (`group.com.draftright.app`) shares data between main app and extension
- Android: SharedPreferences in shared process mode

## Project Structure

```
DraftRightMobile/
├── lib/                              # Flutter (Dart)
│   ├── main.dart                     # App entry point
│   ├── screens/
│   │   ├── onboarding_screen.dart    # Enable keyboard instructions
│   │   ├── settings_screen.dart      # API key, provider, language
│   │   └── playground_screen.dart    # Test rewrite in-app
│   ├── services/
│   │   ├── openai_client.dart        # OpenAI/Ollama API client
│   │   └── settings_service.dart     # Read/write shared settings
│   └── models/
│       └── tone.dart                 # Tone enum + system prompts
│
├── ios/
│   ├── Runner/                       # Flutter iOS app
│   ├── DraftRightKeyboard/           # iOS Keyboard Extension (Swift)
│   │   ├── KeyboardViewController.swift  # Main keyboard extension
│   │   ├── ToolbarView.swift         # Tone buttons toolbar
│   │   ├── DiffSheetView.swift       # Bottom sheet diff preview
│   │   ├── OpenAIClient.swift        # API client
│   │   └── Info.plist                # Extension config
│   └── Podfile
│
├── android/
│   ├── app/                          # Flutter Android app
│   └── keyboard/                     # Android Keyboard Module (Kotlin)
│       ├── DraftRightIME.kt          # InputMethodService
│       ├── ToolbarView.kt            # Tone buttons toolbar
│       ├── DiffSheetView.kt          # Bottom sheet diff preview
│       ├── OpenAIClient.kt           # API client
│       └── AndroidManifest.xml       # IME service declaration
│
├── pubspec.yaml
└── README.md
```

### Key Points

- Tone enum + system prompts duplicated in Dart, Swift, and Kotlin (keeps each layer independent)
- API client duplicated per platform (~50 lines each) since keyboard extensions run in separate process
- Flutter main app and keyboard extensions share data via App Group (iOS) / SharedPreferences (Android)

## System Prompts

| Tone | System Prompt |
|---|---|
| Simple | "Rewrite the following text using simple, easy-to-understand language. Use short sentences and common words. Preserve the original meaning. Return only the rewritten text, no explanations." |
| More Natural | "Rewrite the following text to sound more natural and conversational, as if spoken by a real person. Remove awkward phrasing and make it flow smoothly. Preserve the original meaning. Return only the rewritten text, no explanations." |
| More Polished | "Rewrite the following text to be more polished and professional. Improve grammar, word choice, and sentence structure for a refined, workplace-appropriate tone. Preserve the original meaning. Return only the rewritten text, no explanations." |
| Concise | "Rewrite the following text to be as concise as possible. Remove unnecessary words, redundancy, and filler while preserving the key meaning. Return only the rewritten text, no explanations." |
| Technical | "Rewrite the following text in a technical specification style. Use precise, unambiguous language suitable for documentation, specs, or technical communication. Preserve the original meaning. Return only the rewritten text, no explanations." |
| Translate | "Translate the following text into {targetLanguage}. If the text is already in {targetLanguage}, translate it into English instead. Preserve the original meaning and tone. Return only the translated text, no explanations." |

## Supported Translation Languages

Arabic, Chinese (Simplified), Chinese (Traditional), Czech, Danish, Dutch, English, Finnish, French, German, Greek, Hebrew, Hindi, Hungarian, Indonesian, Italian, Japanese, Korean, Malay, Norwegian, Polish, Portuguese, Romanian, Russian, Spanish, Swedish, Thai, Turkish, Ukrainian, Vietnamese
