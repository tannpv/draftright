import UIKit

class KeyboardViewController: UIInputViewController {

    private let toolbar = ToolbarView()
    private let aiClient = BackendClient()
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

        let heightConstraint = view.heightAnchor.constraint(equalToConstant: 44)
        heightConstraint.priority = .defaultHigh
        heightConstraint.isActive = true
    }

    private func readFullText() -> String {
        let before = textDocumentProxy.documentContextBeforeInput ?? ""
        let after = textDocumentProxy.documentContextAfterInput ?? ""
        return before + after
    }

    private func replaceAllText(with newText: String) {
        if let after = textDocumentProxy.documentContextAfterInput {
            textDocumentProxy.adjustTextPosition(byCharacterOffset: after.count)
        }
        if let before = textDocumentProxy.documentContextBeforeInput {
            for _ in 0..<before.count {
                textDocumentProxy.deleteBackward()
            }
        }
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

        view.constraints.first { $0.firstAttribute == .height }?.constant = 44 + sheetHeight
        self.diffSheet = sheet
    }

    private func dismissDiffSheet() {
        diffSheet?.removeFromSuperview()
        diffSheet = nil
        view.constraints.first { $0.firstAttribute == .height }?.constant = 44
    }

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

extension KeyboardViewController: ToolbarViewDelegate {
    func toolbarDidSelectTone(_ tone: Tone) {
        let text = readFullText().trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }

        if settings.accessToken.isEmpty {
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
