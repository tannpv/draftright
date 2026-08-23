import Foundation

/// Port for on-device speech-to-text. `SpeechVoiceInput` (in the keyboard
/// extension) is the real `SFSpeechRecognizer` implementation;
/// `VoiceSessionControllerTests` drive a fake against this protocol so the
/// state machine is fully exercised without any Speech-framework dependency.
/// Mirrors the Android `VoiceInput` interface.
public protocol VoiceInput: AnyObject {
    /// Begin listening for speech in `localeTag` (BCP-47, e.g. "en-US").
    func start(localeTag: String, listener: VoiceInputListener)

    /// Stop listening and let the recognizer deliver its final result.
    func stop()

    /// Abandon the session immediately — no result should follow.
    func cancel()
}

/// Callbacks from a `VoiceInput`. All fire on the main thread.
public protocol VoiceInputListener: AnyObject {
    /// Live, not-yet-final transcript — drives the partial-transcript preview.
    func onPartial(_ text: String)

    /// The recognizer's final transcript for this session.
    func onFinal(_ text: String)

    /// The recognizer failed before producing any transcript.
    func onError(_ error: VoiceError)
}

/// Recognizer failure reasons, mapped from platform errors in `SpeechVoiceInput`.
public enum VoiceError {
    case permissionDenied
    case recognizerUnavailable
    case noSpeech
    case network
    case other
}

/// Tuning + policy constants shared by the controller and the real recognizer
/// adapter. Mirrors the Android `VoiceConfig`.
public enum VoiceConfig {
    // Raw-vs-polish is now a user setting (SharedSettings.voicePolishEnabled,
    // #197), fed to the session as rawMode = !voicePolishEnabled. The old
    // holdToTalkRawMode constant was removed with it.

    /// Hard stop for a listening session that never produces a result.
    public static let listenTimeoutMs = 30_000

    /// Minimum gap between partial-result UI updates, to avoid preview churn.
    public static let partialDebounceMs = 150

    /// Silence window while holding the mic — long enough that a pause
    /// mid-sentence doesn't auto-end the session before the user releases.
    public static let holdSilenceMs = 10_000

    /// Minimum recording length the recognizer needs before it treats the
    /// session as having spoken input.
    public static let holdMinSpeechMs = 1_000
}

/// Adapts closures to `VoiceInputListener` so `VoiceSessionController` can vend
/// a per-session listener without a bespoke subclass at each call site.
final class ClosureVoiceListener: VoiceInputListener {
    private let partial: (String) -> Void
    private let final: (String) -> Void
    private let error: (VoiceError) -> Void

    init(partial: @escaping (String) -> Void,
         final: @escaping (String) -> Void,
         error: @escaping (VoiceError) -> Void) {
        self.partial = partial
        self.final = final
        self.error = error
    }

    func onPartial(_ text: String) { partial(text) }
    func onFinal(_ text: String) { final(text) }
    func onError(_ error: VoiceError) { self.error(error) }
}
