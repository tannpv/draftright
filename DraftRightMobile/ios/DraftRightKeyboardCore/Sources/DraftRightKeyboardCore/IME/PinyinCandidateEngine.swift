import Foundation

/// Chinese candidate engine (#211): everything the shared
/// `DictionaryCandidateEngine` does (exact word lookup + raw-pinyin fallback),
/// PLUS sentence-level pinyin — segment a run-together pinyin into a hanzi
/// candidate ("woshi" → 我是) — AND initials abbreviation — a word's syllable
/// initials commit the whole word ("nh" → 你好, "bj" → 北京). Wraps the shared
/// engine rather than editing it so Japanese (which uses the same
/// `DictionaryCandidateEngine`) is untouched — Rule #1. Mirror of the Kotlin
/// `PinyinCandidateEngine`.
public final class PinyinCandidateEngine: CandidateEngine {

    private let dictionary: [String: [String]]
    private let base: DictionaryCandidateEngine

    /// initials ("bj") → hanzi words whose syllable initials spell it (北京),
    /// derived once from the dictionary + PinyinSegmenter (no new source of truth).
    private lazy var initialsIndex: [String: [String]] = buildInitialsIndex()

    /// fuzzy-folded reading → hanzi (fold("zhongguo")="zongguo" → 中国), derived
    /// from the dictionary via PinyinFuzzy — no new source of truth.
    private lazy var foldedIndex: [String: [String]] = buildFoldedIndex()

    public init(dictionary: [String: [String]]) {
        self.dictionary = dictionary
        self.base = DictionaryCandidateEngine(dictionary: dictionary)
    }

    public func suggest(composing: String, previousTokens: [String], limit: Int) -> [Candidate] {
        if composing.isEmpty { return [] }
        let baseCands = base.suggest(composing: composing, previousTokens: previousTokens, limit: limit)

        // Candidates derived from the composing pinyin, best-first: a segmented
        // sentence, then any initials-abbreviation matches.
        var derived: [String] = []
        if let segmented = segmentedCandidate(composing) { derived.append(segmented) }
        if let abbr = initialsIndex[composing] { derived.append(contentsOf: abbr) }
        if let fuzzy = foldedIndex[PinyinFuzzy.fold(composing)] { derived.append(contentsOf: fuzzy) }
        if derived.isEmpty { return baseCands }

        // Insert derived candidates just before the raw-pinyin fallback (the
        // entry whose text == composing). Dedup by text.
        var out: [Candidate] = []
        var seen = Set<String>()
        for c in baseCands {
            if c.text == composing {
                for d in derived where seen.insert(d).inserted { out.append(Candidate(text: d)) }
            }
            if seen.insert(c.text).inserted { out.append(c) }
        }
        for d in derived where seen.insert(d).inserted { out.append(Candidate(text: d)) }
        return Array(out.prefix(limit))
    }

    private func buildFoldedIndex() -> [String: [String]] {
        var index: [String: [String]] = [:]
        for (reading, hanziList) in dictionary {
            let folded = PinyinFuzzy.fold(reading)
            if folded == reading { continue } // exact lookup already covers it
            var bucket = index[folded] ?? []
            for h in hanziList where !bucket.contains(h) { bucket.append(h) }
            index[folded] = bucket
        }
        return index
    }

    private func buildInitialsIndex() -> [String: [String]] {
        var index: [String: [String]] = [:]
        for (reading, hanziList) in dictionary {
            guard let segments = PinyinSegmenter.segment(reading), segments.count >= 2 else { continue }
            let initials = segments.map { String($0.prefix(1)) }.joined()
            var bucket = index[initials] ?? []
            for h in hanziList where !bucket.contains(h) { bucket.append(h) }
            index[initials] = bucket
        }
        return index
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
