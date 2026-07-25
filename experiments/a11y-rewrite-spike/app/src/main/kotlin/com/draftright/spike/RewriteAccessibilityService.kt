package com.draftright.spike

import android.accessibilityservice.AccessibilityService
import android.os.Bundle
import android.view.accessibility.AccessibilityEvent
import android.view.accessibility.AccessibilityNodeInfo

/**
 * The mechanic under test: on trigger, find the focused input field in whatever
 * app is in front, read its text, apply a trivial local transform (UPPERCASE),
 * and write it back via ACTION_SET_TEXT. Every step is logged with the node's
 * class / editable / password / package so the per-app matrix can be filled in
 * from `adb logcat -s SPIKE` or the setup screen.
 *
 * UPPERCASE (not a network rewrite) is deliberate: a pass/fail here reflects
 * Android's accessibility constraints, not auth or connectivity.
 */
class RewriteAccessibilityService : AccessibilityService() {

    companion object {
        @Volatile
        var instance: RewriteAccessibilityService? = null
    }

    override fun onServiceConnected() {
        super.onServiceConnected()
        instance = this
        SpikeLog.add(this, "── a11y service CONNECTED ──")
    }

    override fun onUnbind(intent: android.content.Intent?): Boolean {
        instance = null
        SpikeLog.add(this, "── a11y service UNBOUND ──")
        return super.onUnbind(intent)
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) { /* not needed for the spike */ }
    override fun onInterrupt() { }

    /**
     * Called by the overlay bubble tap. Runs entirely on the a11y side so the
     * bubble (FLAG_NOT_FOCUSABLE) never becomes the focused node.
     */
    fun rewriteFocusedField() {
        val node = findFocusedEditable()
        if (node == null) {
            SpikeLog.add(this, "TAP → no focused editable node found (root=${rootInActiveWindow != null})")
            return
        }

        val pkg = node.packageName?.toString() ?: "?"
        val cls = node.className?.toString() ?: "?"
        val editable = node.isEditable
        val password = node.isPassword
        val before = node.text?.toString()

        SpikeLog.add(
            this,
            "TAP → app=$pkg node=$cls editable=$editable password=$password " +
                "readLen=${before?.length ?: -1} read=\"${before?.take(40)}\""
        )

        if (before == null) {
            SpikeLog.add(this, "   READ FAIL: node.text is null (content hidden or not exposed)")
            return
        }

        val rewritten = before.uppercase()
        val args = Bundle().apply {
            putCharSequence(AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE, rewritten)
        }
        val ok = node.performAction(AccessibilityNodeInfo.ACTION_SET_TEXT, args)
        SpikeLog.add(
            this,
            if (ok) "   ✅ ACTION_SET_TEXT OK → \"${rewritten.take(40)}\""
            else "   ❌ ACTION_SET_TEXT REJECTED (node likely WebView/Compose/custom — no SET_TEXT support)"
        )
    }

    /**
     * Prefer the input-focused node in the active window; fall back to scanning
     * all interactive windows (some apps host the field in a secondary window).
     */
    private fun findFocusedEditable(): AccessibilityNodeInfo? {
        rootInActiveWindow?.findFocus(AccessibilityNodeInfo.FOCUS_INPUT)?.let { return it }
        for (w in windows) {
            val root = w.root ?: continue
            root.findFocus(AccessibilityNodeInfo.FOCUS_INPUT)?.let { return it }
        }
        // Last resort: the accessibility-focused node, if editable.
        rootInActiveWindow?.findFocus(AccessibilityNodeInfo.FOCUS_ACCESSIBILITY)
            ?.takeIf { it.isEditable }
            ?.let { return it }
        return null
    }
}
