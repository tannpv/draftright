# VI Auto-Correct-on-Space Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a Vietnamese user finishes a word with space, auto-correct a small typo to the intended VI word, with backspace-to-undo — mirrored Kotlin (Android IME) ↔ Swift (`DraftRightKeyboardCore`), parity-guarded.

**Architecture:** Ship a real ~8.5k VI frequency dictionary generated from one source into both clients (Android `res/raw` tsv + iOS generated Swift array — same data, byte-parity guard). Add exact-match/frequency lookup to `LanguageWordList`. A pure `AutoCorrector` decides corrections (reusing the existing `fuzzyMatches` edit-distance); the IME's space hook applies it and arms a one-shot undo restored by the next backspace. Gated by an opt-in `LanguagePack.autoCorrectEnabled` trait.

**Tech Stack:** Kotlin (Android IME, JUnit), Swift (SPM `DraftRightKeyboardCore`, XCTest), Python parity guards (`scripts/check-*-parity.py` + `mobile-parity-ci.yml`).

## Global Constraints

- **Cross-language data has ONE source, guarded.** Kotlin & Swift can't share source; every duplicated data blob (dictionary, threshold constants, golden vectors) gets a `scripts/check-*-parity.py` guard wired into `.github/workflows/mobile-parity-ci.yml` (both a `paths:` entry and a `run:` step). This is RULE #1 for this repo.
- **Reuse `LanguageWordList.fuzzyMatches`** (bounded Levenshtein, ranked distance-asc/freq-desc, excludes distance-0). NEVER write a second edit-distance.
- **No hardcoded thresholds.** `MaxAutoCorrectEdits = 1`, plus `MinConfidenceFreq` and `MinConfidenceMargin` — named constants, one definition per platform, asserted equal by a guard.
- **iOS dictionary = generated Swift array, not a bundled resource.** `Package.swift` bundles no resources today (deliberately); do NOT add resource bundling. Generate `VietnameseWordList.swift` from the tsv via a committed `tools/` script.
- **Never branch on the language name.** Auto-correct is gated by `LanguagePack.autoCorrectEnabled` (default false; VI true).
- **The just-typed token is `controller.composer.currentComposingText()`** read in the space handler BEFORE the space commits it — the correction decision point on both platforms.
- Android tests run scoped to `:app`: `./gradlew :app:testDebugUnitTest --tests '<FQN>'`; results under `DraftRightMobile/build/app/test-results/`. Swift: `swift test --filter <Class>` from `DraftRightMobile/ios/DraftRightKeyboardCore`.
- Cleaned dictionary already at `scratchpad/wordlist_vi_clean.tsv` (~8,534 rows, `word<TAB>freq`, `#` header). The generator reproduces it from `tools/vi_50k_raw.txt` OR consumes the cleaned tsv directly (Task 1 decides).

---

### Task 1: Dictionary generator + Android tsv + iOS generated Swift array

The one-source dictionary. A committed Python generator emits both platform artifacts from the cleaned VI frequency list, so they cannot drift.

**Files:**
- Create: `DraftRightMobile/tools/gen_vi_wordlist.py`
- Create: `DraftRightMobile/tools/wordlist_vi_source.tsv` (the cleaned ~8.5k list — copy of `scratchpad/wordlist_vi_clean.tsv`)
- Modify (overwrite): `DraftRightMobile/android/app/src/main/res/raw/wordlist_vi.tsv`
- Create: `DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/VietnameseWordList.swift`
- Modify: `DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/Lang/VietnameseLanguagePack.swift` (use `VietnameseWordList.entries` instead of `VietnameseBootstrapWordList.entries`)

**Interfaces:**
- Produces: `VietnameseWordList.entries: [(String, Int)]` (Swift) mirroring the tsv rows; `res/raw/wordlist_vi.tsv` (Android) with the same rows. `VietnameseBootstrapWordList.bigrams` stays as-is (bigrams unchanged); only the unigram `entries` source moves.

