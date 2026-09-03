# Design — Vietnamese auto-correct-on-space (#207 gap #3)

**Date:** 2026-09-03
**Status:** design approved by owner; precedes an implementation plan.
**Issue:** #207 (VI/JP/ZH Samsung parity), gap #3 — typo tolerance / auto-correct.

The iOS + Android keyboards should behave like Samsung for Vietnamese: when the
user finishes a word (space), a small typo is **auto-corrected** to the intended
Vietnamese word. Today there is no auto-correct; fuzzy matches only ever *top up
the candidate bar*, and the bundled dictionary is a ~200-word placeholder that
cannot support correction.

## Why this needs a real dictionary first

`fuzzyMatches` returns results only from a **pack-backed** word list; the base
impl returns empty, and the bundled `wordlist_vi.tsv` is explicitly a
"~200 most common words … replace with the downloadable 50k pack once shipped"
placeholder (iOS is worse: 102 words hardcoded in a Swift array). Auto-correct
over 200 words is **harmful** — it can only recognise the 200 commonest words, so
it either fails to correct or replaces a valid-but-uncommon word toward a common
one. **The dictionary is the true first deliverable.**

## Scope

**In:** a real VI frequency dictionary shipped identically on both clients; the
auto-correct decision logic (mirrored Kotlin↔Swift); backspace-to-undo; an
opt-in `LanguagePack` trait; parity guards + golden-vector tests.

**Out:** JP/ZH auto-correct (trait stays off — future work); a downloadable-pack
delivery pipeline (we bundle the ~8.5k core, which is small — ~79 KB); on-device
suggestion-bar redesign (the candidate strip already exists and benefits for free).

## The dictionary

- **Source:** `hermitdave/FrequencyWords` (OpenSubtitles 2018 VI), **CC BY-SA 4.0**
  — attribution recorded in the file header and this spec.
- **Cleaning (deterministic, reproducible via a committed `tools/` script):** keep
  only valid Vietnamese single syllables — VI alphabet only (no f/j/w/z), exactly
  one contiguous vowel run, a valid coda (open, or one of c/ch/m/n/ng/nh/p/t),
  1–7 chars, frequency ≥ 3. This drops English/foreign junk *structurally*
  (e.g. "sound"/"dark"/"code" fail the coda/vowel-run rules) rather than by
  blocklist. Result: **~8,534 syllables** — effectively the full attested VI
  syllable inventory.
- **One source of truth:** the tsv lives once (Android `res/raw/wordlist_vi.tsv`),
  and iOS bundles the **same file** as a resource (replacing the 102-word Swift
  array; loaded via the existing `WordListLoader`). A `check-vi-wordlist-parity.py`
  guard in `mobile-parity-ci.yml` asserts the two are byte-identical so the
  dictionary cannot drift between clients (RULE #1).
- **Format:** `word<TAB>frequency`, `#` comment lines — unchanged from the current
  loader, so no loader format change.

## The correction decision (one source, mirrored, parity-guarded)

On **space** (word boundary), consider the just-typed token `w`:

1. If `w` is empty / contains no letters → no-op.
2. If `w` **is already in the dictionary** → **never correct** (valid word).
3. Else find dictionary candidates within edit distance ≤ `MaxAutoCorrectEdits`
   (**= 1**, confirmed — conservative, fewer false positives), reusing the
   existing `LanguageWordList.fuzzyMatches` engine (NO new edit-distance impl).
4. Pick the highest-frequency candidate; correct only if its frequency clears
   `MinConfidenceFreq` AND beats the runner-up by `MinConfidenceMargin` (so
   ambiguous typos are left alone). All three are **named constants**, one
   definition per platform, asserted equal by a parity guard.
5. Replace `w` with the chosen word in the text buffer, and **record the
   (original, corrected) pair** for undo.

The decision is pure (string in → optional replacement out), so it is unit-tested
with **golden vectors shared Kotlin↔Swift** (typo→fix, valid-word→untouched,
ambiguous→untouched, below-threshold→untouched).

## Undo (mandatory)

Pressing **backspace immediately after an auto-correction reverts** the buffer to
exactly what the user typed (Samsung/iOS behaviour) and clears the pending-undo
state. Any other key clears the pending-undo state. This lives at **one chokepoint**
in the IME's key handler (`DraftRightIME` / `KeyboardViewController`), not scattered
per-key. Without it, a wrong correction traps the user — it is not optional.

## Extendability (RULE #1 — the next language)

Auto-correct is gated by a new **`autoCorrectEnabled: Bool` trait on `LanguagePack`**
(default **false**; `VietnameseLanguagePack` returns true). JP/ZH/EN are untouched.
A future language opts in by flipping the trait + shipping its dictionary — no
change to the correction engine.

## Data flow

```
key: SPACE ─▶ IME word-boundary hook
   └─ AutoCorrector.correct(token, wordList)   ← pure, mirrored, golden-tested
        ├─ token in dict?  → return none (valid)
        ├─ fuzzyMatches(token, MaxAutoCorrectEdits) → ranked candidates
        └─ top clears MinConfidenceFreq + MinConfidenceMargin?
             ├─ yes → Replacement(original, corrected)
             └─ no  → none
   └─ if Replacement: rewrite buffer + arm undo(original)
key: BACKSPACE while undo armed ─▶ restore original, disarm
key: anything else ─▶ disarm undo
```

## RULE #1 — the machines that keep it honest

| Concern | Single source of truth | Machine |
|---|---|---|
| The dictionary | one `wordlist_vi.tsv` (Android res; iOS bundles the same bytes) | `check-vi-wordlist-parity.py` (byte-identical) |
| Edit-distance / fuzzy | existing `LanguageWordList.fuzzyMatches` | reused, not reimplemented |
| Threshold constants | `MaxAutoCorrectEdits` / `MinConfidenceFreq` / `MinConfidenceMargin` | `check-autocorrect-consts-parity.py` |
| Correction decision (Kotlin vs Swift) | one algorithm, two mirrors | shared golden-vector tests (`autocorrect-vectors.json`) |
| "Is it on for this language?" | `LanguagePack.autoCorrectEnabled` | never branch on the language name |

## Testing

- `AutoCorrector` unit tests (both platforms): typo→fix, valid word→untouched,
  ambiguous→untouched, below-threshold→untouched, empty/punctuation→no-op.
- Undo: correction then backspace restores the typed text; correction then a
  letter disarms undo.
- Golden-vector parity: the same `autocorrect-vectors.json` asserted on both.
- Dictionary + consts parity guards wired into `mobile-parity-ci.yml`.
- **On-device (owner, TestFlight):** type a few known typos + confirm the
  correction and the backspace-undo feel right — the one step that isn't automatable.

## Out of scope / follow-ups

- Downloadable larger pack (the bundled ~8.5k core is enough to ship; the pack
  channel can extend it later).
- Per-user learned corrections / personal dictionary.
- JP/ZH auto-correct (trait off).
- Multi-syllable / compound correction (VI is written syllable-by-syllable; the
  space-delimited token model matches how the keyboard already commits words).
