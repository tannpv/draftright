# Android Multi-Language Keyboard (Tier β) — Design

**Date:** 2026-05-17
**Author:** Tan Nguyen
**Status:** Approved for planning
**Target client:** DraftRight Mobile — Android only (iOS keyboard extension out of scope)
**Strategic note:** This pivots `project_android_keyboard_strategy.md` from "English-only IME, share-intent primary" to "multi-language IME, type-natively primary." Memory note will be rewritten when this ships.

---

## 1. Problem

The DraftRight Android IME today supports English (QWERTY) only. Memory `feedback_android_share_intent.md` flags that the keyboard "can't handle Vietnamese/CJK input," forcing users to switch to Gboard for non-English typing. That defeats the keyboard's purpose — the user has to leave DraftRight every time they want to type their native language, breaking the "select-and-rewrite" flow.

Tier β makes DraftRight a real daily keyboard for six languages. Vietnamese is the highest-priority addition (home market + biggest user base) and the hardest (Telex composition requires a state machine). The five Latin languages with diacritics are mostly layout swaps and long-press accent tables.

## 2. Goals

- **Type natively** in Vietnamese, French, Spanish, German, Italian, Portuguese (and English baseline) without switching to Gboard.
- **Cycle languages** with one globe-key tap from inside the DraftRight keyboard.
- Vietnamese **Telex composition** (vowel marks + tones) with a clean state machine, 25+ unit tests, and backspace un-composes one step.
- **Long-press accent picker** above the held key (iOS / Gboard convention) for diacritic languages.
- **Native-name space-bar label** ("Tiếng Việt", "Español", "Français", …) confirms active language without looking at the language strip.
- **Zero regression** for English-only users — every keystroke commits identically to today.
- **No backend changes** — the rewrite API already accepts UTF-8 from any language.

## 3. Non-goals

- VNI input method (Telex only in Phase 1; VNI deferred to Phase 2 if demand surfaces).
- Autocorrect / silent typo fixes.
- Predictive-text / suggestion bar.
- Per-app language memory ("last used Vietnamese in WhatsApp").
- Voice input, emoji search, cloud-sync of preferences.
- CJK (Japanese, Chinese, Korean) — needs candidate-picker architecture, deferred to a future phase.
- iOS keyboard extension — Apple's "Full Access" prompt depresses adoption; iOS users use the share extension instead.

## 4. Six locked decisions (from brainstorm)

1. **Priority order:** VI → FR → ES → DE → IT → PT (Vietnamese first, both highest-value and highest-risk).
2. **Globe key:** tap cycles DraftRight languages; long-press opens the system IME picker.
3. **Vietnamese input method:** Telex only (Phase 1); VNI deferred.
4. **Autocorrect / predictive text:** NO in Phase 1.
5. **Long-press accent picker:** popup ABOVE the held key (iOS/Gboard convention).
6. **Space-bar indicator:** native display name (Tiếng Việt, Français, …), not flag, not ISO code.

## 5. User flows

### 5.1 English baseline (no change)

Press `a` → `commitText("a", 1)` → field shows `a`. Identical to today.

### 5.2 Vietnamese — typing `việt`

| Key | Composer state | InputConnection call | Field |
|---|---|---|---|
| `v` | `{ word: "v" }` | `setComposingText("v", 1)` | `v` (highlighted) |
| `i` | `{ word: "vi" }` | `setComposingText("vi", 1)` | `vi` |
| `e` | `{ word: "vie", vowel: "ie" }` | `setComposingText("vie", 1)` | `vie` |
| `t` | `{ word: "viet" }` | `setComposingText("viet", 1)` | `viet` |
| `j` | dot-below tone applied to last vowel cluster | `commitText("việt", 1)` + reset | `việt` |

### 5.3 Language cycle via globe

User taps 🌐. Controller commits any pending composing text, calls `composer?.reset()`, advances to next enabled language, refreshes the language strip + layout + space-bar label. Input-field caret stays where it was.

### 5.4 Long-press accent picker (Spanish `á`)

User long-presses `a`. After 300 ms a popup appears above the key with `á à ä â ã`. User drags finger to `á`, releases. `commitText("á", 1)`.

### 5.5 No entities, no rewrite (regression guard)

Any sequence that produces no composer activity routes straight to `commitText(...)`. Backend rewrite flow unchanged. Tone toolbar unchanged.

## 6. Architecture

