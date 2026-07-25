import UIKit

protocol ToolbarViewDelegate: AnyObject {
    func toolbarDidSelectTone(_ tone: Tone)
    /// One-tap primary action: rewrite the whole field with the user's
    /// preset tone and apply directly (no tone pick, no diff confirm).
    func toolbarDidTapOneTap()
    func toolbarDidTapUndo()
}

final class ToolbarView: UIView {
    weak var delegate: ToolbarViewDelegate?

    private let scrollView = UIScrollView()
    private let stackView = UIStackView()
    private var undoButton: UIButton?
    private var oneTapButton: UIButton?
    // Tone buttons in `Tone.allCases` order. Kept as an explicit array so the
    // loading-spinner lookup stays correct regardless of the leading one-tap
    // button or trailing undo button, which would otherwise shift the
    // `stackView.arrangedSubviews` indices out of tone order.
    private var toneButtons: [UIButton] = []
    private var spinner: UIActivityIndicatorView?
    // The button currently showing a spinner + the image to restore when
    // loading clears — lets any button (tone or one-tap) drive the spinner.
    private weak var loadingButton: UIButton?
    private var loadingButtonImage: UIImage?

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
        // Stable id so UI tests can detect that the DraftRight keyboard is
        // the active one (the toolbar is a sibling of the keyboard view,
        // not part of `app.keyboards`).
        accessibilityIdentifier = "dr_toolbar"

        scrollView.showsHorizontalScrollIndicator = false
        scrollView.translatesAutoresizingMaskIntoConstraints = false
        addSubview(scrollView)

        stackView.axis = .horizontal
        stackView.spacing = 4
        stackView.alignment = .center
        stackView.translatesAutoresizingMaskIntoConstraints = false
        scrollView.addSubview(stackView)

        // Leading primary action: one tap → rewrite the whole field with the
        // user's preset tone and apply directly. The closest iOS-legal
        // equivalent of the Android floating bubble's one-tap rewrite.
        let oneTap = createOneTapButton()
        stackView.addArrangedSubview(oneTap)
        oneTapButton = oneTap

        for (index, tone) in Tone.allCases.enumerated() {
            let button = createToneButton(tone, index: index)
            toneButtons.append(button)
            stackView.addArrangedSubview(button)
        }

        let spacer = UIView()
        spacer.setContentHuggingPriority(.defaultLow, for: .horizontal)
        stackView.addArrangedSubview(spacer)

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

    private func createToneButton(_ tone: Tone, index: Int) -> UIButton {
        let button = UIButton(type: .system)
        let config = UIImage.SymbolConfiguration(pointSize: 16, weight: .medium)
        button.setImage(UIImage(systemName: tone.iconName, withConfiguration: config), for: .normal)
        // Brand blue, matching the website primary, so the AI actions stand out.
        button.tintColor = .draftRightBrand
        button.tag = index
        button.addTarget(self, action: #selector(toneTapped(_:)), for: .touchUpInside)
        button.widthAnchor.constraint(equalToConstant: 40).isActive = true
        button.heightAnchor.constraint(equalToConstant: 36).isActive = true
        button.layer.cornerRadius = 6
        button.accessibilityLabel = tone.displayName
        button.accessibilityIdentifier = "dr_tone_\(index)"
        return button
    }

    /// Filled-bolt primary button, brand-tinted so it reads as the quick
    /// action distinct from the outline tone icons.
    private func createOneTapButton() -> UIButton {
        let button = UIButton(type: .system)
        let config = UIImage.SymbolConfiguration(pointSize: 16, weight: .semibold)
        button.setImage(UIImage(systemName: "bolt.fill", withConfiguration: config), for: .normal)
        button.tintColor = .white
        button.backgroundColor = .draftRightBrand
        button.addTarget(self, action: #selector(oneTapTapped), for: .touchUpInside)
        button.widthAnchor.constraint(equalToConstant: 40).isActive = true
        button.heightAnchor.constraint(equalToConstant: 36).isActive = true
        button.layer.cornerRadius = 6
        button.accessibilityLabel = "One-tap rewrite"
        button.accessibilityIdentifier = "dr_onetap"
        return button
    }

    @objc private func toneTapped(_ sender: UIButton) {
        guard Tone.allCases.indices.contains(sender.tag) else { return }
        delegate?.toolbarDidSelectTone(Tone.allCases[sender.tag])
    }

    @objc private func oneTapTapped() {
        delegate?.toolbarDidTapOneTap()
    }

    @objc private func undoTapped() {
        delegate?.toolbarDidTapUndo()
    }

    /// Show a spinner on the tapped tone button while its rewrite runs.
    func setLoading(_ tone: Tone) {
        guard let index = Tone.allCases.firstIndex(of: tone),
              toneButtons.indices.contains(index) else { return }
        startSpinner(on: toneButtons[index])
    }

    /// Show a spinner on the one-tap button while its rewrite runs.
    func setOneTapLoading() {
        guard let button = oneTapButton else { return }
        startSpinner(on: button)
    }

    private func startSpinner(on button: UIButton) {
        loadingButton = button
        loadingButtonImage = button.image(for: .normal)
        isUserInteractionEnabled = false
        button.setImage(nil, for: .normal)
        let spinner = UIActivityIndicatorView(style: .medium)
        spinner.color = button.tintColor
        spinner.startAnimating()
        spinner.translatesAutoresizingMaskIntoConstraints = false
        button.addSubview(spinner)
        spinner.centerXAnchor.constraint(equalTo: button.centerXAnchor).isActive = true
        spinner.centerYAnchor.constraint(equalTo: button.centerYAnchor).isActive = true
        self.spinner = spinner
    }

    /// Remove the spinner and restore whichever button was loading.
    func clearLoading() {
        isUserInteractionEnabled = true
        spinner?.removeFromSuperview()
        spinner = nil
        loadingButton?.setImage(loadingButtonImage, for: .normal)
        loadingButton = nil
        loadingButtonImage = nil
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
