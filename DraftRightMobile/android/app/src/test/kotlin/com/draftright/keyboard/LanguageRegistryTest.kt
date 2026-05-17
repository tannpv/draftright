package com.draftright.keyboard

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test
import java.util.Locale

class LanguageRegistryTest {

    private fun makeStub(idVal: String) = object : LanguagePack {
        override val id = idVal
        override val displayName = idVal.uppercase()
        override val locale: Locale = Locale.ENGLISH
        override val alphaRows = emptyList<List<KeyDef>>()
        override val symbols1Rows = emptyList<List<KeyDef>>()
        override val symbols2Rows = emptyList<List<KeyDef>>()
        override val longPressAccents = emptyMap<Char, List<Char>>()
    }

    @Test
    fun `byId returns the matching pack`() {
        val reg = LanguageRegistry(listOf(makeStub("en"), makeStub("vi")))
        assertEquals("vi", reg.byId("vi").id)
    }

    @Test
    fun `byId throws on unknown id`() {
        val reg = LanguageRegistry(listOf(makeStub("en")))
        assertThrows(NoSuchElementException::class.java) { reg.byId("fr") }
    }

    @Test
    fun `byIdOrDefault falls back to first when id unknown`() {
        val reg = LanguageRegistry(listOf(makeStub("en"), makeStub("vi")))
        assertEquals("en", reg.byIdOrDefault("zz").id)
    }

    @Test
    fun `next cycles in order with wrap-around`() {
        val reg = LanguageRegistry(listOf(makeStub("vi"), makeStub("fr"), makeStub("es")))
        assertEquals("fr", reg.next("vi").id)
        assertEquals("es", reg.next("fr").id)
        assertEquals("vi", reg.next("es").id)
    }

    @Test
    fun `next on unknown id returns first`() {
        val reg = LanguageRegistry(listOf(makeStub("en"), makeStub("vi")))
        assertEquals("en", reg.next("xx").id)
    }

    @Test
    fun `empty packs list throws`() {
        assertThrows(IllegalArgumentException::class.java) {
            LanguageRegistry(emptyList())
        }
    }
}
