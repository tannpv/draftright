package parity

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Service ports the NestJS RewriteService.rewrite() + callAI()
// (backend/src/rewrite/rewrite.service.ts, the parity authority) for the
// authenticated, NON-streaming POST /rewrite.
//
// Intentional deviations from Node (value-level only; invisible to the
// per-fixture-DB-reset shadow gate, which compares response bodies on a fresh
// DB):
//   - The Redis result-cache (cache.get / cache.set) is SKIPPED.
//   - The background batch pre-generation of other tones is SKIPPED.
//   - The usage_logs write (usageService.log) is SKIPPED.
//   - The Prometheus metrics.observe calls are SKIPPED.
//
// What remains is the externally-observable contract: quota gate → prompt
// resolution → provider call → response envelope.

// completer is the consumer-side port for a blocking AI completion. The real
// aicall.Completer satisfies it; tests inject a fake.
type completer interface {
	Complete(ctx context.Context, system, user string) (string, int64, error)
}

// entitlements is the consumer-side port for the daily-quota lookup. The real
// subscription.Service satisfies it via ResolveDailyLimit.
type entitlements interface {
	ResolveDailyLimit(ctx context.Context, userID string) (int, error)
}

// usageCounter is the consumer-side port for today's usage count. The real
// usage.Counter satisfies it via CountToday.
type usageCounter interface {
	CountToday(ctx context.Context, userID string) (int, error)
}

// Service runs the authenticated rewrite flow.
type Service struct {
	c     completer
	ents  entitlements
	usage usageCounter

	// Trial seam (set via WithTrial; nil for authed-only wiring).
	trial      TrialLimiter
	trialLimit int
	now        func() time.Time
}

// NewService wires the rewrite dependencies. now defaults to time.Now so the
// clock is never nil even on the authed-only path (WithTrial overrides it).
func NewService(c completer, ents entitlements, usage usageCounter) *Service {
	return &Service{c: c, ents: ents, usage: usage, now: time.Now}
}

// WithTrial wires the public-trial seam without disturbing the NewService
// signature (so authed-rewrite tests and existing wiring stay untouched).
// limit mirrors Node's TRIAL_LIMIT (3 in production, 999 otherwise); now
// defaults to time.Now when nil.
func (s *Service) WithTrial(l TrialLimiter, limit int, now func() time.Time) *Service {
	s.trial = l
	s.trialLimit = limit
	if now == nil {
		now = time.Now
	}
	s.now = now
	return s
}

// ErrQuotaExceeded is returned when the caller is at/over their daily limit.
// Node throws HttpException({error:'Daily limit reached', …}, 429); the
// AllExceptionsFilter drops the extra fields, leaving the bare message.
var ErrQuotaExceeded = errors.New("Daily limit reached")

// ErrTrialLimit is returned when the IP-keyed trial counter exceeds
// TRIAL_LIMIT. Node throws HttpException({error:'Trial limit reached. Sign up
// for unlimited rewrites!'}, 429); the AllExceptionsFilter drops the extra
// fields and inferCode(429) supplies code: "rate-limited".
var ErrTrialLimit = errors.New("Trial limit reached. Sign up for unlimited rewrites!")

// ErrProviderFailed wraps any upstream provider error. Node maps every
// provider failure to a generic 502 so sensitive internals never reach a
// client; the handler renders providerUnavailableMsg.
var ErrProviderFailed = errors.New("provider failed")

// UnknownToneError mirrors callAI's resolvePrompt==null path: Node throws
// HttpException({error:`Unknown tone: ${tone}`}, 400). Unreachable through the
// DTO (validation rejects it first) but kept faithful.
type UnknownToneError struct{ Tone string }

func (e *UnknownToneError) Error() string { return "Unknown tone: " + e.Tone }

// providerUnavailableMsg is the user-facing copy for any provider failure.
// Mirrors Node's PROVIDER_UNAVAILABLE_MESSAGE byte-for-byte.
const providerUnavailableMsg = "Rewrite service is temporarily unavailable. Please try again shortly."

// rewriteEnvelope is the rewrite/translate success body. Field order pins the
// JSON key order: rewritten_text, usage_today, daily_limit.
type rewriteEnvelope struct {
	RewrittenText string `json:"rewritten_text"`
	UsageToday    int    `json:"usage_today"`
	DailyLimit    int    `json:"daily_limit"`
}

