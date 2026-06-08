package com.draftright.keyboard

import com.draftright.keyboard.composer.PassthroughComposer
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.Locale

class LanguagePackTest {
    private val stub = object : LanguagePack {
        override val id = "stub"
        override val displayName = "Stub"
        override val locale: Locale = Locale.ENGLISH
        override val alphaRows = listOf(
            listOf(KeyDef("a", 'a'.code), KeyDef("b", 'b'.code))
        )
        override val symbols1Rows = emptyList<List<KeyDef>>()
        override val symbols2Rows = emptyList<List<KeyDef>>()
        override val longPressAccents: Map<Char, List<Char>> = emptyMap()
    }

    @Test
    fun `id and displayName are exposed`() {
        assertEquals("stub", stub.id)
        assertEquals("Stub", stub.displayName)
    }

    @Test
    fun `composer factory defaults to passthrough`() {
        assertTrue(stub.composer() is PassthroughComposer)
    }

    @Test
    fun `KeyDef carries label code and width weight`() {
        val k = KeyDef("ñ", 'ñ'.code, widthWeight = 1.5f)
        assertEquals("ñ", k.label)
        assertEquals('ñ'.code, k.code)
        assertEquals(1.5f, k.widthWeight, 0f)
    }
}
