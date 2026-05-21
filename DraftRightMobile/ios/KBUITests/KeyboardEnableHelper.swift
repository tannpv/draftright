import XCTest

/// Drives the system Settings app to add the DraftRight keyboard and grant
/// Full Access. iOS provides no programmatic API to enable a third-party
/// keyboard, so the UI tests must walk Settings once per simulator boot.
///
/// Idempotent: if the keyboard is already present the add step is skipped.
enum KeyboardEnableHelper {

    static let keyboardDisplayName = "DraftRight V2"

    static func enableIfNeeded(timeout: TimeInterval = 30) {
        let settings = XCUIApplication(bundleIdentifier: "com.apple.Preferences")
        settings.launch()
        defer { settings.terminate() }

        navigateToKeyboardsList(in: settings, timeout: timeout)

        // Already added?
        if settings.staticTexts[keyboardDisplayName].waitForExistence(timeout: 3) {
            grantFullAccess(in: settings, timeout: timeout)
            return
        }

        addKeyboard(in: settings, timeout: timeout)
        grantFullAccess(in: settings, timeout: timeout)
    }

    private static func navigateToKeyboardsList(in settings: XCUIApplication, timeout: TimeInterval) {
        tapCell(settings, "General", timeout: timeout)
        tapCell(settings, "Keyboard", timeout: timeout)
        // The first "Keyboards" row leads to the list + count.
        tapCell(settings, "Keyboards", timeout: timeout)
    }

    private static func addKeyboard(in settings: XCUIApplication, timeout: TimeInterval) {
        tapCell(settings, "Add New Keyboard…", timeout: timeout)
        tapCell(settings, keyboardDisplayName, timeout: timeout)
    }

    private static func grantFullAccess(in settings: XCUIApplication, timeout: TimeInterval) {
        // Open the keyboard's detail row, flip Allow Full Access, confirm alert.
        tapCell(settings, keyboardDisplayName, timeout: timeout)
        let toggle = settings.switches["Allow Full Access"]
        if toggle.waitForExistence(timeout: timeout), (toggle.value as? String) == "0" {
            toggle.tap()
            let allow = settings.alerts.buttons["Allow"]
            if allow.waitForExistence(timeout: 5) { allow.tap() }
        }
    }

    /// Tap a row by label, searching the whole element tree (iOS 26
    /// Settings mixes table + collection views) and scrolling to reveal
    /// off-screen rows. A navigable row is a `.button` / `.cell`; section
    /// headers are bare `.staticText`. Prefer the tappable element and
    /// fall back to firstMatch so duplicate labels (e.g. the "Keyboards"
    /// row vs. its section header) don't ambiguate.
    private static func tapCell(_ app: XCUIApplication, _ label: String, timeout: TimeInterval) {
        if let el = findRow(app, label, timeout: 5) {
            tapWhenHittable(app, el)
            return
        }
        for _ in 0..<6 {
            app.swipeUp()
            if let el = findRow(app, label, timeout: 2) {
                tapWhenHittable(app, el)
                return
            }
        }
        XCTFail("Settings row not found: \(label)")
    }

    private static func findRow(_ app: XCUIApplication, _ label: String, timeout: TimeInterval) -> XCUIElement? {
        let button = app.buttons[label]
        if button.waitForExistence(timeout: timeout) { return button }
        let cell = app.cells[label]
        if cell.exists { return cell }
        let texts = app.staticTexts.matching(identifier: label)
        if texts.count > 0 { return texts.element(boundBy: texts.count - 1) }
        let text = app.staticTexts[label]
        return text.exists ? text : nil
    }

    private static func tapWhenHittable(_ app: XCUIApplication, _ el: XCUIElement) {
        var tries = 0
        while !el.isHittable && tries < 4 {
            app.swipeUp()
            tries += 1
        }
        el.tap()
    }
}
