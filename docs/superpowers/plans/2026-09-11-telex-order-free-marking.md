# Telex Order-Free Marking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Samsung-parity order-free Vietnamese marking: tone keys, `w`, doubled vowels, and trailing `d` commute at word scope — any order, any time after their targets exist — proven over every valid Vietnamese syllable.

**Architecture:** Keep `TelexComposer`'s incremental `tryCombine(buffer, incoming)` model. Add (1) a tone-lift wrapper so quality modifiers operate on a tone-free buffer and the tone is re-placed by the existing `applyTone` engine; (2) quality-ROOT matching (via the existing `UNMARK` map) so marks replace each other (`ô`+w → `ơ`); (3) a remote-đ rule. Prove with a generated syllable×ordering matrix run by both the Kotlin and Swift suites.

**Tech Stack:** Kotlin (JUnit4, gradle `:app:testDebugUnitTest`), Swift (XCTest, `swift test` in `DraftRightMobile/ios/DraftRightKeyboardCore`), Python 3 generator.

**Spec:** `docs/superpowers/specs/2026-09-11-telex-order-free-marking-design.md` — its RULE #1 pass and must-not-regress list bind every task.

## Global Constraints

- Kotlin is the reference; Swift mirrors it verbatim (file headers already say so). Never let them drift — the shared vectors + generated matrix are the guard.
- Reuse the composer's existing maps/helpers (`TONE_ROWS_LOWER`, `TONE_INDEX`, `UNTONE`, `UNMARK`, `applyTone`, `caseMap`, `findLastVowelCluster`, `findLastVowelThroughConsonants`). NO new lookup tables except ones derived in code from these (RULE #1).
- Existing tests must stay green at every commit: `TelexComposerTest`, `TelexCorpusTest`, `TelexBaseFirstTest` (+ Swift mirrors).
- Branch: `feature/telex-order-free-20260911` from `develop`. Conventional commits. PR to develop at the end (never push develop directly).
- Test-run commands:
  - Kotlin: `cd DraftRightMobile/android && ./gradlew :app:testDebugUnitTest --tests 'com.draftright.keyboard.composer.*'`
  - Swift: `cd DraftRightMobile/ios/DraftRightKeyboardCore && swift test --filter Telex`

---

### Task 1: Kotlin failing tests encoding Samsung truth

**Files:**
- Create: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/composer/TelexOrderFreeTest.kt`

**Interfaces:**
- Consumes: `TelexComposer` public API (`onKey`, `currentComposingText`) — same `type()` helper pattern as `TelexCorpusTest.kt`.
- Produces: the red suite Tasks 2–4 turn green. Case list is the device-verified Samsung study + derived permutations.

- [ ] **Step 1: Write the failing test file**

```kotlin
package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Samsung-parity order-free marking (spec 2026-09-11): tone keys, w, doubled
 * vowels, and trailing d commute at word scope. Every expected value below was
 * verified against Samsung Honeyboard VI-Telex on a Galaxy A52 (2026-09-11)
 * or derived from the same rule. Mirror of Swift TelexOrderFreeTests.
 */
class TelexOrderFreeTest {

    private fun type(keys: String): String {
        val c = TelexComposer()
        var out = ""
        for (k in keys) {
            when (val r = c.onKey(k)) {
                is ComposeResult.Composing -> out = r.text
                is ComposeResult.Commit -> out = r.text
                else -> {}
            }
        }
        return out
    }

    // --- Mechanic 1: quality modifier AFTER tone (tone-transparent) ---
    @Test fun toneThenHorn() {
        assertEquals("tưởng", type("tuongrw"))      // device-verified vs Samsung
        assertEquals("người", type("nguoifw"))      // device-verified vs Samsung
        assertEquals("mứa", type("muasw"))          // tone must MOVE u→ư target
    }

    @Test fun toneThenCircumflex() {
        assertEquals("cận", type("cansja"))         // tone j then late aa
        assertEquals("đồng", type("ddongfo"))       // tone f then late oo
    }

    // --- Mechanic 2: quality marks REPLACE each other (root matching) ---
    @Test fun hornOverridesCircumflex() {
        assertEquals("ơ", type("oow"))              // ô + w → ơ
        assertEquals("được", type("duocjdw"))       // j promotes uo→uô, w re-horns
    }

