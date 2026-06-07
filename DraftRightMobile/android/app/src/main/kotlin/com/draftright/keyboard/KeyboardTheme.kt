package com.draftright.keyboard

import android.content.Context
import android.content.res.Configuration

/**
 * Single source of truth for the keyboard's dark/light detection.
 *
 * Several views (QwertyKeyboardView, LanguageStripView, popups) previously
 * each re-derived `uiMode and UI_MODE_NIGHT_MASK` inline. Centralising it
 * keeps the detection identical everywhere and gives one place to change
 * if the theme strategy ever moves to a user setting.
 */
object KeyboardTheme {
    /** DraftRight brand blue — matches the website primary, rgb(93 135 255). */
    const val BRAND_BLUE = "#5D87FF"

    fun isDark(context: Context): Boolean {
        val nightMode = context.resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK
        return nightMode == Configuration.UI_MODE_NIGHT_YES
    }
}
