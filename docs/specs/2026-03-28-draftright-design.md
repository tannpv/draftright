# DraftRight — Design Spec

**Date:** 2026-03-28
**Status:** Approved

## Overview

DraftRight is a standalone macOS menu bar app (Swift/SwiftUI, macOS 13+) that registers as a macOS Services provider. When you select text in any app and right-click, the Services submenu shows tone-based rewrite options. Selecting one sends the text to OpenAI's API, then shows a floating side-by-side diff window to accept or reject.

## Architecture

```
┌─────────────────────────────────────────────┐
│  Any macOS App (Claude, Teams, Safari, etc) │
│                                             │
│  1. Select text → Right-click → Services    │
│     → "DraftRight: Professional"            │
│     → "DraftRight: Casual"                  │
│     → "DraftRight: Fix Grammar"             │
│     → "DraftRight: Shorter"                 │
│     → "DraftRight: Longer"                  │
│                                             │
│  2. App grabs selected text via Services    │
│     → sends to OpenAI (gpt-4o-mini)        │
│     → floating diff window appears          │
│                                             │
│  3. User clicks "Replace" or "Cancel"       │
│     → replaced via Services response        │
│     → or clipboard + paste fallback         │
└─────────────────────────────────────────────┘
```

### Components

- **Menu bar icon** — shows status (idle / rewriting), access to settings
- **Services provider** — 5 registered services, one per tone
- **OpenAI client** — calls chat completions endpoint
- **Floating diff window** — side-by-side original vs rewritten
- **Settings view** — API key, model, endpoint config
- **Keychain storage** — secure API key storage

## Services Integration

macOS Services work by declaring `NSServices` entries in the app's `Info.plist`. Each entry registers a menu item that appears in the right-click → Services submenu for any app with selected text.

### Registered Services (5 total)

| Service Menu Label | Tone | Prompt Behavior |
|---|---|---|
| DraftRight: Professional | Professional | Formal, clear, workplace-appropriate |
| DraftRight: Casual | Casual | Friendly, conversational |
| DraftRight: Fix Grammar | Grammar | Fix errors only, preserve tone/style |
| DraftRight: Shorter | Shorter | Condense while keeping meaning |
| DraftRight: Longer | Longer | Expand with more detail/context |

### How Services Work

1. App declares `NSServices` in Info.plist with `sendTypes: [NSStringPboardType]` and `returnTypes: [NSStringPboardType]`
2. macOS picks up the services on app launch (or after running `/System/Library/CoreServices/pbs -update`)
3. When user selects a service, macOS calls the app's registered selector with an `NSPasteboard` containing the selected text
4. App processes the text, writes the result back to the pasteboard
5. macOS replaces the selected text with the result (if the app supports it — otherwise we fall back to clipboard + paste)

The diff window intercepts step 4 — instead of immediately returning the result, we show the floating diff window first. The replacement only happens when the user clicks "Replace".

## Floating Diff Window

A borderless, always-on-top `NSPanel` that appears near the cursor when a rewrite completes.

### Layout

```
┌─────────────────────────────────────────────┐
│  DraftRight — Professional          [tone]  │
├────────────────────┬────────────────────────┤
│  Original          │  Rewritten             │
├────────────────────┼────────────────────────┤
│  Selected text     │  Improved text with    │
│  as-is, with       │  changes highlighted   │
│  deletions in red  │  in green              │
│                    │                        │
├────────────────────┴────────────────────────┤
│           [ Replace ]  [ Copy ]  [ Cancel ] │
└─────────────────────────────────────────────┘
```

### Behavior

- **Positioning**: appears near the mouse cursor, clamped to screen bounds
- **Sizing**: auto-sizes based on text length, max 600x400px, scrollable if longer
- **Diff highlighting**: word-level diff — deletions shown in red on the left, additions in green on the right
- **Always on top**: `NSPanel` with `.floating` level so it stays above the source app
- **Dismiss**: Cancel button, Escape key, or clicking outside the window
- **Replace**: writes result back via Services pasteboard; falls back to clipboard + Cmd+V paste if the source app doesn't support Services return
- **Copy**: copies rewritten text to clipboard without replacing
- **Loading state**: shows a spinner with "Rewriting..." while waiting for OpenAI response

