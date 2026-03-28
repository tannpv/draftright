# DraftRight Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a macOS menu bar app that registers system-wide Services for AI-powered text rewriting with tone options and a side-by-side diff preview.

**Architecture:** Standalone Swift/SwiftUI menu bar app using `NSServices` to register 5 tone-based rewrite options in the system right-click menu. Selected text is sent to OpenAI's chat completions API, and results are shown in a floating `NSPanel` with word-level diff highlighting. API key stored in Keychain, settings in UserDefaults.

**Tech Stack:** Swift 5.9, SwiftUI, AppKit, macOS 13.0+, no external dependencies

**Spec:** `docs/specs/2026-03-28-draftright-design.md`

---

### Task 1: Create Xcode Project Skeleton

**Files:**
- Create: `DraftRight/DraftRightApp.swift`
- Create: `DraftRight/Info.plist`

- [ ] **Step 1: Create the project directory structure**

```bash
cd /opt/openAi/DraftRight
mkdir -p DraftRight/Services DraftRight/AI DraftRight/UI DraftRight/Diff DraftRight/Utils
```

- [ ] **Step 2: Create Info.plist with NSServices declarations**

Create `DraftRight/Info.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>DraftRight</string>
    <key>CFBundleDisplayName</key>
    <string>DraftRight</string>
    <key>CFBundleIdentifier</key>
    <string>com.draftright.app</string>
    <key>CFBundleVersion</key>
    <string>1</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0.0</string>
    <key>CFBundleExecutable</key>
    <string>DraftRight</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSMinimumSystemVersion</key>
    <string>13.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSPrincipalClass</key>
    <string>NSApplication</string>
    <key>NSServices</key>
    <array>
        <dict>
            <key>NSMenuItem</key>
            <dict>
                <key>default</key>
                <string>DraftRight: Professional</string>
            </dict>
            <key>NSMessage</key>
            <string>rewriteProfessional</string>
            <key>NSPortName</key>
            <string>DraftRight</string>
            <key>NSSendTypes</key>
            <array><string>NSStringPboardType</string></array>
            <key>NSReturnTypes</key>
            <array><string>NSStringPboardType</string></array>
        </dict>
        <dict>
            <key>NSMenuItem</key>
            <dict>
                <key>default</key>
                <string>DraftRight: Casual</string>
            </dict>
            <key>NSMessage</key>
            <string>rewriteCasual</string>
            <key>NSPortName</key>
            <string>DraftRight</string>
            <key>NSSendTypes</key>
            <array><string>NSStringPboardType</string></array>
            <key>NSReturnTypes</key>
            <array><string>NSStringPboardType</string></array>
        </dict>
        <dict>
            <key>NSMenuItem</key>
            <dict>
                <key>default</key>
                <string>DraftRight: Fix Grammar</string>
            </dict>
            <key>NSMessage</key>
            <string>rewriteGrammar</string>
            <key>NSPortName</key>
            <string>DraftRight</string>
            <key>NSSendTypes</key>
            <array><string>NSStringPboardType</string></array>
            <key>NSReturnTypes</key>
            <array><string>NSStringPboardType</string></array>
        </dict>
        <dict>
            <key>NSMenuItem</key>
            <dict>
                <key>default</key>
                <string>DraftRight: Shorter</string>
            </dict>
            <key>NSMessage</key>
            <string>rewriteShorter</string>
            <key>NSPortName</key>
            <string>DraftRight</string>
            <key>NSSendTypes</key>
            <array><string>NSStringPboardType</string></array>
            <key>NSReturnTypes</key>
            <array><string>NSStringPboardType</string></array>
        </dict>
        <dict>
            <key>NSMenuItem</key>
            <dict>
                <key>default</key>
                <string>DraftRight: Longer</string>
            </dict>
            <key>NSMessage</key>
            <string>rewriteLonger</string>
            <key>NSPortName</key>
            <string>DraftRight</string>
            <key>NSSendTypes</key>
            <array><string>NSStringPboardType</string></array>
            <key>NSReturnTypes</key>
            <array><string>NSStringPboardType</string></array>
        </dict>
    </array>
</dict>
</plist>
```

- [ ] **Step 3: Create the SwiftUI app entry point**

Create `DraftRight/DraftRightApp.swift`:

