package com.draftright.keyboard

import android.content.Context
import android.graphics.Color
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.widget.HorizontalScrollView
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.TextView

class ToolbarView(
    context: Context,
    private val onToneSelected: (Tone) -> Unit,
    private val onUndo: () -> Unit
) : LinearLayout(context) {

    private val toneButtons = mutableMapOf<Tone, TextView>()
    private var undoButton: TextView? = null
    private var loadingTone: Tone? = null
    private var spinnerView: ProgressBar? = null

    init {
        orientation = HORIZONTAL
        gravity = Gravity.CENTER_VERTICAL
        setBackgroundColor(resolveAttrColor(android.R.attr.colorBackground))
        val dp44 = dpToPx(44)
        layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, dp44)
        setPadding(dpToPx(4), 0, dpToPx(4), 0)

        val scrollView = HorizontalScrollView(context).apply {
            isHorizontalScrollBarEnabled = false
            layoutParams = LayoutParams(0, LayoutParams.MATCH_PARENT, 1f)
        }

        val row = LinearLayout(context).apply {
            orientation = HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
        }

        for (tone in Tone.values()) {
            val btn = TextView(context).apply {
                text = when (tone) {
                    Tone.SIMPLE -> "\u270E"        // ✎ pencil
                    Tone.NATURAL -> "\uD83D\uDCAC" // 💬 speech bubble
                    Tone.POLISHED -> "\u2728"       // ✨ sparkles
                    Tone.CONCISE -> "\u2296"        // ⊖ compact
                    Tone.TECHNICAL -> "\uD83D\uDD27" // 🔧 wrench
                    Tone.TRANSLATE -> "\uD83C\uDF10" // 🌐 globe
                }
                setTextSize(TypedValue.COMPLEX_UNIT_SP, 18f)
                gravity = Gravity.CENTER
                setBackgroundResource(android.R.drawable.btn_default)
                val dp36 = dpToPx(36)
                layoutParams = LayoutParams(dpToPx(44), dp36).apply {
                    marginEnd = dpToPx(2)
                }
                setOnClickListener { onToneSelected(tone) }
            }
            toneButtons[tone] = btn
            row.addView(btn)
        }

        scrollView.addView(row)
        addView(scrollView)

        // Undo button
        val undo = TextView(context).apply {
            text = "Undo"
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 12f)
            gravity = Gravity.CENTER
            setPadding(dpToPx(8), dpToPx(4), dpToPx(8), dpToPx(4))
            visibility = View.GONE
            setOnClickListener { onUndo() }
        }
        undoButton = undo
        addView(undo)
    }

    fun setLoading(tone: Tone) {
        loadingTone = tone
        isEnabled = false
        toneButtons[tone]?.let { btn ->
            btn.visibility = View.INVISIBLE
            val spinner = ProgressBar(context).apply {
                layoutParams = LayoutParams(dpToPx(24), dpToPx(24))
            }
            spinnerView = spinner
            val parent = btn.parent as? LinearLayout
            val index = parent?.indexOfChild(btn) ?: -1
            if (index >= 0) parent?.addView(spinner, index)
        }
    }

    fun clearLoading() {
        isEnabled = true
        spinnerView?.let { (it.parent as? LinearLayout)?.removeView(it) }
        spinnerView = null
        loadingTone?.let { tone ->
            toneButtons[tone]?.visibility = View.VISIBLE
        }
        loadingTone = null
    }

    fun showUndo() {
        undoButton?.visibility = View.VISIBLE
        postDelayed({ undoButton?.visibility = View.GONE }, 5000)
    }

    fun hideUndo() {
        undoButton?.visibility = View.GONE
    }

    private fun dpToPx(dp: Int): Int =
        TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, dp.toFloat(), resources.displayMetrics).toInt()

    private fun resolveAttrColor(attr: Int): Int {
        val tv = TypedValue()
        return if (context.theme.resolveAttribute(attr, tv, true)) tv.data else Color.WHITE
    }
}
