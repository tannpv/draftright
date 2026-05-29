// Package usecase holds the rewrite-service's application logic — the
// "use cases" of clean architecture. Depends ONLY on internal/domain;
// never imports adapters, HTTP, or any concrete tech. The adapter
// layer plugs into the domain.Ports defined in domain/ports.go.
//
// One file per use case. Today that's just rewrite.go; future tasks
// (refund, history, etc.) get their own file.
package usecase

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/domain"
)

// RewriteDeps groups every port the use case needs. Built once in
// main.go (the composition root), passed to every call. The struct is
// the canonical "what does this use case depend on" inventory — adding
// a port later means one place to wire it in.
type RewriteDeps struct {
	Users     domain.UserRepo
	Provider  domain.AiProvider
	RateLimit domain.RateLimiter
	// Now() is injectable so tests can pin the clock + assert exact
	// response_time_ms on the usage log row. Production wires
	// time.Now.
	Now func() time.Time
	// Log is non-fatal: telemetry only. The use case must never panic
	// because Log is nil — guard via nopLogger() below.
	Log *slog.Logger
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

	// 1. Cheap pre-check: per-minute Redis token bucket. Returns
	//    ErrRateLimited fast — saves the DB round-trip when a runaway
	//    client is hammering us.
	if err := deps.RateLimit.Check(ctx, userID); err != nil {
		return nil, nil, err
	}

	// 2. Load user + plan + today's usage. Single round-trip via the
	//    FindUserWithPlan query.
	user, err := deps.Users.Find(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	// 3. Daily quota check. NestJS does the same comparison; keep the
	//    "0 = unlimited" convention in lockstep (domain.Plan).
	if err := user.CheckQuota(); err != nil {
		return nil, nil, err
	}

	// 4. Stream from the provider. The provider owns its own
	//    cancellation via ctx; we wrap its channels so we can:
	//      - forward tokens to the caller while accumulating output
	//        length for the usage log,
	//      - write the usage log when the provider stream closes
	//        cleanly (not on error, not on cancel).
	provTokens, provErrs := deps.Provider.Stream(ctx, req)
	outTokens := make(chan string)
	outErrs := make(chan error, 1)

	go func() {
		defer close(outTokens)
		defer close(outErrs)

		startedAt := now()
		var output strings.Builder
		streamFailed := false

		for {
			select {
			case <-ctx.Done():
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
						outErrs <- ctx.Err()
						return
					}
					select {
					case lateErr, ok := <-provErrs:
						if ok && lateErr != nil {
							streamFailed = true
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
								"provider", deps.Provider.Name(),
								"err", err.Error())
						}
					}
					return
				}
				// Forward + accumulate. Inner select on ctx so a
				// disconnected client doesn't wedge us on a full
				// outbound channel.
				output.WriteString(tok)
				select {
				case outTokens <- tok:
				case <-ctx.Done():
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