```swift
import SwiftUI
import AppKit

@main
struct DraftRightApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) var appDelegate

    var body: some Scene {
        MenuBarExtra("DraftRight", systemImage: "pencil.and.outline") {
            MenuBarView()
                .environmentObject(appDelegate.appModel)
        }
        Settings {
            SettingsView()
                .environmentObject(appDelegate.appModel)
        }
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    let appModel = AppModel()
    private var serviceProvider: ServiceProvider?

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApplication.shared.setActivationPolicy(.accessory)

        // Register services provider
        serviceProvider = ServiceProvider(appModel: appModel)
        NSApp.servicesProvider = serviceProvider
        NSUpdateDynamicServices()
    }
}
```

- [ ] **Step 4: Create a stub AppModel so it compiles**

Create `DraftRight/AppModel.swift` (minimal stub):

```swift
import Foundation
import SwiftUI

@MainActor
final class AppModel: ObservableObject {
    @Published var apiKey: String = ""
    @Published var endpoint: String = "https://api.openai.com/v1/chat/completions"
    @Published var model: String = "gpt-4o-mini"
    @Published var temperature: Double = 0.3
    @Published var isRewriting: Bool = false
}
```

- [ ] **Step 5: Create stub files so the project compiles**

Create empty stubs for `ServiceProvider.swift`, `MenuBarView.swift`, `SettingsView.swift`:

`DraftRight/Services/ServiceProvider.swift`:
```swift
import AppKit

@MainActor
final class ServiceProvider: NSObject {
    let appModel: AppModel
    init(appModel: AppModel) { self.appModel = appModel }
}
```

`DraftRight/UI/MenuBarView.swift`:
```swift
import SwiftUI

struct MenuBarView: View {
    @EnvironmentObject var appModel: AppModel
    var body: some View {
        Text("DraftRight")
    }
}
```

`DraftRight/UI/SettingsView.swift`:
```swift
import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var appModel: AppModel
    var body: some View {
        Text("Settings")
    }
}
```

- [ ] **Step 6: Create Package.swift for building with SwiftPM**

Create `Package.swift` in the project root (`/opt/openAi/DraftRight/`):

```swift
// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "DraftRight",
    platforms: [.macOS(.v13)],
    targets: [
        .executableTarget(
            name: "DraftRight",
            path: "DraftRight",
            resources: [
                .copy("Info.plist")
            ]
        )
    ]
)
```

- [ ] **Step 7: Verify the project compiles**

```bash
cd /opt/openAi/DraftRight && swift build 2>&1
```

Expected: Build succeeds with no errors.

- [ ] **Step 8: Initialize git and commit**

```bash
cd /opt/openAi/DraftRight
git init
echo ".build/\n.DS_Store\n*.swp" > .gitignore
git add .
git commit -m "feat: scaffold DraftRight project with NSServices declarations and SwiftPM build"
```

---

### Task 2: Tone Enum and Prompt Builder

**Files:**
- Create: `DraftRight/AI/TonePrompts.swift`

- [ ] **Step 1: Create the Tone enum and prompt builder**

Create `DraftRight/AI/TonePrompts.swift`:

```swift
import Foundation

enum Tone: String, CaseIterable, Identifiable {
    case professional
    case casual
    case grammar
    case shorter
    case longer

    var id: String { rawValue }

    var displayName: String {
        switch self {
        case .professional: return "Professional"
        case .casual: return "Casual"
        case .grammar: return "Fix Grammar"
        case .shorter: return "Shorter"
        case .longer: return "Longer"
        }
    }

    var systemPrompt: String {
        switch self {
        case .professional:
            return "Rewrite the following text to be professional, clear, and workplace-appropriate. Preserve the original meaning. Return only the rewritten text, no explanations."
        case .casual:
            return "Rewrite the following text to be friendly and conversational. Preserve the original meaning. Return only the rewritten text, no explanations."
        case .grammar:
            return "Fix grammar, spelling, and punctuation errors in the following text. Do not change the tone or style. Return only the corrected text, no explanations."
        case .shorter:
            return "Condense the following text while preserving the key meaning. Return only the shortened text, no explanations."
        case .longer:
            return "Expand the following text with more detail and context while keeping the same tone. Return only the expanded text, no explanations."
        }
    }
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /opt/openAi/DraftRight && swift build 2>&1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add DraftRight/AI/TonePrompts.swift
git commit -m "feat: add Tone enum with display names and system prompts"
```