## OpenAI Client & Prompt Design

### API Configuration

- **Endpoint**: `https://api.openai.com/v1/chat/completions`
- **Model**: `gpt-4o-mini` (fast, cheap, good enough for rewrites)
- **Temperature**: `0.3` (consistent but not robotic)
- **Max tokens**: `1024`
- **Input cap**: `3000` characters (longer text gets truncated with a warning)

### System Prompts per Tone

| Tone | System Prompt |
|---|---|
| Professional | "Rewrite the following text to be professional, clear, and workplace-appropriate. Preserve the original meaning. Return only the rewritten text, no explanations." |
| Casual | "Rewrite the following text to be friendly and conversational. Preserve the original meaning. Return only the rewritten text, no explanations." |
| Fix Grammar | "Fix grammar, spelling, and punctuation errors in the following text. Do not change the tone or style. Return only the corrected text, no explanations." |
| Shorter | "Condense the following text while preserving the key meaning. Return only the shortened text, no explanations." |
| Longer | "Expand the following text with more detail and context while keeping the same tone. Return only the expanded text, no explanations." |

### Error Handling

- No API key → show notification directing to Settings
- Network error / timeout (10s) → show error in diff window with "Retry" button
- Empty response → show "No changes suggested" and dismiss

## Settings & Storage

### Menu Bar Icon

- Uses `NSStatusItem` with a pencil icon (SF Symbol `pencil.and.outline`)
- Click opens a dropdown with:
  - Status indicator (idle / rewriting)
  - "Settings..." opens the settings window
  - "Quit DraftRight"

### Settings Window (SwiftUI)

| Setting | Control | Default |
|---|---|---|
| API Key | Secure text field | (empty, required) |
| API Endpoint | Text field | `https://api.openai.com/v1/chat/completions` |
| Model | Text field | `gpt-4o-mini` |
| Launch at Login | Toggle | Off |

### Storage

- API key → macOS Keychain (via `SecItemAdd`/`SecItemCopyMatching`)
- All other settings → `UserDefaults`
- No telemetry, no logging of user text

### First Launch Flow

1. App appears in menu bar
2. macOS registers the 5 services (may need logout/login or `pbs -update` to appear in right-click)
3. User opens Settings, enters API key
4. Ready to use

## Project Structure

```
DraftRight/
├── DraftRight.xcodeproj
├── DraftRight/
│   ├── Info.plist                    # NSServices declarations (5 tones)
│   ├── DraftRightApp.swift           # SwiftUI app entry, NSApplication delegate
│   ├── Services/
│   │   └── ServiceProvider.swift     # NSServices handler — receives text, triggers rewrite
│   ├── AI/
│   │   ├── OpenAIClient.swift        # Chat completions API client
│   │   └── TonePrompts.swift         # Tone enum + system prompts
│   ├── UI/
│   │   ├── DiffWindow.swift          # Floating NSPanel with side-by-side diff
│   │   ├── DiffView.swift            # SwiftUI view — word-level diff rendering
│   │   ├── MenuBarView.swift         # Status item dropdown
│   │   └── SettingsView.swift        # API key, model, endpoint config
│   ├── Diff/
│   │   └── WordDiff.swift            # Word-level diff algorithm (red/green highlights)
│   └── Utils/
│       ├── KeychainHelper.swift      # Secure API key storage
│       └── ClipboardHelper.swift     # Clipboard + Cmd+V paste fallback
└── README.md
```

### Build Target

- macOS 13.0+, Swift 5.9, SwiftUI
- No external dependencies — URLSession for API calls, built-in Swift diffing for word-level comparison, native macOS APIs for Services/Keychain/Clipboard
