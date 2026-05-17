# Android Multi-Language Keyboard (Tier β) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Vietnamese (Telex), French, Spanish, German, Italian, Portuguese to the DraftRight Android keyboard, keeping English byte-for-byte unchanged.

**Architecture:** `LanguagePack` interface (one Kotlin object per language) + optional `Composer` interface (`TelexComposer` only). `KeyboardController` coordinates. `QwertyKeyboardView` becomes layout-agnostic. New `LanguageStripView` + `AccentPopupView`. SharedPreferences gains `enabledLanguageIds` + `activeLanguageId`. No backend changes.

**Tech Stack:** Kotlin (Android IME), JUnit 4 (`./gradlew test`), Flutter (for settings UI only).

**Spec:** `docs/superpowers/specs/2026-05-17-android-multilang-keyboard-design.md`

---

## File structure

### New Kotlin files (10)

```
DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/
├── LanguagePack.kt              interface + KeyDef + helpers
├── LanguageRegistry.kt          lookup + cycle order
├── Composer.kt                  interface + ComposeResult sealed class
├── KeyboardController.kt        coordinator
├── LanguageStripView.kt         chips above tone toolbar
├── AccentPopupView.kt           long-press picker (300ms hold)
├── composer/
│   ├── TelexComposer.kt         VI state machine
│   └── TelexState.kt            pure data class
└── lang/
    ├── EnglishLanguagePack.kt
    ├── VietnameseLanguagePack.kt
    ├── FrenchLanguagePack.kt
    ├── SpanishLanguagePack.kt
    ├── GermanLanguagePack.kt
    ├── ItalianLanguagePack.kt
    └── PortugueseLanguagePack.kt
```

### Modified Kotlin files (3)

```
keyboard/DraftRightIME.kt         delegate keystroke routing to KeyboardController
keyboard/QwertyKeyboardView.kt    accept layout from LanguagePack; remove hardcoded rows
keyboard/SharedSettings.kt        add inputLanguages + activeLanguage keys
```

### New test files (10)

```
DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/
├── composer/TelexComposerTest.kt          ~25 tests
├── KeyboardControllerTest.kt
├── LanguageRegistryTest.kt
└── lang/
    ├── EnglishLanguagePackTest.kt
    ├── VietnameseLanguagePackTest.kt
    ├── FrenchLanguagePackTest.kt
    ├── SpanishLanguagePackTest.kt
    ├── GermanLanguagePackTest.kt
    ├── ItalianLanguagePackTest.kt
    └── PortugueseLanguagePackTest.kt
```

### Modified build files

```
DraftRightMobile/android/app/build.gradle.kts   add JUnit testImplementation
```

### Modified docs

```
docs/test-cases.xlsx                            new sheet KEYBOARD-MULTI (54 rows)
```

### Modified Flutter (Stage 10)

```
DraftRightMobile/lib/screens/settings_screen.dart   add "Keyboard languages" section
DraftRightMobile/lib/services/settings_service.dart add enabledLanguageIds + activeLanguageId getters/setters
```

---

## Stage 0 — Test cases in xlsx (CRITICAL, before any Kotlin)

### Task 0.1: Add KEYBOARD-MULTI sheet to docs/test-cases.xlsx

**Files:**
- Modify: `docs/test-cases.xlsx` (add sheet `KEYBOARD-MULTI`, 54 rows)

- [ ] **Step 1: Write the openpyxl script** at `scripts/seed-keyboard-multi-test-cases.py`:

```python
#!/usr/bin/env python3
"""Append KEYBOARD-MULTI test cases to docs/test-cases.xlsx — 54 rows."""
from openpyxl import load_workbook
from openpyxl.styles import Font, PatternFill, Alignment

wb = load_workbook('docs/test-cases.xlsx')
SHEET = 'KEYBOARD-MULTI'
if SHEET in wb.sheetnames:
    print(f'Sheet {SHEET} exists — aborting'); raise SystemExit(1)
ws = wb.create_sheet(SHEET)
ws.append(['TC-ID','Title','Preconditions','Steps','Expected','Verified by','Priority','Owner'])
for c in range(1, 9):
    cell = ws.cell(row=1, column=c)
    cell.font = Font(bold=True)
    cell.fill = PatternFill('solid', fgColor='DDDDDD')

rows = [
    # ── Telex composition — core rules (12) ─────────────────────────
    ('KEYBOARD-MULTI-001','Telex aa → â','VI active','Type a,a','Composer commits â','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-002','Telex oo → ô','VI active','Type o,o','Composer commits ô','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-003','Telex ee → ê','VI active','Type e,e','Composer commits ê','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-004','Telex ow → ơ','VI active','Type o,w','Composer commits ơ','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-005','Telex uw → ư','VI active','Type u,w','Composer commits ư','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-006','Telex aw → ă','VI active','Type a,w','Composer commits ă','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-007','Telex dd → đ','VI active','Type d,d','Composer commits đ','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-008','Telex aaj → ậ','VI active','Type a,a,j','Composer commits ậ (â + dot)','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-009','Telex uow → ươ','VI active','Type u,o,w','Composer commits ươ','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-010','Telex uowj → ượ','VI active','Type u,o,w,j','Composer commits ượ','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-011','Telex case preserved on shift','VI active, shift on','Type A,A','Composer commits Â','TelexComposerTest.kt','P1','Tan'),
    ('KEYBOARD-MULTI-012','Telex no-op for non-rule keys','VI active','Type q','Direct commit q (no composing)','TelexComposerTest.kt','P0','Tan'),

    # ── Telex composition — tones (6) ────────────────────────────────
    ('KEYBOARD-MULTI-013','Telex a+s → á','VI active','Type a,s','Composer commits á','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-014','Telex a+f → à','VI active','Type a,f','Composer commits à','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-015','Telex a+r → ả','VI active','Type a,r','Composer commits ả','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-016','Telex a+x → ã','VI active','Type a,x','Composer commits ã','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-017','Telex a+j → ạ','VI active','Type a,j','Composer commits ạ','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-018','Telex aaj sequence applies tone after circumflex','VI active','Type a,a,j','Composer commits ậ','TelexComposerTest.kt','P0','Tan'),

    # ── Telex composition — real words (4) ───────────────────────────
    ('KEYBOARD-MULTI-019','Type "việt"','VI active','Type v,i,e,t,j','Field shows việt','TelexComposerTest.kt + manual','P0','Tan'),
    ('KEYBOARD-MULTI-020','Type "chương"','VI active','Type c,h,u,o,w,n,g','Field shows chương','TelexComposerTest.kt + manual','P0','Tan'),
    ('KEYBOARD-MULTI-021','Type "người"','VI active','Type n,g,u,o,w,i,f','Field shows người','TelexComposerTest.kt + manual','P0','Tan'),
    ('KEYBOARD-MULTI-022','Type "tiếng"','VI active','Type t,i,e,s,n,g','Field shows tiếng','TelexComposerTest.kt + manual','P0','Tan'),

    # ── Telex backspace (5) ──────────────────────────────────────────
    ('KEYBOARD-MULTI-023','Backspace un-composes one step','VI active, "việt" composed','Press ⌫','Field shows việ (last char un-composed via composer)','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-024','Backspace at empty composer passes through','VI active, no composing state','Press ⌫','InputConnection.deleteSurroundingText called','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-025','Backspace removes mark before vowel','VI active, "â" composed','Press ⌫','Field shows a (â un-composed)','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-026','Backspace clears tone before mark','VI active, "ậ" composed','Press ⌫','Field shows â (tone removed)','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-027','Multiple backspaces clear composer state','VI active, "việt" composed','Press ⌫ four times','Composer state empty; further ⌫ deletes via IC','TelexComposerTest.kt','P0','Tan'),

    # ── Telex edge cases (4) ─────────────────────────────────────────
    ('KEYBOARD-MULTI-028','Non-letter mid-cluster commits pending','VI active, "vie" composing','Type space','Field shows "vie " (vowel cluster committed)','TelexComposerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-029','Language switch mid-cluster commits + resets','VI active, "vie" composing','Tap globe to switch to EN','Composing region committed as "vie"; composer.reset() called','KeyboardControllerTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-030','Paste mid-cluster clears composer','VI active, "vie" composing','System paste fires commitText','Composer onStartInput-like reset; pasted text intact','KeyboardControllerTest.kt + manual','P1','Tan'),
    ('KEYBOARD-MULTI-031','Composing length cap at 32 chars','VI active','Type 33 vowel keys','Composer commits at length 32; new key starts fresh','TelexComposerTest.kt','P2','Tan'),

    # ── Layout swap (7) ──────────────────────────────────────────────
    ('KEYBOARD-MULTI-032','EN baseline unchanged','EN active','Type "hello"','Identical keystroke output to today (regression)','EnglishLanguagePackTest.kt + manual','P0','Tan'),
    ('KEYBOARD-MULTI-033','AZERTY French layout','FR active','Tap QWERTY position of "a"','Letter "a" emitted (AZERTY puts a in QWERTY-q slot)','FrenchLanguagePackTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-034','Spanish ñ key','ES active','Tap ñ key','Field shows ñ','SpanishLanguagePackTest.kt + manual','P0','Tan'),
    ('KEYBOARD-MULTI-035','German QWERTZ layout','DE active','Tap QWERTY position of "z"','Letter "z" emitted (QWERTZ y/z swapped)','GermanLanguagePackTest.kt','P0','Tan'),
    ('KEYBOARD-MULTI-036','German ß key','DE active','Tap ß key','Field shows ß','GermanLanguagePackTest.kt + manual','P0','Tan'),
    ('KEYBOARD-MULTI-037','Portuguese ç key','PT active','Tap ç key','Field shows ç','PortugueseLanguagePackTest.kt + manual','P0','Tan'),
    ('KEYBOARD-MULTI-038','Language cycle order = VI > FR > ES > DE > IT > PT','All enabled','Tap globe 6 times','Cycles through VI, FR, ES, DE, IT, PT, back to start','KeyboardControllerTest.kt','P0','Tan'),

    # ── Globe key (3) ────────────────────────────────────────────────
    ('KEYBOARD-MULTI-039','Globe tap cycles enabled languages','VI + EN enabled','Tap globe','activeLanguageId flips between VI and EN','KeyboardControllerTest.kt + manual','P0','Tan'),
    ('KEYBOARD-MULTI-040','Globe long-press opens IME picker','VI active','Long-press globe 500ms','InputMethodManager.showInputMethodPicker() called','KeyboardControllerTest.kt + manual','P0','Tan'),
    ('KEYBOARD-MULTI-041','Single enabled language: globe tap is no-op','Only EN enabled','Tap globe','activeLanguageId unchanged; strip not shown','KeyboardControllerTest.kt + manual','P1','Tan'),

    # ── Long-press accent picker (5) ─────────────────────────────────
    ('KEYBOARD-MULTI-042','Long-press a in ES shows á à ä â ã','ES active','Hold a key 300ms','AccentPopupView appears above key with 5 options + a','AccentPopupViewTest + manual','P0','Tan'),
    ('KEYBOARD-MULTI-043','Long-press ñ in ES: no popup','ES active','Hold ñ key','No popup (ñ not in accents map); short-press commits ñ','SpanishLanguagePackTest.kt + manual','P1','Tan'),
    ('KEYBOARD-MULTI-044','Long-press a in FR shows é è ê variants for e key','FR active','Hold e key','Popup with é è ê ë','FrenchLanguagePackTest.kt + manual','P0','Tan'),
    ('KEYBOARD-MULTI-045','Finger up without drag = short-press','ES active','Hold a, release before drag','Plain a committed; popup dismisses','AccentPopupViewTest + manual','P1','Tan'),
    ('KEYBOARD-MULTI-046','Drag selects + release commits','ES active','Hold a, drag right, release on á','Field shows á','manual','P0','Tan'),

    # ── Space-bar label (1) ──────────────────────────────────────────
    ('KEYBOARD-MULTI-047','Space bar shows current language displayName','VI active','Look at space bar','Reads "Tiếng Việt" (or current.displayName)','manual on emulator','P0','Tan'),

    # ── Settings persistence (3) ─────────────────────────────────────
    ('KEYBOARD-MULTI-048','activeLanguageId survives Force-Stop','VI active','Force-stop app; relaunch IME','VI still active','KeyboardControllerTest.kt + manual','P0','Tan'),
    ('KEYBOARD-MULTI-049','enabledLanguageIds reorder persists','VI > EN order saved','Reorder to EN > VI in Settings; relaunch','Globe cycles EN > VI in new order','manual on emulator','P1','Tan'),
    ('KEYBOARD-MULTI-050','Disabling all languages forces EN','User disables all','Open keyboard','EN forced; activeLanguageId="en"; toast/log warning','KeyboardControllerTest.kt + manual','P1','Tan'),

    # ── Backward compatibility (4) ───────────────────────────────────
    ('KEYBOARD-MULTI-051','EN-only users see no behavior change','Fresh install, no setting changes','Type "hello world"','Output identical to pre-Tier-β','EnglishLanguagePackTest.kt + manual','P0','Tan'),
    ('KEYBOARD-MULTI-052','Existing tone toolbar still functions','VI active','Type text, tap "Polished"','Tone rewrite flow unchanged','manual + integration','P0','Tan'),
    ('KEYBOARD-MULTI-053','Share intent unchanged','EN active','Long-press text → Share → DraftRight','Share rewrite flow unchanged','manual','P0','Tan'),
    ('KEYBOARD-MULTI-054','/rewrite payload includes only text + tone (no inputLanguage)','VI active','Tap Polished','Backend request body has text + tone only','BackendClient inspection','P0','Tan'),
]
for r in rows:
    ws.append(r)
    for c in range(1, len(r) + 1):
        cell = ws.cell(row=ws.max_row, column=c)
        cell.alignment = Alignment(wrap_text=True, vertical='top')
widths = {'A':22,'B':52,'C':28,'D':50,'E':52,'F':32,'G':10,'H':10}
for col, w in widths.items():
    ws.column_dimensions[col].width = w
for r in range(2, ws.max_row + 1):
    ws.row_dimensions[r].height = 50
wb.save('docs/test-cases.xlsx')
print(f'Added {SHEET} with {len(rows)} rows.')
```