---

### Task 3: OpenAI Client

**Files:**
- Create: `DraftRight/AI/OpenAIClient.swift`

- [ ] **Step 1: Create the OpenAI client**

Create `DraftRight/AI/OpenAIClient.swift`:

```swift
import Foundation

struct AIMessage: Codable {
    let role: String
    let content: String
}

struct ChatRequest: Codable {
    let model: String
    let messages: [AIMessage]
    let temperature: Double
    let max_tokens: Int
}

struct ChatResponse: Codable {
    struct Choice: Codable {
        let message: AIMessage
    }
    let choices: [Choice]
}

enum OpenAIClientError: LocalizedError {
    case missingAPIKey
    case invalidEndpoint
    case emptyResponse
    case httpError(Int, String)
    case timeout

    var errorDescription: String? {
        switch self {
        case .missingAPIKey: return "API key not set. Open Settings to configure."
        case .invalidEndpoint: return "Invalid API endpoint URL."
        case .emptyResponse: return "No text returned from API."
        case .httpError(let code, let body): return "HTTP \(code): \(body)"
        case .timeout: return "Request timed out after 10 seconds."
        }
    }
}

final class OpenAIClient {
    private let session: URLSession

    init() {
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 10
        self.session = URLSession(configuration: config)
    }

    func rewrite(text: String, tone: Tone, apiKey: String, endpoint: String, model: String, temperature: Double) async throws -> String {
        guard !apiKey.isEmpty else { throw OpenAIClientError.missingAPIKey }
        guard let url = URL(string: endpoint) else { throw OpenAIClientError.invalidEndpoint }

        let inputText = String(text.prefix(3000))
        let messages = [
            AIMessage(role: "system", content: tone.systemPrompt),
            AIMessage(role: "user", content: inputText)
        ]

        let body = ChatRequest(model: model, messages: messages, temperature: temperature, max_tokens: 1024)
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        request.addValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        request.httpBody = try JSONEncoder().encode(body)

        let (data, response): (Data, URLResponse)
        do {
            (data, response) = try await session.data(for: request)
        } catch let error as URLError where error.code == .timedOut {
            throw OpenAIClientError.timeout
        }

        if let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode >= 400 {
            let bodyText = String(data: data, encoding: .utf8) ?? "Unknown error"
            throw OpenAIClientError.httpError(httpResponse.statusCode, bodyText)
        }

        let decoded = try JSONDecoder().decode(ChatResponse.self, from: data)
        guard let first = decoded.choices.first else { throw OpenAIClientError.emptyResponse }
        return first.message.content.trimmingCharacters(in: .whitespacesAndNewlines)
    }
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /opt/openAi/DraftRight && swift build 2>&1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add DraftRight/AI/OpenAIClient.swift
git commit -m "feat: add OpenAI chat completions client with timeout and error handling"
```

---

### Task 4: Keychain Helper

**Files:**
- Create: `DraftRight/Utils/KeychainHelper.swift`

- [ ] **Step 1: Create the Keychain helper**

Create `DraftRight/Utils/KeychainHelper.swift`:

```swift
import Foundation
import Security

enum KeychainHelper {
    private static let service = "com.draftright.app"
    private static let account = "apiKey"

    @discardableResult
    static func save(_ key: String) -> Bool {
        delete()
        guard let data = key.data(using: .utf8) else { return false }

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlock
        ]

        return SecItemAdd(query as CFDictionary, nil) == errSecSuccess
    }

    static func load() -> String? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne
        ]

        var result: AnyObject?
        guard SecItemCopyMatching(query as CFDictionary, &result) == errSecSuccess,
              let data = result as? Data else { return nil }
        return String(data: data, encoding: .utf8)
    }

    @discardableResult
    static func delete() -> Bool {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account
        ]
        let status = SecItemDelete(query as CFDictionary)
        return status == errSecSuccess || status == errSecItemNotFound
    }
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /opt/openAi/DraftRight && swift build 2>&1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add DraftRight/Utils/KeychainHelper.swift
git commit -m "feat: add Keychain helper for secure API key storage"
```

---

### Task 5: Clipboard Helper

**Files:**
- Create: `DraftRight/Utils/ClipboardHelper.swift`

- [ ] **Step 1: Create the Clipboard helper**

Create `DraftRight/Utils/ClipboardHelper.swift`:

