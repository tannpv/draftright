package shared

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

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
	// Rewrite is the mounted handler for /v1/rewrite. Typed as
	// http.Handler so shared/ does not import the transport package
	// (which imports shared/ for ClaimsFromContext — avoiding a cycle).
	// Production passes *transport.RewriteHandler; tests may use any
	// http.Handler stub.
	Rewrite http.Handler

	// MetricsHandler, when non-nil, exposes /metrics. Production
	// passes the Prometheus handler; tests + dev can leave nil.
	MetricsHandler http.Handler

	// EnableTracing wraps the whole mux with otelhttp middleware so
	// every request becomes a span. No-op when the global tracer
	// provider is the default noop (i.e. tracing.Setup returned
	// without an endpoint).
	EnableTracing bool
}

// Build returns the wired http.Handler. Middleware order matters:
//
//  1. RequestID         every downstream log line gets a correlation id.
//  2. RealIP            puts the client IP into r.RemoteAddr behind a proxy.
//  3. Recoverer         catches panics; without it, a panic in any handler
//                       takes the whole process down.
//  4. withRequestLogger attaches a request-scoped slog (with request_id)
//                       to the context for handlers to pick up.
//  5. structuredLogger  one access-log line per request.
//  6. RequireAuth       scoped to authenticated routes only.
//
// Public routes (health, metrics) mount BEFORE the auth-gated
// subrouter so probes can hit them without a JWT.
func (r *Router) Build() http.Handler {
	if r.Log == nil {
		r.Log = slog.Default()
	}
	mux := chi.NewRouter()

	mux.Use(middleware.RequestID)
	mux.Use(middleware.RealIP)
	mux.Use(middleware.Recoverer)
	mux.Use(withRequestLogger(r.Log))
	mux.Use(structuredLogger(r.Log))

	mux.Get("/health", handleHealth)

	if r.MetricsHandler != nil {
		// Don't run /metrics through structuredLogger / auth — Prom
		// scrapes would flood the log + auth would block them.
		mux.Method(http.MethodGet, "/metrics", r.MetricsHandler)
	}

	mux.Group(func(api chi.Router) {
		api.Use(RequireAuth(r.Verifier, r.Log))
		api.Post("/v1/rewrite", r.Rewrite.ServeHTTP)
	})

	if r.EnableTracing {
		// otelhttp creates one span per request. Mounted at the
		// outermost layer so the span covers the full pipeline.
		return otelhttp.NewHandler(mux, "rewrite-go")
	}
	return mux
}

// handleHealth is the public probe target. Returns 200 unconditionally
// — Task 9 will extend with PG/Redis/provider checks.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{
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
