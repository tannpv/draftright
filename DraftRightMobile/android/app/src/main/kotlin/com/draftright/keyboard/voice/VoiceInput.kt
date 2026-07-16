package com.draftright.keyboard.voice

/**
 * Port for on-device speech-to-text. [SpeechRecognizerVoiceInput] is the real
 * Android implementation; [VoiceSessionController]'s tests drive a fake
 * against this interface so the state machine is fully exercised without any
 * android.speech dependency.
 */
interface VoiceInput {
    /** Begin listening for speech in [localeTag] (BCP-47, e.g. "en-US"). */
    fun start(localeTag: String, listener: Listener)

    /** Stop listening and let the recognizer deliver its final result. */
    fun stop()

    /** Abandon the session immediately — no result should follow. */
    fun cancel()

    interface Listener {
        /** Live, not-yet-final transcript — drives the candidate-bar preview. */
        fun onPartial(text: String)

        /** The recognizer's final transcript for this session. */
        fun onFinal(text: String)

        /** The recognizer failed before producing any transcript. */
        fun onError(error: VoiceError)
    }
}

/** Recognizer failure reasons, mapped from platform error codes in [SpeechRecognizerVoiceInput]. */
enum class VoiceError { PERMISSION_DENIED, RECOGNIZER_UNAVAILABLE, NO_SPEECH, NETWORK, OTHER }

/** Tuning constants shared by the controller and the real recognizer adapter. */
object VoiceConfig {
    /** Hard stop for a listening session that never produces a result. */
    const val LISTEN_TIMEOUT_MS = 30_000L
    /** Minimum gap between partial-result UI updates, to avoid candidate-bar churn. */
    const val PARTIAL_DEBOUNCE_MS = 150L
    /** Silence window while holding the mic — long enough that a pause
     *  mid-sentence doesn't auto-end the session before the user releases. */
    const val HOLD_SILENCE_MS = 10_000L
    /** Minimum recording length the platform recognizer requires before it
     *  will treat the session as having spoken input. */
    const val HOLD_MIN_SPEECH_MS = 1_000L
}
