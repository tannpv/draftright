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
