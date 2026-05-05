package com.draftright.draftright_mobile

import android.content.Intent
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

/**
 * Captures ACTION_SEND text/plain intents and forwards them to Flutter via
 * a method channel.  Two surfaces:
 *
 *  - [getInitialSharedText]  — Flutter calls this on app start to drain any
 *    text the user shared while DraftRight was not running.
 *  - [onSharedText] callback — Flutter sets a handler; we invoke it whenever
 *    the activity receives a fresh share while already running.
 *
 * Required because Samsung / Xiaomi / Huawei strip third-party PROCESS_TEXT
 * actions out of the text-selection popup; ACTION_SEND is the reliable
 * fallback because no OEM restricts the share sheet.
 */
class MainActivity : FlutterActivity() {
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
        // App is already running — push to Flutter immediately, and stash
        // a copy in case Flutter asks for the initial text before its
        // handler is wired up.
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
                    pendingSharedText = null  // consume so reads aren't sticky
                    result.success(t)
                }
                else -> result.notImplemented()
            }
        }
    }

    private fun extractSharedText(intent: Intent?): String? {
        if (intent?.action != Intent.ACTION_SEND) return null
        if (intent.type != "text/plain") return null
        val raw = intent.getStringExtra(Intent.EXTRA_TEXT) ?: return null
        val trimmed = raw.trim()
        return if (trimmed.isEmpty()) null else trimmed
    }
}