    @Test fun circumflexOverridesHorn() {
        assertEquals("ô", type("owo"))              // ơ + o → ô
    }

    // --- Mechanic 3: remote đ ---
    @Test fun remoteD() {
        assertEquals("đuoc", type("duocd"))         // trailing d pairs with initial
        assertEquals("được", type("duocdwj"))       // device-verified vs Samsung
        assertEquals("Được", type("Duocdwj"))       // case preserved from initial
        assertEquals("đùng", type("dungfd"))        // remote d after tone
    }

    @Test fun remoteDCancel() {
        assertEquals("duocd", type("duocdd"))       // second remote d reverts + literal
    }

    @Test fun remoteDDoesNotFireWithoutDInitial() {
        assertEquals("bad", type("bad"))            // d stays literal
    }

    // --- Full permutation of the study word: every mark order → Được ---
    @Test fun duocAllMarkOrders() {
        for (marks in listOf("dwj", "djw", "wdj", "wjd", "jdw", "jwd")) {
            assertEquals("marks=$marks", "được", type("duoc$marks"))
        }
    }

    // --- Must-not-regress spot checks (already green today; guard rail) ---
    @Test fun existingBehavioursHold() {
        assertEquals("tưởng", type("tuongwr"))
        assertEquals("không", type("khongo"))
        assertEquals("hòa", type("hoaf"))
        assertEquals("nguyễn", type("nguyeenx"))
        assertEquals("rượu", type("ruwowuj"))
    }
}
```

- [ ] **Step 2: Run to verify the new mechanics fail and the guard rail passes**

Run: `cd DraftRightMobile/android && ./gradlew :app:testDebugUnitTest --tests 'com.draftright.keyboard.composer.TelexOrderFreeTest'`
Expected: FAIL — `toneThenHorn`, `toneThenCircumflex`, `hornOverridesCircumflex`, `circumflexOverridesHorn`, `remoteD*`, `duocAllMarkOrders` fail; `existingBehavioursHold` passes. If any guard-rail case fails, STOP — the baseline assumption is wrong; investigate before proceeding.

- [ ] **Step 3: Commit**

```bash
git add DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/composer/TelexOrderFreeTest.kt
git commit -m "test(#207): failing Samsung-parity order-free marking cases (Kotlin)"
```

---

### Task 2: Kotlin `liftTone` + derived tone-key map

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/composer/TelexComposer.kt` (companion object)
- Test: same `TelexOrderFreeTest.kt` (add a nested unit test)

**Interfaces:**
- Produces: `internal fun liftTone(buffer: String): Pair<String, Char?>` — buffer with its (single) tone stripped to the root vowel + the tone KEY (`s/f/r/x/j`) that was on it, or `(buffer, null)` when untoned. Task 3 consumes it.

- [ ] **Step 1: Add failing unit test to `TelexOrderFreeTest.kt`**

```kotlin
    @Test fun liftToneStripsAndReports() {
        assertEquals("tuong" to 'r', TelexComposer.liftTone("tuỏng"))
        assertEquals("tương" to 'r', TelexComposer.liftTone("tưởng"))  // quality marks stay
        assertEquals("duoc" to null, TelexComposer.liftTone("duoc"))
        assertEquals("Hoa" to 'f', TelexComposer.liftTone("Hòa"))      // case preserved
    }
```

- [ ] **Step 2: Run to verify it fails** (`liftTone` unresolved) — same gradle command.

- [ ] **Step 3: Implement in the companion object** (below `UNTONE`; both new members derived from existing maps — no new literals):

```kotlin
        /**
         * Tone KEY (s/f/r/x/j) for each toned char — derived from
         * TONE_ROWS_LOWER + TONE_INDEX so it can never drift from them.
         */
        private val TONE_KEY_OF: Map<Char, Char> = buildMap {
            val keyByIdx = TONE_INDEX.entries.associate { (k, v) -> v to k }
            for (row in TONE_ROWS_LOWER.values) {
                for (idx in row.indices) put(row[idx], keyByIdx.getValue(idx))
            }
        }

        /**
         * [buffer] with its tone removed + the tone key that was on it, or
         * null when untoned. Order-free marking (spec 2026-09-11): quality
         * modifiers run on the bare buffer and the tone is re-placed by
         * applyTone afterwards, so placement is always recomputed.
         */
        internal fun liftTone(buffer: String): Pair<String, Char?> {
            for (i in buffer.indices.reversed()) {
                val c = buffer[i]
                val key = TONE_KEY_OF[c.lowercaseChar()] ?: continue
                val root = UNTONE.getValue(c.lowercaseChar())
                val lifted = buffer.substring(0, i) +
                    caseMap(root, c.isUpperCase()) + buffer.substring(i + 1)
                return lifted to key
            }
            return buffer to null
        }
```

