import XCTest
@testable import DraftRightKeyboardCore

/// Telex order-free marking must match the shared golden vectors (#207, RULE #1).
///
/// `parity/telex-order-vectors.json` at the repo root is the single source of
/// truth; Android's `TelexOrderVectorsTest` asserts against the same file, so
/// the Kotlin and Swift composers cannot drift apart.
final class TelexOrderVectorsTests: XCTestCase {

    private struct VectorCase: Decodable {
        let name: String
        let keys: String
        let expected: String
    }

    private struct Vectors: Decodable { let cases: [VectorCase] }

    // Tests/DraftRightKeyboardCoreTests/<file> -> repo root is six levels up.
    private var vectorsURL: URL {
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()   // DraftRightKeyboardCoreTests
            .deletingLastPathComponent()   // Tests
            .deletingLastPathComponent()   // DraftRightKeyboardCore
            .deletingLastPathComponent()   // ios
            .deletingLastPathComponent()   // DraftRightMobile
            .deletingLastPathComponent()   // repo root
            .appendingPathComponent("parity/telex-order-vectors.json")
    }

    func testGoldenVectors() throws {
        let data = try Data(contentsOf: vectorsURL)
        let vectors = try JSONDecoder().decode(Vectors.self, from: data)
        XCTAssertFalse(vectors.cases.isEmpty, "vectors file parsed to zero cases")
        for c in vectors.cases {
            XCTAssertEqual(TelexTyping.type(c.keys), c.expected, "case: \(c.name)")
        }
    }
}

/// Feeds a key string to a fresh `TelexComposer` one character at a time and
/// returns what the user would see. Shared by every Telex suite that works in
/// keystrokes (order-free, vectors, exhaustive) so the driver itself is one
/// source of truth.
enum TelexTyping {
    static func type(_ keys: String) -> String {
        let composer = TelexComposer()
        var last: ComposeResult = .passThrough
        for k in keys { last = composer.onKey(k) }
        switch last {
        case .composing(let s), .commit(let s): return s
        default: return composer.currentComposingText()
        }
    }
}
