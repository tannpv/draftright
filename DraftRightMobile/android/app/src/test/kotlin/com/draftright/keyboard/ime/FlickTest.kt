package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * Japanese flick core (#212): gesture resolution + kana map. TC: JPFLICK-1..4
 */
class FlickTest {

    private val threshold = 30f

    @Test fun `small movement is a tap`() {
        assertEquals(FlickDirection.TAP, FlickGesture.resolve(0f, 0f, threshold))
        assertEquals(FlickDirection.TAP, FlickGesture.resolve(10f, -12f, threshold))
    }

    @Test fun `directions resolve by dominant axis`() {
        assertEquals(FlickDirection.LEFT, FlickGesture.resolve(-100f, 5f, threshold))
        assertEquals(FlickDirection.RIGHT, FlickGesture.resolve(100f, -5f, threshold))
        assertEquals(FlickDirection.UP, FlickGesture.resolve(5f, -100f, threshold))    // y up = negative
        assertEquals(FlickDirection.DOWN, FlickGesture.resolve(-5f, 100f, threshold))
    }

    @Test fun `uniform gojuon rows map vowels`() {
        assertEquals("い", FlickLayout.kanaFor("あ", FlickDirection.LEFT))
        assertEquals("こ", FlickLayout.kanaFor("か", FlickDirection.DOWN))
        assertEquals("つ", FlickLayout.kanaFor("た", FlickDirection.UP))
        assertEquals("せ", FlickLayout.kanaFor("さ", FlickDirection.RIGHT))
        assertEquals("な", FlickLayout.kanaFor("な", FlickDirection.TAP))
    }

    @Test fun `special rows - ya has three kana, wa has n and choonpu`() {
        assertEquals("ゆ", FlickLayout.kanaFor("や", FlickDirection.UP))
        assertEquals("よ", FlickLayout.kanaFor("や", FlickDirection.DOWN))
        assertNull(FlickLayout.kanaFor("や", FlickDirection.LEFT)) // や has no left kana
        assertEquals("ん", FlickLayout.kanaFor("わ", FlickDirection.UP))
        assertEquals("を", FlickLayout.kanaFor("わ", FlickDirection.LEFT))
        assertEquals("ー", FlickLayout.kanaFor("わ", FlickDirection.RIGHT))
    }
}
