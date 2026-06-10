import XCTest
@testable import DraftRightKeyboardCore

/// Locks the JP dict TSV format: `reading<TAB>kanji1,kanji2,...`
/// Comments (#) and blank lines skipped; rank order preserved.
final class DictPackLoaderTests: XCTestCase {

    private func tmpTSV(_ content: String) throws -> URL {
        let url = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString + ".pack")
        try content.write(to: url, atomically: true, encoding: .utf8)
        return url
    }

    func testBasicReadingToKanji() throws {
        let url = try tmpTSV("にほんご\t日本語\nかんじ\t漢字,幹事")
        let dict = try DictPackLoader.load(from: url)
        XCTAssertEqual(dict["にほんご"], ["日本語"])
        XCTAssertEqual(dict["かんじ"], ["漢字", "幹事"])
    }

    func testCommentsAndBlankLinesSkipped() throws {
        let url = try tmpTSV("# header\n\nわたし\t私\n")
        let dict = try DictPackLoader.load(from: url)
        XCTAssertEqual(dict.count, 1)
        XCTAssertEqual(dict["わたし"], ["私"])
    }

    func testRankOrderPreserved() throws {
        let url = try tmpTSV("き\t木,気,来")
        let dict = try DictPackLoader.load(from: url)
        XCTAssertEqual(dict["き"], ["木", "気", "来"])
    }

    func testMalformedLineSkipped() throws {
        let url = try tmpTSV("noTab\nき\t木")
        let dict = try DictPackLoader.load(from: url)
        XCTAssertEqual(dict.count, 1)
    }
}