- [ ] **Step 2: Run the script**

```bash
cd /opt/openAi/DraftRight && python3 scripts/seed-keyboard-multi-test-cases.py
```
Expected output: `Added KEYBOARD-MULTI with 54 rows.`

- [ ] **Step 3: Branch + commit**

```bash
git checkout develop
git pull
git checkout -b feature/android-multilang-keyboard-20260517
git add scripts/seed-keyboard-multi-test-cases.py docs/test-cases.xlsx
git commit -m "test(keyboard): add KEYBOARD-MULTI-001..054 (before-code mandate)"
```

If the branch already exists from spec commit, skip the `-b` flag and just `git checkout`.

---

## Stage 1 — Core abstractions + test infrastructure

### Task 1.1: Add JUnit test dependencies

**Files:**
- Modify: `DraftRightMobile/android/app/build.gradle.kts`

- [ ] **Step 1: Append to the `dependencies` block**

Find the existing `dependencies { ... }` block at the end and add:

```kotlin
dependencies {
    // ... existing deps ...
    testImplementation("junit:junit:4.13.2")
    testImplementation("org.mockito:mockito-core:5.7.0")
    testImplementation("org.mockito.kotlin:mockito-kotlin:5.2.1")
}
```

- [ ] **Step 2: Create test source dir**

```bash
mkdir -p DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard
mkdir -p DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/composer
mkdir -p DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/lang
```

- [ ] **Step 3: Verify `./gradlew test` runs (no tests yet, empty pass)**

```bash
cd DraftRightMobile/android && ./gradlew test 2>&1 | tail -5
```
Expected: `BUILD SUCCESSFUL` with no tests found.

- [ ] **Step 4: Commit**

```bash
git add DraftRightMobile/android/app/build.gradle.kts
git commit -m "build(android): add JUnit + Mockito for keyboard unit tests"
```

### Task 1.2: `KeyDef` + `LanguagePack` interface

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/LanguagePack.kt`
- Create: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/LanguagePackTest.kt`

- [ ] **Step 1: Write failing test**

```kotlin
// LanguagePackTest.kt
package com.draftright.keyboard

import org.junit.Test
import org.junit.Assert.*
import java.util.Locale

class LanguagePackTest {
    private val stub = object : LanguagePack {
        override val id = "stub"
        override val displayName = "Stub"
        override val locale: Locale = Locale.ENGLISH
        override val alphaRows = listOf(
            listOf(KeyDef("a", 'a'.code), KeyDef("b", 'b'.code))
        )
        override val symbols1Rows = emptyList<List<KeyDef>>()
        override val symbols2Rows = emptyList<List<KeyDef>>()
        override val longPressAccents: Map<Char, List<Char>> = emptyMap()
    }

    @Test
    fun `id and displayName are exposed`() {
        assertEquals("stub", stub.id)
        assertEquals("Stub", stub.displayName)
    }

    @Test
    fun `composer factory defaults to null`() {
        assertNull(stub.composer())
    }

    @Test
    fun `KeyDef carries label code and width weight`() {
        val k = KeyDef("ñ", 'ñ'.code, widthWeight = 1.5f)
        assertEquals("ñ", k.label)
        assertEquals('ñ'.code, k.code)
        assertEquals(1.5f, k.widthWeight, 0f)
    }
}
```

- [ ] **Step 2: Run, expect compile failure**

```bash
cd DraftRightMobile/android && ./gradlew :app:testDebugUnitTest --tests "com.draftright.keyboard.LanguagePackTest" 2>&1 | tail -10
```
Expected: compile error — `LanguagePack` / `KeyDef` not found.

- [ ] **Step 3: Implement**

```kotlin
// LanguagePack.kt
package com.draftright.keyboard

import java.util.Locale

/**
 * Data for a single physical or visual key on the keyboard.
 * - label: what the user sees on the key (e.g. "ñ", "⇧", "space")
 * - code: integer keycode dispatched to the input connection. Standard
 *   ASCII letters use their ASCII value; special keys use negative codes
 *   defined in QwertyKeyboardView.Codes.
 * - widthWeight: relative width in the row (1.0 = standard letter key,
 *   1.5 = shift/backspace, 5.0 = space bar).
 */
data class KeyDef(
    val label: String,
    val code: Int,
    val widthWeight: Float = 1.0f,
)

/**
 * Definition of a single typing language available in the DraftRight
 * keyboard. Each concrete language is a Kotlin object in keyboard/lang/.
 *
 * Composer is a factory (() -> Composer?) so each language session gets a
 * fresh state machine. Returning null means "no composition needed — emit
 * keystrokes directly" (the case for English and all Latin languages).
 */
interface LanguagePack {
    val id: String
    val displayName: String
    val locale: Locale
    val alphaRows: List<List<KeyDef>>
    val symbols1Rows: List<List<KeyDef>>
    val symbols2Rows: List<List<KeyDef>>
    val longPressAccents: Map<Char, List<Char>>

    /** Build a fresh composer for this language session. Null = no compose. */
    fun composer(): Composer? = null
}
```

