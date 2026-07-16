package com.draftright.keyboard.voice

import java.util.concurrent.atomic.AtomicInteger

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
    private val now: () -> Long = System::currentTimeMillis,
) {
    enum class State { IDLE, LISTENING, PROCESSING }

    private companion object {
        const val RAW_FALLBACK_HINT = "Polish failed — inserted your words as spoken"
    }

    // Threading contract: startSession/cancelSession are only ever called on
    // the IME's main thread. Recognizer callbacks (onPartial/onFinal/onError)
    // also arrive on the main thread, but `polish` may run its completion on
    // any worker thread. `state` is @Volatile so a worker thread's read (e.g.
    // via isCurrent's happens-before edge below) sees the latest value, and
    // `generation` is an AtomicInteger so a polish completion racing a
    // main-thread cancelSession() always compares against a consistent,
    // visible value — the generation captured before polish started is
    // compared with a fresh get() at completion time, guarding outcome
    // emission against the stale-callback race.
    @Volatile
    private var state: State = State.IDLE
    private var partialCallback: ((String) -> Unit)? = null

    // Bumped on every startSession/cancelSession so callbacks belonging to a
    // superseded session (a late polish result, a stray recognizer event)
    // can recognize themselves as stale and no-op instead of emitting a
    // second outcome for a session that already finished.
    private val generation = AtomicInteger(0)

    private var lastPartialForwardedAt: Long = 0L
    private var hasForwardedPartial: Boolean = false

    /** Registers the live-transcript callback for the candidate bar. */
    fun onPartialText(cb: (String) -> Unit) {
        partialCallback = cb
    }

    fun startSession(localeTag: String, rawMode: Boolean) {
        val sessionGeneration = generation.incrementAndGet()
        hasForwardedPartial = false
        setState(State.LISTENING)

        voice.start(localeTag, object : VoiceInput.Listener {
            override fun onPartial(text: String) {
                if (!isCurrent(sessionGeneration)) return
                val elapsed = now() - lastPartialForwardedAt
                if (!hasForwardedPartial || elapsed >= VoiceConfig.PARTIAL_DEBOUNCE_MS) {
                    hasForwardedPartial = true
                    lastPartialForwardedAt = now()
                    partialCallback?.invoke(text)
                }
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
        // Serialized against the polish-completion handling below: without
        // this, a cancel racing a polish result landing on a worker thread is
        // a check-then-act TOCTOU — both sides can observe "I'm still current"
        // and both emit an outcome. synchronized(this) makes the
        // generation-bump + state mutation + outcome emission one atomic
        // unit. Reentrant on the JVM, so it's safe for this to be called from
        // inside onOutcome's completion handler (same thread) without
        // deadlocking.
        synchronized(this) {
            if (state == State.IDLE) return
            generation.incrementAndGet() // invalidate any in-flight listener/polish callbacks
            voice.cancel()
            setState(State.IDLE)
            onOutcome(VoiceOutcome.Nothing_)
        }
    }

    /**
     * Release on the mic (hold-to-talk): stop the recognizer and let it
     * deliver the final transcript through the existing onFinal path (→
     * VoiceOutcome.Raw for a rawMode session). No-op unless LISTENING — a
     * pause longer than the recognizer's silence window may already have
     * ended the session, in which case there is nothing to finalize.
     * Synchronized on the same lock as cancelSession()/handleFinal so a
     * release racing an auto-delivered final can't double-emit.
     */
    fun finishSession() {
        synchronized(this) {
            if (state != State.LISTENING) return
            voice.stop()
        }
    }

    private fun handleFinal(text: String, rawMode: Boolean, sessionGeneration: Int) {
        if (rawMode) {
            // Same TOCTOU concern as cancelSession(): the isCurrent check and
            // the resulting state mutation + emit must be one atomic block.
            synchronized(this) {
                if (!isCurrent(sessionGeneration)) return
                setState(State.IDLE)
                onOutcome(VoiceOutcome.Raw(text, hint = null))
            }
            return
        }

        setState(State.PROCESSING)
        polish(text) { result ->
            // `polish`'s completion can run on any worker thread and race a
            // main-thread cancelSession() between the isCurrent check and the
            // outcome emission — synchronize on the same lock cancelSession()
            // uses so the two calls interleave atomically instead of racing.
            synchronized(this) {
                if (!isCurrent(sessionGeneration)) return@synchronized // cancelled — drop the late result
                setState(State.IDLE)
                result.fold(
                    onSuccess = { polished -> onOutcome(VoiceOutcome.Polished(polished)) },
                    onFailure = { onOutcome(VoiceOutcome.Raw(text, hint = RAW_FALLBACK_HINT)) }
                )
            }
        }
    }

    private fun isCurrent(sessionGeneration: Int) = sessionGeneration == generation.get()

    private fun setState(newState: State) {
        state = newState
        onState(newState)
    }
}
