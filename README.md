# DraftRight

macOS menu bar app that adds AI-powered text rewriting to the system right-click menu. Select text in any app, right-click, choose a tone from the Services submenu, and get a side-by-side diff preview.

## Features

- **System-wide Services integration** — works in Claude Desktop, Teams, Safari, Chrome, Notes, and any app with text selection
- **5 rewrite tones**: Professional, Casual, Fix Grammar, Shorter, Longer
- **Side-by-side diff** — word-level highlighting showing exactly what changed (red = removed, green = added)
- **Replace / Copy / Cancel** — accept the rewrite, copy it, or dismiss
- **Secure storage** — API key stored in macOS Keychain
- **No external dependencies** — pure Swift/SwiftUI/AppKit

## Setup

1. Build and run the app (see Building below)
2. Click the pencil icon in the menu bar → Settings
3. Enter your OpenAI API key
4. Select text in any app → right-click → Services → choose a DraftRight option

## Building

### SwiftPM (command line)

```bash
cd /opt/openAi/DraftRight
swift build -c release
```

The built binary is at `.build/release/DraftRight`.

### Xcode

Open the folder in Xcode, select the DraftRight scheme, and build (Cmd+B).

## How It Works

DraftRight registers as a macOS Services provider via `NSServices` in Info.plist. When you right-click selected text, macOS shows DraftRight's tone options in the Services submenu. Selecting one sends the text to OpenAI's API, then displays a floating diff window.

## Troubleshooting

- **Services not appearing**: Quit and relaunch the app. Open Settings and click "Refresh Services". You may need to log out and back in for macOS to pick up new services.
- **API errors**: Check your API key and endpoint in Settings.
- **Replace not working**: Some apps don't support Services text replacement. Use "Copy" instead — it copies the rewrite to your clipboard.
