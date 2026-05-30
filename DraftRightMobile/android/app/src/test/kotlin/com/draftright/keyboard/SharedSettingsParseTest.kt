package com.draftright.keyboard

import com.draftright.keyboard.SharedSettings.Companion.JSON_LIST_PREFIX
import com.draftright.keyboard.SharedSettings.Companion.LEGACY_LIST_PREFIX
import com.draftright.keyboard.SharedSettings.Companion.parseStringList
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Locks the keyboard's enabled-language decoding against every
 * shared_preferences StringList encoding that can land on disk. A wrong
 * decode collapses the list to ["en"], which silently kills language
 * switching (globe/space-swipe become no-ops).
 */
class SharedSettingsParseTest {

    @Test fun `new json encoding with bang prefix`() {
        assertEquals(listOf("en", "vi"), parseStringList("$JSON_LIST_PREFIX[\"en\",\"vi\"]"))
    }

    @Test fun `legacy encoding without bang, kotlin toString`() {
        // Old plugin stored the platform's list toString: "[en, vi]" (unquoted).
        assertEquals(listOf("en", "vi"), parseStringList("$LEGACY_LIST_PREFIX[en, vi]"))
    }

    @Test fun `legacy encoding without bang, json payload`() {
        assertEquals(listOf("en", "fr"), parseStringList("$LEGACY_LIST_PREFIX[\"en\",\"fr\"]"))
    }

    @Test fun `bare json array with no prefix`() {
        assertEquals(listOf("en", "vi", "fr"), parseStringList("[\"en\",\"vi\",\"fr\"]"))
    }

    @Test fun `single language`() {
        assertEquals(listOf("vi"), parseStringList("$JSON_LIST_PREFIX[\"vi\"]"))
    }

    @Test fun `null and blank yield empty`() {
        assertEquals(emptyList<String>(), parseStringList(null))
        assertEquals(emptyList<String>(), parseStringList(""))
        assertEquals(emptyList<String>(), parseStringList("   "))
    }

    @Test fun `empty json array yields empty`() {
        assertEquals(emptyList<String>(), parseStringList("$JSON_LIST_PREFIX[]"))
    }
}
