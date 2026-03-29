# Android QWERTY Keyboard for DraftRight IME

**Date:** 2026-03-29
**Status:** Approved
**Platform:** Android

## Overview

Add a full QWERTY keyboard layout beneath the existing DraftRight toolbar in the Android IME (`DraftRightIME`). This makes DraftRight a complete keyboard replacement so users can type and use AI rewrite features without switching input methods.

Currently the IME only renders a toolbar with tone buttons — no key layout. Users must switch to DraftRight as active keyboard but then have no way to type. This change adds a functional keyboard below the toolbar.

## Layout

```
+----------------------------------------------+
| DraftRight Toolbar                           |
| [S][N][P][C][T][Tr]                [Undo]   |
+----------------------------------------------+
| QWERTY Keyboard                              |
|                                              |
|  q  w  e  r  t  y  u  i  o  p               |
|   a  s  d  f  g  h  j  k  l                 |
|  SHIFT  z  x  c  v  b  n  m  BKSP           |
|  ?123  GLOBE  ,  [    space    ]  .  ENTER   |
+----------------------------------------------+
```

**Dimensions:**
- Toolbar: 44dp (existing, unchanged)
- Key rows: 4 rows x 48dp = 192dp
- Total IME height: ~236dp

## Key Layers

### Layer 1: Alpha (default)

Row 1: `q w e r t y u i o p`
Row 2: `a s d f g h j k l` (inset half-key width from edges)
Row 3: `SHIFT z x c v b n m BACKSPACE`
Row 4: `?123 GLOBE , SPACE . ENTER`

### Layer 2: Symbols Page 1 (via `?123`)

Row 1: `1 2 3 4 5 6 7 8 9 0`
Row 2: `@ # $ % & - + ( )`
Row 3: `#+=  !  "  '  :  ;  /  ?  BACKSPACE`
Row 4: `ABC GLOBE , SPACE . ENTER`

### Layer 3: Symbols Page 2 (via `#+=`)

Row 1: `~ \` | * sqrt pi / x para delta`
Row 2: `pound euro yen ^ [ ] { }`
Row 3: `?123  (c)  (r)  TM  \\  <  >  =  BACKSPACE`
Row 4: `ABC GLOBE , SPACE . ENTER`

## Key Behavior

### Typing
- Tap key -> `InputConnection.commitText(character, 1)`
- Backspace -> `InputConnection.deleteSurroundingText(1, 0)`
- Long-press backspace -> repeat delete at ~50ms interval
- Enter -> send `EditorInfo.imeAction` if set (e.g., Send in chat apps), otherwise commit newline
- Space -> commit space character

### Shift States
- Default: lowercase letters
- Single tap SHIFT: next letter uppercase, then auto-revert to lowercase
- Double tap SHIFT: caps lock (visual indicator on shift key). Tap again to unlock.

### Symbol Toggle
- `?123` key: switch to Symbols Page 1
- `#+=` key (on Symbols Page 1): switch to Symbols Page 2
- `?123` key (on Symbols Page 2): switch back to Symbols Page 1
- `ABC` key (on either symbol page): switch back to Alpha layer

### Globe Key
- Calls `switchToNextInputMethod()` to let user switch to Gboard or other keyboards
- Essential since DraftRight has no autocorrect

### Key Press Feedback
- Visual: key background darkens on press (state list drawable)
- Key pop-up: show enlarged letter above finger on press
- No haptic or sound feedback (keep simple)

## Visual Style

- Follow Material You / system theme
- Keys: rounded rectangles with slight elevation
- Dark/light mode follows device setting
- Key background: `?android:attr/colorSurface` with slight contrast
- Key text: `?android:attr/colorOnSurface`
- Special keys (Shift, Backspace, Enter, ?123): slightly darker background
- Spacebar: wider, same style as letter keys
- Pressed state: darken background by ~15%

## Architecture

### New File: `QwertyKeyboardView.kt`

Single self-contained view class, programmatic layout (no XML — consistent with existing codebase).

```
QwertyKeyboardView (LinearLayout, VERTICAL)
  +-- KeyRow (LinearLayout, HORIZONTAL) x 4
       +-- KeyView (TextView) x N per row
```

**Key data model:**
```kotlin
data class KeyDef(
    label: String,        // display text
    code: KeyCode,        // action type
    widthWeight: Float    // relative width (1.0 standard, 1.5 shift, 5.0 space)
)

enum class KeyCode {
    CHAR,       // commits label as text
    BACKSPACE,
    SHIFT,
    ENTER,
    SPACE,
    SYMBOLS,    // toggle to symbol layer
    ALPHA,      // toggle back to alpha
    SYMBOLS2,   // toggle to symbol page 2
    GLOBE       // switch input method
}
```

**Callback interface:**
```kotlin
interface KeyboardActionListener {
    fun onCharTyped(char: String)
    fun onBackspace()
    fun onEnter()
    fun onSpace()
    fun onSwitchKeyboard()
}
```

### Changed File: `DraftRightIME.kt`

- `onCreateInputView()`: add `QwertyKeyboardView` below `ToolbarView` in root layout
- Implement `KeyboardActionListener` using `currentInputConnection`
- Handle backspace repeat via `Handler.postDelayed`

### Unchanged Files

- `ToolbarView.kt` — no changes
- `DiffSheetView.kt` — no changes
- `Tone.kt` — no changes
- `OpenAIClient.kt` — no changes
- `SharedSettings.kt` — no changes
- `AndroidManifest.xml` — no changes (IME already registered)
- `method.xml` — no changes

## Interaction with Existing Features

- When user taps a tone button on the toolbar, the existing flow works unchanged: reads text via `InputConnection`, calls API, shows diff sheet
- The diff sheet overlays on top of the keyboard (existing behavior via `rootLayout.addView`)
- Globe key provides escape hatch to switch back to user's preferred keyboard
