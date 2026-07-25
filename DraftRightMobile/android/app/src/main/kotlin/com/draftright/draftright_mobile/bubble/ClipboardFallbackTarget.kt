package com.draftright.draftright_mobile.bubble

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context

/**
 * Fallback delivery: put the rewrite on the clipboard so the user can paste it.
 * Used when in-place replace is impossible (WebView/Compose/custom editors) or
 * the AccessibilityService isn't enabled — so the feature degrades gracefully
 * instead of failing.
 */
class ClipboardFallbackTarget(
    private val context: Context,
    private val clipLabel: String,
) : RewriteTarget {
    override val kind = RewriteTargetKind.CLIPBOARD_FALLBACK

    override fun deliver(rewritten: String): Boolean {
        val cm = context.getSystemService(Context.CLIPBOARD_SERVICE) as? ClipboardManager
            ?: return false
        cm.setPrimaryClip(ClipData.newPlainText(clipLabel, rewritten))
        return true
    }
}
