package com.draftright.draftright_mobile.bubble

/**
 * Writes the rewrite straight back into the focused field via the
 * AccessibilityService (ACTION_SET_TEXT). Returns false when the field can't
 * be replaced, so the coordinator falls back to the clipboard.
 */
class InPlaceAccessibilityTarget(
    private val service: RewriteAccessibilityService,
) : RewriteTarget {
    override val kind = RewriteTargetKind.ACCESSIBILITY_IN_PLACE

    override fun deliver(rewritten: String): Boolean =
        service.replaceFocusedText(rewritten)
}
