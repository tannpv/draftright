import Foundation
import AVFoundation
import Speech
import DraftRightKeyboardCore

/// `SFSpeechRecognizer` + `AVAudioEngine` implementation of `VoiceInput` for the
/// keyboard extension: continuous recognition with partial results, for
/// hold-to-talk dictation. Mirrors Android's `SpeechRecognizerVoiceInput`.
///
/// Requires keyboard **Full Access** (`RequestsOpenAccess`) plus microphone +
/// speech-recognition authorization (see `requestAuthorization`). All listener
/// callbacks are delivered on the main thread — every bit of session state is
/// mutated on main too, so no locking is needed (start/stop/cancel are called
/// from the IME's main thread and the recognizer callback hops to main).
final class SpeechVoiceInput: VoiceInput {
    private let audioEngine = AVAudioEngine()
    private var request: SFSpeechAudioBufferRecognitionRequest?
    private var task: SFSpeechRecognitionTask?
    private weak var listener: VoiceInputListener?

    /// Bumped on every start/cancel so a late final or a timeout firing after
    /// the session ended recognizes itself as stale. Mirrors Android's sessionId.
    private var sessionId = 0
    private var deliveredFinal = false

    /// True when the device has an available speech recognizer at all.
    static var isAvailable: Bool { SFSpeechRecognizer()?.isAvailable ?? false }

    /// Both permissions the mic needs, already granted.
    static var isAuthorized: Bool {
        guard SFSpeechRecognizer.authorizationStatus() == .authorized else { return false }
        if #available(iOS 17.0, *) {
            return AVAudioApplication.shared.recordPermission == .granted
        } else {
            return AVAudioSession.sharedInstance().recordPermission == .granted
        }
    }

    /// Request speech + microphone authorization. `granted` is true only if both
    /// are allowed. Completion is delivered on the main thread.
    static func requestAuthorization(_ completion: @escaping (Bool) -> Void) {
        SFSpeechRecognizer.requestAuthorization { speechStatus in
            guard speechStatus == .authorized else {
                DispatchQueue.main.async { completion(false) }
                return
            }
            let micHandler: (Bool) -> Void = { granted in
                DispatchQueue.main.async { completion(granted) }
            }
            if #available(iOS 17.0, *) {
                AVAudioApplication.requestRecordPermission(completionHandler: micHandler)
            } else {
                AVAudioSession.sharedInstance().requestRecordPermission(micHandler)
            }
        }
    }

    func start(localeTag: String, listener: VoiceInputListener) {
        self.listener = listener
        sessionId += 1
        let session = sessionId
        deliveredFinal = false

        let recognizer = SFSpeechRecognizer(locale: Locale(identifier: localeTag)) ?? SFSpeechRecognizer()
        guard let recognizer, recognizer.isAvailable else {
            listener.onError(.recognizerUnavailable)
            return
        }

        let req = SFSpeechAudioBufferRecognitionRequest()
        req.shouldReportPartialResults = true
        request = req

        do {
            let audioSession = AVAudioSession.sharedInstance()
            try audioSession.setCategory(.record, mode: .measurement, options: .duckOthers)
            try audioSession.setActive(true, options: .notifyOthersOnDeactivation)
        } catch {
            request = nil
            listener.onError(.other)
            return
        }

        let inputNode = audioEngine.inputNode
        inputNode.removeTap(onBus: 0)
        inputNode.installTap(onBus: 0, bufferSize: 1024, format: inputNode.outputFormat(forBus: 0)) { [weak self] buffer, _ in
            self?.request?.append(buffer) // audio thread; append is safe
        }
        audioEngine.prepare()
        do {
            try audioEngine.start()
        } catch {
            teardownAudio()
            listener.onError(.other)
            return
        }

        task = recognizer.recognitionTask(with: req) { [weak self] result, error in
            // Hop to main so all session-state reads/writes stay single-threaded.
            DispatchQueue.main.async {
                guard let self, session == self.sessionId, !self.deliveredFinal else { return }
                if let result {
                    let text = result.bestTranscription.formattedString
                    if result.isFinal {
                        self.deliveredFinal = true
                        self.teardownAudio()
                        if text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                            self.listener?.onError(.noSpeech)
                        } else {
                            self.listener?.onFinal(text)
                        }
                    } else {
                        self.listener?.onPartial(text)
                    }
                } else if let error {
                    self.deliveredFinal = true
                    self.teardownAudio()
                    self.listener?.onError(Self.mapError(error))
                }
            }
        }

        // Hard stop if the recognizer never produces a result.
        DispatchQueue.main.asyncAfter(deadline: .now() + .milliseconds(VoiceConfig.listenTimeoutMs)) { [weak self] in
            guard let self, session == self.sessionId, !self.deliveredFinal else { return }
            self.stop() // force the recognizer to finalize what it has
        }
    }

    func stop() {
        // Release-to-finalize: stop capturing + end the audio stream so the
        // recognizer emits its final. Keep the task so onFinal still arrives.
        if audioEngine.isRunning {
            audioEngine.stop()
            audioEngine.inputNode.removeTap(onBus: 0)
        }
        request?.endAudio()
        deactivateSession()
    }

    func cancel() {
        sessionId += 1 // invalidate any late callbacks
        task?.cancel()
        teardownAudio()
    }

    // MARK: - Helpers

    private func teardownAudio() {
        if audioEngine.isRunning {
            audioEngine.stop()
            audioEngine.inputNode.removeTap(onBus: 0)
        }
        request?.endAudio()
        request = nil
        task = nil
        deactivateSession()
    }

    private func deactivateSession() {
        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
    }

    private static func mapError(_ error: Error) -> VoiceError {
        let ns = error as NSError
        if ns.domain == NSURLErrorDomain { return .network }
        // kAFAssistantErrorDomain: 203 = no speech, 1110/1700 = no match.
        switch ns.code {
        case 203, 1110, 1700: return .noSpeech
        default: return .other
        }
    }
}