```swift
import AppKit
import Carbon.HIToolbox

enum ClipboardHelper {
    static func copy(text: String) {
        let pasteboard = NSPasteboard.general
        pasteboard.clearContents()
        pasteboard.setString(text, forType: .string)
    }

    static func pasteFromClipboard() {
        let source = CGEventSource(stateID: .hidSystemState)
        let keyDown = CGEvent(keyboardEventSource: source, virtualKey: CGKeyCode(kVK_ANSI_V), keyDown: true)
        keyDown?.flags = .maskCommand
        let keyUp = CGEvent(keyboardEventSource: source, virtualKey: CGKeyCode(kVK_ANSI_V), keyDown: false)
        keyUp?.flags = .maskCommand
        keyDown?.post(tap: .cgSessionEventTap)
        keyUp?.post(tap: .cgSessionEventTap)
    }
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /opt/openAi/DraftRight && swift build 2>&1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add DraftRight/Utils/ClipboardHelper.swift
git commit -m "feat: add clipboard helper with paste simulation"
```

---

### Task 6: Word-Level Diff Algorithm

**Files:**
- Create: `DraftRight/Diff/WordDiff.swift`

- [ ] **Step 1: Create the word-level diff engine**

Create `DraftRight/Diff/WordDiff.swift`:

```swift
import Foundation

enum DiffKind {
    case equal
    case deleted
    case inserted
}

struct DiffToken {
    let text: String
    let kind: DiffKind
}

enum WordDiff {
    /// Computes a word-level diff between `old` and `new` using the LCS (longest common subsequence) algorithm.
    static func diff(old: String, new: String) -> (oldTokens: [DiffToken], newTokens: [DiffToken]) {
        let oldWords = tokenize(old)
        let newWords = tokenize(new)

        let lcs = longestCommonSubsequence(oldWords, newWords)
        var oldTokens: [DiffToken] = []
        var newTokens: [DiffToken] = []

        var oi = 0, ni = 0, li = 0
        while oi < oldWords.count || ni < newWords.count {
            if li < lcs.count {
                // Emit deletions from old until we hit the next LCS word
                while oi < oldWords.count && oldWords[oi] != lcs[li] {
                    oldTokens.append(DiffToken(text: oldWords[oi], kind: .deleted))
                    oi += 1
                }
                // Emit insertions from new until we hit the next LCS word
                while ni < newWords.count && newWords[ni] != lcs[li] {
                    newTokens.append(DiffToken(text: newWords[ni], kind: .inserted))
                    ni += 1
                }
                // Emit the matching word on both sides
                if li < lcs.count {
                    oldTokens.append(DiffToken(text: lcs[li], kind: .equal))
                    newTokens.append(DiffToken(text: lcs[li], kind: .equal))
                    oi += 1
                    ni += 1
                    li += 1
                }
            } else {
                // Past LCS — remaining words are diffs
                while oi < oldWords.count {
                    oldTokens.append(DiffToken(text: oldWords[oi], kind: .deleted))
                    oi += 1
                }
                while ni < newWords.count {
                    newTokens.append(DiffToken(text: newWords[ni], kind: .inserted))
                    ni += 1
                }
            }
        }

        return (oldTokens, newTokens)
    }

    private static func tokenize(_ text: String) -> [String] {
        // Split on whitespace, keeping whitespace tokens for accurate reconstruction
        var tokens: [String] = []
        var current = ""
        for char in text {
            if char.isWhitespace {
                if !current.isEmpty {
                    tokens.append(current)
                    current = ""
                }
                tokens.append(String(char))
            } else {
                current.append(char)
            }
        }
        if !current.isEmpty {
            tokens.append(current)
        }
        return tokens
    }

    private static func longestCommonSubsequence(_ a: [String], _ b: [String]) -> [String] {
        let m = a.count, n = b.count
        var dp = Array(repeating: Array(repeating: 0, count: n + 1), count: m + 1)

        for i in 1...m {
            for j in 1...n {
                if a[i - 1] == b[j - 1] {
                    dp[i][j] = dp[i - 1][j - 1] + 1
                } else {
                    dp[i][j] = max(dp[i - 1][j], dp[i][j - 1])
                }
            }
        }

        var result: [String] = []
        var i = m, j = n
        while i > 0 && j > 0 {
            if a[i - 1] == b[j - 1] {
                result.append(a[i - 1])
                i -= 1
                j -= 1
            } else if dp[i - 1][j] > dp[i][j - 1] {
                i -= 1
            } else {
                j -= 1
            }
        }

        return result.reversed()
    }
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /opt/openAi/DraftRight && swift build 2>&1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add DraftRight/Diff/WordDiff.swift
git commit -m "feat: add word-level diff algorithm using LCS"
```