- [ ] **Step 1: Stage the source list**

```bash
cp /private/tmp/claude-501/-opt-openAi-DraftRight/ff17ced2-0f38-4ef4-a182-4da80ff5a952/scratchpad/wordlist_vi_clean.tsv \
   DraftRightMobile/tools/wordlist_vi_source.tsv
wc -l DraftRightMobile/tools/wordlist_vi_source.tsv   # ~8538 incl. 4 header comment lines
```

- [ ] **Step 2: Write the generator**

`DraftRightMobile/tools/gen_vi_wordlist.py`:
```python
#!/usr/bin/env python3
"""Generate the VI unigram dictionary for BOTH clients from ONE source.
Source: tools/wordlist_vi_source.tsv (word<TAB>freq, # comments).
Emits:  android/.../res/raw/wordlist_vi.tsv (verbatim rows)
        ios/.../IME/VietnameseWordList.swift (static let entries: [(String,Int)])
Keeping both generated from one file is the RULE #1 single-source; the
check-vi-wordlist-parity.py guard asserts they never drift."""
import pathlib
ROOT = pathlib.Path(__file__).resolve().parents[1]  # DraftRightMobile/
SRC = ROOT / "tools" / "wordlist_vi_source.tsv"
ANDROID = ROOT / "android/app/src/main/res/raw/wordlist_vi.tsv"
SWIFT = ROOT / "ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/VietnameseWordList.swift"

def rows():
    out = []
    for ln in SRC.read_text(encoding="utf-8").splitlines():
        if not ln.strip() or ln.startswith("#"):
            continue
        w, f = ln.split("\t")
        out.append((w, int(f)))
    return out

def main():
    data = rows()
    header = ("# DraftRight Vietnamese word list — frequency-ranked single syllables.\n"
              "# Source: hermitdave/FrequencyWords (OpenSubtitles 2018), CC BY-SA 4.0.\n"
              "# Generated by tools/gen_vi_wordlist.py from tools/wordlist_vi_source.tsv — DO NOT EDIT BY HAND.\n"
              "# Format: word<TAB>frequency.\n")
    ANDROID.write_text(header + "".join(f"{w}\t{f}\n" for w, f in data), encoding="utf-8")
    lines = ["// Generated by tools/gen_vi_wordlist.py — DO NOT EDIT BY HAND.",
             "// Mirror of android/app/src/main/res/raw/wordlist_vi.tsv (byte-parity guarded).",
             "enum VietnameseWordList {",
             "    static let entries: [(word: String, freq: Int)] = ["]
    lines += [f'        ("{w}", {f}),' for w, f in data]
    lines += ["    ]", "}", ""]
    SWIFT.write_text("\n".join(lines), encoding="utf-8")
    print(f"wrote {len(data)} rows -> {ANDROID.name} + {SWIFT.name}")

if __name__ == "__main__":
    main()
```

- [ ] **Step 3: Run the generator**

Run: `cd DraftRightMobile && python3 tools/gen_vi_wordlist.py`
Expected: `wrote 8534 rows -> wordlist_vi.tsv + VietnameseWordList.swift`

- [ ] **Step 4: Point the Swift VI pack at the generated list**

In `VietnameseLanguagePack.swift`, find where it builds the word list from `VietnameseBootstrapWordList.entries` and change that reference to `VietnameseWordList.entries`. Keep `VietnameseBootstrapWordList.bigrams` for the bigram source (unchanged). (Android already loads `R.raw.wordlist_vi` via `WordListPackResolver.loadOrFallback` — the overwritten tsv is picked up automatically; no Kotlin change.)

- [ ] **Step 5: Verify both build**

Run: `cd DraftRightMobile/ios/DraftRightKeyboardCore && swift build`
Expected: builds (the 8.5k-entry array compiles).
Run: `cd DraftRightMobile/android && ./gradlew :app:compileDebugKotlin -q`
Expected: BUILD SUCCESSFUL.

