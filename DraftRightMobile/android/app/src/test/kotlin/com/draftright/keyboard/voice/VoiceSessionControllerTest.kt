package com.draftright.keyboard.voice

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Locks the voice-session state machine (VOICE-006..009): every outcome
 * funnels through one commit path, and the golden rule — any polish failure
 * commits the raw transcript, because speech is unrepeatable — is
 * non-negotiable. Exercised entirely against [FakeVoiceInput]; no
 * android.speech import anywhere in this file or the controller.
 */
class VoiceSessionControllerTest {

    private class FakeVoiceInput : VoiceInput {
        var listener: VoiceInput.Listener? = null
        var startCalls = 0
        var cancelCalls = 0

        override fun start(localeTag: String, listener: VoiceInput.Listener) {
            startCalls++
            this.listener = listener
        }
        override fun stop() {}
        override fun cancel() {
            cancelCalls++
            listener = null
        }
    }

    @Test fun polishFailureCommitsRawTranscript() {  // TC: VOICE-007
        val fake = FakeVoiceInput()
        val outcomes = mutableListOf<VoiceOutcome>()
        val c = VoiceSessionController(fake,
            polish = { _, cb -> cb(Result.failure(Exception("timeout"))) },
            onState = {}, onOutcome = { outcomes.add(it) })
        c.startSession("vi-VN", rawMode = false)
        fake.listener!!.onFinal("xin chào ừm thế giới")
        assertEquals(listOf<VoiceOutcome>(VoiceOutcome.Raw("xin chào ừm thế giới", hint = "Polish failed — inserted your words as spoken")), outcomes)
    }

    @Test fun `happy path reports full state sequence and polished outcome`() {  // TC: VOICE-006
        val fake = FakeVoiceInput()
        val states = mutableListOf<VoiceSessionController.State>()
        val outcomes = mutableListOf<VoiceOutcome>()
        val c = VoiceSessionController(fake,
            polish = { text, cb -> cb(Result.success("Polished: $text")) },
            onState = { states.add(it) }, onOutcome = { outcomes.add(it) })

        c.startSession("en-US", rawMode = false)
        fake.listener!!.onFinal("um hello world")

        assertEquals(
            listOf(
                VoiceSessionController.State.LISTENING,
                VoiceSessionController.State.PROCESSING,
                VoiceSessionController.State.IDLE
            ),
            states
        )
        assertEquals(listOf<VoiceOutcome>(VoiceOutcome.Polished("Polished: um hello world")), outcomes)
    }

    @Test fun `cancel while listening inserts nothing`() {  // TC: VOICE-008
        val fake = FakeVoiceInput()
        var polishInvocations = 0
        val states = mutableListOf<VoiceSessionController.State>()
        val outcomes = mutableListOf<VoiceOutcome>()
        val c = VoiceSessionController(fake,
            polish = { _, _ -> polishInvocations++ },
            onState = { states.add(it) }, onOutcome = { outcomes.add(it) })

        c.startSession("en-US", rawMode = false)
        c.cancelSession()

        assertEquals(listOf<VoiceOutcome>(VoiceOutcome.Nothing_), outcomes)
        assertEquals(0, polishInvocations)
        assertEquals(VoiceSessionController.State.IDLE, states.last())
        assertEquals(1, fake.cancelCalls)
    }

    @Test fun `raw mode skips polish entirely`() {  // TC: VOICE-009
        val fake = FakeVoiceInput()
        var polishInvocations = 0
        val outcomes = mutableListOf<VoiceOutcome>()
        val c = VoiceSessionController(fake,
            polish = { _, _ -> polishInvocations++ },
            onState = {}, onOutcome = { outcomes.add(it) })

        c.startSession("en-US", rawMode = true)
        fake.listener!!.onFinal("raw words as spoken")

        assertEquals(0, polishInvocations)
        assertEquals(listOf<VoiceOutcome>(VoiceOutcome.Raw("raw words as spoken", hint = null)), outcomes)
    }

    @Test fun `polish failure via generic exception also falls back to raw and is not retried`() {
        val fake = FakeVoiceInput()
        var polishInvocations = 0
        val outcomes = mutableListOf<VoiceOutcome>()
        val c = VoiceSessionController(fake,
            polish = { _, cb ->
                polishInvocations++
                cb(Result.failure(RuntimeException("boom")))
            },
            onState = {}, onOutcome = { outcomes.add(it) })

        c.startSession("en-US", rawMode = false)
        fake.listener!!.onFinal("some transcript")

        assertEquals(1, polishInvocations)
        assertEquals(
            listOf<VoiceOutcome>(VoiceOutcome.Raw("some transcript", hint = "Polish failed — inserted your words as spoken")),
            outcomes
        )
    }

