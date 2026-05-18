# iOS Multi-Language Keyboard (Tier β) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port Android Tier β (EN + VI Telex + FR/ES/DE/IT/PT) to the iOS keyboard extension at `DraftRightMobile/ios/DraftRightKeyboard/`. Same UX, adapted for iOS sandbox constraints.

## REVISION 2026-05-18 (post-Android-shipping learnings)

The Android Tier β reached production today (2.3.1+52). Several design decisions in this plan have been REVISED based on what actually worked vs broke on real Samsung A52 + Android 14:

1. **Drop `LanguageStripView` from the iOS view tree.** Android ditched the chip strip in favor of **swipe-right/left on the space bar** to cycle languages (Samsung-style). UX is cleaner + saves a row of vertical space.
2. **Drop the redundant `GLOBE_PICKER` ≡ key.** Long-press the system globe (iOS already provides the globe key) for system picker.
3. **Telex composer needs three specific fixes** that surfaced only on real devices:
   - **Empty-buffer backspace must explicitly clear marked text** — when composer strips buffer to empty, the IC's marked-text region survives and looks stuck on the first char. Fix in iOS: after `composer.onBackspace()` returns `.consumed`, call `textDocumentProxy.unmarkText()` + `textDocumentProxy.deleteBackward()` if appropriate.
   - **Space-bar must route through composer** — direct `textDocumentProxy.insertText(" ")` replaces the marked-text region with just `" "`, deleting the whole composing word. Fix: route `keyboardDidSpace` through `controller.onKey(" ")` so the composer commits any pending word before appending the space.
   - **Tone-rewrite Replace must finish composition first** — `replaceAllText` must call `textDocumentProxy.unmarkText()` BEFORE `setSelectedRange + insertText`, otherwise the composing region survives the replace.
4. **Bisection-driven incremental ship.** Each commit was a single-axis change (view structure → registry → strip-GONE → strip-VISIBLE → setter → cycling → composer routing). For iOS, follow the same pattern so any regression bisects in < 1 hr instead of getting lost in a 1k-line refactor.

The relevant stages below have been updated to reflect this. See [[project_session_20260518_android_tier_beta_v2]] for the full Android shipping log.

---

**Spec:** `docs/superpowers/specs/2026-05-18-ios-multilang-keyboard-design.md`
**Reference (Kotlin source of truth):** Android implementation merged to main 2026-05-17. See:
- `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/composer/TelexComposer.kt`
- `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/lang/*LanguagePack.kt`
- `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/composer/TelexComposerTest.kt`

**Tech Stack:** Swift 5.9 (iOS keyboard extension target), XCTest (`swift test` against a SwiftPM target), Flutter (settings UI parity check only — no changes needed if App Group keys are correctly named).

---

## File structure

### New Swift files (10)

```
DraftRightMobile/ios/DraftRightKeyboard/
├── LanguagePack.swift              protocol + KeyDef + helpers
├── LanguageRegistry.swift          lookup + cycle order
├── Composer.swift                  protocol + ComposeResult enum
├── KeyboardController.swift        coordinator + KeystrokeOutcome
├── LanguageStripView.swift         chips above tone toolbar
├── AccentPopupView.swift           long-press picker (400ms hold, pan-to-select)
├── Composer/
│   ├── TelexComposer.swift         VI state machine — port of Kotlin
│   └── TelexState.swift            pure helpers, testable
└── Lang/
    ├── EnglishLanguagePack.swift
    ├── VietnameseLanguagePack.swift
    ├── FrenchLanguagePack.swift
    ├── SpanishLanguagePack.swift
    ├── GermanLanguagePack.swift
    ├── ItalianLanguagePack.swift
    └── PortugueseLanguagePack.swift
```

### Modified Swift files (3)

```
DraftRightKeyboard/KeyboardViewController.swift   delegate keystroke routing to KeyboardController
DraftRightKeyboard/QwertyKeyboardView.swift       accept layout from LanguagePack; remove hardcoded rows
DraftRightKeyboard/SharedSettings.swift           add enabledLanguageIds + activeLanguageId
```

### New test target

```
DraftRightMobile/ios/DraftRightKeyboardTests/
├── Composer/TelexComposerTests.swift          ~29 tests (port of Android TelexComposerTest)
├── Composer/TelexStateTests.swift
├── KeyboardControllerTests.swift
├── LanguageRegistryTests.swift
└── Lang/
    ├── EnglishLanguagePackTests.swift
    ├── VietnameseLanguagePackTests.swift
    └── LatinLanguagePackTests.swift   (FR/ES/DE/IT/PT bundled)
```

Run via `swift test` against a new SwiftPM target named `DraftRightKeyboardCore` that exposes the testable Swift sources. Test target avoids the Xcode simulator for fast iteration.

### Modified docs

```
docs/test-cases.xlsx                            new sheet KEYBOARD-MULTI-IOS (54 rows)
```

### Flutter side — no code changes if App Group keys match

