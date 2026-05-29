package domain

// Tone is the rewrite mode the user picked. Mirrors the NestJS
// `Tone` enum (backend/src/rewrite/tone.ts) byte-for-byte — these
// strings end up in `usage_logs.tone` so divergence between Go and
// NestJS would split analytics. Don't add new tones here without also
// adding them in the NestJS enum (Rule #1 — one source of truth even
// across services).
type Tone string

const (
	ToneSimple    Tone = "simple"
	ToneNatural   Tone = "natural"
	TonePolished  Tone = "polished"
	ToneConcise   Tone = "concise"
	ToneTechnical Tone = "technical"
	ToneClaude    Tone = "claude"
	ToneTranslate Tone = "translate"
)

// ParseTone validates a wire string against the known set + returns
// ErrInvalidInput for unknown values. Centralised so the HTTP handler
// + future CLI / batch callers don't each reinvent the validation.
func ParseTone(s string) (Tone, error) {
	switch Tone(s) {
	case ToneSimple, ToneNatural, TonePolished, ToneConcise,
		ToneTechnical, ToneClaude, ToneTranslate:
		return Tone(s), nil
	default:
		return "", ErrInvalidInput
	}
}

// String for fmt + slog logging.
func (t Tone) String() string { return string(t) }