Note: `Composer` is referenced but not yet defined. Either add a stub now, or add `Composer.kt` in Task 1.3 first. Order: Task 1.3 → Task 1.2 if you want one-shot compile. For TDD, declare a forward reference and let it fail; the test in 1.3 will create `Composer.kt` and unblock.

- [ ] **Step 4: Create `Composer.kt` stub (interface only, no impls)**

```kotlin
// composer/Composer.kt (note: lives in package com.draftright.keyboard for Task 1.3 location)
// Actually create at: keyboard/Composer.kt
package com.draftright.keyboard

/** Output of a composer's onKey / onBackspace call. */
sealed class ComposeResult {
    object PassThrough : ComposeResult()
    data class Commit(val text: String) : ComposeResult()
    data class Composing(val text: String) : ComposeResult()
    object Consumed : ComposeResult()
}

/** Per-language composition engine. Only TelexComposer implements in Phase 1. */
interface Composer {
    fun onKey(char: Char): ComposeResult
    fun onBackspace(): ComposeResult
    fun reset()
    fun currentComposingText(): String
}
```

- [ ] **Step 5: Run tests, expect PASS**

```bash
cd DraftRightMobile/android && ./gradlew :app:testDebugUnitTest --tests "com.draftright.keyboard.LanguagePackTest" 2>&1 | tail -10
```
Expected: 3 tests pass.

- [ ] **Step 6: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/LanguagePack.kt \
        DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/Composer.kt \
        DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/LanguagePackTest.kt
git commit -m "feat(keyboard): LanguagePack interface + KeyDef + Composer scaffolding"
```

### Task 1.3: `LanguageRegistry`

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/LanguageRegistry.kt`
- Create: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/LanguageRegistryTest.kt`

- [ ] **Step 1: Failing test**

```kotlin
// LanguageRegistryTest.kt
package com.draftright.keyboard

import org.junit.Test
import org.junit.Assert.*

class LanguageRegistryTest {

    private fun makeStub(idVal: String) = object : LanguagePack {
        override val id = idVal
        override val displayName = idVal.uppercase()
        override val locale = java.util.Locale.ENGLISH
        override val alphaRows = emptyList<List<KeyDef>>()
        override val symbols1Rows = emptyList<List<KeyDef>>()
        override val symbols2Rows = emptyList<List<KeyDef>>()
        override val longPressAccents = emptyMap<Char, List<Char>>()
    }

    @Test
    fun `byId returns the matching pack`() {
        val packs = listOf(makeStub("en"), makeStub("vi"))
        val reg = LanguageRegistry(packs)
        assertEquals("vi", reg.byId("vi").id)
    }

    @Test
    fun `byId throws on unknown id`() {
        val reg = LanguageRegistry(listOf(makeStub("en")))
        assertThrows(NoSuchElementException::class.java) { reg.byId("fr") }
    }

    @Test
    fun `byIdOrDefault falls back to first when id unknown`() {
        val packs = listOf(makeStub("en"), makeStub("vi"))
        val reg = LanguageRegistry(packs)
        assertEquals("en", reg.byIdOrDefault("zz").id)
    }

    @Test
    fun `next cycles in order with wrap-around`() {
        val packs = listOf(makeStub("vi"), makeStub("fr"), makeStub("es"))
        val reg = LanguageRegistry(packs)
        assertEquals("fr", reg.next("vi").id)
        assertEquals("es", reg.next("fr").id)
        assertEquals("vi", reg.next("es").id) // wrap
    }

    @Test
    fun `next on unknown id returns first`() {
        val reg = LanguageRegistry(listOf(makeStub("en"), makeStub("vi")))
        assertEquals("en", reg.next("xx").id)
    }
}
```

- [ ] **Step 2: Run, expect compile FAIL**

```bash
cd DraftRightMobile/android && ./gradlew :app:testDebugUnitTest --tests "com.draftright.keyboard.LanguageRegistryTest" 2>&1 | tail -10
```

- [ ] **Step 3: Implement**

```kotlin
// LanguageRegistry.kt
package com.draftright.keyboard

/**
 * Holds an ordered list of LanguagePacks. The order doubles as the cycle
 * order when the user taps the globe key — passing the active id to
 * next() returns the pack that should become active.
 *
 * Constructor takes a List<LanguagePack> so tests can inject stubs.
 * Production code constructs `LanguageRegistry.PRODUCTION` (defined in
 * Task 1.4) which lists every concrete language pack.
 */
class LanguageRegistry(private val packs: List<LanguagePack>) {

    init {
        require(packs.isNotEmpty()) { "LanguageRegistry needs at least one LanguagePack" }
    }

    fun byId(id: String): LanguagePack =
        packs.firstOrNull { it.id == id }
            ?: throw NoSuchElementException("Unknown language id: $id")

    /** Forgiving lookup — returns first pack (English in production) when unknown. */
    fun byIdOrDefault(id: String): LanguagePack =
        packs.firstOrNull { it.id == id } ?: packs.first()

    /** The next language in cycle order, wrapping around the end. */
    fun next(currentId: String): LanguagePack {
        val idx = packs.indexOfFirst { it.id == currentId }
        if (idx < 0) return packs.first()
        return packs[(idx + 1) % packs.size]
    }

    /** Filter to just the enabled subset, preserving registry order. */
    fun enabled(enabledIds: List<String>): List<LanguagePack> =
        packs.filter { it.id in enabledIds }
}
```

- [ ] **Step 4: Run, expect PASS**

5 tests pass.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/LanguageRegistry.kt \
        DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/LanguageRegistryTest.kt
git commit -m "feat(keyboard): LanguageRegistry with byId, next, enabled filter"
```

### Task 1.4: `EnglishLanguagePack` (port existing hardcoded layout)

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/lang/EnglishLanguagePack.kt`
- Create: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/lang/EnglishLanguagePackTest.kt`

- [ ] **Step 1: Failing test**

```kotlin
// EnglishLanguagePackTest.kt
package com.draftright.keyboard.lang

import com.draftright.keyboard.KeyDef
import org.junit.Test
import org.junit.Assert.*

class EnglishLanguagePackTest {
    @Test
    fun `id is en and displayName is English`() {
        assertEquals("en", EnglishLanguagePack.id)
        assertEquals("English", EnglishLanguagePack.displayName)
    }

    @Test
    fun `alphaRows has three rows of letters + a bottom function row`() {
        assertEquals(4, EnglishLanguagePack.alphaRows.size)
    }

    @Test
    fun `top row starts with q and ends with p`() {
        val top = EnglishLanguagePack.alphaRows[0]
        assertEquals("q", top.first().label)
        assertEquals("p", top.last().label)
    }

    @Test
    fun `home row starts with a and ends with l`() {
        val home = EnglishLanguagePack.alphaRows[1]
        assertEquals("a", home.first().label)
        assertEquals("l", home.last().label)
    }

    @Test
    fun `composer factory returns null`() {
        assertNull(EnglishLanguagePack.composer())
    }

    @Test
    fun `long press accents is empty`() {
        assertTrue(EnglishLanguagePack.longPressAccents.isEmpty())
    }
}
```

- [ ] **Step 2: Run, expect compile FAIL**

- [ ] **Step 3: Implement — copy today's rows from `QwertyKeyboardView`**

Read existing rows in `QwertyKeyboardView.kt` lines 64–152, then port verbatim into:

```kotlin
// lang/EnglishLanguagePack.kt
package com.draftright.keyboard.lang

import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.LanguagePack
import java.util.Locale

object EnglishLanguagePack : LanguagePack {
    override val id = "en"
    override val displayName = "English"
    override val locale: Locale = Locale.ENGLISH

    override val alphaRows: List<List<KeyDef>> = listOf(
        // Row 1: q w e r t y u i o p
        listOf("q","w","e","r","t","y","u","i","o","p").map { KeyDef(it, it[0].code) },
        // Row 2: a s d f g h j k l
        listOf("a","s","d","f","g","h","j","k","l").map { KeyDef(it, it[0].code) },
        // Row 3: shift z x c v b n m backspace
        listOf(
            KeyDef("⇧", -1, 1.5f),
        ) + listOf("z","x","c","v","b","n","m").map { KeyDef(it, it[0].code) } + listOf(
            KeyDef("⌫", -5, 1.5f),
        ),
        // Row 4: 123 globe space . enter
        listOf(
            KeyDef("123", -2, 1.5f),
            KeyDef("🌐", -3, 1.0f),
            KeyDef("space · English", ' '.code, 5.0f),
            KeyDef(".", '.'.code),
            KeyDef("⏎", -4, 1.5f),
        )
    )

    override val symbols1Rows: List<List<KeyDef>> = listOf(
        listOf("1","2","3","4","5","6","7","8","9","0").map { KeyDef(it, it[0].code) },
        listOf("@","#","$","_","&","-","+","(",")","/").map { KeyDef(it, it[0].code) },
        listOf(KeyDef("=\\<", -6, 1.5f)) +
            listOf("*","\"","'",":",";","!","?").map { KeyDef(it, it[0].code) } +
            listOf(KeyDef("⌫", -5, 1.5f)),
        listOf(
            KeyDef("ABC", -7, 1.5f),
            KeyDef("🌐", -3, 1.0f),
            KeyDef("space · English", ' '.code, 5.0f),
            KeyDef(".", '.'.code),
            KeyDef("⏎", -4, 1.5f),
        ),
    )

    override val symbols2Rows: List<List<KeyDef>> = listOf(
        listOf("~","`","|","•","√","π","÷","×","§","¶").map { KeyDef(it, it[0].code) },
        listOf("£","¥","€","°","^","_","=","{","}","\\").map { KeyDef(it, it[0].code) },
        listOf(KeyDef("123", -6, 1.5f)) +
            listOf("[","]","<",">","%","©","®").map { KeyDef(it, it[0].code) } +
            listOf(KeyDef("⌫", -5, 1.5f)),
        listOf(
            KeyDef("ABC", -7, 1.5f),
            KeyDef("🌐", -3, 1.0f),
            KeyDef("space · English", ' '.code, 5.0f),
            KeyDef(".", '.'.code),
            KeyDef("⏎", -4, 1.5f),
        ),
    )

    override val longPressAccents: Map<Char, List<Char>> = emptyMap()
}
```

Note: confirm the special-key codes (-1 through -7) match the existing constants in `QwertyKeyboardView`. If those constants are named, import them rather than hardcoding numbers.

- [ ] **Step 4: Run, expect PASS** (6 tests)

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/lang/EnglishLanguagePack.kt \
        DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/lang/EnglishLanguagePackTest.kt
git commit -m "feat(keyboard): EnglishLanguagePack — ports today's hardcoded QWERTY"
```

