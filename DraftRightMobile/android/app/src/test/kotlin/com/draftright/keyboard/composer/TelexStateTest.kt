package com.draftright.keyboard.composer

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class TelexStateTest {

    @Test
    fun `isVowel detects a e i o u y and uppercase`() {
        listOf('a', 'e', 'i', 'o', 'u', 'y', 'A', 'E', 'I', 'O', 'U', 'Y').forEach {
            assertTrue("$it should be vowel", TelexState.isVowel(it))
        }
        listOf('b', 'q', 'z').forEach {
            assertFalse("$it should not be vowel", TelexState.isVowel(it))
        }
    }

    @Test
    fun `isVowelLike includes special VN vowels`() {
        listOf('ă', 'â', 'ê', 'ô', 'ơ', 'ư').forEach {
            assertTrue("$it should be vowel-like", TelexState.isVowelLike(it))
        }
    }

    @Test
    fun `isToneMark detects s f r x j`() {
        listOf('s', 'f', 'r', 'x', 'j').forEach {
            assertTrue("$it should be tone mark", TelexState.isToneMark(it))
        }
        assertFalse(TelexState.isToneMark('a'))
    }
}
