import UIKit
import DraftRightKeyboardCore

protocol ToolbarViewDelegate: AnyObject {
    func toolbarDidSelectTone(_ tone: Tone)
    func toolbarDidTapUndo()

    // Hold-to-talk voice input (mirrors Android's VoiceHoldListener).
    /// A hold started. Return true iff a voice session actually began
    /// (permission granted + recognizer available) so a later release ends
    /// THIS session, not a stale one.
    func toolbarVoiceHoldStart() -> Bool
    /// The finger slid past the cancel threshold (or back inside it).
    func toolbarVoiceCancelArmedChanged(_ armed: Bool)
    /// The hold ended; `cancelled` reflects the slide-away gesture.
    func toolbarVoiceHoldEnd(cancelled: Bool)
    /// A press too short to arm a hold — show the "hold to talk" hint.
    func toolbarVoiceTooShortTap()
}

final class ToolbarView: UIView {
    weak var delegate: ToolbarViewDelegate?

    private let scrollView = UIScrollView()
    private let stackView = UIStackView()
    private var undoButton: UIButton?
    private var micButton: UIButton?
    // Hold-to-talk gesture state (see micLongPress).
    private var micHoldStarted = false
    private var micStartPoint: CGPoint = .zero
    private var micCancelArmed = false
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

        // Hold-to-talk mic — hidden until the host confirms the recognizer is
        // available + authorized (setVoiceAvailable). iOS equivalent of the
        // Android keyboard's mic key.
        let mic = createMicButton()
        mic.isHidden = true
        stackView.addArrangedSubview(mic)
        micButton = mic

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

    @objc private func toneTapped(_ sender: UIButton) {
        guard Tone.allCases.indices.contains(sender.tag) else { return }
        delegate?.toolbarDidSelectTone(Tone.allCases[sender.tag])
    }

    // MARK: - Hold-to-talk mic

    private func createMicButton() -> UIButton {
        let button = UIButton(type: .system)
        let config = UIImage.SymbolConfiguration(pointSize: 16, weight: .medium)
        button.setImage(UIImage(systemName: "mic.fill", withConfiguration: config), for: .normal)
        button.tintColor = .draftRightBrand
        button.widthAnchor.constraint(equalToConstant: 40).isActive = true
        button.heightAnchor.constraint(equalToConstant: 36).isActive = true
        button.layer.cornerRadius = 6
        button.accessibilityLabel = "Hold to talk"
        button.accessibilityIdentifier = "dr_mic"

        let longPress = UILongPressGestureRecognizer(target: self, action: #selector(micLongPress(_:)))
        longPress.minimumPressDuration = Double(MicHoldGesture.armMs) / 1000.0
        button.addGestureRecognizer(longPress)

        // A press shorter than the arm delay is a tap, not a hold → show a hint.
        let tap = UITapGestureRecognizer(target: self, action: #selector(micTapped))
        tap.require(toFail: longPress)
        button.addGestureRecognizer(tap)
        return button
    }

    /// Reveal the mic once the host confirms STT is available + authorized.
    func setVoiceAvailable(_ available: Bool) {
        micButton?.isHidden = !available
    }

    @objc private func micLongPress(_ gr: UILongPressGestureRecognizer) {
        guard let mic = micButton else { return }
        switch gr.state {
        case .began:
            micStartPoint = gr.location(in: mic)
            micCancelArmed = false
            micHoldStarted = delegate?.toolbarVoiceHoldStart() ?? false
        case .changed:
            guard micHoldStarted else { return }
            let p = gr.location(in: mic)
            let armed = MicHoldGesture.isCancelArmed(
                dx: p.x - micStartPoint.x, dy: p.y - micStartPoint.y, slop: MicHoldGesture.defaultSlop)
            if armed != micCancelArmed {
                micCancelArmed = armed
                delegate?.toolbarVoiceCancelArmedChanged(armed)
            }
        case .ended:
            if micHoldStarted { delegate?.toolbarVoiceHoldEnd(cancelled: micCancelArmed) }
            micHoldStarted = false
        case .cancelled, .failed:
            if micHoldStarted { delegate?.toolbarVoiceHoldEnd(cancelled: true) }
            micHoldStarted = false
        default:
            break
        }
    }

    @objc private func micTapped() {
        delegate?.toolbarVoiceTooShortTap()
    }

    /// Reflect the voice session state: pulse the mic while listening, spin it
    /// while polishing, restore on idle. Tone + one-tap actions are disabled
    /// during a session so a rewrite can't race the voice commit.
    func setVoiceState(_ state: VoiceSessionController.State) {
        switch state {
        case .listening:
            setActionsEnabled(false)
            startMicPulse()
        case .processing:
            stopMicPulse()
            setActionsEnabled(false)
            if let mic = micButton { startSpinner(on: mic) }
        case .idle:
            stopMicPulse()
            clearLoading()
            setActionsEnabled(true)
        }
    }

    private func startMicPulse() {
        guard let mic = micButton else { return }
        mic.alpha = 1.0
        UIView.animate(withDuration: 0.5, delay: 0,
                       options: [.repeat, .autoreverse, .allowUserInteraction],
                       animations: { mic.alpha = 0.3 })
    }

    private func stopMicPulse() {
        micButton?.layer.removeAllAnimations()
        micButton?.alpha = 1.0
    }

    private func setActionsEnabled(_ enabled: Bool) {
        toneButtons.forEach { $0.isEnabled = enabled }
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