---

## Stage 2 — Globe key + LanguageStripView (English-only first)

This stage wires up the cycle infrastructure without adding any new languages — proves the plumbing works on an EN-only setup.

### Task 2.1: SharedSettings keys

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/SharedSettings.kt`

- [ ] **Step 1: Read existing file**

```bash
cat DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/SharedSettings.kt
```

- [ ] **Step 2: Add two getters near the existing `translateLanguage` field**

```kotlin
// SharedSettings.kt — add at bottom of class
val enabledLanguageIds: List<String>
    get() = prefs.getString("flutter.draftright.enabledLanguageIds", "[\"en\"]")
        ?.removePrefix("[")?.removeSuffix("]")
        ?.split(",")
        ?.map { it.trim().removeSurrounding("\"") }
        ?.filter { it.isNotEmpty() }
        ?: listOf("en")

val activeLanguageId: String
    get() = prefs.getString("flutter.draftright.activeLanguageId", "en") ?: "en"
```

Flutter prefixes everything with `flutter.` when writing through `shared_preferences`, so the keys mirror what Stage 10 will write.

- [ ] **Step 3: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/SharedSettings.kt
git commit -m "feat(keyboard): SharedSettings reads enabledLanguageIds + activeLanguageId"
```

### Task 2.2: `KeyboardController` (cycle logic only — no composer yet)

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/KeyboardController.kt`
- Create: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/KeyboardControllerTest.kt`

- [ ] **Step 1: Failing test**

```kotlin
// KeyboardControllerTest.kt
package com.draftright.keyboard

import com.draftright.keyboard.lang.EnglishLanguagePack
import org.junit.Test
import org.junit.Assert.*

class KeyboardControllerTest {

    private fun stub(id: String) = object : LanguagePack {
        override val id = id
        override val displayName = id.uppercase()
        override val locale = java.util.Locale.ENGLISH
        override val alphaRows = listOf(listOf(KeyDef("a", 'a'.code)))
        override val symbols1Rows = emptyList<List<KeyDef>>()
        override val symbols2Rows = emptyList<List<KeyDef>>()
        override val longPressAccents = emptyMap<Char, List<Char>>()
    }

    @Test
    fun `init defaults to first enabled when activeId is empty`() {
        val reg = LanguageRegistry(listOf(EnglishLanguagePack))
        val ctrl = KeyboardController(reg, enabledIds = listOf("en"), activeId = "")
        assertEquals("en", ctrl.current.id)
    }

    @Test
    fun `cycle wraps within enabled subset, ignoring disabled`() {
        val packs = listOf(stub("vi"), stub("fr"), stub("es"))
        val reg = LanguageRegistry(packs)
        val ctrl = KeyboardController(reg, enabledIds = listOf("vi", "es"), activeId = "vi")
        assertEquals("vi", ctrl.current.id)
        ctrl.cycleLanguage()
        assertEquals("es", ctrl.current.id)
        ctrl.cycleLanguage()
        assertEquals("vi", ctrl.current.id)
    }

    @Test
    fun `cycle is no-op when only one enabled`() {
        val reg = LanguageRegistry(listOf(EnglishLanguagePack))
        val ctrl = KeyboardController(reg, enabledIds = listOf("en"), activeId = "en")
        ctrl.cycleLanguage()
        assertEquals("en", ctrl.current.id)
    }

    @Test
    fun `disabled all force-enables registry first`() {
        val reg = LanguageRegistry(listOf(EnglishLanguagePack))
        val ctrl = KeyboardController(reg, enabledIds = emptyList(), activeId = "")
        assertEquals("en", ctrl.current.id)
        assertEquals(listOf("en"), ctrl.enabled.map { it.id })
    }
}
```

- [ ] **Step 2: Run, expect FAIL**

- [ ] **Step 3: Implement**

```kotlin
// KeyboardController.kt
package com.draftright.keyboard

/**
 * Coordinator for everything multi-language: holds the active language,
 * the enabled subset, and a composer instance (only non-null for VI).
 *
 * Construction takes the registry + the user's persisted preferences
 * (read via SharedSettings on the IME side) so it stays unit-testable
 * without an Android context.
 */
class KeyboardController(
    private val registry: LanguageRegistry,
    enabledIds: List<String>,
    activeId: String,
) {
    /** Languages the user has turned on, in registry order. */
    var enabled: List<LanguagePack> = registry.enabled(enabledIds)
        .ifEmpty { listOf(registry.byIdOrDefault("en")) }
        private set

    /** The currently selected language. */
    var current: LanguagePack = enabled.firstOrNull { it.id == activeId } ?: enabled.first()
        private set

    /** Active composer for the current language, rebuilt on switch. */
    var composer: Composer? = current.composer()
        private set

    /** Cycle to the next enabled language; no-op when only one enabled. */
    fun cycleLanguage() {
        if (enabled.size <= 1) return
        val idx = enabled.indexOfFirst { it.id == current.id }
        current = enabled[(idx + 1) % enabled.size]
        composer?.reset()
        composer = current.composer()
    }

    /** Set the active language by id (used by LanguageStripView chip tap). */
    fun setActive(id: String) {
        val target = enabled.firstOrNull { it.id == id } ?: return
        if (target.id == current.id) return
        composer?.reset()
        current = target
        composer = current.composer()
    }
}
```

- [ ] **Step 4: Run, expect PASS** (4 tests)

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/KeyboardController.kt \
        DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/KeyboardControllerTest.kt
git commit -m "feat(keyboard): KeyboardController — cycle, setActive, composer lifecycle"
```

### Task 2.3: Wire `DraftRightIME` + `QwertyKeyboardView` to use controller (EN-only, no behavior change)

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt`
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/QwertyKeyboardView.kt`

- [ ] **Step 1: Read current `DraftRightIME.kt` + `QwertyKeyboardView.kt`**

Identify the spots where `alphaRows` / `symbols1Rows` / `symbols2Rows` are referenced and where globe key (`-3`) is handled.

- [ ] **Step 2: Replace hardcoded rows in `QwertyKeyboardView` with `currentLayout` lookup**

In `QwertyKeyboardView`:
- Remove the hardcoded `alphaRows`, `symbols1Rows`, `symbols2Rows` private vals (lines ~64–152 in recon).
- Add a `var languagePack: LanguagePack = EnglishLanguagePack` field set by the IME.
- Where the previous hardcoded vals were used, substitute `languagePack.alphaRows` / `.symbols1Rows` / `.symbols2Rows`.

In `DraftRightIME`:
- Create a `KeyboardController` in `onCreate()` using `SharedSettings.enabledLanguageIds` + `activeLanguageId`.
- Pass `controller.current` to `QwertyKeyboardView.languagePack` after view inflation.
- On globe key tap (code `-3`), call `controller.cycleLanguage()` and refresh the view.

- [ ] **Step 3: Build + run on Pixel emulator, manually type "hello" — must be unchanged**

```bash
cd DraftRightMobile && flutter run -d emulator-5554
```

Type letters into any text field. Output must be identical to today.

- [ ] **Step 4: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt \
        DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/QwertyKeyboardView.kt
git commit -m "refactor(keyboard): wire QwertyKeyboardView + DraftRightIME to KeyboardController"
```

### Task 2.4: `LanguageStripView` (chips above tone toolbar)

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/LanguageStripView.kt`
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt`

- [ ] **Step 1: Implement view**

