# Android QWERTY Keyboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a full QWERTY keyboard beneath the DraftRight toolbar in the Android IME so it functions as a complete keyboard replacement.

**Architecture:** Single new file `QwertyKeyboardView.kt` containing all key layout, rendering, shift/symbol state, and touch handling. `DraftRightIME.kt` is updated to add the keyboard view below the toolbar and implement the `KeyboardActionListener` callback interface. All views are programmatic (no XML layouts).

**Tech Stack:** Kotlin, Android SDK (`InputMethodService`, `InputConnection`), programmatic views

**Spec:** `docs/superpowers/specs/2026-03-29-android-qwerty-keyboard-design.md`

---

### Task 1: Create QwertyKeyboardView with Alpha Layer

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/QwertyKeyboardView.kt`

- [ ] **Step 1: Create QwertyKeyboardView.kt with data model, callback interface, and alpha layer**

Create `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/QwertyKeyboardView.kt`:

```kotlin
package com.draftright.keyboard

import android.content.Context
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.graphics.RectF
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.graphics.drawable.StateListDrawable
import android.os.Handler
import android.os.Looper
import android.util.TypedValue
import android.view.Gravity
import android.view.HapticFeedbackConstants
import android.view.MotionEvent
import android.view.View
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.PopupWindow
import android.widget.TextView

enum class KeyCode {
    CHAR,
    BACKSPACE,
    SHIFT,
    ENTER,
    SPACE,
    SYMBOLS,
    ALPHA,
    SYMBOLS2,
    GLOBE
}

data class KeyDef(
    val label: String,
    val code: KeyCode,
    val widthWeight: Float = 1.0f
)

interface KeyboardActionListener {
    fun onCharTyped(char: String)
    fun onBackspace()
    fun onEnter()
    fun onSpace()
    fun onSwitchKeyboard()
}

enum class ShiftState { OFF, SINGLE, CAPS_LOCK }

