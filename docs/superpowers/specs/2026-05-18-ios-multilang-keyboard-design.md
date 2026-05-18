# iOS Multi-Language Keyboard (Tier β) — Design

**Date:** 2026-05-18
**Author:** Tan Nguyen
**Status:** Approved for planning
**Target client:** DraftRight Mobile — iOS keyboard extension at `DraftRightMobile/ios/DraftRightKeyboard/`
**Strategic note:** Android Tier β shipped 2026-05-17 ([[project_session_20260517]]). This spec ports the same feature set to iOS, with adjustments forced by Apple's sandbox + the system globe-key conflict + the "Full Access" prompt that historically depresses iOS keyboard adoption.

---

## 1. Problem

The iOS DraftRight Keyboard today is a QWERTY-only English keyboard with the rewrite toolbar above it. Users who type in Vietnamese / French / Spanish / German / Italian / Portuguese must switch to a different system keyboard (Apple's built-in, Gboard, etc.) — which means losing the inline rewrite toolbar that's the whole point of installing DraftRight Keyboard.

Android Tier β solved this by making the IME multi-language. iOS needs the same.

## 2. Goals (must match Android Tier β feature-for-feature)

- **Type natively** in English + Tiếng Việt + Français + Español + Deutsch + Italiano + Português without leaving the DraftRight Keyboard.
- **Vietnamese Telex composition** with the same rule set as Android (aa→â, oo→ô, ee→ê, aw→ă, ow→ơ, uw→ư, dd→đ, uow→ươ, all five tone marks s/f/r/x/j, diphthong promotion ie/uo/ye → iê/uô/yê on tone, 3-vowel cluster middle-vowel tone placement uoi → uôi).
- **Long-press accent picker** above the held key for diacritic-bearing vowels (Gboard / Apple convention).
- **In-keyboard language cycle** via an explicit `🌐 EN ↔ VI` chip that switches between DraftRight's enabled languages — *separate from* the system globe key (which would throw the user out of DraftRight entirely).
- **Native-name space-bar label** ("Tiếng Việt", "Español", …) confirms active language without a glance up.
- **Settings UI** in the Flutter app to enable / disable / reorder languages, persisted to the App Group so the keyboard ext picks up on next launch.
- **Zero regression** for English-only users.
- **No backend changes** — rewrite API already accepts UTF-8 from any language.

## 3. Non-goals

- VNI input method (Telex only in Phase 1, same as Android).
- Autocorrect / predictive text.
- Per-app language memory.
- Voice input, emoji search, cloud-sync of preferences.
- CJK (Japanese / Chinese / Korean) — candidate-picker architecture deferred.
- Swipe / glide typing.
- Bypassing or replacing the system globe key (Apple owns that interaction).

## 4. iOS-specific constraints that shape the design

| Constraint | Source | Impact on design |
|---|---|---|
| Keyboard extension memory limit ≈ 70 MB | Apple sandbox | 7 language packs must be lazy-loaded as needed; can't eagerly construct all 7 at startup |
| System globe key cycles between *all* installed keyboards | iOS UIInputViewController platform | Our in-keyboard language cycle must be a DIFFERENT button (a chip on the language strip), not the bottom-left globe |
| "Allow Full Access" toggle required for network calls | Apple sandbox | Already required today for the rewrite call; no new permission needed for Tier β (composer is offline) |
| Composer state must use `setMarkedText:selectedRange:` for the underline-while-typing UI | iOS UITextDocumentProxy | Mirror of Android's `setComposingText` — same model |
| Keyboard extension dies between input sessions (no background process) | iOS sandbox | `LanguageRegistry` must rebuild from `App Group` defaults on every `viewDidLoad`; no in-memory caching across sessions |
| App Store review for keyboard ext is stricter (Apple has rejected keyboards that "compete with system services") | App Store Review guideline 4.0 + 4.3 | Submission risk: Apple may push back on having 7 layouts since iOS already does multi-language. Mitigation: ship as ONE keyboard offering 7 languages (not 7 separate keyboards), framed as "DraftRight tone rewrite + native typing" rather than "language selector". |
| No drag-to-select on system popovers | iOS UIKit | Long-press accent picker must be a custom UIView, not `UIAlertController` |
| App Group ID for shared prefs | existing | Reuse `group.com.draftright.app.v2` — already wired for tokens + backendUrl |

## 5. User flows

### 5.1 English baseline (no change)

Press `a` → `textDocumentProxy.insertText("a")` → field shows `a`. Identical to today.

### 5.2 Vietnamese — typing `việt`

| Key | Composer state | textDocumentProxy call | Field display |
|---|---|---|---|
| `v` | `{ buffer: "v" }` | `setMarkedText("v", selectedRange: NSRange(location: 1, length: 0))` | `v` (underlined) |
| `i` | `{ buffer: "vi" }` | `setMarkedText("vi", …)` | `vi` |
| `e` | `{ buffer: "vie" }` | `setMarkedText("vie", …)` | `vie` |
| `t` | `{ buffer: "viet" }` | `setMarkedText("viet", …)` | `viet` |
| `j` | tone applied to "ie" cluster + diphthong promotion → `việt` | `unmarkText()` then `insertText("việt")` | `việt` (no underline) |

### 5.3 Language cycle via the strip chip (NOT the system globe)

User taps the `Tiếng Việt` chip on the language strip above the tone toolbar. Controller:
1. Unmarks any composing text (commits pending composer state).
2. Calls `composer?.reset()`.
3. Advances to the next enabled language.
4. Refreshes the strip + the QWERTY layout + the space-bar label.

The bottom-left system globe key keeps its native iOS behavior: tap once = cycle to the next installed keyboard; long-press = open the system keyboard picker. **DraftRight does not intercept the system globe key.**

### 5.4 Long-press accent picker (Spanish `á`)

User long-presses `a`. After 400 ms (Apple's default), an `AccentPopupView` (custom `UIView` anchored above the key) appears with `á à ä â ã`. User drags finger to `á`, releases. `textDocumentProxy.insertText("á")`.

### 5.5 No regression on rewrite flow

Tone toolbar (Polished / Friendly / Concise / Expand) routes through the same `BackendClient` as today, payload `{ text, tone }`. No `inputLanguage` field added.

## 6. Architecture

```
DraftRightKeyboard (UIInputViewController, modified)
        ↓ delegates keystroke routing to
KeyboardController (new) ──► textDocumentProxy (UITextDocumentProxy)
        │
        ├── LanguagePack (protocol)
        │     ├── EnglishLanguagePack
        │     ├── VietnameseLanguagePack
        │     ├── FrenchLanguagePack
        │     ├── SpanishLanguagePack
        │     ├── GermanLanguagePack
        │     ├── ItalianLanguagePack
        │     └── PortugueseLanguagePack
        │
        ├── Composer (protocol)
        │     └── TelexComposer (VI only in Phase 1)
        │
        ├── LanguageRegistry (lookup + cycle order)
        ├── LanguageStripView (chips at top of IME, new)
        ├── AccentPopupView (long-press picker, new)
        └── SharedSettings (modified — 2 new App Group keys)
```

`QwertyKeyboardView` becomes layout-agnostic. The hardcoded rows move into `EnglishLanguagePack` verbatim.

### 6.1 File structure

```
DraftRightMobile/ios/DraftRightKeyboard/
├── KeyboardViewController.swift     (modified — delegates to controller)
├── KeyboardController.swift         (new — coordinator)
├── QwertyKeyboardView.swift         (modified — layout-agnostic)
├── ToolbarView.swift                (unchanged)
├── LanguageStripView.swift          (new — language chips)
├── AccentPopupView.swift            (new — long-press picker)
├── SharedSettings.swift             (modified — 2 new App Group keys)
├── LanguagePack.swift               (new — protocol + KeyDef)
├── LanguageRegistry.swift           (new — lookup + cycle order)
├── Composer.swift                   (new — protocol + ComposeResult)
├── Composer/
│   ├── TelexComposer.swift          (new — VI state machine, ported from Kotlin)
│   └── TelexState.swift             (new — pure data, testable)
└── Lang/
    ├── EnglishLanguagePack.swift    (new — ports today's hardcoded layout)
    ├── VietnameseLanguagePack.swift (new — QWERTY + TelexComposer factory)
    ├── FrenchLanguagePack.swift     (new — AZERTY + accents)
    ├── SpanishLanguagePack.swift    (new — QWERTY + ñ + accents)
    ├── GermanLanguagePack.swift     (new — QWERTZ + ä ö ü ß)
    ├── ItalianLanguagePack.swift    (new — QWERTY + accents)
    └── PortugueseLanguagePack.swift (new — QWERTY + ç + accents)
```

### 6.2 Unit tests (Swift Package, headless — no simulator)

```
DraftRightMobile/ios/DraftRightKeyboardTests/
├── Composer/TelexComposerTests.swift          ~29 tests (mirror Android)
├── Composer/TelexStateTests.swift
├── LanguageRegistryTests.swift
├── KeyboardControllerTests.swift
└── Lang/
    ├── EnglishLanguagePackTests.swift
    ├── VietnameseLanguagePackTests.swift
    └── LatinLanguagePackTests.swift   (FR/ES/DE/IT/PT bundled)
```

Run via `swift test` against a SwiftPM target so we don't pay the simulator boot cost on every CI run.

## 7. Component contracts (Swift, mirroring Kotlin signatures from Android)

### 7.1 `LanguagePack`

```swift
public protocol LanguagePack {
    var id: String { get }                                  // "en", "vi", "fr", ...
    var displayName: String { get }                         // "English", "Tiếng Việt", ...
    var locale: Locale { get }
    var alphaRows: [[KeyDef]] { get }
    var symbols1Rows: [[KeyDef]] { get }
    var symbols2Rows: [[KeyDef]] { get }
    var longPressAccents: [Character: [Character]] { get }
    func makeComposer() -> Composer?
}

public extension LanguagePack {
    func makeComposer() -> Composer? { nil }                // default: no composition
}

public struct KeyDef {
    public let label: String
    public let code: Int
    public let widthWeight: CGFloat
    public init(_ label: String, _ code: Int, _ widthWeight: CGFloat = 1.0) { … }
}
```

### 7.2 `Composer` (protocol) + `ComposeResult`

```swift
public protocol Composer: AnyObject {
    func onKey(_ char: Character) -> ComposeResult
    func onBackspace() -> ComposeResult
    func reset()
    func currentComposingText() -> String
}

public enum ComposeResult: Equatable {
    case passThrough
    case commit(String)
    case composing(String)
    case consumed
}
```

Only `TelexComposer` implements it in Phase 1.

### 7.3 `TelexComposer` — direct port of Kotlin

Same rules as `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/composer/TelexComposer.kt`. Algorithm:

| Trigger | Result |
|---|---|
| `aa` `oo` `ee` | `â` `ô` `ê` |
| `ow` `uw` | `ơ` `ư` |
| `aw` | `ă` |
| `dd` | `đ` |
| `uow` (longest match — applied before `ow` / `uw`) | `ươ` |
| `<vowel>` + `s/f/r/x/j` | acute / grave / hook / tilde / dot-below tone |
| 2-vowel cluster ie/uo/ye + tone | promote second to special before tone (works in vie+j → việ + t = việt; tieng+s = tiếng) |
| 3-vowel cluster uoi/ieu/yeu + tone | promote middle vowel to special before tone (chuow+ng = chương; nguow+if = người) |
| Backspace | layered strip: tone → mark → drop char |
| Non-letter | commit buffer + pass through new char |
| Length cap | 32 chars per composing word — commit and start fresh |

### 7.4 `KeyboardController`

```swift
public final class KeyboardController {
    public let registry: LanguageRegistry
    public private(set) var enabled: [LanguagePack]
    public private(set) var current: LanguagePack
    public private(set) var composer: Composer?

    public func cycleLanguage()
    public func setActive(id: String)
    public func onKey(_ char: Character) -> KeystrokeOutcome
    public func onBackspace() -> KeystrokeOutcome
}

public enum KeystrokeOutcome {
    case commit(String)
    case composing(String)
    case deleteOne
    case noChange
}
```

### 7.5 `LanguageStripView`

A horizontal `UIScrollView` of `UIButton` chips rendered ABOVE the tone toolbar. Active chip blue-filled. Hidden when only one language is enabled. **Long-press on a chip is NOT bound** (unlike Android where it removes the language — keep iOS simpler; reorder/disable goes through the main app's Settings screen exclusively).

### 7.6 `AccentPopupView`

Custom `UIView` instantiated when `QwertyKeyboardView` detects a 400 ms hold on a key with a non-empty `longPressAccents` entry. Positioned via `convert(_:to:)` above the held key. Pan gesture tracks finger; finger up commits the highlighted option. Dismiss on `UIGestureRecognizerStateCancelled`.

### 7.7 `SharedSettings` additions

```swift
extension SharedSettings {
    // App Group keys — match Android string keys so cross-platform Flutter
    // settings UI can write once, read on both.
    public var enabledLanguageIds: [String] {
        get { ... }
        set { ... }
    }
    public var activeLanguageId: String {
        get { ... }
        set { ... }
    }
}
```

Storage: `UserDefaults(suiteName: "group.com.draftright.app.v2")`. Keys: `flutter.draftright.enabledLanguageIds` (JSON-encoded `[String]`) and `flutter.draftright.activeLanguageId` (String). Flutter's `shared_preferences` library writes these on the app side already (we added them in Android Stage 10).

## 8. Error handling

| Failure | Detection | Behavior |
|---|---|---|
| `activeLanguageId` points to unknown id | `LanguageRegistry.byId` returns nil | Fall back to `"en"`, persist correction |
| `enabledLanguageIds` empty | `controller.enabled.isEmpty` | Force-enable `"en"`, persist |
| Composer stale on input-field switch | `viewWillAppear` / `viewWillDisappear` | `composer?.reset()` |
| Long-press without pan | `UIPanGestureRecognizer.cancelled` over original key | Commit plain letter, dismiss popup |
| Long-press with no accent map | `longPressAccents[char] == nil` | No popup, normal short-press |
| Paste mid-composition | `textWillChange` fires on programmatic set | Composer state reset on next key |
| Memory pressure | `applicationDidReceiveMemoryWarning` (forwarded from main app via extension lifecycle) | Drop accent popup if visible, no other action needed (7 packs ≈ 200 KB total) |

## 9. Testing

Test cases land in `docs/test-cases.xlsx` sheet `KEYBOARD-MULTI-IOS` **BEFORE** any Swift code is written (mirrors Android Stage 0).

### 9.1 Test catalog (54 rows — mirror Android `KEYBOARD-MULTI` sheet)

Same groupings, same priorities, same Telex rule coverage. Differences from Android catalog:
- Globe-key tests reflect iOS system behavior (tap globe = next system keyboard, NOT DraftRight cycle)
- Add 2 new iOS-specific tests: "Full Access toggle reminder shows on first launch if disabled" + "Keyboard ext survives 70 MB memory cap with all 7 packs loaded"

### 9.2 Swift unit tests (`swift test`, no simulator)

Same suite count as Android — 29 Telex tests, 3 TelexState, 6+ per Latin pack, controller + registry + LanguagePack.

### 9.3 Manual QA matrix (Stage 13 gate)

| Device | iOS | Notes |
|---|---|---|
| iPhone 14 (real device) | iOS 17.x | Apple's "Full Access" prompt path; verify network reaches `api.draftright.info` post-enable |
| iPhone SE 3rd gen (real device) | iOS 16.x | Smaller screen — verify language strip + tone toolbar both fit |
| iPhone simulator (Xcode 26) | iOS 17.x | Baseline + fast iteration, but ⚠️ Full Access path can't be fully tested in simulator |

Per device, for each of 7 languages: type a real word, cycle via chip, long-press a vowel + drag, verify space-bar label, force-stop + relaunch.

### 9.4 Performance budgets

| Metric | Budget |
|---|---|
| Keystroke → on-screen | < 30 ms on iPhone SE 3rd gen |
| `TelexComposer.onKey()` | < 1 ms (unit-test asserted) |
| Language cycle | < 100 ms |
| Memory footprint of 7 LanguagePacks loaded | < 2 MB (Instruments diff vs single-pack baseline) |
| Total keyboard ext peak | < 50 MB (Apple's hard cap ~70 MB) |

## 10. Implementation order (Phase 1)

Mirrors Android Tier β stages exactly. High-level:

1. **Stage 0** — Test cases in xlsx (`KEYBOARD-MULTI-IOS`)
2. **Stage 1** — Core abstractions (`LanguagePack`, `Composer`, `LanguageRegistry`) + Swift test target
3. **Stage 2** — Language strip + chip-driven cycle wired to existing English-only
4. **Stage 3** — `TelexComposer` + `TelexState` (TDD — port the Kotlin tests verbatim)
5. **Stage 4** — `VietnameseLanguagePack` + composer integration via `setMarkedText`
6. **Stage 5** — `FrenchLanguagePack` (AZERTY)
7. **Stage 6** — `GermanLanguagePack` (QWERTZ + ä ö ü ß)
8. **Stage 7** — `SpanishLanguagePack` (QWERTY + ñ + accents)
9. **Stage 8** — `ItalianLanguagePack`
10. **Stage 9** — `PortugueseLanguagePack`
11. **Stage 10** — `AccentPopupView` (long-press picker with pan-to-select)
12. **Stage 11** — Cross-platform settings UI parity check (Flutter side already added in Android Stage 10 — verify it reads/writes correctly to the App Group on iOS too)
13. **Stage 12** — Performance sweep + memory check on iPhone SE 3rd gen
14. **Stage 13** — Manual QA matrix (real iPhone 14 + SE + simulator)
15. **Stage 14** — Submit to App Store + monitor for guideline 4.0/4.3 rejection
16. **Stage 15** — Memory note rewrite ([[project_ios_keyboard_strategy]] new memory; supersedes the "iOS keyboard ext is out of scope" line)

Estimated total: **8-10 working days** (slightly longer than Android due to App Store review queue + the in-keyboard-cycle-without-system-globe-conflict design).

## 11. Rollout

1. Feature branch `feature/ios-multilang-keyboard-20260518` from develop
2. Add KEYBOARD-MULTI-IOS-001..056 to xlsx FIRST (Stage 0)
3. TDD all stages — `swift test` green before each commit
4. Merge to develop via `--no-ff`
5. Build via Xcode Cloud OR `fastlane ios beta` (manual)
6. Upload to TestFlight, distribute to internal testers
7. Manual QA matrix (Stage 13) before App Store submission
8. Merge develop → main via `--no-ff`
9. `fastlane ios release` → uploads to App Store Connect → submit for review
10. **Monitor review state** — Apple may push back on the "competing with system services" line. If so, reframe the App Store description and resubmit. Worst case: keep keyboard ext as EN-only, ship multi-language only on Android.

## 12. Strategic risk acknowledgement

Per memory [[project_android_keyboard_strategy]], the iOS keyboard ext historically positioned as "out of scope" because:
1. Apple's "Full Access" prompt depresses adoption (we still pay this today regardless of Tier β).
2. Apple Intelligence's "Writing Tools" already cover the rewrite flow on iOS 18.1+.
3. CJK is the more valuable beachhead and we're deferring it anyway.

Tier β on iOS makes the keyboard ext **competitive with Apple Intelligence on languages Apple Intel doesn't cover yet** (Vietnamese specifically — Apple Intel won't ship VN any time soon based on Apple's roadmap signals). That's the value case. **If the Apple Intel roadmap reveals a VN announcement, abandon iOS Tier β** and double down on Android share-extension + Vietnamese marketing.

## 13. Future work (out of scope for Tier β iOS)

- VNI input method (Phase 2)
- Predictive text / suggestion bar
- Autocorrect with per-language dictionaries
- CJK (J / Zh / Ko) — needs candidate picker
- Per-app language memory
- Cloud-sync of preferences (already deferred on Android too)
- Glide / swipe typing
- Emoji search by text
