package com.draftright.keyboard

import android.content.Context
import android.graphics.Color
import android.graphics.drawable.GradientDrawable
import android.util.TypedValue
import android.view.View
import android.widget.HorizontalScrollView
import android.widget.LinearLayout
import android.widget.TextView

class LanguageStripView(
    context: Context,
    private val controller: KeyboardController,
    private val onLanguageChanged: () -> Unit,
) : HorizontalScrollView(context) {

    private val container = LinearLayout(context).apply {
        orientation = LinearLayout.HORIZONTAL
        setPadding(dp(8), dp(6), dp(8), dp(6))
    }

    init {
        isHorizontalScrollBarEnabled = false
        addView(container)
        refresh()
    }

    fun refresh() {
        container.removeAllViews()
        visibility = if (controller.enabled.size <= 1) View.GONE else View.VISIBLE
        if (visibility == View.GONE) return

        val uiMode = context.resources.configuration.uiMode and android.content.res.Configuration.UI_MODE_NIGHT_MASK
        val isDark = uiMode == android.content.res.Configuration.UI_MODE_NIGHT_YES
        val inactiveText = if (isDark) Color.parseColor("#cbd5e1") else Color.parseColor("#475569")
        val inactiveBg = if (isDark) Color.parseColor("#2a2a2e") else Color.parseColor("#e2e8f0")

        controller.enabled.forEach { pack ->
            val isActive = pack.id == controller.current.id
            val chipBg = GradientDrawable().apply {
                setColor(if (isActive) Color.parseColor("#3b82f6") else inactiveBg)
                cornerRadius = dp(14).toFloat()
            }
            val chip = TextView(context).apply {
                text = pack.displayName
                setTextSize(TypedValue.COMPLEX_UNIT_SP, 12f)
                setPadding(dp(12), dp(6), dp(12), dp(6))
                setTextColor(if (isActive) Color.WHITE else inactiveText)
                background = chipBg
                isClickable = true
                isFocusable = true
                setOnClickListener {
                    controller.setActive(pack.id)
                    refresh()
                    onLanguageChanged()
                }
            }
            val params = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.WRAP_CONTENT,
                LinearLayout.LayoutParams.WRAP_CONTENT,
            ).apply { marginEnd = dp(8) }
            container.addView(chip, params)
        }
    }

    private fun dp(value: Int): Int =
        TypedValue.applyDimension(
            TypedValue.COMPLEX_UNIT_DIP,
            value.toFloat(),
            resources.displayMetrics,
        ).toInt()
}