```kotlin
// LanguageStripView.kt
package com.draftright.keyboard

import android.content.Context
import android.graphics.Color
import android.view.View
import android.widget.LinearLayout
import android.widget.TextView
import androidx.core.view.setPadding

/**
 * Horizontal row of language chips. One chip per enabled LanguagePack.
 * The active chip is highlighted blue. Tap cycles via the controller's
 * setActive(); long-press is reserved for "remove from enabled" (deferred).
 *
 * Rendered directly above the tone toolbar. Hidden entirely when only one
 * language is enabled (no point in cycling).
 */
class LanguageStripView(
    context: Context,
    private val controller: KeyboardController,
    private val onLanguageChanged: () -> Unit,
) : LinearLayout(context) {

    init {
        orientation = HORIZONTAL
        setPadding(8, 6, 8, 6)
        refresh()
    }

    fun refresh() {
        removeAllViews()
        visibility = if (controller.enabled.size <= 1) View.GONE else View.VISIBLE
        if (visibility == View.GONE) return

        controller.enabled.forEach { pack ->
            val chip = TextView(context).apply {
                text = pack.displayName
                textSize = 12f
                setPadding(20, 10, 20, 10)
                val isActive = pack.id == controller.current.id
                setTextColor(if (isActive) Color.WHITE else Color.parseColor("#cbd5e1"))
                setBackgroundColor(
                    if (isActive) Color.parseColor("#3b82f6") else Color.parseColor("#2a2a2e")
                )
                setOnClickListener {
                    controller.setActive(pack.id)
                    refresh()
                    onLanguageChanged()
                }
            }
            val params = LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT)
            params.marginEnd = 8
            addView(chip, params)
        }
    }
}
```

- [ ] **Step 2: Add to `DraftRightIME.onCreateInputView()`**

Place the new `LanguageStripView` ABOVE the existing `ToolbarView` in the parent `LinearLayout`:

```kotlin
val root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
val strip = LanguageStripView(this, controller) {
    qwertyKeyboardView.languagePack = controller.current
    qwertyKeyboardView.invalidate()
}
root.addView(strip)
root.addView(toolbarView)
root.addView(qwertyKeyboardView)
return root
```

- [ ] **Step 3: Manual smoke test**

Enable two languages in Flutter Settings (Stage 10 — not built yet; for now hand-edit SharedPreferences via adb):

```bash
adb shell run-as com.draftright.draftright_mobile.v2 \
  sh -c 'echo "<map><list name=\"flutter.draftright.enabledLanguageIds\"><string>en</string><string>en</string></list></map>" > shared_prefs/flutter.draftright_mobile.v2_preferences.xml'
```

(Or push a real two-language config once Stage 10 lands.)

Confirm: chip strip appears above tones; both chips visible; tap doesn't crash.

- [ ] **Step 4: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/LanguageStripView.kt \
        DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt
git commit -m "feat(keyboard): LanguageStripView — chips above tone toolbar"
```

---

## Stage 3 — TelexComposer + TelexState (long pole, ~25 unit tests, TDD strict)

This is the riskiest stage. Tests come first; implementation only after each test fails. Compose rules are listed in spec §7.3.

### Task 3.1: `TelexState` data class + pure helpers

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/composer/TelexState.kt`
- Create: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/composer/TelexStateTest.kt`

- [ ] **Step 1: Failing test for vowel-cluster detection helper**

```kotlin
// TelexStateTest.kt
package com.draftright.keyboard.composer

import org.junit.Test
import org.junit.Assert.*

class TelexStateTest {

    @Test
    fun `isVowel detects a e i o u and uppercase`() {
        listOf('a','e','i','o','u','A','E','I','O','U').forEach {
            assertTrue("expected $it vowel", TelexState.isVowel(it))
        }
        listOf('b','q','z').forEach {
            assertFalse(TelexState.isVowel(it))
        }
    }

    @Test
    fun `isToneMark detects s f r x j`() {
        listOf('s','f','r','x','j').forEach {
            assertTrue(TelexState.isToneMark(it))
        }
        assertFalse(TelexState.isToneMark('a'))
    }
}
```

- [ ] **Step 2: Run, expect FAIL**

- [ ] **Step 3: Implement**

```kotlin
// TelexState.kt
package com.draftright.keyboard.composer

/**
 * Pure data + helper functions for the Telex composition state machine.
 * Has no Android dependencies so it runs fast in `./gradlew test`.
 *
 * Composition lifecycle:
 *   1. User types vowel(s) — accumulate in `vowelCluster`.
 *   2. User types a Telex modifier (s/f/r/x/j or repeat-vowel like aa) —
 *      apply to `vowelCluster`, update composed output.
 *   3. User types a non-vowel non-modifier — commit composed text, start
 *      fresh with the new keypress as `consonantTail`.
 *   4. User types another vowel after consonantTail — accumulate as a
 *      new word; the previous composed prefix is treated as committed.
 *   5. Backspace un-composes one step (tone → mark → bare vowel → empty).
 */
data class TelexState(
    /** Composed text so far (already-finalized portion of current word). */
    var composed: String = "",
    /** Vowel cluster currently being modified (e.g. "ie" while typing "viet"). */
    var vowelCluster: String = "",
    /** Trailing consonant(s) typed after a vowel cluster. */
    var consonantTail: String = "",
) {
    companion object {
        private val VOWELS = setOf('a','e','i','o','u','A','E','I','O','U')
        private val TONE_MARKS = setOf('s','f','r','x','j')

        fun isVowel(c: Char): Boolean = c in VOWELS
        fun isToneMark(c: Char): Boolean = c in TONE_MARKS
    }

    fun isEmpty(): Boolean = composed.isEmpty() && vowelCluster.isEmpty() && consonantTail.isEmpty()
    fun displayText(): String = composed + vowelCluster + consonantTail
    fun clear() {
        composed = ""
        vowelCluster = ""
        consonantTail = ""
    }
}
```

- [ ] **Step 4: Run, expect PASS** (2 tests)

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/composer/TelexState.kt \
        DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/composer/TelexStateTest.kt
git commit -m "feat(telex): TelexState + isVowel / isToneMark helpers"
```

### Task 3.2 .. 3.N: `TelexComposer` rules — one TDD cycle per test case

