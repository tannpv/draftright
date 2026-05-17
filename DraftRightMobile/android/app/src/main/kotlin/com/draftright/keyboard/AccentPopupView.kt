package com.draftright.keyboard

import android.content.Context
import android.graphics.Color
import android.graphics.drawable.GradientDrawable
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.widget.LinearLayout
import android.widget.PopupWindow
import android.widget.TextView

class AccentPopupView(
    private val context: Context,
    private val anchor: View,
    options: List<Char>,
    private val onPicked: (Char) -> Unit,
) {
    private val container = LinearLayout(context).apply {
        orientation = LinearLayout.HORIZONTAL
        gravity = Gravity.CENTER_VERTICAL
        background = GradientDrawable().apply {
            setColor(Color.parseColor("#1d1d1f"))
            cornerRadius = dp(8).toFloat()
        }
        val pad = dp(6)
        setPadding(pad, pad, pad, pad)
    }
    private val popup: PopupWindow

    init {
        options.forEach { ch ->
            val tv = TextView(context).apply {
                text = ch.toString()
                setTextSize(TypedValue.COMPLEX_UNIT_SP, 20f)
                setTextColor(Color.WHITE)
                gravity = Gravity.CENTER
                setPadding(dp(14), dp(10), dp(14), dp(10))
                isClickable = true
                isFocusable = true
                setOnClickListener {
                    onPicked(ch)
                    dismiss()
                }
            }
            container.addView(tv)
        }
        popup = PopupWindow(
            container,
            LinearLayout.LayoutParams.WRAP_CONTENT,
            LinearLayout.LayoutParams.WRAP_CONTENT,
            true,
        ).apply { isClippingEnabled = false }
    }

    fun show() {
        container.measure(View.MeasureSpec.UNSPECIFIED, View.MeasureSpec.UNSPECIFIED)
        val yOffset = -(anchor.height + container.measuredHeight + dp(4))
        popup.showAsDropDown(anchor, 0, yOffset)
    }

    fun dismiss() {
        if (popup.isShowing) popup.dismiss()
    }

    private fun dp(value: Int): Int =
        TypedValue.applyDimension(
            TypedValue.COMPLEX_UNIT_DIP,
            value.toFloat(),
            context.resources.displayMetrics,
        ).toInt()
}
