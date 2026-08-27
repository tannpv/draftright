import UIKit
import DraftRightKeyboardCore

/// Japanese 12-key flick (フリック) keyboard (#212), the iOS mirror of the Android
/// `FlickKeyboardView`. Each kana key emits the tap kana on a tap and the row's
/// vowel on a flick — resolution comes from the pure `FlickGesture` + `FlickLayout`
/// primitives, so this view only owns touch plumbing + rendering. Emitted kana go
/// to the controller via `KeyboardActionDelegate.keyboardDidType`, feeding the same
/// kana→kanji engine the rōmaji path uses.
///
/// The 小゛゜ modifier key cycles the last kana's dakuten/small variant via
/// `keyboardDidKanaModifier`; the 、。 key is a flick key (tap 、 · flick 。？！).
/// Flick-preview popup is deferred (phase 3b), matching Android.
final class FlickKeyboardView: UIView {

    weak var delegate: KeyboardActionDelegate?

    private let rowHeight = KeyboardDimensions.rowHeight
    private let keyMargin = KeyboardDimensions.keyMargin
    private let keyRadius = KeyboardDimensions.keyRadius

    // A flick counts once travel passes this; below it, the touch is a tap.
    private let flickThreshold: Double = 18

    // The gojūon kana grid (row-head kana). The modifier / わ / punctuation row
    // and the function row are built separately.
    private let kanaRows: [[String]] = [
        ["あ", "か", "さ"],
        ["た", "な", "は"],
        ["ま", "や", "ら"],
    ]

    /// Flick map for the punctuation key: tap 、 · flicks 。？！ (mirrors Android).
    private let punctMap: [FlickDirection: String] = [
        .tap: "、", .left: "。", .up: "？", .right: "！",
    ]

    private var keyboardBgColor: UIColor = .clear
    private var keyColor: UIColor = .white
    private var keyColorSpecial: UIColor = .lightGray
    private var keyColorPressed: UIColor = .lightGray
    private var keyTextColor: UIColor = .black
    private let brand = BrandColor.draftRightBrand

    override init(frame: CGRect) {
        super.init(frame: frame)
        setupColors()
        build()
    }

    required init?(coder: NSCoder) {
        super.init(coder: coder)
        setupColors()
        build()
    }

    var totalHeight: CGFloat { KeyboardDimensions.keyboardHeight }

    override func traitCollectionDidChange(_ previous: UITraitCollection?) {
        super.traitCollectionDidChange(previous)
        if traitCollection.hasDifferentColorAppearance(comparedTo: previous) {
            setupColors()
            build()
        }
    }

    private func setupColors() {
        let isDark = traitCollection.userInterfaceStyle == .dark
        if isDark {
            keyboardBgColor = UIColor(red: 0.106, green: 0.106, blue: 0.122, alpha: 1)
            keyColor = UIColor(red: 0.29, green: 0.29, blue: 0.29, alpha: 1)
            keyColorSpecial = UIColor(red: 0.212, green: 0.212, blue: 0.212, alpha: 1)
            keyColorPressed = UIColor(red: 0.353, green: 0.353, blue: 0.353, alpha: 1)
            keyTextColor = .white
        } else {
            keyboardBgColor = UIColor(red: 0.925, green: 0.937, blue: 0.945, alpha: 1)
            keyColor = .white
            keyColorSpecial = UIColor(red: 0.69, green: 0.745, blue: 0.773, alpha: 1)
            keyColorPressed = UIColor(red: 0.839, green: 0.839, blue: 0.839, alpha: 1)
            keyTextColor = UIColor(red: 0.129, green: 0.129, blue: 0.129, alpha: 1)
        }
        backgroundColor = keyboardBgColor
    }

    // MARK: Build

    private func build() {
        subviews.forEach { $0.removeFromSuperview() }

        let columns = kanaRows + [["小゛゜", "わ", "、"]]
        let outer = UIStackView()
        outer.axis = .vertical
        outer.distribution = .fillEqually
        outer.spacing = 0
        outer.translatesAutoresizingMaskIntoConstraints = false
        addSubview(outer)
        NSLayoutConstraint.activate([
            outer.leadingAnchor.constraint(equalTo: leadingAnchor),
            outer.trailingAnchor.constraint(equalTo: trailingAnchor),
            outer.topAnchor.constraint(equalTo: topAnchor),
            outer.heightAnchor.constraint(equalToConstant: rowHeight * CGFloat(columns.count + 1)),
        ])

        // Three gojūon rows.
        for row in kanaRows { outer.addArrangedSubview(kanaRowView(row)) }

        // Modifier / わ / punctuation row: 小゛゜ | わ | 、。
        let mods = rowStack()
        mods.addArrangedSubview(functionKey("小゛゜") { [weak self] in self?.delegate?.keyboardDidKanaModifier() })
        mods.addArrangedSubview(kanaKey("わ"))
        mods.addArrangedSubview(flickKey("、", map: punctMap))
        outer.addArrangedSubview(mods)

        // Function row: 🌐 | space (2×) | ⌫ | ↵
        let fn = rowStack()
        fn.addArrangedSubview(functionKey("🌐") { [weak self] in self?.delegate?.keyboardDidSwitchKeyboard() })
        let space = functionKey("␣") { [weak self] in self?.delegate?.keyboardDidSpace() }
        fn.addArrangedSubview(space)
        fn.addArrangedSubview(functionKey("⌫") { [weak self] in self?.delegate?.keyboardDidBackspace() })
        fn.addArrangedSubview(functionKey("↵") { [weak self] in self?.delegate?.keyboardDidEnter() })
        space.widthAnchor.constraint(equalTo: fn.arrangedSubviews[0].widthAnchor, multiplier: 2).isActive = true
        outer.addArrangedSubview(fn)
    }

