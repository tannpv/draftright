package parity

import "context"

// TrialLimiter is the consumer-side port for the IP-keyed trial counter.
// Incr increments the counter at key and returns the new count; on first
// increment (count==1) the implementation sets the key to expire after
// ttlSec seconds. Mirrors Node's RewriteCacheService.incrementWithExpiry.
type TrialLimiter interface {
	Incr(ctx context.Context, key string, ttlSec int) (int64, error)
}
