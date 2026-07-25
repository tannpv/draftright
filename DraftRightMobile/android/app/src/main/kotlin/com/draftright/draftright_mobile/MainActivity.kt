package com.draftright.draftright_mobile

import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.Settings
import io.flutter.embedding.android.FlutterFragmentActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

/**
 * Native bridge for the app: floating-bubble lifecycle, overlay/accessibility
 * permission checks + settings deep-links, and the shared pack dir. Exposed to
 * Flutter over the `draftright/share` method channel.
 *
 * `FlutterFragmentActivity` (not `FlutterActivity`) is required by
 * `flutter_stripe`: the Stripe Android SDK presents its Apple Pay / Google Pay
 * sheets via `androidx.fragment` transactions, which need a `FragmentActivity`
 * host. Launching from a plain `FlutterActivity` throws `StripeConfigException`
 * on the first SDK call.
 */
class MainActivity : FlutterFragmentActivity() {
    private val channelName = "draftright/share"
    private var channel: MethodChannel? = null

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        channel = MethodChannel(flutterEngine.dartExecutor.binaryMessenger, channelName)
        channel?.setMethodCallHandler { call, result ->
            when (call.method) {
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
}
