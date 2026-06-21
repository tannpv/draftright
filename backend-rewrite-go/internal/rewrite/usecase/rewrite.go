// Package usecase holds the rewrite-service's application logic — the
// "use cases" of clean architecture. Depends ONLY on internal/rewrite/domain;
// never imports adapters, HTTP, or any concrete tech. The adapter
// layer plugs into the domain.Ports defined in domain/ports.go.
//
// One file per use case. Today that's just rewrite.go; future tasks
// (refund, history, etc.) get their own file.
package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/provenance"
)

// RewriteLogEntry is the training-data row captured on a clean streamed finish.
type RewriteLogEntry struct {
	Tone, InputText, OutputText, Model, ProviderType string
	ResponseTimeMs                                   int64
}

// RewriteLogger is the consumer-side fire-and-forget training-data sink. A nil
// sink disables capture; the use case must never panic on a nil RewriteLog.
type RewriteLogger interface {
	LogRewrite(ctx context.Context, e RewriteLogEntry)
}

// RewriteDeps groups every port the use case needs. Built once in
// main.go (the composition root), passed to every call. The struct is
// the canonical "what does this use case depend on" inventory — adding
// a port later means one place to wire it in.
type RewriteDeps struct {
	Users     domain.UserRepo
	Provider  domain.AiProvider
	RateLimit domain.RateLimiter
	// Metrics is the telemetry sink. Defaults to a no-op if nil so
	// the use case never panics on an unwired field.
	Metrics domain.Metrics
	// Now() is injectable so tests can pin the clock + assert exact
	// response_time_ms on the usage log row. Production wires
	// time.Now.
	Now func() time.Time
	// Log is non-fatal: telemetry only. The use case must never panic
	// because Log is nil — guard via nopLogger() below.
	Log *slog.Logger
	// RewriteLog is the fire-and-forget training-data sink. Captured on a
	// CLEAN streamed finish only (never on error/cancel). Nil-safe: a nil
	// RewriteLog disables capture and the use case must never panic on it.
	RewriteLog RewriteLogger
}

