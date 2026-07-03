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
 * postDelayed callback on the main looper, guarded by [finished] so it can
 * only ever fire once and is always cancelled on any terminal outcome.
 */
class SpeechRecognizerVoiceInput(private val context: Context) : VoiceInput {

    private var recognizer: SpeechRecognizer? = null
    private val timeoutHandler = Handler(Looper.getMainLooper())
    private var timeoutRunnable: Runnable? = null
    private var finished = false

    override fun start(localeTag: String, listener: VoiceInput.Listener) {
        if (!SpeechRecognizer.isRecognitionAvailable(context)) {
            listener.onError(VoiceError.RECOGNIZER_UNAVAILABLE)
            return
        }

        finished = false

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
                if (!markFinished()) return
                cancelTimeout()
                listener.onError(mapError(error))
            }

            override fun onPartialResults(partialResults: Bundle?) {
                firstResult(partialResults)?.let { listener.onPartial(it) }
            }

            override fun onResults(results: Bundle?) {
                if (!markFinished()) return
                cancelTimeout()
                val text = firstResult(results)
                if (text != null) listener.onFinal(text) else listener.onError(VoiceError.NO_SPEECH)
            }
        })

        val intent = Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).apply {
            putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
            putExtra(RecognizerIntent.EXTRA_LANGUAGE, localeTag)
            putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS, true)
        }
        recognizer.startListening(intent)

        val runnable = Runnable {
            if (!markFinished()) return@Runnable
            recognizer.cancel()
            listener.onError(VoiceError.NO_SPEECH)
        }
        timeoutRunnable = runnable
        timeoutHandler.postDelayed(runnable, VoiceConfig.LISTEN_TIMEOUT_MS)
    }

    override fun stop() {
        cancelTimeout()
        recognizer?.stopListening()
    }

    override fun cancel() {
        finished = true
        cancelTimeout()
        recognizer?.cancel()
        recognizer?.destroy()
        recognizer = null
    }

    /** Returns true (and flips [finished]) only the first time it's called for this session. */
    private fun markFinished(): Boolean {
        if (finished) return false
        finished = true
        return true
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
