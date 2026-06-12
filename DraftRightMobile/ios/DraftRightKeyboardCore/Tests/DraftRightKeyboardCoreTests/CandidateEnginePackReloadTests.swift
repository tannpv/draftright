import XCTest
@testable import DraftRightKeyboardCore

/// Regression for issue #10: the candidate-engine cache was process-wide and
/// keyed on nothing, so a pack installed mid-session was ignored until the
/// keyboard process was killed. The cache is now keyed on the resolved pack
/// URL, so a newer pack rebuilds the engine.
final class CandidateEnginePackReloadTests: XCTestCase {

    private func makeContainer() throws -> URL {
        let dir = FileManager.default.temporaryDirectory
            .appendingPathComponent("dr-pack-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(
            at: dir.appendingPathComponent("packs"), withIntermediateDirectories: true)
        return dir
    }

    private func writePack(_ container: URL, _ name: String, _ tsv: String) throws {
        try tsv.write(
            to: container.appendingPathComponent("packs/\(name)"),
            atomically: true, encoding: .utf8)
    }

    override func tearDown() {
        ChineseLanguagePack.appGroupContainer = nil
        super.tearDown()
    }

    func testEngineRebuildsWhenNewerPackInstalledMidSession() throws {
        let container = try makeContainer()
        try writePack(container, "draftright-ime-zh-v1.pack", "pengyou\t友A")
        ChineseLanguagePack.appGroupContainer = container

        let pack = ChineseLanguagePack()
        let e1 = try XCTUnwrap(pack.makeCandidateEngine())
        XCTAssertEqual(e1.suggest(composing: "pengyou").first?.text, "友A")

        // Install a newer pack mid-session — no process restart.
        try writePack(container, "draftright-ime-zh-v2.pack", "pengyou\t友B")
        let e2 = try XCTUnwrap(pack.makeCandidateEngine())
        XCTAssertEqual(
            e2.suggest(composing: "pengyou").first?.text, "友B",
            "a pack installed mid-session must be served without restarting the keyboard")
    }

    func testEngineReusedWhenPackUnchanged() throws {
        let container = try makeContainer()
        try writePack(container, "draftright-ime-zh-v1.pack", "pengyou\t友A")
        ChineseLanguagePack.appGroupContainer = container

        let pack = ChineseLanguagePack()
        let e1 = try XCTUnwrap(pack.makeCandidateEngine())
        let e2 = try XCTUnwrap(pack.makeCandidateEngine())
        XCTAssertTrue(
            (e1 as AnyObject) === (e2 as AnyObject),
            "an unchanged pack must reuse the cached engine (no re-parse per keystroke)")
    }
}
