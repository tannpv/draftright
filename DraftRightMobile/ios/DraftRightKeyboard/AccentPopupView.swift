import UIKit

/// Long-press accent picker. Shows a horizontal strip of accent variants
/// above the pressed key; tapping one inserts it, tapping outside dismisses.
final class AccentPopupView: UIView {

    private let accents: [String]
    private let keyColor: UIColor
    private let textColor: UIColor
    private let radius: CGFloat
    private let onSelect: (String) -> Void
    private let onDismiss: () -> Void

    private let cellWidth: CGFloat = 44
    private let cellHeight: CGFloat = 48
    private let cellGap: CGFloat = 4

    private var container: UIView?

    init(accents: [String],
         keyColor: UIColor,
         textColor: UIColor,
         radius: CGFloat,
         onSelect: @escaping (String) -> Void,
         onDismiss: @escaping () -> Void) {
        self.accents = accents
        self.keyColor = keyColor
        self.textColor = textColor
        self.radius = radius
        self.onSelect = onSelect
        self.onDismiss = onDismiss
        super.init(frame: .zero)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) not implemented") }

    /// Add a full-bounds transparent layer (to catch outside taps) plus the
    /// accent strip anchored above `anchor`, in `host`'s coordinate space.
    func present(over host: UIView, anchoredTo anchor: CGRect) {
        frame = host.bounds
        backgroundColor = .clear
        host.addSubview(self)

        let outsideTap = UITapGestureRecognizer(target: self, action: #selector(outsideTapped))
        outsideTap.cancelsTouchesInView = false
        outsideTap.delegate = self
        addGestureRecognizer(outsideTap)

        let stripWidth = CGFloat(accents.count) * cellWidth + CGFloat(accents.count - 1) * cellGap + 12
        var x = anchor.midX - stripWidth / 2
        x = max(4, min(x, host.bounds.width - stripWidth - 4))
        // Prefer above the key; if there isn't room (top row), drop below so
        // the strip stays on-screen and hittable.
        let containerHeight = cellHeight + 12
        var y = anchor.minY - containerHeight - 8
        if y < 4 { y = anchor.maxY + 8 }

        let container = UIView(frame: CGRect(x: x, y: y, width: stripWidth, height: containerHeight))
        self.container = container
        container.backgroundColor = keyColor
        container.layer.cornerRadius = radius + 2
        container.layer.borderWidth = 0.5
        container.layer.borderColor = UIColor.separator.cgColor
        container.layer.shadowColor = UIColor.black.cgColor
        container.layer.shadowOpacity = 0.2
        container.layer.shadowOffset = CGSize(width: 0, height: 2)
        addSubview(container)

        var cx: CGFloat = 6
        for accent in accents {
            let b = UIButton(type: .custom)
            b.frame = CGRect(x: cx, y: 6, width: cellWidth, height: cellHeight)
            b.setTitle(accent, for: .normal)
            b.setTitleColor(textColor, for: .normal)
            b.titleLabel?.font = .systemFont(ofSize: 22)
            b.layer.cornerRadius = radius
            b.accessibilityIdentifier = "dr_accent_\(accent)"
            b.addTarget(self, action: #selector(accentTapped(_:)), for: .touchUpInside)
            container.addSubview(b)
            cx += cellWidth + cellGap
        }
    }

    @objc private func accentTapped(_ sender: UIButton) {
        guard let title = sender.title(for: .normal) else { return }
        onSelect(title)
    }

    @objc private func outsideTapped() {
        onDismiss()
    }
}

extension AccentPopupView: UIGestureRecognizerDelegate {
    func gestureRecognizer(_ gestureRecognizer: UIGestureRecognizer,
                           shouldReceive touch: UITouch) -> Bool {
        // Only treat taps OUTSIDE the accent strip as dismiss taps; taps on
        // the strip are handled by the accent buttons themselves.
        guard let container else { return true }
        return !container.frame.contains(touch.location(in: self))
    }
}
