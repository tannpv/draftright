package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import org.junit.Test

/**
 * Probe: does the composer support "trailing modifier" Telex — type all base
 * letters first, then the mark(s)/tone at the END of the syllable?
 * e.g. truongwf -> trường, nguyenex -> nguyễn.  Reports actual behaviour; does
 * not assert (measurement only).
 */
class TelexTrailingProbeTest {
    private fun type(keys: String): String {
        val c = TelexComposer()
        var last: ComposeResult = ComposeResult.PassThrough
        for (ch in keys) last = c.onKey(ch)
        return when (last) {
            is ComposeResult.Composing -> last.text
            is ComposeResult.Commit -> last.text
            else -> c.currentComposingText()
        }
    }

    @Test fun `trailing modifier probe`() {
        val cases = linkedMapOf(
            // user's examples
            "truongwf" to "trường",
            "nguyenex" to "nguyễn",
            // horn at end
            "duongwf" to "dường",
            "huongwf" to "hường",
            "cuongwf" to "cường",
            "vuonwj" to "vượn",
            "muonws" to "mướn",
            // circumflex at end (extra vowel appended)
            "vietej" to "việt",
            "tienges" to "tiếng",
            "conof" to "cồn",
            "canaf" to "cần",
            "hoconf" to null, // control-ish
            // horn on single vowel at end
            "tuws" to "tứ",
            "muws" to "mứ",
            // combos: circumflex + tone at end
            "nguyeenx" to "nguyễn",   // classic (control, should pass)
            "nguyenxe" to "nguyễn",   // tone before circumflex, both trailing
            "toanf" to "toàn",
            "hoaf" to "hòa",
        )
        val out = StringBuilder("\nTrailing-modifier Telex probe:\n")
        var ok = 0; var n = 0
        for ((keys, want) in cases) {
            val got = type(keys)
            val mark = when {
                want == null -> "  (no expectation) got '$got'"
                got == want -> { ok++; "OK" }
                else -> "FAIL want '$want' got '$got'"
            }
            if (want != null) n++
            out.append("  %-12s -> %s\n".format(keys, mark))
        }
        out.append("  === $ok/$n trailing-style cases pass ===\n")
        println(out.toString())
    }
}
