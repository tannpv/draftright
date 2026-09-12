package com.draftright.keyboard

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * #272: the app must be able to tell that its keyboard is installed but unusable.
 * Every fresh install starts in NOT_ENABLED, so this decision is what makes the
 * state recoverable in-app instead of silently dead.
 */
class KeyboardStatusTest {

    private val ours = "com.draftright.draftright_mobile.v2"
    private val ourIme = "$ours/com.draftright.keyboard.DraftRightIME"
    private val samsung = "com.samsung.android.honeyboard/.service.HoneyBoardService"

    @Test
    fun freshInstallIsNotEnabled() {
        assertEquals(
            KeyboardStatus.NOT_ENABLED,
            KeyboardStatus.resolve(listOf(samsung), samsung, ours),
        )
    }

    @Test
    fun enabledButAnotherKeyboardSelected() {
        assertEquals(
            KeyboardStatus.ENABLED_NOT_SELECTED,
            KeyboardStatus.resolve(listOf(samsung, ourIme), samsung, ours),
        )
    }

    @Test
    fun enabledAndSelectedIsActive() {
        assertEquals(
            KeyboardStatus.ACTIVE,
            KeyboardStatus.resolve(listOf(samsung, ourIme), ourIme, ours),
        )
    }

    @Test
    fun noDefaultImeAtAllIsNotSelected() {
        assertEquals(
            KeyboardStatus.ENABLED_NOT_SELECTED,
            KeyboardStatus.resolve(listOf(ourIme), null, ours),
        )
    }

    @Test
    fun matchesByPackageSoAClassRenameDoesNotBreakIt() {
        assertEquals(
            KeyboardStatus.ACTIVE,
            KeyboardStatus.resolve(listOf("$ours/.keyboard.DraftRightIME"), "$ours/.keyboard.DraftRightIME", ours),
        )
    }

    @Test
    fun aDifferentPackageWithASimilarNameIsNotUs() {
        val debugVariant = "$ours.debug/com.draftright.keyboard.DraftRightIME"
        assertEquals(
            KeyboardStatus.NOT_ENABLED,
            KeyboardStatus.resolve(listOf(debugVariant), debugVariant, ours),
        )
    }
}