- [ ] **Step 4: Run the lift test — PASS; full composer suites still green.**

Run: `./gradlew :app:testDebugUnitTest --tests 'com.draftright.keyboard.composer.*'`

- [ ] **Step 5: Commit** — `git commit -am "feat(#207): liftTone + derived tone-key map (Kotlin)"`

---

### Task 3: Kotlin tone-transparent + root-matching modifiers

**Files:**
- Modify: `TelexComposer.kt` — the `'w'` branch and the circumflex section of `tryCombine`, plus `applyHornOrBreve`.

**Interfaces:**
- Consumes: `liftTone` (Task 2), existing `applyTone`, `UNMARK`.
- Produces: `toneThenHorn`, `toneThenCircumflex`, `hornOverridesCircumflex`, `circumflexOverridesHorn` green. No signature changes visible outside the companion.

- [ ] **Step 1: Add a quality-root helper** (near `caseMap`):

```kotlin
        /** Quality-mark-insensitive root: ô/ơ→o, â/ă→a, ê→e, ư→u, đ→d. */
        private fun qualityRoot(c: Char): Char = UNMARK[c.lowercaseChar()] ?: c.lowercaseChar()
```

- [ ] **Step 2: Wrap the `'w'` branch of `tryCombine` with tone lifting:**

Replace:
```kotlin
            if (low == 'w') {
                tryCancelHornBreve(buffer, incoming)?.let { return it }
                return applyHornOrBreve(buffer, incoming.isUpperCase())
            }
```
with:
```kotlin
            if (low == 'w') {
                // Order-free marking: lift the tone, run the existing horn/
                // breve logic on the bare buffer, re-place the tone. Placement
                // is recomputed by applyTone, so "tuỏng"+w lands as tưởng.
                val (bare, tone) = liftTone(buffer)
                val applied = tryCancelHornBreve(bare, incoming)
                    ?: applyHornOrBreve(bare, incoming.isUpperCase())
                    ?: return null
                return if (tone == null) applied else applyTone(applied, tone)
            }
```

- [ ] **Step 3: Make `applyHornOrBreve` match quality ROOTS** so marks replace each other. Three edits inside it:
  - the `uo`-pair scan: replace both `buffer[i].lowercaseChar() == 'u'` with `qualityRoot(buffer[i]) == 'u'` and `buffer[i + 1].lowercaseChar() == 'o'` with `qualityRoot(buffer[i + 1]) == 'o'` (covers `uô` from tone-promotion: `duocjdw`).
  - the immediate-last single-vowel `when`: switch on `qualityRoot(last)` instead of `last.lowercaseChar()`.
  - the lookback single-vowel `when`: switch on `qualityRoot(vowelChar)` instead of `vowelChar.lowercaseChar()`.

- [ ] **Step 4: Same treatment for the circumflex section of `tryCombine`** (the `replacement` block, lines ~85–132). Wrap the whole section:

```kotlin
            // Double-vowel circumflex: aa/oo/ee — tone-lifted and root-matched
            // like 'w' above so it commutes with tones and replaces horns.
            val replacement = when (low) {
                'a' -> 'â'; 'o' -> 'ô'; 'e' -> 'ê'
                else -> return null
            }
            val (bare, tone) = liftTone(buffer)
            val applied = applyCircumflexOrCancel(bare, low, replacement, incoming) ?: return null
            return if (tone == null) applied else applyTone(applied, tone)
```
and move the existing immediate-last + cluster-lookback circumflex logic verbatim into a new `private fun applyCircumflexOrCancel(buffer: String, low: Char, replacement: Char, incoming: Char): String?` — with its two matchers upgraded: `last.lowercaseChar() == low` becomes `qualityRoot(last) == low` (so `ơ`+o → `ô`), while the CANCEL matcher (`== replacement`) stays exact (only a real `ô` cancels to `o`). Same for the cluster-lookback pair inside it.

