# Plan — Vietnamese Telex Samsung-parity (keyboard "feel" + suggestions)

Issue: #207. Platform: Android IME (`DraftRightMobile/android/.../keyboard`).
Owner feedback: Samsung VI-Telex is convenient "especially for strokes and
character manipulation"; DraftRight "feels harder / different". Companion study:
`docs/keyboard-samsung-study.md`.

## Root-cause summary (from reading the code, not guessing)

DraftRight's Telex **mechanics are already correct** (`TelexComposer`, 502 lines:
iê/uô/yê + uôi/iêu/uyê promotion, uo→ươ cluster, dd→đ, tone cancel-by-retype,
#152 base-first). The "harder feel" is **three missing convenience layers**, and
two of the three are *already-built infrastructure that is simply underfed*:

| Gap | Current state (file:line) | Why it feels worse than Samsung |
|---|---|---|
| **A. No key feedback** | `QwertyKeyboardView.kt` — zero `performHapticFeedback` / `playSoundEffect` anywhere (grep-confirmed) | Every Samsung keypress buzzes + clicks; DraftRight keys feel "dead" |
| **B. Weak suggestions** | `VietnameseLanguagePack.candidateEngine()` IS wired → `TrigramCandidateEngine` over `R.raw.wordlist_vi`, fed every keystroke by `DraftRightIME.refreshCandidates()` (line 617). But `wordlist_vi.tsv` is a **200-word bootstrap stub** (`# Replace with the downloadable 50k pack`) with **no bigrams** | Samsung predicts the full diacritised word from a large dictionary; DraftRight only completes ~200 words |
| **C. No next-word prediction** | `DraftRightIME.kt:631` hardcodes `previousTokens = emptyList()` (`// n-gram context wired in Task 11 step 7`) → `TrigramCandidateEngine.nextWord()` + bigram boost are dead code paths | Samsung offers the likely next word after space; DraftRight offers nothing until you type |

The candidate **bar, engine seam, loader (bigram-capable), and pack resolver all
already exist** (`ime/CandidateEngine.kt`, `TrigramCandidateEngine.kt`,
`WordListLoader.kt` parses 3-col bigram lines, `WordListPackResolver`). This is a
**feed-the-existing-pipeline** job, not a new subsystem.

## RULE #1 pass (checklist step 0 — before any code)

- **Restates?** Nothing. Reuse the existing `CandidateEngine`/`TrigramCandidateEngine`/
  `LanguageWordList`/`WordListLoader` seam as-is. Key feedback reuses Android's own
  `View.performHapticFeedback` + `AudioManager.playSoundEffect` (which already gate
  on the OS haptic/touch-sound settings — no custom settings reader).
- **Reuse?** Feedback is **cross-cutting** across every key → one chokepoint, not a
  copy per key. `previousTokens` tokenisation is one helper reused by every
  Latin pack, not VI-only.
- **Third case?** Feedback must not hardcode VI — it fires for all packs. Sound map
  is keyed by key *kind* (char/space/delete/enter) via `SpecialKeys`, extendable.
  Dictionary path already generalises to fr/es/de/it/pt/en (same loader).
- **Literal that carries meaning?** Key-kind → `AudioManager.FX_KEYPRESS_*` mapping
  goes in one table, not inline. No magic vibration ms (use the platform constant
  `HapticFeedbackConstants.KEYBOARD_TAP`).
- **Cross-cutting?** Yes — feedback. Chokepoint = one `KeyFeedback` collaborator
  invoked from the single `ACTION_DOWN` branch + the backspace-repeat tick. A test
  asserts every actuation path routes through it (no key bypasses feedback).

## Phase 1 — Key feedback (haptic + sound)  ← smallest, most-felt, ship first

**Goal:** every keypress produces OS-gated haptic + a key-appropriate click,
matching Samsung. Universal (all languages), not VI-specific.

**Design — one chokepoint, no hardcoding:**
- New `KeyFeedback.kt` (small, testable): `fun onKey(kind: KeyKind, view: View)`.
  - Haptic: `view.performHapticFeedback(HapticFeedbackConstants.KEYBOARD_TAP,
    HapticFeedbackConstants.FLAG_IGNORE_GLOBAL_SETTING.inv-not)` — use the plain
    2-arg form so the **system haptic setting is respected** (do NOT force-ignore).
  - Sound: `AudioManager.playSoundEffect(fx)` — the OS mutes it automatically when
    "touch sounds" is off, so no manual gate.
  - `fx` from a `KeyKind → Int` map: char→`FX_KEYPRESS_STANDARD`,
    space→`FX_KEYPRESS_SPACEBAR`, delete→`FX_KEYPRESS_DELETE`,
    enter→`FX_KEYPRESS_RETURN`. Table lives once in `KeyFeedback`.
- `KeyKind` derived from `code` via existing `SpecialKeys` predicates (reuse
  `isSpace`, `BACKSPACE`, `ENTER`, else char).
- Call sites in `QwertyKeyboardView.kt`:
  - `ACTION_DOWN` branch (line ~254) — fire once per initial touch (Samsung fires
    feedback on press-down, not release).
  - `backspaceRunnable` tick (line ~363) — feedback per auto-repeat delete.
  - `AccentPopupView` pick (later; note as follow-up, keep Phase 1 to main keys).

