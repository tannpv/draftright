package com.draftright.keyboard

import android.content.Context
import android.content.SharedPreferences
import org.json.JSONArray
import org.json.JSONException

class SharedSettings(context: Context) {
    private val prefs: SharedPreferences =
        context.getSharedPreferences("FlutterSharedPreferences", Context.MODE_PRIVATE)

    // Flutter shared_preferences prefixes keys with "flutter."

    /** Long-lived dr_ext_* token (preferred). Persisted by the main app
     *  on every login via ExtensionTokenService. Survives access-JWT
     *  expiry, so the IME keeps working after the main app has been
     *  idle for hours/days. */
    val extensionToken: String
        get() = prefs.getString("flutter.draftright.extensionToken", "") ?: ""

    /** Short-lived user JWT (legacy fallback). Will be removed in a
     *  follow-up release once everyone has launched the new main app
     *  version at least once. */
    val accessToken: String
        get() = prefs.getString("flutter.draftright.accessToken", "") ?: ""

    /** The token to actually present in Authorization headers. Prefer
     *  the long-lived extension token; fall back to the access JWT for
     *  users who haven't upgraded the main app yet. */
    val bearerToken: String
        get() = if (extensionToken.isNotEmpty()) extensionToken else accessToken

    val backendUrl: String
        get() = prefs.getString("flutter.draftright.backendUrl", "https://api.draftright.info")
            ?: "https://api.draftright.info"

    val translateLanguage: String
        get() = prefs.getString("flutter.draftright.translateLanguage", "Vietnamese") ?: "Vietnamese"

    /** IDs of enabled keyboard languages, in user-chosen order.
     *  Stored as a JSON array string from the Flutter side, parsed by
     *  removing brackets and splitting on commas — Flutter's
     *  shared_preferences StringList is JSON-encoded that way. */
    val enabledLanguageIds: List<String>
        get() = parseStringList(prefs.getString("flutter.draftright.enabledLanguageIds", null))
            .ifEmpty { listOf("en") }

    /** Currently active keyboard language id. Defaults to "en". */
    val activeLanguageId: String
        get() = prefs.getString("flutter.draftright.activeLanguageId", "en") ?: "en"

    private fun parseStringList(raw: String?) = Companion.parseStringList(raw)

    internal companion object {
        // shared_preferences_android 2.x encodes StringList with a base64 sentinel
        // followed by "!" then the JSON array. Older plugin versions omit the "!".
        // Both must be handled so users upgrading from old installs don't lose their
        // enabled-language list (which collapses to ["en"], silently killing language
        // switching).
        const val LEGACY_LIST_PREFIX = "VGhpcyBpcyB0aGUgcHJlZml4IGZvciBhIGxpc3Qu"
        const val JSON_LIST_PREFIX = "$LEGACY_LIST_PREFIX!"

        internal fun parseStringList(raw: String?): List<String> {
            if (raw.isNullOrBlank()) return emptyList()
            val s = raw.trim()
                .removePrefix(JSON_LIST_PREFIX)
                .removePrefix(LEGACY_LIST_PREFIX)
                .trim()
            try {
                val arr = JSONArray(s)
                val out = (0 until arr.length()).mapNotNull { i ->
                    arr.optString(i).takeIf { it.isNotEmpty() }
                }
                if (out.isNotEmpty()) return out
            } catch (_: Exception) { }
            // Fallback: Kotlin toString format "[en, vi]" — unquoted, comma-space
            return s.removePrefix("[").removeSuffix("]")
                .split(",")
                .map { it.trim().trim('"') }
                .filter { it.isNotEmpty() }
        }
    }
}
