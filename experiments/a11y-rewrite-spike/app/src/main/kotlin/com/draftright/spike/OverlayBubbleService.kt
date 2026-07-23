package com.draftright.spike

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.graphics.Color
import android.graphics.PixelFormat
import android.graphics.drawable.GradientDrawable
import android.os.Build
import android.os.IBinder
import android.util.TypedValue
import android.view.Gravity
import android.view.MotionEvent
import android.view.View
import android.view.WindowManager
import android.widget.TextView
import kotlin.math.abs

/**
 * Foreground service hosting a small draggable overlay bubble. A tap asks the
 * AccessibilityService to rewrite the currently focused field. The window is
 * FLAG_NOT_FOCUSABLE so tapping the bubble does NOT steal input focus from the
 * target text box — otherwise findFocus would return the bubble, not the field.
 */
class OverlayBubbleService : Service() {

    companion object {
        const val ACTION_START = "com.draftright.spike.START"
        const val ACTION_STOP = "com.draftright.spike.STOP"
        private const val CHANNEL_ID = "spike_bubble"
        private const val NOTIF_ID = 42
    }

    private lateinit var wm: WindowManager
    private var bubble: View? = null
    private lateinit var lp: WindowManager.LayoutParams

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> { removeBubble(); stopSelf() }
            else -> { startForegroundNotif(); showBubble() }
        }
        return START_STICKY
    }

    private fun startForegroundNotif() {
        val nm = getSystemService(NotificationManager::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            nm.createNotificationChannel(
                NotificationChannel(CHANNEL_ID, "Spike bubble", NotificationManager.IMPORTANCE_LOW)
            )
        }
        val n: Notification = Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("A11y rewrite spike")
            .setContentText("Bubble active — tap it over a text field")
            .setSmallIcon(android.R.drawable.ic_menu_edit)
            .build()
        startForeground(NOTIF_ID, n)
    }

    private fun dp(v: Float) =
        TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, v, resources.displayMetrics)

    private fun showBubble() {
        if (bubble != null) return

        val size = dp(56f).toInt()
        val view = TextView(this).apply {
            text = "AA"
            setTextColor(Color.WHITE)
            gravity = Gravity.CENTER
            background = GradientDrawable().apply {
                shape = GradientDrawable.OVAL
                setColor(Color.parseColor("#6C4CF1"))
            }
        }

        lp = WindowManager.LayoutParams(
            size, size,
            WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY,
            // NOT_FOCUSABLE is the load-bearing flag — keeps input focus on the
            // target field so the a11y service can find it.
            WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                WindowManager.LayoutParams.FLAG_NOT_TOUCH_MODAL,
            PixelFormat.TRANSLUCENT
        ).apply {
            gravity = Gravity.TOP or Gravity.START
            x = dp(24f).toInt()
            y = dp(160f).toInt()
        }

        view.setOnTouchListener(DragTapListener())
        wm = getSystemService(WindowManager::class.java)
        wm.addView(view, lp)
        bubble = view
    }

    private fun removeBubble() {
        bubble?.let { runCatching { wm.removeView(it) } }
        bubble = null
    }

    override fun onDestroy() {
        removeBubble()
        super.onDestroy()
    }

    /** Distinguishes a tap (trigger) from a drag (reposition). */
    private inner class DragTapListener : View.OnTouchListener {
        private var startX = 0f
        private var startY = 0f
        private var startLpX = 0
        private var startLpY = 0
        private var moved = false
        private val slop = dp(8f)

        override fun onTouch(v: View, e: MotionEvent): Boolean {
            when (e.action) {
                MotionEvent.ACTION_DOWN -> {
                    startX = e.rawX; startY = e.rawY
                    startLpX = lp.x; startLpY = lp.y
                    moved = false
                }
                MotionEvent.ACTION_MOVE -> {
                    val dx = e.rawX - startX
                    val dy = e.rawY - startY
                    if (abs(dx) > slop || abs(dy) > slop) moved = true
                    lp.x = startLpX + dx.toInt()
                    lp.y = startLpY + dy.toInt()
                    runCatching { wm.updateViewLayout(v, lp) }
                }
                MotionEvent.ACTION_UP -> {
                    if (!moved) {
                        val svc = RewriteAccessibilityService.instance
                        if (svc == null) {
                            SpikeLog.add(this@OverlayBubbleService,
                                "TAP → a11y service NOT enabled (enable it in Accessibility settings)")
                        } else {
                            svc.rewriteFocusedField()
                        }
                    }
                }
            }
            return true
        }
    }
}
