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

        for tone in Tone.allCases {
            let button = createToneButton(tone)
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

    private func createToneButton(_ tone: Tone) -> UIButton {
        let button = UIButton(type: .system)
        let config = UIImage.SymbolConfiguration(pointSize: 16, weight: .medium)
        button.setImage(UIImage(systemName: tone.iconName, withConfiguration: config), for: .normal)
        button.tag = Tone.allCases.firstIndex(of: tone)!
        button.addTarget(self, action: #selector(toneTapped(_:)), for: .touchUpInside)
        button.widthAnchor.constraint(equalToConstant: 40).isActive = true
        button.heightAnchor.constraint(equalToConstant: 36).isActive = true
        button.layer.cornerRadius = 6
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
