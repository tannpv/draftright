package com.draftright.keyboard.composer

import java.io.File
import org.json.JSONArray
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Test

/**
 * Every valid Vietnamese syllable, typed in every mark/tone order, must come
 * out the same (#207, spec 2026-09-11).
 *
 * The matrix is produced by `DraftRightMobile/tools/gen_telex_cases.py` — the
 * only owner of the orthography tables (RULE #1) — and the Swift
 * `TelexExhaustiveTests` runs the identical generator, so a divergence in
 * either port fails on both. Skipped when python3 is unavailable; both CI
 * runners have it.
 */
class TelexExhaustiveTest {

    private val repoRoot: File
        get() {
            // Tests run with the module dir as CWD; climb to the repo root.
            var dir: File? = File(".").absoluteFile
            while (dir != null && !File(dir, GENERATOR_PATH).isFile) {
                dir = dir.parentFile
            }
            requireNotNull(dir) { "could not locate $GENERATOR_PATH above ${File(".").absolutePath}" }
            return dir
        }

    /** The generated matrix, or null when python3 is not installed. */
    private fun generateCases(): JSONArray? {
        val out = File.createTempFile("telex_cases_kotlin", ".json")
        val process = try {
            ProcessBuilder("python3", File(repoRoot, GENERATOR_PATH).path, "--out", out.path)
                .redirectErrorStream(true)
                .start()
        } catch (e: java.io.IOException) {
            return null
        }
        val log = process.inputStream.bufferedReader().readText()
        val status = process.waitFor()
        assertTrue("generator exited $status: $log", status == 0)
        return JSONArray(out.readText())
    }

    @Test
    fun everySyllableInEveryOrder() {
        val cases = generateCases()
        assumeTrue("python3 not available — the CI runners have it", cases != null)
        requireNotNull(cases)
        assertTrue("generator produced a suspiciously small matrix", cases.length() > 1000)

        val divergences = mutableListOf<String>()
        for (i in 0 until cases.length()) {
            val case = cases.getJSONObject(i)
            val keys = case.getString("keys")
            val expected = case.getString("expected")
            val actual = TelexTyping.type(keys)
            if (actual != expected) divergences += "$keys -> $actual, expected $expected"
        }
        if (divergences.isNotEmpty()) {
            val shown = divergences.take(MAX_REPORTED).joinToString("\n  ")
            throw AssertionError(
                "${divergences.size} divergence(s) of ${cases.length()} cases:\n  $shown"
            )
        }
    }

    private companion object {
        const val GENERATOR_PATH = "DraftRightMobile/tools/gen_telex_cases.py"

        /**
         * Divergences reported before truncating — enough to see the pattern
         * without burying the failure message in thousands of lines.
         */
        const val MAX_REPORTED = 50
    }
}
