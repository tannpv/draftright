import UIKit
import MobileCoreServices

class ActionViewController: UIViewController {

    private let settings = SharedSettings()
    private let aiClient = BackendClient()

    private var inputText = ""
    private var selectedTone: Tone?

    // UI elements
    private let headerLabel = UILabel()
    private let inputTextView = UITextView()
    private let toneStack = UIStackView()
    private let resultTextView = UITextView()
    private let spinner = UIActivityIndicatorView(style: .medium)
    private let copyButton = UIButton(type: .system)
    private let statusLabel = UILabel()

    override func viewDidLoad() {
        super.viewDidLoad()
        setupUI()
        extractInputText()
    }

    // MARK: - UI Setup

    private func setupUI() {
        view.backgroundColor = .systemBackground

        // Navigation bar
        title = "DraftRight"
        navigationItem.leftBarButtonItem = UIBarButtonItem(
            barButtonSystemItem: .cancel, target: self, action: #selector(cancelTapped))

        // Header
        headerLabel.text = "Select a tone to rewrite:"
        headerLabel.font = .systemFont(ofSize: 15, weight: .semibold)
        headerLabel.textColor = .label
        headerLabel.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(headerLabel)

        // Tone buttons
        toneStack.axis = .horizontal
        toneStack.spacing = 8
        toneStack.distribution = .fillEqually
        toneStack.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(toneStack)

        // First row: simple, natural, polished
        let topRow = UIStackView()
        topRow.axis = .horizontal
        topRow.spacing = 8
        topRow.distribution = .fillEqually
        topRow.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(topRow)

        // Second row: concise, technical, translate
        let bottomRow = UIStackView()
        bottomRow.axis = .horizontal
        bottomRow.spacing = 8
        bottomRow.distribution = .fillEqually
        bottomRow.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(bottomRow)

        for (index, tone) in Tone.allCases.enumerated() {
            let btn = makeToneButton(tone)
            btn.tag = index
            if index < 3 {
                topRow.addArrangedSubview(btn)
            } else {
                bottomRow.addArrangedSubview(btn)
            }
        }

        // Input text (original)
        let origLabel = makeSectionLabel("Original")
        origLabel.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(origLabel)

        inputTextView.isEditable = false
        inputTextView.font = .systemFont(ofSize: 14)
        inputTextView.backgroundColor = UIColor.systemGray6
        inputTextView.layer.cornerRadius = 8
        inputTextView.textContainerInset = UIEdgeInsets(top: 8, left: 8, bottom: 8, right: 8)
        inputTextView.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(inputTextView)

        // Result text (rewritten)
        let resultLabel = makeSectionLabel("Rewritten")
        resultLabel.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(resultLabel)

        resultTextView.isEditable = false
        resultTextView.font = .systemFont(ofSize: 14)
        resultTextView.backgroundColor = UIColor.systemGreen.withAlphaComponent(0.05)
        resultTextView.layer.cornerRadius = 8
        resultTextView.textContainerInset = UIEdgeInsets(top: 8, left: 8, bottom: 8, right: 8)
        resultTextView.text = "Pick a tone above to rewrite your text"
        resultTextView.textColor = .placeholderText
        resultTextView.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(resultTextView)

        // Spinner
        spinner.hidesWhenStopped = true
        spinner.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(spinner)

        // Copy button
        copyButton.setTitle("Copy to Clipboard", for: .normal)
        copyButton.titleLabel?.font = .systemFont(ofSize: 16, weight: .semibold)
        copyButton.backgroundColor = .systemBlue
        copyButton.setTitleColor(.white, for: .normal)
        copyButton.layer.cornerRadius = 10
        copyButton.isHidden = true
        copyButton.addTarget(self, action: #selector(copyTapped), for: .touchUpInside)
        copyButton.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(copyButton)

        // Status label
        statusLabel.font = .systemFont(ofSize: 12)
        statusLabel.textAlignment = .center
        statusLabel.isHidden = true
        statusLabel.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(statusLabel)

        let guide = view.safeAreaLayoutGuide
        NSLayoutConstraint.activate([
            headerLabel.topAnchor.constraint(equalTo: guide.topAnchor, constant: 16),
            headerLabel.leadingAnchor.constraint(equalTo: guide.leadingAnchor, constant: 16),

            topRow.topAnchor.constraint(equalTo: headerLabel.bottomAnchor, constant: 12),
            topRow.leadingAnchor.constraint(equalTo: guide.leadingAnchor, constant: 16),
            topRow.trailingAnchor.constraint(equalTo: guide.trailingAnchor, constant: -16),
            topRow.heightAnchor.constraint(equalToConstant: 36),

            bottomRow.topAnchor.constraint(equalTo: topRow.bottomAnchor, constant: 8),
            bottomRow.leadingAnchor.constraint(equalTo: guide.leadingAnchor, constant: 16),
            bottomRow.trailingAnchor.constraint(equalTo: guide.trailingAnchor, constant: -16),
            bottomRow.heightAnchor.constraint(equalToConstant: 36),

            origLabel.topAnchor.constraint(equalTo: bottomRow.bottomAnchor, constant: 16),
            origLabel.leadingAnchor.constraint(equalTo: guide.leadingAnchor, constant: 16),

            inputTextView.topAnchor.constraint(equalTo: origLabel.bottomAnchor, constant: 4),
            inputTextView.leadingAnchor.constraint(equalTo: guide.leadingAnchor, constant: 16),
            inputTextView.trailingAnchor.constraint(equalTo: guide.trailingAnchor, constant: -16),
            inputTextView.heightAnchor.constraint(equalToConstant: 80),

            resultLabel.topAnchor.constraint(equalTo: inputTextView.bottomAnchor, constant: 12),
            resultLabel.leadingAnchor.constraint(equalTo: guide.leadingAnchor, constant: 16),

            spinner.centerYAnchor.constraint(equalTo: resultLabel.centerYAnchor),
            spinner.leadingAnchor.constraint(equalTo: resultLabel.trailingAnchor, constant: 8),

            resultTextView.topAnchor.constraint(equalTo: resultLabel.bottomAnchor, constant: 4),
            resultTextView.leadingAnchor.constraint(equalTo: guide.leadingAnchor, constant: 16),
            resultTextView.trailingAnchor.constraint(equalTo: guide.trailingAnchor, constant: -16),
            resultTextView.bottomAnchor.constraint(equalTo: copyButton.topAnchor, constant: -12),

            copyButton.leadingAnchor.constraint(equalTo: guide.leadingAnchor, constant: 16),
            copyButton.trailingAnchor.constraint(equalTo: guide.trailingAnchor, constant: -16),
            copyButton.heightAnchor.constraint(equalToConstant: 48),
            copyButton.bottomAnchor.constraint(equalTo: statusLabel.topAnchor, constant: -8),

            statusLabel.leadingAnchor.constraint(equalTo: guide.leadingAnchor, constant: 16),
            statusLabel.trailingAnchor.constraint(equalTo: guide.trailingAnchor, constant: -16),
            statusLabel.bottomAnchor.constraint(equalTo: guide.bottomAnchor, constant: -8),
        ])
    }

    // MARK: - Extract input text

    private func extractInputText() {
        guard let items = extensionContext?.inputItems as? [NSExtensionItem] else {
            showError("No input received")
            return
        }

        for item in items {
            guard let attachments = item.attachments else { continue }
            for provider in attachments {
                if provider.hasItemConformingToTypeIdentifier(kUTTypePlainText as String) {
                    provider.loadItem(forTypeIdentifier: kUTTypePlainText as String, options: nil) { [weak self] (text, _) in
                        DispatchQueue.main.async {
                            if let text = text as? String, !text.isEmpty {
                                self?.inputText = text
                                self?.inputTextView.text = text
                            } else {
                                self?.showError("No text found")
                            }
                        }
                    }
                    return
                }
            }
        }
        showError("No text content found. Select text before sharing.")
    }

    // MARK: - Tone selection

    private func makeToneButton(_ tone: Tone) -> UIButton {
        let btn = UIButton(type: .system)
        let config = UIImage.SymbolConfiguration(pointSize: 12, weight: .medium)
        let image = UIImage(systemName: tone.iconName, withConfiguration: config)
        btn.setImage(image, for: .normal)
        btn.setTitle(" \(tone.displayName)", for: .normal)
        btn.titleLabel?.font = .systemFont(ofSize: 12, weight: .medium)
        btn.backgroundColor = .systemGray5
        btn.layer.cornerRadius = 8
        btn.addTarget(self, action: #selector(toneTapped(_:)), for: .touchUpInside)
        return btn
    }

    @objc private func toneTapped(_ sender: UIButton) {
        let tone = Tone.allCases[sender.tag]
        guard !inputText.isEmpty else { return }

        if settings.accessToken.isEmpty {
            showError("Please login in the DraftRight app first")
            return
        }

        // Highlight selected tone
        selectedTone = tone
        highlightSelectedTone(sender)

        // Show loading
        resultTextView.text = ""
        resultTextView.textColor = .label
        copyButton.isHidden = true
        statusLabel.isHidden = true
        spinner.startAnimating()

        aiClient.rewrite(text: inputText, tone: tone, settings: settings) { [weak self] result in
            DispatchQueue.main.async {
                self?.spinner.stopAnimating()
                switch result {
                case .success(let rewritten):
                    self?.resultTextView.text = rewritten
                    self?.resultTextView.textColor = .label
                    self?.copyButton.isHidden = false
                case .failure(let error):
                    self?.showError(error.localizedDescription)
                }
            }
        }
    }

    private func highlightSelectedTone(_ selected: UIButton) {
        // Reset all tone buttons in both rows
        for case let stack as UIStackView in view.subviews {
            for case let btn as UIButton in stack.arrangedSubviews {
                btn.backgroundColor = .systemGray5
                btn.tintColor = .systemBlue
            }
        }
        selected.backgroundColor = .systemBlue
        selected.tintColor = .white
        selected.setTitleColor(.white, for: .normal)
    }

    // MARK: - Actions

    @objc private func cancelTapped() {
        extensionContext?.completeRequest(returningItems: nil, completionHandler: nil)
    }

    @objc private func copyTapped() {
        guard let text = resultTextView.text, !text.isEmpty else { return }
        UIPasteboard.general.string = text
        statusLabel.text = "Copied to clipboard! Paste it back in your app."
        statusLabel.textColor = .systemGreen
        statusLabel.isHidden = false

        // Auto-close after 1.5s
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) { [weak self] in
            self?.extensionContext?.completeRequest(returningItems: nil, completionHandler: nil)
        }
    }

    // MARK: - Helpers

    private func makeSectionLabel(_ text: String) -> UILabel {
        let label = UILabel()
        label.text = text
        label.font = .systemFont(ofSize: 12, weight: .semibold)
        label.textColor = .secondaryLabel
        return label
    }

    private func showError(_ message: String) {
        statusLabel.text = message
        statusLabel.textColor = .systemRed
        statusLabel.isHidden = false
    }
}
