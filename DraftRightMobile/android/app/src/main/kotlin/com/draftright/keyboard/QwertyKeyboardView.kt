package com.draftright.keyboard

import android.content.Context
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Handler
import android.os.Looper
import android.util.TypedValue
import android.view.Gravity
import android.view.MotionEvent
import android.view.View
import android.widget.LinearLayout
import android.widget.PopupWindow
import android.widget.TextView
import com.draftright.keyboard.lang.EnglishLanguagePack

interface KeyboardActionListener {
    fun onCharTyped(char: String)
    fun onBackspace()
    fun onEnter()
    fun onSpace()
    fun onSwitchKeyboard()
    fun onSwitchKeyboardLongPress()
    /** Horizontal swipe on the space bar. +1 = right (next language),
     *  -1 = left (previous language). Samsung-style language cycle. */
    fun onSpaceSwipe(direction: Int) {}
}

class QwertyKeyboardView(
    context: Context,
    private val listener: KeyboardActionListener
) : LinearLayout(context) {

    private val ROW_HEIGHT_DP = 48
    private val KEY_MARGIN_DP = 3
    private val KEY_RADIUS_DP = 5

    // Key-press preview popup (the magnified character shown above a pressed
    // key). Sized to match the larger, easier-to-read preview on stock
    // Samsung / Gboard keyboards.
    private val KEY_POPUP_WIDTH_DP = 56
    private val KEY_POPUP_HEIGHT_DP = 64
    private val KEY_POPUP_TEXT_SP = 32f

    private var shiftState = ShiftState.OFF
    private var currentLayer = 0 // 0=alpha, 1=symbols1, 2=symbols2
    private var lastShiftTap = 0L

    private val handler = Handler(Looper.getMainLooper())
    private var backspaceRepeating = false

    var languagePack: LanguagePack = EnglishLanguagePack
        set(value) {
            field = value
            currentLayer = 0
            shiftState = ShiftState.OFF
            buildKeyboard()
        }

    /** Current shift state, so the IME can feed it back into auto-cap. */
    val currentShiftState: ShiftState get() = shiftState

    /**
     * Apply an auto-capitalization decision from the IME (see [AutoCapitalize]).
     * Only rebuilds when the state actually changes, to avoid flicker on every
     * cursor update, and only on the alpha layer so a symbol layer is untouched.
     */
    fun applyAutoShift(state: ShiftState) {
        if (currentLayer != 0 || state == shiftState) return
        shiftState = state
        buildKeyboard()
    }

    private val keyColor: Int
    private val keyColorSpecial: Int
    private val keyColorPressed: Int
    private val keyTextColor: Int
    private val keyboardBgColor: Int

    init {
        orientation = VERTICAL

        val isDark = KeyboardTheme.isDark(context)

        if (isDark) {
            keyboardBgColor = Color.parseColor("#1B1B1F")
            keyColor = Color.parseColor("#4A4A4A")
            keyColorSpecial = Color.parseColor("#363636")
            keyColorPressed = Color.parseColor("#5A5A5A")
            keyTextColor = Color.WHITE
        } else {
            keyboardBgColor = Color.parseColor("#ECEFF1")
            keyColor = Color.WHITE
            keyColorSpecial = Color.parseColor("#B0BEC5")
            keyColorPressed = Color.parseColor("#D6D6D6")
            keyTextColor = Color.parseColor("#212121")
        }

        setBackgroundColor(keyboardBgColor)
        buildKeyboard()
    }

    private fun isCharKey(code: Int): Boolean = SpecialKeys.isCharKey(code)
    private fun isSpaceKey(code: Int): Boolean = SpecialKeys.isSpace(code)

    fun buildKeyboard() {
        removeAllViews()
        val rows = when (currentLayer) {
            0 -> languagePack.alphaRows
            1 -> languagePack.symbols1Rows
            else -> languagePack.symbols2Rows
        }

        for ((rowIndex, row) in rows.withIndex()) {
            val rowLayout = LinearLayout(context).apply {
                orientation = HORIZONTAL
                gravity = Gravity.CENTER
                layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, dpToPx(ROW_HEIGHT_DP))
                setPadding(dpToPx(KEY_MARGIN_DP), 0, dpToPx(KEY_MARGIN_DP), 0)
            }

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

    private var accentPopup: AccentPopupView? = null

    private fun createKeyView(keyDef: KeyDef): View {
        val code = keyDef.code
        val isSpecial = !isCharKey(code) && !isSpaceKey(code)
        val isShiftActive = code == SpecialKeys.SHIFT && shiftState != ShiftState.OFF

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
            isCharKey(code) && currentLayer == 0 && shiftState != ShiftState.OFF ->
                keyDef.label.uppercase(languagePack.locale)
            isSpaceKey(code) -> languagePack.displayName
            code == SpecialKeys.SHIFT && shiftState == ShiftState.CAPS_LOCK -> "⬆️"
            else -> keyDef.label
        }

        val textSize = when (code) {
            SpecialKeys.SYMBOLS, SpecialKeys.SYMBOLS2, SpecialKeys.ALPHA -> 12f
            else -> if (isSpaceKey(code)) 12f else 18f
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

        var longPressFired = false
        val accentChars: List<Char>? = if (isCharKey(code) && keyDef.label.length == 1) {
            languagePack.longPressAccents[keyDef.label[0]]
        } else null
        val longPressRunnable = Runnable {
            longPressFired = true
            dismissKeyPopup()
            if (accentChars != null && accentChars.isNotEmpty()) {
                accentPopup?.dismiss()
                val options = listOf(keyDef.label[0]) + accentChars
                val popup = AccentPopupView(context, tv, options) { picked ->
                    val out = if (currentLayer == 0 && shiftState != ShiftState.OFF) {
                        picked.toString().uppercase(languagePack.locale)
                    } else picked.toString()
                    listener.onCharTyped(out)
                    if (shiftState == ShiftState.SINGLE) {
                        shiftState = ShiftState.OFF
                        buildKeyboard()
                    }
                    accentPopup = null
                }
                accentPopup = popup
                popup.show()
            }
        }

        var spaceDownX = 0f
        var spaceSwiped = false
        tv.setOnTouchListener { v, event ->
            when (event.action) {
                MotionEvent.ACTION_DOWN -> {
                    bg.setColor(keyColorPressed)
                    v.invalidate()
                    longPressFired = false
                    if (isCharKey(code) && keyDef.label.length == 1 && keyDef.label !in SpecialKeys.NO_PREVIEW_LABELS) {
                        showKeyPopup(v, displayLabel)
                    }
                    if (accentChars != null && accentChars.isNotEmpty()) {
                        handler.postDelayed(longPressRunnable, SpecialKeys.LONG_PRESS_MS)
                    }
                    if (code == SpecialKeys.BACKSPACE) {
                        handleKeyPress(keyDef)
                        startBackspaceRepeat()
                    }
                    if (isSpaceKey(code)) {
                        spaceDownX = event.rawX
                        spaceSwiped = false
                    }
                    true
                }
                MotionEvent.ACTION_MOVE -> {
                    if (isSpaceKey(code) && !spaceSwiped) {
                        val dx = event.rawX - spaceDownX
                        if (kotlin.math.abs(dx) > SpecialKeys.SPACE_SWIPE_THRESHOLD_PX) {
                            spaceSwiped = true
                            listener.onSpaceSwipe(if (dx > 0) 1 else -1)
                        }
                    }
                    true
                }
                MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> {
                    handler.removeCallbacks(longPressRunnable)
                    val restoreColor = when {
                        isShiftActive && code == SpecialKeys.SHIFT -> keyColorPressed
                        isSpecial -> keyColorSpecial
                        else -> keyColor
                    }
                    bg.setColor(restoreColor)
                    v.invalidate()
                    dismissKeyPopup()
                    if (code == SpecialKeys.BACKSPACE) {
                        stopBackspaceRepeat()
                    } else if (event.action == MotionEvent.ACTION_UP && !longPressFired && !spaceSwiped) {
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
        val code = keyDef.code
        when {
            isCharKey(code) -> {
                val char = if (currentLayer == 0 && shiftState != ShiftState.OFF) {
                    keyDef.label.uppercase(languagePack.locale)
                } else {
                    keyDef.label
                }
                listener.onCharTyped(char)
                if (shiftState == ShiftState.SINGLE) {
                    shiftState = ShiftState.OFF
                    buildKeyboard()
                }
            }
            isSpaceKey(code) -> listener.onSpace()
            code == SpecialKeys.BACKSPACE -> listener.onBackspace()
            code == SpecialKeys.ENTER -> listener.onEnter()
            code == SpecialKeys.SHIFT -> {
                val now = System.currentTimeMillis()
                if (now - lastShiftTap < SpecialKeys.DOUBLE_TAP_MS) {
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
            code == SpecialKeys.SYMBOLS -> {
                currentLayer = 1
                buildKeyboard()
            }
            code == SpecialKeys.SYMBOLS2 -> {
                currentLayer = 2
                buildKeyboard()
            }
            code == SpecialKeys.ALPHA -> {
                currentLayer = 0
                buildKeyboard()
            }
            code == SpecialKeys.GLOBE -> listener.onSwitchKeyboard()
            code == SpecialKeys.GLOBE_PICKER -> listener.onSwitchKeyboardLongPress()
        }
    }

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
        handler.postDelayed(backspaceRunnable, 400)
    }

    private fun stopBackspaceRepeat() {
        backspaceRepeating = false
        handler.removeCallbacks(backspaceRunnable)
    }

    private var popupWindow: PopupWindow? = null

    private fun showKeyPopup(anchorView: View, label: String) {
        dismissKeyPopup()

        val popupView = TextView(context).apply {
            text = label
            setTextSize(TypedValue.COMPLEX_UNIT_SP, KEY_POPUP_TEXT_SP)
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

        val popup = PopupWindow(popupView, dpToPx(KEY_POPUP_WIDTH_DP), dpToPx(KEY_POPUP_HEIGHT_DP), false).apply {
            isClippingEnabled = false
        }

        val xOff = (anchorView.width - dpToPx(KEY_POPUP_WIDTH_DP)) / 2
        val yOff = -(dpToPx(KEY_POPUP_HEIGHT_DP) + dpToPx(ROW_HEIGHT_DP))
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
