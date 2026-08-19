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
