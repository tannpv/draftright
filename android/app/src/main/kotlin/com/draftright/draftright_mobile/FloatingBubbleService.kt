package com.draftright.draftright_mobile

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.SharedPreferences
import android.content.pm.ServiceInfo
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
import android.widget.FrameLayout
import android.widget.TextView
import kotlin.math.abs

/**
 * Tier 1 floating bubble. Foreground service hosts a small draggable
 * overlay window. Tap → launches MainActivity with the current clipboard
 * text, which routes to the share-rewrite screen via the existing
 * `draftright/share` method channel. No Accessibility Service involved.
 *
 * Lifecycle:
 *   START_BUBBLE  → create overlay if not present, watch clipboard
 *   STOP_BUBBLE   → remove overlay, stop service
 *
 * The clipboard listener fires when text is copied in any app while the
 * service is alive. We pulse the bubble briefly to suggest "tap to rewrite."
 */
class FloatingBubbleService : Service() {

    companion object {
        const val ACTION_START = "com.draftright.bubble.START"
        const val ACTION_STOP  = "com.draftright.bubble.STOP"
        private const val CHANNEL_ID = "draftright_bubble"
        private const val NOTIFICATION_ID = 4711
        private const val PREFS = "draftright_bubble_prefs"
        private const val KEY_X = "x"
        private const val KEY_Y = "y"
    }

    private var bubbleView: View? = null
    private var clipboardListener: ClipboardManager.OnPrimaryClipChangedListener? = null
    private val params = WindowManager.LayoutParams()

