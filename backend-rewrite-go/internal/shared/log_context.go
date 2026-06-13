package shared

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// Request-scoped logger plumbing. Every handler downstream of
// withRequestLogger() can call LogFromContext(ctx) to get a *slog.Logger
// that already has request_id stamped on it. One place owns the
// "request_id" attribute name + the way it's attached, so handlers
// never repeat `log.With("request_id", ...)` (Rule #1 — reusable).

type loggerCtxKey struct{}

// withRequestLogger is a chi-style middleware that attaches a
// request-scoped *slog.Logger (with request_id baked in) to the
// request's context.
//
// Wire AFTER middleware.RequestID so the id is populated; wire
// BEFORE auth + handlers so they can use it.
func withRequestLogger(base *slog.Logger) func(http.Handler) http.Handler {
	if base == nil {
		base = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := middleware.GetReqID(r.Context())
			scoped := base.With("request_id", rid)
			ctx := context.WithValue(r.Context(), loggerCtxKey{}, scoped)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LogFromContext returns the request-scoped logger. Falls back to
// slog.Default() if the middleware wasn't wired (test paths, /health
// before logger middleware runs, …) — never nil.
func LogFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerCtxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
