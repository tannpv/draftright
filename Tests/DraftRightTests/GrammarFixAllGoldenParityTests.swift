import XCTest
@testable import DraftRight

/// macOS grammar fixAll must match the shared golden vectors (#107, RULE #1).
///
/// `shared/grammar_fixall_golden_vectors.json` at the repo root is the single
/// source of truth; the Windows (C#) and Linux (Python) ports assert against
/// the same file, so the three copies of the apply-all logic cannot drift.
final class GrammarFixAllGoldenParityTests: XCTestCase {

    private struct IssueSpec: Decodable {
        let original: String
        let suggestion: String
        let offset: Int
    }
    private struct GoldenCase: Decodable {
        let name: String
        let text: String
        let issues: [IssueSpec]
        let expected: String
    }
    private struct Golden: Decodable { let cases: [GoldenCase] }

    private var goldenURL: URL {
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()   // DraftRightTests
            .deletingLastPathComponent()   // Tests
            .deletingLastPathComponent()   // repo root
            .appendingPathComponent("shared/grammar_fixall_golden_vectors.json")
    }

    func testEveryCaseMatchesTheSharedVectors() throws {
        let data = try Data(contentsOf: goldenURL)
        let golden = try JSONDecoder().decode(Golden.self, from: data)
        XCTAssertFalse(golden.cases.isEmpty, "golden file is empty")

        for c in golden.cases {
            let issues = c.issues.map {
                GrammarIssue(type: "grammar", offset: $0.offset,
                             length: $0.original.count,
                             original: $0.original, suggestion: $0.suggestion,
                             reason: "golden")
            }
            XCTAssertEqual(GrammarFix.fixAll(c.text, issues), c.expected, c.name)
        }
    }
}