class QwertyKeyboardView(
    context: Context,
    private val listener: KeyboardActionListener
) : LinearLayout(context) {

    private val ROW_HEIGHT_DP = 48
    private val KEY_MARGIN_DP = 3
    private val KEY_RADIUS_DP = 5

    private var shiftState = ShiftState.OFF
    private var currentLayer = 0 // 0=alpha, 1=symbols1, 2=symbols2
    private var lastShiftTap = 0L

    private val handler = Handler(Looper.getMainLooper())
    private var backspaceRepeating = false

    // Key layout definitions
    private val alphaRows = listOf(
        listOf(
            KeyDef("q", KeyCode.CHAR), KeyDef("w", KeyCode.CHAR), KeyDef("e", KeyCode.CHAR),
            KeyDef("r", KeyCode.CHAR), KeyDef("t", KeyCode.CHAR), KeyDef("y", KeyCode.CHAR),
            KeyDef("u", KeyCode.CHAR), KeyDef("i", KeyCode.CHAR), KeyDef("o", KeyCode.CHAR),
            KeyDef("p", KeyCode.CHAR)
        ),
        listOf(
            KeyDef("a", KeyCode.CHAR), KeyDef("s", KeyCode.CHAR), KeyDef("d", KeyCode.CHAR),
            KeyDef("f", KeyCode.CHAR), KeyDef("g", KeyCode.CHAR), KeyDef("h", KeyCode.CHAR),
            KeyDef("j", KeyCode.CHAR), KeyDef("k", KeyCode.CHAR), KeyDef("l", KeyCode.CHAR)
        ),
        listOf(
            KeyDef("\u21E7", KeyCode.SHIFT, 1.5f),
            KeyDef("z", KeyCode.CHAR), KeyDef("x", KeyCode.CHAR), KeyDef("c", KeyCode.CHAR),
            KeyDef("v", KeyCode.CHAR), KeyDef("b", KeyCode.CHAR), KeyDef("n", KeyCode.CHAR),
            KeyDef("m", KeyCode.CHAR),
            KeyDef("\u232B", KeyCode.BACKSPACE, 1.5f)
        ),
        listOf(
            KeyDef("?123", KeyCode.SYMBOLS, 1.5f),
            KeyDef("\uD83C\uDF10", KeyCode.GLOBE, 1.0f),
            KeyDef(",", KeyCode.CHAR, 1.0f),
            KeyDef(" ", KeyCode.SPACE, 5.0f),
            KeyDef(".", KeyCode.CHAR, 1.0f),
            KeyDef("\u23CE", KeyCode.ENTER, 1.5f)
        )
    )

    private val symbols1Rows = listOf(
        listOf(
            KeyDef("1", KeyCode.CHAR), KeyDef("2", KeyCode.CHAR), KeyDef("3", KeyCode.CHAR),
            KeyDef("4", KeyCode.CHAR), KeyDef("5", KeyCode.CHAR), KeyDef("6", KeyCode.CHAR),
            KeyDef("7", KeyCode.CHAR), KeyDef("8", KeyCode.CHAR), KeyDef("9", KeyCode.CHAR),
            KeyDef("0", KeyCode.CHAR)
        ),
        listOf(
            KeyDef("@", KeyCode.CHAR), KeyDef("#", KeyCode.CHAR), KeyDef("$", KeyCode.CHAR),
            KeyDef("%", KeyCode.CHAR), KeyDef("&", KeyCode.CHAR), KeyDef("-", KeyCode.CHAR),
            KeyDef("+", KeyCode.CHAR), KeyDef("(", KeyCode.CHAR), KeyDef(")", KeyCode.CHAR)
        ),
        listOf(
            KeyDef("#+=", KeyCode.SYMBOLS2, 1.5f),
            KeyDef("!", KeyCode.CHAR), KeyDef("\"", KeyCode.CHAR), KeyDef("'", KeyCode.CHAR),
            KeyDef(":", KeyCode.CHAR), KeyDef(";", KeyCode.CHAR), KeyDef("/", KeyCode.CHAR),
            KeyDef("?", KeyCode.CHAR),
            KeyDef("\u232B", KeyCode.BACKSPACE, 1.5f)
        ),
        listOf(
            KeyDef("ABC", KeyCode.ALPHA, 1.5f),
            KeyDef("\uD83C\uDF10", KeyCode.GLOBE, 1.0f),
            KeyDef(",", KeyCode.CHAR, 1.0f),
            KeyDef(" ", KeyCode.SPACE, 5.0f),
            KeyDef(".", KeyCode.CHAR, 1.0f),
            KeyDef("\u23CE", KeyCode.ENTER, 1.5f)
        )
    )

    private val symbols2Rows = listOf(
        listOf(
            KeyDef("~", KeyCode.CHAR), KeyDef("`", KeyCode.CHAR), KeyDef("|", KeyCode.CHAR),
            KeyDef("\u2022", KeyCode.CHAR), KeyDef("\u221A", KeyCode.CHAR), KeyDef("\u03C0", KeyCode.CHAR),
            KeyDef("\u00F7", KeyCode.CHAR), KeyDef("\u00D7", KeyCode.CHAR), KeyDef("\u00B6", KeyCode.CHAR),
            KeyDef("\u0394", KeyCode.CHAR)
        ),
        listOf(
            KeyDef("\u00A3", KeyCode.CHAR), KeyDef("\u20AC", KeyCode.CHAR), KeyDef("\u00A5", KeyCode.CHAR),
            KeyDef("^", KeyCode.CHAR), KeyDef("[", KeyCode.CHAR), KeyDef("]", KeyCode.CHAR),
            KeyDef("{", KeyCode.CHAR), KeyDef("}", KeyCode.CHAR)
        ),
        listOf(
            KeyDef("?123", KeyCode.SYMBOLS, 1.5f),
            KeyDef("\u00A9", KeyCode.CHAR), KeyDef("\u00AE", KeyCode.CHAR), KeyDef("\u2122", KeyCode.CHAR),
            KeyDef("\\", KeyCode.CHAR), KeyDef("<", KeyCode.CHAR), KeyDef(">", KeyCode.CHAR),
            KeyDef("=", KeyCode.CHAR),
            KeyDef("\u232B", KeyCode.BACKSPACE, 1.5f)
        ),
        listOf(
            KeyDef("ABC", KeyCode.ALPHA, 1.5f),
            KeyDef("\uD83C\uDF10", KeyCode.GLOBE, 1.0f),
            KeyDef(",", KeyCode.CHAR, 1.0f),
            KeyDef(" ", KeyCode.SPACE, 5.0f),
            KeyDef(".", KeyCode.CHAR, 1.0f),
            KeyDef("\u23CE", KeyCode.ENTER, 1.5f)
        )
    )

    private val keyColor: Int
    private val keyColorSpecial: Int
    private val keyColorPressed: Int
    private val keyTextColor: Int
    private val keyboardBgColor: Int

    init {
        orientation = VERTICAL
        val tv = TypedValue()
        val theme = context.theme

        // Resolve theme colors with fallbacks
        keyTextColor = if (theme.resolveAttribute(android.R.attr.colorForeground, tv, true)) tv.data else Color.BLACK
        keyboardBgColor = if (theme.resolveAttribute(android.R.attr.colorBackground, tv, true)) tv.data else Color.parseColor("#ECEFF1")

        // Derive key colors from background
        val isDark = Color.luminance(keyboardBgColor) < 0.5f
        if (isDark) {
            keyColor = Color.parseColor("#4A4A4A")
            keyColorSpecial = Color.parseColor("#363636")
            keyColorPressed = Color.parseColor("#5A5A5A")
        } else {
            keyColor = Color.WHITE
            keyColorSpecial = Color.parseColor("#B0BEC5")
            keyColorPressed = Color.parseColor("#D6D6D6")
        }

        setBackgroundColor(keyboardBgColor)
        buildKeyboard()
    }

    private fun buildKeyboard() {
        removeAllViews()
        val rows = when (currentLayer) {
            0 -> alphaRows
            1 -> symbols1Rows
            else -> symbols2Rows
        }

        for ((rowIndex, row) in rows.withIndex()) {
            val rowLayout = LinearLayout(context).apply {
                orientation = HORIZONTAL
                gravity = Gravity.CENTER
                layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, dpToPx(ROW_HEIGHT_DP))
                setPadding(dpToPx(KEY_MARGIN_DP), 0, dpToPx(KEY_MARGIN_DP), 0)
            }

            // Row 2 (index 1) in alpha layer has 9 keys — add half-key padding for centering
            if (currentLayer == 0 && rowIndex == 1) {
                val halfKeyPad = dpToPx(KEY_MARGIN_DP * 2)
                rowLayout.setPadding(halfKeyPad + dpToPx(KEY_MARGIN_DP), 0, halfKeyPad + dpToPx(KEY_MARGIN_DP), 0)
            }

            for (keyDef in row) {
                val keyView = createKeyView(keyDef)
                val params = LayoutParams(0, LayoutParams.MATCH_PARENT, keyDef.widthWeight)
                params.setMargins(dpToPx(KEY_MARGIN_DP), dpToPx(KEY_MARGIN_DP), dpToPx(KEY_MARGIN_DP), dpToPx(KEY_MARGIN_DP))
                keyView.layoutParams = params
                rowLayout.addView(keyView)
            }
            addView(rowLayout)
        }
    }

    private fun createKeyView(keyDef: KeyDef): View {
        val isSpecial = keyDef.code != KeyCode.CHAR && keyDef.code != KeyCode.SPACE
        val isShiftActive = keyDef.code == KeyCode.SHIFT && shiftState != ShiftState.OFF

        val bgColor = when {
            isShiftActive -> keyColorPressed
            isSpecial -> keyColorSpecial
            else -> keyColor
        }

        val bg = GradientDrawable().apply {
            setColor(bgColor)
            cornerRadius = dpToPx(KEY_RADIUS_DP).toFloat()
        }

        val displayLabel = when {
            keyDef.code == KeyCode.CHAR && currentLayer == 0 && shiftState != ShiftState.OFF ->
                keyDef.label.uppercase()
            keyDef.code == KeyCode.SPACE -> ""
            keyDef.code == KeyCode.SHIFT && shiftState == ShiftState.CAPS_LOCK -> "\u21EA" // caps lock icon
            else -> keyDef.label
        }

        val textSize = when (keyDef.code) {
            KeyCode.SYMBOLS, KeyCode.SYMBOLS2, KeyCode.ALPHA -> 12f
            else -> 18f
        }

        val tv = TextView(context).apply {
            text = displayLabel
            setTextSize(TypedValue.COMPLEX_UNIT_SP, textSize)
            setTextColor(keyTextColor)
            gravity = Gravity.CENTER
            background = bg
            isClickable = true
            isFocusable = true
            typeface = Typeface.DEFAULT
        }

        // Touch handling
        tv.setOnTouchListener { v, event ->
            when (event.action) {
                MotionEvent.ACTION_DOWN -> {
                    bg.setColor(keyColorPressed)
                    v.invalidate()
                    if (keyDef.code == KeyCode.CHAR && keyDef.label.length == 1 && keyDef.label != "," && keyDef.label != ".") {
                        showKeyPopup(v, displayLabel)
                    }
                    if (keyDef.code == KeyCode.BACKSPACE) {
                        handleKeyPress(keyDef)
                        startBackspaceRepeat()
                    }
                    true
                }
                MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> {
                    val restoreColor = when {
                        isShiftActive && keyDef.code == KeyCode.SHIFT -> keyColorPressed
                        isSpecial -> keyColorSpecial
                        else -> keyColor
                    }
                    bg.setColor(restoreColor)
                    v.invalidate()
                    dismissKeyPopup()
                    if (keyDef.code == KeyCode.BACKSPACE) {
                        stopBackspaceRepeat()
                    } else if (event.action == MotionEvent.ACTION_UP) {
                        handleKeyPress(keyDef)
                    }
                    true
                }
                else -> false
            }
        }

        return tv
    }

    private fun handleKeyPress(keyDef: KeyDef) {
        when (keyDef.code) {
            KeyCode.CHAR -> {
                val char = if (currentLayer == 0 && shiftState != ShiftState.OFF) {
                    keyDef.label.uppercase()
                } else {
                    keyDef.label
                }
                listener.onCharTyped(char)
                if (shiftState == ShiftState.SINGLE) {
                    shiftState = ShiftState.OFF
                    buildKeyboard()
                }
            }
            KeyCode.BACKSPACE -> {
                listener.onBackspace()
            }
            KeyCode.ENTER -> {
                listener.onEnter()
            }
            KeyCode.SPACE -> {
                listener.onSpace()
            }
            KeyCode.SHIFT -> {
                val now = System.currentTimeMillis()
                if (now - lastShiftTap < 300) {
                    // Double tap
                    shiftState = if (shiftState == ShiftState.CAPS_LOCK) ShiftState.OFF else ShiftState.CAPS_LOCK
                } else {
                    shiftState = when (shiftState) {
                        ShiftState.OFF -> ShiftState.SINGLE
                        ShiftState.SINGLE -> ShiftState.OFF
                        ShiftState.CAPS_LOCK -> ShiftState.OFF
                    }
                }
                lastShiftTap = now
                buildKeyboard()
            }
            KeyCode.SYMBOLS -> {
                currentLayer = 1
                buildKeyboard()
            }
            KeyCode.SYMBOLS2 -> {
                currentLayer = 2
                buildKeyboard()
            }
            KeyCode.ALPHA -> {
                currentLayer = 0
                buildKeyboard()
            }
            KeyCode.GLOBE -> {
                listener.onSwitchKeyboard()
            }
        }
    }

    // Backspace repeat
    private val backspaceRunnable = object : Runnable {
        override fun run() {
            if (backspaceRepeating) {
                listener.onBackspace()
                handler.postDelayed(this, 50)
            }
        }
    }

    private fun startBackspaceRepeat() {
        backspaceRepeating = true
        handler.postDelayed(backspaceRunnable, 400) // initial delay before repeat starts
    }

    private fun stopBackspaceRepeat() {
        backspaceRepeating = false
        handler.removeCallbacks(backspaceRunnable)
    }

    // Key popup preview
    private var popupWindow: PopupWindow? = null

    private fun showKeyPopup(anchorView: View, label: String) {
        dismissKeyPopup()

        val popupView = TextView(context).apply {
            text = label
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 24f)
            setTextColor(keyTextColor)
            gravity = Gravity.CENTER
            val popBg = GradientDrawable().apply {
                setColor(keyColor)
                cornerRadius = dpToPx(KEY_RADIUS_DP).toFloat()
                setStroke(1, Color.LTGRAY)
            }
            background = popBg
            setPadding(dpToPx(12), dpToPx(8), dpToPx(12), dpToPx(8))
        }

        val popup = PopupWindow(popupView, dpToPx(48), dpToPx(52), false).apply {
            isClippingEnabled = false
        }

        val xOff = (anchorView.width - dpToPx(48)) / 2
        val yOff = -(dpToPx(52) + dpToPx(ROW_HEIGHT_DP))
        popup.showAsDropDown(anchorView, xOff, yOff)
        popupWindow = popup
    }

    private fun dismissKeyPopup() {
        popupWindow?.dismiss()
        popupWindow = null
    }

    private fun dpToPx(dp: Int): Int =
        TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, dp.toFloat(), resources.displayMetrics).toInt()
}
```

- [ ] **Step 2: Commit**

```bash
cd /opt/openAi/DraftRight
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/QwertyKeyboardView.kt
git commit -m "feat: add QwertyKeyboardView with alpha and symbol layers"
```

---

### Task 2: Integrate Keyboard into DraftRightIME

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt`

