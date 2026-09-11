package com.draftright.keyboard.composer

import java.io.File
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Telex order-free marking must match the shared golden vectors (#207, RULE #1).
 *
 * `parity/telex-order-vectors.json` at the repo root is the single source of
 * truth; the iOS `TelexOrderVectorsTests` asserts against the same file, so the
 * Kotlin and Swift composers cannot drift apart.
 */
class TelexOrderVectorsTest {

    private val vectorsFile: File
        get() {
            // Tests run with the module dir as CWD; climb to the repo root.
            var dir: File? = File(".").absoluteFile
            while (dir != null && !File(dir, VECTORS_PATH).isFile) {
                dir = dir.parentFile
            }
            requireNotNull(dir) { "could not locate $VECTORS_PATH above ${File(".").absolutePath}" }
            return File(dir, VECTORS_PATH)
        }

    @Test
    fun goldenVectors() {
        val cases = JSONObject(vectorsFile.readText()).getJSONArray("cases")
        assertTrue("vectors file parsed to zero cases", cases.length() > 0)
        for (i in 0 until cases.length()) {
            val case = cases.getJSONObject(i)
            assertEquals(
                "case: ${case.getString("name")}",
                case.getString("expected"),
                TelexTyping.type(case.getString("keys")),
            )
        }
    }

    private companion object {
        const val VECTORS_PATH = "parity/telex-order-vectors.json"
    }
}
