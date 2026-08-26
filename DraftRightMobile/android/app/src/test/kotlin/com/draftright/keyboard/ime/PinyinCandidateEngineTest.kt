package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Sentence-level pinyin candidates (#211). TC: ZHPY-7..10
 */
class PinyinCandidateEngineTest {

    private val dict = mapOf(
        "ni" to listOf("你"),
        "hao" to listOf("好"),
        "nihao" to listOf("你好"),
        "wo" to listOf("我"),
        "shi" to listOf("是", "时"),
        "men" to listOf("们"),
        "women" to listOf("我们"),
        "beijing" to listOf("北京"),
    )
    private val engine = PinyinCandidateEngine(dict)

    private fun texts(s: String) = engine.suggest(s, emptyList(), 7).map { it.text }

    @Test fun `run-together pinyin builds a segmented hanzi candidate`() {
        // "woshi" is not a dict word; segment wo+shi → 我 + 是 → 我是.
        assertTrue(texts("woshi").contains("我是"))
    }

    @Test fun `three syllables`() {
        // wo+men+shi → 我们是
        assertTrue(texts("womenshi").contains("我们是"))
    }

    @Test fun `raw pinyin is still offered as a fallback`() {
        assertTrue(texts("woshi").contains("woshi"))
    }

    @Test fun `exact dictionary word ranks above the segmented form`() {
        // "nihao" is an exact word (你好); it must come before/at the raw fallback.
        val t = texts("nihao")
        assertTrue(t.contains("你好"))
        assertEquals("你好", t.first())
    }

    @Test fun `single syllable is unchanged (no segmentation)`() {
        assertEquals(listOf("我"), texts("wo").filter { it != "wo" })
    }

    @Test fun `unsegmentable pinyin falls back to the base engine`() {
        assertEquals(listOf("xyz"), texts("xyz"))
    }

    @Test fun `initials abbreviation commits the whole word`() {
        // "nh" = ni-hao initials → 你好; "bj" = bei-jing initials → 北京.
        assertTrue(texts("nh").contains("你好"))
        assertTrue(texts("bj").contains("北京"))
    }

    @Test fun `abbreviation of a two-syllable word`() {
        // "wm" = wo-men → 我们.
        assertTrue(texts("wm").contains("我们"))
    }
}