- [ ] **Step 6: Commit**

```bash
git add DraftRightMobile/tools/gen_vi_wordlist.py DraftRightMobile/tools/wordlist_vi_source.tsv \
        DraftRightMobile/android/app/src/main/res/raw/wordlist_vi.tsv \
        DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/VietnameseWordList.swift \
        DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/Lang/VietnameseLanguagePack.swift
git commit -m "feat(#207): ship ~8.5k VI frequency dictionary (one source, both clients)"
```

---

### Task 2: `wordlist_vi.tsv` ↔ Swift array parity guard + CI

**Files:**
- Create: `scripts/check-vi-wordlist-parity.py`
- Modify: `.github/workflows/mobile-parity-ci.yml` (add `paths:` entries + a `run:` step)

**Interfaces:**
- Consumes: the two Task 1 artifacts. Produces: a CI gate that fails if they diverge.

- [ ] **Step 1: Write the guard (mirror `check-vi-bigram-parity.py`)**

`scripts/check-vi-wordlist-parity.py`:
```python
#!/usr/bin/env python3
"""Assert the VI unigram dictionary is byte-identical across clients.
Android res/raw/wordlist_vi.tsv rows  ==  Swift VietnameseWordList.entries."""
import pathlib, re, sys
ROOT = pathlib.Path(__file__).resolve().parents[1]
TSV = ROOT / "DraftRightMobile/android/app/src/main/res/raw/wordlist_vi.tsv"
SWIFT = ROOT / "DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/VietnameseWordList.swift"

def parse_tsv(p):
    out = []
    for ln in p.read_text(encoding="utf-8").splitlines():
        if not ln.strip() or ln.startswith("#"):
            continue
        w, f = ln.split("\t")
        out.append((w, int(f)))
    return out

def parse_swift(p):
    text = p.read_text(encoding="utf-8")
    body = re.search(r"static let entries[^=]*=\s*\[(.*?)\]\s*}", text, re.S)
    if not body:
        print("FAIL: could not find entries array in Swift", file=sys.stderr); sys.exit(1)
    pairs = re.findall(r'\(\s*"([^"]+)"\s*,\s*(\d+)\s*\)', body.group(1))
    return [(w, int(f)) for w, f in pairs]

def main():
    a, s = parse_tsv(TSV), parse_swift(SWIFT)
    if a == s:
        print(f"✓ VI wordlist parity OK — {len(a)} entries agree")
        return
    print(f"FAIL: VI wordlist mismatch — tsv={len(a)} swift={len(s)}", file=sys.stderr)
    for i, (x, y) in enumerate(zip(a, s)):
        if x != y:
            print(f"  first diff at row {i}: tsv={x} swift={y}", file=sys.stderr); break
    sys.exit(1)

if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Run it — expect PASS**

Run: `python3 scripts/check-vi-wordlist-parity.py`
Expected: `✓ VI wordlist parity OK — 8534 entries agree`

- [ ] **Step 3: Wire into CI**

In `.github/workflows/mobile-parity-ci.yml`: add the two artifact paths to the `paths:` filter (next to the existing `wordlist_vi*` entries), and add a step in the `vi-bigram-parity` job:
```yaml
      - name: VI wordlist parity (unigrams)
        run: python3 scripts/check-vi-wordlist-parity.py
