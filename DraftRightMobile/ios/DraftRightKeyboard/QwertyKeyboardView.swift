import UIKit
import DraftRightKeyboardCore

// MARK: - Key model

enum ShiftState {
    case off, single, capsLock
}

// MARK: - Delegate

protocol KeyboardActionDelegate: AnyObject {
    func keyboardDidType(_ char: String)
    func keyboardDidBackspace()
    func keyboardDidEnter()
    func keyboardDidSpace()
    func keyboardDidSwitchKeyboard()
    /// Horizontal swipe on the space bar. +1 = right, -1 = left.
    /// Used to cycle through enabled keyboard languages (Samsung-style).
    func keyboardDidSpaceSwipe(direction: Int)
}

// MARK: - QwertyKeyboardView

final class QwertyKeyboardView: UIView {

    weak var delegate: KeyboardActionDelegate?

    /// Active language pack — drives the rendered layout (alpha + symbol
    /// rows) and the long-press accent map. Setting it rebuilds the
    /// keyboard and resets to the alpha layer.
    var languagePack: LanguagePack = EnglishLanguagePack() {
        didSet {
            currentLayer = 0
            shiftState = .off
            buildKeyboard()
        }
    }

    private let rowHeight: CGFloat = 42
    private let keyMargin: CGFloat = 3
    private let keyRadius: CGFloat = 5
    private let spaceCode = Int(Character(" ").unicodeScalars.first!.value)

    private var shiftState: ShiftState = .off
    private var currentLayer = 0 // 0=alpha, 1=symbols1, 2=symbols2
    private var lastShiftTap: TimeInterval = 0

    private var backspaceTimer: Timer?

    // Swipe-space cycle state. Matches Android's 80 dp threshold.
    private let spaceSwipeThresholdPx: CGFloat = 80
    private var spaceSwipeFired = false

    // Long-press accent popup.
    private var longPressTimer: Timer?
    private var longPressFired = false
    private var accentPopup: AccentPopupView?
    private let longPressDelay: TimeInterval = 0.3

    // Key preview
    private var previewLabel: UILabel?

    // MARK: Colors

    private var keyColor: UIColor = .white
    private var keyColorSpecial: UIColor = .systemGray4
    private var keyColorPressed: UIColor = .systemGray3
    private var keyTextColor: UIColor = .black
    private var keyboardBgColor: UIColor = .systemGray6

    // MARK: Key-code helpers

    private func isChar(_ code: Int) -> Bool { code >= 0 && code != spaceCode }
    private func isSpace(_ code: Int) -> Bool { code == spaceCode }

    // MARK: Init

    override init(frame: CGRect) {
        super.init(frame: frame)
        setupColors()
        buildKeyboard()
    }

    required init?(coder: NSCoder) {
        super.init(coder: coder)
        setupColors()
        buildKeyboard()
    }

    private func setupColors() {
        let isDark = traitCollection.userInterfaceStyle == .dark
        if isDark {
            keyboardBgColor = UIColor(red: 0.106, green: 0.106, blue: 0.122, alpha: 1) // #1B1B1F
            keyColor = UIColor(red: 0.29, green: 0.29, blue: 0.29, alpha: 1)            // #4A4A4A
            keyColorSpecial = UIColor(red: 0.212, green: 0.212, blue: 0.212, alpha: 1)  // #363636
            keyColorPressed = UIColor(red: 0.353, green: 0.353, blue: 0.353, alpha: 1)  // #5A5A5A
            keyTextColor = .white
        } else {
            keyboardBgColor = UIColor(red: 0.925, green: 0.937, blue: 0.945, alpha: 1) // #ECEFF1
            keyColor = .white
            keyColorSpecial = UIColor(red: 0.69, green: 0.745, blue: 0.773, alpha: 1)  // #B0BEC5
            keyColorPressed = UIColor(red: 0.839, green: 0.839, blue: 0.839, alpha: 1) // #D6D6D6
            keyTextColor = UIColor(red: 0.129, green: 0.129, blue: 0.129, alpha: 1)    // #212121
        }
        backgroundColor = keyboardBgColor
    }

    override func traitCollectionDidChange(_ previousTraitCollection: UITraitCollection?) {
        super.traitCollectionDidChange(previousTraitCollection)
        if traitCollection.hasDifferentColorAppearance(comparedTo: previousTraitCollection) {
            setupColors()
            buildKeyboard()
        }
    }

    var totalHeight: CGFloat {
        return rowHeight * 4
    }

    // MARK: Build keyboard

    private func rowsForCurrentLayer() -> [[KeyDef]] {
        switch currentLayer {
        case 1:  return languagePack.symbols1Rows
        case 2:  return languagePack.symbols2Rows
        default: return languagePack.alphaRows
        }
    }

