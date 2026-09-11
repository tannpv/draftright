import XCTest
@testable import DraftRightKeyboardCore

/// Every valid Vietnamese syllable, typed in every mark/tone order, must come
/// out the same (#207, spec 2026-09-11).
///
/// The matrix is produced by `DraftRightMobile/tools/gen_telex_cases.py` — the
/// only owner of the orthography tables (RULE #1) — and the Kotlin
/// `TelexExhaustiveTest` runs the identical generator, so a divergence in
/// either port fails on both. Skipped when python3 is unavailable; both CI
/// runners have it.
final class TelexExhaustiveTests: XCTestCase {

    private struct Case: Decodable {
        let keys: String
        let expected: String
    }

    /// Divergences reported before truncating — enough to see the pattern
    /// without burying the failure message in thousands of lines.
    private static let maxReported = 50

    // Tests/DraftRightKeyboardCoreTests/<file> -> repo root is six levels up.
    private var repoRoot: URL {
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
    }

    private func generateCases() throws -> [Case]? {
        let script = repoRoot.appendingPathComponent("DraftRightMobile/tools/gen_telex_cases.py")
        let out = URL(fileURLWithPath: NSTemporaryDirectory())
            .appendingPathComponent("telex_cases_swift.json")
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
        process.arguments = ["python3", script.path, "--out", out.path]
        do {
            try process.run()
        } catch {
            return nil                                   // no python3 on this box
        }
        process.waitUntilExit()
        guard process.terminationStatus == 0 else {
            XCTFail("generator exited \(process.terminationStatus)")
            return []
        }
        return try JSONDecoder().decode([Case].self, from: Data(contentsOf: out))
    }

    func testEverySyllableInEveryOrder() throws {
        guard let cases = try generateCases() else {
            throw XCTSkip("python3 not available — the CI runners have it")
        }
        XCTAssertGreaterThan(cases.count, 1000, "generator produced a suspiciously small matrix")

        var divergences: [String] = []
        for c in cases {
            let actual = TelexTyping.type(c.keys)
            if actual != c.expected {
                divergences.append("\(c.keys) -> \(actual), expected \(c.expected)")
            }
        }
        if !divergences.isEmpty {
            let shown = divergences.prefix(Self.maxReported).joined(separator: "\n  ")
            XCTFail("\(divergences.count) divergence(s) of \(cases.count) cases:\n  \(shown)")
        }
    }
}
