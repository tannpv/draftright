// Package redistrial implements the trial IP rate-limit counter as a
// Redis INCR + first-increment EXPIRE, mirroring the NestJS
// RewriteCacheService.incrementWithExpiry contract.
package redistrial

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultTimeout = 250 * time.Millisecond

// Limiter is the live Redis-backed trial counter. Safe for concurrent use.
type Limiter struct {
	rdb     *redis.Client
	timeout time.Duration
}

// New wires a Limiter with a sane per-call timeout.
func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb, timeout: defaultTimeout}
}

// Incr increments key and, on the first increment (count==1), sets it to
// expire after ttlSec seconds. Returns the new count. A redis error is
// returned to the caller, which decides fail-open posture (the parity
// service treats any error as count 0 → allow, matching Node's catch→0).
func (l *Limiter) Incr(ctx context.Context, key string, ttlSec int) (int64, error) {
	callCtx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	n, err := l.rdb.Incr(callCtx, key).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		l.rdb.Expire(callCtx, key, time.Duration(ttlSec)*time.Second)
	}
	return n, nil
}
