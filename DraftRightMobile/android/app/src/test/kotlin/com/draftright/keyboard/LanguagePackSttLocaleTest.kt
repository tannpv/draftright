package com.draftright.keyboard

import com.draftright.keyboard.lang.EnglishLanguagePack
import com.draftright.keyboard.lang.JapaneseLanguagePack
import com.draftright.keyboard.lang.VietnameseLanguagePack
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class LanguagePackSttLocaleTest {
    @Test
    fun `english and vietnamese expose stt locales`() {
        assertEquals("en-US", EnglishLanguagePack.sttLocale)
        assertEquals("vi-VN", VietnameseLanguagePack.sttLocale)
    }

    @Test
    fun `packs without voice support return null`() {
        assertNull(JapaneseLanguagePack.sttLocale)
    }
}
