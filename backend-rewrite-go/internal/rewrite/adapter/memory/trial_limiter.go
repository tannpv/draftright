package memory

import (
	"context"
	"sync"
)

// TrialLimiter is an in-process counting limiter for the dev/no-Redis path.
// It increments a per-key counter; ttlSec is accepted for interface parity but
// ignored (process-lifetime counters are fine for a single-node dev backend).
// Production uses the Redis-backed redistrial adapter instead. It satisfies
// parity.TrialLimiter structurally.
type TrialLimiter struct {
	mu     sync.Mutex
	counts map[string]int64
}

// NewTrialLimiter starts with an empty counter map.
func NewTrialLimiter() *TrialLimiter {
	return &TrialLimiter{counts: map[string]int64{}}
}

// Incr increments the counter at key and returns the new count. Safe for
// concurrent use.
func (l *TrialLimiter) Incr(_ context.Context, key string, _ int) (int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.counts[key]++
	return l.counts[key], nil
}
