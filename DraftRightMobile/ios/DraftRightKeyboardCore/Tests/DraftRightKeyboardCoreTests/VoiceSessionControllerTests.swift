import XCTest
@testable import DraftRightKeyboardCore

/// Locks the voice-session state machine: every outcome funnels through one
/// commit path, and the golden rule — any polish failure commits the raw
/// transcript, because speech is unrepeatable — is non-negotiable. Exercised
/// entirely against `FakeVoiceInput`; no Speech-framework import anywhere.
/// Port of the Android `VoiceSessionControllerTest`.
final class VoiceSessionControllerTests: XCTestCase {

    private struct PolishError: Error {}

    private final class FakeVoiceInput: VoiceInput {
        var listener: VoiceInputListener?
        var startCalls = 0
        var cancelCalls = 0
        var stopCalls = 0

        func start(localeTag: String, listener: VoiceInputListener) {
            startCalls += 1
            self.listener = listener
        }
        func stop() { stopCalls += 1 } // keeps listener; real recognizer then fires onFinal
        func cancel() {
            cancelCalls += 1
            listener = nil
        }
    }

    func test_polishFailureCommitsRawTranscript() { // VOICE-007
        let fake = FakeVoiceInput()
        var outcomes: [VoiceOutcome] = []
        let c = VoiceSessionController(
            voice: fake,
            polish: { _, cb in cb(.failure(PolishError())) },
            onState: { _ in }, onOutcome: { outcomes.append($0) })
        c.startSession(localeTag: "vi-VN", rawMode: false)
        fake.listener!.onFinal("xin chào ừm thế giới")
        XCTAssertEqual(outcomes, [.raw(text: "xin chào ừm thế giới", hint: "Polish failed — inserted your words as spoken")])
    }

    func test_error_after_partials_commits_last_partial_as_raw() { // #65-1
        let fake = FakeVoiceInput()
        var outcomes: [VoiceOutcome] = []
        let c = VoiceSessionController(
            voice: fake,
            polish: { _, _ in },
            onState: { _ in }, onOutcome: { outcomes.append($0) })
        c.startSession(localeTag: "vi-VN", rawMode: false)
        fake.listener!.onPartial("xin")
        fake.listener!.onPartial("xin chào thế")
        fake.listener!.onError(.other) // e.g. SPEECH_TIMEOUT mid-sentence
        // Words were spoken — preserve them as raw, don't drop them.
        XCTAssertEqual(outcomes, [.raw(text: "xin chào thế",
                                       hint: "Recognition interrupted — inserted what was heard")])
    }

    func test_happy_path_full_state_sequence_and_polished_outcome() { // VOICE-006
        let fake = FakeVoiceInput()
        var states: [VoiceSessionController.State] = []
        var outcomes: [VoiceOutcome] = []
        let c = VoiceSessionController(
            voice: fake,
            polish: { text, cb in cb(.success("Polished: \(text)")) },
            onState: { states.append($0) }, onOutcome: { outcomes.append($0) })

        c.startSession(localeTag: "en-US", rawMode: false)
        fake.listener!.onFinal("um hello world")

        XCTAssertEqual(states, [.listening, .processing, .idle])
        XCTAssertEqual(outcomes, [.polished("Polished: um hello world")])
    }

    func test_cancel_while_listening_inserts_nothing() { // VOICE-008
        let fake = FakeVoiceInput()
        var polishInvocations = 0
        var states: [VoiceSessionController.State] = []
        var outcomes: [VoiceOutcome] = []
        let c = VoiceSessionController(
            voice: fake,
            polish: { _, _ in polishInvocations += 1 },
            onState: { states.append($0) }, onOutcome: { outcomes.append($0) })

        c.startSession(localeTag: "en-US", rawMode: false)
        c.cancelSession()

        XCTAssertEqual(outcomes, [.nothing])
        XCTAssertEqual(polishInvocations, 0)
        XCTAssertEqual(states.last, .idle)
        XCTAssertEqual(fake.cancelCalls, 1)
    }

    func test_raw_mode_skips_polish_entirely() { // VOICE-009
        let fake = FakeVoiceInput()
        var polishInvocations = 0
        var outcomes: [VoiceOutcome] = []
        let c = VoiceSessionController(
            voice: fake,
            polish: { _, _ in polishInvocations += 1 },
            onState: { _ in }, onOutcome: { outcomes.append($0) })

        c.startSession(localeTag: "en-US", rawMode: true)
        fake.listener!.onFinal("raw words as spoken")

        XCTAssertEqual(polishInvocations, 0)
        XCTAssertEqual(outcomes, [.raw(text: "raw words as spoken", hint: nil)])
    }

    func test_polish_failure_generic_error_falls_back_to_raw_and_not_retried() {
        let fake = FakeVoiceInput()
        var polishInvocations = 0
        var outcomes: [VoiceOutcome] = []
        let c = VoiceSessionController(
            voice: fake,
            polish: { _, cb in polishInvocations += 1; cb(.failure(PolishError())) },
            onState: { _ in }, onOutcome: { outcomes.append($0) })

        c.startSession(localeTag: "en-US", rawMode: false)
        fake.listener!.onFinal("some transcript")

        XCTAssertEqual(polishInvocations, 1)
        XCTAssertEqual(outcomes, [.raw(text: "some transcript", hint: "Polish failed — inserted your words as spoken")])
    }

