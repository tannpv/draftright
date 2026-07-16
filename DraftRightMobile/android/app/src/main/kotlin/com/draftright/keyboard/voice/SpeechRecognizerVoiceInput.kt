package com.draftright.keyboard.voice

import android.content.Context
import android.content.Intent
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.speech.RecognitionListener
import android.speech.RecognizerIntent
import android.speech.SpeechRecognizer

/**
 * Thin adapter over [SpeechRecognizer] — maps platform callbacks onto
 * [VoiceInput.Listener] and nothing more. All session/outcome logic lives in
 * [VoiceSessionController], which is unit-testable; this class touches
 * android.speech directly so it is device-only and intentionally not
 * unit-tested (mirrors the plan's "not unit-tested, thin adapter" note).
 *
 * [VoiceConfig.LISTEN_TIMEOUT_MS] is a hard stop for the whole session — it
 * is NOT the platform's silence-detection extra (that is a different concept
 * with a different, much shorter, default). It is enforced here with a
 * postDelayed callback on the main looper.
 *
 * Lifecycle guard: each [start] mints a fresh [SpeechRecognizer] and a fresh
 * [sessionId]. Every callback/timeout runnable captures that id and only acts
 * if it still equals [sessionId] at the time it fires — i.e. no newer session
 * has started since. This is stronger than the single shared boolean flag it
 * replaces: a boolean only stops a session's own callbacks from firing twice,
 * it can't distinguish "stale event from session N-1" from "event for the
 * current session N" once a new session has started in between (e.g. a
 * cancelled recognizer delivering a late onResults just as the user re-taps
 * the mic). All of start()/RecognitionListener callbacks/the timeout runnable
 * execute on the main looper (platform contract for SpeechRecognizer + our
 * Handler(mainLooper)), so [sessionId] requires no synchronization.
 */
class SpeechRecognizerVoiceInput(private val context: Context) : VoiceInput {

    private var recognizer: SpeechRecognizer? = null
    private val timeoutHandler = Handler(Looper.getMainLooper())
    private var timeoutRunnable: Runnable? = null

    /** Monotonically increasing per-session token — see class doc. */
    private var sessionId = 0

    override fun start(localeTag: String, listener: VoiceInput.Listener) {
        if (!SpeechRecognizer.isRecognitionAvailable(context)) {
            listener.onError(VoiceError.RECOGNIZER_UNAVAILABLE)
            return
        }

        val id = ++sessionId
        // Safety net: drop any leftover recognizer/timeout a caller may have
        // left behind by starting a new session without cancelling the old
        // one first (VoiceSessionController always cancels first, but this
        // keeps the adapter correct on its own).
        terminalCleanup()

        val recognizer = SpeechRecognizer.createSpeechRecognizer(context)
        this.recognizer = recognizer
        recognizer.setRecognitionListener(object : RecognitionListener {
            override fun onReadyForSpeech(params: Bundle?) = Unit
            override fun onBeginningOfSpeech() = Unit
            override fun onRmsChanged(rmsdB: Float) = Unit
            override fun onBufferReceived(buffer: ByteArray?) = Unit
            override fun onEndOfSpeech() = Unit
            override fun onEvent(eventType: Int, params: Bundle?) = Unit

            override fun onError(error: Int) {
                if (id != sessionId) return
                terminalCleanup()
                listener.onError(mapError(error))
            }

            override fun onPartialResults(partialResults: Bundle?) {
                if (id != sessionId) return
                firstResult(partialResults)?.let { listener.onPartial(it) }
            }

            override fun onResults(results: Bundle?) {
                if (id != sessionId) return
                terminalCleanup()
                val text = firstResult(results)
                if (text != null) listener.onFinal(text) else listener.onError(VoiceError.NO_SPEECH)
            }
        })

        val intent = Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).apply {
            putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
            putExtra(RecognizerIntent.EXTRA_LANGUAGE, localeTag)
            putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS, true)
            // Hold-to-talk: keep listening through pauses until the user
            // releases (which calls stop()). These are hints; some OEM
            // recognizers ignore them — the 30 s LISTEN_TIMEOUT_MS backstop
            // and the release-to-stop path still bound the session.
            putExtra(RecognizerIntent.EXTRA_SPEECH_INPUT_COMPLETE_SILENCE_LENGTH_MILLIS, VoiceConfig.HOLD_SILENCE_MS)
            putExtra(RecognizerIntent.EXTRA_SPEECH_INPUT_POSSIBLY_COMPLETE_SILENCE_LENGTH_MILLIS, VoiceConfig.HOLD_SILENCE_MS)
            putExtra(RecognizerIntent.EXTRA_SPEECH_INPUT_MINIMUM_LENGTH_MILLIS, 1_000L)
        }
        recognizer.startListening(intent)

        val runnable = Runnable {
            if (id != sessionId) return@Runnable
            recognizer.cancel()
            terminalCleanup()
            listener.onError(VoiceError.NO_SPEECH)
        }
        timeoutRunnable = runnable
        timeoutHandler.postDelayed(runnable, VoiceConfig.LISTEN_TIMEOUT_MS)
    }

    override fun stop() {
        // Destroys the recognizer immediately, so the async final onResults
        // that stopListening() would normally deliver is dropped. No caller
        // today ("stop and keep what you heard" isn't in the UX); a future
        // caller wanting that behavior must defer terminalCleanup() until
        // onResults/onError fires.
        recognizer?.stopListening()
        terminalCleanup()
    }

    override fun cancel() {
        sessionId++ // invalidate any in-flight callback/timeout for this session
        recognizer?.cancel()
        terminalCleanup()
    }

    /**
     * Cancels the pending timeout and destroys+drops the recognizer. Shared
     * terminal path for every way a session can end (result, error, timeout,
     * cancel, stop) so a finished session never leaks a live
     * [SpeechRecognizer] — [start] always mints a new one for the next
     * session rather than reusing this one.
     */
    private fun terminalCleanup() {
        cancelTimeout()
        recognizer?.destroy()
        recognizer = null
    }

    private fun cancelTimeout() {
        timeoutRunnable?.let { timeoutHandler.removeCallbacks(it) }
        timeoutRunnable = null
    }

    private fun firstResult(bundle: Bundle?): String? =
        bundle?.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION)?.firstOrNull()

    private fun mapError(error: Int): VoiceError = when (error) {
        SpeechRecognizer.ERROR_INSUFFICIENT_PERMISSIONS -> VoiceError.PERMISSION_DENIED
        SpeechRecognizer.ERROR_NO_MATCH,
        SpeechRecognizer.ERROR_SPEECH_TIMEOUT -> VoiceError.NO_SPEECH
        SpeechRecognizer.ERROR_NETWORK,
        SpeechRecognizer.ERROR_NETWORK_TIMEOUT,
        SpeechRecognizer.ERROR_SERVER -> VoiceError.NETWORK
        SpeechRecognizer.ERROR_RECOGNIZER_BUSY,
        SpeechRecognizer.ERROR_CLIENT -> VoiceError.OTHER
        else -> VoiceError.OTHER
    }
}
