import UIKit

protocol DiffSheetDelegate: AnyObject {
    func diffSheetDidReplace(_ text: String)
    func diffSheetDidCopy(_ text: String)
    func diffSheetDidCancel()
}

final class DiffSheetView: UIView {
    weak var delegate: DiffSheetDelegate?
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

        let handle = UIView()
        handle.backgroundColor = .systemGray3
        handle.layer.cornerRadius = 2.5
        handle.translatesAutoresizingMaskIntoConstraints = false
        addSubview(handle)

        let origHeader = makeHeader("Original")
        let rewriteHeader = makeHeader("Rewritten")

        let originalLabel = UITextView()
        originalLabel.text = original
        originalLabel.font = .systemFont(ofSize: 14)
        originalLabel.isEditable = false
        originalLabel.backgroundColor = UIColor.systemRed.withAlphaComponent(0.03)
        originalLabel.layer.cornerRadius = 6
        originalLabel.textContainerInset = UIEdgeInsets(top: 8, left: 4, bottom: 8, right: 4)

        let rewrittenLabel = UITextView()
        rewrittenLabel.text = rewritten
        rewrittenLabel.font = .systemFont(ofSize: 14)
        rewrittenLabel.isEditable = false
        rewrittenLabel.backgroundColor = UIColor.systemGreen.withAlphaComponent(0.03)
        rewrittenLabel.layer.cornerRadius = 6
        rewrittenLabel.textContainerInset = UIEdgeInsets(top: 8, left: 4, bottom: 8, right: 4)

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

        let cancelBtn = makeButton("Cancel", primary: false)
        cancelBtn.accessibilityIdentifier = "dr_diff_cancel"
        cancelBtn.addTarget(self, action: #selector(cancelTapped), for: .touchUpInside)

        let copyBtn = makeButton("Copy", primary: false)
        copyBtn.accessibilityIdentifier = "dr_diff_copy"
        copyBtn.addTarget(self, action: #selector(copyTapped), for: .touchUpInside)

        let replaceBtn = makeButton("Replace", primary: true)
        replaceBtn.accessibilityIdentifier = "dr_diff_replace"
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

    private func makeButton(_ title: String, primary: Bool) -> UIButton {
        let button = UIButton(type: .system)
        button.setTitle(title, for: .normal)
        button.titleLabel?.font = .systemFont(ofSize: 14, weight: .medium)
        button.layer.cornerRadius = 8
        if primary {
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
