package com.draftright.keyboard.composer

/**
 * Composer for Korean: accumulates the typed jamo and exposes the live Hangul
 * assembly as the composing buffer. Korean input is deterministic composition
 * (no candidate bar) — the whole of it is [HangulAssembler].
 *
 * Reuses [BufferingComposer]; only the transform is Korean-specific. The raw
 * buffer holds compatibility jamo (the key labels), assembled into syllables.
 */
class HangulComposer : BufferingComposer() {
    override fun transform(raw: String): String = HangulAssembler.assemble(raw)
}
