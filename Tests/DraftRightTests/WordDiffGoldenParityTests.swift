import XCTest
@testable import DraftRight

/// macOS WordDiff must match the shared golden vectors (#107, RULE #1).
///
/// `parity/word-diff-vectors.json` at the repo root is the single source of
/// truth; the Windows (C#) and Linux (Python) ports assert against the same
/// file, so the three LCS word-diff implementations cannot drift apart.
final class WordDiffGoldenParityTests: XCTestCase {

    private struct GoldenCase: Decodable {
        let name: String
        let old: String
        let new: String
        let oldTokens: [[String]]
        let newTokens: [[String]]
        enum CodingKeys: String, CodingKey {
            case name, old, new
            case oldTokens = "old_tokens"
            case newTokens = "new_tokens"
        }
    }

    private struct Golden: Decodable { let cases: [GoldenCase] }

    // Tests/DraftRightTests/<file> -> repo root is three levels up.
    private var goldenURL: URL {
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()   // DraftRightTests
            .deletingLastPathComponent()   // Tests
            .deletingLastPathComponent()   // repo root
            .appendingPathComponent("parity/word-diff-vectors.json")
    }

    private func wire(_ kind: DiffKind) -> String {
        switch kind {
        case .equal: return "equal"
        case .deleted: return "deleted"
        case .inserted: return "inserted"
        }
    }

    func testEveryCaseMatchesTheSharedVectors() throws {
        let data = try Data(contentsOf: goldenURL)
        let golden = try JSONDecoder().decode(Golden.self, from: data)
        XCTAssertFalse(golden.cases.isEmpty, "golden file is empty")

        for c in golden.cases {
            let (oldTokens, newTokens) = WordDiff.diff(old: c.old, new: c.new)
            let gotOld = oldTokens.map { [$0.text, wire($0.kind)] }
            let gotNew = newTokens.map { [$0.text, wire($0.kind)] }
            XCTAssertEqual(gotOld, c.oldTokens, "\(c.name) (old side)")
            XCTAssertEqual(gotNew, c.newTokens, "\(c.name) (new side)")
        }
    }
}
