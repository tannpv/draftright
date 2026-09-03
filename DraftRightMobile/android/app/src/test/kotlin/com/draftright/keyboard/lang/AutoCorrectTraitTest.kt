package com.draftright.keyboard.lang

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Auto-correct-on-space (#207) is a pack trait, never a check on the language
 * name: a pack opts in when its dictionary is good enough to override the user.
 * Vietnamese ships the ~8.5k frequency list, so it's the only one on today.
 */
class AutoCorrectTraitTest {

    @Test
    fun vietnameseOptsIn() {
        assertTrue(VietnameseLanguagePack.autoCorrectEnabled)
    }

    @Test
    fun otherPacksAreOffByDefault() {
        assertFalse(EnglishLanguagePack.autoCorrectEnabled)
        assertFalse(JapaneseLanguagePack.autoCorrectEnabled)
    }
}
