package transport

import (
	"errors"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
)

// httpError is the wire shape every error response uses. Single helper
// + single struct so error bodies never drift between handlers
// (Rule #1 — one place owns the contract).
type httpError struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// statusForDomainErr maps domain sentinel errors to HTTP status + a
// stable machine-readable code. Centralised so every handler — current
// and future — uses the same mapping. Add new domain sentinels here in
// one place; never branch on error type inside a handler.
//
// Codes are kebab-case + service-prefixed so clients can match without
// guessing at message strings ("user-not-found" beats parsing
// "domain: user not found").
func statusForDomainErr(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrRateLimited):
		return http.StatusTooManyRequests, "rate-limited"
	case errors.Is(err, domain.ErrQuotaExceeded):
		// 402 Payment Required — semantically closest to "your plan
		// won't let you do more today". NestJS uses 429; we diverge
		// deliberately so clients can distinguish "slow down" (429)
		// from "upgrade your plan" (402).
		return http.StatusPaymentRequired, "quota-exceeded"
	case errors.Is(err, domain.ErrUserNotFound):
		// Auth was valid (RequireAuth passed), but the JWT's sub
		// claim doesn't match any row. Treat as 401 — the token
		// belongs to a deleted user and any session built on it is
		// untrustworthy.
		return http.StatusUnauthorized, "user-not-found"
	case errors.Is(err, domain.ErrInvalidInput):
		return http.StatusBadRequest, "invalid-input"
	case errors.Is(err, domain.ErrProviderUnavailable):
		return http.StatusServiceUnavailable, "provider-unavailable"
	case errors.Is(err, domain.ErrProviderFailed):
		return http.StatusBadGateway, "provider-failed"
	default:
		// Everything unmapped is a programmer error. Log noisily
		// (caller's responsibility) and return 500.
		return http.StatusInternalServerError, "internal"
	}
}
