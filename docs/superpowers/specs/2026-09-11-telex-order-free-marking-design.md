# Telex Order-Free Marking — Samsung Parity (Design)

**Date:** 2026-09-11 · **Owner:** Tan · **Refs:** #207 (keyboard parity), `docs/keyboard-samsung-study.md`
**Platforms:** Android (`composer/TelexComposer.kt`) + iOS (`Composer/TelexComposer.swift`), mirrored per the keyboard seam rules in `DraftRightMobile/CLAUDE.md`.

## Problem

Samsung's Vietnamese keyboard lets you type a word's base letters first, then
append the marks — tone, horn/breve, circumflex, đ — **at the end of the word,
in any order**, and it places every mark correctly. The owner types this way
and DraftRight "feels harder" because it only partially supports it.

Empirical study (2026-09-11, Galaxy A52, identical tap sequences on both
keyboards):

| Typed | Samsung | DraftRight today |
|---|---|---|
| `hoa` + `f` | Hòa | hòa ✅ |
| `tuong` + `w` + `r` | Tưởng | tưởng ✅ |
| `khong` + `o` | Không | không ✅ |
| `tuong` + `r` + `w` (tone first) | Tưởng | `tuỏngw` ❌ — `w` falls out literal |
| `nguoi` + `f` + `w` | Người | `nguòiw` ❌ |
| `duoc` + `d` + `w` + `j` | **Được** | `duocdwj` ❌ — everything literal |

## The rule being implemented

**Word-scope, order-free marking.** Within the current composing word, the
mark keys commute: any of `{tone key, w, doubled vowel, trailing d}` may
arrive in any order, at any point after its target letters exist, and the
rendered word is always the same as if they had been typed in canonical
adjacent Telex order.

Three concrete mechanics fall out of today's failures:

1. **Tone-transparent quality modifiers.** Applying horn/breve (`w`),
   circumflex (`aa`/`oo`/`ee`), or breve to a cluster that already carries a
   tone must work: strip the tone off the cluster, apply the quality change
   with the existing modifier logic, then re-place the tone with the existing
   `applyTone` placement rules (which already prefer quality-marked vowels —
   this is what turns `tuỏng`+`w` into `tưởng`, not `tửong`).
2. **Modifier reach-back through coda consonants.** The tone engine already
   reaches the vowel cluster through trailing consonants
   (`findLastVowelThroughConsonants`); the quality modifiers must use the same
   reach-back so `duoc`+`w` → `dươc`-cluster targeting works. Reach-back
   limits (`canReachBack`) stay as they are — only the modifier appliers gain
   the ability the tone applier already has.
3. **Remote đ.** A trailing `d` typed on a word whose initial letter is
   `d`/`D` converts that initial to `đ`/`Đ` (case preserved). This is
   unambiguous: no Vietnamese syllable ends in `d`, so a trailing `d` on a
   d-initial word can only mean đ. On a word not starting with `d`, a typed
   `d` stays what it is today (start of the next letters). Adjacent `dd`
   keeps working unchanged.

