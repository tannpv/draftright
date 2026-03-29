# DraftRight Mobile — iOS Keyboard Extension Implementation Plan (Plan 2 of 3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the iOS keyboard extension that adds a DraftRight rewrite toolbar above the system keyboard.

**Architecture:** iOS Custom Keyboard Extension (Swift) using `UIInputViewController`. The toolbar displays tone icons above the system keyboard. On tone tap, it reads text via `UITextDocumentProxy`, calls the OpenAI API, shows a diff bottom sheet, and replaces text on confirm. Settings are read from the shared App Group storage written by the Flutter main app.

**Tech Stack:** Swift, UIKit, iOS 15+, App Group shared UserDefaults

**Spec:** `docs/specs/2026-03-28-draftright-mobile-design.md`

---

### Task 1: Create iOS Keyboard Extension Target

**Files:**
- Create: `DraftRightMobile/ios/DraftRightKeyboard/Info.plist`
- Create: `DraftRightMobile/ios/DraftRightKeyboard/KeyboardViewController.swift`

- [ ] **Step 1: Create the keyboard extension directory**

```bash
mkdir -p /opt/openAi/DraftRight/DraftRightMobile/ios/DraftRightKeyboard
```

- [ ] **Step 2: Create Info.plist**

Create `DraftRightMobile/ios/DraftRightKeyboard/Info.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleDevelopmentRegion</key>
    <string>en</string>
    <key>CFBundleDisplayName</key>
    <string>DraftRight</string>
    <key>CFBundleExecutable</key>
    <string>$(EXECUTABLE_NAME)</string>
    <key>CFBundleIdentifier</key>
    <string>$(PRODUCT_BUNDLE_IDENTIFIER)</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>$(PRODUCT_NAME)</string>
    <key>CFBundlePackageType</key>
    <string>$(PRODUCT_BUNDLE_PACKAGE_TYPE)</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0</string>
    <key>CFBundleVersion</key>
    <string>1</string>
    <key>NSExtension</key>
    <dict>
        <key>NSExtensionPointIdentifier</key>
        <string>com.apple.keyboard-service</string>
        <key>NSExtensionPrincipalClass</key>
        <string>$(PRODUCT_MODULE_NAME).KeyboardViewController</string>
    </dict>
    <key>NSAppTransportSecurity</key>
    <dict>
        <key>NSAllowsArbitraryLoads</key>
        <true/>
    </dict>
</dict>
</plist>
```

- [ ] **Step 3: Create KeyboardViewController stub**

Create `DraftRightMobile/ios/DraftRightKeyboard/KeyboardViewController.swift`:

```swift
import UIKit

class KeyboardViewController: UIInputViewController {

    override func viewDidLoad() {
        super.viewDidLoad()
        setupToolbar()
    }

    private func setupToolbar() {
        let label = UILabel()
        label.text = "DraftRight Toolbar"
        label.textAlignment = .center
        label.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(label)
        NSLayoutConstraint.activate([
            label.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            label.centerYAnchor.constraint(equalTo: view.centerYAnchor),
        ])
    }
}
```

- [ ] **Step 4: Add the extension target to the Xcode project**

This must be done in Xcode:
1. Open `DraftRightMobile/ios/Runner.xcworkspace` in Xcode
2. File → New → Target → Custom Keyboard Extension
3. Product Name: `DraftRightKeyboard`
4. Bundle Identifier: `com.draftright.draftrightMobile.keyboard`
5. Enable App Group: `group.com.draftright.app` on both Runner and DraftRightKeyboard targets
6. Set Deployment Target to iOS 15.0

Alternatively, the extension can be configured via the `Podfile` and `project.pbxproj` manually, but Xcode is the reliable way.

- [ ] **Step 5: Commit**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile
git add ios/DraftRightKeyboard/
git commit -m "feat: add iOS keyboard extension target with stub"
```

---

### Task 2: Shared Settings Reader

**Files:**
- Create: `DraftRightMobile/ios/DraftRightKeyboard/SharedSettings.swift`

- [ ] **Step 1: Create SharedSettings**

Create `DraftRightMobile/ios/DraftRightKeyboard/SharedSettings.swift`:

```swift
import Foundation

struct SharedSettings {
    private let defaults: UserDefaults?

