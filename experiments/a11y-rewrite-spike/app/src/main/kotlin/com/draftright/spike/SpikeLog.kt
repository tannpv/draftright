package com.draftright.spike

import android.content.Context
import android.util.Log

/**
 * Dead-simple append-only result log, persisted to SharedPreferences so the
 * setup screen can show what happened per app without a database. Also mirrors
 * to Logcat (tag SPIKE) for `adb logcat -s SPIKE`.
 */
object SpikeLog {
    const val TAG = "SPIKE"
    private const val PREFS = "spike"
    private const val KEY = "log"

    fun add(ctx: Context, line: String) {
        Log.d(TAG, line)
        val p = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val prev = p.getString(KEY, "") ?: ""
        p.edit().putString(KEY, prev + line + "\n").apply()
    }

    fun read(ctx: Context): String =
        ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY, "") ?: ""

    fun clear(ctx: Context) {
        ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit().remove(KEY).apply()
    }
}
