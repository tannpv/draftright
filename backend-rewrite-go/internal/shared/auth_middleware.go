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
	"encoding/json"
	"errors"
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
				writeUnauthorized(w, "missing or malformed Authorization header")
				return
			}
			claims, err := v.Verify(tok)
			if err != nil {
				// Log at debug — auth failures are normal at scale (expired
				// tokens, refresh races). Surface details to the caller via
				// a generic message; never echo the raw token to logs.
				log.Debug("auth: token rejected", "err", err.Error())
				writeUnauthorized(w, friendlyAuthError(err))
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

// friendlyAuthError maps internal auth error sentinels to a single
// user-facing message. We don't tell the client which specific
// validation failed (expired vs invalid vs malformed) so an attacker
// can't probe for valid-shape-wrong-signature.
func friendlyAuthError(err error) string {
	switch {
	case errors.Is(err, auth.ErrTokenExpired):
		return "token expired"
	case errors.Is(err, auth.ErrMissingSubject):
		return "token missing subject claim"
	default:
		return "invalid token"
	}
}

// writeUnauthorized writes a 401 with a JSON error body. Single helper
// so middleware and handlers don't drift on the body shape.
func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("WWW-Authenticate", `Bearer realm="rewrite-go"`)
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": msg,
	})
}
