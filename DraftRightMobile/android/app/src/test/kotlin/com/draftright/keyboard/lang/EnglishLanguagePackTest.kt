package com.draftright.keyboard.lang

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class EnglishLanguagePackTest {

    @Test
    fun `id is en and displayName is English`() {
        assertEquals("en", EnglishLanguagePack.id)
        assertEquals("English", EnglishLanguagePack.displayName)
    }

    @Test
    fun `alphaRows has four rows total`() {
        assertEquals(4, EnglishLanguagePack.alphaRows.size)
    }

    @Test
    fun `top row starts with q and ends with p`() {
        val top = EnglishLanguagePack.alphaRows[0]
        assertEquals("q", top.first().label)
        assertEquals("p", top.last().label)
        assertEquals(10, top.size)
    }

    @Test
    fun `home row starts with a and ends with l`() {
        val home = EnglishLanguagePack.alphaRows[1]
        assertEquals("a", home.first().label)
        assertEquals("l", home.last().label)
        assertEquals(9, home.size)
    }

    @Test
    fun `composer factory returns passthrough`() {
        assertTrue(EnglishLanguagePack.composer() is com.draftright.keyboard.composer.PassthroughComposer)
    }

    @Test
    fun `long press accents is empty`() {
        assertTrue(EnglishLanguagePack.longPressAccents.isEmpty())
    }

    @Test
    fun `symbols1 first row is digits 1 through 0`() {
        val digits = EnglishLanguagePack.symbols1Rows[0]
        assertEquals("1", digits.first().label)
        assertEquals("0", digits.last().label)
        assertEquals(10, digits.size)
    }

    @Test
    fun `symbols2 first row contains tilde and pi`() {
        val row = EnglishLanguagePack.symbols2Rows[0]
        assertEquals("~", row.first().label)
        assertTrue(row.any { it.label == "π" })
    }

    @Test
    fun `symbols1 second row contains underscore`() {
        val row = EnglishLanguagePack.symbols1Rows[1]
        assertTrue(row.any { it.label == "_" })
    }
}
