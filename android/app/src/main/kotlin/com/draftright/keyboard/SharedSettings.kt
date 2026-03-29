package com.draftright.keyboard

import android.content.Context
import android.content.SharedPreferences

class SharedSettings(context: Context) {
    private val prefs: SharedPreferences =
        context.getSharedPreferences("FlutterSharedPreferences", Context.MODE_PRIVATE)

    // Flutter shared_preferences prefixes keys with "flutter."
    val aiProvider: String get() = prefs.getString("flutter.draftright.aiProvider", "openai") ?: "openai"
    val apiKey: String get() = prefs.getString("flutter.draftright.apiKey", "") ?: ""
    val endpoint: String get() = prefs.getString("flutter.draftright.endpoint", "https://api.openai.com/v1/chat/completions") ?: "https://api.openai.com/v1/chat/completions"
    val model: String get() = prefs.getString("flutter.draftright.model", "gpt-4o-mini") ?: "gpt-4o-mini"
    val temperature: Double get() {
        return try {
            prefs.getFloat("flutter.draftright.temperature", 0.3f).toDouble()
        } catch (_: ClassCastException) {
            try {
                // Flutter shared_preferences stores doubles as Long bits
                val bits = prefs.getLong("flutter.draftright.temperature", (0.3).toRawBits())
                Double.fromBits(bits)
            } catch (_: ClassCastException) {
                // Fallback: stored as string
                prefs.getString("flutter.draftright.temperature", "0.3")?.toDoubleOrNull() ?: 0.3
            }
        }
    }
    val translateLanguage: String get() = prefs.getString("flutter.draftright.translateLanguage", "Vietnamese") ?: "Vietnamese"
}
