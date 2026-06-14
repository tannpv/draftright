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

	// Core Phase 0 endpoints. Health is public; Me is auth-gated.
	// Both nil-guarded: when Health is nil Build falls back to the
	// stub handleHealth (keeps router tests that don't set it green);
	// when Me is nil the /auth/me route is simply not mounted.
	Health http.Handler // GET /health
	Me     http.Handler // GET /auth/me (mounted inside the auth group)

	// Phase 1a auth endpoints. Public: Login, Refresh. Auth-gated:
	// ChangePassword, Account, DeleteAccount. All nil-guarded so the
	// router stays functional when the auth stack is absent (no DB).
	Login   http.Handler // POST /auth/login (public)
	Refresh http.Handler // POST /auth/refresh (public)

	// Phase 1b auth-lifecycle endpoints. All PUBLIC (unauthenticated):
	// signup + email-verification + password-reset flows. All nil-guarded
	// like Login/Refresh so the router stays functional without the auth
	// stack (no DB).
	Register           http.Handler // POST /auth/register (public)
	VerifyEmail        http.Handler // POST /auth/verify-email (public)
	ResendVerification http.Handler // POST /auth/resend-verification (public)
	ForgotPassword     http.Handler // POST /auth/forgot-password (public)
	ResetPassword      http.Handler // POST /auth/reset-password (public)
	Social             http.Handler // POST /auth/social (public)

	Plans http.Handler // GET /plans (public)

	ChangePassword http.Handler // POST /auth/change-password (auth)
	Account        http.Handler // GET /auth/account (auth)
	DeleteAccount  http.Handler // DELETE /auth/account (auth)

	Subscription  http.Handler // GET /subscription (auth)
	VerifyReceipt http.Handler // POST /subscription/verify-receipt (auth)

	MintExtToken   http.Handler // POST   /auth/extension-tokens          (auth)
	ListExtTokens  http.Handler // GET    /auth/extension-tokens          (auth)
	RevokeExtToken http.Handler // DELETE /auth/extension-tokens/{id}      (auth)

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
//     takes the whole process down.
//  4. withRequestLogger attaches a request-scoped slog (with request_id)
//     to the context for handlers to pick up.
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

	mux.Use(RequestID)
	mux.Use(middleware.RealIP)
	mux.Use(middleware.Recoverer)
	mux.Use(withRequestLogger(r.Log))
	mux.Use(structuredLogger(r.Log))

	// Prefer the wired core health handler (Node-parity body); fall
	// back to the stub when unset so router tests stay green.
	if r.Health != nil {
		mux.Method(http.MethodGet, "/health", r.Health)
	} else {
		mux.Get("/health", handleHealth)
	}

	if r.MetricsHandler != nil {
		// Don't run /metrics through structuredLogger / auth — Prom
		// scrapes would flood the log + auth would block them.
		mux.Method(http.MethodGet, "/metrics", r.MetricsHandler)
	}

	if r.Login != nil {
		mux.Method(http.MethodPost, "/auth/login", r.Login)
	}
	if r.Refresh != nil {
		mux.Method(http.MethodPost, "/auth/refresh", r.Refresh)
	}
	if r.Register != nil {
		mux.Method(http.MethodPost, "/auth/register", r.Register)
	}
	if r.VerifyEmail != nil {
		mux.Method(http.MethodPost, "/auth/verify-email", r.VerifyEmail)
	}
	if r.ResendVerification != nil {
		mux.Method(http.MethodPost, "/auth/resend-verification", r.ResendVerification)
	}
	if r.ForgotPassword != nil {
		mux.Method(http.MethodPost, "/auth/forgot-password", r.ForgotPassword)
	}
	if r.ResetPassword != nil {
		mux.Method(http.MethodPost, "/auth/reset-password", r.ResetPassword)
	}
	if r.Social != nil {
		mux.Method(http.MethodPost, "/auth/social", r.Social)
	}
	if r.Plans != nil {
		mux.Method(http.MethodGet, "/plans", r.Plans)
	}

	mux.Group(func(api chi.Router) {
		api.Use(RequireAuth(r.Verifier, r.Log))
		api.Post("/v1/rewrite", r.Rewrite.ServeHTTP)
		if r.Me != nil {
			api.Method(http.MethodGet, "/auth/me", r.Me)
		}
		if r.ChangePassword != nil {
			api.Method(http.MethodPost, "/auth/change-password", r.ChangePassword)
		}
		if r.Account != nil {
			api.Method(http.MethodGet, "/auth/account", r.Account)
		}
		if r.DeleteAccount != nil {
			api.Method(http.MethodDelete, "/auth/account", r.DeleteAccount)
		}
		if r.Subscription != nil {
			api.Method(http.MethodGet, "/subscription", r.Subscription)
		}
		if r.VerifyReceipt != nil {
			api.Method(http.MethodPost, "/subscription/verify-receipt", r.VerifyReceipt)
		}
		if r.MintExtToken != nil {
			api.Method(http.MethodPost, "/auth/extension-tokens", r.MintExtToken)
		}
		if r.ListExtTokens != nil {
			api.Method(http.MethodGet, "/auth/extension-tokens", r.ListExtTokens)
		}
		if r.RevokeExtToken != nil {
			api.Method(http.MethodDelete, "/auth/extension-tokens/{id}", r.RevokeExtToken)
		}
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
// remote, and the RequestID so we can grep by correlation id.
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
				"request_id", RequestIDFromContext(req.Context()),
			)
		})
	}
}
