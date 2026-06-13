// Package shared holds cross-cutting HTTP concerns reused by every
// feature module — the chi router build, JWT auth middleware, request
// logging, and the JSON render helper. It depends on platform + the
// shared sqlc types, never on a feature module.
//
// This file: JWT extraction middleware. Handlers downstream pull the
// verified claims out of context via ClaimsFromContext().
package shared

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	auth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
)

// ctxKey is unexported on purpose — only handlers in this package can
// pull claims out of context. Prevents accidental cross-package use
// where one layer reaches into another's request state.
type ctxKey int

const claimsKey ctxKey = 1

// RequireAuth returns a middleware that:
//  1. Extracts the Bearer token from the Authorization header
//  2. Verifies via the shared auth.Verifier
//  3. Injects the verified Claims into the request context
//  4. On any failure: writes a 401 with a JSON error body and stops the chain
//
// Reusable across every authenticated route — wire once in main.go,
// apply to whichever chi groups need auth (Rule #1).
func RequireAuth(v *auth.Verifier, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := extractBearer(r.Header.Get("Authorization"))
			if !ok {
				writeUnauthorized(w, r)
				return
			}
			claims, err := v.Verify(tok)
			if err != nil {
				// Log at debug — auth failures are normal at scale (expired
				// tokens, refresh races). The response body stays generic
				// ("Unauthorized") so an attacker can't probe which check
				// failed; never echo the raw token to logs.
				log.Debug("auth: token rejected", "err", err.Error())
				writeUnauthorized(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(ContextWithClaims(r.Context(), claims)))
		})
	}
}

// ContextWithClaims stamps verified claims onto a context under the
// unexported claimsKey. The single writer of that key — RequireAuth
// uses it, tests use it to simulate an authenticated request — so the
// context wiring lives in exactly one place (Rule #1).
func ContextWithClaims(ctx context.Context, c *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

// ClaimsFromContext returns the auth claims a previous RequireAuth
// middleware stamped on the request. Returns (nil, false) when the
// route wasn't wrapped — call sites that require auth should handle
// the false case as an internal error (router misconfiguration).
func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*auth.Claims)
	return c, ok
}

// extractBearer parses "Authorization: Bearer <token>" without
// allocation when malformed. Returns (token, ok) for the happy path.
func extractBearer(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	tok := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if tok == "" {
		return "", false
	}
	return tok, true
}

// writeUnauthorized emits the canonical error envelope for any auth
// rejection: { error: "Unauthorized", code: "invalid-token",
// request_id }, status 401 — byte-for-byte the Node global filter's
// output for a JwtAuthGuard rejection (UnauthorizedException →
// inferCode(401)="invalid-token", message="Unauthorized"). Routes
// through shared.WriteError so every error body stays on one contract
// (Rule #1).
func writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, "invalid-token", "Unauthorized")
}
