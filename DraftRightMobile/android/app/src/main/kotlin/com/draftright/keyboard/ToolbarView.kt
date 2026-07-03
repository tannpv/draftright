package com.draftright.keyboard

import android.content.Context
import android.content.res.ColorStateList
import android.graphics.Color
import android.util.TypedValue
import android.view.Gravity
import android.view.MotionEvent
import android.view.View
import android.view.animation.AlphaAnimation
import android.view.animation.Animation
import android.widget.HorizontalScrollView
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.TextView
import com.draftright.keyboard.voice.VoiceSessionController

class ToolbarView(
    context: Context,
    private val onToneSelected: (Tone) -> Unit,
    private val onUndo: () -> Unit,
    /**
     * Null hides the mic button entirely. IME passes null when the active
     * pack has no [LanguagePack.sttLocale] or the device has no speech
     * recognizer (see DraftRightIME.onCreateInputView) — Rule #1: the
     * capability check lives once, at the call site, not duplicated here.
     * `true` argument = raw mode (long-press), `false` = polished (tap).
     */
    private val onMicTapped: ((Boolean) -> Unit)? = null
) : LinearLayout(context) {

    private val toneButtons = mutableMapOf<Tone, View>()
    private var undoButton: TextView? = null
    private var loadingTone: Tone? = null
    private var spinnerView: ProgressBar? = null
    private var micButton: View? = null
    private var micSpinner: ProgressBar? = null

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

        // Mic button — only built when the IME actually offers voice input
        // for the active language on this device (see class doc above).
        onMicTapped?.let { tapped ->
            val mic = ImageView(context).apply {
                setImageResource(KeyIcons.resolve("mic"))
                imageTintList = iconTint
                scaleType = ImageView.ScaleType.CENTER_INSIDE
                setBackgroundResource(android.R.drawable.btn_default)
                val pad = dpToPx(9)
                setPadding(pad, pad, pad, pad)
                layoutParams = LayoutParams(dpToPx(44), dpToPx(40)).apply {
                    marginEnd = dpToPx(2)
                }
                contentDescription = "Voice input"
            }

            // Same tap-vs-long-press mechanism as QwertyKeyboardView's key
            // touch handler (reused, not reinvented): a postDelayed runnable
            // armed on ACTION_DOWN and cancelled on ACTION_UP/CANCEL. Firing
            // the runnable flags longPressFired so ACTION_UP doesn't also
            // fire the tap action.
            var longPressFired = false
            val longPressRunnable = Runnable {
                longPressFired = true
                tapped(true) // raw mode
            }
            mic.setOnTouchListener { v, event ->
                when (event.action) {
                    MotionEvent.ACTION_DOWN -> {
                        longPressFired = false
                        v.postDelayed(longPressRunnable, SpecialKeys.LONG_PRESS_MS)
                        true
                    }
                    MotionEvent.ACTION_UP -> {
                        v.removeCallbacks(longPressRunnable)
                        if (!longPressFired) tapped(false) // polished mode
                        true
                    }
                    MotionEvent.ACTION_CANCEL -> {
                        v.removeCallbacks(longPressRunnable)
                        true
                    }
                    else -> false
                }
            }

            micButton = mic
            row.addView(mic)
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

    /**
     * Drives the mic button's visual state (mirrors [setLoading]/[clearLoading]
     * for the spinner half). LISTENING pulses the mic icon so it's obvious a
     * recording is live; PROCESSING swaps it for a spinner (AI-polish call in
     * flight); IDLE restores the plain icon.
     *
     * Also locks the tone buttons individually via [setToneButtonsEnabled] for
     * LISTENING/PROCESSING, and restores them for IDLE. Without this, a tap on
     * a tone button during an active voice session fires a rewrite call that
     * races the voice commit for the same InputConnection. Note: the mic
     * button itself is intentionally left enabled in every state — tapping it
     * mid-session is the spec's cancel gesture (see DraftRightIME.handleMicTapped).
     */
    fun setVoiceState(state: VoiceSessionController.State) {
        val mic = micButton ?: return
        when (state) {
            VoiceSessionController.State.IDLE -> {
                setToneButtonsEnabled(true)
                mic.clearAnimation()
                micSpinner?.let { (it.parent as? LinearLayout)?.removeView(it) }
                micSpinner = null
                mic.visibility = View.VISIBLE
            }
            VoiceSessionController.State.LISTENING -> {
                setToneButtonsEnabled(false)
                micSpinner?.let { (it.parent as? LinearLayout)?.removeView(it) }
                micSpinner = null
                mic.visibility = View.VISIBLE
                mic.startAnimation(
                    AlphaAnimation(1f, 0.3f).apply {
                        duration = 500
                        repeatMode = Animation.REVERSE
                        repeatCount = Animation.INFINITE
                    }
                )
            }
            VoiceSessionController.State.PROCESSING -> {
                setToneButtonsEnabled(false)
                mic.clearAnimation()
                mic.visibility = View.INVISIBLE
                val spinner = ProgressBar(context).apply {
                    layoutParams = LayoutParams(dpToPx(24), dpToPx(24))
                }
                micSpinner = spinner
                val parent = mic.parent as? LinearLayout
                val index = parent?.indexOfChild(mic) ?: -1
                if (index >= 0) parent?.addView(spinner, index)
            }
        }
    }

    /**
     * Toggles each tone button's own [View.isEnabled] rather than the
     * container's — a disabled ViewGroup does NOT stop child views from
     * receiving touches/click callbacks on Android, so setting `isEnabled`
     * on `this` (as the previous fix attempted) never actually blocked tone
     * taps during a voice session. A disabled child View, by contrast, does
     * suppress its own `setOnClickListener`.
     */
    private fun setToneButtonsEnabled(enabled: Boolean) {
        for (btn in toneButtons.values) {
            btn.isEnabled = enabled
        }
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
