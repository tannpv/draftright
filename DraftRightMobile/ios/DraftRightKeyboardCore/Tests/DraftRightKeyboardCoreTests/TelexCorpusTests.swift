import XCTest
@testable import DraftRightKeyboardCore

/// Broad Vietnamese Telex correctness corpus (mirrors the Android
/// TelexCorpusTest). Covers circumflex, horn/breve, đ, tone placement on di/
/// triphthongs, and the ươ cluster typed with a trailing vowel (rượu / người /
/// hươu) which previously horned the wrong vowel.
final class TelexCorpusTests: XCTestCase {

    private func type(_ keys: String) -> String {
        let c = TelexComposer()
        var last: ComposeResult = .passThrough
        for k in keys { last = c.onKey(k) }
        switch last {
        case .composing(let s), .commit(let s): return s
        default: return c.currentComposingText()
        }
    }

    func testTelexCorpus() {
        let cases: [(String, String)] = [
            ("aa", "â"), ("oo", "ô"), ("ee", "ê"),
            ("caan", "cân"), ("coon", "côn"),
            ("dd", "đ"), ("ddi", "đi"),
            ("ow", "ơ"), ("uw", "ư"), ("aw", "ă"),
            ("as", "á"), ("af", "à"), ("ar", "ả"), ("ax", "ã"), ("aj", "ạ"),
            ("hoaf", "hòa"),
            ("vietj", "việt"),
            ("tieengs", "tiếng"),
            ("nguyeenx", "nguyễn"),
            ("ddoongf", "đồng"),
            ("uow", "ươ"),
            ("dduwowngf", "đường"),
            ("nguwowif", "người"),
            ("ruwowuj", "rượu"),
            // ươ cluster with a trailing vowel — the reported regressions
            ("ruouwj", "rượu"),
            ("ruwowju", "rượu"),
            ("huouw", "hươu"),
            ("nguoiwf", "người"),
            ("muonwj", "mượn"),
            // Flexible modifier after trailing consonants (salvaged from the
            // superseded telex-circumflex branch).
            ("nguyeexn", "nguyễn"),
            ("nguyenex", "nguyễn"),
            ("nguyene", "nguyên"),
            ("vietej", "việt"),
            ("kae", "kae"),
            ("bee", "bê"),
            ("beee", "bee"),
        ]
        let failures = cases.compactMap { (keys, expected) -> String? in
            let got = type(keys)
            return got == expected ? nil : "\(keys) -> expected '\(expected)' but got '\(got)'"
        }
        XCTAssertTrue(failures.isEmpty, "Telex failures:\n" + failures.joined(separator: "\n"))
    }
}
