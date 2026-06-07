package com.draftright.keyboard

import android.content.Context
import android.content.res.ColorStateList
import android.graphics.Color
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.widget.HorizontalScrollView
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.TextView

class ToolbarView(
    context: Context,
    private val onToneSelected: (Tone) -> Unit,
    private val onUndo: () -> Unit
) : LinearLayout(context) {

    private val toneButtons = mutableMapOf<Tone, View>()
    private var undoButton: TextView? = null
    private var loadingTone: Tone? = null
    private var spinnerView: ProgressBar? = null

    init {
        orientation = HORIZONTAL
        gravity = Gravity.CENTER_VERTICAL
        // Explicit colors — theme attrs are unreliable inside InputMethodService,
        // which previously left the tinted tone icons invisible against the bar.
        val isDark = KeyboardTheme.isDark(context)
        val barBgColor = if (isDark) Color.parseColor("#1B1B1F") else Color.parseColor("#ECEFF1")
        // Tone icons use the DraftRight brand blue (matches the website primary).
        val iconColor = Color.parseColor(KeyboardTheme.BRAND_BLUE)
        setBackgroundColor(barBgColor)
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

        // Each tone renders its Material vector icon (Tone.iconRes) tinted to the
        // theme text color — see KeyIcons for the name -> drawable mapping.
        val iconTint = ColorStateList.valueOf(iconColor)
        for (tone in Tone.values()) {
            val btn = ImageView(context).apply {
                setImageResource(KeyIcons.resolve(tone.iconRes))
                imageTintList = iconTint
                scaleType = ImageView.ScaleType.CENTER_INSIDE
                setBackgroundResource(android.R.drawable.btn_default)
                val pad = dpToPx(9)
                setPadding(pad, pad, pad, pad)
                layoutParams = LayoutParams(dpToPx(44), dpToPx(40)).apply {
                    marginEnd = dpToPx(2)
                }
                contentDescription = tone.displayName
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
}