    func test_recognizer_error_yields_nothing_and_returns_to_idle() {
        let fake = FakeVoiceInput()
        var polishInvocations = 0
        var states: [VoiceSessionController.State] = []
        var outcomes: [VoiceOutcome] = []
        let c = VoiceSessionController(
            voice: fake,
            polish: { _, _ in polishInvocations += 1 },
            onState: { states.append($0) }, onOutcome: { outcomes.append($0) })

        c.startSession(localeTag: "en-US", rawMode: false)
        fake.listener!.onError(.noSpeech)

        XCTAssertEqual(outcomes, [.nothing])
        XCTAssertEqual(polishInvocations, 0)
        XCTAssertEqual(states.last, .idle)
    }

    func test_cancel_during_processing_drops_late_polish_success() {
        let fake = FakeVoiceInput()
        var capturedCallback: ((Result<String, Error>) -> Void)?
        var outcomes: [VoiceOutcome] = []
        let c = VoiceSessionController(
            voice: fake,
            polish: { _, cb in capturedCallback = cb },
            onState: { _ in }, onOutcome: { outcomes.append($0) })

        c.startSession(localeTag: "en-US", rawMode: false)
        fake.listener!.onFinal("some transcript")
        XCTAssertNotNil(capturedCallback, "polish should have been invoked and captured")

        c.cancelSession()
        XCTAssertEqual(outcomes, [.nothing])

        // Backend thread finally responds after the session was cancelled —
        // must be dropped, not appended as a second outcome.
        capturedCallback!(.success("too late"))
        XCTAssertEqual(outcomes, [.nothing])
    }

    func test_onPartialText_forwards_live_transcript_while_listening() {
        let fake = FakeVoiceInput()
        var time = 0
        var partials: [String] = []
        let c = VoiceSessionController(
            voice: fake,
            polish: { _, cb in cb(.success("done")) },
            onState: { _ in }, onOutcome: { _ in }, now: { time })
        c.onPartialText { partials.append($0) }

        c.startSession(localeTag: "en-US", rawMode: false)
        fake.listener!.onPartial("um")
        time += VoiceConfig.partialDebounceMs // outside the debounce window
        fake.listener!.onPartial("um hello")

        XCTAssertEqual(partials, ["um", "um hello"])
    }

    func test_cancel_when_idle_is_a_noop() {
        let fake = FakeVoiceInput()
        var outcomes: [VoiceOutcome] = []
        let c = VoiceSessionController(
            voice: fake,
            polish: { _, cb in cb(.success("done")) },
            onState: { _ in }, onOutcome: { outcomes.append($0) })

        c.cancelSession()

        XCTAssertTrue(outcomes.isEmpty)
        XCTAssertEqual(fake.cancelCalls, 0)
    }

    func test_reentrant_cancel_from_polished_outcome_no_deadlock_no_double_emit() {
        let fake = FakeVoiceInput()
        var outcomes: [VoiceOutcome] = []
        var controllerRef: VoiceSessionController!
        let c = VoiceSessionController(
            voice: fake,
            polish: { text, cb in cb(.success("Polished: \(text)")) },
            onState: { _ in },
            onOutcome: { outcome in
                outcomes.append(outcome)
                if case .polished = outcome { controllerRef.cancelSession() }
            })
        controllerRef = c

        c.startSession(localeTag: "en-US", rawMode: false)
        fake.listener!.onFinal("hello")

        XCTAssertEqual(outcomes, [.polished("Polished: hello")])
        XCTAssertEqual(fake.cancelCalls, 0) // already idle — cancelSession's guard short-circuits
    }

    func test_rapid_partials_within_debounce_throttled_after_first() { // VOICE-010
        let fake = FakeVoiceInput()
        var time = 0
        var partials: [String] = []
        let c = VoiceSessionController(
            voice: fake,
            polish: { _, cb in cb(.success("done")) },
            onState: { _ in }, onOutcome: { _ in }, now: { time })
        c.onPartialText { partials.append($0) }

        c.startSession(localeTag: "en-US", rawMode: false)
        fake.listener!.onPartial("a")   // first partial always forwarded
        time += 50
        fake.listener!.onPartial("ab")  // 50ms — dropped
        time += 99
        fake.listener!.onPartial("abc") // 149ms — still dropped

        XCTAssertEqual(partials, ["a"])
    }

    func test_partials_spaced_debounce_apart_all_forwarded() { // VOICE-010
        let fake = FakeVoiceInput()
        var time = 0
        var partials: [String] = []
        let c = VoiceSessionController(
            voice: fake,
            polish: { _, cb in cb(.success("done")) },
            onState: { _ in }, onOutcome: { _ in }, now: { time })
        c.onPartialText { partials.append($0) }

        c.startSession(localeTag: "en-US", rawMode: false)
        fake.listener!.onPartial("a")
        time += 150
        fake.listener!.onPartial("ab")
        time += 150
        fake.listener!.onPartial("abc")

        XCTAssertEqual(partials, ["a", "ab", "abc"])
    }

    func test_finishSession_stops_recognizer_then_commits_raw_final() {
        let fake = FakeVoiceInput()
        var outcomes: [VoiceOutcome] = []
        let c = VoiceSessionController(
            voice: fake,
            polish: { _, cb in cb(.success("should-not-run")) },
            onState: { _ in }, onOutcome: { outcomes.append($0) })

        c.startSession(localeTag: "vi-VN", rawMode: true)
        c.finishSession()
        XCTAssertEqual(fake.stopCalls, 1)     // release asks recognizer to finalize
        fake.listener!.onFinal("xin chào")    // recognizer delivers the final

        XCTAssertEqual(outcomes, [.raw(text: "xin chào", hint: nil)])
    }

    func test_finishSession_when_idle_is_a_noop() {
        let fake = FakeVoiceInput()
        let c = VoiceSessionController(
            voice: fake, polish: { _, _ in }, onState: { _ in }, onOutcome: { _ in })
        c.finishSession()
        XCTAssertEqual(fake.stopCalls, 0)
    }
}
