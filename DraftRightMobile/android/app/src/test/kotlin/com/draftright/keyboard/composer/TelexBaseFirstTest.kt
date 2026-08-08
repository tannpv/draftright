package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Base-first Telex: type all the base letters, then the marks at the end (#152).
 *
 * This already worked for syllables ending in a CONSONANT ("canaf" -> cần).
 * It failed the moment the syllable ended in a vowel, because the lookback
 * stopped at the first vowel-like char it met — for "day" that is the 'y',
 * which is not the letter typed, so the modifier was inserted as a literal and
 * a following tone landed on the wrong vowel ("dayaj" -> "daỵa").
 *
 * Vowel-final syllables are a large slice of everyday Vietnamese, so this is
 * the difference between the keyboard feeling reliable and feeling arbitrary.
 */
class TelexBaseFirstTest {

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

    // ── The reported cases ────────────────────────────────────────────────

    @Test fun `dayaj composes dậy - the reported case`() = assertEquals("dậy", type("dayaj"))
    @Test fun `daya composes dây - circumflex after a trailing vowel`() = assertEquals("dây", type("daya"))
    @Test fun `mayas composes mấy`() = assertEquals("mấy", type("mayas"))
    @Test fun `tayas composes tấy`() = assertEquals("tấy", type("tayas"))
    @Test fun `caya composes cây`() = assertEquals("cây", type("caya"))

    // ── Inline typing must be unaffected ──────────────────────────────────

    @Test fun `daayj still composes dậy inline`() = assertEquals("dậy", type("daayj"))
    @Test fun `maays still composes mấy inline`() = assertEquals("mấy", type("maays"))
    @Test fun `toois still composes tối`() = assertEquals("tối", type("toois"))

    // ── Base-first past consonants keeps working ──────────────────────────

    @Test fun `canaf still composes cần`() = assertEquals("cần", type("canaf"))
    @Test fun `nguyenex still composes nguyễn`() = assertEquals("nguyễn", type("nguyenex"))
    @Test fun `truongwf still composes trường`() = assertEquals("trường", type("truongwf"))
    @Test fun `vietej still composes việt`() = assertEquals("việt", type("vietej"))

    // ── The guard: a modifier must NOT reach past another nucleus vowel ───
    //
    // "oeo" (ngoẻo) and "oao" are real three-vowel clusters whose final letter
    // is a literal vowel, not a modifier for the first one. Letting the scan
    // cross a nucleus turned these into "ôe"/"ôa" — 312 corpus regressions on
    // the first attempt at this fix.

    @Test fun `oeo stays literal - o is a nucleus, not a modifier`() = assertEquals("oeo", type("oeo"))
    @Test fun `oao stays literal`() = assertEquals("oao", type("oao"))
    @Test fun `boeo stays literal`() = assertEquals("boeo", type("boeo"))
    @Test fun `coaos stays literal with its tone`() = assertEquals("coáo", type("coaos"))

    // ── Cancel-by-retype still works through the same path ────────────────

    @Test fun `aa gives â and aaa cancels back`() {
        assertEquals("â", type("aa"))
        assertEquals("aa", type("aaa"))
    }

    @Test fun `oo gives ô and ooo cancels back`() {
        assertEquals("ô", type("oo"))
        assertEquals("oo", type("ooo"))
    }

    @Test fun `hoaf composes hòa - two-vowel cluster with a tone`() = assertEquals("hòa", type("hoaf"))
    @Test fun `loan is untouched when no modifier follows`() = assertEquals("loan", type("loan"))
}
