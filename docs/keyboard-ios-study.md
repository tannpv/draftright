# Apple iOS Keyboard — Behavior Study & DraftRight Parity Map

Companion to `docs/keyboard-samsung-study.md`, for the iOS side. Goal: make
DraftRight's iOS keyboard feel like Apple's stock keyboard. Sourced from the
documented iOS keyboard behavior + a scan of DraftRight's iOS extension
(`DraftRightKeyboard/` UI + `DraftRightKeyboardCore/` engine). Feeds #209.

## 1. Apple stock iOS keyboard — behaviors / functions / UI

| Area | Behavior |
|---|---|
| **Key feedback** | Key **click sound** (gated on Settings ▸ Sounds); **key-preview bubble** magnifies the letter above the finger on iPhone; optional **haptic** (iOS 16+, Settings ▸ Sounds & Haptics ▸ Keyboard Feedback). Haptic in a 3rd-party keyboard needs **Full Access**. |
| **QuickType predictive bar** | Suggestion strip above the keys: next-word predictions + inline **autocorrect** candidate; tap to insert; autocorrect auto-applies on space/punct. |
| **Autocorrect & text** | Auto-capitalize at sentence start, **double-space → ". "**, **smart punctuation** (straight→curly quotes, -- → —), auto-apostrophe. |
| **Long-press** | Letter → **accent popup** (é è ê …); `.?123` press-drag-release for a one-shot symbol; period → domain suffixes (.com) on URL fields. |
| **Shift** | Tap = one-shot; **double-tap = caps lock**; auto-shift after sentence end. |
| **Delete** | Tap = 1 char; hold = accelerating repeat → word-wise. |
| **Spacebar cursor** | **Long-press spacebar → trackpad** to move the caret (signature iOS gesture). |
| **Globe / languages** | Tap cycles keyboards; long-press → list + **one-handed** left/right + emoji. |
| **Emoji keyboard** | Built-in 😀 key / globe entry. |
| **Field-adaptive layout** | Return key label adapts (**Go / Search / Send / Done / Join**); email field surfaces `@` `.`; URL field surfaces `/` `.com`; **number-only pad** for `numberPad`/OTP. |
| **Dictation** | System mic. |
| **Glide typing** | **QuickPath** swipe-to-type (iOS 13+). |
| **Undo** | Shake / 3-finger swipe. |

## 2. DraftRight iOS keyboard today — has vs lacks

Architecture already mirrors Android: `DraftRightKeyboardCore` (TelexComposer
476 lines, TrigramCandidateEngine, WordListPackResolver, bootstrap wordlists,
KeyboardController, VoiceSessionController — all with tests) + `DraftRightKeyboard`
UI (KeyboardViewController 557, QwertyKeyboardView 565, CandidateBarView,
AccentPopupView, ToolbarView).

| Apple behavior | DraftRight iOS | Status |
|---|---|---|
| Key-preview bubble | `showKeyPreview` in QwertyKeyboardView | ✅ has |
| Accent long-press popup | `AccentPopupView` | ✅ has |
| Auto-capitalize | `updateAutoCaps` / `AutoCapitalize.atSentenceStart` | ✅ has |
| Globe / language switch | `advanceToNextInputMode` + space-swipe language cycle | ✅ has (space-swipe, not iOS trackpad) |
| Shift one-shot / caps lock | shiftState machine | ✅ has |
| Delete repeat | hold-repeat | ✅ has |
| Voice / dictation | `SpeechVoiceInput` | ✅ has |
| Telex / composer + candidate bar | Core engine + `CandidateBarView` | ✅ has (engine) |
| Numeric layer on numeric fields | `setNumericLayer` via `keyboardType` | ⚠️ reuses symbols layer — **needs #208-style number-only pad** |
| **Key click sound + haptic** | none (grep: no `playInputClick`/`UIImpactFeedback`) | ❌ **lacks** (#209 item 1) |
| **Predictive next-word / autocorrect** | `previousTokens: []` hardcoded (KeyboardViewController.swift:203); no bigrams in `VietnameseBootstrapWordList` | ❌ **lacks** (#209 item 2) |
| **Smart punctuation** (curly quotes, --→—, ". ") | none found | ❌ lacks |
| **Adaptive return-key label** (Go/Search/Send/Done) | generic ↵ | ❌ lacks |
| **Field-adaptive keys** (email `@`/`.`, URL `.com`) | only numeric detection | ❌ lacks |
| **Spacebar-cursor trackpad** | spacebar pan = language swipe, not caret move | ❌ lacks (gesture repurposed) |
| **Emoji keyboard** | none | ❌ lacks |
| **Glide / QuickPath typing** | none (only space-swipe) | ❌ lacks |
| **One-handed keyboard** | none | ❌ lacks |

## 3. Replication roadmap (iOS, mirrors the Android parity order)

Same three high-impact items as Android (#209), then iOS-specific polish:

1. **Key feedback** — Swift `KeyFeedback`: `UIDevice.playInputClick()` (controller
   conforms to `UIInputViewAudioFeedback` + `enableInputClicksWhenVisible`);
   `UIImpactFeedbackGenerator` for haptic **when Full Access is granted**
   (degrade to click-only otherwise). One chokepoint at keypress. (#209-1)
2. **Predictive + autocorrect** — wire `previousTokens` from
   `documentContextBeforeInput` (Swift `PreviousTokens`, mirror the Kotlin
   helper), add VI bigrams to the bootstrap wordlist (agree with Android's
   `wordlist_vi_bigrams.tsv` via a parity test). Then inline autocorrect. (#209-2)
3. **Number-only pad** — port #208's dedicated numeric keypad to the Swift
   `QwertyLayout` + point `setNumericLayer` at it. (#209-3)
4. **iOS-specific, high-value:**
   - **Adaptive return-key label** — read `textDocumentProxy.returnKeyType`, relabel ↵ (Go/Search/Send/Done). Cheap, very "native".
   - **Smart punctuation** — double-space→". ", curly quotes, --→—.
   - **Field-adaptive keys** — email (`@` `.`) / URL (`.com` `/`) rows off `keyboardType`.
   - **Spacebar-cursor trackpad** — the signature iOS caret gesture. Note it
     conflicts with the current space-swipe language cycle; pick a gesture split
     (e.g. long-press-then-pan = cursor, quick-swipe = language).
5. **Later / larger:** emoji keyboard, glide typing, one-handed mode.

## 4. RULE #1 for the iOS port

Where Android and iOS can't share code (Kotlin vs Swift), the duplicated facts
— numeric layout, VI bigram data, key-kind→sound map, `NumericField` class list,
`PreviousTokens` rules — each get an **agreement test** so the two copies can't
drift (the #22 lesson). `NumericField.isNumericKeyboard` already documents this
Kotlin↔Swift mirror intent.

## 5. What NOT to chase first

Glide typing, emoji, and one-handed are large and not what makes typing "feel
native" — feedback + prediction + adaptive return key do. Match the Android
order: feedback → prediction → numeric pad, then the cheap iOS-native polish
(return-key label, smart punctuation).