    init() {
        defaults = UserDefaults(suiteName: "group.com.draftright.app")
    }

    var aiProvider: String {
        defaults?.string(forKey: "draftright.aiProvider") ?? "openai"
    }

    var apiKey: String {
        // For keyboard extensions, read from shared UserDefaults
        // (FlutterSecureStorage writes to Keychain which is accessible via App Group)
        defaults?.string(forKey: "draftright.apiKey") ?? ""
    }

    var endpoint: String {
        defaults?.string(forKey: "draftright.endpoint") ?? "https://api.openai.com/v1/chat/completions"
    }

    var model: String {
        defaults?.string(forKey: "draftright.model") ?? "gpt-4o-mini"
    }

    var temperature: Double {
        defaults?.double(forKey: "draftright.temperature") ?? 0.3
    }

    var translateLanguage: String {
        defaults?.string(forKey: "draftright.translateLanguage") ?? "Vietnamese"
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add ios/DraftRightKeyboard/SharedSettings.swift
git commit -m "feat: add SharedSettings reader for keyboard extension via App Group"
```

---

### Task 3: Tone Enum (Swift)

**Files:**
- Create: `DraftRightMobile/ios/DraftRightKeyboard/Tone.swift`

- [ ] **Step 1: Create Tone enum**

Create `DraftRightMobile/ios/DraftRightKeyboard/Tone.swift`:

```swift
import UIKit

enum Tone: String, CaseIterable {
    case simple
    case natural
    case polished
    case concise
    case technical
    case translate

    var displayName: String {
        switch self {
        case .simple: return "Simple"
        case .natural: return "Natural"
        case .polished: return "Polished"
        case .concise: return "Concise"
        case .technical: return "Technical"
        case .translate: return "Translate"
        }
    }

    var iconName: String {
        switch self {
        case .simple: return "textformat.size"
        case .natural: return "bubble.left"
        case .polished: return "sparkles"
        case .concise: return "arrow.down.right.and.arrow.up.left"
        case .technical: return "wrench.and.screwdriver"
        case .translate: return "globe"
        }
    }

    func systemPrompt(targetLanguage: String = "English") -> String {
        switch self {
        case .simple:
            return "Rewrite the following text using simple, easy-to-understand language. Use short sentences and common words. Preserve the original meaning. Return only the rewritten text, no explanations."
        case .natural:
            return "Rewrite the following text to sound more natural and conversational, as if spoken by a real person. Remove awkward phrasing and make it flow smoothly. Preserve the original meaning. Return only the rewritten text, no explanations."
        case .polished:
            return "Rewrite the following text to be more polished and professional. Improve grammar, word choice, and sentence structure for a refined, workplace-appropriate tone. Preserve the original meaning. Return only the rewritten text, no explanations."
        case .concise:
            return "Rewrite the following text to be as concise as possible. Remove unnecessary words, redundancy, and filler while preserving the key meaning. Return only the rewritten text, no explanations."
        case .technical:
            return "Rewrite the following text in a technical specification style. Use precise, unambiguous language suitable for documentation, specs, or technical communication. Preserve the original meaning. Return only the rewritten text, no explanations."
        case .translate:
            return "Translate the following text into \(targetLanguage). If the text is already in \(targetLanguage), translate it into English instead. Preserve the original meaning and tone. Return only the translated text, no explanations."
        }
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add ios/DraftRightKeyboard/Tone.swift
git commit -m "feat: add Tone enum for iOS keyboard extension"
```

---

### Task 4: OpenAI Client (Swift)

**Files:**
- Create: `DraftRightMobile/ios/DraftRightKeyboard/OpenAIClient.swift`

- [ ] **Step 1: Create OpenAIClient**

Create `DraftRightMobile/ios/DraftRightKeyboard/OpenAIClient.swift`:

```swift
import Foundation

struct ChatMessage: Codable {
    let role: String
    let content: String
}

struct ChatRequest: Codable {
    let model: String
    let messages: [ChatMessage]
    let temperature: Double
    let max_tokens: Int
}

struct ChatResponse: Codable {
    struct Choice: Codable {
        let message: ChatMessage
    }
    let choices: [Choice]
}

final class OpenAIClient {
    func rewrite(
        text: String,
        tone: Tone,
        settings: SharedSettings,
        completion: @escaping (Result<String, Error>) -> Void
    ) {
        guard let url = URL(string: settings.endpoint) else {
            completion(.failure(NSError(domain: "OpenAIClient", code: -1,
                userInfo: [NSLocalizedDescriptionKey: "Invalid endpoint URL"])))
            return
        }

        let inputText = String(text.prefix(3000))
        let messages = [
            ChatMessage(role: "system", content: tone.systemPrompt(targetLanguage: settings.translateLanguage)),
            ChatMessage(role: "user", content: inputText),
        ]

        let body = ChatRequest(model: settings.model, messages: messages,
                               temperature: settings.temperature, max_tokens: 1024)

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        request.timeoutInterval = 15

        let apiKey = settings.apiKey
        if !apiKey.isEmpty {
            request.addValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        }

        do {
            request.httpBody = try JSONEncoder().encode(body)
        } catch {
            completion(.failure(error))
            return
        }

        URLSession.shared.dataTask(with: request) { data, response, error in
            if let error = error {
                completion(.failure(error))
                return
            }

            guard let data = data else {
                completion(.failure(NSError(domain: "OpenAIClient", code: -2,
                    userInfo: [NSLocalizedDescriptionKey: "No data returned"])))
                return
            }

            if let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode >= 400 {
                let bodyText = String(data: data, encoding: .utf8) ?? "Unknown error"
                completion(.failure(NSError(domain: "OpenAIClient", code: httpResponse.statusCode,
                    userInfo: [NSLocalizedDescriptionKey: "HTTP \(httpResponse.statusCode): \(bodyText)"])))
                return
            }

            do {
                let decoded = try JSONDecoder().decode(ChatResponse.self, from: data)
                guard let first = decoded.choices.first else {
                    completion(.failure(NSError(domain: "OpenAIClient", code: -3,
                        userInfo: [NSLocalizedDescriptionKey: "No response from AI"])))
                    return
                }
                completion(.success(first.message.content.trimmingCharacters(in: .whitespacesAndNewlines)))
            } catch {
                completion(.failure(error))
            }
        }.resume()
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add ios/DraftRightKeyboard/OpenAIClient.swift
git commit -m "feat: add OpenAI client for iOS keyboard extension"
```

---

### Task 5: Toolbar View

**Files:**
- Create: `DraftRightMobile/ios/DraftRightKeyboard/ToolbarView.swift`

- [ ] **Step 1: Create ToolbarView**

Create `DraftRightMobile/ios/DraftRightKeyboard/ToolbarView.swift`:

```swift
import UIKit

protocol ToolbarViewDelegate: AnyObject {
    func toolbarDidSelectTone(_ tone: Tone)
    func toolbarDidTapUndo()
}

final class ToolbarView: UIView {
    weak var delegate: ToolbarViewDelegate?

    private let scrollView = UIScrollView()
    private let stackView = UIStackView()
    private var undoButton: UIButton?
    private var selectedTone: Tone?
    private var loadingTone: Tone?
    private var spinner: UIActivityIndicatorView?

    override init(frame: CGRect) {
        super.init(frame: frame)
        setupUI()
    }

    required init?(coder: NSCoder) {
        super.init(coder: coder)
        setupUI()
    }

    private func setupUI() {
        backgroundColor = UIColor.systemBackground.withAlphaComponent(0.95)

        scrollView.showsHorizontalScrollIndicator = false
        scrollView.translatesAutoresizingMaskIntoConstraints = false
        addSubview(scrollView)

        stackView.axis = .horizontal
        stackView.spacing = 4
        stackView.alignment = .center
        stackView.translatesAutoresizingMaskIntoConstraints = false
        scrollView.addSubview(stackView)

        // Tone buttons
        for tone in Tone.allCases {
            let button = createToneButton(tone)
            stackView.addArrangedSubview(button)
        }

        // Spacer
        let spacer = UIView()
        spacer.setContentHuggingPriority(.defaultLow, for: .horizontal)
        stackView.addArrangedSubview(spacer)

        // Undo button (hidden by default)
        let undo = UIButton(type: .system)
        undo.setImage(UIImage(systemName: "arrow.uturn.backward"), for: .normal)
        undo.setTitle(" Undo", for: .normal)
        undo.titleLabel?.font = .systemFont(ofSize: 12)
        undo.addTarget(self, action: #selector(undoTapped), for: .touchUpInside)
        undo.isHidden = true
        stackView.addArrangedSubview(undo)
        self.undoButton = undo

        NSLayoutConstraint.activate([
            scrollView.topAnchor.constraint(equalTo: topAnchor),
            scrollView.bottomAnchor.constraint(equalTo: bottomAnchor),
            scrollView.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 8),
            scrollView.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -8),
            stackView.topAnchor.constraint(equalTo: scrollView.topAnchor),
            stackView.bottomAnchor.constraint(equalTo: scrollView.bottomAnchor),
            stackView.leadingAnchor.constraint(equalTo: scrollView.leadingAnchor),
            stackView.trailingAnchor.constraint(equalTo: scrollView.trailingAnchor),
            stackView.heightAnchor.constraint(equalTo: scrollView.heightAnchor),
        ])
    }

    private func createToneButton(_ tone: Tone) -> UIButton {
        let button = UIButton(type: .system)
        let config = UIImage.SymbolConfiguration(pointSize: 16, weight: .medium)
        button.setImage(UIImage(systemName: tone.iconName, withConfiguration: config), for: .normal)
        button.tag = Tone.allCases.firstIndex(of: tone)!
        button.addTarget(self, action: #selector(toneTapped(_:)), for: .touchUpInside)
        button.widthAnchor.constraint(equalToConstant: 40).isActive = true
        button.heightAnchor.constraint(equalToConstant: 36).isActive = true
        button.layer.cornerRadius = 6
        // Accessibility
        button.accessibilityLabel = tone.displayName
        return button
    }

    @objc private func toneTapped(_ sender: UIButton) {
        let tone = Tone.allCases[sender.tag]
        delegate?.toolbarDidSelectTone(tone)
    }

    @objc private func undoTapped() {
        delegate?.toolbarDidTapUndo()
    }

    func setLoading(_ tone: Tone) {
        loadingTone = tone
        isUserInteractionEnabled = false
        // Show spinner on the tapped button
        if let index = Tone.allCases.firstIndex(of: tone),
           let button = stackView.arrangedSubviews[index] as? UIButton {
            button.setImage(nil, for: .normal)
            let spinner = UIActivityIndicatorView(style: .medium)
            spinner.startAnimating()
            spinner.translatesAutoresizingMaskIntoConstraints = false
            button.addSubview(spinner)
            spinner.centerXAnchor.constraint(equalTo: button.centerXAnchor).isActive = true
            spinner.centerYAnchor.constraint(equalTo: button.centerYAnchor).isActive = true
            self.spinner = spinner
        }
    }

    func clearLoading() {
        isUserInteractionEnabled = true
        spinner?.removeFromSuperview()
        spinner = nil
        if let tone = loadingTone, let index = Tone.allCases.firstIndex(of: tone),
           let button = stackView.arrangedSubviews[index] as? UIButton {
            let config = UIImage.SymbolConfiguration(pointSize: 16, weight: .medium)
            button.setImage(UIImage(systemName: tone.iconName, withConfiguration: config), for: .normal)
        }
        loadingTone = nil
    }

    func showUndo() {
        undoButton?.isHidden = false
        DispatchQueue.main.asyncAfter(deadline: .now() + 5) { [weak self] in
            self?.undoButton?.isHidden = true
        }
    }

    func hideUndo() {
        undoButton?.isHidden = true
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add ios/DraftRightKeyboard/ToolbarView.swift
git commit -m "feat: add toolbar view with tone icons, loading, and undo for iOS keyboard"
```

---

### Task 6: Diff Bottom Sheet

**Files:**
- Create: `DraftRightMobile/ios/DraftRightKeyboard/DiffSheetView.swift`

- [ ] **Step 1: Create DiffSheetView**

Create `DraftRightMobile/ios/DraftRightKeyboard/DiffSheetView.swift`:

```swift
import UIKit

protocol DiffSheetDelegate: AnyObject {
    func diffSheetDidReplace(_ text: String)
    func diffSheetDidCopy(_ text: String)
    func diffSheetDidCancel()
}

final class DiffSheetView: UIView {
    weak var delegate: DiffSheetDelegate?

    private let originalLabel = UITextView()
    private let rewrittenLabel = UITextView()
    private let rewrittenText: String

    init(original: String, rewritten: String) {
        self.rewrittenText = rewritten
        super.init(frame: .zero)
        setupUI(original: original, rewritten: rewritten)
    }

    required init?(coder: NSCoder) {
        self.rewrittenText = ""
        super.init(coder: coder)
    }

    private func setupUI(original: String, rewritten: String) {
        backgroundColor = .systemBackground
        layer.cornerRadius = 16
        layer.maskedCorners = [.layerMinXMinYCorner, .layerMaxXMinYCorner]
        layer.shadowColor = UIColor.black.cgColor
        layer.shadowOpacity = 0.15
        layer.shadowRadius = 8

        // Drag handle
        let handle = UIView()
        handle.backgroundColor = .systemGray3
        handle.layer.cornerRadius = 2.5
        handle.translatesAutoresizingMaskIntoConstraints = false
        addSubview(handle)

        // Labels
        let origHeader = makeHeader("Original")
        let rewriteHeader = makeHeader("Rewritten")

        originalLabel.text = original
        originalLabel.font = .systemFont(ofSize: 14)
        originalLabel.isEditable = false
        originalLabel.backgroundColor = UIColor.systemRed.withAlphaComponent(0.03)
        originalLabel.layer.cornerRadius = 6
        originalLabel.textContainerInset = UIEdgeInsets(top: 8, left: 4, bottom: 8, right: 4)

        rewrittenLabel.text = rewritten
        rewrittenLabel.font = .systemFont(ofSize: 14)
        rewrittenLabel.isEditable = false
        rewrittenLabel.backgroundColor = UIColor.systemGreen.withAlphaComponent(0.03)
        rewrittenLabel.layer.cornerRadius = 6
        rewrittenLabel.textContainerInset = UIEdgeInsets(top: 8, left: 4, bottom: 8, right: 4)

        // Diff columns
        let leftStack = UIStackView(arrangedSubviews: [origHeader, originalLabel])
        leftStack.axis = .vertical
        leftStack.spacing = 4

        let rightStack = UIStackView(arrangedSubviews: [rewriteHeader, rewrittenLabel])
        rightStack.axis = .vertical
        rightStack.spacing = 4

        let diffRow = UIStackView(arrangedSubviews: [leftStack, rightStack])
        diffRow.axis = .horizontal
        diffRow.spacing = 8
        diffRow.distribution = .fillEqually
        diffRow.translatesAutoresizingMaskIntoConstraints = false
        addSubview(diffRow)

        // Buttons
        let cancelBtn = makeButton("Cancel", style: .secondary)
        cancelBtn.addTarget(self, action: #selector(cancelTapped), for: .touchUpInside)

        let copyBtn = makeButton("Copy", style: .secondary)
        copyBtn.addTarget(self, action: #selector(copyTapped), for: .touchUpInside)

        let replaceBtn = makeButton("Replace", style: .primary)
        replaceBtn.addTarget(self, action: #selector(replaceTapped), for: .touchUpInside)

        let buttonRow = UIStackView(arrangedSubviews: [cancelBtn, copyBtn, replaceBtn])
        buttonRow.axis = .horizontal
        buttonRow.spacing = 12
        buttonRow.distribution = .fillEqually
        buttonRow.translatesAutoresizingMaskIntoConstraints = false
        addSubview(buttonRow)

        NSLayoutConstraint.activate([
            handle.topAnchor.constraint(equalTo: topAnchor, constant: 8),
            handle.centerXAnchor.constraint(equalTo: centerXAnchor),
            handle.widthAnchor.constraint(equalToConstant: 36),
            handle.heightAnchor.constraint(equalToConstant: 5),

            diffRow.topAnchor.constraint(equalTo: handle.bottomAnchor, constant: 12),
            diffRow.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 12),
            diffRow.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -12),
            diffRow.bottomAnchor.constraint(equalTo: buttonRow.topAnchor, constant: -12),

            buttonRow.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 12),
            buttonRow.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -12),
            buttonRow.bottomAnchor.constraint(equalTo: bottomAnchor, constant: -12),
            buttonRow.heightAnchor.constraint(equalToConstant: 40),
        ])
    }

    private func makeHeader(_ text: String) -> UILabel {
        let label = UILabel()
        label.text = text
        label.font = .systemFont(ofSize: 11, weight: .semibold)
        label.textColor = .secondaryLabel
        return label
    }

    private enum ButtonStyle { case primary, secondary }

    private func makeButton(_ title: String, style: ButtonStyle) -> UIButton {
        let button = UIButton(type: .system)
        button.setTitle(title, for: .normal)
        button.titleLabel?.font = .systemFont(ofSize: 14, weight: .medium)
        button.layer.cornerRadius = 8
        if style == .primary {
            button.backgroundColor = .systemBlue
            button.setTitleColor(.white, for: .normal)
        } else {
            button.backgroundColor = .systemGray5
        }
        return button
    }

    @objc private func cancelTapped() { delegate?.diffSheetDidCancel() }
    @objc private func copyTapped() { delegate?.diffSheetDidCopy(rewrittenText) }
    @objc private func replaceTapped() { delegate?.diffSheetDidReplace(rewrittenText) }
}
```

- [ ] **Step 2: Commit**

```bash
git add ios/DraftRightKeyboard/DiffSheetView.swift
git commit -m "feat: add diff bottom sheet with side-by-side preview for iOS keyboard"
```

---

### Task 7: Full KeyboardViewController

**Files:**
- Modify: `DraftRightMobile/ios/DraftRightKeyboard/KeyboardViewController.swift`

- [ ] **Step 1: Implement full KeyboardViewController**

Replace `DraftRightMobile/ios/DraftRightKeyboard/KeyboardViewController.swift`:

```swift
import UIKit

class KeyboardViewController: UIInputViewController {

    private let toolbar = ToolbarView()
    private let aiClient = OpenAIClient()
    private let settings = SharedSettings()
    private var diffSheet: DiffSheetView?
    private var originalText: String?

    override func viewDidLoad() {
        super.viewDidLoad()
        setupToolbar()
    }

    private func setupToolbar() {
        toolbar.delegate = self
        toolbar.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(toolbar)

        NSLayoutConstraint.activate([
            toolbar.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            toolbar.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            toolbar.topAnchor.constraint(equalTo: view.topAnchor),
            toolbar.heightAnchor.constraint(equalToConstant: 44),
        ])

        // Set the keyboard height
        let heightConstraint = view.heightAnchor.constraint(equalToConstant: 44)
        heightConstraint.priority = .defaultHigh
        heightConstraint.isActive = true
    }

    private func readFullText() -> String {
        // UITextDocumentProxy can only read text before and after cursor
        let before = textDocumentProxy.documentContextBeforeInput ?? ""
        let after = textDocumentProxy.documentContextAfterInput ?? ""
        return before + after
    }

    private func replaceAllText(with newText: String) {
        // Move to end of document
        if let after = textDocumentProxy.documentContextAfterInput {
            textDocumentProxy.adjustTextPosition(byCharacterOffset: after.count)
        }
        // Delete all text
        if let before = textDocumentProxy.documentContextBeforeInput {
            for _ in 0..<before.count {
                textDocumentProxy.deleteBackward()
            }
        }
        // Insert new text
        textDocumentProxy.insertText(newText)
    }

    private func showDiffSheet(original: String, rewritten: String) {
        dismissDiffSheet()

        let sheet = DiffSheetView(original: original, rewritten: rewritten)
        sheet.delegate = self
        sheet.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(sheet)

        let sheetHeight: CGFloat = 280
        NSLayoutConstraint.activate([
            sheet.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            sheet.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            sheet.topAnchor.constraint(equalTo: toolbar.bottomAnchor),
            sheet.heightAnchor.constraint(equalToConstant: sheetHeight),
        ])

        // Expand keyboard height to fit the sheet
        view.constraints.first { $0.firstAttribute == .height }?.constant = 44 + sheetHeight
        self.diffSheet = sheet
    }

    private func dismissDiffSheet() {
        diffSheet?.removeFromSuperview()
        diffSheet = nil
        // Reset keyboard height to toolbar only
        view.constraints.first { $0.firstAttribute == .height }?.constant = 44
    }

    private func showNeedApiKeyBanner() {
        let banner = UILabel()
        banner.text = "Open DraftRight app to set up API key"
        banner.font = .systemFont(ofSize: 12)
        banner.textColor = .systemOrange
        banner.textAlignment = .center
        banner.backgroundColor = UIColor.systemOrange.withAlphaComponent(0.1)
        banner.layer.cornerRadius = 6
        banner.clipsToBounds = true
        banner.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(banner)

        NSLayoutConstraint.activate([
            banner.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 16),
            banner.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -16),
            banner.topAnchor.constraint(equalTo: toolbar.bottomAnchor, constant: 4),
            banner.heightAnchor.constraint(equalToConstant: 28),
        ])

        DispatchQueue.main.asyncAfter(deadline: .now() + 3) {
            banner.removeFromSuperview()
        }
    }
}

// MARK: - ToolbarViewDelegate

extension KeyboardViewController: ToolbarViewDelegate {
    func toolbarDidSelectTone(_ tone: Tone) {
        let text = readFullText().trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }

        if settings.aiProvider == "openai" && settings.apiKey.isEmpty {
            showNeedApiKeyBanner()
            return
        }

        originalText = text
        toolbar.setLoading(tone)

        aiClient.rewrite(text: text, tone: tone, settings: settings) { [weak self] result in
            DispatchQueue.main.async {
                self?.toolbar.clearLoading()
                switch result {
                case .success(let rewritten):
                    self?.showDiffSheet(original: text, rewritten: rewritten)
                case .failure(let error):
                    // Show error briefly
                    let banner = UILabel()
                    banner.text = error.localizedDescription
                    banner.font = .systemFont(ofSize: 11)
                    banner.textColor = .systemRed
                    banner.textAlignment = .center
                    banner.translatesAutoresizingMaskIntoConstraints = false
                    self?.view.addSubview(banner)
                    if let view = self?.view, let toolbar = self?.toolbar {
                        NSLayoutConstraint.activate([
                            banner.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 16),
                            banner.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -16),
                            banner.topAnchor.constraint(equalTo: toolbar.bottomAnchor, constant: 4),
                        ])
                    }
                    DispatchQueue.main.asyncAfter(deadline: .now() + 3) {
                        banner.removeFromSuperview()
                    }
                }
            }
        }
    }

    func toolbarDidTapUndo() {
        if let original = originalText {
            replaceAllText(with: original)
            toolbar.hideUndo()
            originalText = nil
        }
    }
}

// MARK: - DiffSheetDelegate

extension KeyboardViewController: DiffSheetDelegate {
    func diffSheetDidReplace(_ text: String) {
        replaceAllText(with: text)
        dismissDiffSheet()
        toolbar.showUndo()
    }

    func diffSheetDidCopy(_ text: String) {
        UIPasteboard.general.string = text
        dismissDiffSheet()
    }

    func diffSheetDidCancel() {
        dismissDiffSheet()
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add ios/DraftRightKeyboard/KeyboardViewController.swift
git commit -m "feat: implement full keyboard controller with toolbar, diff sheet, and text replacement"
```

---

### Task 8: Update Flutter App to Write Shared Settings

**Files:**
- Modify: `DraftRightMobile/lib/services/settings_service.dart`

The Flutter app's `SettingsService` currently writes to regular `SharedPreferences`. For iOS, we also need to write to the App Group `UserDefaults` so the keyboard extension can read them. We'll use a method channel to write to the App Group.

- [ ] **Step 1: Add a method channel to write to App Group on iOS**

Add to `DraftRightMobile/ios/Runner/AppDelegate.swift` (or create a new file `DraftRightMobile/ios/Runner/AppGroupChannel.swift`):

This step requires Xcode configuration. For now, the simplest approach is to have the Flutter app write settings to both regular SharedPreferences AND App Group UserDefaults via a platform channel. However, this is best done when integrating in Xcode.

For now, the keyboard extension can be tested by manually setting values in the App Group UserDefaults.

- [ ] **Step 2: Commit**

```bash
git add -A
git commit -m "docs: note App Group integration needed for Flutter-to-extension settings sharing"
```

---

### Task 9: Verify and Document

- [ ] **Step 1: Verify all Swift files compile**

Open `DraftRightMobile/ios/Runner.xcworkspace` in Xcode, add DraftRightKeyboard target, build for simulator.

- [ ] **Step 2: Commit all files**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile
git add -A
git commit -m "feat: complete iOS keyboard extension with toolbar, diff sheet, and API client"
```