- [ ] **Step 5: Run the suite.**
Expected: `toneThenHorn`, `toneThenCircumflex`, `hornOverridesCircumflex`, `circumflexOverridesHorn` PASS; `remoteD*` still FAIL; all pre-existing Telex suites green. If `TelexCorpusTest`/`TelexBaseFirstTest` regress, fix before committing — the must-not-regress list is binding.

- [ ] **Step 6: Commit** — `git commit -am "feat(#207): tone-transparent, root-matching quality modifiers (Kotlin)"`

---

### Task 4: Kotlin remote đ

**Files:**
- Modify: `TelexComposer.kt` — the `'d'` branch of `tryCombine`.

**Interfaces:**
- Produces: `remoteD`, `remoteDCancel`, `remoteDDoesNotFireWithoutDInitial`, `duocAllMarkOrders` green → Task 1 fully green.

- [ ] **Step 1: Extend the `'d'` branch.** After the two existing adjacent rules (last==`đ`, last==`d`), before `return null`:

```kotlin
                // Remote đ (order-free marking): a trailing d on a d-initial
                // word converts the INITIAL to đ — unambiguous because no
                // Vietnamese syllable ends in d. A second remote d reverts,
                // mirroring the adjacent dd cancel.
                val first = buffer.first()
                val restHasD = buffer.drop(1).any { qualityRoot(it) == 'd' }
                val hasVowel = buffer.any { TelexState.isVowelLike(it) }
                if (first.lowercaseChar() == 'd' && hasVowel && !restHasD) {
                    return caseMap('đ', first.isUpperCase()) + buffer.substring(1)
                }
                if (first.lowercaseChar() == 'đ' && buffer.length > 1 && !restHasD) {
                    return caseMap('d', first.isUpperCase()) + buffer.substring(1) + incoming
                }
                return null
```
(Note: `bufferHasTonableVowel` is about tone targets; here plain `isVowelLike` suffices — toned vowels are `isVowelLike` too. Verify with `dungfd`: buffer `dùng` — `ù` must count. If `TelexState.isVowelLike` returns false for toned chars, use `buffer.any { TelexState.isVowelLike(it) || UNTONE.containsKey(it.lowercaseChar()) }` instead — check `TelexState` first and use whichever is true.)

- [ ] **Step 2: Run — ALL of `TelexOrderFreeTest` green; every other composer suite green.**

- [ ] **Step 3: Commit** — `git commit -am "feat(#207): remote đ — trailing d marks the d-initial (Kotlin)"`

---

### Task 5: Swift mirror (Tasks 1–4 ported)

**Files:**
- Create: `DraftRightMobile/ios/DraftRightKeyboardCore/Tests/DraftRightKeyboardCoreTests/TelexOrderFreeTests.swift`
- Modify: `DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/Composer/TelexComposer.swift`

