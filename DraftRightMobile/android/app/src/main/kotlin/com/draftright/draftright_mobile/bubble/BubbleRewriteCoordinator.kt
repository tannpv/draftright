package com.draftright.draftright_mobile.bubble

import android.content.ClipboardManager
import android.content.Context
import android.os.Handler
import android.os.Looper
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

    fun rewriteFocusedField() {
        val service = RewriteAccessibilityService.instance
        val captured = service?.captureFocusedInput()

        if (captured?.isPassword == true) {
            toast(R.string.bubble_rewrite_password_blocked)
            return
        }

        // Prefer the focused field's text; fall back to clipboard as the source
        // when the field can't be read (a11y off / WebView).
        val source = captured?.text?.takeIf { it.isNotBlank() } ?: clipboardText()
        if (source.isNullOrBlank()) {
            toast(if (service == null) R.string.bubble_rewrite_no_field else R.string.bubble_rewrite_empty)
            return
        }

        val settings = SharedSettings(context)
        if (settings.bearerToken.isEmpty()) {
            toast(R.string.bubble_rewrite_login_required)
            return
        }

        toast(R.string.bubble_rewrite_working)
        backend.rewrite(source, settings.bubblePresetTone, settings) { result ->
            main.post {
                result
                    .onSuccess { rewritten -> deliver(rewritten, captured) }
                    .onFailure { err ->
                        toast(err.message ?: context.getString(R.string.bubble_rewrite_failed))
                    }
            }
        }
    }

    private fun deliver(rewritten: String, captured: CapturedInput?) {
        val service = RewriteAccessibilityService.instance
        val canInPlace = service != null && captured?.replaceable == true
        if (canInPlace && InPlaceAccessibilityTarget(service!!).deliver(rewritten)) {
            toast(R.string.bubble_rewrite_done)
            return
        }
        val label = context.getString(R.string.bubble_clip_label)
        val copied = ClipboardFallbackTarget(context, label).deliver(rewritten)
        toast(if (copied) R.string.bubble_rewrite_copied_fallback else R.string.bubble_rewrite_failed)
    }

    private fun clipboardText(): String? {
        val cm = context.getSystemService(Context.CLIPBOARD_SERVICE) as? ClipboardManager ?: return null
        return cm.primaryClip?.getItemAt(0)?.coerceToText(context)?.toString()
    }

    private fun toast(resId: Int) = Toast.makeText(context, resId, Toast.LENGTH_SHORT).show()
    private fun toast(message: String) = Toast.makeText(context, message, Toast.LENGTH_SHORT).show()
}
