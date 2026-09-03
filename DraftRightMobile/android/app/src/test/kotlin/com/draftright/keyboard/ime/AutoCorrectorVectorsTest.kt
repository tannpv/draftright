package com.draftright.keyboard.ime

import java.io.File
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Auto-correct decisions must match the shared golden vectors (#207, RULE #1).
 *
 * `parity/autocorrect-vectors.json` at the repo root is the single source of
 * truth; the iOS `AutoCorrectorVectorsTests` asserts against the same file, so
 * the Kotlin and Swift deciders cannot drift apart.
 */
class AutoCorrectorVectorsTest {

    private val vectorsFile: File
        get() {
            // Tests run with the module dir as CWD; climb to the repo root.
            var dir: File? = File(".").absoluteFile
            while (dir != null && !File(dir, "parity/autocorrect-vectors.json").isFile) {
                dir = dir.parentFile
            }
            requireNotNull(dir) { "could not locate parity/autocorrect-vectors.json above ${File(".").absolutePath}" }
            return File(dir, "parity/autocorrect-vectors.json")
        }

    @Test
    fun goldenVectors() {
        val cases = JSONObject(vectorsFile.readText()).getJSONArray("cases")
        assertTrue("vectors file parsed to zero cases", cases.length() > 0)
        for (i in 0 until cases.length()) {
            val case = cases.getJSONObject(i)
            val name = case.getString("name")
            val dict = case.getJSONArray("dict")
            val entries = (0 until dict.length()).map { j ->
                val row = dict.getJSONArray(j)
                row.getString(0) to row.getInt(1)
            }
            val expect = if (case.isNull("expect")) null else case.getString("expect")
            val actual = AutoCorrector.correct(case.getString("token"), InMemoryWordList(entries))
            assertEquals("case: $name", expect, actual)
        }
    }
}
