import UIKit

// MARK: - Key model

enum KeyCode {
    case char
    case backspace
    case shift
    case enter
    case space
    case symbols   // "?123"
    case alpha     // "ABC"
    case symbols2  // "#+=
    case globe     // switch keyboard
}

struct KeyDef {
    let label: String
    let code: KeyCode
    let widthWeight: CGFloat

    init(_ label: String, _ code: KeyCode, _ widthWeight: CGFloat = 1.0) {
        self.label = label
        self.code = code
        self.widthWeight = widthWeight
    }
}

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
}

// MARK: - QwertyKeyboardView

final class QwertyKeyboardView: UIView {

    weak var delegate: KeyboardActionDelegate?

    private let rowHeight: CGFloat = 42
    private let keyMargin: CGFloat = 3
    private let keyRadius: CGFloat = 5

    private var shiftState: ShiftState = .off
    private var currentLayer = 0 // 0=alpha, 1=symbols1, 2=symbols2
    private var lastShiftTap: TimeInterval = 0

    private var backspaceTimer: Timer?

    // Key preview
    private var previewLabel: UILabel?

    // MARK: Colors

    private var keyColor: UIColor = .white
    private var keyColorSpecial: UIColor = .systemGray4
    private var keyColorPressed: UIColor = .systemGray3
    private var keyTextColor: UIColor = .black
    private var keyboardBgColor: UIColor = .systemGray6

    // MARK: Key layouts

    private let alphaRows: [[KeyDef]] = [
        [
            KeyDef("q", .char), KeyDef("w", .char), KeyDef("e", .char),
            KeyDef("r", .char), KeyDef("t", .char), KeyDef("y", .char),
            KeyDef("u", .char), KeyDef("i", .char), KeyDef("o", .char),
            KeyDef("p", .char),
        ],
        [
            KeyDef("a", .char), KeyDef("s", .char), KeyDef("d", .char),
            KeyDef("f", .char), KeyDef("g", .char), KeyDef("h", .char),
            KeyDef("j", .char), KeyDef("k", .char), KeyDef("l", .char),
        ],
        [
            KeyDef("\u{2B06}", .shift, 1.5),
            KeyDef("z", .char), KeyDef("x", .char), KeyDef("c", .char),
            KeyDef("v", .char), KeyDef("b", .char), KeyDef("n", .char),
            KeyDef("m", .char),
            KeyDef("\u{232B}", .backspace, 1.5),
        ],
        [
            KeyDef("?123", .symbols, 1.5),
            KeyDef("\u{1F310}", .globe, 1.0),
            KeyDef(",", .char, 1.0),
            KeyDef(" ", .space, 5.0),
            KeyDef(".", .char, 1.0),
            KeyDef("\u{21B5}", .enter, 1.5),
        ],
    ]

    private let symbols1Rows: [[KeyDef]] = [
        [
            KeyDef("1", .char), KeyDef("2", .char), KeyDef("3", .char),
            KeyDef("4", .char), KeyDef("5", .char), KeyDef("6", .char),
            KeyDef("7", .char), KeyDef("8", .char), KeyDef("9", .char),
            KeyDef("0", .char),
        ],
        [
            KeyDef("@", .char), KeyDef("#", .char), KeyDef("$", .char),
            KeyDef("%", .char), KeyDef("&", .char), KeyDef("-", .char),
            KeyDef("+", .char), KeyDef("(", .char), KeyDef(")", .char),
        ],
        [
            KeyDef("#+=", .symbols2, 1.5),
            KeyDef("!", .char), KeyDef("\"", .char), KeyDef("'", .char),
            KeyDef(":", .char), KeyDef(";", .char), KeyDef("/", .char),
            KeyDef("?", .char),
            KeyDef("\u{232B}", .backspace, 1.5),
        ],
        [
            KeyDef("ABC", .alpha, 1.5),
            KeyDef("\u{1F310}", .globe, 1.0),
            KeyDef(",", .char, 1.0),
            KeyDef(" ", .space, 5.0),
            KeyDef(".", .char, 1.0),
            KeyDef("\u{21B5}", .enter, 1.5),
        ],
    ]

    private let symbols2Rows: [[KeyDef]] = [
        [
            KeyDef("~", .char), KeyDef("`", .char), KeyDef("|", .char),
            KeyDef("\u{2022}", .char), KeyDef("\u{221A}", .char), KeyDef("\u{03C0}", .char),
            KeyDef("\u{00F7}", .char), KeyDef("\u{00D7}", .char), KeyDef("\u{00B6}", .char),
            KeyDef("\u{0394}", .char),
        ],
        [
            KeyDef("\u{00A3}", .char), KeyDef("\u{20AC}", .char), KeyDef("\u{00A5}", .char),
            KeyDef("^", .char), KeyDef("[", .char), KeyDef("]", .char),
            KeyDef("{", .char), KeyDef("}", .char),
        ],
        [
            KeyDef("?123", .symbols, 1.5),
            KeyDef("\u{00A9}", .char), KeyDef("\u{00AE}", .char), KeyDef("\u{2122}", .char),
            KeyDef("\\", .char), KeyDef("<", .char), KeyDef(">", .char),
            KeyDef("=", .char),
            KeyDef("\u{232B}", .backspace, 1.5),
        ],
        [
            KeyDef("ABC", .alpha, 1.5),
            KeyDef("\u{1F310}", .globe, 1.0),
            KeyDef(",", .char, 1.0),
            KeyDef(" ", .space, 5.0),
            KeyDef(".", .char, 1.0),
            KeyDef("\u{21B5}", .enter, 1.5),
        ],
    ]

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