---

### Task 7: Floating Diff Window

**Files:**
- Create: `DraftRight/UI/DiffWindow.swift`
- Create: `DraftRight/UI/DiffView.swift`

- [ ] **Step 1: Create the DiffView (SwiftUI side-by-side view)**

Create `DraftRight/UI/DiffView.swift`:

```swift
import SwiftUI

struct DiffView: View {
    let tone: Tone
    let original: String
    let rewritten: String
    let onReplace: () -> Void
    let onCopy: () -> Void
    let onCancel: () -> Void
    let onRetry: (() -> Void)?

    @State private var errorMessage: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            // Header
            HStack {
                Image(systemName: "pencil.and.outline")
                    .foregroundColor(.accentColor)
                Text("DraftRight")
                    .font(.headline)
                Text("— \(tone.displayName)")
                    .font(.subheadline)
                    .foregroundColor(.secondary)
                Spacer()
                Button(action: onCancel) {
                    Image(systemName: "xmark")
                        .foregroundColor(.secondary)
                }
                .buttonStyle(.borderless)
            }

            // Side-by-side diff
            HStack(alignment: .top, spacing: 1) {
                // Original (left)
                VStack(alignment: .leading, spacing: 4) {
                    Text("Original")
                        .font(.caption)
                        .foregroundColor(.secondary)
                        .fontWeight(.semibold)
                    ScrollView {
                        diffText(tokens: diffResult.oldTokens, highlightKind: .deleted, color: .red)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .textSelection(.enabled)
                    }
                }
                .frame(maxWidth: .infinity)
                .padding(8)
                .background(Color.red.opacity(0.03))
                .cornerRadius(6)

                // Rewritten (right)
                VStack(alignment: .leading, spacing: 4) {
                    Text("Rewritten")
                        .font(.caption)
                        .foregroundColor(.secondary)
                        .fontWeight(.semibold)
                    ScrollView {
                        diffText(tokens: diffResult.newTokens, highlightKind: .inserted, color: .green)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .textSelection(.enabled)
                    }
                }
                .frame(maxWidth: .infinity)
                .padding(8)
                .background(Color.green.opacity(0.03))
                .cornerRadius(6)
            }

            // Buttons
            HStack {
                Spacer()
                Button("Cancel", action: onCancel)
                    .keyboardShortcut(.cancelAction)
                Button("Copy", action: onCopy)
                Button("Replace", action: onReplace)
                    .keyboardShortcut(.defaultAction)
            }
        }
        .padding(14)
        .background(.ultraThickMaterial, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
    }

    private var diffResult: (oldTokens: [DiffToken], newTokens: [DiffToken]) {
        WordDiff.diff(old: original, new: rewritten)
    }

    private func diffText(tokens: [DiffToken], highlightKind: DiffKind, color: Color) -> Text {
        var result = Text("")
        for token in tokens {
            if token.kind == highlightKind {
                result = result + Text(token.text)
                    .foregroundColor(.white)
                    .bold()
                    .background(color.opacity(0.6))
            } else if token.kind == .equal {
                result = result + Text(token.text)
            }
            // Skip tokens of the opposite diff kind (they belong to the other panel)
        }
        return result
    }
}
```

- [ ] **Step 2: Create the DiffWindow (NSPanel host)**

Create `DraftRight/UI/DiffWindow.swift`:

```swift
import SwiftUI
import AppKit
import Carbon.HIToolbox

@MainActor
final class DiffWindow {
    static let shared = DiffWindow()

    private var window: NSPanel?
    private var clickMonitor: Any?
    private var keyMonitor: Any?

    func present(
        tone: Tone,
        original: String,
        rewritten: String,
        replaceHandler: @escaping () -> Void,
        copyHandler: @escaping () -> Void
    ) {
        close()

        let content = DiffView(
            tone: tone,
            original: original,
            rewritten: rewritten,
            onReplace: {
                replaceHandler()
                self.close()
            },
            onCopy: {
                copyHandler()
                self.close()
            },
            onCancel: { self.close() },
            onRetry: nil
        )

        let hosting = NSHostingView(rootView: content)
        let size = CGSize(width: 600, height: 400)
        hosting.frame = CGRect(origin: .zero, size: size)

        let cursor = NSEvent.mouseLocation
        let origin = CGPoint(
            x: max(0, cursor.x - 10),
            y: max(0, cursor.y - size.height - 20)
        )

        let panel = NSPanel(
            contentRect: NSRect(origin: origin, size: size),
            styleMask: [.borderless, .nonactivatingPanel],
            backing: .buffered,
            defer: false
        )
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.level = .floating
        panel.hasShadow = true
        panel.isReleasedWhenClosed = false
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
        panel.contentView = hosting
        panel.isMovableByWindowBackground = true
        panel.makeKeyAndOrderFront(nil)
        self.window = panel

        clickMonitor = NSEvent.addGlobalMonitorForEvents(matching: [.leftMouseDown, .rightMouseDown]) { [weak self] _ in
            self?.close()
        }
        keyMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
            if event.keyCode == UInt16(kVK_Escape) {
                self?.close()
                return nil
            }
            return event
        }
    }

    func showLoading(tone: Tone) {
        close()

        let content = VStack(spacing: 12) {
            ProgressView()
                .scaleEffect(0.8)
            Text("Rewriting as \(tone.displayName)...")
                .font(.subheadline)
                .foregroundColor(.secondary)
        }
        .padding(24)
        .background(.ultraThickMaterial, in: RoundedRectangle(cornerRadius: 12, style: .continuous))

        let hosting = NSHostingView(rootView: content)
        let size = CGSize(width: 240, height: 80)
        hosting.frame = CGRect(origin: .zero, size: size)

        let cursor = NSEvent.mouseLocation
        let origin = CGPoint(x: max(0, cursor.x - 10), y: max(0, cursor.y - size.height - 20))

        let panel = NSPanel(
            contentRect: NSRect(origin: origin, size: size),
            styleMask: [.borderless, .nonactivatingPanel],
            backing: .buffered,
            defer: false
        )
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.level = .floating
        panel.hasShadow = true
        panel.isReleasedWhenClosed = false
        panel.contentView = hosting
        panel.makeKeyAndOrderFront(nil)
        self.window = panel
    }

    func close() {
        if let clickMonitor {
            NSEvent.removeMonitor(clickMonitor)
            self.clickMonitor = nil
        }
        if let keyMonitor {
            NSEvent.removeMonitor(keyMonitor)
            self.keyMonitor = nil
        }
        window?.orderOut(nil)
        window = nil
    }
}
```

- [ ] **Step 3: Verify it compiles**

```bash
cd /opt/openAi/DraftRight && swift build 2>&1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add DraftRight/UI/DiffView.swift DraftRight/UI/DiffWindow.swift
git commit -m "feat: add floating diff window with side-by-side word-level diff"
```

---

### Task 8: Service Provider (Core Services Handler)

**Files:**
- Modify: `DraftRight/Services/ServiceProvider.swift`

- [ ] **Step 1: Implement the full ServiceProvider**

Replace the stub `DraftRight/Services/ServiceProvider.swift` with:

