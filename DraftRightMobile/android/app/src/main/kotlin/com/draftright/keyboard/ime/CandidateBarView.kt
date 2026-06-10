package com.draftright.keyboard.ime

import android.content.Context
import android.graphics.Color
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.widget.HorizontalScrollView
import android.widget.LinearLayout
import android.widget.TextView

/**
 * Horizontal scroll strip that renders suggestion chips above the AI tone
 * toolbar. Engine-agnostic: it just receives a list of [Candidate]s and
 * calls back when the user taps one. The same view will render Vietnamese
 * trigram completions today and Japanese RIME candidates tomorrow.
 *
 * No XML — programmatic, consistent with the rest of the keyboard. Colors
 * follow the existing toolbar palette so the strip blends visually.
 *
 * Caller wires it via [setCandidates] (on every keystroke) and listens
 * for [onCandidatePicked]. An empty list collapses the strip to invisible
 * so it doesn't reserve vertical space when there's nothing to suggest.
 */
class CandidateBarView(context: Context) : HorizontalScrollView(context) {

    /** Invoked when the user taps a chip. Caller commits + resets composer. */
    var onCandidatePicked: ((Candidate) -> Unit)? = null

    private val row = LinearLayout(context).apply {
        orientation = LinearLayout.HORIZONTAL
        gravity = Gravity.CENTER_VERTICAL
        setPadding(dp(6), dp(2), dp(6), dp(2))
    }

    init {
        isHorizontalScrollBarEnabled = false
        setBackgroundColor(BG_COLOR)
        addView(
            row,
            LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT)
        )
        visibility = View.GONE  // hidden until first non-empty update
    }

    /**
     * Replace the visible chips. Empty list hides the bar entirely so the
     * keyboard reclaims the vertical real estate.
     */
    fun setCandidates(candidates: List<Candidate>) {
        row.removeAllViews()
        if (candidates.isEmpty()) {
            visibility = View.GONE
            return
        }
        visibility = View.VISIBLE
        scrollX = 0
        for (cand in candidates) {
            row.addView(buildChip(cand))
        }
    }

    private fun buildChip(candidate: Candidate): TextView {
        val tv = TextView(context).apply {
            text = candidate.display
            setTextColor(TEXT_COLOR)
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 14f)
            setPadding(dp(14), dp(8), dp(14), dp(8))
            isClickable = true
            isFocusable = true
            background = makeChipBackground()
            setOnClickListener {
                onCandidatePicked?.invoke(candidate)
            }
        }
        val lp = LinearLayout.LayoutParams(
            LinearLayout.LayoutParams.WRAP_CONTENT,
            LinearLayout.LayoutParams.WRAP_CONTENT
        )
        lp.marginStart = dp(4)
        lp.marginEnd = dp(4)
        tv.layoutParams = lp
        return tv
    }

    private fun makeChipBackground(): android.graphics.drawable.Drawable {
        val gd = android.graphics.drawable.GradientDrawable()
        gd.shape = android.graphics.drawable.GradientDrawable.RECTANGLE
        gd.cornerRadius = dp(14).toFloat()
        gd.setColor(CHIP_COLOR)
        gd.setStroke(dp(1), CHIP_BORDER)
        return gd
    }

    private fun dp(v: Int): Int =
        (v * context.resources.displayMetrics.density).toInt()

    companion object {
        // Palette matches ToolbarView's dark tokens so the strip doesn't
        // visually float — same hex values used there for backgrounds.
        private val BG_COLOR = Color.parseColor("#1E293B")        // slate-800
        private val CHIP_COLOR = Color.parseColor("#334155")      // slate-700
        private val CHIP_BORDER = Color.parseColor("#475569")     // slate-600
        private val TEXT_COLOR = Color.parseColor("#E2E8F0")      // slate-200
    }
}
