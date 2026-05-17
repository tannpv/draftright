package com.draftright.keyboard

sealed class ComposeResult {
    object PassThrough : ComposeResult()
    data class Commit(val text: String) : ComposeResult()
    data class Composing(val text: String) : ComposeResult()
    object Consumed : ComposeResult()
}

interface Composer {
    fun onKey(char: Char): ComposeResult
    fun onBackspace(): ComposeResult
    fun reset()
    fun currentComposingText(): String
}
