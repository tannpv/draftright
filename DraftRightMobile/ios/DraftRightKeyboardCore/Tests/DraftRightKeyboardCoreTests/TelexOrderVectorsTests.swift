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
