package com.draftright.draftright_mobile

import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.Settings
import io.flutter.embedding.android.FlutterFragmentActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

/**
 * Captures ACTION_SEND text/plain intents (system Share or floating bubble
 * tap) and forwards them to Flutter via a method channel.  Same channel
 * also exposes the floating-bubble lifecycle and the overlay-permission
 * check / settings deep-link.
 *
 * Method channel: `draftright/share`
 *
 *  Methods Flutter → native:
 *    getInitialSharedText  — drains pending share text (consumed once)
 *    canDrawOverlays       — Settings.canDrawOverlays(this)
 *    openOverlaySettings   — launch ACTION_MANAGE_OVERLAY_PERMISSION
 *    startBubble           — start FloatingBubbleService
 *    stopBubble            — stop FloatingBubbleService
 *
 *  Native → Flutter:
 *    onSharedText(String)  — fired when a fresh share arrives while the
 *                            app is running (live update).
 */
/**
 * `FlutterFragmentActivity` (not `FlutterActivity`) is required by
 * `flutter_stripe`: the Stripe Android SDK presents its Apple Pay /
 * Google Pay sheets via `androidx.fragment` transactions, which need
 * a `FragmentActivity` host.  Launching from a plain `FlutterActivity`
 * throws `StripeConfigException` on the first SDK call.
 */
class MainActivity : FlutterFragmentActivity() {
    private val channelName = "draftright/share"
    private var pendingSharedText: String? = null
    private var channel: MethodChannel? = null

    override fun onCreate(savedInstanceState: android.os.Bundle?) {
        super.onCreate(savedInstanceState)
        pendingSharedText = extractSharedText(intent)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        val text = extractSharedText(intent) ?: return
        pendingSharedText = text
        channel?.invokeMethod("onSharedText", text)
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        channel = MethodChannel(flutterEngine.dartExecutor.binaryMessenger, channelName)
        channel?.setMethodCallHandler { call, result ->
            when (call.method) {
                "getInitialSharedText" -> {
                    val t = pendingSharedText
                    pendingSharedText = null
                    result.success(t)
                }
                "canDrawOverlays" -> {
                    result.success(canDrawOverlays())
                }
                "openOverlaySettings" -> {
                    openOverlaySettings()
                    result.success(null)
                }
                "startBubble" -> {
                    if (!canDrawOverlays()) {
                        result.error("NO_PERMISSION",
                            "Overlay permission not granted", null)
                    } else {
                        val svc = Intent(this, FloatingBubbleService::class.java)
                            .apply { action = FloatingBubbleService.ACTION_START }
                        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                            startForegroundService(svc)
                        } else {
                            startService(svc)
                        }
                        result.success(true)
                    }
                }
                "stopBubble" -> {
                    val svc = Intent(this, FloatingBubbleService::class.java)
                        .apply { action = FloatingBubbleService.ACTION_STOP }
                    startService(svc)
                    result.success(true)
                }
                "dismissToBackground" -> {
                    // Send DraftRight to the back of the task stack — the
                    // app the user was in before tapping the bubble (or
                    // before sharing) comes back to foreground. Lets us
                    // simulate "auto-paste-back" without Accessibility:
                    // user lands in the source app + clipboard already
                    // contains the rewrite, so a single Paste is one tap.
                    moveTaskToBack(true)
                    result.success(null)
                }
                "sharedPackDir" -> {
                    // App files dir — shared with the IME (same package), so
                    // downloaded language packs are readable by the keyboard.
                    result.success(filesDir.absolutePath)
                }
                "openAccessibilitySettings" -> {
                    // Deep-link to system Accessibility settings so the user can
                    // enable DraftRight's in-place rewrite service.
                    startActivity(
                        Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)
                            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                    )
                    result.success(null)
                }
                "isInPlaceRewriteReady" -> {
                    // True when the AccessibilityService is enabled + bound.
                    result.success(
                        com.draftright.draftright_mobile.bubble
                            .RewriteAccessibilityService.isReady
                    )
                }
                else -> result.notImplemented()
            }
        }
    }

    private fun canDrawOverlays(): Boolean =
        Build.VERSION.SDK_INT < Build.VERSION_CODES.M ||
        Settings.canDrawOverlays(this)

    private fun openOverlaySettings() {
        val i = Intent(
            Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
            Uri.parse("package:$packageName")
        ).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        startActivity(i)
    }

    private fun extractSharedText(intent: Intent?): String? {
        if (intent?.action != Intent.ACTION_SEND) return null
        if (intent.type != "text/plain") return null
        val raw = intent.getStringExtra(Intent.EXTRA_TEXT) ?: return null
        val trimmed = raw.trim()
        return if (trimmed.isEmpty()) null else trimmed
    }
}