```
DraftRightIME (existing) ──► KeyboardController (new) ──► InputConnection (Android)
                                       │
                                       ├── LanguagePack (interface)
                                       │     ├── EnglishLanguagePack
                                       │     ├── VietnameseLanguagePack
                                       │     ├── FrenchLanguagePack
                                       │     ├── SpanishLanguagePack
                                       │     ├── GermanLanguagePack
                                       │     ├── ItalianLanguagePack
                                       │     └── PortugueseLanguagePack
                                       │
                                       ├── Composer (interface)
                                       │     └── TelexComposer (VI only in Phase 1)
                                       │
                                       ├── LanguageRegistry (lookup + cycle order)
                                       ├── LanguageStripView (chips at top of IME)
                                       └── SharedSettings (new keys: activeLanguageId,
                                                          enabledLanguageIds)
```

`QwertyKeyboardView` becomes layout-agnostic. The hardcoded `alphaRows` / `symbols1Rows` / `symbols2Rows` arrays move into `EnglishLanguagePack`, preserving existing behavior byte-for-byte.

### 6.1 File structure

```
android/app/src/main/kotlin/com/draftright/keyboard/
├── DraftRightIME.kt              (modified — delegates to KeyboardController)
├── KeyboardController.kt         (new — coordinator)
├── QwertyKeyboardView.kt         (modified — layout-agnostic)
├── ToolbarView.kt                (unchanged)
├── LanguageStripView.kt          (new — language chips)
├── AccentPopupView.kt            (new — long-press picker)
├── SharedSettings.kt             (modified — 2 new keys)
├── LanguagePack.kt               (new — interface + KeyDef)
├── LanguageRegistry.kt           (new — lookup + cycle order)
├── Composer.kt                   (new — interface + ComposeResult)
├── composer/
│   ├── TelexComposer.kt          (new — VI state machine)
│   └── TelexState.kt             (new — pure data, testable)
└── lang/
    ├── EnglishLanguagePack.kt    (new — ports today's hardcoded layout)
    ├── VietnameseLanguagePack.kt (new — QWERTY + Telex composer factory)
    ├── FrenchLanguagePack.kt     (new — AZERTY + accents)
    ├── SpanishLanguagePack.kt    (new — QWERTY + ñ + accents)
    ├── GermanLanguagePack.kt     (new — QWERTZ + ä ö ü ß)
    ├── ItalianLanguagePack.kt    (new — QWERTY + accents)
    └── PortugueseLanguagePack.kt  (new — QWERTY + ç + accents)
```

## 7. Component contracts

### 7.1 `LanguagePack` (interface)

```kotlin
interface LanguagePack {
    val id: String                              // "en", "vi", "fr", "es", "de", "it", "pt"
    val displayName: String                     // "English", "Tiếng Việt", "Français", ...
    val locale: java.util.Locale                // for case mapping
    val alphaRows: List<List<KeyDef>>           // primary layout
    val symbols1Rows: List<List<KeyDef>>        // shared default; override when needed
    val symbols2Rows: List<List<KeyDef>>        // shared default; override when needed
    val longPressAccents: Map<Char, List<Char>> // 'a' → ['á','à','â','ä','ã']; empty for EN
    val composer: () -> Composer? = { null }    // factory; default no composition
}
```

Identity: `id` field. Lookup via `LanguageRegistry.byId(id)`.

### 7.2 `Composer` (interface)

```kotlin
interface Composer {
    fun onKey(char: Char): ComposeResult
    fun onBackspace(): ComposeResult
    fun reset()
    fun currentComposingText(): String
}

sealed class ComposeResult {
    object PassThrough : ComposeResult()                     // commit char as-is
    data class Commit(val text: String) : ComposeResult()    // replace composing region
    data class Composing(val text: String) : ComposeResult() // setComposingText
    object Consumed : ComposeResult()                        // backspace handled, no commit
}
```

Only `TelexComposer` implements it in Phase 1.

### 7.3 `TelexComposer` rules

| Trigger | Result |
|---|---|
| `aa` `oo` `ee` | `â` `ô` `ê` |
| `ow` `uw` | `ơ` `ư` |
| `aw` | `ă` |
| `dd` | `đ` |
| `<vowel>` + `s` | acute tone |
| `<vowel>` + `f` | grave tone |
| `<vowel>` + `r` | hook-above tone |
| `<vowel>` + `x` | tilde tone |
| `<vowel>` + `j` | dot-below tone |
| Backspace mid-composition | un-compose one step |
| Non-Telex letter after composing vowel | commit composed vowel, start fresh with new letter |
| Non-letter (space, digit, punctuation) | commit composed text, pass through new key |
| `reset()` | drop pending state immediately |