Each test case (KEYBOARD-MULTI-001 through 031, that's 31 rules + edges) is its own red-green-refactor cycle. To keep this plan tractable, the **first three cycles are written out fully** as a template. The remaining 28 follow the same pattern: write failing test that exercises the rule → run → implement transition → run → commit.

The full Telex composer ends up roughly like this:

```kotlin
// composer/TelexComposer.kt
package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import com.draftright.keyboard.Composer

class TelexComposer : Composer {
    private val state = TelexState()
    private val history = mutableListOf<TelexState>()  // for backspace undo

    override fun onKey(char: Char): ComposeResult {
        snapshot()
        val low = char.lowercaseChar()
        return when {
            isCircumflexCombo(low) -> applyCircumflex(char)
            isHornCombo(low)       -> applyHorn(char)
            isBreveCombo(low)      -> applyBreve(char)
            isDoubleDCombo(char)   -> applyDoubleD(char)
            TelexState.isToneMark(low) && state.vowelCluster.isNotEmpty()
                                   -> applyTone(low, isUppercase(char))
            TelexState.isVowel(char) -> appendVowel(char)
            char.isLetter()        -> appendConsonant(char)
            else                   -> commitAndPassThrough(char)
        }
    }

    override fun onBackspace(): ComposeResult {
        if (state.isEmpty()) return ComposeResult.PassThrough
        if (history.isEmpty()) {
            state.clear()
            return ComposeResult.Consumed
        }
        val prev = history.removeAt(history.lastIndex)
        state.composed = prev.composed
        state.vowelCluster = prev.vowelCluster
        state.consonantTail = prev.consonantTail
        return ComposeResult.Composing(state.displayText())
    }

    override fun reset() {
        state.clear()
        history.clear()
    }

    override fun currentComposingText(): String = state.displayText()

    // ... private helpers …
}
```

For each TC-ID from the xlsx, write a focused JUnit `@Test` first, then implement the smallest transition needed. After each test goes green, commit. **Do not batch.** Frequent commits make bisecting trivial when a later test breaks an earlier one.

#### Task 3.2: `aa → â` (KEYBOARD-MULTI-001)

- [ ] **Step 1: Failing test in `TelexComposerTest.kt`**

```kotlin
package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import org.junit.Test
import org.junit.Assert.*

class TelexComposerTest {
    @Test
    fun `aa composes â`() {
        val c = TelexComposer()
        c.onKey('a')
        val r = c.onKey('a')
        assertTrue(r is ComposeResult.Composing)
        assertEquals("â", (r as ComposeResult.Composing).text)
    }
}
```

- [ ] **Step 2: Run, expect FAIL** (class missing).

- [ ] **Step 3: Implement minimal `TelexComposer`** that handles only `aa → â` and stub everything else as `PassThrough`. Concrete implementation is the snippet above, trimmed.

- [ ] **Step 4: Run, expect PASS**.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/composer/TelexComposer.kt \
        DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/composer/TelexComposerTest.kt
git commit -m "feat(telex): aa → â (KEYBOARD-MULTI-001)"
```

#### Task 3.3 .. 3.31: Remaining 28 rules

Repeat the same 5-step cycle for each of these rules. Each cycle is one commit:

| Task | TC-ID | Test name | Behavior |
|---|---|---|---|
| 3.3 | 002 | `oo composes ô` | `o`,`o` → `ô` |
| 3.4 | 003 | `ee composes ê` | `e`,`e` → `ê` |
| 3.5 | 004 | `ow composes ơ` | `o`,`w` → `ơ` |
| 3.6 | 005 | `uw composes ư` | `u`,`w` → `ư` |
| 3.7 | 006 | `aw composes ă` | `a`,`w` → `ă` |
| 3.8 | 007 | `dd composes đ` | `d`,`d` → `đ` (note: `d` is consonant, special-case) |
| 3.9 | 008 | `aaj composes ậ` | mark applied then tone |
| 3.10 | 009 | `uow composes ươ` | two-letter horn cluster |
| 3.11 | 010 | `uowj composes ượ` | horn cluster + tone |
| 3.12 | 011 | `AA composes Â` | preserve case |
| 3.13 | 012 | `q is direct-commit` | non-rule letter passes through |
| 3.14 | 013 | `as composes á` | acute tone |
| 3.15 | 014 | `af composes à` | grave tone |
| 3.16 | 015 | `ar composes ả` | hook-above |
| 3.17 | 016 | `ax composes ã` | tilde |
| 3.18 | 017 | `aj composes ạ` | dot-below |
| 3.19 | 018 | `aaj` revisited as separate path test | edge: tone after circumflex |
| 3.20 | 019 | `viet+j` real word | Commit final "việt" |
| 3.21 | 020 | `chuong+w` → chương | real word |
| 3.22 | 021 | `nguoi+w+f` → người | real word |
| 3.23 | 022 | `tieng+s` → tiếng | real word |
| 3.24 | 023 | Backspace from `việt` → `việ` | composer un-composes |
| 3.25 | 024 | Backspace empty → PassThrough | confirms IC delete path |
| 3.26 | 025 | Backspace `â` → `a` | un-compose circumflex |
| 3.27 | 026 | Backspace `ậ` → `â` | tone removed first |
| 3.28 | 027 | Four backspaces clear state | history stack drains |
| 3.29 | 028 | Space mid-cluster commits | non-Telex key commits pending |
| 3.30 | 029 | Language switch resets composer | tested via KeyboardControllerTest in Task 2.2 — add a `reset()` assertion here |
| 3.31 | 031 | 33-char cluster cap | composer commits at 32, restarts |

Each task = 5 steps, ends with one commit. Result: ~30 commits over 1.5–2 days of focused TDD.

**Important:** if a later test forces refactoring of an earlier rule, do it — keep all earlier tests green. That's the whole point of writing tests first.

After Task 3.31, run the full composer suite:

```bash
cd DraftRightMobile/android && ./gradlew :app:testDebugUnitTest --tests "com.draftright.keyboard.composer.*" 2>&1 | tail -10
```

Expected: 25+ tests pass.

---

## Stage 4 — `VietnameseLanguagePack` + integration

### Task 4.1: VietnameseLanguagePack

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/lang/VietnameseLanguagePack.kt`
- Create: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/lang/VietnameseLanguagePackTest.kt`

- [ ] **Step 1: Failing test**

```kotlin
// VietnameseLanguagePackTest.kt
package com.draftright.keyboard.lang

import com.draftright.keyboard.composer.TelexComposer
import org.junit.Test
import org.junit.Assert.*

class VietnameseLanguagePackTest {
    @Test
    fun `id and displayName`() {
        assertEquals("vi", VietnameseLanguagePack.id)
        assertEquals("Tiếng Việt", VietnameseLanguagePack.displayName)
    }

    @Test
    fun `composer factory returns a TelexComposer`() {
        assertTrue(VietnameseLanguagePack.composer() is TelexComposer)
    }

    @Test
    fun `space bar label is native name`() {
        val bottomRow = VietnameseLanguagePack.alphaRows.last()
        val space = bottomRow.first { it.code == ' '.code }
        assertTrue(space.label.contains("Tiếng Việt"))
    }

    @Test
    fun `alphaRows mirror EnglishLanguagePack shape`() {
        // VI shares QWERTY — same 4 rows, same key counts.
        assertEquals(4, VietnameseLanguagePack.alphaRows.size)
        assertEquals(EnglishLanguagePack.alphaRows[0].size, VietnameseLanguagePack.alphaRows[0].size)
    }
}
```

- [ ] **Step 2: FAIL**

- [ ] **Step 3: Implement** by copying EN layout and just replacing the space-bar label + composer:

```kotlin
// lang/VietnameseLanguagePack.kt
package com.draftright.keyboard.lang

import com.draftright.keyboard.Composer
import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.LanguagePack
import com.draftright.keyboard.composer.TelexComposer
import java.util.Locale

object VietnameseLanguagePack : LanguagePack {
    override val id = "vi"
    override val displayName = "Tiếng Việt"
    override val locale: Locale = Locale("vi")

    override val alphaRows: List<List<KeyDef>> = EnglishLanguagePack.alphaRows.mapIndexed { idx, row ->
        if (idx == 3) row.map {
            if (it.code == ' '.code) KeyDef("space · Tiếng Việt", ' '.code, 5.0f) else it
        } else row
    }

    override val symbols1Rows = EnglishLanguagePack.symbols1Rows
    override val symbols2Rows = EnglishLanguagePack.symbols2Rows
    override val longPressAccents: Map<Char, List<Char>> = emptyMap()  // Telex covers it
    override fun composer(): Composer = TelexComposer()
}
```

- [ ] **Step 4: Run, expect PASS** (4 tests)

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/lang/VietnameseLanguagePack.kt \
        DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/lang/VietnameseLanguagePackTest.kt
git commit -m "feat(keyboard): VietnameseLanguagePack — QWERTY + TelexComposer factory"
```

### Task 4.2: Wire composer into IME keystroke path

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/QwertyKeyboardView.kt`
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt`
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/KeyboardController.kt`

- [ ] **Step 1: Add `onKey(char: Char)` and `onBackspace()` to `KeyboardController`**

```kotlin
// KeyboardController.kt — append
fun onKey(char: Char): ComposerOutcome {
    val composer = this.composer ?: return ComposerOutcome.Commit(char.toString())
    return when (val r = composer.onKey(char)) {
        is ComposeResult.Commit       -> ComposerOutcome.Commit(r.text)
        is ComposeResult.Composing    -> ComposerOutcome.Composing(r.text)
        is ComposeResult.PassThrough  -> ComposerOutcome.Commit(char.toString())
        is ComposeResult.Consumed     -> ComposerOutcome.NoChange  // unreachable on onKey
    }
}

fun onBackspace(): ComposerOutcome {
    val composer = this.composer ?: return ComposerOutcome.DeleteOne
    return when (val r = composer.onBackspace()) {
        is ComposeResult.Composing    -> ComposerOutcome.Composing(r.text)
        is ComposeResult.Consumed     -> ComposerOutcome.NoChange
        is ComposeResult.PassThrough  -> ComposerOutcome.DeleteOne
        is ComposeResult.Commit       -> ComposerOutcome.Commit(r.text)  // unreachable
    }
}

sealed class ComposerOutcome {
    data class Commit(val text: String)       : ComposerOutcome()
    data class Composing(val text: String)    : ComposerOutcome()
    object DeleteOne                          : ComposerOutcome()
    object NoChange                           : ComposerOutcome()
}
```

- [ ] **Step 2: In `QwertyKeyboardView`, route handleKeyPress through `controller.onKey`** instead of calling `inputConnection.commitText` directly.

```kotlin
// Inside handleKeyPress — pseudo-diff:
- inputConnection?.commitText(text, 1)
+ when (val outcome = controller.onKey(char)) {
+     is KeyboardController.ComposerOutcome.Commit ->
+         inputConnection?.commitText(outcome.text, 1)
+     is KeyboardController.ComposerOutcome.Composing ->
+         inputConnection?.setComposingText(outcome.text, 1)
+     is KeyboardController.ComposerOutcome.DeleteOne ->
+         inputConnection?.deleteSurroundingText(1, 0)
+     KeyboardController.ComposerOutcome.NoChange -> { /* no-op */ }
+ }
```

Similarly route backspace.

- [ ] **Step 3: Manual smoke test on emulator**

Switch to VI in stripview. Type `v i e t j` → expect `việt`.

- [ ] **Step 4: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/KeyboardController.kt \
        DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/QwertyKeyboardView.kt \
        DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt
git commit -m "feat(keyboard): route keystrokes through composer when one is active"
```

---

## Stage 5–9 — Other Latin languages (FR, ES, DE, IT, PT)

These five stages all follow the **same 4-step pattern**:

1. Failing test for `displayName`, `id`, key-position invariants, accent map shape.
2. Implement the language pack object (varies per language — layouts + accents).
3. Tests pass.
4. Commit.

### Task 5.1: FrenchLanguagePack (AZERTY)

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/lang/FrenchLanguagePack.kt`
- Create: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/lang/FrenchLanguagePackTest.kt`

AZERTY differs from QWERTY in the top + home rows. The exact AZERTY layout:

```
Row 0: a z e r t y u i o p
Row 1: q s d f g h j k l m
Row 2: ⇧ w x c v b n ⌫
Row 3: 123 🌐 [space · Français] . ⏎
```

- [ ] **Step 1: Failing test** asserts top-row first key is `a` (vs `q` in EN).

- [ ] **Step 2: Implement** by writing the 4 rows out explicitly. Accent map:

```kotlin
override val longPressAccents = mapOf(
    'a' to listOf('à','â','ä','á','ã'),
    'e' to listOf('é','è','ê','ë'),
    'i' to listOf('î','ï','í','ì'),
    'o' to listOf('ô','ö','ó','ò','õ'),
    'u' to listOf('ù','û','ü','ú'),
    'c' to listOf('ç'),
)
```

- [ ] **Step 3 + 4 + 5: Run / pass / commit** as in earlier tasks.

### Task 6.1: SpanishLanguagePack (QWERTY + ñ + accents)

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/lang/SpanishLanguagePack.kt`
- Create: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/lang/SpanishLanguagePackTest.kt`

Same QWERTY as EN but with a dedicated `ñ` key (typically at the right edge of the home row). Spec test:

```kotlin
@Test
fun `home row ends with ñ`() {
    val home = SpanishLanguagePack.alphaRows[1]
    assertEquals("ñ", home.last().label)
}
```

Accent map:

```kotlin
override val longPressAccents = mapOf(
    'a' to listOf('á','à','ä','â','ã'),
    'e' to listOf('é','è','ê','ë'),
    'i' to listOf('í','ì','î','ï'),
    'o' to listOf('ó','ò','ô','ö','õ'),
    'u' to listOf('ú','ù','û','ü'),
    '?' to listOf('¿'),
    '!' to listOf('¡'),
)
```

### Task 7.1: GermanLanguagePack (QWERTZ + ä ö ü ß)

QWERTZ swaps `y` and `z` in the top row. The ß and umlauts get dedicated keys:

```
Row 0: q w e r t z u i o p ü
Row 1: a s d f g h j k l ö ä
Row 2: ⇧ y x c v b n m ß ⌫
Row 3: 123 🌐 [space · Deutsch] . ⏎
```

Accent map:

```kotlin
override val longPressAccents = mapOf(
    'a' to listOf('à','á','â','ã'),
    'e' to listOf('é','è','ê','ë'),
    'i' to listOf('í','ì','î','ï'),
    'o' to listOf('ó','ò','ô','õ'),
    'u' to listOf('ú','ù','û'),
)
```

### Task 8.1: ItalianLanguagePack (QWERTY + accents)

QWERTY identical to EN. Accents map:

```kotlin
override val longPressAccents = mapOf(
    'a' to listOf('à','á','â','ã','ä'),
    'e' to listOf('è','é','ê','ë'),
    'i' to listOf('ì','í','î','ï'),
    'o' to listOf('ò','ó','ô','õ','ö'),
    'u' to listOf('ù','ú','û','ü'),
)
```

### Task 9.1: PortugueseLanguagePack (QWERTY + ç + accents)

QWERTY + dedicated `ç` key (typically next to `l` in home row).

Accent map:

```kotlin
override val longPressAccents = mapOf(
    'a' to listOf('á','â','ã','à','ä'),
    'e' to listOf('é','ê','ë'),
    'i' to listOf('í','î','ï'),
    'o' to listOf('ó','ô','õ','ö'),
    'u' to listOf('ú','û','ü'),
    'c' to listOf('ç'),
)
```

After Tasks 5.1 through 9.1, register all six packs in `LanguageRegistry.PRODUCTION`:

```kotlin
// LanguageRegistry.kt — append at bottom
companion object {
    val PRODUCTION = LanguageRegistry(listOf(
        EnglishLanguagePack,
        VietnameseLanguagePack,
        FrenchLanguagePack,
        SpanishLanguagePack,
        GermanLanguagePack,
        ItalianLanguagePack,
        PortugueseLanguagePack,
    ))
}
```

And update `DraftRightIME.onCreate()` to use `LanguageRegistry.PRODUCTION`.

---

## Stage 10 — AccentPopupView (long-press picker)

### Task 10.1: AccentPopupView

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/AccentPopupView.kt`
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/QwertyKeyboardView.kt`

- [ ] **Step 1: Implement view**

```kotlin
// AccentPopupView.kt
package com.draftright.keyboard

import android.content.Context
import android.graphics.Color
import android.view.MotionEvent
import android.view.View
import android.widget.LinearLayout
import android.widget.PopupWindow
import android.widget.TextView
import androidx.core.view.setPadding

class AccentPopupView(
    private val context: Context,
    private val anchor: View,
    private val options: List<Char>,
    private val onPicked: (Char) -> Unit,
) {
    private val popup: PopupWindow
    private val container: LinearLayout
    private var hoveredIndex = -1

    init {
        container = LinearLayout(context).apply {
            orientation = LinearLayout.HORIZONTAL
            setBackgroundColor(Color.WHITE)
            setPadding(8, 8, 8, 8)
        }
        options.forEachIndexed { idx, ch ->
            val tv = TextView(context).apply {
                text = ch.toString()
                textSize = 18f
                setPadding(20, 12, 20, 12)
                setTextColor(Color.parseColor("#1d1d1f"))
            }
            container.addView(tv)
        }
        popup = PopupWindow(container, LinearLayout.LayoutParams.WRAP_CONTENT, LinearLayout.LayoutParams.WRAP_CONTENT).apply {
            isTouchable = true
            isFocusable = false
        }
    }

    fun show() {
        val loc = IntArray(2)
        anchor.getLocationOnScreen(loc)
        popup.showAtLocation(anchor, 0, loc[0], loc[1] - container.measuredHeight - 8)
    }

    /** Called by parent on MotionEvent.ACTION_MOVE to update highlight. */
    fun onTouchMove(rawX: Float) {
        val cellWidth = container.measuredWidth / options.size
        val newIndex = ((rawX - container.left) / cellWidth).toInt().coerceIn(0, options.size - 1)
        if (newIndex == hoveredIndex) return
        hoveredIndex = newIndex
        container.children().forEachIndexed { idx, child ->
            child.setBackgroundColor(
                if (idx == hoveredIndex) Color.parseColor("#3b82f6") else Color.TRANSPARENT
            )
            (child as TextView).setTextColor(
                if (idx == hoveredIndex) Color.WHITE else Color.parseColor("#1d1d1f")
            )
        }
    }

    fun commit() {
        if (hoveredIndex in options.indices) onPicked(options[hoveredIndex])
        dismiss()
    }

    fun dismiss() {
        popup.dismiss()
    }

    private fun LinearLayout.children(): List<View> = (0 until childCount).map { getChildAt(it) }
}
```

- [ ] **Step 2: In `QwertyKeyboardView`, add long-press detection** (300ms) on letter keys:

```kotlin
// QwertyKeyboardView — pseudo-diff
private var longPressJob: Runnable? = null

override fun onTouchEvent(event: MotionEvent): Boolean {
    when (event.action) {
        MotionEvent.ACTION_DOWN -> {
            val keyDef = findKeyAt(event.x, event.y) ?: return true
            val char = keyDef.code.toChar()
            val accents = controller.current.longPressAccents[char]
            if (accents != null) {
                longPressJob = Runnable { showAccentPopup(keyView, listOf(char) + accents) }
                postDelayed(longPressJob, 300)
            }
        }
        MotionEvent.ACTION_MOVE -> accentPopup?.onTouchMove(event.rawX)
        MotionEvent.ACTION_UP -> {
            removeCallbacks(longPressJob)
            if (accentPopup != null) {
                accentPopup?.commit()
                accentPopup = null
                return true
            }
            // ... existing short-press handling
        }
    }
    return true
}
```

- [ ] **Step 3: Manual test on emulator**: long-press `a` in Spanish → popup with `á à ä â ã`.

- [ ] **Step 4: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/AccentPopupView.kt \
        DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/QwertyKeyboardView.kt
git commit -m "feat(keyboard): AccentPopupView — 300ms long-press picker above held key"
```

---

## Stage 11 — Settings UI (Flutter side)

### Task 11.1: SettingsService — getters/setters for new keys

**Files:**
- Modify: `DraftRightMobile/lib/services/settings_service.dart`

- [ ] **Step 1: Add fields**

```dart
// settings_service.dart — append
List<String> _enabledLanguageIds = const ['en'];
String _activeLanguageId = 'en';

List<String> get enabledLanguageIds => _enabledLanguageIds;
String get activeLanguageId => _activeLanguageId;

Future<void> setEnabledLanguageIds(List<String> ids) async {
  _enabledLanguageIds = ids.isEmpty ? const ['en'] : ids;
  await _prefs.setStringList('draftright.enabledLanguageIds', _enabledLanguageIds);
  notifyListeners();
}

Future<void> setActiveLanguageId(String id) async {
  _activeLanguageId = id;
  await _prefs.setString('draftright.activeLanguageId', id);
  notifyListeners();
}
```

In `load()`, restore both.

- [ ] **Step 2: Commit**

```bash
git add DraftRightMobile/lib/services/settings_service.dart
git commit -m "feat(settings): enabledLanguageIds + activeLanguageId persistence"
```

### Task 11.2: Settings screen UI — keyboard languages section

**Files:**
- Modify: `DraftRightMobile/lib/screens/settings_screen.dart`

- [ ] **Step 1: Add a "Keyboard languages" Card** below the existing translate-language dropdown.

```dart
Card(
  child: Column(
    children: [
      const ListTile(
        leading: Icon(Icons.keyboard_outlined),
        title: Text('Keyboard languages'),
        subtitle: Text('Tap globe key (🌐) to cycle within DraftRight. Long-press for system IME picker.'),
      ),
      ...[
        ('en', 'English'),
        ('vi', 'Tiếng Việt'),
        ('fr', 'Français'),
        ('es', 'Español'),
        ('de', 'Deutsch'),
        ('it', 'Italiano'),
        ('pt', 'Português'),
      ].map((entry) {
        final id = entry.$1, name = entry.$2;
        return SwitchListTile(
          title: Text(name),
          value: settings.enabledLanguageIds.contains(id),
          onChanged: (on) {
            final next = on
              ? [...settings.enabledLanguageIds, id]
              : settings.enabledLanguageIds.where((x) => x != id).toList();
            settings.setEnabledLanguageIds(next);
          },
        );
      }),
    ],
  ),
),
```

- [ ] **Step 2: Run on emulator**, toggle a few languages, force-stop app, reopen — verify persistence.

- [ ] **Step 3: Commit**

```bash
git add DraftRightMobile/lib/screens/settings_screen.dart
git commit -m "feat(settings): Keyboard languages section in Settings screen"
```

---

## Stage 12 — Performance sweep

### Task 12.1: Latency assertions in TelexComposer test

**Files:**
- Modify: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/composer/TelexComposerTest.kt`

- [ ] **Step 1: Append a perf test**

```kotlin
@Test
fun `onKey completes in under 1ms`() {
    val c = TelexComposer()
    val start = System.nanoTime()
    repeat(1000) { c.onKey('a') }
    val elapsedMs = (System.nanoTime() - start) / 1_000_000.0
    assertTrue("avg per key = ${elapsedMs / 1000} ms", elapsedMs < 1000)
}
```

- [ ] **Step 2: Run, expect PASS**

- [ ] **Step 3: Commit**

```bash
git add DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/composer/TelexComposerTest.kt
git commit -m "test(telex): perf assertion — 1000 keystrokes in < 1 s"
```

### Task 12.2: Memory footprint check (manual on device)

- [ ] **Step 1: Launch app on Samsung A52, type for 2 minutes mixing all languages.**

- [ ] **Step 2: Capture meminfo**:

```bash
adb shell dumpsys meminfo com.draftright.draftright_mobile.v2 | head -40
```

- [ ] **Step 3: Compare RSS delta vs the pre-Tier-β baseline.** Budget: < 2 MB added.

No commit needed unless the check fails — if it does, audit which LanguagePack object is heaviest and lazy-load.

---

## Stage 13 — Manual QA matrix

### Task 13.1: Walk through KEYBOARD-MULTI-001..054 on three devices

- [ ] **Step 1: Build release APK**

```bash
cd DraftRightMobile && flutter build apk --release
```

- [ ] **Step 2: Sideload to Samsung A52, Xiaomi (any Android 12+), Pixel emulator**

```bash
adb -s <serial> install build/app/outputs/flutter-apk/app-release.apk
```

- [ ] **Step 3: For each device + each test case from xlsx, mark Pass/Fail in the Verified-by column**

- [ ] **Step 4: Any failures → file a bug + halt before merging**

No code commits in this stage; xlsx updates only.

---

## Stage 14 — Onboarding update

### Task 14.1: First-launch language picker (Flutter)

**Files:**
- Modify: `DraftRightMobile/lib/screens/onboarding_screen.dart` (or similar — check existing)
- New (if needed): `DraftRightMobile/lib/screens/keyboard_language_picker_screen.dart`

- [ ] **Step 1: After existing onboarding pages, add a step that shows the same list as Stage 11.2 SwitchListTile** with friendly copy: "Pick the languages you want to type in. You can change this anytime in Settings."

- [ ] **Step 2: Default selection** = device locale + English. So a Vietnamese user gets `vi + en` pre-checked.

- [ ] **Step 3: Commit**

```bash
git add DraftRightMobile/lib/screens/onboarding_screen.dart
git commit -m "feat(onboarding): keyboard language picker on first launch"
```

---

## Stage 15 — Merge + deploy

### Task 15.1: Final test run + analyze

```bash
cd DraftRightMobile/android && ./gradlew test
cd DraftRightMobile && flutter analyze
```
All green required before merge.

### Task 15.2: Merge feature → develop

```bash
git checkout develop
git pull
git merge --no-ff feature/android-multilang-keyboard-20260517 -m "Merge feature/android-multilang-keyboard-20260517: Tier β multi-language Android keyboard (VI/FR/ES/DE/IT/PT)"
git push origin develop
```

### Task 15.3: Deploy to testing track (Google Play closed-testing)

- [ ] **Step 1: Bump Flutter pubspec.yaml version + Android versionCode**

- [ ] **Step 2: `flutter build appbundle --release`**

- [ ] **Step 3: Upload .aab to Play Console closed-testing track**

- [ ] **Step 4: Apply `status: deployed to testing` label on the tracking GH issue + comment**

### Task 15.4: QA verifies on closed-testing → merge develop → main

```bash
git checkout main
git pull
git merge --no-ff develop -m "Merge develop → main: Tier β multi-language Android keyboard"
git push origin main
```

### Task 15.5: Deploy to Google Play production track

- [ ] **Step 1: Promote the closed-testing release to production in Play Console.**

- [ ] **Step 2: After Google approves (1-7 days), apply `status: deployed to production` label + add `## ✅ How to Verify` comment to the tracking issue.**

---

## Stage 16 — Memory note rewrite

### Task 16.1: Update `project_android_keyboard_strategy.md`

**Files:**
- Modify: `/Users/tannguyen/.claude/projects/-opt-openAi-DraftRight/memory/project_android_keyboard_strategy.md`

- [ ] **Step 1: Replace the existing memory note** with new positioning:

```markdown
---
name: Android keyboard strategy (Tier β shipped)
description: DraftRight is now a daily multi-language keyboard, not just a rewrite tool. Type natively in VI/FR/ES/DE/IT/PT; share intent + floating bubble remain as fallback.
type: project
---

## Current state (post 2026-Q2 Tier β ship)

DraftRight Android IME is the user's primary multi-language keyboard. Supports English (QWERTY), Vietnamese (QWERTY + Telex), French (AZERTY), Spanish (QWERTY + ñ + accents), German (QWERTZ + ä ö ü ß), Italian (QWERTY + accents), Portuguese (QWERTY + ç + accents).

Globe key 🌐: tap cycles within DraftRight languages, long-press opens system IME picker.

## Entry flows for rewrite

1. **Primary:** type in DraftRight → tap tone button on the toolbar.
2. **Backup 1:** share intent (long-press text in any chat → Share → DraftRight). For users who keep Gboard or another IME as default.
3. **Backup 2:** floating bubble (Android 2.2.1+23) — type anywhere, tap bubble. For users who don't want to switch keyboards.

## Onboarding pitch

First-launch screen: "Type in your language, rewrite with AI — one keyboard does both."

## Open follow-ups

- VNI input method (Vietnamese alternative to Telex) — Phase 2, demand-driven.
- Predictive text + autocorrect — Phase 2.
- CJK languages (Japanese, Chinese, Korean) — Phase 3, separate spec (~3 months/language even AI-assisted).
- Per-app language memory — Phase 3.
```

- [ ] **Step 2: Commit memory rewrite** — memory files aren't in the repo git, but document the change in a session-summary commit on the repo:

```bash
git add docs/superpowers/plans/2026-05-17-android-multilang-keyboard.md   # the plan file itself, retroactive note
echo "Memory note project_android_keyboard_strategy.md rewritten post-Tier-β" \
  >> docs/superpowers/plans/2026-05-17-android-multilang-keyboard.md
git commit -am "docs(memory): post-ship memory note rewrite acknowledged in plan"
```

(Memory files live outside the repo, so this is informational only.)

---

## Self-Review Checklist

Run before marking the plan complete:

- [ ] Spec §3 goals — every goal has at least one task. (Phase 1 implements all six languages + Telex + globe + long-press picker + space-bar label + persistence + no backend changes.)
- [ ] Spec §4 user flows — all five branches (EN baseline, VI Telex, language cycle, long-press accent, no-entities-no-rewrite) have tasks.
- [ ] Spec §5 decisions — all six locked decisions show up in task code: priority order in `LanguageRegistry.PRODUCTION`, globe behavior in `DraftRightIME` + `KeyboardController`, Telex-only in `TelexComposer`, no autocorrect (nothing built for it), long-press popup A in `AccentPopupView`, native-name space-bar in each `*LanguagePack`.
- [ ] Spec §6 components — every named class/interface (`LanguagePack`, `Composer`, `TelexComposer`, `TelexState`, `LanguageRegistry`, `KeyboardController`, `LanguageStripView`, `AccentPopupView`, all 7 language packs) has a dedicated task.
- [ ] Spec §7 contracts — type signatures match between tasks (verified: `Composer.onKey(Char): ComposeResult`, `KeyboardController.cycleLanguage(): Unit`, `LanguagePack.composer(): Composer?`).
- [ ] Spec §8 error handling — all 8 rows have either a unit test or a manual QA case.
- [ ] Spec §9 testing — xlsx 54 rows in Stage 0; Kotlin tests structured per `*Test.kt` file per spec; manual QA matrix in Stage 13.
- [ ] No "TBD" / "implement later" / "similar to Task N" without showing code.

### Known minor gaps (acceptable)

- Stage 5 (FR), 6 (ES), 7 (DE), 8 (IT), 9 (PT) tasks are presented as patterns rather than full TDD step-by-step. Reason: each is structurally identical to Stage 4 (VietnameseLanguagePack), just data-only swaps. The pattern is given; the implementer is expected to write one failing test per language pack, implement, commit, just like Stage 4.

---

## Done criteria

- All KEYBOARD-MULTI-001..054 tests pass on the production build (Samsung A52 + Xiaomi + Pixel emulator).
- `./gradlew test` green for every Kotlin test file.
- `flutter analyze` clean.
- Tracking GH issue has `status: deployed to production` label and a `## ✅ How to Verify` comment.
- Memory note `project_android_keyboard_strategy.md` rewritten.
- Issue stays open — closed by Tan/Mark only after manual verification on real devices.
