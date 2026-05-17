package com.draftright.keyboard

import com.draftright.keyboard.lang.EnglishLanguagePack
import org.junit.Assert.assertEquals
import org.junit.Test
import java.util.Locale

class KeyboardControllerTest {

    private fun stub(idVal: String) = object : LanguagePack {
        override val id = idVal
        override val displayName = idVal.uppercase()
        override val locale: Locale = Locale.ENGLISH
        override val alphaRows = listOf(listOf(KeyDef("a", 'a'.code)))
        override val symbols1Rows = emptyList<List<KeyDef>>()
        override val symbols2Rows = emptyList<List<KeyDef>>()
        override val longPressAccents = emptyMap<Char, List<Char>>()
    }

    @Test
    fun `init defaults to first enabled when activeId is empty`() {
        val reg = LanguageRegistry(listOf(EnglishLanguagePack))
        val ctrl = KeyboardController(reg, enabledIds = listOf("en"), activeId = "")
        assertEquals("en", ctrl.current.id)
    }

    @Test
    fun `init honors activeId when present in enabled subset`() {
        val reg = LanguageRegistry(listOf(stub("vi"), stub("fr"), stub("es")))
        val ctrl = KeyboardController(reg, enabledIds = listOf("vi", "es"), activeId = "es")
        assertEquals("es", ctrl.current.id)
    }

    @Test
    fun `cycle wraps within enabled subset, ignoring disabled`() {
        val reg = LanguageRegistry(listOf(stub("vi"), stub("fr"), stub("es")))
        val ctrl = KeyboardController(reg, enabledIds = listOf("vi", "es"), activeId = "vi")
        assertEquals("vi", ctrl.current.id)
        ctrl.cycleLanguage()
        assertEquals("es", ctrl.current.id)
        ctrl.cycleLanguage()
        assertEquals("vi", ctrl.current.id)
    }

    @Test
    fun `cycle is no-op when only one enabled`() {
        val reg = LanguageRegistry(listOf(EnglishLanguagePack))
        val ctrl = KeyboardController(reg, enabledIds = listOf("en"), activeId = "en")
        ctrl.cycleLanguage()
        assertEquals("en", ctrl.current.id)
    }

    @Test
    fun `disabled all force-enables registry first`() {
        val reg = LanguageRegistry(listOf(EnglishLanguagePack))
        val ctrl = KeyboardController(reg, enabledIds = emptyList(), activeId = "")
        assertEquals("en", ctrl.current.id)
        assertEquals(listOf("en"), ctrl.enabled.map { it.id })
    }

    @Test
    fun `setActive switches to enabled pack`() {
        val reg = LanguageRegistry(listOf(stub("vi"), stub("fr")))
        val ctrl = KeyboardController(reg, enabledIds = listOf("vi", "fr"), activeId = "vi")
        ctrl.setActive("fr")
        assertEquals("fr", ctrl.current.id)
    }

    @Test
    fun `setActive is no-op for disabled or same-id`() {
        val reg = LanguageRegistry(listOf(stub("vi"), stub("fr"), stub("es")))
        val ctrl = KeyboardController(reg, enabledIds = listOf("vi", "fr"), activeId = "vi")
        ctrl.setActive("es")
        assertEquals("vi", ctrl.current.id)
        ctrl.setActive("vi")
        assertEquals("vi", ctrl.current.id)
    }
}