```swift
import AppKit
import UserNotifications

@MainActor
final class ServiceProvider: NSObject {
    let appModel: AppModel

    private let aiClient = OpenAIClient()
    private let diffWindow = DiffWindow.shared

    init(appModel: AppModel) {
        self.appModel = appModel
        super.init()
    }

    // MARK: - Service selectors (one per tone, matching Info.plist NSMessage values)

    @objc func rewriteProfessional(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .professional, error: error)
    }

    @objc func rewriteCasual(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .casual, error: error)
    }

    @objc func rewriteGrammar(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .grammar, error: error)
    }

    @objc func rewriteShorter(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .shorter, error: error)
    }

    @objc func rewriteLonger(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .longer, error: error)
    }

    // MARK: - Core rewrite logic

    private func handleRewrite(pasteboard: NSPasteboard, tone: Tone, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        guard let text = pasteboard.string(forType: .string), !text.isEmpty else {
            error.pointee = "No text provided." as NSString
            return
        }

        guard !appModel.apiKey.isEmpty else {
            showNotification("API key not set. Open DraftRight settings to configure.")
            return
        }

        appModel.isRewriting = true
        diffWindow.showLoading(tone: tone)

        Task {
            do {
                let rewritten = try await aiClient.rewrite(
                    text: text,
                    tone: tone,
                    apiKey: appModel.apiKey,
                    endpoint: appModel.endpoint,
                    model: appModel.model,
                    temperature: appModel.temperature
                )

                diffWindow.present(
                    tone: tone,
                    original: text,
                    rewritten: rewritten,
                    replaceHandler: {
                        ClipboardHelper.copy(text: rewritten)
                        // Small delay to ensure clipboard is ready
                        DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
                            ClipboardHelper.pasteFromClipboard()
                        }
                    },
                    copyHandler: {
                        ClipboardHelper.copy(text: rewritten)
                    }
                )
            } catch {
                diffWindow.close()
                showNotification("Rewrite failed: \(error.localizedDescription)")
            }

            appModel.isRewriting = false
        }
    }

    private func showNotification(_ message: String) {
        let center = UNUserNotificationCenter.current()
        center.requestAuthorization(options: [.alert, .sound]) { _, _ in }
        let content = UNMutableNotificationContent()
        content.title = "DraftRight"
        content.body = message
        let request = UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil)
        center.add(request, withCompletionHandler: nil)
    }
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /opt/openAi/DraftRight && swift build 2>&1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add DraftRight/Services/ServiceProvider.swift
git commit -m "feat: implement service provider handling 5 tone-based rewrites"
```

---

### Task 9: AppModel (Full Implementation)

**Files:**
- Modify: `DraftRight/AppModel.swift`

- [ ] **Step 1: Implement full AppModel with settings persistence**

Replace `DraftRight/AppModel.swift` with:

```swift
import Foundation
import SwiftUI

@MainActor
final class AppModel: ObservableObject {
    @Published var apiKey: String {
        didSet { KeychainHelper.save(apiKey) }
    }
    @Published var endpoint: String {
        didSet { defaults.set(endpoint, forKey: Keys.endpoint) }
    }
    @Published var model: String {
        didSet { defaults.set(model, forKey: Keys.model) }
    }
    @Published var temperature: Double {
        didSet { defaults.set(temperature, forKey: Keys.temperature) }
    }
    @Published var launchAtLogin: Bool {
        didSet { defaults.set(launchAtLogin, forKey: Keys.launchAtLogin) }
    }
    @Published var isRewriting: Bool = false

    private let defaults = UserDefaults.standard

    private enum Keys {
        static let endpoint = "draftright.endpoint"
        static let model = "draftright.model"
        static let temperature = "draftright.temperature"
        static let launchAtLogin = "draftright.launchAtLogin"
    }

    init() {
        self.apiKey = KeychainHelper.load() ?? ""
        self.endpoint = defaults.string(forKey: Keys.endpoint) ?? "https://api.openai.com/v1/chat/completions"
        self.model = defaults.string(forKey: Keys.model) ?? "gpt-4o-mini"
        self.temperature = defaults.object(forKey: Keys.temperature) as? Double ?? 0.3
        self.launchAtLogin = defaults.bool(forKey: Keys.launchAtLogin)
    }
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /opt/openAi/DraftRight && swift build 2>&1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add DraftRight/AppModel.swift
git commit -m "feat: implement AppModel with Keychain and UserDefaults persistence"
```

---

### Task 10: Menu Bar View

**Files:**
- Modify: `DraftRight/UI/MenuBarView.swift`

- [ ] **Step 1: Implement the menu bar dropdown**

Replace `DraftRight/UI/MenuBarView.swift` with:

```swift
import SwiftUI

struct MenuBarView: View {
    @EnvironmentObject var appModel: AppModel

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Circle()
                    .fill(appModel.isRewriting ? Color.orange : Color.green)
                    .frame(width: 8, height: 8)
                Text(appModel.isRewriting ? "Rewriting..." : "Ready")
                    .font(.subheadline)
                    .foregroundColor(.secondary)
            }

            Divider()

            Text("Right-click selected text and look")
                .font(.caption)
                .foregroundColor(.secondary)
            Text("under Services for DraftRight options.")
                .font(.caption)
                .foregroundColor(.secondary)

            Divider()

            Button("Settings...") {
                NSApp.sendAction(Selector(("showSettingsWindow:")), to: nil, from: nil)
                NSApp.activate(ignoringOtherApps: true)
            }
            Button("Quit DraftRight") {
                NSApp.terminate(nil)
            }
        }
        .padding(12)
        .frame(width: 280)
    }
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /opt/openAi/DraftRight && swift build 2>&1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add DraftRight/UI/MenuBarView.swift
git commit -m "feat: implement menu bar view with status indicator"
```

