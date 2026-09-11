package com.draftright.keyboard.composer

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Samsung-parity order-free marking (spec 2026-09-11): tone keys, w, doubled
 * vowels, and trailing d commute at word scope. Every expected value below was
 * verified against Samsung Honeyboard VI-Telex on a Galaxy A52 (2026-09-11)
 * or derived from the same rule. Mirror of Swift TelexOrderFreeTests.
 */
class TelexOrderFreeTest {

    private fun type(keys: String): String = TelexTyping.type(keys)

    // --- Mechanic 1: quality modifier AFTER tone (tone-transparent) ---
    @Test fun toneThenHorn() {
        assertEquals("tưởng", type("tuongrw"))      // device-verified vs Samsung
        assertEquals("người", type("nguoifw"))      // device-verified vs Samsung
        assertEquals("mứa", type("muasw"))          // tone must MOVE u→ư target
    }

    @Test fun toneThenCircumflex() {
        assertEquals("cận", type("cansja"))         // tone j then late aa
        assertEquals("đồng", type("ddongfo"))       // tone f then late oo
    }

    // --- Mechanic 2: quality marks REPLACE each other (root matching) ---
    @Test fun hornOverridesCircumflex() {
        assertEquals("ơ", type("oow"))              // ô + w → ơ
        assertEquals("được", type("duocjdw"))       // j promotes uo→uô, w re-horns
    }

    @Test fun circumflexOverridesHorn() {
        assertEquals("ô", type("owo"))              // ơ + o → ô
    }

    // --- Mechanic 3: remote đ ---
    @Test fun remoteD() {
        assertEquals("đuoc", type("duocd"))         // trailing d pairs with initial
        assertEquals("được", type("duocdwj"))       // device-verified vs Samsung
        assertEquals("Được", type("Duocdwj"))       // case preserved from initial
        assertEquals("đùng", type("dungfd"))        // remote d after tone
    }

    @Test fun remoteDCancel() {
        assertEquals("duocd", type("duocdd"))       // second remote d reverts + literal
    }

    @Test fun remoteDDoesNotFireWithoutDInitial() {
        assertEquals("bad", type("bad"))            // d stays literal
    }

    // --- Full permutation of the study word: every mark order → Được ---
    @Test fun duocAllMarkOrders() {
        for (marks in listOf("dwj", "djw", "wdj", "wjd", "jdw", "jwd")) {
            assertEquals("marks=$marks", "được", type("duoc$marks"))
        }
    }

    // --- Must-not-regress spot checks (already green today; guard rail) ---
    @Test fun existingBehavioursHold() {
        assertEquals("tưởng", type("tuongwr"))
        assertEquals("không", type("khongo"))
        assertEquals("hòa", type("hoaf"))
        assertEquals("nguyễn", type("nguyeenx"))
        assertEquals("rượu", type("ruwowuj"))
    }

    // --- liftTone: tone removal for order-free marking ---
    @Test fun liftToneStripsAndReports() {
        assertEquals("tuong" to 'r', TelexComposer.liftTone("tuỏng"))
        assertEquals("tương" to 'r', TelexComposer.liftTone("tưởng"))  // quality marks stay
        assertEquals("duoc" to null, TelexComposer.liftTone("duoc"))
        assertEquals("Hoa" to 'f', TelexComposer.liftTone("Hòa"))      // case preserved
    }
}
