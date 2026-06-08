package com.draftright.keyboard.ime

import java.io.File
import java.io.InputStream

/**
 * Parses a Japanese reading→kanji dictionary pack (`.pack`) into the map
 * that [JapaneseDictionaryEngine] consumes.
 *
 * Format — one reading per line:
 *   `reading<TAB>kanji1,kanji2,...`
 * Lines starting with `#` are comments; blank lines are skipped.
 * Rank order of candidates is preserved (first = most preferred).
 *
 * Rule #1: pure parsing, no platform I/O, no hardcoded filenames — callers
 * supply the File/InputStream; `JapaneseLanguagePack` wires the resolver.
 */
object DictPackLoader {

    /** Load a reading→kanji map from a `.pack` file on disk. */
    fun load(file: File): Map<String, List<String>> =
        file.inputStream().use { parse(it) }

    /** Parse from an [InputStream] — useful for bundled resources. */
    fun parse(stream: InputStream): Map<String, List<String>> =
        parse(stream.bufferedReader().readText())

    /** Parse raw TSV content — useful for testing without disk I/O. */
    fun parse(tsv: String): Map<String, List<String>> {
        val result = mutableMapOf<String, List<String>>()
        for (rawLine in tsv.lineSequence()) {
            val line = rawLine.trim()
            if (line.isEmpty() || line.startsWith('#')) continue
            val tab = line.indexOf('\t')
            if (tab <= 0) continue
            val reading = line.substring(0, tab).trim()
            val candidatesRaw = line.substring(tab + 1).trim()
            if (reading.isEmpty() || candidatesRaw.isEmpty()) continue
            val candidates = candidatesRaw.split(',')
                .map { it.trim() }
                .filter { it.isNotEmpty() }
            if (candidates.isNotEmpty()) result[reading] = candidates
        }
        return result
    }
}
