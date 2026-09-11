package com.draftright.keyboard.lang

import com.draftright.keyboard.KeyboardController
import com.draftright.keyboard.LanguageRegistry
import com.draftright.keyboard.composer.KanaComposer
import com.draftright.keyboard.composer.RomajiKanaComposer
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Japanese types on the LATIN keyboard: the ja pack renders the same ASCII
 * QWERTY as English and converts rōmaji to kana in the composer, never in the
 * layout. The 12-key kana grid is opt-in (jpFlickEnabled, #212).
 *
 * This is the machine check behind "the Japanese keyboard IS the latin one" —
 * a kana key sneaking into the ja rows, or the flick composer being picked
 * while the flick switch is off, would silently change what users see.
 * Mirror of Swift JapaneseLatinLayoutTests.
 */
class JapaneseLatinLayoutTest {

    private val japanese = JapaneseLanguagePack
    private val english = EnglishLanguagePack

    private fun controller(activeId: String, jpFlick: Boolean) = KeyboardController(
        LanguageRegistry(listOf(EnglishLanguagePack, JapaneseLanguagePack)),
        enabledIds = listOf("en", "ja"),
        activeId = activeId,
        jpFlick = jpFlick,
    )

    @Test
    fun japaneseRendersTheSameRowsAsEnglish() {
        assertEquals(english.alphaRows, japanese.alphaRows)
        assertEquals(english.symbols1Rows, japanese.symbols1Rows)
        assertEquals(english.symbols2Rows, japanese.symbols2Rows)
    }

    @Test
    fun japaneseAlphaKeysAreLatinLetters() {
        val letters = japanese.alphaRows.flatten()
            .map { it.label }
            .filter { it.length == 1 && it[0].isLetter() }
        assertEquals("a QWERTY alpha layer has 26 letter keys", 26, letters.size)
        for (label in letters) {
            assertTrue(
                "non-ASCII key '$label' in the Japanese alpha rows",
                label[0].code in 'a'.code..'z'.code,
            )
        }
    }

    @Test
    fun romajiComposerIsUsedWhenFlickIsOff() {
        val controller = controller(activeId = "ja", jpFlick = false)
        assertTrue(
            "flick off must keep the rōmaji composer",
            controller.composer is RomajiKanaComposer,
        )
    }

    @Test
    fun flickComposerIsUsedOnlyWhenTheSwitchIsOn() {
        val controller = controller(activeId = "ja", jpFlick = true)
        assertTrue(
            "flick on must swap in the kana composer",
            controller.composer is KanaComposer,
        )
    }

    @Test
    fun flickSwitchNeverAffectsOtherLanguages() {
        val controller = controller(activeId = "en", jpFlick = true)
        assertTrue(
            "English must never get the kana composer",
            controller.composer !is KanaComposer,
        )
    }

    @Test
    fun typingRomajiOnTheLatinKeysProducesKana() {
        val composer = japanese.composer()
        for (key in "konnichiha") composer.onKey(key)
        assertEquals("こんにちは", composer.currentComposingText())
    }
}
