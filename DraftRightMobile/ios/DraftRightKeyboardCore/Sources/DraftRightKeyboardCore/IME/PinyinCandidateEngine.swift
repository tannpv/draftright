import Foundation

/// Chinese candidate engine (#211): everything the shared
/// `DictionaryCandidateEngine` does (exact word lookup + raw-pinyin fallback),
/// PLUS sentence-level pinyin — when the run-together pinyin isn't a known word,
/// segment it and build a hanzi candidate from each syllable's top match
/// ("woshi" → 我是). Wraps the shared engine rather than editing it so Japanese
/// (which uses the same `DictionaryCandidateEngine`) is untouched — Rule #1.
/// Mirror of the Kotlin `PinyinCandidateEngine`.
public final class PinyinCandidateEngine: CandidateEngine {

    private let dictionary: [String: [String]]
    private let base: DictionaryCandidateEngine

    public init(dictionary: [String: [String]]) {
        self.dictionary = dictionary
        self.base = DictionaryCandidateEngine(dictionary: dictionary)
    }

    public func suggest(composing: String, previousTokens: [String], limit: Int) -> [Candidate] {
        if composing.isEmpty { return [] }
        let baseCands = base.suggest(composing: composing, previousTokens: previousTokens, limit: limit)
        guard let segmented = segmentedCandidate(composing) else { return baseCands }

        // Insert the segmented sentence just before the raw-pinyin fallback
        // (the entry whose text == composing). Dedup by text.
        var out: [Candidate] = []
        var seen = Set<String>()
        for c in baseCands {
            if c.text == composing, seen.insert(segmented).inserted {
                out.append(Candidate(text: segmented))
            }
            if seen.insert(c.text).inserted { out.append(c) }
        }
        if seen.insert(segmented).inserted { out.append(Candidate(text: segmented)) }
        return Array(out.prefix(limit))
    }

    private func segmentedCandidate(_ pinyin: String) -> String? {
        if pinyin.count <= 1 { return nil }
        guard let segments = PinyinSegmenter.segment(pinyin), segments.count >= 2 else { return nil }
        var hanzi = ""
        for syllable in segments {
            guard let top = dictionary[syllable]?.first else { return nil }
            hanzi += top
        }
        return hanzi
    }

    public func close() { base.close() }
}
