package memory

import (
	"context"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
)

// RateLimiter is the simplest possible domain.RateLimiter — returns a
// fixed error (or nil) on every call. Tests that need per-user budget
// behaviour can wrap this with a counter, but most just want "always
// pass" or "always reject".
type RateLimiter struct {
	err error
}

// NewRateLimiter starts with no error (every call passes).
func NewRateLimiter() *RateLimiter { return &RateLimiter{} }

// WithError makes Check always return the given error.
// Pass domain.ErrRateLimited to exercise the "burst protection" path.
func (r *RateLimiter) WithError(err error) *RateLimiter {
	r.err = err
	return r
}

// Check returns the stubbed error or nil.
func (r *RateLimiter) Check(_ context.Context, _ domain.UserID) error {
	return r.err
}
