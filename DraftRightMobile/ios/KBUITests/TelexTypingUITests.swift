import XCTest

/// PARKED — third-party keyboard extensions render in a separate process
/// whose key buttons do not bridge into the host app's accessibility tree,
/// so XCUITest cannot see or tap the DraftRight keys (verified 2026-05-20:
/// the host tree exposed only the system QuickType/AutoFill bar). The
/// IC-marshalling logic these tests aimed at is instead covered headlessly
/// by `KeystrokeDispatcherTests` in DraftRightKeyboardCore. Scaffold kept
/// for a future attempt that exposes per-key accessibility identifiers.
final class TelexTypingUITests: XCTestCase {

    private let hostBundleId = "com.draftright.kbtesthost"
    private var app: XCUIApplication!

    override func setUpWithError() throws {
        continueAfterFailure = false
        KeyboardEnableHelper.enableIfNeeded()
        app = XCUIApplication(bundleIdentifier: hostBundleId)
        app.launch()
    }

    override func tearDownWithError() throws {
        app?.terminate()
    }

    // MARK: - Helpers

    private var field: XCUIElement {
        app.textFields[HostFieldID]
    }

    /// Switch the active keyboard to DraftRight via the next-keyboard
    /// (globe) key. Cycles up to N times until the DraftRight toolbar
    /// tone buttons are visible.
    private func switchToDraftRightKeyboard(maxHops: Int = 6) {
        let globe = app.keyboards.buttons["Next keyboard"]
        XCTAssertTrue(globe.waitForExistence(timeout: 15), "globe key not visible")
        for _ in 0..<maxHops {
            if draftRightToolbarVisible { return }
            globe.tap()
        }
        XCTAssertTrue(draftRightToolbarVisible, "could not switch to DraftRight keyboard")
    }

    private var draftRightToolbarVisible: Bool {
        // Toolbar exposes the tone buttons; ✎ is the Simplify glyph.
        app.keyboards.buttons["✎"].exists || app.buttons["✎"].exists
    }

    /// Tap a sequence of letter keys on the visible keyboard.
    private func type(_ chars: String) {
        for ch in chars {
            let key = app.keyboards.keys[String(ch)]
            XCTAssertTrue(key.waitForExistence(timeout: 5), "key '\(ch)' not found")
            key.tap()
        }
    }

    /// Tap the in-keyboard globe to cycle DraftRight's own language.
    private func cycleLanguage() {
        app.keyboards.buttons["Next keyboard"].tap()
    }

    // MARK: - Tests

    func test_english_passthrough_typesPlainLetters() {
        switchToDraftRightKeyboard()
        type("viet")
        XCTAssertEqual(field.value as? String, "viet")
    }

    func test_vietnamese_telex_composesViet() {
        switchToDraftRightKeyboard()
        cycleLanguage() // EN -> VI
        type("vietj")
        XCTAssertEqual(field.value as? String, "việt")
    }

    func test_vietnamese_telex_toneCancel() {
        switchToDraftRightKeyboard()
        cycleLanguage() // EN -> VI
        type("ass")
        XCTAssertEqual(field.value as? String, "as")
    }
}

private let HostFieldID = "kb_target_field"
