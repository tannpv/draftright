package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * Sentence-level pinyin segmentation (#211). TC: ZHPY-1..6
 */
class PinyinSegmenterTest {

    @Test fun `two-syllable word`() {
        assertEquals(listOf("wo", "shi"), PinyinSegmenter.segment("woshi"))
        assertEquals(listOf("ni", "hao"), PinyinSegmenter.segment("nihao"))
    }

    @Test fun `prefers the longest syllable`() {
        // "xian" is one syllable, not xi+an.
        assertEquals(listOf("xian"), PinyinSegmenter.segment("xian"))
        // place name: xiang + gang, not xi+ang+gang.
        assertEquals(listOf("xiang", "gang"), PinyinSegmenter.segment("xianggang"))
    }

    @Test fun `multi-syllable sentence`() {
        assertEquals(listOf("wo", "men", "shi"), PinyinSegmenter.segment("womenshi"))
        assertEquals(listOf("zhong", "guo", "ren"), PinyinSegmenter.segment("zhongguoren"))
    }

    @Test fun `single syllable`() {
        assertEquals(listOf("hao"), PinyinSegmenter.segment("hao"))
    }

    @Test fun `invalid or partial pinyin returns null`() {
        assertNull(PinyinSegmenter.segment("xyz"))
        // "woxq" — wo is valid but "xq" can't segment → whole thing fails.
        assertNull(PinyinSegmenter.segment("woxq"))
    }

    @Test fun `empty is empty`() {
        assertEquals(emptyList<String>(), PinyinSegmenter.segment(""))
    }

    @Test fun `backtracks when a greedy choice blocks the rest`() {
        // Longest-first tries "xie", but the trailing "r" can't segment, so it
        // backtracks to "xi" + "er". Proves it doesn't fail on a bad greedy pick.
        assertEquals(listOf("xi", "er"), PinyinSegmenter.segment("xier"))
    }
}
