package com.draftright.keyboard

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * The three shift states must map to three distinct Material icons, so a user
 * can tell "cap first" (SINGLE) from "normal" (OFF) at a glance — the stock
 * Samsung keyboard shows a different icon for each.
 */
class ShiftIconTest {

    @Test fun `each state has a non-blank icon name`() {
        ShiftState.values().forEach { assertTrue(it.name, it.iconName().isNotBlank()) }
    }

    @Test fun `all three icon names are distinct`() {
        val names = ShiftState.values().map { it.iconName() }
        assertEquals(names.size, names.toSet().size)
    }
}
