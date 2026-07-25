package com.draftright.draftright_mobile.bubble

import android.content.ClipboardManager
import android.content.Context
import android.os.Handler
import android.os.Looper
import android.util.Log
import android.widget.Toast
import com.draftright.draftright_mobile.v2.R
import com.draftright.keyboard.BackendClient
import com.draftright.keyboard.SharedSettings

/**
 * Orchestrates one bubble-triggered in-place rewrite: capture the focused
 * field → guard → call the shared /rewrite backend with the preset tone →
 * deliver via the best available [RewriteTarget], falling back to the
 * clipboard when in-place replace isn't possible.
 *
 * Owns no UI and no network of its own — it reuses [BackendClient],
 * [SharedSettings], and the [RewriteAccessibilityService]. The bubble calls
 * [rewriteFocusedField]; everything user-facing is a localized string.
 */
class BubbleRewriteCoordinator(private val context: Context) {

    private val main = Handler(Looper.getMainLooper())
    private val backend = BackendClient()

    private companion object { const val TAG = "DRBubble" }

    /**
     * @param onBusy invoked with true when the network rewrite starts and false
     *   when it finishes (success or failure) — the bubble uses it to pulse while
     *   the user waits. Not called for the early guard returns (nothing async).
     */
    fun rewriteFocusedField(onBusy: (Boolean) -> Unit = {}) {
        val service = RewriteAccessibilityService.instance
        val captured = service?.captureFocusedInput()
        Log.d(TAG, "rewriteFocusedField: service=${service != null} captured=$captured")

        if (captured?.isPassword == true) {
            Log.d(TAG, "  password field — blocked")
            toast(R.string.bubble_rewrite_password_blocked)
            return
        }

        // Prefer the focused field's text; fall back to clipboard as the source
        // when the field can't be read (a11y off / WebView).
        val source = captured?.text?.takeIf { it.isNotBlank() } ?: clipboardText()
        Log.d(TAG, "  source len=${source?.length ?: -1}")
        if (source.isNullOrBlank()) {
            Log.d(TAG, "  no source — toasting no-field/empty")
            toast(if (service == null) R.string.bubble_rewrite_no_field else R.string.bubble_rewrite_empty)
            return
        }

        val settings = SharedSettings(context)
        if (settings.bearerToken.isEmpty()) {
            Log.d(TAG, "  no bearer token — login required")
            toast(R.string.bubble_rewrite_login_required)
            return
        }

        Log.d(TAG, "  calling /rewrite tone=${settings.bubblePresetTone.apiValue}")
        onBusy(true)
        toast(R.string.bubble_rewrite_working)
        backend.rewrite(source, settings.bubblePresetTone, settings) { result ->
            main.post {
                onBusy(false)
                result
                    .onSuccess { rewritten ->
                        Log.d(TAG, "  /rewrite OK len=${rewritten.length}")
                        deliver(rewritten, captured)
                    }
                    .onFailure { err ->
                        Log.d(TAG, "  /rewrite FAIL: ${err.message}")
                        toast(err.message ?: context.getString(R.string.bubble_rewrite_failed))
                    }
            }
        }
    }

    private fun deliver(rewritten: String, captured: CapturedInput?) {
        val service = RewriteAccessibilityService.instance
        val canInPlace = service != null && captured?.replaceable == true
        Log.d(TAG, "deliver: canInPlace=$canInPlace replaceable=${captured?.replaceable}")
        if (canInPlace && InPlaceAccessibilityTarget(service!!).deliver(rewritten)) {
            Log.d(TAG, "  delivered in place OK")
            toast(R.string.bubble_rewrite_done)
            return
        }
        val label = context.getString(R.string.bubble_clip_label)
        val copied = ClipboardFallbackTarget(context, label).deliver(rewritten)
        Log.d(TAG, "  in-place failed → clipboard fallback copied=$copied")
        toast(if (copied) R.string.bubble_rewrite_copied_fallback else R.string.bubble_rewrite_failed)
    }

    private fun clipboardText(): String? {
        val cm = context.getSystemService(Context.CLIPBOARD_SERVICE) as? ClipboardManager ?: return null
        return cm.primaryClip?.getItemAt(0)?.coerceToText(context)?.toString()
    }

    private fun toast(resId: Int) = Toast.makeText(context, resId, Toast.LENGTH_SHORT).show()
    private fun toast(message: String) = Toast.makeText(context, message, Toast.LENGTH_SHORT).show()
}
