package com.draftright.keyboard

import android.content.Context
import android.graphics.Color
import android.graphics.drawable.GradientDrawable
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.widget.GridLayout
import android.widget.PopupWindow
import android.widget.TextView
import com.draftright.keyboard.ime.FlickDirection

/**
 * Flick-preview popup (#212, phase 3b): while a flick key is held, show the five
 * reachable characters in a plus/cross above the key (center = tap, arms = the
 * four flick directions) and highlight whichever the finger currently selects —
 * the way Gboard/Samsung フリック previews do.
 *
 * Pure view: it takes the same `resolve: (FlickDirection) -> String?` the key
 * already uses (RULE #1 — the kana mapping stays in FlickLayout, this only
 * renders it), so it never re-encodes the layout and works for every flick key
 * (kana and punctuation alike).
 */
class FlickPreviewPopup(
    private val context: Context,
    private val keyColor: Int,
    private val keyTextColor: Int,
    private val brand: Int,
) {
    private val CELL_DP = 40
    private val CELL_TEXT_SP = 20f
    private val CELL_MARGIN_DP = 2
    private val RADIUS_DP = 6

    // Grid cell (row, col) for each direction; the plus leaves the corners empty.
    private val slots: Map<FlickDirection, Pair<Int, Int>> = mapOf(
        FlickDirection.UP to (0 to 1),
        FlickDirection.LEFT to (1 to 0),
        FlickDirection.TAP to (1 to 1),
        FlickDirection.RIGHT to (1 to 2),
        FlickDirection.DOWN to (2 to 1),
    )

    private var popup: PopupWindow? = null
    private val cells = mutableMapOf<FlickDirection, TextView>()

    /** Show the preview above [anchor], populated from [resolve]. */
    fun show(anchor: View, resolve: (FlickDirection) -> String?) {
        dismiss() // drop any popup a prior press left up (missed UP/CANCEL)
        val cell = dpToPx(CELL_DP)
        val grid = GridLayout(context).apply {
            rowCount = 3; columnCount = 3
            setBackgroundColor(Color.TRANSPARENT)
        }
        cells.clear()
        for ((dir, rc) in slots) {
            val text = resolve(dir) ?: continue
            val tv = makeCell(text)
            grid.addView(tv, GridLayout.LayoutParams(
                GridLayout.spec(rc.first), GridLayout.spec(rc.second)
            ).apply {
                width = cell; height = cell
                setMargins(dpToPx(CELL_MARGIN_DP), dpToPx(CELL_MARGIN_DP), dpToPx(CELL_MARGIN_DP), dpToPx(CELL_MARGIN_DP))
            })
            cells[dir] = tv
        }
        highlight(FlickDirection.TAP)

        val pw = PopupWindow(grid, WRAP, WRAP, false).apply {
            isClippingEnabled = false
            isTouchable = false
        }
        popup = pw

        // Center horizontally over the key, float just above it.
        val loc = IntArray(2)
        anchor.getLocationInWindow(loc)
        val popW = cell * 3 + dpToPx(CELL_MARGIN_DP) * 6
        val popH = popW
        val x = loc[0] + anchor.width / 2 - popW / 2
        val y = loc[1] - popH
        pw.showAtLocation(anchor, Gravity.NO_GRAVITY, x, y)
    }

    /** Highlight the cell for [dir] (falling back to TAP when that arm is empty). */
    fun update(dir: FlickDirection) {
        highlight(if (cells.containsKey(dir)) dir else FlickDirection.TAP)
    }

    fun dismiss() {
        popup?.dismiss()
        popup = null
        cells.clear()
    }

    private fun highlight(active: FlickDirection) {
        for ((dir, tv) in cells) {
            val on = dir == active
            (tv.background as GradientDrawable).setColor(if (on) brand else keyColor)
            tv.setTextColor(if (on) Color.WHITE else keyTextColor)
        }
    }

    private fun makeCell(text: String): TextView {
        val bg = GradientDrawable().apply {
            setColor(keyColor)
            cornerRadius = dpToPx(RADIUS_DP).toFloat()
        }
        return TextView(context).apply {
            this.text = text
            gravity = Gravity.CENTER
            setTextSize(TypedValue.COMPLEX_UNIT_SP, CELL_TEXT_SP)
            setTextColor(keyTextColor)
            background = bg
        }
    }

    private fun dpToPx(dp: Int): Int =
        TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, dp.toFloat(), context.resources.displayMetrics).toInt()

    private companion object {
        const val WRAP = android.view.ViewGroup.LayoutParams.WRAP_CONTENT
    }
}