---

### Task 11: Settings View

**Files:**
- Modify: `DraftRight/UI/SettingsView.swift`

- [ ] **Step 1: Implement the settings window**

Replace `DraftRight/UI/SettingsView.swift` with:

```swift
import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var appModel: AppModel
    @State private var tempApiKey: String = ""

    var body: some View {
        Form {
            Section(header: Text("OpenAI API")) {
                SecureField("API Key", text: Binding(
                    get: { tempApiKey },
                    set: {
                        tempApiKey = $0
                        appModel.apiKey = $0
                    }
                ))
                .help("Stored securely in macOS Keychain")

                TextField("Endpoint", text: $appModel.endpoint)

                TextField("Model", text: $appModel.model)

                HStack {
                    Text("Temperature")
                    Slider(value: $appModel.temperature, in: 0...1)
                    Text(String(format: "%.2f", appModel.temperature))
                        .font(.footnote)
                        .foregroundColor(.secondary)
                        .frame(width: 30)
                }
            }

            Section(header: Text("General")) {
                Toggle("Launch at Login", isOn: $appModel.launchAtLogin)
            }

            Section(header: Text("Services")) {
                Text("After launching DraftRight, the rewrite options appear in the right-click → Services menu of any app.")
                    .font(.caption)
                    .foregroundColor(.secondary)

                Button("Refresh Services") {
                    NSUpdateDynamicServices()
                }
                .help("Force macOS to re-scan available services")
            }
        }
        .padding(12)
        .frame(width: 480)
        .onAppear {
            tempApiKey = appModel.apiKey
        }
    }
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /opt/openAi/DraftRight && swift build 2>&1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add DraftRight/UI/SettingsView.swift
git commit -m "feat: implement settings view with API config and services refresh"
```

---

### Task 12: Final App Entry Point + README

**Files:**
- Modify: `DraftRight/DraftRightApp.swift`
- Create: `README.md`

- [ ] **Step 1: Finalize the app entry point**

The app entry point from Task 1 should already be correct. Verify it references all real types now (no more stubs). If `DraftRightApp.swift` still has the stub version, replace it with the final version from Task 1 Step 3.

- [ ] **Step 2: Create README.md**

Create `README.md` at the project root:

```markdown
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
```

- [ ] **Step 3: Full build verification**

```bash
cd /opt/openAi/DraftRight && swift build 2>&1
```

Expected: Build succeeds with no errors.

- [ ] **Step 4: Commit**

```bash
git add README.md DraftRight/DraftRightApp.swift
git commit -m "feat: finalize app entry point and add README"
```

---

### Task 13: Manual Testing Checklist

This is not a code task — it's a verification checklist to run after building.

- [ ] **Step 1: Build and launch**

```bash
cd /opt/openAi/DraftRight && swift build -c release && .build/release/DraftRight &
```

Verify: pencil icon appears in menu bar.

- [ ] **Step 2: Check menu bar dropdown**

Click the pencil icon. Verify: shows "Ready" status, "Settings..." and "Quit" buttons.

- [ ] **Step 3: Configure API key**

Open Settings, enter your OpenAI API key. Verify: field accepts input.

- [ ] **Step 4: Test Services registration**

Open TextEdit, type some text, select it, right-click. Verify: Services submenu contains "DraftRight: Professional", "DraftRight: Casual", etc.

If services don't appear: run `/System/Library/CoreServices/pbs -update` in Terminal, then try again.

- [ ] **Step 5: Test a rewrite**

Select text in TextEdit → right-click → Services → DraftRight: Fix Grammar. Verify: loading spinner appears, then side-by-side diff window shows.

- [ ] **Step 6: Test Replace and Copy buttons**

Click "Replace" — verify text is pasted back. Try "Copy" — verify text is on clipboard. Try "Cancel" — verify window dismisses.

- [ ] **Step 7: Test in other apps**

Repeat the rewrite test in: Safari (any text field), Claude Desktop (chat input), Teams (compose box). Verify services appear and rewrite works.
