package com.draftright.keyboard

import android.content.Context
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.util.TypedValue
import android.view.Gravity
import android.view.MotionEvent
import android.view.View
import android.widget.LinearLayout
import android.widget.TextView
import com.draftright.keyboard.ime.FlickGesture
import com.draftright.keyboard.ime.FlickLayout

/**
 * Japanese 12-key flick (フリック) keyboard (#212, phase 2). Each kana key emits
 * the tap kana on a tap and the row's vowel on a flick — resolution comes from
 * the pure [FlickGesture] + [FlickLayout] primitives (phase 1), so this view only
 * owns the touch plumbing + rendering. The resolved kana goes to the IME via
 * [KeyboardActionListener.onCharTyped], feeding the existing kana→kanji engine.
 *
 * Minimal for phase 2: no flick-preview popup yet (phase 3), and 小゛゜/punctuation
 * keys are deferred to phase 3; this ships the core gojūon flick input.
 */
class FlickKeyboardView(
    context: Context,
    private val listener: KeyboardActionListener,
) : LinearLayout(context) {

    private val ROW_HEIGHT_DP = 52
    private val KEY_MARGIN_DP = 3
    private val KEY_RADIUS_DP = 6
    private val KANA_TEXT_SP = 22f

    // A flick counts once travel passes this; below it, the touch is a tap.
    private val flickThresholdPx: Float =
        dpToPx(18).toFloat()

    // The 12-key kana grid (row-head kana). Function keys live in the bottom row.
    private val kanaRows: List<List<String>> = listOf(
        listOf("あ", "か", "さ"),
        listOf("た", "な", "は"),
        listOf("ま", "や", "ら"),
        listOf("＾_＾", "わ", "、。"), // placeholders (modifier / punctuation) — phase 3
    )

    private val keyColor: Int
    private val keyColorSpecial: Int
    private val keyColorPressed: Int
    private val keyTextColor: Int
    private val bgColor: Int
    private val brand = Color.parseColor(KeyboardTheme.BRAND_BLUE)

    init {
        orientation = VERTICAL
        val dark = KeyboardTheme.isDark(context)
        if (dark) {
            bgColor = Color.parseColor("#1B1B1F"); keyColor = Color.parseColor("#4A4A4A")
            keyColorSpecial = Color.parseColor("#363636"); keyColorPressed = Color.parseColor("#5A5A5A")
            keyTextColor = Color.WHITE
        } else {
            bgColor = Color.parseColor("#ECEFF1"); keyColor = Color.WHITE
            keyColorSpecial = Color.parseColor("#B0BEC5"); keyColorPressed = Color.parseColor("#D6D6D6")
            keyTextColor = Color.parseColor("#212121")
        }
        setBackgroundColor(bgColor)
        build()
    }

    private fun build() {
        removeAllViews()
        for (row in kanaRows) {
            val rl = rowLayout()
            for (rowHead in row) rl.addView(kanaKey(rowHead), keyParams(1f))
            addView(rl)
        }
        // Bottom function row: 🌐 | space (wide) | ⌫ | ↵
        val fn = rowLayout()
        fn.addView(functionKey("🌐") { listener.onSwitchKeyboard() }, keyParams(1f))
        fn.addView(functionKey("␣") { listener.onSpace() }, keyParams(2f))
        fn.addView(functionKey("⌫") { listener.onBackspace() }, keyParams(1f))
        fn.addView(functionKey("↵") { listener.onEnter() }, keyParams(1f))
        addView(fn)
    }

    private fun rowLayout() = LinearLayout(context).apply {
        orientation = HORIZONTAL
        gravity = Gravity.CENTER
        layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, dpToPx(ROW_HEIGHT_DP))
    }

    private fun keyParams(weight: Float) = LayoutParams(0, LayoutParams.MATCH_PARENT, weight).apply {
        setMargins(dpToPx(KEY_MARGIN_DP), dpToPx(KEY_MARGIN_DP), dpToPx(KEY_MARGIN_DP), dpToPx(KEY_MARGIN_DP))
    }

    private fun baseKey(label: String, special: Boolean): TextView {
        val bg = GradientDrawable().apply {
            setColor(if (special) keyColorSpecial else keyColor)
            cornerRadius = dpToPx(KEY_RADIUS_DP).toFloat()
        }
        return TextView(context).apply {
            text = label
            setTextSize(TypedValue.COMPLEX_UNIT_SP, KANA_TEXT_SP)
            setTextColor(if (special) brand else keyTextColor)
            gravity = Gravity.CENTER
            typeface = Typeface.DEFAULT
            background = bg
            isClickable = true
        }
    }

    /** A kana key: tap → row-head kana, flick → the direction's kana. */
    private fun kanaKey(rowHead: String): TextView {
        val key = baseKey(rowHead, special = false)
        val bg = key.background as GradientDrawable
        var startX = 0f
        var startY = 0f
        key.setOnTouchListener { _, e ->
            when (e.action) {
                MotionEvent.ACTION_DOWN -> {
                    startX = e.rawX; startY = e.rawY
                    bg.setColor(keyColorPressed); key.invalidate(); true
                }
                MotionEvent.ACTION_UP -> {
                    bg.setColor(keyColor); key.invalidate()
                    val dir = FlickGesture.resolve(e.rawX - startX, e.rawY - startY, flickThresholdPx)
                    // Fall back to the tap kana when the flicked direction has none (e.g. や←).
                    val kana = FlickLayout.kanaFor(rowHead, dir)
                        ?: FlickLayout.kanaFor(rowHead, com.draftright.keyboard.ime.FlickDirection.TAP)
                    if (kana != null) listener.onCharTyped(kana)
                    true
                }
                MotionEvent.ACTION_CANCEL -> { bg.setColor(keyColor); key.invalidate(); true }
                else -> false
            }
        }
        return key
    }

    private fun functionKey(label: String, onTap: () -> Unit): TextView {
        val key = baseKey(label, special = true)
        val bg = key.background as GradientDrawable
        key.setOnTouchListener { _, e ->
            when (e.action) {
                MotionEvent.ACTION_DOWN -> { bg.setColor(keyColorPressed); key.invalidate(); true }
                MotionEvent.ACTION_UP -> { bg.setColor(keyColorSpecial); key.invalidate(); onTap(); true }
                MotionEvent.ACTION_CANCEL -> { bg.setColor(keyColorSpecial); key.invalidate(); true }
                else -> false
            }
        }
        return key
    }

    private fun dpToPx(dp: Int): Int =
        TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, dp.toFloat(), resources.displayMetrics).toInt()
}
