import XCTest
@testable import DraftRightKeyboardCore

/// Locks the Samsung/iOS auto-capitalization policy: engage SINGLE (cap-first)
/// at the start of a field and after sentence-ending punctuation, revert to OFF
/// elsewhere, and never override a user-engaged CAPS_LOCK.
final class AutoCapitalizeTests: XCTestCase {

    // MARK: atSentenceStart

    func testEmptyOrNilContextIsSentenceStart() {
        XCTAssertTrue(AutoCapitalize.atSentenceStart(before: nil))
        XCTAssertTrue(AutoCapitalize.atSentenceStart(before: ""))
        XCTAssertTrue(AutoCapitalize.atSentenceStart(before: "   "))
    }

    func testAfterSentenceTerminatorAndSpaceIsSentenceStart() {
        XCTAssertTrue(AutoCapitalize.atSentenceStart(before: "Hello. "))
        XCTAssertTrue(AutoCapitalize.atSentenceStart(before: "Really?\n"))
        XCTAssertTrue(AutoCapitalize.atSentenceStart(before: "Wow! "))
    }

    func testMidWordOrAfterPlainSpaceIsNotSentenceStart() {
        XCTAssertFalse(AutoCapitalize.atSentenceStart(before: "hello"))
        XCTAssertFalse(AutoCapitalize.atSentenceStart(before: "hello "))   // space, no terminator
        XCTAssertFalse(AutoCapitalize.atSentenceStart(before: "Mr. Smit")) // mid-word after the period
    }

    // MARK: resolve

    func testResolveEngagesSingleAtSentenceStartWhenCapsEnabled() {
        XCTAssertEqual(AutoCapitalize.resolve(current: .off, sentenceCaps: true, atSentenceStart: true), .single)
    }

    func testResolveOffWhenNotSentenceStart() {
        XCTAssertEqual(AutoCapitalize.resolve(current: .off, sentenceCaps: true, atSentenceStart: false), .off)
    }

    func testResolveOffWhenCapsDisabled() {
        XCTAssertEqual(AutoCapitalize.resolve(current: .off, sentenceCaps: false, atSentenceStart: true), .off)
    }

    func testResolvePreservesCapsLock() {
        XCTAssertEqual(AutoCapitalize.resolve(current: .capsLock, sentenceCaps: true, atSentenceStart: false), .capsLock)
        XCTAssertEqual(AutoCapitalize.resolve(current: .capsLock, sentenceCaps: true, atSentenceStart: true), .capsLock)
    }
}
