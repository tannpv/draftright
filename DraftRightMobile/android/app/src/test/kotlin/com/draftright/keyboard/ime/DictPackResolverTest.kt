package com.draftright.keyboard.ime

import android.content.Context
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.mock
import java.io.File
import java.nio.file.Files

/**
 * Regression for issue #10: `resolvedPackFile` is the cache key a language
 * pack uses to decide whether to rebuild its candidate engine. It must point
 * at the highest-version installed pack and change when a newer pack is
 * installed mid-session — otherwise the keyboard serves the stale dictionary.
 */
class DictPackResolverTest {

    private fun ctxWithFilesDir(dir: File): Context = mock { on { filesDir } doReturn dir }

    private fun writePack(filesDir: File, name: String, body: String) {
        val packs = File(filesDir, "packs").apply { mkdirs() }
        File(packs, name).writeText(body)
    }

    @Test
    fun `null when no packs installed`() {
        val dir = Files.createTempDirectory("dr-pack").toFile()
        assertNull(DictPackResolver.resolvedPackFile(ctxWithFilesDir(dir), "draftright-ime-zh"))
    }

    @Test
    fun `picks highest version, and changes when a newer pack lands mid-session`() {
        val dir = Files.createTempDirectory("dr-pack").toFile()
        val ctx = ctxWithFilesDir(dir)
        writePack(dir, "draftright-ime-zh-v1.pack", "pengyou\t友A")

        val first = DictPackResolver.resolvedPackFile(ctx, "draftright-ime-zh")
        assertEquals("draftright-ime-zh-v1.pack", first?.name)

        // New pack installed without restarting — resolver must move to it, so
        // the pack's cache key changes and the engine rebuilds.
        writePack(dir, "draftright-ime-zh-v2.pack", "pengyou\t友B")
        val second = DictPackResolver.resolvedPackFile(ctx, "draftright-ime-zh")
        assertEquals("draftright-ime-zh-v2.pack", second?.name)
    }
}