**Files:** `+KeyFeedback.kt`, edit `QwertyKeyboardView.kt` (inject + 2 call sites),
maybe `SpecialKeys.kt` (a `kindOf(code)` helper if cleaner).

**Tests (first):** `KeyFeedbackTest` — key-kind→FX mapping is total + correct;
a fake `View`/`AudioManager` seam records that DOWN + backspace-repeat both invoke
feedback (the "no key bypasses the chokepoint" guard). Run scoped:
`./gradlew :app:testDebugUnitTest --tests '*KeyFeedback*'`.

**Effort:** small. **Impact:** highest immediate "feel" win.

## Phase 2 — Vietnamese suggestion quality (dictionary + next-word)

**Goal:** the candidate bar offers many real, correctly-diacritised VI words and a
likely next word — the "convenient strokes/manipulation" the owner values.

**2a. Bigger bundled VI dictionary.**
- Replace the 200-word `wordlist_vi.tsv` with a substantially larger frequency
  list (target a few thousand entries bundled; the 50k mmap pack stays the
  downloadable path via `WordListPackResolver` — unchanged).
- Add bigram lines (`prev<TAB>next<TAB>count`) — `WordListLoader.parseBigrams`
  already handles them; source them into a sibling resource
  (`R.raw.wordlist_vi_bigrams`) and pass `bigramsResId` through the resolver.
- **Source + licence:** hand-curate/derive from a CC BY-SA VI frequency corpus,
  cite the source in the tsv header (external-spec exemption — same as the current
  file's header). No scraping of licence-incompatible corpora.
- **RULE #1:** this generalises to the other Latin packs later via the same
  resolver signature — extend the signature once, don't fork per language.

**2b. Wire `previousTokens` (kills the dead next-word path).**
- In `DraftRightIME.refreshCandidates()`, replace the hardcoded `emptyList()` with
  the last N committed tokens read from the field:
  `currentInputConnection.getTextBeforeCursor(K, 0)` → strip the live composing
  text → tokenise on whitespace → take last `N` (start N=2 for trigram context).
- Put tokenisation in a tiny pure helper (`PreviousTokens.fromTextBeforeCursor`)
  so it is unit-testable and **reused by every pack**, not inlined in the IME.
- Guard: never include the active composing buffer as a "previous" token
  (off-by-one that would double-suggest the current word).

**Files:** replace `res/raw/wordlist_vi.tsv` (+ `wordlist_vi_bigrams.tsv`), edit
`VietnameseLanguagePack.kt` (pass bigram res), `WordListPackResolver` (thread the
optional bigram id — check its current signature first), `DraftRightIME.kt:631`,
`+PreviousTokens.kt`.

**Tests (first):** `PreviousTokensTest` (tokenises "xin chào |" → ["xin","chào"];
excludes composing buffer; handles empty/leading space). `TrigramCandidateEngine`
already has coverage — add VI fixtures asserting a known bigram surfaces the
expected successor.

**Effort:** medium (dictionary sourcing is the bulk). **Impact:** high.

## Phase 3 — Refinements + open verify (after 1 & 2 land)

- **Telex-from-raw-keystrokes completion:** today `completions()` prefix-matches on
  the *diacritised* composing buffer, so "vie" doesn't yet match "viết" mid-syllable.
  Evaluate matching on the raw Telex keystrokes too. Scope only after 2 is on-device
  — may be unnecessary if the diacritised completion already feels good.
- **⚠️ `dậý` acute-leak verify (from the study):** committing `dayaj` rendered
  `dậý` (acute on final `y`) in the Playground field — expected `dậy`. Reproduce on
  device, and if real, root-cause in `TelexComposer` (systematic-debugging, not a
  guess-patch) as its own bug/issue. Do this BEFORE claiming Telex "correct".
- **AccentPopupView feedback** — extend the Phase-1 `KeyFeedback` to long-press
  accent picks.

## Sequencing & delivery (per Development Task Checklist)

1. Test cases → `docs/test-cases.xlsx` before code (feedback fires; suggestion
   surfaces expected word; next-word after space).
2. Branch `feature/207-vi-telex-parity-20260825` from develop.
3. Phase 1 → Phase 2 → (Phase 3 evaluated). Each phase: implement → tests →
   `flutter analyze` + scoped gradle unit tests → **on-device manual check**
   (build `.debug` APK, both keyboards side-by-side) → `/cleanup-garbage` on the
   diff → `/epiphanydev:full-review` → `--no-ff` merge to develop.
4. **No backend/server involved** — this is client-only; the only "deploy" is the
   debug APK to the device for QA, then the normal store pipeline when the owner
   decides. No prod-deploy auth needed for Phase 1–3 development.
5. Label #207 `status: developed` after develop merge; owner verifies on device;
   never auto-close.

## What this plan deliberately does NOT do

- No swipe-to-type, themes/resize, clipboard, emoji, handwriting (study §4 lists
  them; out of scope for VI-feel parity — separate issues if wanted).
- No JP/ZH parity yet (#207 phases 2–3; thin composers, revisit after VI).
- No new prediction *model* — the static trigram seam stays; a neural LM can drop
  into the same `CandidateEngine` interface later with zero UI change.
