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
    /// App Group seed, optionally pointing the rewrite backend at a local
    /// stub + dummy token, then switch the keyboard to DraftRight.
    private func launchOnDraftRight(lang: String, backend: String? = nil, token: String? = nil) {
        var args = ["-drLang", lang]
        if let backend { args += ["-drBackend", backend] }
        if let token { args += ["-drToken", token] }
        app.launchArguments = args
        app.launch()
        XCTAssertTrue(field.waitForExistence(timeout: 10), "host field missing")
        field.tap()
        switchToDraftRightKeyboard()
    }

    /// Tap DraftRight's own globe to cycle its active language (EN→VI→FR).
    private func cycleDraftRightLanguage() {
        app.buttons["dr_globe"].firstMatch.tap()
    }

    // MARK: - Helpers

    private var field: XCUIElement {
        app.textFields[HostFieldID]
    }

    private var field2: XCUIElement {
        app.textFields[HostField2ID]
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
        // under `app.keyboards`. On iOS 26 the system keyboard exposes it as a
        // `Key` element (UIAccessibilityElementKBKey), not a `Button` — so
        // query both element types.
        for label in ["Next keyboard", "next keyboard", "Emoji", "🌐"] {
            let b = app.buttons[label].firstMatch
            if b.exists { return b }
            let k = app.keys[label].firstMatch
            if k.exists { return k }
        }
        let pred = NSPredicate(format: "label CONTAINS[c] 'keyboard' OR label == 'Emoji'")
        let btn = app.buttons.matching(pred).firstMatch
        if btn.exists { return btn }
        return app.keys.matching(pred).firstMatch
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

    /// Full human journey: switch language with the in-keyboard globe and
    /// verify the composer + layout change with it (EN → VI Telex → FR
    /// AZERTY). Exercises the real cycleLanguage path, not launch-arg pins.
    func test_globe_cyclesLanguages_en_vi_fr() {
        launchOnDraftRight(lang: "en")
        // EN: plain + space bar shows the language name.
        XCTAssertEqual(spaceBarLabel(), "English")
        type("viet")
        XCTAssertEqual(field.value as? String, "viet")
        clearField()

        // EN → VI: Telex composes + label updates.
        cycleDraftRightLanguage()
        XCTAssertTrue(drKey("v").waitForExistence(timeout: 5))
        XCTAssertEqual(spaceBarLabel(), "Tiếng Việt")
        type("vietj")
        XCTAssertEqual(field.value as? String, "việt")
        clearField()

        // VI → FR: AZERTY layout (a leads the top row) + label updates.
        cycleDraftRightLanguage()
        XCTAssertTrue(drKey("a").waitForExistence(timeout: 5))
        XCTAssertEqual(spaceBarLabel(), "Français")
        type("azerty")
        XCTAssertEqual(field.value as? String, "azerty")
    }

    private func spaceBarLabel() -> String {
        app.buttons["dr_space"].firstMatch.label
    }

    /// Cross-field composer leak: a partial Telex composition in one field must
    /// not carry into the next field when focus changes while the keyboard
    /// stays up (the iOS analog of the Android Zalo "za" bug). Type "a" in VI
    /// (composer marks "a", uncommitted), switch to field 2, then type "s"
    /// (sắc tone). If the composer leaked its buffer, "s" tones the stale "a"
    /// and field 2 shows "á". Correct behaviour: a fresh composer, so field 2
    /// shows "s".
    func test_crossField_composerDoesNotLeakIntoNextField() {
        launchOnDraftRight(lang: "vi")
        type("a")
        XCTAssertTrue(field2.waitForExistence(timeout: 5), "second host field missing")
        field2.tap()
        type("s")
        XCTAssertEqual(field2.value as? String, "s",
                       "composer leaked the previous field's pending composition into field 2")
    }

    /// Tone → rewrite → replace, against a local stub backend. Skips when
    /// the stub isn't reachable (run via run-ui-tests.sh, which starts it).
    func test_tone_rewrite_and_replace() throws {
        launchOnDraftRight(lang: "en", backend: "http://localhost:8099", token: "uitest-token")
        type("hello")
        XCTAssertEqual(field.value as? String, "hello")

        // Tap the first tone button in the toolbar.
        let tone = app.buttons["dr_tone_0"].firstMatch
        XCTAssertTrue(tone.waitForExistence(timeout: 5), "tone button not found")
        tone.tap()

        let replace = app.buttons["dr_diff_replace"].firstMatch
        guard replace.waitForExistence(timeout: 10) else {
            throw XCTSkip("rewrite stub not reachable — run via ios/KBUITests/run-ui-tests.sh")
        }
        replace.tap()
        XCTAssertEqual(field.value as? String, "REWRITTEN_OK")
    }

    // Clear the host field between sub-steps using DraftRight's backspace.
    // Telex backspace can strip diacritic layers, so loop until empty.
    private func clearField() {
        let backspace = app.buttons["dr_backspace"].firstMatch
        var guardCount = 0
        while let value = field.value as? String, !value.isEmpty, guardCount < 30 {
            backspace.tap()
            guardCount += 1
        }
    }
}

private let HostFieldID = "kb_target_field"
private let HostField2ID = "kb_target_field_2"
