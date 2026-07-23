package com.draftright.draftright_mobile.bubble

/**
 * How a rewrite is delivered back to the user's text. A strategy port so new
 * delivery mechanisms (in-place a11y write, clipboard, a future IME-commit)
 * are added as implementations without touching the bubble or the rewrite call.
 * Mirrors the desktop One-Click strategy split.
 */
interface RewriteTarget {
    val kind: RewriteTargetKind

    /** Deliver [rewritten] to the user. Returns true on success; false lets the
     *  coordinator fall back to the next target (e.g. clipboard). */
    fun deliver(rewritten: String): Boolean
}

/** Typed delivery kinds — never raw strings/ints at call sites. */
enum class RewriteTargetKind { ACCESSIBILITY_IN_PLACE, CLIPBOARD_FALLBACK }

/**
 * What the AccessibilityService read from the focused field. [text] is the
 * current contents; [isPassword]/[isEditable] gate whether we may rewrite it;
 * [replaceable] is true only when an in-place write is actually possible.
 */
data class CapturedInput(
    val text: String,
    val isEditable: Boolean,
    val isPassword: Boolean,
    val replaceable: Boolean,
)
