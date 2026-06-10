package com.draftright.keyboard.ime

import android.content.Context
import android.util.Log
import java.io.File

/**
 * Picks the most-recent installed wordlist pack for a language and falls
 * back to the in-APK bootstrap when none is installed.
 *
 * Pack files live under `<filesDir>/packs/<packId>.pack`, written by the
 * Flutter [ImePackService] in the host app. The IME service shares the app's
 * private storage so it can mmap the file without a permission grant.
 *
 * Per Rule #1: this is the single point that swaps "downloaded" for
 * "bundled" — VietnameseLanguagePack (and any future Latin pack) just calls
 * [loadOrFallback] and gets a [LanguageWordList] regardless of which source
 * served it.
 */
object WordListPackResolver {

    private const val TAG = "WordListPackResolver"
    private const val PACKS_SUBDIR = "packs"

    /**
     * Try the installed pack; fall back to the bundled raw resource.
     *
     * @param packIdPrefix Stable prefix matching the backend manifest's
     *                     download URL (e.g. `"draftright-wordlist-vi"`).
     *                     The resolver scans for `<prefix>-v<N>.pack` and
     *                     picks the highest N — that's the latest installed
     *                     version even if the catalog has rotated.
     * @param fallbackResId Raw resource ID of the bootstrap TSV that ships
     *                      with the APK.
     */
    fun loadOrFallback(
        context: Context,
        packIdPrefix: String,
        fallbackResId: Int,
    ): LanguageWordList {
        val installed = findLatestInstalled(context, packIdPrefix)
        if (installed != null) {
            try {
                Log.i(TAG, "Using installed pack ${installed.name} (${installed.length()} bytes)")
                return WordListLoader.loadWordsFromFile(installed)
            } catch (e: Exception) {
                // Don't let a corrupt pack kill suggestions — fall back to
                // the bundled list, log so it shows up in /errors.
                Log.w(TAG, "Failed to load installed pack ${installed.name}; falling back to bootstrap", e)
            }
        }
        return WordListLoader.loadWords(context, fallbackResId)
    }

    /**
     * Returns the highest-version installed pack matching the prefix, or
     * null when nothing is installed. Versioning lets a rollout cleanly
     * coexist with an older client mid-update.
     */
    private fun findLatestInstalled(context: Context, packIdPrefix: String): File? {
        val packsDir = File(context.filesDir, PACKS_SUBDIR)
        if (!packsDir.isDirectory) return null
        val regex = Regex("^${Regex.escape(packIdPrefix)}-v(\\d+)\\.pack$")
        var best: File? = null
        var bestVersion = -1
        packsDir.listFiles()?.forEach { f ->
            val m = regex.matchEntire(f.name) ?: return@forEach
            val v = m.groupValues[1].toIntOrNull() ?: return@forEach
            if (v > bestVersion && f.length() > 0) {
                best = f
                bestVersion = v
            }
        }
        return best
    }
}