    private func buildKeyboard() {
        subviews.forEach { $0.removeFromSuperview() }

        let rows: [[KeyDef]]
        switch currentLayer {
        case 1:  rows = symbols1Rows
        case 2:  rows = symbols2Rows
        default: rows = alphaRows
        }

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

            // Extra horizontal padding for row 1 (alpha layer) to center 9 keys under 10
            let extraPad: CGFloat = (currentLayer == 0 && rowIndex == 1) ? 16 : 0

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

        let isSpecial = keyDef.code != .char && keyDef.code != .space
        let isShiftActive = keyDef.code == .shift && shiftState != .off

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
        switch keyDef.code {
        case .char where currentLayer == 0 && shiftState != .off:
            displayLabel = keyDef.label.uppercased()
        case .space:
            displayLabel = ""
        case .shift where shiftState == .capsLock:
            displayLabel = "\u{2B06}\u{FE0F}" // ⬆️ with variation selector
        default:
            displayLabel = keyDef.label
        }

        let fontSize: CGFloat
        switch keyDef.code {
        case .symbols, .symbols2, .alpha:
            fontSize = 12
        default:
            fontSize = 18
        }

        button.setTitle(displayLabel, for: .normal)
        button.setTitleColor(keyTextColor, for: .normal)
        button.titleLabel?.font = .systemFont(ofSize: fontSize)

        // Store key def info via tag + objc association
        let wrapper = KeyDefWrapper(keyDef: keyDef, isSpecial: isSpecial, isShiftActive: isShiftActive, normalColor: bgColor)
        objc_setAssociatedObject(button, &AssociatedKeys.keyDef, wrapper, .OBJC_ASSOCIATION_RETAIN_NONATOMIC)

        button.addTarget(self, action: #selector(keyTouchDown(_:)), for: .touchDown)
        button.addTarget(self, action: #selector(keyTouchUp(_:)), for: [.touchUpInside, .touchUpOutside, .touchCancel])

        return button
    }

    // MARK: Touch handling

    @objc private func keyTouchDown(_ sender: UIButton) {
        guard let wrapper = objc_getAssociatedObject(sender, &AssociatedKeys.keyDef) as? KeyDefWrapper else { return }
        let keyDef = wrapper.keyDef

        sender.backgroundColor = keyColorPressed

        // Key preview for character keys
        if keyDef.code == .char && keyDef.label.count == 1 && keyDef.label != "," && keyDef.label != "." {
            let label = (currentLayer == 0 && shiftState != .off) ? keyDef.label.uppercased() : keyDef.label
            showKeyPreview(above: sender, label: label)
        }

        // Backspace: fire immediately + start repeat
        if keyDef.code == .backspace {
            handleKeyPress(keyDef)
            startBackspaceRepeat()
        }
    }

    @objc private func keyTouchUp(_ sender: UIButton) {
        guard let wrapper = objc_getAssociatedObject(sender, &AssociatedKeys.keyDef) as? KeyDefWrapper else { return }
        let keyDef = wrapper.keyDef

        // Restore color
        let restoreColor: UIColor
        if wrapper.isShiftActive && keyDef.code == .shift {
            restoreColor = keyColorPressed
        } else if wrapper.isSpecial {
            restoreColor = keyColorSpecial
        } else {
            restoreColor = keyColor
        }
        sender.backgroundColor = restoreColor

        dismissKeyPreview()

        if keyDef.code == .backspace {
            stopBackspaceRepeat()
        } else {
            handleKeyPress(keyDef)
        }
    }

    // MARK: Key actions

    private func handleKeyPress(_ keyDef: KeyDef) {
        switch keyDef.code {
        case .char:
            let char: String
            if currentLayer == 0 && shiftState != .off {
                char = keyDef.label.uppercased()
            } else {
                char = keyDef.label
            }
            delegate?.keyboardDidType(char)
            if shiftState == .single {
                shiftState = .off
                buildKeyboard()
            }

        case .backspace:
            delegate?.keyboardDidBackspace()

        case .enter:
            delegate?.keyboardDidEnter()

        case .space:
            delegate?.keyboardDidSpace()

        case .shift:
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

        case .symbols:
            currentLayer = 1
            buildKeyboard()

        case .symbols2:
            currentLayer = 2
            buildKeyboard()

        case .alpha:
            currentLayer = 0
            buildKeyboard()

        case .globe:
            delegate?.keyboardDidSwitchKeyboard()
        }
    }

    // MARK: Backspace repeat

    private func startBackspaceRepeat() {
        stopBackspaceRepeat()
        backspaceTimer = Timer.scheduledTimer(withTimeInterval: 0.05, repeats: true) { [weak self] _ in
            self?.delegate?.keyboardDidBackspace()
        }
        // Initial delay before repeat kicks in
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
}

// MARK: - Associated object helpers

private enum AssociatedKeys {
    static var keyDef: UInt8 = 0
}

private class KeyDefWrapper {
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
