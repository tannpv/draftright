package com.draftright.keyboard.ime

import android.content.Context
import java.io.File

/**
 * Picks the most-recent installed JP dictionary pack and falls back to the
 * bundled seed when none is installed.
 *
 * Mirrors [WordListPackResolver] but typed to `Map<String, List<String>>`
 * (reading → kanji candidates) so each engine type has a clean resolver.
 *
 * Pack files live under `<filesDir>/packs/<prefix>-v<N>.pack`, written by
 * the Flutter `ImePackService`.
 */
object JapanesePackResolver {

    /**
     * @param context       Android context — used to locate the packs directory.
     * @param packIdPrefix  Stable prefix matching the backend manifest URL
     *                      (e.g. `"draftright-ime-ja"`).
     * @param fallback      Returns the built-in seed when no installed pack found.
     */
    fun loadOrFallback(
        context: Context,
        packIdPrefix: String,
        fallback: () -> Map<String, List<String>>,
    ): Map<String, List<String>> {
        val installed = findLatestInstalled(context, packIdPrefix)
        if (installed != null) {
            val dict = runCatching { JapaneseDictLoader.load(installed) }.getOrNull()
            if (!dict.isNullOrEmpty()) return dict
        }
        return fallback()
    }

    private fun findLatestInstalled(context: Context, packIdPrefix: String): File? {
        val packsDir = File(context.filesDir, "packs")
        if (!packsDir.isDirectory) return null
        val regex = Regex("^${Regex.escape(packIdPrefix)}-v(\\d+)\\.pack$")
        var bestVersion = -1
        var bestFile: File? = null
        packsDir.listFiles()?.forEach { file ->
            val match = regex.matchEntire(file.name) ?: return@forEach
            val version = match.groupValues[1].toIntOrNull() ?: return@forEach
            if (file.length() > 0 && version > bestVersion) {
                bestVersion = version
                bestFile = file
            }
        }
        return bestFile
    }
}
