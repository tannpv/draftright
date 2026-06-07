package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Broad correctness corpus for Vietnamese Telex — one row per (keystrokes ->
 * expected text). Covers circumflex, horn/breve, đ, tone placement on di/
 * triphthongs, and the ươ cluster typed with a trailing vowel (rượu / người /
 * hươu), which previously horned the wrong vowel.
 */
class TelexCorpusTest {

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

    private val cases = linkedMapOf(
        // circumflex
        "aa" to "â", "oo" to "ô", "ee" to "ê",
        "caan" to "cân", "coon" to "côn",
        // đ
        "dd" to "đ", "ddi" to "đi",
        // single horn / breve
        "ow" to "ơ", "uw" to "ư", "aw" to "ă",
        // tones on a single vowel
        "as" to "á", "af" to "à", "ar" to "ả", "ax" to "ã", "aj" to "ạ",
        // diphthong promotion + tone
        "hoaf" to "hòa",
        "vietj" to "việt",
        "tieengs" to "tiếng",
        "nguyeenx" to "nguyễn",
        "ddoongf" to "đồng",
        // ươ cluster, classic ordering (w right after each vowel)
        "uow" to "ươ",
        "dduwowngf" to "đường",
        "nguwowif" to "người",
        "ruwowuj" to "rượu",
        // ươ cluster with a trailing vowel — the reported regressions
        "ruouwj" to "rượu",
        "ruwowju" to "rượu",
        "huouw" to "hươu",
        "nguoiwf" to "người",
        "muonwj" to "mượn",
        // Flexible modifier after trailing consonants (salvaged from the
        // superseded telex-circumflex branch) — both spellings reach the vowel.
        "nguyeexn" to "nguyễn",
        "nguyenex" to "nguyễn",
        "nguyene" to "nguyên",
        "vietej" to "việt",
        "kae" to "kae",       // no false circumflex when no matching vowel
        "bee" to "bê",
        "beee" to "bee",      // re-type cancels ê back to e + literal
    )

    @Test fun `telex corpus`() {
        val failures = cases.entries.mapNotNull { (keys, expected) ->
            val got = type(keys)
            if (got != expected) "$keys -> expected '$expected' but got '$got'" else null
        }
        assertEquals("Telex failures:\n" + failures.joinToString("\n"), 0, failures.size)
    }
}
