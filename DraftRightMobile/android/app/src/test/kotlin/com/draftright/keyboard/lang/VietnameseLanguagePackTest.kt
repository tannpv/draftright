package com.draftright.keyboard.lang

import com.draftright.keyboard.composer.TelexComposer
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class VietnameseLanguagePackTest {

    @Test
    fun `id and displayName`() {
        assertEquals("vi", VietnameseLanguagePack.id)
        assertEquals("Tiếng Việt", VietnameseLanguagePack.displayName)
    }

    @Test
    fun `composer factory returns a fresh TelexComposer`() {
        val a = VietnameseLanguagePack.composer()
        val b = VietnameseLanguagePack.composer()
        assertTrue(a is TelexComposer)
        assertTrue(b is TelexComposer)
        assertTrue("expected distinct composer instances", a !== b)
    }

    @Test
    fun `alphaRows mirror English QWERTY shape`() {
        assertEquals(4, VietnameseLanguagePack.alphaRows.size)
        assertEquals(EnglishLanguagePack.alphaRows[0].size, VietnameseLanguagePack.alphaRows[0].size)
    }
}
