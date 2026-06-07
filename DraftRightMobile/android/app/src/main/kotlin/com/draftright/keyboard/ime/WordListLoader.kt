package com.draftright.keyboard.ime

import android.content.Context
import android.util.Log

/**
 * Loads a TSV word list (and optional bigram file) from a raw resource into
 * an [InMemoryWordList]. Two-column word lines (`word<TAB>frequency`); three-
 * column bigram lines (`prev<TAB>next<TAB>count`). Lines starting with `#`
 * are comments; blank lines are skipped.
 *
 * Used as the bootstrap path for Latin-script suggestions before the
 * downloadable pack arrives. The format is intentionally human-readable so
 * the same loader handles a hand-curated 200-entry list (now) and a
 * 5k-entry list (next) without code changes — Rule #1 (extendable).
 */
object WordListLoader {

    /** Load just a word-frequency list; no bigrams. */
    fun loadWords(context: Context, wordsResId: Int): InMemoryWordList =
        loadWords(context, wordsResId, bigramsResId = null)

    /** Load a word-frequency list + an optional bigram successor file. */
    fun loadWords(context: Context, wordsResId: Int, bigramsResId: Int?): InMemoryWordList {
        val words = parseWords { line -> readResource(context, wordsResId, line) }
        val bigrams = if (bigramsResId != null) parseBigrams { line -> readResource(context, bigramsResId, line) } else emptyMap()
        Log.i(TAG, "Loaded ${words.size} words; bigram heads=${bigrams.size} (from raw res)")
        return InMemoryWordList(words, bigrams)
    }

    /**
     * Load a TSV pack from disk — used after [WordListPackResolver] picks
     * the latest installed pack. Same format as the bundled raw resource
     * so the build script can produce one artifact for both code paths.
     */
    fun loadWordsFromFile(wordsFile: java.io.File, bigramsFile: java.io.File? = null): InMemoryWordList {
        val words = parseWords { handler -> readFile(wordsFile, handler) }
        val bigrams = if (bigramsFile != null && bigramsFile.exists())
            parseBigrams { handler -> readFile(bigramsFile, handler) } else emptyMap()
        Log.i(TAG, "Loaded ${words.size} words; bigram heads=${bigrams.size} (from ${wordsFile.name})")
        return InMemoryWordList(words, bigrams)
    }

    private inline fun parseWords(crossinline readLines: (handler: (String) -> Unit) -> Unit): List<Pair<String, Int>> {
        val out = ArrayList<Pair<String, Int>>(2048)
        readLines { line ->
            val tab = line.indexOf('\t')
            if (tab <= 0) return@readLines
            val word = line.substring(0, tab).trim()
            val freq = line.substring(tab + 1).trim().toIntOrNull() ?: return@readLines
            if (word.isEmpty()) return@readLines
            out.add(word to freq)
        }
        // Sort by descending frequency so prefix scans hit the most-likely
        // candidates first and can short-circuit at `limit`.
        out.sortByDescending { it.second }
        return out
    }

    private inline fun parseBigrams(crossinline readLines: (handler: (String) -> Unit) -> Unit): Map<String, Map<String, Int>> {
        val out = HashMap<String, HashMap<String, Int>>()
        readLines { line ->
            val parts = line.split('\t')
            if (parts.size != 3) return@readLines
            val prev = parts[0].trim()
            val next = parts[1].trim()
            val count = parts[2].trim().toIntOrNull() ?: return@readLines
            if (prev.isEmpty() || next.isEmpty()) return@readLines
            val inner = out.getOrPut(prev) { HashMap() }
            inner[next] = (inner[next] ?: 0) + count
        }
        return out
    }

    private fun readResource(context: Context, resId: Int, handle: (String) -> Unit) {
        context.resources.openRawResource(resId).bufferedReader(Charsets.UTF_8).useLines { seq ->
            for (raw in seq) {
                val line = raw.trim()
                if (line.isEmpty() || line.startsWith('#')) continue
                handle(line)
            }
        }
    }

    private fun readFile(file: java.io.File, handle: (String) -> Unit) {
        file.bufferedReader(Charsets.UTF_8).useLines { seq ->
            for (raw in seq) {
                val line = raw.trim()
                if (line.isEmpty() || line.startsWith('#')) continue
                handle(line)
            }
        }
    }

    private const val TAG = "WordListLoader"
}