    private func buildKeyboard() {
        subviews.forEach { $0.removeFromSuperview() }
        dismissAccentPopup()

        let rows = rowsForCurrentLayer()

        for (rowIndex, row) in rows.enumerated() {
            let rowView = UIView()
            rowView.translatesAutoresizingMaskIntoConstraints = false
            addSubview(rowView)

            NSLayoutConstraint.activate([
                rowView.leadingAnchor.constraint(equalTo: leadingAnchor),
                rowView.trailingAnchor.constraint(equalTo: trailingAnchor),
                rowView.topAnchor.constraint(equalTo: topAnchor, constant: CGFloat(rowIndex) * rowHeight),
                rowView.heightAnchor.constraint(equalToConstant: rowHeight),
            ])

            // Extra horizontal padding for the middle alpha row to centre a
            // shorter (e.g. 9-key) row under a 10-key row.
            let extraPad: CGFloat = (currentLayer == 0 && rowIndex == 1 && row.count < rows[0].count) ? 16 : 0

            let totalWeight = row.reduce(CGFloat(0)) { $0 + $1.widthWeight }
            var prevAnchor = rowView.leadingAnchor
            let totalMargins = CGFloat(row.count) * keyMargin * 2 + extraPad * 2

            for (keyIndex, keyDef) in row.enumerated() {
                let keyBtn = createKeyButton(keyDef)
                rowView.addSubview(keyBtn)

                let widthFraction = keyDef.widthWeight / totalWeight

                NSLayoutConstraint.activate([
                    keyBtn.topAnchor.constraint(equalTo: rowView.topAnchor, constant: keyMargin),
                    keyBtn.bottomAnchor.constraint(equalTo: rowView.bottomAnchor, constant: -keyMargin),
                    keyBtn.leadingAnchor.constraint(equalTo: prevAnchor, constant: keyIndex == 0 ? keyMargin + extraPad : keyMargin),
                    keyBtn.widthAnchor.constraint(equalTo: rowView.widthAnchor, multiplier: widthFraction, constant: -totalMargins * widthFraction),
                ])

                prevAnchor = keyBtn.trailingAnchor
            }
        }
    }

    // MARK: Create key button