The Flutter settings UI already added "Keyboard languages" FilterChips in Android Stage 10 (`DraftRightMobile/lib/screens/settings_screen.dart` line 206+) using `SettingsService.keyboardLanguageCatalog`. The keys it writes are `draftright.enabledLanguageIds` and `draftright.activeLanguageId`. Flutter's `shared_preferences` library prefixes them with `flutter.` automatically.

iOS-side `SharedSettings` reads from the App Group `group.com.draftright.app.v2` using those same prefixed keys. **No Flutter code changes** if `SharedSettings.swift` reads `UserDefaults(suiteName: "group.com.draftright.app.v2")?.stringArray(forKey: "flutter.draftright.enabledLanguageIds")` exactly.

---

## Stage 0 — Test cases in xlsx (CRITICAL, before any Swift)

### Task 0.1: Add KEYBOARD-MULTI-IOS sheet

**Files:**
- Create: `scripts/seed-keyboard-multi-ios-test-cases.py`
- Modify: `docs/test-cases.xlsx`

Adapt `scripts/seed-keyboard-multi-test-cases.py` — copy verbatim, change:
- `SHEET = "KEYBOARD-MULTI-IOS"`
- Test IDs to `KEYBOARD-MULTI-IOS-001..054`
- "Verified by" column: replace `TelexComposerTest.kt` → `TelexComposerTests.swift`, `KeyboardControllerTest.kt` → `KeyboardControllerTests.swift`, etc.
- Replace globe-key tests:
  - `IOS-039`: "Strip chip tap cycles enabled languages" (replaces Android's globe-tap cycle)
  - `IOS-040`: "System globe key opens system keyboard picker (unchanged from iOS default)"
  - `IOS-041`: "Single enabled language: strip hidden, no cycle UI" (same)
- Add 2 iOS-specific tests at the end:
  - `IOS-055`: "Full Access disabled → keyboard works for typing but rewrite button shows 'Allow Full Access in Settings' banner"
  - `IOS-056`: "All 7 language packs loaded → memory under 50 MB on iPhone SE 3rd gen (Instruments Allocations diff)"

Total: 56 rows.

- [ ] **Step 1: Write the script** (model on the Android seeder)
- [ ] **Step 2: Run it** — `python3 scripts/seed-keyboard-multi-ios-test-cases.py`. Expected: `Added KEYBOARD-MULTI-IOS with 56 rows.`
- [ ] **Step 3: Branch + commit**

```bash
git checkout develop
git pull
git checkout -b feature/ios-multilang-keyboard-20260518
git add scripts/seed-keyboard-multi-ios-test-cases.py docs/test-cases.xlsx
git commit -m "test(ios): add KEYBOARD-MULTI-IOS-001..056 (before-code mandate)"
```

---

## Stage 1 — Core abstractions + Swift test target

### Task 1.1: Create SwiftPM testable subtarget

**Files:**
- Modify: `DraftRightMobile/ios/Podfile` and/or create `Package.swift` at `DraftRightMobile/ios/DraftRightKeyboardCore/`

Goal: factor the testable Swift sources (LanguagePack, Composer, etc.) into a SwiftPM package so `swift test` can run them headless without launching the simulator.

- [ ] **Step 1:** Create `DraftRightMobile/ios/DraftRightKeyboardCore/Package.swift` with `library` product `DraftRightKeyboardCore` + `testTarget` `DraftRightKeyboardCoreTests`. Both target macOS 13 + iOS 13 so the same code compiles for both host JVM-equivalent (here: macOS for `swift test`) and the iOS extension build.
- [ ] **Step 2:** Add `import DraftRightKeyboardCore` to `KeyboardViewController.swift` once the package is wired into the Xcode workspace via a local-path dependency.
- [ ] **Step 3:** Verify `cd DraftRightMobile/ios/DraftRightKeyboardCore && swift test` builds (no tests yet, empty pass).
- [ ] **Step 4:** Commit.

### Task 1.2: `KeyDef` + `LanguagePack` protocol

**Files:**
- Create: `…/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/LanguagePack.swift`
- Create: `…/DraftRightKeyboardCore/Tests/DraftRightKeyboardCoreTests/LanguagePackTests.swift`

- [ ] **Step 1: Failing test** — assert protocol shape (id, displayName, locale, alphaRows count, KeyDef widthWeight default).
- [ ] **Step 2: Run, expect compile FAIL** (no protocol yet).
- [ ] **Step 3: Implement** — mirror Kotlin `LanguagePack.kt`:

```swift
public struct KeyDef {
    public let label: String
    public let code: Int
    public let widthWeight: CGFloat
    public init(_ label: String, _ code: Int, widthWeight: CGFloat = 1.0) {
        self.label = label
        self.code = code
        self.widthWeight = widthWeight
    }
}

public protocol LanguagePack {
    var id: String { get }
    var displayName: String { get }
    var locale: Locale { get }
    var alphaRows: [[KeyDef]] { get }
    var symbols1Rows: [[KeyDef]] { get }
    var symbols2Rows: [[KeyDef]] { get }
    var longPressAccents: [Character: [Character]] { get }
    func makeComposer() -> Composer?
}

public extension LanguagePack {
    func makeComposer() -> Composer? { nil }
}
```

- [ ] **Step 4: Run, expect PASS** (3 tests).
- [ ] **Step 5: Commit.**

### Task 1.3: `Composer` protocol + `ComposeResult`

**Files:**
- Create: `…/Sources/DraftRightKeyboardCore/Composer.swift`

```swift
public enum ComposeResult: Equatable {
    case passThrough
    case commit(String)
    case composing(String)
    case consumed
}

public protocol Composer: AnyObject {
    func onKey(_ char: Character) -> ComposeResult
    func onBackspace() -> ComposeResult
    func reset()
    func currentComposingText() -> String
}
```

No tests yet — concrete implementations get their own.

- [ ] Commit alongside Task 1.2.

### Task 1.4: `SpecialKeys` constants

**Files:**
- Create: `…/Sources/DraftRightKeyboardCore/SpecialKeys.swift`

Mirror Android constants exactly:

```swift
public enum SpecialKeys {
    public static let shift = -1
    public static let symbols = -2
    public static let globe = -3
    public static let enter = -4
    public static let backspace = -5
    public static let symbols2 = -6
    public static let alpha = -7
    public static let globePicker = -8

    public static func isSpecial(_ code: Int) -> Bool { code < 0 }
}
```

- [ ] Commit.

### Task 1.5: `LanguageRegistry`

**Files:**
- Create: `…/Sources/DraftRightKeyboardCore/LanguageRegistry.swift`
- Create: `…/Tests/.../LanguageRegistryTests.swift`

Same 6 test cases as Kotlin (byId / byIdOrDefault / next / wrap / empty-init throws). Port verbatim.

- [ ] Test → implement → run → commit.

### Task 1.6: `EnglishLanguagePack` (port today's hardcoded rows verbatim)

**Files:**
- Create: `…/Sources/DraftRightKeyboardCore/Lang/EnglishLanguagePack.swift`
- Create: `…/Tests/.../EnglishLanguagePackTests.swift`

Read the current `DraftRightMobile/ios/DraftRightKeyboard/QwertyKeyboardView.swift` rows (around lines 50-160). Port verbatim into the new pack. Same 7-key bottom row, same space-bar that today reads "space" — will be updated to display `displayName` once the controller is wired.

- [ ] 8 tests (mirror Android `EnglishLanguagePackTest.kt`): id, displayName, 4 alpha rows, top row q…p, home row a…l, composer nil, accents empty, symbols digits 1-0, symbols2 contains π.

---

## Stage 2 — KeyboardController + swipe-space cycling (no strip)

> **REVISED 2026-05-18 evening**: original plan called for a horizontal `LanguageStripView` of chips. Android shipping experience showed users prefer the Samsung-style **swipe-space-bar to cycle**. This stage now wires that gesture instead of adding a strip. The `LanguageStripView.swift` from the original spec is NOT built.

## Stage 2 (deprecated original) — KeyboardController + LanguageStripView

### Task 2.1: SharedSettings App Group keys

**Files:**
- Modify: `DraftRightMobile/ios/DraftRightKeyboard/SharedSettings.swift`

Add two computed properties reading from `UserDefaults(suiteName: "group.com.draftright.app.v2")` with Flutter prefixes:

```swift
extension SharedSettings {
    public var enabledLanguageIds: [String] {
        let raw = defaults?.stringArray(forKey: "flutter.draftright.enabledLanguageIds") ?? []
        return raw.isEmpty ? ["en"] : raw
    }

    public var activeLanguageId: String {
        defaults?.string(forKey: "flutter.draftright.activeLanguageId") ?? "en"
    }
}
```

- [ ] Commit.

### Task 2.2: `KeyboardController` + `KeystrokeOutcome`

**Files:**
- Create: `…/Sources/DraftRightKeyboardCore/KeyboardController.swift`
- Create: `…/Tests/.../KeyboardControllerTests.swift`

Port Kotlin `KeyboardController.kt` line-by-line. Same 7 tests:
1. Init defaults to first enabled when activeId empty
2. Init honors activeId in enabled
3. Cycle wraps within enabled subset
4. Cycle no-op when single enabled
5. Disabled-all force-enables registry first
6. setActive switches to enabled pack
7. setActive no-op for disabled or same id

Plus `KeystrokeOutcome` enum + `onKey` + `onBackspace` methods routing through the composer (mirrors Android `KeystrokeOutcome` sealed class).

- [ ] Commit.

### Task 2.3: Wire `KeyboardViewController` + `QwertyKeyboardView` to controller (EN-only, no behavior change)

**Files:**
- Modify: `DraftRightMobile/ios/DraftRightKeyboard/KeyboardViewController.swift`
- Modify: `DraftRightMobile/ios/DraftRightKeyboard/QwertyKeyboardView.swift`

- [ ] Construct `KeyboardController` in `viewDidLoad()`
- [ ] Remove `QwertyKeyboardView`'s hardcoded `alphaRows` / `symbols1Rows` / `symbols2Rows` properties
- [ ] Add `var languagePack: LanguagePack = EnglishLanguagePack()` with a `didSet { buildKeyboard() }` observer
- [ ] In existing `handleKeyPress`, replace `KeyCode.X` enum switch with `if code == SpecialKeys.X` checks (mirror Android port)
- [ ] On simulator: type "hello" — must be identical to today

- [ ] Commit.

### Task 2.4 (REVISED): Swipe-space gesture instead of LanguageStripView

**Files:**
- Modify: `DraftRightMobile/ios/DraftRightKeyboard/QwertyKeyboardView.swift`
- Modify: `DraftRightMobile/ios/DraftRightKeyboard/KeyboardViewController.swift`

Add a `UIPanGestureRecognizer` to the space-bar `UIButton` in `QwertyKeyboardView.createKeyButton`. Threshold: 80 px horizontal Δ. On trigger, call `delegate?.keyboardDidSwipeSpace(direction: +1|-1)`.

In `KeyboardViewController`:
```swift
func keyboardDidSwipeSpace(direction: Int) {
    guard let c = controller, c.enabled.count > 1 else { return }
    c.cycleLanguage(reverse: direction < 0)
    textDocumentProxy.unmarkText()
    refreshKeyboardForActiveLanguage()
}
```

Space-bar label is the active `pack.displayName` — so user gets visual feedback after swipe (no separate strip needed).

### Task 2.4 (DEPRECATED original): `LanguageStripView`

**Files:**
- Create: `…/DraftRightKeyboard/LanguageStripView.swift`
- Modify: `…/DraftRightKeyboard/KeyboardViewController.swift`

```swift
final class LanguageStripView: UIScrollView {
    private let stack = UIStackView()
    private let controller: KeyboardController
    private let onLanguageChanged: () -> Void

    init(frame: CGRect, controller: KeyboardController, onLanguageChanged: @escaping () -> Void) {
        self.controller = controller
        self.onLanguageChanged = onLanguageChanged
        super.init(frame: frame)
        stack.axis = .horizontal
        stack.spacing = 8
        stack.distribution = .equalSpacing
        addSubview(stack)
        refresh()
    }

    func refresh() {
        stack.arrangedSubviews.forEach { $0.removeFromSuperview() }
        isHidden = controller.enabled.count <= 1
        for pack in controller.enabled {
            let isActive = pack.id == controller.current.id
            let chip = UIButton(type: .system)
            chip.setTitle(pack.displayName, for: .normal)
            chip.backgroundColor = isActive ? .systemBlue : .secondarySystemBackground
            chip.setTitleColor(isActive ? .white : .label, for: .normal)
            chip.layer.cornerRadius = 14
            chip.contentEdgeInsets = .init(top: 6, left: 12, bottom: 6, right: 12)
            chip.addAction(UIAction { [weak self] _ in
                self?.controller.setActive(id: pack.id)
                self?.refresh()
                self?.onLanguageChanged()
            }, for: .touchUpInside)
            stack.addArrangedSubview(chip)
        }
    }
}
```

In `KeyboardViewController.viewDidLoad()`, insert the strip view ABOVE the tone toolbar in the root vertical stack.

- [ ] Manual smoke test on simulator: chip strip appears (only when ≥2 languages enabled), tap doesn't crash.
- [ ] Commit.

---

## Stage 3 — TelexComposer + TelexState (long pole, port from Kotlin)

### Task 3.1: `TelexState.swift`

Direct port of `TelexState.kt`. Three tests (isVowel, isVowelLike, isToneMark). One-shot.

- [ ] Commit.

### Task 3.2: `TelexComposer.swift` — full port

**Files:**
- Create: `…/Sources/DraftRightKeyboardCore/Composer/TelexComposer.swift`
- Create: `…/Tests/.../Composer/TelexComposerTests.swift`

This is the riskiest stage. Approach: **port all 29 Kotlin tests verbatim FIRST**, then port the implementation. Each test that passes confirms one rule. If a port fails, fix the Swift implementation until it matches the Kotlin reference behavior — DO NOT change the test expectations.

Test cases to port:
- `aa → â`, `oo → ô`, `ee → ê`, `ow → ơ`, `uw → ư`, `aw → ă`, `dd → đ`
- `aaj → ậ`, `uow → ươ`, `uowj → ượ`
- `AA → Â` (case preservation)
- `q` direct-commit
- All five tone marks on `a`: `as=á af=à ar=ả ax=ã aj=ạ`
- Real-word: `vietj → việt`, `chuowng → chương`, `nguowif → người`, `tiesng → tiếng`
- Backspace: from việt → việ, empty → passThrough, from â → a, from ậ → â
- Multiple backspaces clear viet state
- Space mid-cluster commits
- Reset
- 32-char length cap

```swift
// TelexComposer.swift — Swift port mirroring Kotlin algorithm
public final class TelexComposer: Composer {
    private var buffer = ""
    public init() {}

    public func onKey(_ char: Character) -> ComposeResult {
        if buffer.count >= 32 {
            let committed = buffer
            buffer = String(char)
            return .commit(committed + String(char))
        }
        if !char.isLetter {
            if buffer.isEmpty { return .passThrough }
            let out = buffer + String(char)
            buffer = ""
            return .commit(out)
        }
        if let combined = TelexComposer.tryCombine(buffer, char) {
            buffer = combined
        } else {
            buffer.append(char)
        }
        return .composing(buffer)
    }

    public func onBackspace() -> ComposeResult {
        guard !buffer.isEmpty else { return .passThrough }
        buffer = TelexComposer.stripOneLayer(buffer)
        return buffer.isEmpty ? .consumed : .composing(buffer)
    }

    public func reset() { buffer = "" }
    public func currentComposingText() -> String { buffer }

    // tryCombine, applyTone, pickToneVowelIndex, findLastVowelCluster,
    // applyToneToChar, stripOneLayer, caseMap — all direct ports of the
    // Kotlin static methods. Same TONE_INDEX, TONE_ROWS_LOWER, UNTONE, UNMARK
    // dictionaries.
    // …
}
```

- [ ] Verify all 29 + 3 (state) = 32 tests pass before any other work.
- [ ] Single commit covering both the state + composer + tests.

---

## Stage 4 — `VietnameseLanguagePack` + composer integration

> **REVISED 2026-05-18 evening**: Tasks 4.2 + 4.3 below add the three composer-routing fixes that emerged during Android real-device testing (backspace empty-clear, space-mid-composition, Replace finishComposing).

### Task 4.3: Composer routing fixes (post-Android learnings)

When wiring `keyboardDidType` / `keyboardDidBackspace` / `keyboardDidSpace` / tone-rewrite Replace, mirror these specific behaviors:

1. **`keyboardDidBackspace`** when controller returns `.noChange` (composer just emptied buffer via stripOneLayer):
   ```swift
   case .noChange:
       textDocumentProxy.unmarkText()  // explicitly clear the marked region
   ```
   Without this, the IC's marked-text shows the last value and the user appears stuck on the first char.

2. **`keyboardDidSpace`** must route through the composer like any letter:
   ```swift
   func keyboardDidSpace() {
       guard let c = controller else {
           textDocumentProxy.insertText(" ")
           return
       }
       switch c.onKey(" ") {
       case .commit(let text):    textDocumentProxy.insertText(text)
       case .composing(let text): textDocumentProxy.setMarkedText(text, selectedRange: NSRange(location: text.count, length: 0))
       case .deleteOne:           textDocumentProxy.deleteBackward()
       case .noChange:            textDocumentProxy.insertText(" ")
       }
   }
   ```
   Direct `insertText(" ")` REPLACES the marked region, making the whole composing word disappear.

3. **Tone-rewrite Replace** (when user accepts a rewritten sentence) — call `unmarkText` first, then reset composer, then replace:
   ```swift
   func replaceAllText(_ newText: String) {
       textDocumentProxy.unmarkText()
       controller?.composer?.reset()
       // ... selectAll + insertText newText
   }
   ```

These three fixes correspond to Android commits 0a9818f2 (backspace+space) and 0020cb5d (Replace).

### Task 4.2 (original): Wire composer into IME keystroke path

### Task 4.1: `VietnameseLanguagePack.swift`

Mirror `VietnameseLanguagePack.kt`:

```swift
public struct VietnameseLanguagePack: LanguagePack {
    public let id = "vi"
    public let displayName = "Tiếng Việt"
    public let locale = Locale(identifier: "vi")
    public let alphaRows: [[KeyDef]]
    public let symbols1Rows: [[KeyDef]]
    public let symbols2Rows: [[KeyDef]]
    public let longPressAccents: [Character: [Character]] = [:]

    public init() {
        let en = EnglishLanguagePack()
        self.alphaRows = en.alphaRows
        self.symbols1Rows = en.symbols1Rows
        self.symbols2Rows = en.symbols2Rows
    }

    public func makeComposer() -> Composer? { TelexComposer() }
}
```

Three tests: id+displayName, makeComposer returns TelexComposer, alphaRows mirror English.

### Task 4.2: Wire composer into `QwertyKeyboardView` via `setMarkedText`

**Files:**
- Modify: `DraftRightMobile/ios/DraftRightKeyboard/QwertyKeyboardView.swift`
- Modify: `DraftRightMobile/ios/DraftRightKeyboard/KeyboardViewController.swift`

iOS uses `textDocumentProxy.setMarkedText(_:selectedRange:)` for composing text (mirror of Android's `setComposingText`). `unmarkText()` finalizes.

In `KeyboardViewController.keyboardDidType(_:)`:

```swift
func keyboardDidType(_ char: String) {
    guard char.count == 1 else {
        textDocumentProxy.insertText(char)
        return
    }
    let outcome = controller.onKey(Character(char))
    switch outcome {
    case .commit(let text):
        textDocumentProxy.unmarkText()
        textDocumentProxy.insertText(text)
    case .composing(let text):
        let range = NSRange(location: text.count, length: 0)
        textDocumentProxy.setMarkedText(text, selectedRange: range)
    case .deleteOne:
        textDocumentProxy.deleteBackward()
    case .noChange:
        break
    }
}

func keyboardDidBackspace() {
    let outcome = controller.onBackspace()
    switch outcome {
    case .commit(let text):
        textDocumentProxy.unmarkText()
        textDocumentProxy.insertText(text)
    case .composing(let text):
        textDocumentProxy.setMarkedText(text, selectedRange: NSRange(location: text.count, length: 0))
    case .deleteOne:
        textDocumentProxy.deleteBackward()
    case .noChange:
        break
    }
}
```

Manual smoke test on simulator: switch to VI via strip chip, type `v i e t j` → field shows `việt`.

- [ ] Commit.

---

## Stages 5–9 — Other Latin packs (FR, ES, DE, IT, PT)

Same 4-step pattern per stage:
1. Failing test for displayName, id, distinctive layout marker, accent map shape
2. Implement pack (rows + accent map)
3. Tests pass
4. Commit

Layout + accent specs are in spec §4 ("Constraints") and mirrored from `docs/superpowers/specs/2026-05-17-android-multilang-keyboard-design.md` Stages 5-9. Specifically:

- **Stage 5 — FrenchLanguagePack (AZERTY)**: top row `a z e r t y u i o p`, home row `q s d f g h j k l m`, accent map `'a': [à â ä á ã]`, `'e': [é è ê ë]`, `'i': [î ï í ì]`, `'o': [ô ö ó ò õ]`, `'u': [ù û ü ú]`, `'c': [ç]`, `'y': [ÿ]`
- **Stage 6 — GermanLanguagePack (QWERTZ)**: top row `q w e r t z u i o p ü`, home row `a s d f g h j k l ö ä`, third row `⇧ y x c v b n m ß ⌫`, accent map covers à á â ã / é è ê ë / í ì î ï / ó ò ô õ / ú ù û
- **Stage 7 — SpanishLanguagePack**: QWERTY + `ñ` at home-row end. Accent map for `'a' 'e' 'i' 'o' 'u'` + `'?': [¿]`, `'!': [¡]`
- **Stage 8 — ItalianLanguagePack**: QWERTY identical to EN. Accent map grave-first: `'a': [à á â ã ä]`, `'e': [è é ê ë]`, etc.
- **Stage 9 — PortugueseLanguagePack**: QWERTY + `ç` at home-row end. Accent map covers `'a': [á â ã à ä]`, `'e' 'i' 'o' 'u'`, `'c': [ç]`

After Task 9: register all 7 packs in `LanguageRegistry.production` (computed static, lazy):

```swift
public extension LanguageRegistry {
    static let production: LanguageRegistry = LanguageRegistry(packs: [
        EnglishLanguagePack(),
        VietnameseLanguagePack(),
        FrenchLanguagePack(),
        SpanishLanguagePack(),
        GermanLanguagePack(),
        ItalianLanguagePack(),
        PortugueseLanguagePack(),
    ])
}
```

`KeyboardViewController` uses `LanguageRegistry.production`. Latin packs are structs (value types) so the 7-pack `Array` allocation is cheap — well under the 50 MB ext memory ceiling.

---

## Stage 10 — AccentPopupView (long-press picker with pan-to-select)

### Task 10.1: `AccentPopupView.swift`

**Files:**
- Create: `…/DraftRightKeyboard/AccentPopupView.swift`
- Modify: `…/DraftRightKeyboard/QwertyKeyboardView.swift`

```swift
final class AccentPopupView: UIView {
    private let options: [Character]
    private let stack = UIStackView()
    private var highlightedIndex = 0
    private let onPicked: (Character) -> Void

    init(frame: CGRect, anchor: UIView, options: [Character], onPicked: @escaping (Character) -> Void) {
        self.options = options
        self.onPicked = onPicked
        super.init(frame: frame)
        backgroundColor = UIColor.black.withAlphaComponent(0.92)
        layer.cornerRadius = 8
        stack.axis = .horizontal
        stack.spacing = 4
        stack.alignment = .center
        addSubview(stack)
        for ch in options {
            let label = UILabel()
            label.text = String(ch)
            label.textColor = .white
            label.font = .systemFont(ofSize: 20)
            label.textAlignment = .center
            label.translatesAutoresizingMaskIntoConstraints = false
            label.widthAnchor.constraint(equalToConstant: 36).isActive = true
            label.heightAnchor.constraint(equalToConstant: 36).isActive = true
            stack.addArrangedSubview(label)
        }
        // Pan tracking via UIPanGestureRecognizer
        let pan = UIPanGestureRecognizer(target: self, action: #selector(handlePan(_:)))
        addGestureRecognizer(pan)
    }

    @objc private func handlePan(_ g: UIPanGestureRecognizer) {
        let cell = stack.bounds.width / CGFloat(options.count)
        let x = g.location(in: stack).x
        let idx = min(options.count - 1, max(0, Int(x / cell)))
        if idx != highlightedIndex {
            highlightedIndex = idx
            for (i, view) in stack.arrangedSubviews.enumerated() {
                view.backgroundColor = (i == idx) ? .systemBlue : .clear
            }
        }
        if g.state == .ended { onPicked(options[highlightedIndex]); removeFromSuperview() }
        if g.state == .cancelled { removeFromSuperview() }
    }
}
```

In `QwertyKeyboardView.swift`: track touch-down time on each char key. If still pressed after 400 ms AND the key has a non-empty `languagePack.longPressAccents[label[0]]`, instantiate `AccentPopupView` anchored above the key. Initial highlightedIndex = 0 (the base char). Cancel on touch-up before 400 ms.

- [ ] Manual smoke test on simulator: ES active, long-press `a` → popup with `a á à ä â ã`, drag finger, release → committed.
- [ ] Commit.

---

## Stage 11 — Flutter settings parity check (verify, do not re-implement)

Flutter UI already exists from Android Stage 10. Verify on iOS:

- [ ] Run `cd DraftRightMobile && flutter run -d "iPhone 15 simulator"`.
- [ ] Open Settings → "Keyboard languages" section is visible (same as on Android).
- [ ] Toggle a chip (e.g., Tiếng Việt) — confirm:
  - `UserDefaults(suiteName: "group.com.draftright.app.v2").stringArray(forKey: "flutter.draftright.enabledLanguageIds")` includes "vi"
  - Open WhatsApp / Notes / any text field → switch to DraftRight Keyboard → strip shows EN + Tiếng Việt
- [ ] If keys don't sync: check `Runner.entitlements` includes `com.apple.security.application-groups` with value `group.com.draftright.app.v2`, AND the iOS keyboard extension's `DraftRightKeyboard.entitlements` has the same value. If missing on either side, add and re-deploy.

If everything works, no code change for this stage. Document the verification in commit message.

- [ ] Commit.

---

## Stage 12 — Performance sweep + memory ceiling

### Task 12.1: Telex p95 + composer cycle perf

Port `TelexComposerPerfTest.kt` to Swift. Asserts:
- `onKey` p95 over 1000 keystrokes < 1 ms
- 32-char-buffer cycle median < 1 ms

Run via `swift test`. Same budgets as Android.

### Task 12.2: Memory check on iPhone SE 3rd gen

- [ ] Connect physical iPhone SE 3rd gen via cable
- [ ] Xcode → Product → Profile → Allocations instrument
- [ ] Reproduce: open Notes, switch to DraftRight Keyboard, type one sentence in each of 7 languages
- [ ] Check peak total bytes: must stay under 50 MB
- [ ] If over: investigate the language pack with largest `alphaRows` arrays (German has 11+11+10 keys = ~32 KeyDefs per layer × 3 layers ≈ 96 instances). Convert to a static `let` so the constant table is shared across pack instances.

- [ ] Commit perf test + any memory fixes.

---

## Stage 13 — Manual QA matrix (Stage gate before App Store submit)

Walk this matrix on three devices. Fill in pass/fail before merging to main.

| Device | iOS | Notes |
|---|---|---|
| iPhone 14 (real) | 17.x | Full Access flow + network rewrite |
| iPhone SE 3rd gen (real) | 16.x | Small screen — strip + tone toolbar both visible |
| iPhone 15 simulator (Xcode 26) | 17.x | Baseline, fastest iteration. Full Access path is unreliable in sim — DO NOT skip the real-device row for that. |

Per device:
1. Enable EN + Tiếng Việt in Flutter app Settings
2. Switch to DraftRight Keyboard in Messages app
3. Type `v i e t j` in chat → expect `việt` in message field
4. Type `t i e s n g` → expect `tiếng`
5. Tap "Tiếng Việt" chip → cycle to EN (or vice versa)
6. Type `hello` in EN → identical to today's behavior
7. Switch to ES, long-press `a`, drag to `á`, release → `á` committed
8. Repeat for FR (AZERTY layout check), DE (QWERTZ + ß key)
9. Force-quit DraftRight, relaunch keyboard → active language persists
10. Run a tone rewrite (Polished) on a non-English sentence → backend roundtrip clean

Sign-off table (fill in at completion):

| Device | Tester | Date | Result | Notes |
|---|---|---|---|---|
| iPhone 14 | | | | |
| iPhone SE 3rd gen | | | | |
| iPhone 15 simulator | | | | |

Apply `status: tested` label on the GitHub tracking issue once all three rows are green.

---

## Stage 14 — TestFlight + App Store submission

### Task 14.1: Bump pubspec to 2.4.0+41 (Android) and macOS to 2.4.0 if cross-publishing simultaneously

Optional version-parity step. Pure cosmetic.

### Task 14.2: TestFlight upload

```bash
cd DraftRightMobile
fastlane ios beta   # or bundle exec fastlane ios beta
```

This uploads to App Store Connect TestFlight internal testing. Wait for build processing (~10 min).

### Task 14.3: TestFlight smoke test by internal testers

- Add the 12 closed-test Android testers (or a smaller set) as TestFlight testers in App Store Connect
- They install DraftRight via TestFlight app on iPhone
- 7-day clock isn't a thing for TestFlight (unlike Play Closed Testing's 14-day rule) — proceed as soon as smoke tests pass

### Task 14.4: Submit for App Store review

```bash
fastlane ios release
```

Submits to App Store Connect for review. Apple typically takes 24-48 h.

**Watch for:**
- Guideline **4.0** / **4.3 (Spam)**: "Apps that compete with system services" — Apple sometimes pushes back on keyboards that replicate built-in functionality. Mitigation in spec §4. If rejected: respond to App Review with the use-case justification (Vietnamese support, tone rewrite — both functions Apple Intel does NOT provide).
- Guideline **5.5** (Privacy Policy): keyboards collect keystrokes by definition; our privacy policy must specifically call out that keystroke data is sent to `api.draftright.info` ONLY when the tone toolbar is tapped, not during typing. Update `https://draftright.info/privacy` before submission if needed.
- Build version code: Apple uses the `CFBundleVersion` integer — must be strictly higher than any previously uploaded build. Bump if necessary.

### Task 14.5: Monitor + release

- On approval: release manually via App Store Connect → "Release this version"
- On rejection: read the rejection details, fix, resubmit. App review rejection cycle is ~24 h per round.

---

## Stage 15 — Memory note rewrite

Once iOS Tier β ships to App Store, update memory:

- New file: `feedback_ios_full_access_telex_works.md` — confirms that Telex composition runs entirely OFFLINE (no Full Access needed for typing; only the rewrite call requires it). Important strategic note.
- Update [[project_android_keyboard_strategy]] → rename it or split into a cross-platform `project_keyboard_strategy.md`:
  - Android: multi-language IME first, share intent fallback
  - iOS: multi-language IME first (after Tier β ships), share extension fallback for CJK + Apple Intel competition
- Update [[reference_release_automation_parity]] with the iOS TestFlight + App Store one-command release path

---

## Estimated total

| Stage block | Days |
|---|---|
| Stages 0-2 (test cases + abstractions + controller) | 1 |
| Stage 3 (Telex port) | 1 |
| Stages 4-9 (VN + 5 Latin packs) | 1.5 |
| Stage 10 (accent popup with pan-to-select) | 1 |
| **Revision adjustments (swipe-space + composer fixes)** | already absorbed above |
| Stage 11 (settings parity, mostly verification) | 0.5 |
| Stage 12 (perf + memory sweep) | 0.5 |
| Stage 13 (manual QA matrix) | 1 |
| Stage 14 (TestFlight + App Store submit + review wait) | 2-3 (mostly Apple wait) |
| Stage 15 (memory rewrite) | 0.5 |
| **Total** | **8-10 working days** |

---

## Risk register

| Risk | Likelihood | Mitigation |
|---|---|---|
| App Store rejection under Guideline 4.0/4.3 | Medium | Frame in submission notes: "DraftRight Keyboard adds Vietnamese typing + AI tone rewrite — features Apple Intelligence does NOT cover." If rejected, appeal with the use-case data. |
| Memory limit blown by 7 packs | Low | Spec §9.4 budgets pre-empt this. Lazy-init only if Stage 12 measurements show > 50 MB. |
| Telex algorithm differs subtly from Kotlin → tests fail | Low | Port the Kotlin TelexComposerTest verbatim — same test names, same expected outputs. The implementation is what gets debugged, not the tests. |
| `setMarkedText` behavior differs between iOS versions | Low | Test on iOS 16 + iOS 17. Documented contract is stable since iOS 6. |
| Flutter App Group keys not syncing to iOS keyboard ext | Medium | Stage 11 explicitly verifies. If broken, check entitlements on both Runner + DraftRightKeyboard targets. |
| App Store review takes > 1 week | Low | Apple's median is 24-48 h. Budget 3 days for the queue + 1-2 rejection cycles. |

## Future work (out of scope for this plan)

- iOS share extension polish (separate effort, not blocking Tier β)
- VNI input method as second Vietnamese option
- CJK candidate-picker IME (Japanese / Chinese / Korean — major architectural lift)
- Apple-Intel-aware "smart suggestion" pass that defers to Writing Tools when Apple covers the language
- Cross-device sync of keyboard preferences via iCloud KVS
