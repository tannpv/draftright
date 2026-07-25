import Foundation

/// Outcome of one voice session — the single shape the keyboard's commit path
/// switches on. Speech is unrepeatable, so every failure mode still carries a
/// transcript when one exists (golden rule). Mirrors the Android `VoiceOutcome`.
public enum VoiceOutcome: Equatable {
    /// AI-polished transcript, ready to commit as-is.
    case polished(String)

    /// Verbatim transcript — either raw mode was requested, or polish failed.
    case raw(text: String, hint: String?)

    /// Nothing to commit: the session was cancelled, or the recognizer never
    /// produced a transcript.
    case nothing
}

/// Drives one dictate → (optionally) AI-polish → commit session end to end.
/// Faithful port of the Android `VoiceSessionController`.
///
/// State machine:
/// - `startSession` → `.listening`, delegates to `voice`.
/// - final transcript + `rawMode` → `.raw` with no hint, `polish` is never run.
/// - final transcript, not raw → `.processing` while `polish` runs; success →
///   `.polished`; ANY failure → `.raw` with the fallback hint (golden rule) —
///   polish is never retried automatically.
/// - `cancelSession` while listening/processing → cancels the recognizer,
///   returns to `.idle`, emits `.nothing`. A `polish` result arriving after
///   cancellation (it can run on a worker thread) is dropped.
/// - recognizer error → `.nothing` + `.idle` (no transcript ever existed).
///
/// Threading: `startSession`/`cancelSession`/`finishSession` and the recognizer
/// callbacks run on the main thread, but `polish`'s completion may land on any
/// thread. A recursive lock (reentrant, so `cancelSession()` can be called from
/// inside an `onOutcome` handler) plus a monotonic `generation` guard the
/// outcome emission against the stale-callback / cancel-races-polish TOCTOU.
public final class VoiceSessionController {
    public enum State { case idle, listening, processing }

    private static let rawFallbackHint = "Polish failed — inserted your words as spoken"

    private let voice: VoiceInput
    private let polish: (String, @escaping (Result<String, Error>) -> Void) -> Void
    private let onState: (State) -> Void
    private let onOutcome: (VoiceOutcome) -> Void
    private let now: () -> Int

    private let lock = NSRecursiveLock()
    private var state: State = .idle
    private var generation = 0
    private var partialCallback: ((String) -> Void)?
    private var lastPartialForwardedAt = 0
    private var hasForwardedPartial = false

    public init(
        voice: VoiceInput,
        polish: @escaping (String, @escaping (Result<String, Error>) -> Void) -> Void,
        onState: @escaping (State) -> Void,
        onOutcome: @escaping (VoiceOutcome) -> Void,
        now: @escaping () -> Int = { Int(Date().timeIntervalSince1970 * 1000) }
    ) {
        self.voice = voice
        self.polish = polish
        self.onState = onState
        self.onOutcome = onOutcome
        self.now = now
    }

    /// Registers the live-transcript callback for the preview.
    public func onPartialText(_ cb: @escaping (String) -> Void) {
        lock.lock(); defer { lock.unlock() }
        partialCallback = cb
    }

    public func startSession(localeTag: String, rawMode: Bool) {
        let sessionGeneration: Int = {
            lock.lock(); defer { lock.unlock() }
            generation += 1
            hasForwardedPartial = false
            return generation
        }()
        setState(.listening)

        voice.start(localeTag: localeTag, listener: ClosureVoiceListener(
            partial: { [weak self] text in self?.handlePartial(text, sessionGeneration) },
            final: { [weak self] text in self?.handleFinalEntry(text, rawMode, sessionGeneration) },
            error: { [weak self] _ in self?.handleError(sessionGeneration) }
        ))
    }

    public func cancelSession() {
        lock.lock(); defer { lock.unlock() }
        if state == .idle { return }
        generation += 1 // invalidate any in-flight listener/polish callbacks
        voice.cancel()
        setState(.idle)
        onOutcome(.nothing)
    }

    /// Release on the mic (hold-to-talk): stop the recognizer and let it deliver
    /// the final transcript through the existing `onFinal` path. No-op unless
    /// listening — a pause longer than the recognizer's silence window may
    /// already have ended the session.
    public func finishSession() {
        lock.lock(); defer { lock.unlock() }
        if state != .listening { return }
        voice.stop()
    }

    // MARK: - Recognizer callbacks

    private func handlePartial(_ text: String, _ sessionGeneration: Int) {
        var forward = false
        var cb: ((String) -> Void)?
        lock.lock()
        if sessionGeneration == generation {
            let elapsed = now() - lastPartialForwardedAt
            if !hasForwardedPartial || elapsed >= VoiceConfig.partialDebounceMs {
                hasForwardedPartial = true
                lastPartialForwardedAt = now()
                forward = true
            }
            cb = partialCallback
        }
        lock.unlock()
        if forward { cb?(text) }
    }

    private func handleFinalEntry(_ text: String, _ rawMode: Bool, _ sessionGeneration: Int) {
        guard isCurrent(sessionGeneration) else { return }
        handleFinal(text, rawMode, sessionGeneration)
    }

    private func handleError(_ sessionGeneration: Int) {
        lock.lock(); defer { lock.unlock() }
        guard sessionGeneration == generation else { return }
        // No transcript ever existed — nothing to preserve.
        setState(.idle)
        onOutcome(.nothing)
    }

    private func handleFinal(_ text: String, _ rawMode: Bool, _ sessionGeneration: Int) {
        if rawMode {
            lock.lock(); defer { lock.unlock() }
            guard sessionGeneration == generation else { return }
            setState(.idle)
            onOutcome(.raw(text: text, hint: nil))
            return
        }

        setState(.processing)
        polish(text) { [weak self] result in
            guard let self else { return }
            self.lock.lock(); defer { self.lock.unlock() }
            guard sessionGeneration == self.generation else { return } // cancelled — drop the late result
            self.setState(.idle)
            switch result {
            case .success(let polished): self.onOutcome(.polished(polished))
            case .failure: self.onOutcome(.raw(text: text, hint: Self.rawFallbackHint))
            }
        }
    }

    private func isCurrent(_ sessionGeneration: Int) -> Bool {
        lock.lock(); defer { lock.unlock() }
        return sessionGeneration == generation
    }

    private func setState(_ newState: State) {
        state = newState
        onState(newState)
    }
}
