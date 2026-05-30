// Package domain holds the entities + ports of the rewrite service.
// Per clean architecture this is the INNERMOST layer:
//   - no imports outside this package + stdlib + tiny utilities
//     (uuid, errors).
//   - never depends on adapter/, usecase/, http/, or platform/.
//
// The compiler enforces this by Go's import-cycle prohibition + the
// allow-list above. A future Cargo-workspace-style enforcement is
// possible via go-cleanarch lint in CI; until then it's discipline +
// PR review.
package domain

import "errors"

// Sentinel errors — exported so use cases, adapters, and the HTTP
// transport can errors.Is them without importing each other. One
// canonical name per failure mode (Rule #1 — no magic strings for
// error categorisation).
//
// HTTP mapping (in transport/http/errors.go later):
//   ErrQuotaExceeded      → 429 Too Many Requests
//   ErrUserNotFound       → 404 (or 401 if we want to leak less)
//   ErrInvalidInput       → 400 Bad Request
//   ErrProviderUnavailable → 503 Service Unavailable
//   ErrProviderFailed     → 502 Bad Gateway
var (
	ErrQuotaExceeded       = errors.New("domain: quota exceeded")
	ErrUserNotFound        = errors.New("domain: user not found")
	ErrInvalidInput        = errors.New("domain: invalid input")
	ErrProviderUnavailable = errors.New("domain: no active ai provider")
	ErrProviderFailed      = errors.New("domain: ai provider call failed")
	ErrRateLimited         = errors.New("domain: rate limit exceeded")
)