    @Test fun `recognizer error yields nothing and returns to idle`() {
        val fake = FakeVoiceInput()
        var polishInvocations = 0
        val states = mutableListOf<VoiceSessionController.State>()
        val outcomes = mutableListOf<VoiceOutcome>()
        val c = VoiceSessionController(fake,
            polish = { _, _ -> polishInvocations++ },
            onState = { states.add(it) }, onOutcome = { outcomes.add(it) })

        c.startSession("en-US", rawMode = false)
        fake.listener!!.onError(VoiceError.NO_SPEECH)

        assertEquals(listOf<VoiceOutcome>(VoiceOutcome.Nothing_), outcomes)
        assertEquals(0, polishInvocations)
        assertEquals(VoiceSessionController.State.IDLE, states.last())
    }

    @Test fun `cancel during processing drops a late polish success`() {
        val fake = FakeVoiceInput()
        var capturedCallback: ((Result<String>) -> Unit)? = null
        val outcomes = mutableListOf<VoiceOutcome>()
        val c = VoiceSessionController(fake,
            polish = { _, cb -> capturedCallback = cb },
            onState = {}, onOutcome = { outcomes.add(it) })

        c.startSession("en-US", rawMode = false)
        fake.listener!!.onFinal("some transcript")
        assertTrue("polish should have been invoked and captured", capturedCallback != null)

        c.cancelSession()
        assertEquals(listOf<VoiceOutcome>(VoiceOutcome.Nothing_), outcomes)

        // Backend thread finally responds after the session was cancelled —
        // must be dropped, not appended as a second outcome.
        capturedCallback!!.invoke(Result.success("too late"))
        assertEquals(listOf<VoiceOutcome>(VoiceOutcome.Nothing_), outcomes)
    }

    @Test fun `onPartialText forwards live transcript while listening`() {
        val fake = FakeVoiceInput()
        var time = 0L
        val partials = mutableListOf<String>()
        val c = VoiceSessionController(fake,
            polish = { _, cb -> cb(Result.success("done")) },
            onState = {}, onOutcome = {}, now = { time })
        c.onPartialText { partials.add(it) }

        c.startSession("en-US", rawMode = false)
        fake.listener!!.onPartial("um")
        time += VoiceConfig.PARTIAL_DEBOUNCE_MS // outside the debounce window
        fake.listener!!.onPartial("um hello")

        assertEquals(listOf("um", "um hello"), partials)
    }

    @Test fun `cancel when idle is a no-op`() {
        val fake = FakeVoiceInput()
        val outcomes = mutableListOf<VoiceOutcome>()
        val c = VoiceSessionController(fake,
            polish = { _, cb -> cb(Result.success("done")) },
            onState = {}, onOutcome = { outcomes.add(it) })

        c.cancelSession()

        assertTrue(outcomes.isEmpty())
        assertEquals(0, fake.cancelCalls)
    }

    @Test fun `rapid partials within the debounce window are throttled after the first`() {  // TC: VOICE-010
        val fake = FakeVoiceInput()
        var time = 0L
        val partials = mutableListOf<String>()
        val c = VoiceSessionController(fake,
            polish = { _, cb -> cb(Result.success("done")) },
            onState = {}, onOutcome = {}, now = { time })
        c.onPartialText { partials.add(it) }

        c.startSession("en-US", rawMode = false)
        fake.listener!!.onPartial("a") // first partial of the session always forwarded
        time += 50
        fake.listener!!.onPartial("ab") // 50ms since last forward — dropped
        time += 99
        fake.listener!!.onPartial("abc") // 149ms since last forward — still dropped

        assertEquals(listOf("a"), partials)
    }

    @Test fun `partials spaced at least the debounce interval apart are all forwarded`() {  // TC: VOICE-010
        val fake = FakeVoiceInput()
        var time = 0L
        val partials = mutableListOf<String>()
        val c = VoiceSessionController(fake,
            polish = { _, cb -> cb(Result.success("done")) },
            onState = {}, onOutcome = {}, now = { time })
        c.onPartialText { partials.add(it) }

        c.startSession("en-US", rawMode = false)
        fake.listener!!.onPartial("a")
        time += 150
        fake.listener!!.onPartial("ab")
        time += 150
        fake.listener!!.onPartial("abc")

        assertEquals(listOf("a", "ab", "abc"), partials)
    }
}
