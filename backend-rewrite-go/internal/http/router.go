package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
)

// Router is the composition of the HTTP transport layer: chi mux +
// middleware chain + mounted handlers. Built once in main; serves as
// the http.Handler the *http.Server runs.
//
// Built via a struct + Build() rather than a free function so adding a
// new dependency (Prometheus collector, audit sink, …) is one new
// field, not a new positional argument in every call site.
type Router struct {
	Log      *slog.Logger
	Verifier *auth.Verifier
	Rewrite  *RewriteHandler
}

// Build returns the wired http.Handler. Middleware order matters:
//
//  1. RequestID  → every downstream log line gets a correlation id.
//  2. RealIP     → puts the client IP into r.RemoteAddr behind a proxy.
//  3. Recoverer  → catches panics; without it, a panic in any handler
//                  takes the whole process down.
//  4. structuredLogger → one access-log line per request.
//  5. RequireAuth → scoped to authenticated routes only.
//
// Public routes (health) mount BEFORE the auth-gated subrouter so a
// probe can hit them without a JWT.
func (r *Router) Build() http.Handler {
	mux := chi.NewRouter()

	mux.Use(middleware.RequestID)
	mux.Use(middleware.RealIP)
	mux.Use(middleware.Recoverer)
	mux.Use(structuredLogger(r.Log))

	mux.Get("/health", handleHealth)

	// Authenticated subrouter — RequireAuth applies once for all routes
	// mounted here, so future endpoints (/v1/history, /v1/usage, …)
	// inherit auth without re-listing it (Rule #1 — reusable).
	mux.Group(func(api chi.Router) {
		api.Use(RequireAuth(r.Verifier, r.Log))
		api.Post("/v1/rewrite", r.Rewrite.ServeHTTP)
	})

	return mux
}

// handleHealth is the public probe target. Returns 200 unconditionally
// — Task 9 will extend with PG/Redis/provider checks.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "rewrite-go",
	})
}

// structuredLogger is a chi-style middleware that emits one slog
// access-log line per request. Captures method, path, status, ms,
// remote, and the chi RequestID so we can grep by correlation id.
func structuredLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, req.ProtoMajor)
			next.ServeHTTP(ww, req)
			log.Info("http",
				"method", req.Method,
				"path", req.URL.Path,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
				"bytes", ww.BytesWritten(),
				"remote", req.RemoteAddr,
				"request_id", middleware.GetReqID(req.Context()),
			)
		})
	}
}

