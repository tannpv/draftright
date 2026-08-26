package com.draftright.keyboard.ime

import kotlin.math.abs

/** The five flick outcomes on a 12-key Japanese kana key (#212). */
enum class FlickDirection { TAP, LEFT, UP, RIGHT, DOWN }

/**
 * Resolves a touch movement into a flick direction (#212). Pure math — no
 * Android types — so it unit-tests directly and mirrors 1:1 in Swift.
 *
 * Screen coordinates: y increases downward, so an upward flick has a negative
 * dy. A movement shorter than [tapThreshold] on both axes is a plain tap; other-
 * wise the larger axis wins (horizontal ties resolve to horizontal, matching the
 * usual flick keyboard where left/right sit slightly closer than up/down).
 */
object FlickGesture {

    fun resolve(dx: Float, dy: Float, tapThreshold: Float): FlickDirection {
        if (abs(dx) < tapThreshold && abs(dy) < tapThreshold) return FlickDirection.TAP
        return if (abs(dx) >= abs(dy)) {
            if (dx < 0f) FlickDirection.LEFT else FlickDirection.RIGHT
        } else {
            if (dy < 0f) FlickDirection.UP else FlickDirection.DOWN
        }
    }
}
