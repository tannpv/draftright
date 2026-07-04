package domain

// InputKind distinguishes typed text from voice-dictated text on the
// rewrite request. Mirrors the NestJS `input_kind` field
// (backend/src/rewrite/rewrite.dto.ts) byte-for-byte — keep the wire
// strings in lockstep across services (Rule #1 — one source of truth
// even across services).
type InputKind string

const (
	InputKindTyped  InputKind = "typed"
	InputKindSpeech InputKind = "speech"
)

// SpeechPreamble mirrors parity.SpeechPreamble (backend/src/rewrite/tones.ts
// SPEECH_PREAMBLE) byte-for-byte — note the trailing single space; do not
// trim it. Prepended to the per-tone system prompt on the native streaming
// path when InputKind is speech, so voice-dictated input gets the same
// filler-word/punctuation cleanup instruction the non-streaming parity path
// applies via ResolvePrompt.
const SpeechPreamble = "The input is dictated speech: remove filler words and false starts, restore punctuation and casing, keep the meaning and language. "

// ApplySpeechPreamble prepends SpeechPreamble to base when kind is
// InputKindSpeech; otherwise returns base unchanged. The native streaming
// adapters (openai/anthropic/ollama Stream) each build their own per-tone
// system prompt via a local systemFmt func(Tone) string — this helper is
// the single shared seam so all three prepend the identical preamble
// instead of three copies of the same if-statement (Rule #1).
func ApplySpeechPreamble(base string, kind InputKind) string {
	if kind == InputKindSpeech {
		return SpeechPreamble + base
	}
	return base
}

// ParseInputKind validates a wire string against the known set + returns
// ErrInvalidInput for unknown values. Empty string defaults to
// InputKindTyped — older clients that don't send input_kind at all are
// typed by definition. Centralised so the HTTP handler + future CLI /
// batch callers don't each reinvent the validation.
func ParseInputKind(s string) (InputKind, error) {
	switch InputKind(s) {
	case "":
		return InputKindTyped, nil
	case InputKindTyped, InputKindSpeech:
		return InputKind(s), nil
	default:
		return "", ErrInvalidInput
	}
}

// String for fmt + slog logging.
func (k InputKind) String() string { return string(k) }