Length cap: 32 chars per composing word (defensive against runaway state).

### 7.4 `KeyboardController`

Owns:
- `var current: LanguagePack`
- `var composer: Composer?` (rebuilt on language switch)
- `val enabled: List<LanguagePack>` (from SharedSettings)

Public API:
- `onKey(char: Char)` — routes to composer or directly commits
- `onBackspace()` — composer first chance, else `deleteSurroundingText(1, 0)`
- `cycleLanguage()` — commit pending, reset composer, advance index, refresh strip + layout
- `setActive(langId: String)` — used by `LanguageStripView` chip taps
- `currentLayout(): List<List<KeyDef>>` — wraps current `alphaRows` + symbol-layer state

### 7.5 `LanguageStripView`

Always rendered as a dedicated row **directly above the existing tone toolbar** — fixed position, no responsive variants. One chip per enabled language. Active chip highlighted (blue fill). Tap chip = `controller.setActive(lang.id)`. Long-press chip = remove from enabled (Settings shortcut). The strip is hidden when the user has only one enabled language (no point cycling within one).

### 7.6 `AccentPopupView`

Created on-demand when `QwertyKeyboardView` detects a 300 ms hold over a key with a non-empty `longPressAccents` entry. Positioned above the held key (iOS convention). Finger drag selects, finger up commits.

### 7.7 `SharedSettings` additions

```kotlin
// new keys
"draftright.enabledLanguageIds"  // JSON-encoded List<String>, e.g. ["en","vi"]
"draftright.activeLanguageId"    // single String, e.g. "vi"

// defaults on first launch
enabledLanguageIds = ["en"]
activeLanguageId   = "en"
```

Existing key `draftright.translateLanguage` (rewrite target) is unchanged. The new keys are additive.

## 8. Error handling

| Failure | Detection | Behavior |
|---|---|---|
| `activeLanguageId` points to unknown id | `LanguageRegistry.byId` throws | Fall back to `"en"`, persist correction, log |
| `enabledLanguageIds` empty | `controller.enabled.isEmpty()` | Force-enable `"en"`, persist |
| Composer in stale state on field switch | `IME.onStartInput()` fires | `composer.reset()` |
| Long-press without drag | `MotionEvent.ACTION_UP` over original key | Commit plain letter, dismiss popup |
| Long-press with no accent map | `longPressAccents[char] == null` | No popup, normal short-press |
| Paste mid-composition | Composer's region cleared by `commitText` from system | Composer state reset on next key |
| Config change (rotation, theme) | `onCreateInputView` re-runs | Re-read `activeLanguageId`, composer state lost (acceptable) |
| Composer exception | `try/catch` around composer calls in controller | Fall back to direct commit, log |

Backward-compatibility guarantees (regression-proof EN):

1. `EnglishLanguagePack.alphaRows` is the exact data extracted from today's hardcoded `QwertyKeyboardView`.
2. Globe button still opens IME picker on long-press; new cycle behavior is on tap only.
3. No new permissions, no manifest edits, no backend changes.
4. Existing `draftright.translateLanguage` key is untouched.
5. Any new code path that throws falls back to today's commit-as-typed.

## 9. Testing

Test cases land in `docs/test-cases.xlsx` sheet `KEYBOARD-MULTI` **BEFORE** any Kotlin code is written (CLAUDE.md mandate).

### 9.1 xlsx catalog (54 rows)

| Group | Count |
|---|---|
| Telex composition — core rules | 12 |
| Telex composition — tones | 6 |
| Telex composition — real words | 4 |
| Telex backspace | 5 |
| Telex edge cases | 4 |
| Layout swap | 7 |
| Globe key | 3 |
| Long-press accent picker | 5 |
| Space-bar label | 1 |
| Settings persistence | 3 |
| Backward compatibility | 4 |

### 9.2 Kotlin unit tests (run via `./gradlew test`, no emulator)

```
android/app/src/test/kotlin/com/draftright/keyboard/
├── composer/TelexComposerTest.kt          ≈ 25 tests
├── lang/EnglishLanguagePackTest.kt
├── lang/VietnameseLanguagePackTest.kt
├── lang/FrenchLanguagePackTest.kt
├── lang/SpanishLanguagePackTest.kt
├── lang/GermanLanguagePackTest.kt
├── lang/ItalianLanguagePackTest.kt
├── lang/PortugueseLanguagePackTest.kt
├── LanguageRegistryTest.kt
└── KeyboardControllerTest.kt
```

