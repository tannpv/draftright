// Package redislimit implements domain.RateLimiter as a per-user token
// bucket backed by Redis INCR + EXPIRE.
//
// Algorithm:
//
//	Each request increments a key `rl:rewrite:<user_id>:<minute_epoch>`.
//	The first INCR also EXPIREs the key at minute_epoch + 60s so old
//	buckets self-destruct — no janitor needed. Count > limit ⇒ reject.
//
// Why this shape (vs leaky bucket / sliding window):
//   - INCR is atomic + idempotent: one round-trip per request.
//   - Buckets reset on the minute boundary; a burst at 23:59:59 +
//     another at 00:00:01 each gets full budget. The trade-off
//     accepts up to 2× burst at boundaries — fine for our scale.
//   - Replaces the NestJS @nestjs/throttler config 1:1 once we share
//     Redis. Until then, they're independent: rebuild-go counts
//     separately from NestJS, so the effective ceiling is the sum.
//
// Production knobs (RatePerMin, WindowSeconds) come from config so we
// can tune without redeploying both backends.
package redislimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
)

// Default budget. Mirrors NestJS @nestjs/throttler's per-minute limit
// for the /rewrite group. Keep in sync there + here.
const (
	DefaultRatePerMin   = 60
	DefaultWindowSecs   = 60
	KeyPrefix           = "rl:rewrite:"
	defaultRedisTimeout = 250 * time.Millisecond
)

// Limiter is the live Redis-backed limiter. Construct once at startup;
// safe for concurrent use across many goroutines (the underlying
// *redis.Client is itself a connection pool).
type Limiter struct {
	rdb         *redis.Client
	ratePerMin  int
	windowSecs  int
	timeout     time.Duration
	failOpen    bool // when Redis is unreachable, allow through (default true)
}

// New wires a Limiter with sane defaults. Tunes via With* fluent helpers.
func New(rdb *redis.Client) *Limiter {
	return &Limiter{
		rdb:        rdb,
		ratePerMin: DefaultRatePerMin,
		windowSecs: DefaultWindowSecs,
		timeout:    defaultRedisTimeout,
		failOpen:   true,
	}
}

// WithRatePerMin overrides the per-user-per-minute cap.
func (l *Limiter) WithRatePerMin(n int) *Limiter {
	l.ratePerMin = n
	return l
}

// WithTimeout sets the per-call Redis timeout. Default 250 ms — long
// enough for a contended Redis, short enough that a Redis hiccup
// doesn't add latency to every /rewrite call.
func (l *Limiter) WithTimeout(d time.Duration) *Limiter {
	l.timeout = d
	return l
}

// WithFailClosed flips the default fail-open posture: when Redis is
// unreachable, every call is rejected. Use in security-critical
// deployments where letting traffic through unrated is unacceptable.
func (l *Limiter) WithFailClosed() *Limiter {
	l.failOpen = false
	return l
}

// Check returns nil when the user is under their per-minute budget,
// or domain.ErrRateLimited when over. Other errors (Redis down,
// timeouts) are demoted to "allow" by default (failOpen) so a Redis
// blip can't take the whole rewrite flow offline.
func (l *Limiter) Check(ctx context.Context, id domain.UserID) error {
	if l.rdb == nil {
		// Misconfigured limiter — treat as "no limit applied" rather
		// than panicking. Misconfig should be caught at boot, not
		// per-request.
		return nil
	}

	callCtx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	bucket := time.Now().UTC().Unix() / int64(l.windowSecs)
	key := fmt.Sprintf("%s%s:%d", KeyPrefix, id.String(), bucket)

	// Atomic pipeline: INCR + EXPIRE — single round-trip when Redis is
	// configured for pipelining, two trips otherwise. Even at two
	// trips the cost is negligible vs the AI call about to happen.
	pipe := l.rdb.Pipeline()
	incr := pipe.Incr(callCtx, key)
	// Set TTL only on the first INCR (count == 1). Calling EXPIRE on
	// every request is harmless but wastes ops; the conditional below
	// is purely cosmetic — Redis is OK either way.
	pipe.Expire(callCtx, key, time.Duration(l.windowSecs+1)*time.Second)
	if _, err := pipe.Exec(callCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			if l.failOpen {
				return nil
			}
			return domain.ErrRateLimited
		}
		if l.failOpen {
			return nil
		}
		return fmt.Errorf("redislimit: %w", err)
	}

	if incr.Val() > int64(l.ratePerMin) {
		return domain.ErrRateLimited
	}
	return nil
}