Existing behaviours that must NOT regress: everything in the current Telex
suite — base-first (#152), iê/uô/yê promotion, uo→ươ, tone
cancel-by-retype, double-modifier cancel (`ww`), offglide-aware targeting,
late tone, late circumflex (`khongo`→không).

## Approach (chosen: A — surgical, reuse both engines)

Keep the incremental `tryCombine(buffer, incoming)` architecture and the
rendered-string buffer. Add one shared helper per platform:

- `applyModifierToneTransparent(buffer, apply)`: find the target cluster; if
  it carries a tone, record + strip it; run `apply` (the existing horn/breve
  or circumflex logic); re-apply the recorded tone via `applyTone`. All
  existing modifier entry points route through it — a single chokepoint, no
  per-modifier special cases.
- Extend the modifier target search to use the same
  through-coda reach-back as tones.
- `tryRemoteD(buffer, incoming)`: incoming `d` when `buffer` matches
  `^[dD][^dD]*$` and contains a vowel → replace initial with đ/Đ. Tried
  before the literal-append fallback, after the existing adjacent-`dd` rule.

Rejected: full state-model rewrite (base+marks+tone re-rendered per key) —
commutative by construction but rewrites a mature 500-line composer with a
year of verified behaviour for the same user-visible result; and a `w`-only
special case — leaves the other modifiers inconsistent (RULE #1 drift).

## Exhaustive proof (the harness)

Tap-testing samples; the composer is pure logic, so we prove **all** cases:

1. **Syllable enumerator** (`DraftRightMobile/tools/gen_telex_cases.py`, the
   single source of truth): generates every valid Vietnamese syllable from
   onset × nucleus × coda × tone tables (with tone/coda compatibility rules),
   cross-checked against `tools/wordlist_vi_source.tsv` — every syllable
   appearing in the shipped 8.5k word list must be produced by the tables
   (build fails otherwise).
2. **Ordering generator:** for each syllable, emit its keystroke sequences:
   canonical adjacent Telex; all marks appended at word end in **every
   permutation** of `{remote-d, quality marks, tone}`; marks interleaved
   mid-word; doubled-vowel forms adjacent and late. Expected output = the
   syllable itself, for every sequence.
3. **Scale:** ~7–8k syllables × up to ~12 orderings ≈ 80–100k cases. The
   generator writes one deterministic case file consumed by BOTH test suites
   (Kotlin JUnit + Swift XCTest) — generated at build/test time, not
   committed (too large); a curated ~40-case human-readable subset (including
   `duocdwj`→Được, `tuongrw`→tưởng, `nguoifw`→người) is committed to
   `parity/telex-order-vectors.json` and wired into `mobile-parity-ci.yml`
   like the existing guards.
4. **Definition of done:** zero divergences across the full generated matrix
   on both platforms, plus the curated vectors green in CI.
5. **Device sanity pass:** re-run the study battery on the A52 (release
   keyboard) and on iOS via the Xcode Cloud TestFlight build — unit truth
   confirmed in the real IME pipeline before merge (per the keyboard
   on-device-verify rule).

## RULE #1 pass (answered before any code — checklist step 0)

- **What would this restate that something already owns?** Vowel/tone tables,
  cluster targeting, tone placement, reach-back — ALL already owned by
  `TelexComposer`'s existing helpers (`applyTone`, `pickToneVowelIndex`,
  `findLastVowelThroughConsonants`, `canReachBack`, `UNTONE`). The new code
  calls them; it must not re-derive any vowel/tone knowledge. The syllable
  enumerator's linguistic tables are NEW data with one owner
  (`gen_telex_cases.py`) — nothing else may re-list onsets/nuclei/codas, and
  the wordlist cross-check is the machine guard proving the tables complete.
- **What does it reuse?** The two existing mark engines (quality-modifier
  apply + tone apply) wrapped by ONE chokepoint
  (`applyModifierToneTransparent`) — no second Levenshtein-style duplicate,
  no per-modifier copies. `UNTONE`/strip helpers reused for tone lifting.
- **Would a third case fit?** A future mark key (e.g. VNI digits, or a new
  quality mark) enters through the same chokepoint + the same ordering
  generator — data/config extension, not copy-paste. The harness's ordering
  set is generated from the mark list, so a new mark automatically multiplies
  the matrix.
- **Any literal that carries meaning?** The remote-đ trigger pattern and the
  mark-key sets stay where they already live (the composer's existing
  constants); the generator reads its mark list from ONE table. No new magic
  strings at call sites.
- **Cross-cutting + the machine that proves it:** Kotlin and Swift cannot
  share source (the standing keyboard constraint) — the parity chokepoints
  are the generated case matrix run by BOTH suites plus the curated
  `parity/telex-order-vectors.json` guard in `mobile-parity-ci.yml`. A
  divergence in either mirror fails CI; no reviewer-memory enforcement.

## Appendix — implementation results (2026-09-11)

**Shipped shape.** The chokepoint is `liftTone(buffer) -> (bare, toneKey)`:
the `w` branch and the circumflex branch both lift the tone, run the existing
quality logic on the bare buffer, and re-place the tone through `applyTone`,
so placement is always recomputed. Quality matching is by ROOT (via the
existing `UNMARK` map) so marks replace each other (`ô`+w → `ơ`). Remote đ
lives in the `d` branch. Same code, same order, in both ports.

**Matrix.** 20.5k syllables × their orderings = **43,791 cases**, zero
divergences on both platforms (`TelexExhaustiveTest` / `TelexExhaustiveTests`,
which run the generator themselves). `--check-wordlist` passes: every wordlist
row at or above the documented frequency floor is generated.

**Divergence classes the matrix found** (all fixed in both ports):

1. *Horn hit the offglide.* `oi`+w left the w literal, `uu`+w gave `uư`. The
   single horn/breve now resolves the nucleus of the trailing cluster,
   skipping offglides and the qu/gi onset glide (`hornTargetIndex`).
2. *Deep circumflex cancelled instead of applying.* After tone auto-promotion
   (`iecs` → iếc) the `e` key was read as cancel-by-retype and undid a mark
   the user never typed. Cancel is now adjacent-only; deep matches apply
   idempotently. Every pre-existing cancel test types its mark last, so this
   narrowing regresses nothing.
3. *qu/gi glide skip lived only in `applyTone`*, so the horn path produced
   `qừo` for `quow`. Extracted to a shared `skipOnsetGlide`.

**Deliberate non-parity notes.**

- **`uơ` (huơ, thuở) is unreachable.** Its keystrokes are `uo`+w, which Telex
  already spends on `ươ`. The generator documents and skips it; after a `qu`
  onset the nucleus is a plain `ơ` (quơ, quở), which is generated and passes.
- **Tone placement stays modern.** The corpus still carries pre-1980s
  placements (hoà, khoẻ, thuỷ); we produce hòa/khỏe/thủy, matching Samsung.
  `--check-wordlist` accepts the old spelling as covered when we generate the
  same letters with the same tone on the modern vowel.

## Out of scope

- Word prediction/suggestion changes (bar already shipped in #207).
- Swipe-to-type, auto-spacing, double-space period — separate initiatives
  (the "typing conveniences" bucket from brainstorming, specced separately).
- VNI/VIQR input styles; multi-syllable (cross-space) marking.
- Changing tone placement style (stays new-style, matching Samsung).

## Risks

- **Ordering ambiguity discovered by the harness** (a sequence where Samsung
  and strict orthography disagree): resolve by matching Samsung, note the
  case in the spec appendix during implementation.
- **Remote đ surprise** for users typing foreign words with d…d in a VI
  field: accepted — Samsung behaves the same and the VI pack is opt-in.
- **Performance:** tryCombine stays O(word length); the harness runs offline.
  No runtime cost concerns.
