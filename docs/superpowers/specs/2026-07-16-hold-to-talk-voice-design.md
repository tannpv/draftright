# Hold-to-talk voice — DraftRight Android keyboard

**Date:** 2026-07-16
**Scope:** Android custom keyboard (IME) only. iOS/desktop unchanged.
**Status:** Approved design.

## Problem

The keyboard's mic is **tap-to-toggle** today (`onMicTapped` → `startSession`/`cancelSession`). Users want **push-to-talk**: press and hold the mic, speak, release to send — or slide away and release to cancel. Walkie-talkie / Zalo / WhatsApp voice-message style.

## Decisions (from brainstorming)

- **Hold-only.** Replace tap-to-toggle. A tap shorter than the arm threshold does nothing except flash a "Hold to talk" hint.
- **Slide-away to cancel.** Release on the mic → proceed; slide the finger away past a threshold then release → cancel.
- **Insert raw transcription** on proceed. No AI polish on this path (tone buttons remain for rewriting afterward).

## Interaction & state machine

Driven by the ToolbarView mic button's existing `setOnTouchListener` (currently long-press = raw-mode toggle; that toggle is removed). Reuses `VoiceSessionController`'s `IDLE / LISTENING / PROCESSING` machine.

| Touch event | Behaviour |
|---|---|
| `ACTION_DOWN` | Record the down position. Arm a `Handler.postDelayed` at **ARM_MS ≈ 180ms**. |
| arm fires (still held) | `startSession(locale, rawMode = true)` → `LISTENING`. Mic goes active; candidate bar shows live partial + "◀ slide to cancel" hint. |
| `ACTION_MOVE` | If displacement from the down point exceeds **CANCEL_SLOP ≈ 96px** (up/left biased) → **cancel-armed** (hint flips to "release to cancel"). Sliding back within slop → disarm. |
| `ACTION_UP` (session started, not cancel-armed) | `stop()` → recognizer delivers final → **insert raw transcript** at cursor. |
| `ACTION_UP` (cancel-armed) | `cancelSession()` — discard, nothing inserted. |
| `ACTION_UP` before arm fired | Cancel the armed runnable. Flash "Hold to talk". No session. |
| `ACTION_CANCEL` | `cancelSession()`. |

Cancellation of the armed runnable on early UP prevents a race where a quick tap still starts a session.

## Keep listening until release (recognizer)

Android `SpeechRecognizer` auto-endpoints on silence, which would cut a hold short on a mid-sentence pause. `SpeechRecognizerVoiceInput.start()` will set, in the `RecognizerIntent`:

- `EXTRA_SPEECH_INPUT_COMPLETE_SILENCE_LENGTH_MILLIS` — large (e.g. `VoiceConfig.HOLD_SILENCE_MS`, ~10s)
- `EXTRA_SPEECH_INPUT_POSSIBLY_COMPLETE_SILENCE_LENGTH_MILLIS` — large
- `EXTRA_SPEECH_INPUT_MINIMUM_LENGTH_MILLIS` — small

On release, `stop()` (`recognizer.stopListening()`) fetches the final. The existing `VoiceConfig.LISTEN_TIMEOUT_MS` (30s) hard cap stays as a backstop.

**Rejected alternative:** auto-restart-and-stitch the recognizer when it ends early. More robust against stubborn OEM endpointing but adds transcript-stitching complexity. Not built unless device testing shows cutoffs (YAGNI). These extras are hints some OEM recognizers ignore; if a specific device cuts off, revisit.

## Feedback & edge cases

- **Recording:** mic shows an active/recording state (color change); candidate bar streams the partial transcript (existing `onPartialText` path) plus the "◀ slide to cancel" affordance.
- **Cancel-armed:** hint text changes; on release nothing is inserted, candidate preview cleared.
- **No speech** (`VoiceError.NO_SPEECH`) or too-short hold → silent no-op, no error toast.
- **Permission denied** → the existing mic-permission prompt (unchanged).
- **Proceed = raw insert** — no `aiClient.rewrite` round-trip on this path.

## Components touched

- `ToolbarView.kt` — replace the mic long-press/tap wiring with the down/move/up gesture; expose callbacks `onVoiceHoldStart`, `onVoiceHoldCancelArmed(Boolean)`, `onVoiceHoldEnd(cancelled: Boolean)` (or an equivalent small interface) instead of `onMicTapped(Boolean)`.
- `DraftRightIME.kt` — wire those callbacks to `voiceSession.startSession(rawMode=true)` / `stop` / `cancelSession`; keep the raw-insert outcome path (no polish).
- `SpeechRecognizerVoiceInput.kt` — add the silence-length extras.
- `VoiceInput.kt` (`VoiceConfig`) — add `HOLD_SILENCE_MS`, `ARM_MS`, `CANCEL_SLOP_PX` (or house UI constants in ToolbarView).
- `VoiceSessionController.kt` — unchanged state machine; confirm `rawMode=true` inserts the raw transcript with no polish. Adjust only if the raw path isn't already wired.

## Testing

- `VoiceSessionControllerTest` — extend: hold-start → final → raw commit; hold-start → cancel → no commit; too-short (never started) → no session.
- Gesture logic (arm threshold, slop, cancel-arm/disarm) — extract into a pure helper if practical so it's unit-testable without Android touch events; otherwise a thin `androidTest`/manual pass.
- Manual on-device: hold-speak-release inserts; hold-speak-slide-release cancels; quick tap no-ops; mid-sentence pause doesn't cut off.

## Non-goals

- No change to iOS / desktop voice.
- No new voice provider (device `SpeechRecognizer` stays; the provider spec `2026-04-03-voice-input-design.md` is separate).
- No AI-polish-on-release variant (raw only).
