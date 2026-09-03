import XCTest
@testable import DraftRightKeyboardCore

/// Exact-frequency lookup backing auto-correct (#207) — mirror of Android
/// `LanguageWordListFrequencyTest`. Case-insensitive like prefix/fuzzy lookups.
final class LanguageWordListFrequencyTests: XCTestCase {

    private let list = InMemoryWordList(words: [("là", 100), ("anh", 50), ("Saigon", 20)])

    func testKnownWordReturnsFrequency() {
        XCTAssertEqual(list.frequencyOf("là"), 100)
    }

    func testUnknownWordReturnsZero() {
        XCTAssertEqual(list.frequencyOf("xyz"), 0)
    }

    func testLookupIgnoresCase() {
        XCTAssertEqual(list.frequencyOf("saigon"), 20)
        XCTAssertEqual(list.frequencyOf("ANH"), 50)
    }

    func testEmptyTokenReturnsZero() {
        XCTAssertEqual(list.frequencyOf(""), 0)
    }

    func testDuplicateEntriesKeepTheHighestFrequency() {
        let dupes = InMemoryWordList(words: [("ta", 10), ("ta", 900)])
        XCTAssertEqual(dupes.frequencyOf("ta"), 900)
    }
}
