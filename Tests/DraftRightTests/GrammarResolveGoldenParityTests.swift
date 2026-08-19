import XCTest
@testable import DraftRight

/// macOS grammar resolveRange must match the shared golden vectors (#107, RULE #1).
///
/// `shared/grammar_resolve_golden_vectors.json` at the repo root is the single
/// source of truth; the Windows (C#) and Linux (Python) ports assert against the
/// same file, so the three copies of the content-first resolve logic (LLM-offset
/// gotcha, BR#49) cannot drift.
final class GrammarResolveGoldenParityTests: XCTestCase {

    private struct Expected: Decodable { let start: Int; let length: Int }
    private struct GoldenCase: Decodable {
        let name: String
        let text: String
        let original: String
        let offset: Int
        let expected: Expected?
    }
    private struct Golden: Decodable { let cases: [GoldenCase] }

    private var goldenURL: URL {
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()   // DraftRightTests
            .deletingLastPathComponent()   // Tests
            .deletingLastPathComponent()   // repo root
            .appendingPathComponent("shared/grammar_resolve_golden_vectors.json")
    }

    func testEveryCaseMatchesTheSharedVectors() throws {
        let data = try Data(contentsOf: goldenURL)
        let golden = try JSONDecoder().decode(Golden.self, from: data)
        XCTAssertFalse(golden.cases.isEmpty, "golden file is empty")

        for c in golden.cases {
            let issue = GrammarIssue(type: "grammar", offset: c.offset,
                                     length: c.original.count,
                                     original: c.original, suggestion: "X",
                                     reason: "golden")
            let range = GrammarFix.resolveRange(of: issue, in: c.text)
            if let expected = c.expected {
                guard let r = range else {
                    XCTFail("\(c.name): expected a match, got nil"); continue
                }
                let start = c.text.distance(from: c.text.startIndex, to: r.lowerBound)
                let length = c.text.distance(from: r.lowerBound, to: r.upperBound)
                XCTAssertEqual(start, expected.start, "\(c.name) (start)")
                XCTAssertEqual(length, expected.length, "\(c.name) (length)")
            } else {
                XCTAssertNil(range, "\(c.name): expected no match")
            }
        }
    }
}
