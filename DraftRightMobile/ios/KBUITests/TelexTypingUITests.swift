import XCTest

/// End-to-end: the real DraftRight keyboard extension typing into a native
/// UITextField (KBTestHost). Verifies Vietnamese Telex composition + the
/// tone-cancel rule through the full IC path.
///
/// Requirements to run locally:
///  - Runner.app (which carries DraftRightKeyboard.appex) installed on the
///    target simulator so the keyboard is available in Settings.
///  - Simulator hardware keyboard OFF (I/O ▸ Keyboard ▸ Connect Hardware
///    Keyboard, or `defaults write com.apple.iphonesimulator
///    ConnectHardwareKeyboard -bool false`) — otherwise the software
///    keyboard never presents and no keys bridge.
/// DraftRight keys/toolbar expose stable `dr_*` accessibility identifiers
/// (set in QwertyKeyboardView + ToolbarView) so switching + typing are
/// deterministic.
final class TelexTypingUITests: XCTestCase {

    private let hostBundleId = "com.draftright.kbtesthost"
    private var app: XCUIApplication!

    override func setUpWithError() throws {
        continueAfterFailure = false
        // Keyboard enablement is handled out-of-band by preconfiguring the
        // simulator's AppleKeyboards to exactly [en_US, DraftRight] (see the
        // suite header). No Settings navigation needed.
        app = XCUIApplication(bundleIdentifier: hostBundleId)
    }

    override func tearDownWithError() throws {
        app?.terminate()
    }

    /// Launch the host with DraftRight pinned to `lang` (en/vi/fr) via the
    /// App Group seed, then switch the keyboard to DraftRight.
    private func launchOnDraftRight(lang: String) {
        app.launchArguments = ["-drLang", lang]
        app.launch()
        XCTAssertTrue(field.waitForExistence(timeout: 10), "host field missing")
        field.tap()
        switchToDraftRightKeyboard()
    }

    // MARK: - Helpers

    private var field: XCUIElement {
        app.textFields[HostFieldID]
    }

    /// Toggle to DraftRight via the next-keyboard (globe) key. With only
    /// two keyboards enabled this is a single tap; detected by DraftRight's
    /// stable `dr_*` accessibility ids.
    private func switchToDraftRightKeyboard() {
        // A keyboard is "present" if either the system keyboard registered
        // (app.keyboards) or DraftRight is already up (its custom input
        // view exposes dr_* at app level but is not typed as a Keyboard).
        let present = NSPredicate { [self] _, _ in
            app.keyboards.element.exists || draftRightActive
        }
        expectation(for: present, evaluatedWith: NSNull(), handler: nil)
        waitForExpectations(timeout: 15)
        if draftRightActive { return }

        // The simulator is preconfigured with exactly two keyboards
        // (en_US + DraftRight), so a single globe tap toggles to DraftRight.
        for _ in 0..<3 {
            nextKeyboardKey().tap()
            if waitForDraftRight() { return }
        }
        XCTAssertTrue(draftRightActive, "could not switch to DraftRight keyboard")
    }

    private func waitForDraftRight(timeout: TimeInterval = 4) -> Bool {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if draftRightActive { return true }
            usleep(200_000)
        }
        return false
    }

    private func nextKeyboardKey() -> XCUIElement {
        // The next-keyboard (globe) key is exposed at the app level, not
        // under `app.keyboards`.
        for label in ["Next keyboard", "next keyboard", "Emoji", "🌐"] {
            let b = app.buttons[label].firstMatch
            if b.exists { return b }
        }
        return app.buttons
            .matching(NSPredicate(format: "label CONTAINS[c] 'keyboard' OR label == 'Emoji'"))
            .firstMatch
    }

    /// DraftRight's toolbar is a sibling of the keyboard view, so query it
    /// at the app level rather than under `app.keyboards`.
    private var draftRightToolbar: XCUIElement { app.otherElements["dr_toolbar"] }
    private var draftRightActive: Bool {
        draftRightToolbar.exists || app.buttons["dr_globe"].exists
    }

    /// Resolve a DraftRight key by id (button or key element).
    private func drKey(_ ch: String) -> XCUIElement {
        let b = app.buttons["dr_key_\(ch)"].firstMatch
        return b.exists ? b : app.keys["dr_key_\(ch)"].firstMatch
    }

    /// Tap a sequence of DraftRight letter keys by their stable ids.
    private func type(_ chars: String) {
        for ch in chars {
            let target = drKey(String(ch))
            XCTAssertTrue(target.waitForExistence(timeout: 5), "DraftRight key '\(ch)' not found")
            target.tap()
        }
    }

    // MARK: - Tests

    func test_english_passthrough_typesPlainLetters() {
        launchOnDraftRight(lang: "en")
        type("viet")
        XCTAssertEqual(field.value as? String, "viet")
    }

    func test_vietnamese_telex_composesViet() {
        launchOnDraftRight(lang: "vi")
        type("vietj")
        XCTAssertEqual(field.value as? String, "việt")
    }

    func test_vietnamese_telex_toneCancel() {
        launchOnDraftRight(lang: "vi")
        type("ass")
        XCTAssertEqual(field.value as? String, "as")
    }

    func test_french_azerty_layoutAndTyping() {
        launchOnDraftRight(lang: "fr")
        // AZERTY-specific keys exist (a + z lead the top row).
        XCTAssertTrue(app.buttons["dr_key_a"].exists || app.keys["dr_key_a"].exists)
        XCTAssertTrue(app.buttons["dr_key_z"].exists || app.keys["dr_key_z"].exists)
        type("azerty")
        XCTAssertEqual(field.value as? String, "azerty")
    }

    func test_french_longPressAccent_insertsAccentedE() {
        launchOnDraftRight(lang: "fr")
        let e = drKey("e")
        XCTAssertTrue(e.waitForExistence(timeout: 5), "dr_key_e not found")
        e.press(forDuration: 0.6)
        let accent = app.buttons["dr_accent_é"].firstMatch
        XCTAssertTrue(accent.waitForExistence(timeout: 4), "accent é not shown")
        accent.tap()
        XCTAssertEqual(field.value as? String, "é")
    }
}

private let HostFieldID = "kb_target_field"