```

- [ ] **Step 4: Commit**

```bash
git add scripts/check-vi-wordlist-parity.py .github/workflows/mobile-parity-ci.yml
git commit -m "test(#207): byte-parity guard for the VI unigram dictionary"
```

---

### Task 3: `LanguageWordList.frequencyOf` — Kotlin

Auto-correct needs "is this a real word / how common is it". Only prefix + fuzzy exist; add an exact-frequency lookup (0 = absent).

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/ime/LanguageWordList.kt`
- Test: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/ime/LanguageWordListFrequencyTest.kt`

**Interfaces:**
- Produces: `fun LanguageWordList.frequencyOf(word: String): Int` — interface method, default `0`; `InMemoryWordList` returns the stored frequency or 0. `word in dict` ⇔ `frequencyOf(word) > 0`.

- [ ] **Step 1: Write the failing test**

```kotlin
package com.draftright.keyboard.ime
import org.junit.Assert.assertEquals
import org.junit.Test
class LanguageWordListFrequencyTest {
    private val list = InMemoryWordList(listOf("là" to 100, "anh" to 50), emptyMap())
    @Test fun knownWordReturnsFreq() = assertEquals(100, list.frequencyOf("là"))
    @Test fun unknownWordReturnsZero() = assertEquals(0, list.frequencyOf("xyz"))
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd DraftRightMobile/android && ./gradlew :app:testDebugUnitTest --tests 'com.draftright.keyboard.ime.LanguageWordListFrequencyTest'`
Expected: FAIL — `frequencyOf` unresolved.

- [ ] **Step 3: Implement**

In `LanguageWordList` interface add: `fun frequencyOf(word: String): Int = 0`.
In `InMemoryWordList` add a freq map built from its entries (in the ctor/init) and override:
```kotlin
private val freqByWord: Map<String, Int> = entries.associate { it.first to it.second }
override fun frequencyOf(word: String): Int = freqByWord[word] ?: 0
```
(Use the ctor's existing entries list — match its actual field name from the file.)

- [ ] **Step 4: Run — verify pass**

Run: same as Step 2. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/ime/LanguageWordList.kt \
        DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/ime/LanguageWordListFrequencyTest.kt
git commit -m "feat(#207): LanguageWordList.frequencyOf exact lookup (Kotlin)"
```

---

### Task 4: `LanguageWordList.frequencyOf` — Swift (mirror)

**Files:**
- Modify: `DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/LanguageWordList.swift`
- Test: `DraftRightMobile/ios/DraftRightKeyboardCore/Tests/DraftRightKeyboardCoreTests/LanguageWordListFrequencyTests.swift`

**Interfaces:**
- Produces: `func frequencyOf(_ word: String) -> Int` on the protocol (default 0) + `InMemoryWordList` override. Mirrors Task 3.

- [ ] **Step 1: Write the failing test**

```swift
import XCTest
@testable import DraftRightKeyboardCore
final class LanguageWordListFrequencyTests: XCTestCase {
    private let list = InMemoryWordList(entries: [("là", 100), ("anh", 50)], bigrams: [:])
    func testKnownWordReturnsFreq() { XCTAssertEqual(list.frequencyOf("là"), 100) }
    func testUnknownWordReturnsZero() { XCTAssertEqual(list.frequencyOf("xyz"), 0) }
}
```
(Match `InMemoryWordList`'s real Swift initializer signature from the file — adjust arg labels if they differ.)

- [ ] **Step 2: Run — verify it fails**

Run: `cd DraftRightMobile/ios/DraftRightKeyboardCore && swift test --filter LanguageWordListFrequencyTests`
Expected: FAIL — no `frequencyOf`.

- [ ] **Step 3: Implement** — protocol requirement `func frequencyOf(_ word: String) -> Int` + `extension LanguageWordList { func frequencyOf(_ word: String) -> Int { 0 } }`; `InMemoryWordList` builds `private let freqByWord: [String:Int]` from entries and returns `freqByWord[word] ?? 0`.

- [ ] **Step 4: Run — verify pass.** Same as Step 2. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/LanguageWordList.swift \
        DraftRightMobile/ios/DraftRightKeyboardCore/Tests/DraftRightKeyboardCoreTests/LanguageWordListFrequencyTests.swift
git commit -m "feat(#207): LanguageWordList.frequencyOf exact lookup (Swift mirror)"
```

---

### Task 5: `AutoCorrector` decision logic + constants — Kotlin

The pure decision: token → optional replacement. No IME, no side effects.

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/ime/AutoCorrector.kt`
- Test: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/ime/AutoCorrectorTest.kt`

**Interfaces:**
- Consumes: `LanguageWordList.frequencyOf` (Task 3), `LanguageWordList.fuzzyMatches`.
- Produces: `object AutoCorrector` with consts `MAX_EDITS = 1`, `MIN_CONFIDENCE_FREQ = 500`, `MIN_CONFIDENCE_MARGIN = 4` and `fun correct(token: String, words: LanguageWordList): String?` — returns the corrected word, or null for no correction.

- [ ] **Step 1: Write the failing test**

```kotlin
package com.draftright.keyboard.ime
import org.junit.Assert.*
import org.junit.Test
class AutoCorrectorTest {
    // "khôgn" (typo) -> "không"; "không" is common, no rival within edit 1.
    private val words = InMemoryWordList(
        listOf("không" to 668048, "khô" to 4000, "anh" to 469245), emptyMap())
    @Test fun typoIsCorrected() = assertEquals("không", AutoCorrector.correct("khôgn", words))
    @Test fun validWordUntouched() = assertNull(AutoCorrector.correct("anh", words))
    @Test fun unknownFarWordUntouched() = assertNull(AutoCorrector.correct("zzzz", words))
    @Test fun emptyUntouched() = assertNull(AutoCorrector.correct("", words))
}
```

- [ ] **Step 2: Run — verify it fails.** `./gradlew :app:testDebugUnitTest --tests '...AutoCorrectorTest'` → FAIL.

- [ ] **Step 3: Implement**

```kotlin
package com.draftright.keyboard.ime

/** Pure typo→correction decision for auto-correct-on-space (#207).
 *  Reuses LanguageWordList.fuzzyMatches (edit distance); never reimplements it. */
object AutoCorrector {
    const val MAX_EDITS = 1
    const val MIN_CONFIDENCE_FREQ = 500
    const val MIN_CONFIDENCE_MARGIN = 4  // top candidate freq must be >= MARGIN * runner-up

    /** Returns the corrected word, or null to leave [token] as typed. */
    fun correct(token: String, words: LanguageWordList): String? {
        if (token.isEmpty() || token.any { !it.isLetter() }) return null
        if (words.frequencyOf(token) > 0) return null          // already a real word
        val cands = words.fuzzyMatches(token, MAX_EDITS, limit = 4)
        if (cands.isEmpty()) return null
        val (word, freq) = cands[0]
        if (freq < MIN_CONFIDENCE_FREQ) return null
        val runnerUp = cands.getOrNull(1)?.second ?: 0
        if (runnerUp > 0 && freq < runnerUp * MIN_CONFIDENCE_MARGIN) return null  // ambiguous
        return word
    }
}
```

- [ ] **Step 4: Run — verify pass.** Same as Step 2. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/ime/AutoCorrector.kt \
        DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/ime/AutoCorrectorTest.kt
git commit -m "feat(#207): AutoCorrector decision logic + thresholds (Kotlin)"
```

---

### Task 6: `AutoCorrector` — Swift (mirror) + shared golden vectors + consts parity

**Files:**
- Create: `DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/AutoCorrector.swift`
- Create: `parity/autocorrect-vectors.json`
- Test: `DraftRightMobile/ios/DraftRightKeyboardCore/Tests/DraftRightKeyboardCoreTests/AutoCorrectorTests.swift`
- Test (add): `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/ime/AutoCorrectorVectorsTest.kt`
- Create: `scripts/check-autocorrect-consts-parity.py`
- Modify: `.github/workflows/mobile-parity-ci.yml`

**Interfaces:**
- Produces: Swift `enum AutoCorrector` with the SAME consts + `static func correct(_ token: String, _ words: LanguageWordList) -> String?`. `parity/autocorrect-vectors.json` = `[{"dict":[["word",freq],...],"token":"...","expect":"..."|null}]` read by both platforms.

- [ ] **Step 1: Write the golden vectors**

`parity/autocorrect-vectors.json`:
```json
[
  {"dict": [["không", 668048], ["khô", 4000]], "token": "khôgn", "expect": "không"},
  {"dict": [["anh", 469245]], "token": "anh", "expect": null},
  {"dict": [["không", 668048]], "token": "zzzz", "expect": null},
  {"dict": [["ta", 342219], ["tôi", 711535]], "token": "", "expect": null}
]
```

- [ ] **Step 2: Write the failing Swift test (reads the vectors)**

```swift
import XCTest
@testable import DraftRightKeyboardCore
final class AutoCorrectorTests: XCTestCase {
    func testGoldenVectors() throws {
        let url = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("parity/autocorrect-vectors.json")
        struct V: Decodable { let dict: [[AnyCodablePair]]; let token: String; let expect: String? }
        // Simpler: decode dict as [[ [String or Int] ]] — use a hand parser below.
        let data = try Data(contentsOf: url)
        let arr = try JSONSerialization.jsonObject(with: data) as! [[String: Any]]
        for c in arr {
            let pairs = (c["dict"] as! [[Any]]).map { ($0[0] as! String, $0[1] as! Int) }
            let words = InMemoryWordList(entries: pairs, bigrams: [:])
            let got = AutoCorrector.correct(c["token"] as! String, words)
            XCTAssertEqual(got, c["expect"] as? String, "token=\(c["token"]!)")
        }
    }
}
```
(Drop the unused `V` struct; the JSONSerialization path is the one that runs. Fix the `#filePath`→repo-root climb to the actual depth — `parity/` is at repo root; count the `Tests/.../` depth from this file and adjust `deletingLastPathComponent()` calls so the final URL is `<repo>/parity/autocorrect-vectors.json`.)

- [ ] **Step 3: Run — verify it fails.** `swift test --filter AutoCorrectorTests` → FAIL (no `AutoCorrector`).

- [ ] **Step 4: Implement the Swift mirror**

```swift
/// Pure typo→correction decision (#207) — mirror of Kotlin AutoCorrector.
public enum AutoCorrector {
    public static let maxEdits = 1
    public static let minConfidenceFreq = 500
    public static let minConfidenceMargin = 4

    public static func correct(_ token: String, _ words: LanguageWordList) -> String? {
        if token.isEmpty || token.contains(where: { !$0.isLetter }) { return nil }
        if words.frequencyOf(token) > 0 { return nil }
        let cands = words.fuzzyMatches(token, maxEdits: maxEdits, limit: 4)
        guard let top = cands.first else { return nil }
        if top.1 < minConfidenceFreq { return nil }
        let runnerUp = cands.count > 1 ? cands[1].1 : 0
        if runnerUp > 0 && top.1 < runnerUp * minConfidenceMargin { return nil }
        return top.0
    }
}
```
(Match `fuzzyMatches`'s real Swift arg labels + tuple element access from the file.)

- [ ] **Step 5: Run — verify pass.** Same as Step 3. Expected: PASS.

- [ ] **Step 6: Add the Kotlin vectors test (same JSON)**

`AutoCorrectorVectorsTest.kt` — load `parity/autocorrect-vectors.json` (climb from the test file to repo root; parse with `org.json` already on the test classpath or a tiny manual split), build `InMemoryWordList` per case, assert `AutoCorrector.correct(token, words) == expect`. Run: `./gradlew :app:testDebugUnitTest --tests '...AutoCorrectorVectorsTest'` → PASS.

- [ ] **Step 7: Consts parity guard**

`scripts/check-autocorrect-consts-parity.py` — regex-extract `MAX_EDITS`/`MIN_CONFIDENCE_FREQ`/`MIN_CONFIDENCE_MARGIN` from `AutoCorrector.kt` and `maxEdits`/`minConfidenceFreq`/`minConfidenceMargin` from `AutoCorrector.swift`; assert the three values pairwise-equal; exit 1 on mismatch (mirror `check-vi-bigram-parity.py` structure). Run it → PASS. Add a `run:` step + `paths:` entries to `mobile-parity-ci.yml`.

- [ ] **Step 8: Commit**

```bash
git add DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/AutoCorrector.swift \
        DraftRightMobile/ios/DraftRightKeyboardCore/Tests/DraftRightKeyboardCoreTests/AutoCorrectorTests.swift \
        DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/ime/AutoCorrectorVectorsTest.kt \
        parity/autocorrect-vectors.json scripts/check-autocorrect-consts-parity.py \
        .github/workflows/mobile-parity-ci.yml
git commit -m "feat(#207): AutoCorrector Swift mirror + golden vectors + consts parity guard"
```

---

### Task 7: `LanguagePack.autoCorrectEnabled` trait — both platforms

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/lang/LanguagePack.kt` + `VietnameseLanguagePack.kt`
- Modify: `DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/Lang/LanguagePack.swift` + `VietnameseLanguagePack.swift`
- Test: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/lang/AutoCorrectTraitTest.kt` + Swift `AutoCorrectTraitTests.swift`

**Interfaces:**
- Produces: `LanguagePack.autoCorrectEnabled: Boolean/Bool` — default `false`, `VietnameseLanguagePack` overrides `true`. Follows the exact `convertsOnSpace` pattern (Kotlin interface `get() = false`; Swift protocol req + `public extension` default `false`).

- [ ] **Step 1: Failing tests (both):** assert `VietnameseLanguagePack().autoCorrectEnabled == true` and e.g. `EnglishLanguagePack().autoCorrectEnabled == false`. Run both → FAIL.
- [ ] **Step 2: Implement** the trait on both interfaces (default false) + VI override true, mirroring `convertsOnSpace`.
- [ ] **Step 3: Run both → PASS.**
- [ ] **Step 4: Commit** `git commit -m "feat(#207): LanguagePack.autoCorrectEnabled trait (both platforms)"`

---

### Task 8: Wire auto-correct + undo into the IME — Android

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt` (`onSpace` ~452-479, `onBackspace` ~372-404, `onCharTyped` ~351-370)
- Test: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/ime/AutoCorrectUndoTest.kt` (test the pure undo-state machine — see below)

**Interfaces:**
- Consumes: `AutoCorrector.correct` (Task 5), `LanguagePack.autoCorrectEnabled` (Task 7).
- Produces: an `AutoCorrectUndo` helper (pure, testable) holding the last `(original, corrected)`; the IME applies corrections on space and reverts on the next backspace.

- [ ] **Step 1: Write the failing test for the pure undo state**

```kotlin
package com.draftright.keyboard.ime
import org.junit.Assert.*
import org.junit.Test
class AutoCorrectUndoTest {
    @Test fun backspaceAfterCorrectionRevertsOnce() {
        val u = AutoCorrectUndo()
        u.arm(original = "khôgn", corrected = "không")
        assertEquals("khôgn", u.revertText())       // what to put back (incl. trailing space handling by caller)
        assertTrue(u.consume())                       // first backspace consumes the undo
        assertFalse(u.consume())                      // second does nothing
    }
    @Test fun anyKeyDisarms() {
        val u = AutoCorrectUndo(); u.arm("a", "á"); u.disarm(); assertFalse(u.consume())
    }
}
```

- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement `AutoCorrectUndo`** (pure): fields `original`, `corrected`, `armed`; `arm(original, corrected)` sets them; `revertText()` returns `original`; `consume(): Boolean` returns true once then disarms; `disarm()`. Then in `DraftRightIME`: in `onSpace`, BEFORE committing, if `controller?.current?.autoCorrectEnabled == true`, read `token = controller?.composer?.currentComposingText()`, compute `AutoCorrector.correct(token, wordList)`; if non-null, replace the composing text with the corrected word (via the existing commit path / `ic.commitText`) and `undo.arm(token, corrected)` — then insert the space. In `onBackspace`, if `undo.consume()`, delete the corrected word + trailing space and `commitText(undo.revertText() + " ")` instead of the normal backspace. In `onCharTyped` (and any non-backspace key), call `undo.disarm()`. Reuse the existing commit/delete helpers — do not hand-roll InputConnection edits beyond what the IME already does.
- [ ] **Step 4: Run → PASS** (the unit test covers the state machine; the InputConnection wiring is device-verified).
- [ ] **Step 5: Commit** `git commit -m "feat(#207): apply auto-correct on space + backspace-undo (Android)"`

---

### Task 9: Wire auto-correct + undo into the IME — iOS (mirror)

**Files:**
- Modify: `DraftRightMobile/ios/DraftRightKeyboard/KeyboardViewController.swift` (`keyboardDidSpace` ~542-550, `keyboardDidBackspace` ~526-530, `keyboardDidType` ~515-524)
- Test: `DraftRightMobile/ios/DraftRightKeyboardCore/Tests/DraftRightKeyboardCoreTests/AutoCorrectUndoTests.swift`

**Interfaces:**
- Consumes: `AutoCorrector.correct` (Task 6), `LanguagePack.autoCorrectEnabled` (Task 7). Produces: `AutoCorrectUndo` (Swift mirror of Task 8's pure helper, in `DraftRightKeyboardCore/IME/`).

- [ ] **Step 1: Failing Swift test** mirroring Task 8 Step 1 (`arm`/`revertText`/`consume`/`disarm`). Run `swift test --filter AutoCorrectUndoTests` → FAIL.
- [ ] **Step 2: Implement `AutoCorrectUndo.swift`** (pure mirror) + wire `KeyboardViewController`: in `keyboardDidSpace`, if `controller?.current?.autoCorrectEnabled == true`, read `controller?.composer?.currentComposingText()`, `AutoCorrector.correct(...)`; if non-nil, replace via `textDocumentProxy` (delete the composing token, insert the corrected word) then the space, and `undo.arm(...)`. In `keyboardDidBackspace`, if `undo.consume()`, delete corrected+space and `insertText(original + " ")`. In `keyboardDidType`, `undo.disarm()`. Route through `KeystrokeDispatcher`/`UIKitTextProxy` like the existing handlers.
- [ ] **Step 3: Run → PASS.**
- [ ] **Step 4: Commit** `git commit -m "feat(#207): apply auto-correct on space + backspace-undo (iOS mirror)"`

---

## Final verification (after all tasks)

- [ ] `cd DraftRightMobile/ios/DraftRightKeyboardCore && swift test` — all green.
- [ ] `cd DraftRightMobile/android && ./gradlew :app:testDebugUnitTest --tests 'com.draftright.keyboard.*'` — green.
- [ ] All parity guards: `for s in scripts/check-*-parity.py; do python3 "$s"; done` — every one ✓ (incl. the two new ones).
- [ ] Grep proves single-source: the ~8.5k VI words exist only in `wordlist_vi.tsv` + the generated `VietnameseWordList.swift` (both from `tools/gen_vi_wordlist.py`); thresholds only in the two `AutoCorrector` files.

## Out of scope (owner / later / device)

- On-device feel test (TestFlight/APK): type known typos → correction + backspace-undo. The one non-automatable step.
- Tuning `MIN_CONFIDENCE_FREQ`/`MIN_CONFIDENCE_MARGIN` from real usage.
- Downloadable larger pack; personal/learned dictionary; JP/ZH auto-correct (trait off).
