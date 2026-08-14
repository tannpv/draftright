package com.draftright.draftright_mobile

import android.content.Intent
import android.os.Bundle
import io.flutter.embedding.android.FlutterFragmentActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

/**
 * Native bridge for the app over the `draftright/share` method channel:
 *  - `sharedPackDir` — app files dir, shared with the IME (language packs).
 *  - `getSharedText` — text sent to the app via ACTION_SEND (share sheet from
 *    Teams/Zalo/SMS/…), consumed once by Flutter to open Smart Extract (#143).
 *    Cold start: captured in onCreate. Warm start: pushed via `sharedText`.
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

    // Text shared into the app via ACTION_SEND, pending pickup by Flutter.
    private var pendingSharedText: String? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        pendingSharedText = extractSharedText(intent)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        val text = extractSharedText(intent)
        if (text != null) {
            pendingSharedText = text
            // App already running → push to Flutter immediately.
            channel?.invokeMethod("sharedText", text)
        }
    }

    private fun extractSharedText(intent: Intent?): String? {
        if (intent?.action != Intent.ACTION_SEND || intent.type != "text/plain") return null
        val t = intent.getStringExtra(Intent.EXTRA_TEXT)
        return if (t.isNullOrBlank()) null else t
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        channel = MethodChannel(flutterEngine.dartExecutor.binaryMessenger, channelName)
        channel?.setMethodCallHandler { call, result ->
            when (call.method) {
                "sharedPackDir" -> result.success(filesDir.absolutePath)
                "getSharedText" -> {
                    val t = pendingSharedText
                    pendingSharedText = null // consume once
                    result.success(t)
                }
                else -> result.notImplemented()
            }
        }
    }
}
