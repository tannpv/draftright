package com.draftright.keyboard.lang

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** Covers FR / ES / DE / IT / PT in one suite — each pack gets focused
 *  invariant assertions (id, displayName, distinctive layout markers,
 *  accent-map shape). */
class LatinLanguagePackTest {

    // --- French (AZERTY) ---

    @Test
    fun `French id and displayName`() {
        assertEquals("fr", FrenchLanguagePack.id)
        assertEquals("Français", FrenchLanguagePack.displayName)
    }

    @Test
    fun `French AZERTY top row starts with a`() {
        assertEquals("a", FrenchLanguagePack.alphaRows[0].first().label)
    }

    @Test
    fun `French home row ends with m (AZERTY)`() {
        assertEquals("m", FrenchLanguagePack.alphaRows[1].last().label)
    }

    @Test
    fun `French accents include é è ê ë on e`() {
        val accents = FrenchLanguagePack.longPressAccents['e'] ?: emptyList()
        listOf('é', 'è', 'ê', 'ë').forEach { assertTrue("missing $it", it in accents) }
    }

    @Test
    fun `French composer is passthrough`() =
        assertTrue(FrenchLanguagePack.composer() is com.draftright.keyboard.composer.PassthroughComposer)

    // --- Spanish ---

    @Test
    fun `Spanish id and displayName`() {
        assertEquals("es", SpanishLanguagePack.id)
        assertEquals("Español", SpanishLanguagePack.displayName)
    }

    @Test
    fun `Spanish home row ends with ñ`() {
        assertEquals("ñ", SpanishLanguagePack.alphaRows[1].last().label)
    }

    @Test
    fun `Spanish accents include inverted punctuation`() {
        assertTrue('¿' in (SpanishLanguagePack.longPressAccents['?'] ?: emptyList()))
        assertTrue('¡' in (SpanishLanguagePack.longPressAccents['!'] ?: emptyList()))
    }

    // --- German (QWERTZ) ---

    @Test
    fun `German id and displayName`() {
        assertEquals("de", GermanLanguagePack.id)
        assertEquals("Deutsch", GermanLanguagePack.displayName)
    }

    @Test
    fun `German top row sixth key is z (QWERTZ)`() {
        assertEquals("z", GermanLanguagePack.alphaRows[0][5].label)
    }

    @Test
    fun `German alpha rows include ä ö ü and ß`() {
        val all = GermanLanguagePack.alphaRows.flatten().map { it.label }
        listOf("ä", "ö", "ü", "ß").forEach { assertTrue("missing $it", it in all) }
    }

    // --- Italian ---

    @Test
    fun `Italian id and displayName`() {
        assertEquals("it", ItalianLanguagePack.id)
        assertEquals("Italiano", ItalianLanguagePack.displayName)
    }

    @Test
    fun `Italian QWERTY identical to English alphaRows`() {
        assertEquals(EnglishLanguagePack.alphaRows, ItalianLanguagePack.alphaRows)
    }

    @Test
    fun `Italian accents lead with grave forms`() {
        assertEquals('à', ItalianLanguagePack.longPressAccents['a']?.first())
        assertEquals('è', ItalianLanguagePack.longPressAccents['e']?.first())
    }

    // --- Portuguese ---

    @Test
    fun `Portuguese id and displayName`() {
        assertEquals("pt", PortugueseLanguagePack.id)
        assertEquals("Português", PortugueseLanguagePack.displayName)
    }

    @Test
    fun `Portuguese home row ends with ç`() {
        assertEquals("ç", PortugueseLanguagePack.alphaRows[1].last().label)
    }

    @Test
    fun `Portuguese accents include tilde forms`() {
        assertTrue('ã' in (PortugueseLanguagePack.longPressAccents['a'] ?: emptyList()))
        assertTrue('õ' in (PortugueseLanguagePack.longPressAccents['o'] ?: emptyList()))
    }
}
