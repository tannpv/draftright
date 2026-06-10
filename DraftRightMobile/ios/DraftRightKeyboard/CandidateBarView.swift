import UIKit
import DraftRightKeyboardCore

/// Horizontal scroll strip that renders suggestion chips above the AI
/// tone toolbar. Engine-agnostic: just receives [Candidate]s and emits
/// `onCandidatePicked` when a chip is tapped. Mirror of
/// `keyboard.ime.CandidateBarView` on Android — same role, same palette.
///
/// `barHeight` matches the strip's intrinsic vertical footprint. The
/// keyboard view controller adds it to the input view's height constraint
/// when candidates are non-empty and removes it otherwise.
final class CandidateBarView: UIView {

    static let barHeight: CGFloat = 36

    /// Tap callback. Caller commits the candidate and clears the strip.
    var onCandidatePicked: ((Candidate) -> Void)?

    private let scroll = UIScrollView()
    private let stack = UIStackView()

    /// Palette matches ToolbarView's dark tokens so the strip doesn't
    /// visually float — same hex values used there.
    private let bgColor      = UIColor(red: 0x1E/255, green: 0x29/255, blue: 0x3B/255, alpha: 1) // slate-800
    private let chipColor    = UIColor(red: 0x33/255, green: 0x41/255, blue: 0x55/255, alpha: 1) // slate-700
    private let chipBorder   = UIColor(red: 0x47/255, green: 0x55/255, blue: 0x69/255, alpha: 1) // slate-600
    private let textColor    = UIColor(red: 0xE2/255, green: 0xE8/255, blue: 0xF0/255, alpha: 1) // slate-200

    override init(frame: CGRect) {
        super.init(frame: frame)
        setupViews()
    }
    required init?(coder: NSCoder) { fatalError("not used") }

    private func setupViews() {
        backgroundColor = bgColor

        scroll.translatesAutoresizingMaskIntoConstraints = false
        scroll.showsHorizontalScrollIndicator = false
        scroll.showsVerticalScrollIndicator = false
        addSubview(scroll)

        stack.translatesAutoresizingMaskIntoConstraints = false
        stack.axis = .horizontal
        stack.alignment = .center
        stack.spacing = 8
        stack.layoutMargins = UIEdgeInsets(top: 2, left: 6, bottom: 2, right: 6)
        stack.isLayoutMarginsRelativeArrangement = true
        scroll.addSubview(stack)

        NSLayoutConstraint.activate([
            scroll.leadingAnchor.constraint(equalTo: leadingAnchor),
            scroll.trailingAnchor.constraint(equalTo: trailingAnchor),
            scroll.topAnchor.constraint(equalTo: topAnchor),
            scroll.bottomAnchor.constraint(equalTo: bottomAnchor),

            stack.leadingAnchor.constraint(equalTo: scroll.leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: scroll.trailingAnchor),
            stack.topAnchor.constraint(equalTo: scroll.topAnchor),
            stack.bottomAnchor.constraint(equalTo: scroll.bottomAnchor),
            stack.heightAnchor.constraint(equalTo: scroll.heightAnchor),
        ])
        isHidden = true
    }

    /// Replace the visible chips. Empty list hides the bar so the
    /// keyboard reclaims the vertical real estate.
    func setCandidates(_ candidates: [Candidate]) {
        stack.arrangedSubviews.forEach { v in
            stack.removeArrangedSubview(v)
            v.removeFromSuperview()
        }
        if candidates.isEmpty {
            isHidden = true
            return
        }
        isHidden = false
        scroll.setContentOffset(.zero, animated: false)
        for cand in candidates {
            stack.addArrangedSubview(makeChip(cand))
        }
    }

    private func makeChip(_ candidate: Candidate) -> UIView {
        let btn = UIButton(type: .system)
        btn.setTitle(candidate.display, for: .normal)
        btn.setTitleColor(textColor, for: .normal)
        btn.titleLabel?.font = .systemFont(ofSize: 14, weight: .medium)
        btn.contentEdgeInsets = UIEdgeInsets(top: 6, left: 12, bottom: 6, right: 12)
        btn.backgroundColor = chipColor
        btn.layer.cornerRadius = 14
        btn.layer.borderWidth = 1
        btn.layer.borderColor = chipBorder.cgColor
        // capture into a value the closure can keep without retaining self
        btn.addAction(UIAction { [weak self] _ in
            self?.onCandidatePicked?(candidate)
        }, for: .touchUpInside)
        return btn
    }
}
