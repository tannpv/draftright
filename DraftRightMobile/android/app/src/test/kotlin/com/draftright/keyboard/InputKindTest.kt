package com.draftright.keyboard

import org.junit.Assert.assertEquals
import org.junit.Test

class InputKindTest {
    @Test fun `apiValues`() {
        assertEquals("typed", InputKind.TYPED.apiValue)
        assertEquals("speech", InputKind.SPEECH.apiValue)
    }
}
