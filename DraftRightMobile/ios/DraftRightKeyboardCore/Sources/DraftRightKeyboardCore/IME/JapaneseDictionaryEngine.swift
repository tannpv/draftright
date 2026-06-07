import Foundation

/// Japanese kana→kanji candidate engine — the lightweight alternative to
/// bridging librime into the iOS keyboard extension.
///
/// It composes the existing `RomajiComposer` (rōmaji→kana) with a
/// reading→kanji dictionary (a downloadable, mmap-friendly pack in the same
/// framework Latin word-lists already use). Pure Swift, no native code, no C++
/// toolchain, no GPL dictionary — so it stays inside the ~30 MB extension
/// memory budget that full librime struggles with.
///
/// Plugs into the engine-agnostic `CandidateEngine` seam, so the keyboard,
/// candidate bar, and pack registry are untouched.
public final class JapaneseDictionaryEngine: CandidateEngine {

    /// kana reading → ranked kanji candidates (best first).
    private let dictionary: [String: [String]]

    public init(dictionary: [String: [String]]) {
        self.dictionary = dictionary
    }

    public func suggest(composing: String, previousTokens: [String], limit: Int) -> [Candidate] {
        guard !composing.isEmpty else { return [] }

        // rōmaji → kana via the shared composer (fresh instance = stateless call).
        let composer = RomajiComposer()
        composer.feed(composing)
        let kana = composer.text()
        guard !kana.isEmpty else { return [] }

        var out: [Candidate] = []
        // Exact reading → kanji candidates, in stored rank order.
        if let kanji = dictionary[kana] {
            out += kanji.map { Candidate(text: $0, display: $0) }
        }
        // The plain kana is always a valid commit (user may want hiragana).
        if !out.contains(where: { $0.text == kana }) {
            out.append(Candidate(text: kana, display: kana))
        }
        return Array(out.prefix(limit))
    }
}
