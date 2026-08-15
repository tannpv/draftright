import XCTest
@testable import DraftRight

/// Cross-platform parity for the RewriteCache key. The key algorithm is
/// duplicated in three languages (macOS Swift, Windows C#, Linux Python); this
/// asserts the Swift `RewriteCache.key` reproduces every vector in the shared
/// `parity/rewrite-cache-key-vectors.json` fixture — the single source of truth
/// all three platforms check against (issue #174, guarding #147/#108). If this
/// fails, the Swift key format drifted from the others: fix the code, not the
/// fixture (unless the format was changed on purpose in all three).
final class RewriteCacheKeyParityTests: XCTestCase {

    private struct Vector: Decodable {
        let text: String
        let tone: String
        let language: String?
        let expectedKey: String
    }

    private struct Fixture: Decodable {
        let vectors: [Vector]
    }

    /// Locate the repo-root `parity/` fixture by walking up from this source
    /// file — robust to the SPM build layout and to running from CI or a dev box.
    private func fixtureURL() throws -> URL {
        var dir = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        while dir.path != "/" {
            let candidate = dir.appendingPathComponent("parity/rewrite-cache-key-vectors.json")
            if FileManager.default.fileExists(atPath: candidate.path) { return candidate }
            dir = dir.deletingLastPathComponent()
        }
        throw XCTSkip("parity/rewrite-cache-key-vectors.json not found walking up from \(#filePath)")
    }

    func testKeyMatchesSharedGoldenVectors() throws {
        let data = try Data(contentsOf: try fixtureURL())
        let fixture = try JSONDecoder().decode(Fixture.self, from: data)
        XCTAssertFalse(fixture.vectors.isEmpty, "fixture must contain at least one vector")
        for v in fixture.vectors {
            XCTAssertEqual(
                RewriteCache.key(text: v.text, tone: v.tone, language: v.language),
                v.expectedKey,
                "key drift for text=\(v.text) tone=\(v.tone) language=\(String(describing: v.language))"
            )
        }
    }
}
