import XCTest
@testable import DraftRight

/// Client-side rewrite cache (#147). Cases mirror
/// DraftRightWindows.PureTests/RewriteCacheTests.cs so the three
/// implementations cannot drift.
final class RewriteCacheTests: XCTestCase {

    private let tone = "polished"

    // Each test builds its own instance rather than touching `.shared`, so
    // there is no cross-test state and eviction can be exercised without
    // inserting 200 entries.
    private func makeCache(max: Int = RewriteCache.defaultMaxEntries,
                           evict: Int = RewriteCache.defaultEvictFraction) -> RewriteCache {
        RewriteCache(maxEntries: max, evictFraction: evict)
    }

    func testGetReturnsNilOnMiss() {
        XCTAssertNil(makeCache().get(text: "hello", tone: tone))
    }

    func testSetThenGetRoundTrips() {
        let c = makeCache()
        c.set(text: "hello", tone: tone, result: "Hello.")
        XCTAssertEqual(c.get(text: "hello", tone: tone), "Hello.")
    }

    func testDifferentToneIsADifferentEntry() {
        let c = makeCache()
        c.set(text: "hello", tone: "polished", result: "Polished.")
        c.set(text: "hello", tone: "concise", result: "Concise.")
        XCTAssertEqual(c.get(text: "hello", tone: "polished"), "Polished.")
        XCTAssertEqual(c.get(text: "hello", tone: "concise"), "Concise.")
    }

    /// The bug this guards (#147): the old key omitted the language entirely,
    /// so translating to one language and then switching served the previous
    /// language's result — wrong output, not merely a missed hit.
    func testLanguageIsPartOfTheKey() {
        let c = makeCache()
        c.set(text: "hello", tone: "translate", result: "Xin chào", language: "Vietnamese")

        XCTAssertEqual(c.get(text: "hello", tone: "translate", language: "Vietnamese"), "Xin chào")
        XCTAssertNil(c.get(text: "hello", tone: "translate", language: "Japanese"))
        XCTAssertNil(c.get(text: "hello", tone: "translate"))
    }

    func testReSettingSameKeyOverwritesWithoutGrowing() {
        let c = makeCache()
        c.set(text: "hello", tone: tone, result: "first")
        c.set(text: "hello", tone: tone, result: "second")
        XCTAssertEqual(c.get(text: "hello", tone: tone), "second")
        XCTAssertEqual(c.count, 1)
    }

    func testClearDropsEverything() {
        let c = makeCache()
        c.set(text: "a", tone: tone, result: "A")
        c.set(text: "b", tone: tone, result: "B")
        c.clear()
        XCTAssertEqual(c.count, 0)
        XCTAssertNil(c.get(text: "a", tone: tone))
    }

    /// The previous implementation evicted `cache.keys.prefix(...)`. Swift
    /// dictionaries have no key order, so it dropped an arbitrary quarter
    /// while claiming to drop the oldest. This pins real FIFO.
    func testEvictsOldestSliceWhenFull() {
        let c = makeCache(max: 8, evict: 4)   // drop the 2 oldest on the 9th
        for i in 0..<8 { c.set(text: "t\(i)", tone: tone, result: "r\(i)") }
        XCTAssertEqual(c.count, 8)

        c.set(text: "t8", tone: tone, result: "r8")

        XCTAssertEqual(c.count, 7)                                  // 8 - 2 + 1
        XCTAssertNil(c.get(text: "t0", tone: tone))                 // oldest two gone
        XCTAssertNil(c.get(text: "t1", tone: tone))
        XCTAssertEqual(c.get(text: "t2", tone: tone), "r2")         // third-oldest survives
        XCTAssertEqual(c.get(text: "t8", tone: tone), "r8")         // newcomer present
    }

    /// FIFO by insertion, not by last use — matching Windows and Linux.
    /// Documented so a future switch to LRU is deliberate, not accidental.
    func testEvictionOrderIsInsertionNotLastUse() {
        let c = makeCache(max: 4, evict: 4)
        for i in 0..<4 { c.set(text: "t\(i)", tone: tone, result: "r\(i)") }
        _ = c.get(text: "t0", tone: tone)          // touch the oldest
        c.set(text: "t4", tone: tone, result: "r4")

        XCTAssertNil(c.get(text: "t0", tone: tone))  // still evicted despite the read
    }

    func testReSettingExistingKeyDoesNotChangeItsEvictionPosition() {
        let c = makeCache(max: 4, evict: 4)
        for i in 0..<4 { c.set(text: "t\(i)", tone: tone, result: "r\(i)") }
        c.set(text: "t0", tone: tone, result: "updated")  // overwrite oldest
        c.set(text: "t4", tone: tone, result: "r4")       // evicts 1

        XCTAssertNil(c.get(text: "t0", tone: tone))       // t0 was still oldest
        XCTAssertEqual(c.get(text: "t1", tone: tone), "r1")
    }

    func testBoundsAreClampedToAtLeastOne() {
        for bad in [0, -5] {
            let c = makeCache(max: bad)
            c.set(text: "a", tone: tone, result: "A")
            XCTAssertEqual(c.get(text: "a", tone: tone), "A")
            c.set(text: "b", tone: tone, result: "B")
            XCTAssertEqual(c.get(text: "b", tone: tone), "B")
        }
        let c = makeCache(max: 4, evict: 0)
        for i in 0..<5 { c.set(text: "t\(i)", tone: tone, result: "r\(i)") }
        XCTAssertEqual(c.get(text: "t4", tone: tone), "r4")
    }

    func testDefaultsMatchTheOtherClients() {
        XCTAssertEqual(RewriteCache.defaultMaxEntries, 200)
        XCTAssertEqual(RewriteCache.defaultEvictFraction, 4)
    }

    func testKeyShapeMatchesWindowsAndLinux() {
        XCTAssertEqual(RewriteCache.key(text: "hi", tone: "polished"), "polished::hi")
        XCTAssertEqual(RewriteCache.key(text: "hi", tone: "translate", language: "Spanish"),
                       "Spanish::translate::hi")
        // Empty language is treated as absent, not as a distinct bucket.
        XCTAssertEqual(RewriteCache.key(text: "hi", tone: "polished", language: ""), "polished::hi")
    }

    // MARK: - Tone gating

    func testOnlyTranslateUsesTargetLanguage() {
        XCTAssertTrue(Tone.translate.usesTargetLanguage)
        for t in Tone.allCases where t != .translate {
            XCTAssertFalse(t.usesTargetLanguage, "\(t) should not use a target language")
        }
    }

    func testOnlyGrammarCheckProducesUncacheableOutput() {
        XCTAssertFalse(Tone.grammarCheck.producesCacheableText)
        for t in Tone.allCases where t != .grammarCheck {
            XCTAssertTrue(t.producesCacheableText, "\(t) should be cacheable")
        }
    }
}
