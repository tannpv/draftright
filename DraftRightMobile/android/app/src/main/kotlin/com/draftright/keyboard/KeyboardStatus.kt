package com.draftright.keyboard

/**
 * Whether the DraftRight keyboard is usable, from the app's point of view (#272).
 *
 * Android never auto-enables a newly installed input method — an IME can observe
 * everything typed, so enabling it is an explicit user decision. That means every
 * fresh install (and every reinstall, which is what #272 turned out to be) leaves
 * the keyboard present but dead, and until now the app said nothing about it.
 */
enum class KeyboardStatus {
    /** Installed but not in the system's enabled list — needs Settings. */
    NOT_ENABLED,

    /** Enabled but some other IME is selected — needs the input-method picker. */
    ENABLED_NOT_SELECTED,

    /** Enabled and selected; nothing to prompt. */
    ACTIVE;

    companion object {
        /**
         * Resolve the status from the two system facts the app can read.
         *
         * Pure on purpose: the framework lookups (InputMethodManager,
         * Settings.Secure) stay in MainActivity, so the decision itself is unit
         * testable without Robolectric.
         *
         * Ids are compared by package, not by exact component, because the
         * service's class name is an implementation detail the system may report
         * with a relative or absolute form.
         */
        fun resolve(
            enabledImeIds: List<String>,
            defaultImeId: String?,
            ourPackage: String,
        ): KeyboardStatus {
            val enabled = enabledImeIds.any { it.substringBefore('/') == ourPackage }
            if (!enabled) return NOT_ENABLED
            val selected = defaultImeId?.substringBefore('/') == ourPackage
            return if (selected) ACTIVE else ENABLED_NOT_SELECTED
        }
    }
}