// Rewrite is the orchestrator. Synchronous pre-flight (rate-limit,
// quota), then a goroutine streams tokens + logs usage when the
// provider closes its stream.
//
// Returns (tokens, errs, nil) on the happy path. Caller drains BOTH
// channels until they close. ctx cancellation aborts the upstream
// call and surfaces ctx.Err() on errs.
//
// Returns (nil, nil, err) for pre-flight failures the transport layer
// can map directly to HTTP statuses (domain.ErrRateLimited → 429, etc).
func Rewrite(
	ctx context.Context,
	deps RewriteDeps,
	userID domain.UserID,
	req domain.RewriteRequest,
) (<-chan string, <-chan error, error) {
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	log := deps.Log
	if log == nil {
		log = nopLogger()
	}
	metrics := deps.Metrics
	if metrics == nil {
		metrics = noopMetrics{}
	}

	preflightStart := now()
	providerName := deps.Provider.Name()

	// 1. Cheap pre-check: per-minute Redis token bucket. Returns
	//    ErrRateLimited fast — saves the DB round-trip when a runaway
	//    client is hammering us.
	if err := deps.RateLimit.Check(ctx, userID); err != nil {
		metrics.ObserveRewrite(outcomeFromErr(err), req.Tone(), providerName, now().Sub(preflightStart))
		return nil, nil, err
	}

	// 2. Load user + plan + today's usage. Single round-trip via the
	//    FindUserWithPlan query.
	user, err := deps.Users.Find(ctx, userID)
	if err != nil {
		metrics.ObserveRewrite(outcomeFromErr(err), req.Tone(), providerName, now().Sub(preflightStart))
		return nil, nil, err
	}

	// 3. Daily quota check. NestJS does the same comparison; keep the
	//    "0 = unlimited" convention in lockstep (domain.Plan).
	if err := user.CheckQuota(); err != nil {
		metrics.ObserveRewrite(outcomeFromErr(err), req.Tone(), providerName, now().Sub(preflightStart))
		return nil, nil, err
	}

	// 4. Stream from the provider. The provider owns its own
	//    cancellation via ctx; we wrap its channels so we can:
	//      - forward tokens to the caller while accumulating output
	//        length for the usage log,
	//      - write the usage log when the provider stream closes
	//        cleanly (not on error, not on cancel).
	ctx, prov := provenance.NewContext(ctx)
	provTokens, provErrs := deps.Provider.Stream(ctx, req)
	outTokens := make(chan string)
	outErrs := make(chan error, 1)

	go func() {
		defer close(outTokens)
		defer close(outErrs)

		// Anchor both the metric duration AND the usage log
		// ResponseTimeMs on preflightStart. Preflight (Redis + DB
		// round-trip) is ~1-5 ms in practice — conflating into one
		// "request duration" simplifies the clock interface (single
		// `now` call site for the entire pipeline) and keeps metric +
		// log values comparable.
		startedAt := preflightStart
		var output strings.Builder
		var tokenCount int
		streamFailed := false

		// recordOutcome MUST be called on every terminal path
		// (success, mid-stream error, ctx cancel) — guarantees the
		// metric is bumped exactly once per request lifecycle.
		recordOutcome := func(outcome domain.RewriteOutcome) {
			dur := now().Sub(preflightStart)
			metrics.ObserveRewrite(outcome, req.Tone(), providerName, dur)
			if tokenCount > 0 {
				metrics.AddTokensStreamed(providerName, tokenCount)
			}
		}

		for {
			select {
			case <-ctx.Done():
				recordOutcome(domain.OutcomeClientGone)
				outErrs <- ctx.Err()
				return

			case tok, open := <-provTokens:
				if !open {
					// Tokens channel closed. Three reasons to handle:
					//   1. ctx canceled — provider unwound; no log.
					//   2. Provider errored — we may not have seen the
					//      error yet because select is random. Drain
					//      provErrs non-blockingly so we don't log
					//      usage for a failed call.
					//   3. Clean finish — log usage.
					if ctx.Err() != nil {
						recordOutcome(domain.OutcomeClientGone)
						outErrs <- ctx.Err()
						return
					}
					select {
					case lateErr, ok := <-provErrs:
						if ok && lateErr != nil {
							streamFailed = true
							recordOutcome(domain.OutcomeProviderFailed)
							outErrs <- lateErr
							return
						}
					default:
					}
					if !streamFailed {
						elapsedMs := now().Sub(startedAt).Milliseconds()
						err := deps.Users.LogUsage(ctx, domain.UsageLog{
							UserID:         userID,
							Tone:           req.Tone(),
							InputLength:    req.InputLength(),
							OutputLength:   int32(output.Len()),
							AIProviderID:   deps.Provider.ID(),
							ResponseTimeMs: int32(elapsedMs),
						})
						if err != nil {
							// Telemetry write fails are non-fatal —
							// the user already got their rewrite. Log
							// so ops can audit.
							log.Warn("usage log write failed",
								"user_id", userID.String(),
								"provider", providerName,
								"err", err.Error())
						}
						if deps.RewriteLog != nil {
							model, ptype := prov.Read()
							deps.RewriteLog.LogRewrite(ctx, RewriteLogEntry{
								Tone:           req.Tone().String(),
								InputText:      req.Text(),
								OutputText:     output.String(),
								Model:          model,
								ProviderType:   ptype,
								ResponseTimeMs: elapsedMs,
							})
						}
						recordOutcome(domain.OutcomeOK)
					}
					return
				}
				// Forward + accumulate. Inner select on ctx so a
				// disconnected client doesn't wedge us on a full
				// outbound channel.
				output.WriteString(tok)
				tokenCount++
				select {
				case outTokens <- tok:
				case <-ctx.Done():
					recordOutcome(domain.OutcomeClientGone)
					outErrs <- ctx.Err()
					return
				}

			case provErr, open := <-provErrs:
				if !open {
					// Error channel closed without a value — treat as
					// no-error-this-round; the provider may close
					// tokens next loop iteration.
					provErrs = nil
					continue
				}
				if provErr != nil {
					streamFailed = true
					recordOutcome(domain.OutcomeProviderFailed)
					outErrs <- provErr
					return
				}
			}
		}
	}()

	return outTokens, outErrs, nil
}

// nopLogger returns a discard slog so callers don't need to pass one
// in tests. Production main.go always wires the real logger.
func nopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(devNull{}, nil))
}

type devNull struct{}

func (devNull) Write(p []byte) (int, error) { return len(p), nil }

// noopMetrics is the in-package fallback when deps.Metrics is nil.
// Test code can swap a real implementation; production wires
// platform/metrics.Prometheus.
type noopMetrics struct{}

func (noopMetrics) ObserveRewrite(_ domain.RewriteOutcome, _ domain.Tone, _ string, _ time.Duration) {
}
func (noopMetrics) AddTokensStreamed(_ string, _ int) {}

// outcomeFromErr maps a use-case error to the bounded outcome label
// set. Single place owns the mapping so the metric cardinality stays
// stable (Rule #1).
func outcomeFromErr(err error) domain.RewriteOutcome {
	switch {
	case err == nil:
		return domain.OutcomeOK
	case errors.Is(err, domain.ErrRateLimited):
		return domain.OutcomeRateLimited
	case errors.Is(err, domain.ErrQuotaExceeded):
		return domain.OutcomeQuotaExceeded
	case errors.Is(err, domain.ErrUserNotFound):
		return domain.OutcomeUserNotFound
	case errors.Is(err, domain.ErrInvalidInput):
		return domain.OutcomeInvalidInput
	case errors.Is(err, domain.ErrProviderUnavailable),
		errors.Is(err, domain.ErrProviderFailed):
		return domain.OutcomeProviderFailed
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return domain.OutcomeClientGone
	default:
		return domain.OutcomeInternal
	}
}
