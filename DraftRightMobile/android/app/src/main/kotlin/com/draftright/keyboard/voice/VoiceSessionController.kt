package com.draftright.keyboard.voice

/**
 * Outcome of one voice session — the single shape [DraftRightIME]'s commit
 * path switches on. Speech is unrepeatable, so every failure mode still
 * carries a transcript when one exists (golden rule, VOICE-007).
 */
sealed class VoiceOutcome {
    /** AI-polished transcript, ready to commit as-is. */
    data class Polished(val text: String) : VoiceOutcome()

    /** Verbatim transcript — either raw mode was requested, or polish failed. */
    data class Raw(val text: String, val hint: String?) : VoiceOutcome()

    /** Nothing to commit: the session was cancelled, or the recognizer never produced a transcript. */
    object Nothing_ : VoiceOutcome()
}

/**
 * Drives one dictate → (optionally) AI-polish → commit session end to end.
 *
 * State machine (VOICE-006..009):
 * - `startSession` → LISTENING, delegates to [voice].
 * - final transcript + `rawMode` → [VoiceOutcome.Raw] with no hint, `polish` is
 *   never invoked (VOICE-009).
 * - final transcript, not raw → PROCESSING while `polish` runs; success →
 *   [VoiceOutcome.Polished]; ANY failure → [VoiceOutcome.Raw] with
 *   [RAW_FALLBACK_HINT] (VOICE-007) — polish is never retried automatically.
 * - `cancelSession` while LISTENING or PROCESSING → cancels the underlying
 *   recognizer, returns to IDLE, emits [VoiceOutcome.Nothing_] (VOICE-008).
 *   A `polish` result that arrives after cancellation (it can run on a
 *   worker thread) is dropped — no outcome is emitted for it.
 * - recognizer `onError` → [VoiceOutcome.Nothing_] + IDLE (no transcript ever
 *   existed, so there is nothing to fall back to).
 */
class VoiceSessionController(
    private val voice: VoiceInput,
    private val polish: (text: String, onResult: (Result<String>) -> Unit) -> Unit,
    private val onState: (State) -> Unit,
    private val onOutcome: (VoiceOutcome) -> Unit,
) {
    enum class State { IDLE, LISTENING, PROCESSING }

    private companion object {
        const val RAW_FALLBACK_HINT = "Polish failed — inserted your words as spoken"
    }

    private var state: State = State.IDLE
    private var partialCallback: ((String) -> Unit)? = null

    // Bumped on every startSession/cancelSession so callbacks belonging to a
    // superseded session (a late polish result, a stray recognizer event)
    // can recognize themselves as stale and no-op instead of emitting a
    // second outcome for a session that already finished.
    private var generation = 0

    /** Registers the live-transcript callback for the candidate bar. */
    fun onPartialText(cb: (String) -> Unit) {
        partialCallback = cb
    }

    fun startSession(localeTag: String, rawMode: Boolean) {
        val sessionGeneration = ++generation
        setState(State.LISTENING)

        voice.start(localeTag, object : VoiceInput.Listener {
            override fun onPartial(text: String) {
                if (!isCurrent(sessionGeneration)) return
                partialCallback?.invoke(text)
            }

            override fun onFinal(text: String) {
                if (!isCurrent(sessionGeneration)) return
                handleFinal(text, rawMode, sessionGeneration)
            }

            override fun onError(error: VoiceError) {
                if (!isCurrent(sessionGeneration)) return
                // No transcript ever existed — nothing to preserve.
                setState(State.IDLE)
                onOutcome(VoiceOutcome.Nothing_)
            }
        })
    }

    fun cancelSession() {
        if (state == State.IDLE) return
        generation++ // invalidate any in-flight listener/polish callbacks
        voice.cancel()
        setState(State.IDLE)
        onOutcome(VoiceOutcome.Nothing_)
    }

    private fun handleFinal(text: String, rawMode: Boolean, sessionGeneration: Int) {
        if (rawMode) {
            setState(State.IDLE)
            onOutcome(VoiceOutcome.Raw(text, hint = null))
            return
        }

        setState(State.PROCESSING)
        polish(text) { result ->
            if (!isCurrent(sessionGeneration)) return@polish // cancelled — drop the late result
            setState(State.IDLE)
            result.fold(
                onSuccess = { polished -> onOutcome(VoiceOutcome.Polished(polished)) },
                onFailure = { onOutcome(VoiceOutcome.Raw(text, hint = RAW_FALLBACK_HINT)) }
            )
        }
    }

    private fun isCurrent(sessionGeneration: Int) = sessionGeneration == generation

    private fun setState(newState: State) {
        state = newState
        onState(newState)
    }
}
