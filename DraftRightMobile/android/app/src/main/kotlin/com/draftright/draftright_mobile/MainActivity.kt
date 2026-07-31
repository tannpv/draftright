package com.draftright.draftright_mobile

import io.flutter.embedding.android.FlutterFragmentActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

/**
 * Native bridge for the app: exposes the shared pack dir (used by the IME) to
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
                "sharedPackDir" -> {
                    // App files dir — shared with the IME (same package), so
                    // downloaded language packs are readable by the keyboard.
                    result.success(filesDir.absolutePath)
                }
                else -> result.notImplemented()
            }
        }
    }
}
