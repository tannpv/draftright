package com.draftright.draftright_mobile

import android.content.Intent
import android.provider.Settings
import android.view.inputmethod.InputMethodManager
import com.draftright.keyboard.KeyboardStatus
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
                // #272: Android never auto-enables a newly installed input
                // method, so after any fresh install our keyboard exists but is
                // dead. The app has to be able to see that and offer the fix.
                "keyboardStatus" -> result.success(keyboardStatus().name)
                "openKeyboardSettings" -> {
                    startActivity(Intent(Settings.ACTION_INPUT_METHOD_SETTINGS))
                    result.success(null)
                }
                "openKeyboardPicker" -> {
                    inputMethodManager().showInputMethodPicker()
                    result.success(null)
                }
                else -> result.notImplemented()
            }
        }
    }

    private fun inputMethodManager() =
        getSystemService(INPUT_METHOD_SERVICE) as InputMethodManager

    /**
     * Read the two system facts and let [KeyboardStatus] decide. The framework
     * lookups live here so the decision itself stays pure and unit tested.
     */
    private fun keyboardStatus(): KeyboardStatus = KeyboardStatus.resolve(
        enabledImeIds = inputMethodManager().enabledInputMethodList.map { it.id },
        defaultImeId = Settings.Secure.getString(
            contentResolver,
            Settings.Secure.DEFAULT_INPUT_METHOD,
        ),
        ourPackage = packageName,
    )
}