- [ ] **Step 1: Update DraftRightIME to add keyboard view and implement KeyboardActionListener**

Replace the entire contents of `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt`:

```kotlin
package com.draftright.keyboard

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.inputmethodservice.InputMethodService
import android.os.Handler
import android.os.Looper
import android.view.View
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.ExtractedTextRequest
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast

class DraftRightIME : InputMethodService(), KeyboardActionListener {

    private lateinit var settings: SharedSettings
    private val aiClient = OpenAIClient()
    private val mainHandler = Handler(Looper.getMainLooper())
    private var toolbar: ToolbarView? = null
    private var keyboard: QwertyKeyboardView? = null
    private var diffSheet: DiffSheetView? = null
    private var rootLayout: LinearLayout? = null
    private var originalText: String? = null

    override fun onCreateInputView(): View {
        settings = SharedSettings(this)

        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
        }

        val tb = ToolbarView(this,
            onToneSelected = { tone -> handleToneSelected(tone) },
            onUndo = { handleUndo() }
        )
        toolbar = tb
        root.addView(tb)

        val kb = QwertyKeyboardView(this, this)
        keyboard = kb
        root.addView(kb)

        rootLayout = root
        return root
    }

    // --- KeyboardActionListener ---

    override fun onCharTyped(char: String) {
        currentInputConnection?.commitText(char, 1)
    }

    override fun onBackspace() {
        currentInputConnection?.deleteSurroundingText(1, 0)
    }

    override fun onEnter() {
        val ic = currentInputConnection ?: return
        val ei = currentInputEditorInfo
        if (ei != null && ei.imeOptions and EditorInfo.IME_FLAG_NO_ENTER_ACTION == 0) {
            val action = ei.imeOptions and EditorInfo.IME_MASK_ACTION
            if (action != EditorInfo.IME_ACTION_NONE && action != EditorInfo.IME_ACTION_UNSPECIFIED) {
                ic.performEditorAction(action)
                return
            }
        }
        ic.commitText("\n", 1)
    }

    override fun onSpace() {
        currentInputConnection?.commitText(" ", 1)
    }

    @Suppress("DEPRECATION")
    override fun onSwitchKeyboard() {
        val imeManager = getSystemService(INPUT_METHOD_SERVICE) as android.view.inputmethod.InputMethodManager
        imeManager.switchToNextInputMethod(window.window?.attributes?.token, false)
    }

    // --- Existing tone/rewrite logic (unchanged) ---

    private fun readFullText(): String {
        val ic = currentInputConnection ?: return ""
        val extracted = ic.getExtractedText(ExtractedTextRequest(), 0) ?: return ""
        return extracted.text?.toString() ?: ""
    }

    private fun replaceAllText(newText: String) {
        val ic = currentInputConnection ?: return
        val extracted = ic.getExtractedText(ExtractedTextRequest(), 0) ?: return
        val length = extracted.text?.length ?: 0
        ic.setSelection(0, length)
        ic.commitText(newText, 1)
    }

    private fun handleToneSelected(tone: Tone) {
        val text = readFullText().trim()
        if (text.isEmpty()) return

        if (settings.aiProvider == "openai" && settings.apiKey.isEmpty()) {
            showBanner("Open DraftRight app to set up API key")
            return
        }

        originalText = text
        toolbar?.setLoading(tone)

        aiClient.rewrite(text, tone, settings) { result ->
            mainHandler.post {
                toolbar?.clearLoading()
                result.onSuccess { rewritten ->
                    showDiffSheet(text, rewritten)
                }
                result.onFailure { error ->
                    showBanner(error.message ?: "Rewrite failed")
                }
            }
        }
    }

    private fun showDiffSheet(original: String, rewritten: String) {
        dismissDiffSheet()

        val sheet = DiffSheetView(this, original, rewritten,
            onReplace = { text ->
                replaceAllText(text)
                dismissDiffSheet()
                toolbar?.showUndo()
            },
            onCopy = { text ->
                val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
                clipboard.setPrimaryClip(ClipData.newPlainText("DraftRight", text))
                Toast.makeText(this, "Copied", Toast.LENGTH_SHORT).show()
                dismissDiffSheet()
            },
            onCancel = {
                dismissDiffSheet()
            }
        )

        sheet.layoutParams = LinearLayout.LayoutParams(
            LinearLayout.LayoutParams.MATCH_PARENT,
            dpToPx(280)
        )
        diffSheet = sheet
        rootLayout?.addView(sheet)
    }

    private fun dismissDiffSheet() {
        diffSheet?.let { rootLayout?.removeView(it) }
        diffSheet = null
    }

    private fun handleUndo() {
        originalText?.let {
            replaceAllText(it)
            toolbar?.hideUndo()
            originalText = null
        }
    }

    private fun showBanner(message: String) {
        val banner = TextView(this).apply {
            text = message
            textSize = 12f
            setTextColor(0xFFFF9800.toInt())
            setBackgroundColor(0x1AFF9800)
            setPadding(dpToPx(16), dpToPx(4), dpToPx(16), dpToPx(4))
        }
        rootLayout?.addView(banner, 1)
        mainHandler.postDelayed({ rootLayout?.removeView(banner) }, 3000)
    }

    private fun dpToPx(dp: Int): Int {
        return (dp * resources.displayMetrics.density).toInt()
    }
}
```