// grammarEnvelope is the grammar_check success body. Field order: grammar,
// usage_today, daily_limit.
type grammarEnvelope struct {
	Grammar    json.RawMessage `json:"grammar"`
	UsageToday int             `json:"usage_today"`
	DailyLimit int             `json:"daily_limit"`
}

// trialRewriteEnvelope is the public-trial rewrite/translate success body.
// Unlike rewriteEnvelope it omits usage_today / daily_limit (Node's trial path
// returns only the rewritten text).
type trialRewriteEnvelope struct {
	RewrittenText string `json:"rewritten_text"`
}

// trialGrammarEnvelope is the public-trial grammar_check success body. Omits
// the usage fields, mirroring Node's trial path.
type trialGrammarEnvelope struct {
	Grammar json.RawMessage `json:"grammar"`
}

// failedGrammar mirrors Node's parseGrammarResult fallback object emitted when
// the provider text is not valid JSON.
const failedGrammar = `{"score":0,"issues":[],"error":"Failed to parse grammar analysis"}`

// Rewrite ports RewriteService.rewrite(): quota gate → callAI → envelope.
// Returns an opaque marshalable value (rewriteEnvelope or grammarEnvelope) on
// success, or one of the typed errors above.
func (s *Service) Rewrite(ctx context.Context, userID, text, tone, target, source string) (any, error) {
	dailyLimit, err := s.ents.ResolveDailyLimit(ctx, userID)
	if err != nil {
		return nil, err
	}
	usageToday, err := s.usage.CountToday(ctx, userID)
	if err != nil {
		return nil, err
	}

	// -1 is the ONLY unlimited sentinel here — match Node exactly (NOT the
	// rewrite domain's <=0 rule).
	if dailyLimit != -1 && usageToday >= dailyLimit {
		return nil, ErrQuotaExceeded
	}

	out, err := s.callAI(ctx, text, tone, target, source)
	if err != nil {
		return nil, err
	}

	if tone == "grammar_check" {
		return grammarEnvelope{
			Grammar:    parseGrammarResult(out),
			UsageToday: usageToday + 1,
			DailyLimit: dailyLimit,
		}, nil
	}

	return rewriteEnvelope{
		RewrittenText: out,
		UsageToday:    usageToday + 1,
		DailyLimit:    dailyLimit,
	}, nil
}

// TrialRewrite ports RewriteService.trialRewrite(): IP-keyed daily gate →
// callAI → usage-free envelope. The clientIp only forms the rate-limit key.
func (s *Service) TrialRewrite(ctx context.Context, text, tone, clientIp, target, source string) (any, error) {
	today := s.now().UTC().Format("2006-01-02")
	key := "trial:" + clientIp + ":" + today

	count, err := s.trial.Incr(ctx, key, 86400)
	if err != nil {
		// Fail-open: any redis error → count 0 → allow (mirrors Node's catch→0).
		count = 0
	}
	if count > int64(s.trialLimit) {
		return nil, ErrTrialLimit
	}

	// Truncate to 500 runes. This only bounds the prompt sent upstream; the
	// rewrite output value is ignored by the shadow gate, so rune-vs-UTF16
	// precision is value-invisible.
	if r := []rune(text); len(r) > 500 {
		text = string(r[:500])
	}

	out, err := s.callAI(ctx, text, tone, target, source)
	if err != nil {
		return nil, err
	}

	if tone == "grammar_check" {
		return trialGrammarEnvelope{Grammar: parseGrammarResult(out)}, nil
	}
	return trialRewriteEnvelope{RewrittenText: out}, nil
}

// callAI ports RewriteService.callAI(): resolve the prompt, call the provider,
// and return the raw provider text. Returns *UnknownToneError for an unknown
// tone or ErrProviderFailed for any provider error. Exposed (lowercase but
// reusable in-package) so the trial endpoint can share it.
func (s *Service) callAI(ctx context.Context, text, tone, target, source string) (string, error) {
	prompt := ResolvePrompt(tone, target, source)
	if prompt == "" {
		return "", &UnknownToneError{Tone: tone}
	}
	out, _, err := s.c.Complete(ctx, prompt, text)
	if err != nil {
		return "", ErrProviderFailed
	}
	return out, nil
}

// parseGrammarResult mirrors Node's parseGrammarResult: parse the provider text
// as JSON for the `grammar` field; on parse error, substitute the fixed
// failed-parse object.
func parseGrammarResult(text string) json.RawMessage {
	if json.Valid([]byte(text)) {
		return json.RawMessage(text)
	}
	return json.RawMessage(failedGrammar)
}
