# DraftRight Windows/Linux Desktop — Design Spec

**Date:** 2026-03-29
**Status:** Approved
**Sub-project:** 4 of 4 (Windows/Linux Desktop)

## Overview

Flutter Desktop apps for Windows and Linux that provide AI-powered text rewriting via system tray + global hotkey. User selects text in any app, presses a hotkey, picks a tone, and gets the rewrite. Connects to the DraftRight backend for auth and rewrite proxying.

Note: macOS keeps its existing native Swift app. This sub-project covers Windows and Linux only.

## How It Works

1. App runs in system tray (background)
2. User selects text in any application
3. User presses global hotkey (`Ctrl+Shift+R`)
4. App reads clipboard (user must copy first, or app sends Ctrl+C)
5. Floating panel appears with tone buttons
6. User taps a tone → calls backend `/rewrite`
7. Diff preview shows original vs rewritten
8. Replace (pastes back via clipboard) or Copy or Cancel

## Tech Stack

- **Framework:** Flutter Desktop (same codebase as mobile)
- **Platforms:** Windows, Linux
- **Packages:**
  - `system_tray` — system tray icon + menu
  - `hotkey_manager` — global hotkey registration
  - `window_manager` — floating window positioning, always-on-top
  - `flutter_secure_storage` — token storage
  - `http` — API calls

## Screens / Views

### System Tray Menu
- "DraftRight" (disabled label)
- Separator
- "Settings" → opens settings window
- "About" → version info
- "Quit" → exit app

### Settings Window (reuses mobile settings pattern)
- Login/Register (same as mobile)
- Backend URL
- Translation language
- Subscription status
- Hotkey display (read-only for now, `Ctrl+Shift+R`)

### Floating Rewrite Panel
- Appears near cursor when hotkey pressed
- Tone buttons row: Simple, Natural, Polished, Concise, Technical, Translate
- Loading spinner on tone selection
- Diff view: original vs rewritten (side by side)
- Buttons: Replace (pastes to clipboard + simulates Ctrl+V), Copy, Cancel
- Click outside or Escape dismisses
- Always-on-top, frameless window

## Architecture

```
┌─────────────────────────────┐
│ Flutter Desktop App          │
│                              │
│  System Tray (background)    │
│  Global Hotkey Listener      │
│                              │
│  ┌────────────────────────┐  │
│  │ Floating Rewrite Panel │  │
│  │ [Tones] [Diff] [Actions]│  │
│  └────────────────────────┘  │
│                              │
│  Auth Service (JWT)          │
│  Backend Client (/rewrite)   │
│  Settings Service            │
└──────────────┬───────────────┘
               │
               ▼
        DraftRight Backend
```

## File Structure

Reuses the existing `DraftRightMobile` Flutter project with platform-specific desktop code:

```
DraftRightMobile/
├── lib/
│   ├── main.dart                    # MODIFIED — detect platform, show tray or mobile UI
│   ├── desktop/
│   │   ├── desktop_app.dart         # Desktop entry point, tray + hotkey setup
│   │   ├── floating_panel.dart      # Rewrite panel UI
│   │   └── tray_manager.dart        # System tray setup
│   ├── screens/                     # Shared with mobile
│   │   ├── login_screen.dart
│   │   ├── settings_screen.dart
│   │   └── ...
│   └── services/                    # Shared with mobile
│       ├── auth_service.dart
│       ├── backend_client.dart
│       └── settings_service.dart
├── windows/                         # Flutter Windows runner
└── linux/                           # Flutter Linux runner
```

## Desktop-Specific Behavior

### Clipboard Workflow
1. Hotkey pressed → app sends `Ctrl+C` to system (copies selected text)
2. Small delay (100ms) → read clipboard
3. If clipboard has text → show floating panel
4. If empty → show toast "Select text first"
5. On "Replace" → write rewritten text to clipboard → send `Ctrl+V`

### Window Management
- Floating panel: 400x350px, frameless, always-on-top
- Position: near mouse cursor (offset by 20px)
- Dismiss: Escape key, click outside, or Cancel button
- Settings window: standard 500x600px window

### Platform Differences
| Feature | Windows | Linux |
|---|---|---|
| System tray | Native tray icon | AppIndicator / StatusNotifier |
| Global hotkey | Win32 RegisterHotKey | X11/Wayland keybinding |
| Clipboard | Win32 clipboard API | xclip/wl-clipboard |
| Paste simulation | SendInput(Ctrl+V) | xdotool/wtype |

These are handled by the Flutter packages — no platform-specific code needed.

## Build & Distribution

```bash
# Windows
flutter build windows --release

# Linux
flutter build linux --release
```

Output: standalone executable in `build/windows/` or `build/linux/`.

Future: package as `.msi` (Windows) and `.deb`/`.AppImage` (Linux).

## Shared Code with Mobile

The desktop app reuses:
- `auth_service.dart` — login, token management
- `backend_client.dart` — API calls
- `settings_service.dart` — backend URL, language
- `tone.dart` — tone definitions

New desktop-only code:
- `desktop_app.dart` — tray + hotkey orchestration
- `floating_panel.dart` — rewrite UI
- `tray_manager.dart` — system tray setup
