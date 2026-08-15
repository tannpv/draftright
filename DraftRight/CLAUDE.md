# DraftRight macOS App

Native Swift/SwiftUI menu bar app for AI-powered text rewriting via system Services.

## Architecture

- Menu bar app (`LSUIElement = true`) — no dock icon
- Registers 5 NSServices in system right-click menu (one per tone)
- Floating `NSPanel` for side-by-side diff preview
- Grammarly-style floating trigger button

## Modules

```
DraftRight/
├── DraftRightApp.swift        # Entry point, @main
├── AppModel.swift             # State: tokens, backend URL, settings (Keychain + UserDefaults)
├── Info.plist                 # NSServices declarations, bundle ID
├── AI/BackendClient.swift     # Calls backend /rewrite with JWT auth
├── Services/ServiceProvider.swift  # Handles NSServices callbacks
├── UI/
│   ├── MenuBarView.swift      # Menu bar icon + dropdown
│   ├── SettingsView.swift     # Login/logout, backend URL, language
│   └── ...
├── Diff/                      # Word-level diff algorithm
├── Utils/
│   ├── KeychainHelper.swift   # Secure token storage (service: com.draftright.app.v2)
│   └── ClipboardHelper.swift  # Paste simulation
└── Accessibility/             # Selection monitoring
```

## Bundle IDs

- **V1**: `com.draftright.app`
- **V2**: `com.draftright.app.v2`

## Build

```bash
swift build                    # via Package.swift at project root
# Or open in Xcode
```

## V1 → V2 Changes

- `OpenAIClient.swift` → `BackendClient.swift` (calls backend instead of OpenAI directly)
- `AppModel` stores JWT tokens in Keychain instead of API key
- `SettingsView` has login/logout instead of API key field

## Rewrite trigger (Settings → Trigger)

Two mutually-exclusive triggers via `TriggerMode` enum (`TriggerMode.swift`):
**Pencil** (drag-highlight text → floating pencil appears → click) or **Hotkey**.
The mode is decoupled from the hotkey combo (`AppModel.hotkeyString`) so
switching to Pencil keeps the saved shortcut. `SelectionMonitor` gates each
mechanism on `usesPencil`/`usesHotkey`.

**Hard-won rules (the #176–#182 saga — full detail in the maintainer's
`feedback_macos_pencil_trigger` memory):**
- Pencil shows on a **drag that selected text ONLY** — never a click/double-click
  (a double-click selects a word; AX reporting text is NOT a trigger signal).
- **Never synthesize ⌘C on selection** — only on a deliberate action (pencil
  click / hotkey), via `captureViaCmdC()`. ⌘C-on-selection broke the user's own
  Copy (#178). Pre-capture is AX-read-only.
- Terminal/Electron are **AX-blind** (`kAXSelectedText` err -25205/-25212) → a
  drag gesture is the only signal there.
- `shouldShowPencil` is pure + unit-tested; position/render bugs need the log
  (`~/Library/Logs/DraftRight/draftright.log`, `[MONITOR]` lines).
