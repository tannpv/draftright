package com.draftright.draftright_mobile.bubble

import android.accessibilityservice.AccessibilityService
import android.content.Intent
import android.os.Bundle
import android.view.accessibility.AccessibilityEvent
import android.view.accessibility.AccessibilityNodeInfo

/**
 * Reads and writes the focused text field of whatever app is in front. Owns
 * only node I/O — the capture/rewrite/deliver orchestration lives in
 * [BubbleRewriteCoordinator], and the overlay lives in FloatingBubbleService.
 *
 * Feasibility proven by the spike (experiments/a11y-rewrite-spike): read +
 * ACTION_SET_TEXT works on EditText-backed apps (Messages/WhatsApp/Zalo/Gmail),
 * Vietnamese diacritics preserved. WebView/Compose/secure fields report
 * replaceable=false so the coordinator can fall back to the clipboard.
 */
class RewriteAccessibilityService : AccessibilityService() {

    companion object {
        @Volatile
        var instance: RewriteAccessibilityService? = null
            private set

        /** True when the service is bound and can act on the focused field. */
        val isReady: Boolean get() = instance != null
    }

    override fun onServiceConnected() {
        super.onServiceConnected()
        instance = this
    }

    override fun onUnbind(intent: Intent?): Boolean {
        instance = null
        return super.onUnbind(intent)
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) { /* trigger is the bubble tap, not events */ }
    override fun onInterrupt() { }

    /** Snapshot the focused editable field, or null if none is focused. */
    fun captureFocusedInput(): CapturedInput? {
        val node = findFocusedEditable() ?: return null
        val text = node.text?.toString().orEmpty()
        // Replaceable only for editable, non-password nodes that accept SET_TEXT.
        val replaceable = node.isEditable &&
            !node.isPassword &&
            node.actionList.any { it.id == AccessibilityNodeInfo.ACTION_SET_TEXT }
        return CapturedInput(
            text = text,
            isEditable = node.isEditable,
            isPassword = node.isPassword,
            replaceable = replaceable,
        )
    }

    /** Replace the focused field's text in place. Returns false when the node
     *  is gone or rejects ACTION_SET_TEXT (WebView/Compose/custom editors). */
    fun replaceFocusedText(rewritten: String): Boolean {
        val node = findFocusedEditable() ?: return false
        if (!node.isEditable || node.isPassword) return false
        val args = Bundle().apply {
            putCharSequence(AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE, rewritten)
        }
        return node.performAction(AccessibilityNodeInfo.ACTION_SET_TEXT, args)
    }

    private fun findFocusedEditable(): AccessibilityNodeInfo? {
        rootInActiveWindow?.findFocus(AccessibilityNodeInfo.FOCUS_INPUT)?.let { return it }
        for (w in windows) {
            w.root?.findFocus(AccessibilityNodeInfo.FOCUS_INPUT)?.let { return it }
        }
        return null
    }
}