    private func createKeyButton(_ keyDef: KeyDef) -> UIButton {
        let button = UIButton(type: .custom)
        button.translatesAutoresizingMaskIntoConstraints = false
        button.layer.cornerRadius = keyRadius
        button.clipsToBounds = true

        let code = keyDef.code
        let isSpecial = SpecialKeys.isSpecial(code)
        let isShiftActive = code == SpecialKeys.shift && shiftState != .off

        let bgColor: UIColor
        if isShiftActive {
            bgColor = keyColorPressed
        } else if isSpecial {
            bgColor = keyColorSpecial
        } else {
            bgColor = keyColor
        }
        button.backgroundColor = bgColor

        // Display label
        let displayLabel: String
        if isSpace(code) {
            displayLabel = ""
        } else if code == SpecialKeys.shift && shiftState == .capsLock {
            displayLabel = "\u{2B06}\u{FE0F}" // ⬆️ with variation selector
        } else if isChar(code) && currentLayer == 0 && shiftState != .off {
            displayLabel = keyDef.label.uppercased()
        } else {
            displayLabel = keyDef.label
        }

        let fontSize: CGFloat =
            (code == SpecialKeys.symbols || code == SpecialKeys.symbols2 || code == SpecialKeys.alpha) ? 12 : 18

        button.setTitle(displayLabel, for: .normal)
        button.setTitleColor(keyTextColor, for: .normal)
        button.titleLabel?.font = .systemFont(ofSize: fontSize)
        button.accessibilityIdentifier = accessibilityId(for: keyDef)

        let wrapper = KeyDefWrapper(keyDef: keyDef, isSpecial: isSpecial, isShiftActive: isShiftActive, normalColor: bgColor)
        objc_setAssociatedObject(button, &AssociatedKeys.keyDef, wrapper, .OBJC_ASSOCIATION_RETAIN_NONATOMIC)

        button.addTarget(self, action: #selector(keyTouchDown(_:)), for: .touchDown)
        button.addTarget(self, action: #selector(keyTouchUp(_:)), for: [.touchUpInside, .touchUpOutside, .touchCancel])

        // Space bar gets a horizontal pan recognizer for the Samsung-style
        // language-cycle swipe. cancelsTouchesInView = false so a plain
        // tap still fires .touchUpInside and types a space.
        if isSpace(code) {
            let pan = UIPanGestureRecognizer(target: self, action: #selector(spacePan(_:)))
            pan.cancelsTouchesInView = false
            pan.delegate = self
            button.addGestureRecognizer(pan)
        }

        return button
    }

    /// Stable accessibility identifiers so UI tests can locate DraftRight's
    /// own keys (which otherwise carry the same labels as the system
    /// keyboard). Letter/char keys -> "dr_key_<label>"; specials by role.
    private func accessibilityId(for keyDef: KeyDef) -> String {
        let code = keyDef.code
        switch code {
        case SpecialKeys.backspace: return "dr_backspace"
        case SpecialKeys.shift:     return "dr_shift"
        case SpecialKeys.enter:     return "dr_enter"
        case SpecialKeys.globe:     return "dr_globe"
        case SpecialKeys.symbols:   return "dr_symbols"
        case SpecialKeys.symbols2:  return "dr_symbols2"
        case SpecialKeys.alpha:     return "dr_alpha"
        default:
            if isSpace(code) { return "dr_space" }
            return "dr_key_\(keyDef.label.lowercased())"
        }
    }

    private func accentsFor(_ keyDef: KeyDef) -> [Character]? {
        guard isChar(keyDef.code), let ch = keyDef.label.lowercased().first else { return nil }
        let accents = languagePack.longPressAccents[ch]
        return (accents?.isEmpty == false) ? accents : nil
    }

    // MARK: Touch handling

    @objc private func keyTouchDown(_ sender: UIButton) {
        guard let wrapper = objc_getAssociatedObject(sender, &AssociatedKeys.keyDef) as? KeyDefWrapper else { return }
        let keyDef = wrapper.keyDef
        let code = keyDef.code

        sender.backgroundColor = keyColorPressed
        longPressFired = false

        // Key preview for character keys (skip narrow punctuation).
        if isChar(code) && keyDef.label.count == 1 && keyDef.label != "," && keyDef.label != "." {
            let label = (currentLayer == 0 && shiftState != .off) ? keyDef.label.uppercased() : keyDef.label
            showKeyPreview(above: sender, label: label)
        }

        // Long-press accent picker for keys that have accent variants.
        if let accents = accentsFor(keyDef) {
            longPressTimer?.invalidate()
            longPressTimer = Timer.scheduledTimer(withTimeInterval: longPressDelay, repeats: false) { [weak self, weak sender] _ in
                guard let self, let sender else { return }
                self.longPressFired = true
                self.dismissKeyPreview()
                self.showAccentPopup(above: sender, base: keyDef.label, accents: accents)
            }
        }

        // Backspace: fire immediately + start repeat.
        if code == SpecialKeys.backspace {
            handleKeyPress(keyDef)
            startBackspaceRepeat()
        }
    }

    @objc private func keyTouchUp(_ sender: UIButton) {
        guard let wrapper = objc_getAssociatedObject(sender, &AssociatedKeys.keyDef) as? KeyDefWrapper else { return }
        let keyDef = wrapper.keyDef
        let code = keyDef.code

        longPressTimer?.invalidate()
        longPressTimer = nil

        // Restore color
        let restoreColor: UIColor
        if wrapper.isShiftActive && code == SpecialKeys.shift {
            restoreColor = keyColorPressed
        } else if wrapper.isSpecial {
            restoreColor = keyColorSpecial
        } else {
            restoreColor = keyColor
        }
        sender.backgroundColor = restoreColor

        dismissKeyPreview()

        // When the accent picker is open it handles its own taps; leave it
        // up for the user to choose (or tap outside to dismiss).
        if longPressFired || accentPopup != nil {
            return
        }

        if code == SpecialKeys.backspace {
            stopBackspaceRepeat()
        } else if isSpace(code) && spaceSwipeFired {
            // Swipe already cycled language; suppress the trailing tap so
            // we don't insert a stray space.
            spaceSwipeFired = false
        } else {
            handleKeyPress(keyDef)
        }
    }

    @objc private func spacePan(_ gesture: UIPanGestureRecognizer) {
        switch gesture.state {
        case .began:
            spaceSwipeFired = false
        case .changed:
            guard !spaceSwipeFired else { return }
            let dx = gesture.translation(in: gesture.view).x
            if abs(dx) > spaceSwipeThresholdPx {
                spaceSwipeFired = true
                delegate?.keyboardDidSpaceSwipe(direction: dx > 0 ? 1 : -1)
            }
        default:
            break
        }
    }

    // MARK: Key actions

    private func handleKeyPress(_ keyDef: KeyDef) {
        let code = keyDef.code

        if isSpace(code) {
            delegate?.keyboardDidSpace()
            return
        }
        if isChar(code) {
            let char = (currentLayer == 0 && shiftState != .off) ? keyDef.label.uppercased() : keyDef.label
            delegate?.keyboardDidType(char)
            if shiftState == .single {
                shiftState = .off
                buildKeyboard()
            }
            return
        }

        switch code {
        case SpecialKeys.backspace:
            delegate?.keyboardDidBackspace()

        case SpecialKeys.enter:
            delegate?.keyboardDidEnter()

        case SpecialKeys.shift:
            let now = Date.timeIntervalSinceReferenceDate
            if now - lastShiftTap < 0.3 {
                shiftState = (shiftState == .capsLock) ? .off : .capsLock
            } else {
                switch shiftState {
                case .off:      shiftState = .single
                case .single:   shiftState = .off
                case .capsLock: shiftState = .off
                }
            }
            lastShiftTap = now
            buildKeyboard()

        case SpecialKeys.symbols:
            currentLayer = 1
            buildKeyboard()

        case SpecialKeys.symbols2:
            currentLayer = 2
            buildKeyboard()

        case SpecialKeys.alpha:
            currentLayer = 0
            buildKeyboard()

        case SpecialKeys.globe:
            delegate?.keyboardDidSwitchKeyboard()

        default:
            break
        }
    }

    // MARK: Backspace repeat

    private func startBackspaceRepeat() {
        stopBackspaceRepeat()
        backspaceTimer = Timer.scheduledTimer(withTimeInterval: 0.05, repeats: true) { [weak self] _ in
            self?.delegate?.keyboardDidBackspace()
        }
        backspaceTimer?.fireDate = Date().addingTimeInterval(0.4)
    }

    private func stopBackspaceRepeat() {
        backspaceTimer?.invalidate()
        backspaceTimer = nil
    }

    // MARK: Key preview popup

    private func showKeyPreview(above button: UIButton, label: String) {
        dismissKeyPreview()

        let preview = UILabel()
        preview.text = label
        preview.font = .systemFont(ofSize: 24)
        preview.textColor = keyTextColor
        preview.textAlignment = .center
        preview.backgroundColor = keyColor
        preview.layer.cornerRadius = keyRadius
        preview.layer.borderWidth = 0.5
        preview.layer.borderColor = UIColor.separator.cgColor
        preview.clipsToBounds = true
        preview.layer.shadowColor = UIColor.black.cgColor
        preview.layer.shadowOpacity = 0.15
        preview.layer.shadowOffset = CGSize(width: 0, height: 1)
        preview.layer.masksToBounds = false

        let previewSize = CGSize(width: 48, height: 52)
        let buttonFrame = button.convert(button.bounds, to: self)
        let x = buttonFrame.midX - previewSize.width / 2
        let y = buttonFrame.minY - previewSize.height - 4

        preview.frame = CGRect(x: x, y: y, width: previewSize.width, height: previewSize.height)
        addSubview(preview)
        self.previewLabel = preview
    }

    private func dismissKeyPreview() {
        previewLabel?.removeFromSuperview()
        previewLabel = nil
    }

    // MARK: Accent popup

    private func showAccentPopup(above button: UIButton, base: String, accents: [Character]) {
        dismissAccentPopup()
        let isUpper = currentLayer == 0 && shiftState != .off
        let entries = accents.map { isUpper ? String($0).uppercased() : String($0) }
        let popup = AccentPopupView(
            accents: entries,
            keyColor: keyColor,
            textColor: keyTextColor,
            radius: keyRadius,
            onSelect: { [weak self] accent in
                guard let self else { return }
                self.delegate?.keyboardDidType(accent)
                if self.shiftState == .single { self.shiftState = .off; self.buildKeyboard() }
                self.dismissAccentPopup()
            },
            onDismiss: { [weak self] in self?.dismissAccentPopup() }
        )
        // Present over the window so the strip (which sits ABOVE the pressed
        // key) is not clipped by this keyboard view's bounds for top-row keys.
        let host: UIView = window ?? self
        let buttonFrame = button.convert(button.bounds, to: host)
        popup.present(over: host, anchoredTo: buttonFrame)
        accentPopup = popup
    }

    private func dismissAccentPopup() {
        accentPopup?.removeFromSuperview()
        accentPopup = nil
    }
}

// MARK: - Gesture delegate

extension QwertyKeyboardView: UIGestureRecognizerDelegate {
    func gestureRecognizer(_ gestureRecognizer: UIGestureRecognizer,
                           shouldRecognizeSimultaneouslyWith other: UIGestureRecognizer) -> Bool {
        // Let the button keep receiving its internal touch events so a
        // plain tap on space still fires .touchUpInside (-> insert space).
        return true
    }
}

// MARK: - Associated object helpers

private enum AssociatedKeys {
    static var keyDef: UInt8 = 0
}

private final class KeyDefWrapper {
    let keyDef: KeyDef
    let isSpecial: Bool
    let isShiftActive: Bool
    let normalColor: UIColor

    init(keyDef: KeyDef, isSpecial: Bool, isShiftActive: Bool, normalColor: UIColor) {
        self.keyDef = keyDef
        self.isSpecial = isSpecial
        self.isShiftActive = isShiftActive
        self.normalColor = normalColor
    }
}
