package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Fuzzy-pinyin folding (#211). TC: ZHPY-15
 */
class PinyinFuzzyTest {

    @Test fun `retroflex initials fold to dental`() {
        assertEquals("si", PinyinFuzzy.fold("shi"))
        assertEquals("zongguo", PinyinFuzzy.fold("zhongguo"))
        assertEquals("can", PinyinFuzzy.fold("chang")) // ch->c then ang->an
    }

    @Test fun `nasal finals fold`() {
        assertEquals("jin", PinyinFuzzy.fold("jing"))
        assertEquals("ban", PinyinFuzzy.fold("bang"))
        assertEquals("min", PinyinFuzzy.fold("ming"))
    }

    @Test fun `already-plain pinyin is unchanged`() {
        assertEquals("ni", PinyinFuzzy.fold("ni"))
        assertEquals("hao", PinyinFuzzy.fold("hao"))
    }
}