- [ ] **Step 2: Commit**

```bash
cd /opt/openAi/DraftRight
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt
git commit -m "feat: integrate QwertyKeyboardView into DraftRightIME"
```

---

### Task 3: Build Verification

**Files:** None (verification only)

- [ ] **Step 1: Run Gradle build to verify compilation**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile/android && ./gradlew assembleDebug 2>&1 | tail -20
```

Expected: `BUILD SUCCESSFUL`

- [ ] **Step 2: Fix any compilation errors**

If build fails, read the error output and fix the relevant file. Common issues:
- Missing imports (add them to QwertyKeyboardView.kt)
- Type mismatches in callback interface

- [ ] **Step 3: Commit any fixes**

```bash
cd /opt/openAi/DraftRight
git add -A DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/
git commit -m "fix: resolve build errors in keyboard implementation"
```

Only commit if there were fixes. Skip if build passed clean.

---

### Task 4: Manual Verification on Device

**Files:** None (testing only)

- [ ] **Step 1: Install on device/emulator**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter run
```

Or install the APK directly:

```bash
cd /opt/openAi/DraftRight/DraftRightMobile/android && ./gradlew installDebug
```

- [ ] **Step 2: Enable DraftRight keyboard**

1. Open device Settings > System > Language & Input > On-screen keyboard > Manage keyboards
2. Enable "DraftRight"
3. Open any app with a text field (e.g., MS Teams chat, Notes)
4. Switch to DraftRight keyboard via the keyboard switcher (globe icon or nav bar keyboard icon)

- [ ] **Step 3: Verify these behaviors**

| Test | Expected |
|---|---|
| Tap letter keys | Letters appear in text field |
| Tap shift then a letter | One uppercase letter, then reverts to lowercase |
| Double-tap shift | Caps lock — all letters uppercase until shift tapped again |
| Tap `?123` | Switches to number/symbol layout |
| Tap `#+=` | Switches to second symbol page |
| Tap `ABC` | Returns to QWERTY letters |
| Tap backspace | Deletes one character |
| Long-press backspace | Continuous delete |
| Tap enter | Sends message (in chat) or newline (in notes) |
| Tap space | Space character |
| Tap globe icon | Switches to next keyboard (e.g., Gboard) |
| Tap tone button on toolbar | Reads text, calls API, shows diff sheet |
| Key popup | Enlarged letter appears above finger on press |
| Dark mode | Keys follow system dark theme |
