package com.draftright.keyboard

/**
 * The three states of the keyboard shift key, mirroring the stock Samsung
 * keyboard: OFF (lowercase), SINGLE (capitalize the next character only, then
 * auto-revert), CAPS_LOCK (capitalize every character until toggled off).
 */
enum class ShiftState { OFF, SINGLE, CAPS_LOCK }