    private val windowManager by lazy { getSystemService(WINDOW_SERVICE) as WindowManager }
    private val clipboard by lazy { getSystemService(CLIPBOARD_SERVICE) as ClipboardManager }
    private val prefs: SharedPreferences by lazy {
        getSharedPreferences(PREFS, Context.MODE_PRIVATE)
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> { stopBubble(); return START_NOT_STICKY }
            else -> startBubble()
        }
        return START_STICKY
    }

    private fun startBubble() {
        startInForeground()
        if (bubbleView == null) {
            bubbleView = buildBubbleView()
            attachToWindow(bubbleView!!)
        }
        registerClipboardListener()
    }

    private fun stopBubble() {
        bubbleView?.let { runCatching { windowManager.removeView(it) } }
        bubbleView = null
        unregisterClipboardListener()
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    // ── Foreground notification ────────────────────────────────────────────
    private fun startInForeground() {
        val nm = getSystemService(NotificationManager::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val ch = NotificationChannel(
                CHANNEL_ID, "DraftRight bubble",
                NotificationManager.IMPORTANCE_MIN
            )
            ch.description = "Persistent notification while the floating rewrite bubble is active."
            ch.setShowBadge(false)
            nm.createNotificationChannel(ch)
        }
        val tapIntent = Intent(this, MainActivity::class.java).apply {
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        }
        val pi = PendingIntent.getActivity(
            this, 0, tapIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val notif: Notification = Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("DraftRight bubble active")
            .setContentText("Tap the floating bubble to rewrite text.")
            .setSmallIcon(android.R.drawable.ic_menu_edit)
            .setOngoing(true)
            .setContentIntent(pi)
            .build()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            startForeground(NOTIFICATION_ID, notif,
                ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        } else {
            startForeground(NOTIFICATION_ID, notif)
        }
    }

    // ── Bubble view ────────────────────────────────────────────────────────
    private fun buildBubbleView(): View {
        val ctx = this
        val sizeDp = 56
        val sizePx = dp(sizeDp.toFloat()).toInt()

        val container = FrameLayout(ctx).apply {
            layoutParams = FrameLayout.LayoutParams(sizePx, sizePx)
        }
        val circle = TextView(ctx).apply {
            text = "✎"
            textSize = 22f
            gravity = Gravity.CENTER
            setTextColor(Color.WHITE)
            background = GradientDrawable().apply {
                shape = GradientDrawable.OVAL
                setColor(Color.parseColor("#1E40AF"))
                setStroke(dp(2f).toInt(), Color.parseColor("#FFFFFF"))
            }
            elevation = dp(6f)
            layoutParams = FrameLayout.LayoutParams(sizePx, sizePx)
        }
        container.addView(circle)
        container.setOnTouchListener(BubbleDragListener(circle))
        return container
    }

    private fun attachToWindow(v: View) {
        params.apply {
            type = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O)
                WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY
            else
                @Suppress("DEPRECATION")
                WindowManager.LayoutParams.TYPE_PHONE
            flags = WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                    WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS
            format = PixelFormat.TRANSLUCENT
            width = WindowManager.LayoutParams.WRAP_CONTENT
            height = WindowManager.LayoutParams.WRAP_CONTENT
            gravity = Gravity.TOP or Gravity.START
            x = prefs.getInt(KEY_X, dp(280f).toInt())
            y = prefs.getInt(KEY_Y, dp(400f).toInt())
        }
        runCatching { windowManager.addView(v, params) }
    }

    /** Translates touch events into drags; treats short, low-movement touches as taps. */
    private inner class BubbleDragListener(private val target: View) : View.OnTouchListener {
        private var initialX = 0
        private var initialY = 0
        private var touchX = 0f
        private var touchY = 0f
        private var downAt = 0L
        private var moved = false
        private val slop = dp(8f)

        override fun onTouch(v: View, e: MotionEvent): Boolean {
            when (e.action) {
                MotionEvent.ACTION_DOWN -> {
                    initialX = params.x; initialY = params.y
                    touchX = e.rawX; touchY = e.rawY
                    downAt = System.currentTimeMillis(); moved = false
                    target.alpha = 0.85f
                    return true
                }
                MotionEvent.ACTION_MOVE -> {
                    val dx = e.rawX - touchX
                    val dy = e.rawY - touchY
                    if (!moved && (abs(dx) > slop || abs(dy) > slop)) moved = true
                    if (moved) {
                        params.x = (initialX + dx).toInt()
                        params.y = (initialY + dy).toInt()
                        runCatching { windowManager.updateViewLayout(bubbleView!!, params) }
                    }
                    return true
                }
                MotionEvent.ACTION_UP -> {
                    target.alpha = 1f
                    val held = System.currentTimeMillis() - downAt
                    if (!moved && held < 350) {
                        onBubbleTap()
                    } else {
                        prefs.edit().putInt(KEY_X, params.x).putInt(KEY_Y, params.y).apply()
                    }
                    return true
                }
            }
            return false
        }
    }

    private fun onBubbleTap() {
        // Pull current clipboard. Empty / non-text → still launch the app
        // (user might want to paste into Playground); the share screen
        // refuses to launch with empty text, MainActivity falls back to
        // its normal launch flow in that case.
        val item = clipboard.primaryClip?.getItemAt(0)
        val text = item?.coerceToText(this)?.toString()?.trim().orEmpty()

        val launch = Intent(this, MainActivity::class.java).apply {
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP)
            if (text.isNotEmpty()) {
                action = Intent.ACTION_SEND
                type = "text/plain"
                putExtra(Intent.EXTRA_TEXT, text)
            }
        }
        startActivity(launch)
    }

    // ── Clipboard pulse ────────────────────────────────────────────────────
    private fun registerClipboardListener() {
        if (clipboardListener != null) return
        clipboardListener = ClipboardManager.OnPrimaryClipChangedListener {
            bubbleView?.let { v -> v.animate().scaleX(1.18f).scaleY(1.18f).setDuration(120)
                .withEndAction { v.animate().scaleX(1f).scaleY(1f).setDuration(160).start() }
                .start() }
        }
        clipboard.addPrimaryClipChangedListener(clipboardListener)
    }

    private fun unregisterClipboardListener() {
        clipboardListener?.let { clipboard.removePrimaryClipChangedListener(it) }
        clipboardListener = null
    }

    private fun dp(value: Float): Float =
        TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, value, resources.displayMetrics)

    override fun onDestroy() {
        bubbleView?.let { runCatching { windowManager.removeView(it) } }
        bubbleView = null
        unregisterClipboardListener()
        super.onDestroy()
    }
}
