package com.draftright.keyboard

import android.app.Activity
import android.content.Intent
import android.graphics.Color
import android.os.Bundle
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.view.Window
import android.view.WindowManager
import android.widget.Button
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.TextView
import android.widget.Toast

/**
 * Receives text selected in any app (via Android's PROCESS_TEXT intent),
 * shows a tone picker, rewrites via backend, returns the rewritten text
 * which Android replaces in-place.
 *
 * No keyboard switching needed — user selects text, taps the toolbar menu,
 * picks "DraftRight", picks a tone.
 */
class ProcessTextActivity : Activity() {

    private lateinit var settings: SharedSettings
    private val client = BackendClient()
    private var loading = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        requestWindowFeature(Window.FEATURE_NO_TITLE)

        // Receive the text Android passed to us
        val incoming = intent.getCharSequenceExtra(Intent.EXTRA_PROCESS_TEXT)?.toString() ?: ""
        val readOnly = intent.getBooleanExtra(Intent.EXTRA_PROCESS_TEXT_READONLY, false)

        if (incoming.isBlank()) {
            Toast.makeText(this, "No text selected", Toast.LENGTH_SHORT).show()
            finish()
            return
        }

        settings = SharedSettings(this)

        if (settings.bearerToken.isEmpty()) {
            Toast.makeText(this, "Please open DraftRight app and sign in first", Toast.LENGTH_LONG).show()
            finish()
            return
        }

        setContentView(buildUI(incoming, readOnly))

        // Style as a bottom sheet that doesn't block the user's view of the source app
        window.setGravity(Gravity.BOTTOM)
        val params = window.attributes
        params.width = WindowManager.LayoutParams.MATCH_PARENT
        params.dimAmount = 0.4f
        window.attributes = params
        window.addFlags(WindowManager.LayoutParams.FLAG_DIM_BEHIND)
    }

    private fun buildUI(text: String, readOnly: Boolean): View {
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setBackgroundColor(Color.parseColor("#FFFFFF"))
            setPadding(dp(16), dp(20), dp(16), dp(20))
        }

        // Header
        val header = TextView(this).apply {
            this.text = "DraftRight — Rewrite"
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 16f)
            setTextColor(Color.parseColor("#202936"))
            setTypeface(typeface, android.graphics.Typeface.BOLD)
        }
        root.addView(header)

        // Preview of selected text (truncated)
        val preview = TextView(this).apply {
            this.text = if (text.length > 120) text.substring(0, 120) + "…" else text
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 12f)
            setTextColor(Color.parseColor("#7c8fac"))
            setPadding(0, dp(8), 0, dp(16))
            maxLines = 3
        }
        root.addView(preview)

        // Tone grid (3 columns)
        val grid = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
        }

        var currentRow: LinearLayout? = null
        val tones = Tone.values()
        for ((index, tone) in tones.withIndex()) {
            if (index % 3 == 0) {
                currentRow = LinearLayout(this).apply {
                    orientation = LinearLayout.HORIZONTAL
                    layoutParams = LinearLayout.LayoutParams(
                        LinearLayout.LayoutParams.MATCH_PARENT,
                        LinearLayout.LayoutParams.WRAP_CONTENT
                    )
                }
                grid.addView(currentRow)
            }

            val btn = Button(this).apply {
                this.text = tone.displayName
                setTextSize(TypedValue.COMPLEX_UNIT_SP, 12f)
                setBackgroundColor(Color.parseColor("#5d87ff"))
                setTextColor(Color.WHITE)
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f).apply {
                    setMargins(dp(4), dp(4), dp(4), dp(4))
                }
                setOnClickListener { startRewrite(text, tone, readOnly) }
            }
            currentRow!!.addView(btn)
        }

        // Pad the last row if needed
        currentRow?.let {
            while (it.childCount < 3) {
                val spacer = View(this).apply {
                    layoutParams = LinearLayout.LayoutParams(0, 1, 1f).apply {
                        setMargins(dp(4), 0, dp(4), 0)
                    }
                }
                it.addView(spacer)
            }
        }

        root.addView(grid)

        // Progress bar (hidden until rewrite starts)
        val spinner = ProgressBar(this).apply {
            visibility = View.GONE
            id = SPINNER_ID
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.WRAP_CONTENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply { gravity = Gravity.CENTER; topMargin = dp(16) }
        }
        root.addView(spinner)

        // Cancel button
        val cancel = Button(this).apply {
            this.text = "Cancel"
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 12f)
            setBackgroundColor(Color.TRANSPARENT)
            setTextColor(Color.parseColor("#7c8fac"))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply { topMargin = dp(8) }
            setOnClickListener {
                setResult(RESULT_CANCELED)
                finish()
            }
        }
        root.addView(cancel)

        return root
    }

    private fun startRewrite(text: String, tone: Tone, readOnly: Boolean) {
        if (loading) return
        loading = true
        findViewById<ProgressBar>(SPINNER_ID)?.visibility = View.VISIBLE

        client.rewrite(text, tone, settings) { result ->
            runOnUiThread {
                loading = false
                result.fold(
                    onSuccess = { rewritten ->
                        if (readOnly) {
                            // Source field is read-only (e.g. system app). Copy to clipboard instead.
                            val cm = getSystemService(CLIPBOARD_SERVICE) as android.content.ClipboardManager
                            cm.setPrimaryClip(android.content.ClipData.newPlainText("DraftRight", rewritten))
                            Toast.makeText(this, "Copied: $rewritten", Toast.LENGTH_LONG).show()
                            setResult(RESULT_CANCELED)
                        } else {
                            // Replace selected text in the source app
                            val out = Intent().apply {
                                putExtra(Intent.EXTRA_PROCESS_TEXT, rewritten)
                            }
                            setResult(RESULT_OK, out)
                        }
                        finish()
                    },
                    onFailure = { err ->
                        Toast.makeText(this, "Error: ${err.message}", Toast.LENGTH_LONG).show()
                        findViewById<ProgressBar>(SPINNER_ID)?.visibility = View.GONE
                    }
                )
            }
        }
    }

    private fun dp(value: Int): Int =
        (value * resources.displayMetrics.density).toInt()

    companion object {
        private const val SPINNER_ID = 0x12345
    }
}