### 9.3 Manual QA matrix (Stage 12 gate)

| Device | OS | Notes |
|---|---|---|
| Samsung A52 | Android 13 (One UI 5) | Samsung strips Process Text; verify globe cycle + Telex work in WhatsApp/Messages |
| Xiaomi Redmi | Android 12 (MIUI 14) | MIUI throttles background services; verify responsiveness |
| Pixel emulator | Android 14 | Baseline, fast iteration |

Per device, for each of the 7 languages:
- Type a real word with the language's diacritics
- Cycle languages mid-sentence via globe
- Long-press a vowel → verify accent popup appears in correct position
- Verify space-bar label matches `displayName`
- Force-stop + relaunch; verify `activeLanguageId` persists

### 9.4 CI

`./gradlew test` runs on push via GitHub Actions. Existing workflow extends to cover the new `android/app/src/test/` tree.

### 9.5 Performance budgets

| Metric | Budget |
|---|---|
| Keystroke → onscreen update | < 30 ms (Samsung A52) |
| `TelexComposer.onKey()` | < 1 ms (unit test asserts) |
| Language cycle | < 100 ms |
| Memory footprint of 7 LanguagePacks loaded | < 2 MB (dumpsys meminfo diff) |

## 10. Implementation order (Phase 1)

The full TDD plan is generated by `superpowers:writing-plans` next. High level:

1. **Stage 0 — Test cases in xlsx** (1 commit, no code)
2. **Stage 1 — Core abstractions** (`LanguagePack`, `Composer`, `LanguageRegistry`)
3. **Stage 2 — Globe key + LanguageStripView** wired to existing English-only
4. **Stage 3 — TelexComposer + TelexState** (TDD — 25+ unit tests come first)
5. **Stage 4 — VietnameseLanguagePack** + integration with QwertyKeyboardView
6. **Stage 5 — FrenchLanguagePack** (AZERTY layout)
7. **Stage 6 — GermanLanguagePack** (QWERTZ + ä ö ü ß)
8. **Stage 7 — SpanishLanguagePack** (QWERTY + ñ + accents)
9. **Stage 8 — ItalianLanguagePack** (QWERTY + accents)
10. **Stage 9 — PortugueseLanguagePack** (QWERTY + ç + accents)
11. **Stage 10 — Settings UI (Flutter side)** — enable/disable + reorder
12. **Stage 11 — Performance sweep** (latency + memory budgets)
13. **Stage 12 — Manual QA matrix** (Samsung + Xiaomi + Pixel emulator)
14. **Stage 13 — Onboarding update** (first-launch pitch + store copy + screenshots)
15. **Stage 14 — Merge + deploy** (develop → testing → main → production)
16. **Stage 15 — Memory note rewrite** (`project_android_keyboard_strategy.md`)

Estimated total: 7-9 working days.

## 11. Rollout

1. Feature branch `feature/android-multilang-keyboard-20260517` from develop
2. Add KEYBOARD-MULTI-001..054 to xlsx FIRST (Stage 0)
3. TDD all stages — `./gradlew test` green before each commit
4. Merge to develop via `--no-ff`
5. Deploy to testing server (here: build APK locally + sideload to Samsung A52)
6. Manual QA matrix (Stage 12) before merging to main
7. Merge develop → main via `--no-ff`
8. Build release APK + sign + upload to Google Play closed-testing track
9. Apply `status: deployed to production` label after Play Store publishes
10. Add `## ✅ How to Verify` comment on the tracking issue (URL + steps)

## 12. Open questions

None at design lock. Implementation plan will surface any remaining detail decisions.

## 13. Future work (explicitly out of scope here)

- VNI input method as a second Vietnamese option (per-user toggle)
- Predictive text / suggestion bar (build on top of current architecture)
- Autocorrect with per-language dictionaries
- CJK languages (Japanese / Chinese / Korean) — needs candidate-picker IME architecture, ~3-month build per language even with AI assistance
- iOS keyboard extension parity (Apple's "Full Access" prompt depresses adoption)
- Per-app language memory ("last used X in app Y")
- Cloud-sync of keyboard preferences across devices
- Glide / swipe typing
- Emoji search by text
