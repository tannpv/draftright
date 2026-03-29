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
            KeyDef("\u2B06", KeyCode.SHIFT, 1.5f),
            KeyDef("z", KeyCode.CHAR), KeyDef("x", KeyCode.CHAR), KeyDef("c", KeyCode.CHAR),
            KeyDef("v", KeyCode.CHAR), KeyDef("b", KeyCode.CHAR), KeyDef("n", KeyCode.CHAR),
            KeyDef("m", KeyCode.CHAR),
            KeyDef("\u2190", KeyCode.BACKSPACE, 1.5f)
        ),
        listOf(
            KeyDef("?123", KeyCode.SYMBOLS, 1.5f),
            KeyDef("\uD83C\uDF10", KeyCode.GLOBE, 1.0f),
            KeyDef(",", KeyCode.CHAR, 1.0f),
            KeyDef(" ", KeyCode.SPACE, 5.0f),
            KeyDef(".", KeyCode.CHAR, 1.0f),
            KeyDef("\u21B5", KeyCode.ENTER, 1.5f)
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
            KeyDef("\u2190", KeyCode.BACKSPACE, 1.5f)
        ),
        listOf(
            KeyDef("ABC", KeyCode.ALPHA, 1.5f),
            KeyDef("\uD83C\uDF10", KeyCode.GLOBE, 1.0f),
            KeyDef(",", KeyCode.CHAR, 1.0f),
            KeyDef(" ", KeyCode.SPACE, 5.0f),
            KeyDef(".", KeyCode.CHAR, 1.0f),
            KeyDef("\u21B5", KeyCode.ENTER, 1.5f)
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
            KeyDef("\u2190", KeyCode.BACKSPACE, 1.5f)
        ),
        listOf(
            KeyDef("ABC", KeyCode.ALPHA, 1.5f),
            KeyDef("\uD83C\uDF10", KeyCode.GLOBE, 1.0f),
            KeyDef(",", KeyCode.CHAR, 1.0f),
            KeyDef(" ", KeyCode.SPACE, 5.0f),
            KeyDef(".", KeyCode.CHAR, 1.0f),
            KeyDef("\u21B5", KeyCode.ENTER, 1.5f)
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
            keyDef.code == KeyCode.SHIFT && shiftState == ShiftState.CAPS_LOCK -> "\u2B06\uFE0F"
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
