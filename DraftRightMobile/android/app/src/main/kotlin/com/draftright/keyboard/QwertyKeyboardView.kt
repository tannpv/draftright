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

enum class LegacyKeyCode {
    CHAR,
    BACKSPACE,
    SHIFT,
    ENTER,
    SPACE,
    SYMBOLS,
    ALPHA,
    SYMBOLS2,
    GLOBE,
    GLOBE_PICKER
}

data class LegacyKeyDef(
    val label: String,
    val code: LegacyKeyCode,
    val widthWeight: Float = 1.0f
)

interface KeyboardActionListener {
    fun onCharTyped(char: String)
    fun onBackspace()
    fun onEnter()
    fun onSpace()
    fun onSwitchKeyboard()
    fun onSwitchKeyboardLongPress()
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
            LegacyKeyDef("q", LegacyKeyCode.CHAR), LegacyKeyDef("w", LegacyKeyCode.CHAR), LegacyKeyDef("e", LegacyKeyCode.CHAR),
            LegacyKeyDef("r", LegacyKeyCode.CHAR), LegacyKeyDef("t", LegacyKeyCode.CHAR), LegacyKeyDef("y", LegacyKeyCode.CHAR),
            LegacyKeyDef("u", LegacyKeyCode.CHAR), LegacyKeyDef("i", LegacyKeyCode.CHAR), LegacyKeyDef("o", LegacyKeyCode.CHAR),
            LegacyKeyDef("p", LegacyKeyCode.CHAR)
        ),
        listOf(
            LegacyKeyDef("a", LegacyKeyCode.CHAR), LegacyKeyDef("s", LegacyKeyCode.CHAR), LegacyKeyDef("d", LegacyKeyCode.CHAR),
            LegacyKeyDef("f", LegacyKeyCode.CHAR), LegacyKeyDef("g", LegacyKeyCode.CHAR), LegacyKeyDef("h", LegacyKeyCode.CHAR),
            LegacyKeyDef("j", LegacyKeyCode.CHAR), LegacyKeyDef("k", LegacyKeyCode.CHAR), LegacyKeyDef("l", LegacyKeyCode.CHAR)
        ),
        listOf(
            LegacyKeyDef("\u2B06", LegacyKeyCode.SHIFT, 1.5f),
            LegacyKeyDef("z", LegacyKeyCode.CHAR), LegacyKeyDef("x", LegacyKeyCode.CHAR), LegacyKeyDef("c", LegacyKeyCode.CHAR),
            LegacyKeyDef("v", LegacyKeyCode.CHAR), LegacyKeyDef("b", LegacyKeyCode.CHAR), LegacyKeyDef("n", LegacyKeyCode.CHAR),
            LegacyKeyDef("m", LegacyKeyCode.CHAR),
            LegacyKeyDef("\u2190", LegacyKeyCode.BACKSPACE, 1.5f)
        ),
        listOf(
            LegacyKeyDef("?123", LegacyKeyCode.SYMBOLS, 1.5f),
            LegacyKeyDef("\uD83C\uDF10", LegacyKeyCode.GLOBE, 1.0f),
            LegacyKeyDef("\u2261", LegacyKeyCode.GLOBE_PICKER, 1.0f),
            LegacyKeyDef(",", LegacyKeyCode.CHAR, 1.0f),
            LegacyKeyDef(" ", LegacyKeyCode.SPACE, 4.0f),
            LegacyKeyDef(".", LegacyKeyCode.CHAR, 1.0f),
            LegacyKeyDef("\u21B5", LegacyKeyCode.ENTER, 1.5f)
        )
    )

    private val symbols1Rows = listOf(
        listOf(
            LegacyKeyDef("1", LegacyKeyCode.CHAR), LegacyKeyDef("2", LegacyKeyCode.CHAR), LegacyKeyDef("3", LegacyKeyCode.CHAR),
            LegacyKeyDef("4", LegacyKeyCode.CHAR), LegacyKeyDef("5", LegacyKeyCode.CHAR), LegacyKeyDef("6", LegacyKeyCode.CHAR),
            LegacyKeyDef("7", LegacyKeyCode.CHAR), LegacyKeyDef("8", LegacyKeyCode.CHAR), LegacyKeyDef("9", LegacyKeyCode.CHAR),
            LegacyKeyDef("0", LegacyKeyCode.CHAR)
        ),
        listOf(
            LegacyKeyDef("@", LegacyKeyCode.CHAR), LegacyKeyDef("#", LegacyKeyCode.CHAR), LegacyKeyDef("$", LegacyKeyCode.CHAR),
            LegacyKeyDef("%", LegacyKeyCode.CHAR), LegacyKeyDef("&", LegacyKeyCode.CHAR), LegacyKeyDef("-", LegacyKeyCode.CHAR),
            LegacyKeyDef("+", LegacyKeyCode.CHAR), LegacyKeyDef("(", LegacyKeyCode.CHAR), LegacyKeyDef(")", LegacyKeyCode.CHAR)
        ),
        listOf(
            LegacyKeyDef("#+=", LegacyKeyCode.SYMBOLS2, 1.5f),
            LegacyKeyDef("!", LegacyKeyCode.CHAR), LegacyKeyDef("\"", LegacyKeyCode.CHAR), LegacyKeyDef("'", LegacyKeyCode.CHAR),
            LegacyKeyDef(":", LegacyKeyCode.CHAR), LegacyKeyDef(";", LegacyKeyCode.CHAR), LegacyKeyDef("/", LegacyKeyCode.CHAR),
            LegacyKeyDef("?", LegacyKeyCode.CHAR),
            LegacyKeyDef("\u2190", LegacyKeyCode.BACKSPACE, 1.5f)
        ),
        listOf(
            LegacyKeyDef("ABC", LegacyKeyCode.ALPHA, 1.5f),
            LegacyKeyDef("\uD83C\uDF10", LegacyKeyCode.GLOBE, 1.0f),
            LegacyKeyDef("\u2261", LegacyKeyCode.GLOBE_PICKER, 1.0f),
            LegacyKeyDef(",", LegacyKeyCode.CHAR, 1.0f),
            LegacyKeyDef(" ", LegacyKeyCode.SPACE, 4.0f),
            LegacyKeyDef(".", LegacyKeyCode.CHAR, 1.0f),
            LegacyKeyDef("\u21B5", LegacyKeyCode.ENTER, 1.5f)
        )
    )

    private val symbols2Rows = listOf(
        listOf(
            LegacyKeyDef("~", LegacyKeyCode.CHAR), LegacyKeyDef("`", LegacyKeyCode.CHAR), LegacyKeyDef("|", LegacyKeyCode.CHAR),
            LegacyKeyDef("\u2022", LegacyKeyCode.CHAR), LegacyKeyDef("\u221A", LegacyKeyCode.CHAR), LegacyKeyDef("\u03C0", LegacyKeyCode.CHAR),
            LegacyKeyDef("\u00F7", LegacyKeyCode.CHAR), LegacyKeyDef("\u00D7", LegacyKeyCode.CHAR), LegacyKeyDef("\u00B6", LegacyKeyCode.CHAR),
            LegacyKeyDef("\u0394", LegacyKeyCode.CHAR)
        ),
        listOf(
            LegacyKeyDef("\u00A3", LegacyKeyCode.CHAR), LegacyKeyDef("\u20AC", LegacyKeyCode.CHAR), LegacyKeyDef("\u00A5", LegacyKeyCode.CHAR),
            LegacyKeyDef("^", LegacyKeyCode.CHAR), LegacyKeyDef("[", LegacyKeyCode.CHAR), LegacyKeyDef("]", LegacyKeyCode.CHAR),
            LegacyKeyDef("{", LegacyKeyCode.CHAR), LegacyKeyDef("}", LegacyKeyCode.CHAR)
        ),
        listOf(
            LegacyKeyDef("?123", LegacyKeyCode.SYMBOLS, 1.5f),
            LegacyKeyDef("\u00A9", LegacyKeyCode.CHAR), LegacyKeyDef("\u00AE", LegacyKeyCode.CHAR), LegacyKeyDef("\u2122", LegacyKeyCode.CHAR),
            LegacyKeyDef("\\", LegacyKeyCode.CHAR), LegacyKeyDef("<", LegacyKeyCode.CHAR), LegacyKeyDef(">", LegacyKeyCode.CHAR),
            LegacyKeyDef("=", LegacyKeyCode.CHAR),
            LegacyKeyDef("\u2190", LegacyKeyCode.BACKSPACE, 1.5f)
        ),
        listOf(
            LegacyKeyDef("ABC", LegacyKeyCode.ALPHA, 1.5f),
            LegacyKeyDef("\uD83C\uDF10", LegacyKeyCode.GLOBE, 1.0f),
            LegacyKeyDef("\u2261", LegacyKeyCode.GLOBE_PICKER, 1.0f),
            LegacyKeyDef(",", LegacyKeyCode.CHAR, 1.0f),
            LegacyKeyDef(" ", LegacyKeyCode.SPACE, 4.0f),
            LegacyKeyDef(".", LegacyKeyCode.CHAR, 1.0f),
            LegacyKeyDef("\u21B5", LegacyKeyCode.ENTER, 1.5f)
        )
    )

    private val keyColor: Int
    private val keyColorSpecial: Int
    private val keyColorPressed: Int
    private val keyTextColor: Int
    private val keyboardBgColor: Int

    init {
        orientation = VERTICAL

        // Detect dark mode via UI_MODE flag (theme attrs are unreliable in IME context)
        val uiMode = context.resources.configuration.uiMode and android.content.res.Configuration.UI_MODE_NIGHT_MASK
        val isDark = uiMode == android.content.res.Configuration.UI_MODE_NIGHT_YES

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

    private fun createKeyView(keyDef: LegacyKeyDef): View {
        val isSpecial = keyDef.code != LegacyKeyCode.CHAR && keyDef.code != LegacyKeyCode.SPACE
        val isShiftActive = keyDef.code == LegacyKeyCode.SHIFT && shiftState != ShiftState.OFF

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
            keyDef.code == LegacyKeyCode.CHAR && currentLayer == 0 && shiftState != ShiftState.OFF ->
                keyDef.label.uppercase()
            keyDef.code == LegacyKeyCode.SPACE -> ""
            keyDef.code == LegacyKeyCode.SHIFT && shiftState == ShiftState.CAPS_LOCK -> "\u2B06\uFE0F"
            else -> keyDef.label
        }

        val textSize = when (keyDef.code) {
            LegacyKeyCode.SYMBOLS, LegacyKeyCode.SYMBOLS2, LegacyKeyCode.ALPHA -> 12f
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

        tv.setOnTouchListener { v, event ->
            when (event.action) {
                MotionEvent.ACTION_DOWN -> {
                    bg.setColor(keyColorPressed)
                    v.invalidate()
                    if (keyDef.code == LegacyKeyCode.CHAR && keyDef.label.length == 1 && keyDef.label != "," && keyDef.label != ".") {
                        showKeyPopup(v, displayLabel)
                    }
                    if (keyDef.code == LegacyKeyCode.BACKSPACE) {
                        handleKeyPress(keyDef)
                        startBackspaceRepeat()
                    }
                    true
                }
                MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> {
                    val restoreColor = when {
                        isShiftActive && keyDef.code == LegacyKeyCode.SHIFT -> keyColorPressed
                        isSpecial -> keyColorSpecial
                        else -> keyColor
                    }
                    bg.setColor(restoreColor)
                    v.invalidate()
                    dismissKeyPopup()
                    if (keyDef.code == LegacyKeyCode.BACKSPACE) {
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

    private fun handleKeyPress(keyDef: LegacyKeyDef) {
        when (keyDef.code) {
            LegacyKeyCode.CHAR -> {
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
            LegacyKeyCode.BACKSPACE -> {
                listener.onBackspace()
            }
            LegacyKeyCode.ENTER -> {
                listener.onEnter()
            }
            LegacyKeyCode.SPACE -> {
                listener.onSpace()
            }
            LegacyKeyCode.SHIFT -> {
                val now = System.currentTimeMillis()
                if (now - lastShiftTap < 300) {
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
            LegacyKeyCode.SYMBOLS -> {
                currentLayer = 1
                buildKeyboard()
            }
            LegacyKeyCode.SYMBOLS2 -> {
                currentLayer = 2
                buildKeyboard()
            }
            LegacyKeyCode.ALPHA -> {
                currentLayer = 0
                buildKeyboard()
            }
            LegacyKeyCode.GLOBE -> {
                listener.onSwitchKeyboard()
            }
            LegacyKeyCode.GLOBE_PICKER -> {
                listener.onSwitchKeyboardLongPress()
            }
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
