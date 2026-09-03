import XCTest
@testable import DraftRightKeyboardCore

/// Auto-correct decisions must match the shared golden vectors (#207, RULE #1).
///
/// `parity/autocorrect-vectors.json` at the repo root is the single source of
/// truth; Android's `AutoCorrectorVectorsTest` asserts against the same file,
/// so the Kotlin and Swift deciders cannot drift apart.
final class AutoCorrectorVectorsTests: XCTestCase {

    private struct VectorCase: Decodable {
        let name: String
        let token: String
        let expect: String?
        let dict: [[Entry]]

        /// Each dict row is a heterogeneous `["word", freq]` pair.
        enum Entry: Decodable {
            case word(String)
            case freq(Int)

            init(from decoder: Decoder) throws {
                let c = try decoder.singleValueContainer()
                if let i = try? c.decode(Int.self) { self = .freq(i) }
                else { self = .word(try c.decode(String.self)) }
            }
        }
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
            .appendingPathComponent("parity/autocorrect-vectors.json")
    }

    private func entries(_ rows: [[VectorCase.Entry]]) -> [(word: String, freq: Int)] {
        rows.compactMap { row in
            guard row.count == 2,
                  case .word(let w) = row[0],
                  case .freq(let f) = row[1] else { return nil }
            return (w, f)
        }
    }

    func testGoldenVectors() throws {
        let vectors = try JSONDecoder().decode(Vectors.self, from: Data(contentsOf: vectorsURL))
        XCTAssertFalse(vectors.cases.isEmpty, "vectors file decoded to zero cases")
        for c in vectors.cases {
            let rows = entries(c.dict)
            // A dropped row would silently weaken the case, so pin the count.
            XCTAssertEqual(rows.count, c.dict.count, "malformed dict in case: \(c.name)")
            let words = InMemoryWordList(words: rows)
            XCTAssertEqual(AutoCorrector.correct(c.token, words), c.expect, "case: \(c.name)")
        }
    }
}