    private func rowStack() -> UIStackView {
        let s = UIStackView()
        s.axis = .horizontal
        s.distribution = .fillEqually
        s.spacing = keyMargin
        s.isLayoutMarginsRelativeArrangement = true
        s.layoutMargins = UIEdgeInsets(top: keyMargin, left: keyMargin, bottom: keyMargin, right: keyMargin)
        return s
    }

    private func kanaRowView(_ row: [String]) -> UIStackView {
        let s = rowStack()
        for head in row { s.addArrangedSubview(kanaKey(head)) }
        return s
    }

    // MARK: Keys

    private func baseKey(_ label: String, special: Bool) -> FlickKeyView {
        let key = FlickKeyView()
        key.label.text = label
        key.label.textColor = special ? brand : keyTextColor
        key.normalColor = special ? keyColorSpecial : keyColor
        key.pressedColor = keyColorPressed
        key.backgroundColor = key.normalColor
        key.layer.cornerRadius = keyRadius
        return key
    }

    /// A kana key: tap → row-head kana, flick → the direction's kana.
    private func kanaKey(_ rowHead: String) -> FlickKeyView {
        flickKey(rowHead) { FlickLayout.kanaFor(rowHead, $0) }
    }

    /// A flick key driven by a fixed direction→text map (e.g. punctuation).
    private func flickKey(_ label: String, map: [FlickDirection: String]) -> FlickKeyView {
        flickKey(label) { map[$0] }
    }

    /// A key whose emitted text depends on the flick direction. `resolve` returns
    /// the text for a direction, or nil when undefined (then fall back to the tap
    /// text). One touch-plumbing chokepoint for every flick key.
    private func flickKey(_ label: String, resolve: @escaping (FlickDirection) -> String?) -> FlickKeyView {
        let key = baseKey(label, special: false)
        key.threshold = flickThreshold
        key.onResolve = { [weak self] direction in
            let text = resolve(direction) ?? resolve(.tap)
            if let text { self?.delegate?.keyboardDidType(text) }
        }
        return key
    }

    private func functionKey(_ label: String, onTap: @escaping () -> Void) -> FlickKeyView {
        let key = baseKey(label, special: true)
        key.threshold = .greatestFiniteMagnitude // any travel still counts as a tap
        key.onResolve = { _ in onTap() }
        return key
    }
}

/// A single flick-capable key: renders a centered label, tracks the touch's
/// travel, and reports the resolved `FlickDirection` on release. Kept dumb — the
/// direction→text mapping lives in the owning `FlickKeyboardView`.
final class FlickKeyView: UIView {

    let label = UILabel()
    var normalColor: UIColor = .white
    var pressedColor: UIColor = .lightGray
    var threshold: Double = 18
    var onResolve: ((FlickDirection) -> Void)?

    private var startPoint: CGPoint = .zero

    override init(frame: CGRect) {
        super.init(frame: frame)
        label.textAlignment = .center
        label.font = .systemFont(ofSize: 22)
        label.translatesAutoresizingMaskIntoConstraints = false
        addSubview(label)
        NSLayoutConstraint.activate([
            label.centerXAnchor.constraint(equalTo: centerXAnchor),
            label.centerYAnchor.constraint(equalTo: centerYAnchor),
        ])
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) not used") }

    override func touchesBegan(_ touches: Set<UITouch>, with event: UIEvent?) {
        startPoint = touches.first?.location(in: window) ?? .zero
        backgroundColor = pressedColor
    }

    override func touchesEnded(_ touches: Set<UITouch>, with event: UIEvent?) {
        backgroundColor = normalColor
        let end = touches.first?.location(in: window) ?? startPoint
        let direction = FlickGesture.resolve(
            dx: Double(end.x - startPoint.x),
            dy: Double(end.y - startPoint.y),
            tapThreshold: threshold
        )
        onResolve?(direction)
    }

    override func touchesCancelled(_ touches: Set<UITouch>, with event: UIEvent?) {
        backgroundColor = normalColor
    }
}