**Interfaces:**
- Consumes: the Kotlin implementation as the verbatim reference (read the final `TelexComposer.kt` FIRST; the Swift file's header mandates 1:1).
- Produces: identical behaviour, proven by the mirrored test file + (Task 6) the shared vectors.

- [ ] **Step 1: Port `TelexOrderFreeTest.kt` case-for-case** into `TelexOrderFreeTests.swift` using the `type()` helper pattern from `TelexCorpusTests.swift` (composer loop + `.composing/.commit` capture). Same test names in camelCase, same expected strings, including `liftTone` unit cases (make `liftTone` `internal`, tests use `@testable import`).

- [ ] **Step 2: Run — new tests fail, existing green.** `swift test --filter TelexOrderFree` → FAIL; `swift test --filter Telex` → old suites PASS.

- [ ] **Step 3: Port the implementation** — mirror exactly what landed in Kotlin Tasks 2–4, adapted to the file's existing idioms (`Array(buf)` for indexing, `Character(x.lowercased())`, explicit `caseMap`-equivalent inline mapping):
  - `static let toneKeyOf: [Character: Character]` derived from the file's existing tone tables (find the Swift equivalents of `TONE_ROWS_LOWER`/`TONE_INDEX`/`UNTONE`/`UNMARK` — same names in camelCase — and derive, never re-list).
  - `static func liftTone(_ buffer: String) -> (String, Character?)`
  - `static func qualityRoot(_ c: Character) -> Character`
  - `'w'` branch + circumflex section wrapped with lift/re-apply; `applyHornOrBreve` + extracted `applyCircumflexOrCancel` root-matched; remote-đ rules in the `d` branch — all placed at the same positions as Kotlin.

- [ ] **Step 4: Run — all Swift Telex suites green.** `swift test` (full package) → 0 failures.

- [ ] **Step 5: Commit** — `git commit -am "feat(#207): order-free marking — Swift mirror"`

---

### Task 6: Case generator + curated parity vectors

**Files:**
- Create: `DraftRightMobile/tools/gen_telex_cases.py`
- Create: `parity/telex-order-vectors.json`
- Modify: `.github/workflows/mobile-parity-ci.yml` (paths list only)

**Interfaces:**
- Produces: `python3 DraftRightMobile/tools/gen_telex_cases.py --out <path>` writes `[{"keys": "...", "expected": "..."}, ...]` (deterministic order); `--check-wordlist` exits 1 if any syllable in `wordlist_vi_source.tsv` is not generated. `parity/telex-order-vectors.json` = committed curated cases, same schema plus `"name"`. Tasks 7 consumes both.

- [ ] **Step 1: Write the generator.** Single source of truth for the linguistic tables; ~200 lines. Core structure (complete tables in the file — these are external-spec constants, cite "Vietnamese orthography" in the header):

```python
#!/usr/bin/env python3
"""Exhaustive Telex order-free case generator (spec 2026-09-11).

Enumerates every valid Vietnamese syllable (onset x nucleus x coda x tone,
with compatibility rules) and emits keystroke sequences for each under many
typing orders. Expected output is always the syllable itself. Consumed by
TelexExhaustiveTest.kt and TelexExhaustiveTests.swift — the ONLY owner of
these tables (RULE #1).
"""
import argparse, itertools, json, sys, unicodedata
from pathlib import Path

ONSETS = ["", "b", "c", "ch", "d", "dd", "g", "gh", "gi", "h", "k", "kh", "l",
          "m", "n", "ng", "ngh", "nh", "ph", "qu", "r", "s", "t", "th", "tr",
          "v", "x"]  # "dd" renders đ; its mark key is the extra d

# nucleus: rendered form -> (base keys, mark keys in canonical adjacent order)
NUCLEI = {
    "a": ("a", []), "ă": ("a", ["w"]), "â": ("a", ["a"]),
    "e": ("e", []), "ê": ("e", ["e"]),
    "i": ("i", []), "y": ("y", []),
    "o": ("o", []), "ô": ("o", ["o"]), "ơ": ("o", ["w"]),
    "u": ("u", []), "ư": ("u", ["w"]),
    "ai": ("ai", []), "ao": ("ao", []), "au": ("au", []), "ay": ("ay", []),
    "âu": ("au", ["a"]), "ây": ("ay", ["a"]),
    "eo": ("eo", []), "êu": ("eu", ["e"]),
    "ia": ("ia", []), "iê": ("ie", ["e"]), "iu": ("iu", []),
    "oa": ("oa", []), "oă": ("oa", ["w"]), "oe": ("oe", []),
    "oi": ("oi", []), "ôi": ("oi", ["o"]), "ơi": ("oi", ["w"]),
    "ua": ("ua", []), "uâ": ("ua", ["a"]), "uô": ("uo", ["o"]),
    "ui": ("ui", []), "uy": ("uy", []), "uyê": ("uye", ["e"]),
    "ưa": ("ua", ["w"]), "ươ": ("uo", ["w"]), "ưu": ("uu", ["w"]),
    "uê": ("ue", ["e"]), "uơ": ("uo", ["w"]),      # NOTE: uơ vs ươ resolved by onset q
    "iêu": ("ieu", ["e"]), "yêu": ("yeu", ["e"]),
    "oai": ("oai", []), "oay": ("oay", []), "oeo": ("oeo", []), "oao": ("oao", []),
    "uôi": ("uoi", ["o"]), "ươi": ("uoi", ["w"]), "ươu": ("uou", ["w"]),
    "uyu": ("uyu", []), "uya": ("uya", []),
}
CODAS = ["", "c", "ch", "m", "n", "ng", "nh", "p", "t"]
STOP_CODAS = {"c", "ch", "p", "t"}          # only sắc/nặng tones
TONE_KEYS = ["", "s", "f", "r", "x", "j"]

# Tone placement: NFC-compose the tone onto the correct vowel of the rendered
# nucleus. Position table: index of tone vowel per rendered nucleus.
TONE_POS = { ... }   # complete in implementation: derived rules — special
                     # vowel wins (last special), open 2-vowel -> first,
                     # closed/3-vowel -> per spec; encode explicitly per nucleus.

def orderings(base, marks, tone, dd):
    """Yield keystroke sequences: canonical adjacent, plus all end-permutations
    of mark keys + tone key (+ remote d when dd onset), plus interleaved."""
    ...

def main():
    ...

if __name__ == "__main__":
    sys.exit(main())
```
Implementation completes `TONE_POS` (explicit per-nucleus index — 40 entries, matching the composer's modern-style placement: hòa/khỏe/thúy first-vowel for open oa/oe/uy; special vowel otherwise; verify each against `TelexCorpusTest` expectations), `orderings` (canonical; marks+tone end-appended in `itertools.permutations`; marks after coda with tone last; remote-d variants where onset is `dd`: the second `d` moves to the end-permutation set), validity filter (stop codas force `s/j`; `k/gh/ngh` onset spelling rules before `i/e/ê`; `q` requires `u`-glide nuclei), and `--check-wordlist` (split `wordlist_vi_source.tsv` words into syllables, NFC-lowercase, assert ⊆ generated).

- [ ] **Step 2: Run the self-check.**
Run: `python3 DraftRightMobile/tools/gen_telex_cases.py --check-wordlist DraftRightMobile/tools/wordlist_vi_source.tsv --out /tmp/telex_cases.json && python3 -c "import json;print(len(json.load(open('/tmp/telex_cases.json'))))"`
Expected: exit 0; case count in the 60k–120k range. Any wordlist syllable not generated = fix the tables NOW (this is the completeness proof).

- [ ] **Step 3: Write `parity/telex-order-vectors.json`** — 40 curated human-readable cases: the 6 device-verified study rows, all 6 `duoc*` permutations, the Task 1 list, plus 20 sampled from the generator across nucleus families. Schema: `[{"name": "...", "keys": "duocdwj", "expected": "được"}, ...]`.

- [ ] **Step 4: Wire CI paths.** In `.github/workflows/mobile-parity-ci.yml` add to the `paths:` list: `'DraftRightMobile/tools/gen_telex_cases.py'`, `'parity/telex-order-vectors.json'`, and the two composer file paths if not present. No new job — the vectors run inside the unit suites (next task), which `mobile-test-ci`/`mobile-swift-ci` already gate.

- [ ] **Step 5: Commit** — `git commit -am "feat(#207): exhaustive Telex case generator + curated order vectors"`

---

### Task 7: Exhaustive + vectors tests, both platforms — drive to zero divergence

**Files:**
- Create: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/composer/TelexOrderVectorsTest.kt`
- Create: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/composer/TelexExhaustiveTest.kt`
- Create: `DraftRightMobile/ios/DraftRightKeyboardCore/Tests/DraftRightKeyboardCoreTests/TelexOrderVectorsTests.swift`
- Create: `DraftRightMobile/ios/DraftRightKeyboardCore/Tests/DraftRightKeyboardCoreTests/TelexExhaustiveTests.swift`
- Modify (likely): `TelexComposer.kt` + `TelexComposer.swift` for divergences the matrix exposes.

**Interfaces:**
- Consumes: Task 6 generator + vectors file. Path resolution: climb parent dirs from the test file to the repo root exactly the way `AutoCorrectorVectorsTest.kt` / `AutoCorrectorVectorsTests.swift` locate `parity/` — copy that mechanism.
- Produces: the permanent proof suites.

- [ ] **Step 1: Vectors tests (both platforms).** Same shape as the AutoCorrectorVectors pair: load `parity/telex-order-vectors.json`, run each through the `type()` helper, assert. Run both → PASS (they cover what Tasks 1–5 built).

- [ ] **Step 2: Exhaustive tests.** Each generates the matrix by invoking the generator (`ProcessBuilder("python3", scriptPath, "--out", tmp)` in Kotlin; `Process()` in Swift `setUp`), loads the JSON, loops, and collects ALL failures into one report (`fail("N divergences:\n" + first 50)`) rather than stopping at the first. Both CI runners (Mac) have `python3`; skip with a clear message if it's absent locally: `assumeTrue(python3Exists)`.

- [ ] **Step 3: Run Kotlin exhaustive.** Expect a nonzero divergence list on first run. For each divergence class: decide against the spec rule (and, where genuinely ambiguous, against Samsung on the A52 — document any Samsung-divergence in a spec appendix commit), fix in `TelexComposer.kt`, keep `TelexOrderFreeTest` + corpus suites green, re-run. Iterate to ZERO.

- [ ] **Step 4: Port every fix to Swift** (verbatim, same order), run Swift exhaustive to ZERO.

- [ ] **Step 5: Full test sweep both platforms** — every keyboard suite green: Kotlin `--tests 'com.draftright.keyboard.*'`; Swift full `swift test` (279+ tests).

- [ ] **Step 6: Commit** — `git commit -am "test(#207): exhaustive syllable-x-ordering matrix green on both platforms"` (plus intermediate fix commits per divergence class: `fix(#207): <class>`).

---

### Task 8: Device sanity, cleanup, review, PR

**Files:**
- Modify: `docs/superpowers/specs/2026-09-11-telex-order-free-marking-design.md` (append results + any Samsung-divergence appendix; correct the study table if Task 1's guard rail contradicted a row — the `duocdwj` DraftRight baseline is suspect: the device test may have run with the English pack active).

- [ ] **Step 1: Android device battery.** Build + install debug APK, enable VI (recipe in memory `project_session_20260903_vi_autocorrect`: `run-as` writes `FlutterSharedPreferences.xml`), **verify the spacebar shows "Tiếng Việt" via screenshot BEFORE typing** (the lesson from the invalid baseline), then tap-type: `tuongrw`, `nguoifw`, `duocdwj`, `Duocdwj`, `khongo`, `hoaf` — screenshot each, expect the Samsung column of the spec table. Uninstall debug app + restore release IME afterwards (standing phone policy).
- [ ] **Step 2: iOS.** Core proof = Swift suites (they run in `mobile-swift-ci` on the Mac). Runtime verify rides the next Xcode Cloud TestFlight build (sync mirror → `POST ciBuildRuns` — recipe in memory `feedback_xcode_cloud_default_workflow`); note in the PR that keyboard-extension runtime verify on iPhone is owner-manual.
- [ ] **Step 3: `/cleanup-garbage`** over the branch diff (checklist step 6): no dead code, no orphaned helpers (e.g. if root-matching made a matcher branch unreachable, DELETE it), no stale comments describing pre-order-free behaviour.
- [ ] **Step 4: `/epiphanydev:full-review`** over the branch diff (checklist step 7); fix findings.
- [ ] **Step 5: Test-cases sheet.** Add `KBD-VI-ORDERFREE` rows (VIOF-1..n: the study table + permutations) to `docs/test-cases.xlsx` — same openpyxl append pattern as VIAC-6..13.
- [ ] **Step 6: PR** to develop: study table, mechanics, matrix size + zero-divergence statement, device screenshots, How-to-Verify steps. `gh pr create --base develop ...`; merge after checks; label #207 flow per GitFlow (this rides under #207 or a new issue — create issue "Telex order-free marking (Samsung parity)" assigned tannpv, `status: developed` on merge).

---

## Self-Review (done at write time)

- **Spec coverage:** mechanics 1–3 → Tasks 2–4 (+5 mirror); harness → 6–7; device sanity + docs → 8; RULE #1 constraints → Global Constraints + derived-map implementations. Spec's "coda reach-back" mechanic: already present for horn singles (`findLastVowelThroughConsonants` in `applyHornOrBreve`) and pairs (`findLastVowelCluster` skips codas) — the exhaustive matrix (Task 7) proves it rather than a dedicated task; any hole it finds is fixed in Task 7 Step 3. ✓
- **Placeholders:** Task 6's `TONE_POS`/`orderings` bodies are deliberately specified-by-rule with explicit completion instructions in-step (the full 40-entry table belongs in the file, not the plan); everything else is complete code. ✓
- **Type consistency:** `liftTone(String): Pair<String, Char?>` used identically in Tasks 2/3/5; `qualityRoot(Char): Char` in 3/4/5; generator schema `{keys, expected}` in 6/7. ✓
