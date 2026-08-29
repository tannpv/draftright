package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Kana dakuten/handakuten/small cycling (#212 phase 3). TC: JPFLICK-7
 */
class KanaModifierTest {

    @Test fun `dakuten toggles k s t z rows`() {
        assertEquals("が", KanaModifier.cycle("か"))
        assertEquals("か", KanaModifier.cycle("が")) // wraps back
        assertEquals("じ", KanaModifier.cycle("し"))
        assertEquals("だ", KanaModifier.cycle("た"))
    }

    @Test fun `h row cycles base to dakuten to handakuten`() {
        assertEquals("ば", KanaModifier.cycle("は"))
        assertEquals("ぱ", KanaModifier.cycle("ば"))
        assertEquals("は", KanaModifier.cycle("ぱ")) // wraps
    }

    @Test fun `tsu cycles small then dakuten`() {
        assertEquals("っ", KanaModifier.cycle("つ"))
        assertEquals("づ", KanaModifier.cycle("っ"))
        assertEquals("つ", KanaModifier.cycle("づ"))
    }

    @Test fun `ya row and vowels get small`() {
        assertEquals("ゃ", KanaModifier.cycle("や"))
        assertEquals("ぁ", KanaModifier.cycle("あ"))
    }

    @Test fun `kana without a variant is unchanged`() {
        assertEquals("な", KanaModifier.cycle("な"))
        assertEquals("ん", KanaModifier.cycle("ん"))
        assertEquals("ら", KanaModifier.cycle("ら"))
    }
}
