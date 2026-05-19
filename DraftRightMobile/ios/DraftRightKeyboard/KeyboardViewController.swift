import UIKit
import DraftRightKeyboardCore

class KeyboardViewController: UIInputViewController {

    private let toolbar = ToolbarView()
    private let keyboard = QwertyKeyboardView()
    private let aiClient = BackendClient()
    private let settings = SharedSettings()
    private var diffSheet: DiffSheetView?
    private var originalText: String?
    private var heightConstraint: NSLayoutConstraint!

    // Tier β: language registry + per-language composer routing.
    private let registry = LanguageRegistry.production
    private var controller: KeyboardController!

    private var totalHeight: CGFloat {
        return 44 + keyboard.totalHeight // toolbar + keyboard rows
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        rebuildController()
        setupUI()
    }

    override func viewWillAppear(_ animated: Bool) {
        super.viewWillAppear(animated)
        // Re-read enabled / active language ids each time the keyboard
        // appears so a settings change in the main app takes effect on
        // the next invocation without requiring an extension reload.
        rebuildController()
    }

    private func rebuildController() {
        let enabledIds = settings.enabledLanguageIds
        let activeId = settings.activeLanguageId
        NSLog("[DraftRightKB] rebuildController enabled=\(enabledIds) active=\(activeId)")
        controller = KeyboardController(
            registry: registry,
            enabledIds: enabledIds,
            activeId: activeId
        )
        NSLog("[DraftRightKB] controller.enabled.count=\(controller.enabled.count) current=\(controller.current.id)")
    }

    private func setupUI() {
        // Height constraint for the entire input view
        heightConstraint = view.heightAnchor.constraint(equalToConstant: totalHeight)
        heightConstraint.priority = .defaultHigh
        heightConstraint.isActive = true

        // Toolbar
        toolbar.delegate = self
        toolbar.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(toolbar)

        // Keyboard
        keyboard.delegate = self
        keyboard.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(keyboard)

        NSLayoutConstraint.activate([
            toolbar.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            toolbar.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            toolbar.topAnchor.constraint(equalTo: view.topAnchor),
            toolbar.heightAnchor.constraint(equalToConstant: 44),

            keyboard.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            keyboard.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            keyboard.topAnchor.constraint(equalTo: toolbar.bottomAnchor),
            keyboard.heightAnchor.constraint(equalToConstant: keyboard.totalHeight),
        ])
    }

    // MARK: - Text operations

    private func readFullText() -> String {
        let before = textDocumentProxy.documentContextBeforeInput ?? ""
        let after = textDocumentProxy.documentContextAfterInput ?? ""
        return before + after
    }

    private func replaceAllText(with newText: String) {
        // 1. Commit any pending Telex composition. Without this, the
        //    marked-text region survives the delete loop and the
        //    rewritten text appears partial.
        textDocumentProxy.unmarkText()
        controller?.composer?.reset()
        // 2. Walk to end of document.
        if let after = textDocumentProxy.documentContextAfterInput {
            textDocumentProxy.adjustTextPosition(byCharacterOffset: after.count)
        }
        // 3. Delete the entire field, then insert.
        if let before = textDocumentProxy.documentContextBeforeInput {
            for _ in 0..<before.count {
                textDocumentProxy.deleteBackward()
            }
        }
        textDocumentProxy.insertText(newText)
    }

    // MARK: - Diff sheet

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

        // Push keyboard below diff sheet
        heightConstraint.constant = 44 + sheetHeight + keyboard.totalHeight
        self.diffSheet = sheet
    }

    private func dismissDiffSheet() {
        diffSheet?.removeFromSuperview()
        diffSheet = nil
        heightConstraint.constant = totalHeight
    }

    // MARK: - Banner

    private func showBanner(_ text: String, color: UIColor) {
        let banner = UILabel()
        banner.text = text
        banner.font = .systemFont(ofSize: 12)
        banner.textColor = color
        banner.textAlignment = .center
        banner.backgroundColor = color.withAlphaComponent(0.1)
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

        if settings.bearerToken.isEmpty {
            showBanner("Please login in DraftRight app", color: .systemOrange)
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
                    self?.showBanner(error.localizedDescription, color: .systemRed)
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

// MARK: - KeyboardActionDelegate

extension KeyboardViewController: KeyboardActionDelegate {
    func keyboardDidType(_ char: String) {
        guard let ch = char.first, char.count == 1 else {
            textDocumentProxy.insertText(char)
            return
        }
        dispatch(controller.onKey(ch), fallback: char)
    }

    func keyboardDidBackspace() {
        dispatchBackspace(controller.onBackspace())
    }

    func keyboardDidEnter() {
        // Commit any pending composition so newline doesn't get swallowed
        // by the marked-text region.
        textDocumentProxy.unmarkText()
        controller.composer?.reset()
        textDocumentProxy.insertText("\n")
    }

    func keyboardDidSpace() {
        // Route space through the composer so a pending Telex composition
        // commits FIRST, then space appends. Without this, a direct
        // insertText(" ") replaces the marked region (e.g. "viet")
        // with a single space and the word vanishes.
        dispatch(controller.onKey(" "), fallback: " ")
    }

    func keyboardDidSwitchKeyboard() {
        // Single tap on globe: cycle to next enabled language. If only
        // one is enabled, fall back to system keyboard switcher.
        if controller.enabled.count > 1 {
            controller.cycleLanguage()
            // Future: refresh visible layout when per-language layouts ship.
        } else {
            advanceToNextInputMode()
        }
    }

    func keyboardDidSpaceSwipe(direction: Int) {
        guard controller.enabled.count > 1 else { return }
        controller.cycleLanguage(reverse: direction < 0)
        // Future: refresh visible layout when per-language layouts ship.
    }

    // MARK: - KeystrokeOutcome dispatch

    private func dispatch(_ outcome: KeystrokeOutcome, fallback: String) {
        switch outcome {
        case .commit(let text):
            textDocumentProxy.unmarkText()
            textDocumentProxy.insertText(text)
        case .composing(let text):
            textDocumentProxy.setMarkedText(text, selectedRange: NSRange(location: text.utf16.count, length: 0))
        case .deleteOne:
            textDocumentProxy.deleteBackward()
        case .noChange:
            textDocumentProxy.unmarkText()
        }
    }

    private func dispatchBackspace(_ outcome: KeystrokeOutcome) {
        switch outcome {
        case .commit(let text):
            textDocumentProxy.unmarkText()
            textDocumentProxy.insertText(text)
        case .composing(let text):
            textDocumentProxy.setMarkedText(text, selectedRange: NSRange(location: text.utf16.count, length: 0))
        case .deleteOne:
            textDocumentProxy.deleteBackward()
        case .noChange:
            // Composer just emptied its buffer via stripOneLayer. The
            // marked-text region still shows the previous frame — clear
            // it so the user doesn't appear stuck on the first character.
            textDocumentProxy.unmarkText()
        }
    }
}
