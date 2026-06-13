package shared

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// requestIDCtxKey is unexported so only this package can write the value;
// handlers read it via RequestIDFromContext.
type requestIDCtxKey struct{}

// requestIDHeader is the canonical header both backends use. Matches
// the Node filter's `req.requestId` source so a request traced through
// Caddy keeps one id end to end.
const requestIDHeader = "X-Request-Id"

// RequestID middleware mints a UUID per request (or reuses an inbound
// X-Request-Id), stamps it on the response header, and threads it into
// the context. The error envelope reads it so every error body carries
// `request_id` — parity with the Node AllExceptionsFilter.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDCtxKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the id stamped by RequestID, or "" when
// the middleware did not run (e.g. a unit test calling a handler
// directly). Callers tolerate "" — an empty request_id is still valid
// JSON, just less useful for support.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDCtxKey{}).(string)
	return id
}
