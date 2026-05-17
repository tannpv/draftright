package com.draftright.keyboard.composer

import org.junit.Assert.assertTrue
import org.junit.Test

class TelexComposerPerfTest {

    @Test
    fun `onKey p95 stays under 1ms over a 1000-keystroke session`() {
        val c = TelexComposer()
        val word = "vietj " // typical Vietnamese word + space break
        val times = LongArray(1000)
        for (i in 0 until 1000) {
            val ch = word[i % word.length]
            val start = System.nanoTime()
            c.onKey(ch)
            times[i] = System.nanoTime() - start
        }
        times.sort()
        val p95 = times[(times.size * 0.95).toInt()] / 1_000_000.0
        assertTrue(
            "TelexComposer p95 was ${p95}ms — budget is <1ms",
            p95 < 1.0,
        )
    }

    @Test
    fun `transform of a 32-char buffer completes within 1ms`() {
        val c = TelexComposer()
        val nanos = LongArray(50)
        repeat(50) { iter ->
            val start = System.nanoTime()
            for (ch in "uowj") c.onKey(ch)
            for (ch in "abcdefghijklmnopqrstuvwxyz") c.onKey(ch)
            c.onKey(' ') // commit
            nanos[iter] = System.nanoTime() - start
        }
        val medianMs = nanos.sorted()[nanos.size / 2] / 1_000_000.0
        assertTrue(
            "Median full-buffer cycle was ${medianMs}ms — budget is <1ms",
            medianMs < 1.0,
        )
    }
}
