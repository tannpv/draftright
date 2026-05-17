package com.draftright.keyboard

import android.content.Context
import android.content.SharedPreferences

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

    private fun parseStringList(raw: String?): List<String> {
        if (raw.isNullOrBlank()) return emptyList()
        return raw.trim()
            .removePrefix("[").removeSuffix("]")
            .split(",")
            .map { it.trim().removeSurrounding("\"") }
            .filter { it.isNotEmpty() }
    }
}
