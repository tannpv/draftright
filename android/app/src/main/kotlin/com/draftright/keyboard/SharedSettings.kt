package com.draftright.keyboard

import android.content.Context
import android.content.SharedPreferences

class SharedSettings(context: Context) {
    private val prefs: SharedPreferences =
        context.getSharedPreferences("FlutterSharedPreferences", Context.MODE_PRIVATE)

    // Flutter shared_preferences prefixes keys with "flutter."
    val accessToken: String
        get() = prefs.getString("flutter.draftright.accessToken", "") ?: ""

    val backendUrl: String
        get() = prefs.getString("flutter.draftright.backendUrl", "https://api.draftright.info")
            ?: "https://api.draftright.info"

    val translateLanguage: String
        get() = prefs.getString("flutter.draftright.translateLanguage", "Vietnamese") ?: "Vietnamese"
}
