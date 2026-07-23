package com.draftright.spike

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.provider.Settings
import android.view.Gravity
import android.view.View
import android.widget.Button
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity

/**
 * Setup + results screen. No XML layout — programmatic so the whole spike is a
 * handful of files. Grant overlay + accessibility, start the bubble, then go
 * test other apps; come back here (or `adb logcat -s SPIKE`) to read results.
 */
class MainActivity : AppCompatActivity() {

    private lateinit var logView: TextView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val pad = (16 * resources.displayMetrics.density).toInt()
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(pad, pad, pad, pad)
        }

        fun button(label: String, onClick: () -> Unit) = Button(this).apply {
            text = label
            setOnClickListener { onClick() }
            root.addView(this)
        }

        TextView(this).apply {
            text = "A11y Rewrite Spike\nTap a field in another app, tap the bubble → text should UPPERCASE in place."
            root.addView(this)
        }

        button("1. Grant overlay permission") {
            startActivity(
                Intent(
                    Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                    Uri.parse("package:$packageName")
                )
            )
        }
        button("2. Open Accessibility settings → enable 'A11y Rewrite Spike'") {
            startActivity(Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS))
        }
        button("3. Start bubble") {
            startService(Intent(this, OverlayBubbleService::class.java).apply {
                action = OverlayBubbleService.ACTION_START
            })
        }
        button("Stop bubble") {
            startService(Intent(this, OverlayBubbleService::class.java).apply {
                action = OverlayBubbleService.ACTION_STOP
            })
        }
        button("Refresh log") { refreshLog() }
        button("Clear log") { SpikeLog.clear(this); refreshLog() }

        val scroll = ScrollView(this)
        logView = TextView(this).apply {
            setTextIsSelectable(true)
            textSize = 12f
            typeface = android.graphics.Typeface.MONOSPACE
        }
        scroll.addView(logView)
        root.addView(scroll, LinearLayout.LayoutParams(
            LinearLayout.LayoutParams.MATCH_PARENT, 0, 1f
        ))

        setContentView(root)
    }

    override fun onResume() {
        super.onResume()
        refreshLog()
    }

    private fun refreshLog() {
        val text = SpikeLog.read(this)
        logView.text = if (text.isBlank()) "(no results yet)" else text
    }
}
