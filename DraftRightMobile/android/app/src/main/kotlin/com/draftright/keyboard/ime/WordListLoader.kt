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
        val words = parseWords(context, wordsResId)
        val bigrams = if (bigramsResId != null) parseBigrams(context, bigramsResId) else emptyMap()
        Log.i(TAG, "Loaded ${words.size} words; bigram heads=${bigrams.size}")
        return InMemoryWordList(words, bigrams)
    }

    private fun parseWords(context: Context, resId: Int): List<Pair<String, Int>> {
        val out = ArrayList<Pair<String, Int>>(2048)
        readLines(context, resId) { line ->
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

    private fun parseBigrams(context: Context, resId: Int): Map<String, Map<String, Int>> {
        val out = HashMap<String, HashMap<String, Int>>()
        readLines(context, resId) { line ->
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

    private inline fun readLines(context: Context, resId: Int, handle: (String) -> Unit) {
        context.resources.openRawResource(resId).bufferedReader(Charsets.UTF_8).useLines { seq ->
            for (raw in seq) {
                val line = raw.trim()
                if (line.isEmpty() || line.startsWith('#')) continue
                handle(line)
            }
        }
    }

    private const val TAG = "WordListLoader"
}
