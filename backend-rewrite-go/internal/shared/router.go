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

	// Parity rewrite endpoints (Node /rewrite controller; SEPARATE from the
	// Go-only streaming /v1/rewrite above). Tones + Trial are PUBLIC; RewriteParity
	// uses the SAME dual-auth (ExtVerifier OR JWT) as /v1/rewrite. All nil-guarded.
	Tones         http.Handler // GET  /rewrite/tones (public)
	RewriteParity http.Handler // POST /rewrite       (dual-auth: ext token OR JWT)
	RewriteTrial  http.Handler // POST /rewrite/trial (public)

	// ExtVerifier, when non-nil, enables dual auth on /v1/rewrite: a
	// bearer token prefixed dr_ext_ is verified as an extension token
	// (mirroring Node's RewriteAuthGuard) instead of a JWT. nil-guarded:
	// when absent (e.g. no DB → no exttoken service) /v1/rewrite stays
	// JWT-only, byte-identical to before this field existed. main.go must
	// leave this a nil INTERFACE (not a typed-nil *exttoken.Service) when
	// there is no service, or ExtOrJWT would call Verify on a nil pointer.
	ExtVerifier ExtVerifier

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

	PaymentMethods http.Handler // GET /payment/methods       (public)
	PaymentStatus  http.Handler // GET /payment/status/{ref}   (public)
	PaymentHistory http.Handler // GET /payment/history        (auth)

	PaymentWebhookStripe       http.Handler // POST /payment/webhook/stripe       (public)
	PaymentWebhookVietQR       http.Handler // POST /payment/webhook/vietqr       (public)
	PaymentWebhookCasso        http.Handler // POST /payment/webhook/casso        (public)
	PaymentWebhookSepay        http.Handler // POST /payment/webhook/sepay        (public)
	PaymentWebhookLemonSqueezy http.Handler // POST /payment/webhook/lemonsqueezy (public)

	// Phase 4a ancillary endpoints. All PUBLIC (mounted before the auth
	// group). Errors accepts an optional best-effort JWT (the handler reads
	// the claim itself; the route is not RequireAuth-gated). All nil-guarded.
	ImePacksManifest http.Handler // GET  /ime-packs/manifest (public)
	UpdatesLatest    http.Handler // GET  /updates/latest     (public)
	ErrorsIngest     http.Handler // POST /errors             (public, optional JWT)

	PaymentCheckout  http.Handler // POST /payment/checkout       (auth)
	PaymentPortal    http.Handler // GET /payment/portal          (auth)
	PaymentCancelSub http.Handler // DELETE /payment/subscription (auth)

	// Phase 4b LLM-ingest endpoints. All nil-guarded like Phase 4a.
	//
	//   ExtractHandler — POST /extract is the ONLY auth-gated one (Node's
	//     @UseGuards(JwtAuthGuard)); mounted INSIDE the RequireAuth group.
	//   EmailWebhook   — POST /webhooks/resend is PUBLIC and reads the RAW
	//     request body for Svix/HMAC verification. Mounted at the top level
	//     via mux.Method so NO body-consuming middleware runs on it (the
	//     global chain is RequestID/RealIP/Recoverer/logger — none buffer or
	//     rewrite the body), keeping the bytes intact for the signature.
	//   BugReportIngest — POST /bug-reports is PUBLIC (multipart form).
	//   FeedbackCreate  — POST /feedback        is PUBLIC.
	//   FeedbackList    — GET  /feedback        is PUBLIC.
	//   FeedbackVote    — POST /feedback/{id}/vote is PUBLIC at the route
	//     level; the handler enforces its own JWT (Node: public route,
	//     UnauthorizedException thrown inside when no user).
	ExtractHandler  http.Handler // POST /extract           (auth)
	EmailWebhook    http.Handler // POST /webhooks/resend    (public, raw body)
	BugReportIngest http.Handler // POST /bug-reports        (public, multipart)
	FeedbackCreate  http.Handler // POST /feedback           (public)
	FeedbackList    http.Handler // GET  /feedback           (public)
	FeedbackVote    http.Handler // POST /feedback/{id}/vote (public; handler enforces JWT)

	// Phase 4c-1 admin foundation. AdminLogin is PUBLIC (mounted before the
	// auth group). AdminChangePassword + AdminMe sit behind RequireAuth THEN
	// RequireAdmin (the RolesGuard('admin') equivalent). All nil-guarded.
	AdminLogin          http.Handler // POST   /admin/auth/login           (public)
	AdminChangePassword http.Handler // POST   /admin/auth/change-password (admin)
	AdminMe             http.Handler // GET    /admin/auth/me              (admin)

	// Phase 4c-2 admin content/ops CRUD. All http.Handler, mounted in Task 21. nil-guarded.
	AiProvidersList      http.Handler // GET    /admin/ai-providers           (admin)
	AiProvidersPaginated http.Handler // GET    /admin/ai-providers/paginated (admin)
	AiProviderCreate     http.Handler // POST   /admin/ai-providers           (admin)
	AiProviderUpdate     http.Handler // PATCH  /admin/ai-providers/{id}      (admin)
	AiProviderDelete     http.Handler // DELETE /admin/ai-providers/{id}      (admin)
	AiProviderTest       http.Handler // POST   /admin/ai-providers/{id}/test (admin)

	AppSettingsGet       http.Handler // GET   /admin/settings            (admin)
	AppSettingsPatch     http.Handler // PATCH /admin/settings            (admin)
	AppSettingsTestEmail http.Handler // POST  /admin/settings/test-email (admin)

	AdminPlansList  http.Handler // GET    /admin/plans      (admin)
	AdminPlanCreate http.Handler // POST   /admin/plans      (admin)
	AdminPlanUpdate http.Handler // PATCH  /admin/plans/{id} (admin)
	AdminPlanDelete http.Handler // DELETE /admin/plans/{id} (admin)

	AdminUsersList  http.Handler // GET   /admin/users      (admin)
	AdminUserGet    http.Handler // GET   /admin/users/{id} (admin)
	AdminUserUpdate http.Handler // PATCH /admin/users/{id} (admin)

	AdminAccountsList  http.Handler // GET    /admin/admin-users      (admin)
	AdminAccountCreate http.Handler // POST   /admin/admin-users      (admin)
	AdminAccountUpdate http.Handler // PATCH  /admin/admin-users/{id} (admin)
	AdminAccountDelete http.Handler // DELETE /admin/admin-users/{id} (admin)

	AdminEmailLogs http.Handler // GET /admin/email-logs (admin)

	AdminEmailTemplatesList   http.Handler // GET    /admin/email-templates               (admin)
	AdminEmailTemplateUpdate  http.Handler // PATCH  /admin/email-templates/{key}         (admin)
	AdminEmailTemplateReset   http.Handler // DELETE /admin/email-templates/{key}         (admin)
	AdminEmailTemplatePreview http.Handler // GET    /admin/email-templates/{key}/preview (admin)

	// Phase 4c-3 admin reporting. All http.Handler, nil-guarded, mounted in
	// the admin group (jwtMW → RequireAdmin) like the 4c-2 routes.
	AdminStats        http.Handler // GET  /admin/stats
	AdminAnalytics    http.Handler // GET  /admin/analytics
	AdminTransactions http.Handler // GET  /admin/transactions

	TrainingDataStats  http.Handler // GET   /admin/training-data/stats
	TrainingDataList   http.Handler // GET   /admin/training-data
	TrainingDataReview http.Handler // PATCH /admin/training-data/{id}
	TrainingDataExport http.Handler // GET   /admin/training-data/export

	AdminPaymentsStats  http.Handler // GET  /admin/payments/stats
	AdminPaymentsList   http.Handler // GET  /admin/payments
	AdminPaymentConfirm http.Handler // POST /admin/payments/{id}/confirm
	AdminPaymentRefund  http.Handler // POST /admin/payments/{id}/refund

	// ── Phase 4c-4 admin triage (19 routes) ──────────────────────────────
	AdminErrorsList     http.Handler // GET    /admin/errors
	AdminErrorGet       http.Handler // GET    /admin/errors/{id}
	AdminErrorPatch     http.Handler // PATCH  /admin/errors/{id}
	AdminErrorDelete    http.Handler // DELETE /admin/errors/{id}
	AdminErrorSuggest   http.Handler // POST   /admin/errors/{id}/suggest-fix
	AdminErrorsRunCron  http.Handler // POST   /admin/errors/run-ai-cron
	AdminBugList        http.Handler // GET    /admin/bug-reports
	AdminBugGet         http.Handler // GET    /admin/bug-reports/{id}
	AdminBugScreenshot  http.Handler // GET    /admin/bug-reports/{id}/screenshot
	AdminBugPatch       http.Handler // PATCH  /admin/bug-reports/{id}
	AdminBugDelete      http.Handler // DELETE /admin/bug-reports/{id}
	AdminBugFixProposal http.Handler // POST   /admin/bug-reports/{id}/fix-proposal
	AdminInboxCounts    http.Handler // GET    /admin/inbox/counts
	AdminInbox          http.Handler // GET    /admin/inbox
	AdminReleasesList   http.Handler // GET    /admin/releases
	AdminReleaseUpsert  http.Handler // POST   /admin/releases
	AdminReleaseDelete  http.Handler // DELETE /admin/releases/{platform}/{channel}
	AdminPolicyUpsert   http.Handler // POST   /admin/release-policies
	AdminGrantSub       http.Handler // POST   /admin/subscriptions/grant

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

	mux.Use(CORS)
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
	// /rewrite/tones — PUBLIC static catalog (Node @Get('tones') has no guard).
	if r.Tones != nil {
		mux.Method(http.MethodGet, "/rewrite/tones", r.Tones)
	}
	if r.PaymentMethods != nil {
		mux.Method(http.MethodGet, "/payment/methods", r.PaymentMethods)
	}
	if r.PaymentStatus != nil {
		mux.Method(http.MethodGet, "/payment/status/{ref}", r.PaymentStatus)
	}
	if r.PaymentWebhookStripe != nil {
		mux.Method(http.MethodPost, "/payment/webhook/stripe", r.PaymentWebhookStripe)
	}
	if r.PaymentWebhookVietQR != nil {
		mux.Method(http.MethodPost, "/payment/webhook/vietqr", r.PaymentWebhookVietQR)
	}
	if r.PaymentWebhookCasso != nil {
		mux.Method(http.MethodPost, "/payment/webhook/casso", r.PaymentWebhookCasso)
	}
	if r.PaymentWebhookSepay != nil {
		mux.Method(http.MethodPost, "/payment/webhook/sepay", r.PaymentWebhookSepay)
	}
	if r.PaymentWebhookLemonSqueezy != nil {
		mux.Method(http.MethodPost, "/payment/webhook/lemonsqueezy", r.PaymentWebhookLemonSqueezy)
	}

	// Phase 4a ancillary public endpoints — mounted BEFORE the auth group
	// so they're reachable without a JWT (parity with Node: all three are
	// public; /errors reads an optional best-effort JWT inside the handler).
	if r.ImePacksManifest != nil {
		mux.Method(http.MethodGet, "/ime-packs/manifest", r.ImePacksManifest)
	}
	if r.UpdatesLatest != nil {
		mux.Method(http.MethodGet, "/updates/latest", r.UpdatesLatest)
	}
	if r.ErrorsIngest != nil {
		mux.Method(http.MethodPost, "/errors", r.ErrorsIngest)
	}
	// /rewrite/trial — PUBLIC (Node @Post('trial') has no guard).
	if r.RewriteTrial != nil {
		mux.Method(http.MethodPost, "/rewrite/trial", r.RewriteTrial)
	}

	// Phase 4b public endpoints — mounted BEFORE the auth group (no JWT).
	// /webhooks/resend reads the RAW body for signature verification, so it
	// MUST mount here at the top level (the global chain never buffers the
	// body) and NEVER inside a JSON-parsing/body-rewriting middleware.
	if r.EmailWebhook != nil {
		mux.Method(http.MethodPost, "/webhooks/resend", r.EmailWebhook)
	}
	if r.BugReportIngest != nil {
		mux.Method(http.MethodPost, "/bug-reports", r.BugReportIngest)
	}
	if r.FeedbackCreate != nil {
		mux.Method(http.MethodPost, "/feedback", r.FeedbackCreate)
	}
	if r.FeedbackList != nil {
		mux.Method(http.MethodGet, "/feedback", r.FeedbackList)
	}
	if r.FeedbackVote != nil {
		// Public route; the handler enforces JWT itself (Node parity).
		mux.Method(http.MethodPost, "/feedback/{id}/vote", r.FeedbackVote)
	}

	// Phase 4c-1 public admin-auth endpoint (no JWT required).
	if r.AdminLogin != nil {
		mux.Method(http.MethodPost, "/admin/auth/login", r.AdminLogin)
	}

	// Build the JWT middleware once so /v1/rewrite (dual-auth) and the
	// JWT-only group share the exact same RequireAuth instance.
	jwtMW := RequireAuth(r.Verifier, r.Log)

	// /v1/rewrite mounts OUTSIDE the JWT group: it accepts a dr_ext_
	// extension token OR a JWT (Node's RewriteAuthGuard). Mounted at the
	// top level via mux.Method so it still runs through the global
	// middleware chain (RequestID/RealIP/Recoverer/logger), which are
	// router-level. With an ExtVerifier it gets dual auth; without one it
	// stays JWT-only — byte-identical to before this task.
	if r.Rewrite != nil {
		if r.ExtVerifier != nil {
			mux.Method(http.MethodPost, "/v1/rewrite", ExtOrJWT(r.ExtVerifier, jwtMW, r.Log)(r.Rewrite))
		} else {
			mux.Method(http.MethodPost, "/v1/rewrite", jwtMW(r.Rewrite))
		}
	}

	// POST /rewrite — Node /rewrite controller's authed endpoint. SAME dual-auth
	// as /v1/rewrite (dr_ext_ token OR JWT). chi treats /rewrite, /rewrite/trial,
	// /rewrite/tones as distinct paths — no conflict.
	if r.RewriteParity != nil {
		if r.ExtVerifier != nil {
			mux.Method(http.MethodPost, "/rewrite", ExtOrJWT(r.ExtVerifier, jwtMW, r.Log)(r.RewriteParity))
		} else {
			mux.Method(http.MethodPost, "/rewrite", jwtMW(r.RewriteParity))
		}
	}

	mux.Group(func(api chi.Router) {
		api.Use(jwtMW)
		if r.Me != nil {
			api.Method(http.MethodGet, "/auth/me", r.Me)
		}
		if r.ExtractHandler != nil {
			// Node: @Controller('extract') @UseGuards(JwtAuthGuard) @Post().
			api.Method(http.MethodPost, "/extract", r.ExtractHandler)
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
		if r.PaymentHistory != nil {
			api.Method(http.MethodGet, "/payment/history", r.PaymentHistory)
		}
		if r.PaymentCheckout != nil {
			api.Method(http.MethodPost, "/payment/checkout", r.PaymentCheckout)
		}
		if r.PaymentPortal != nil {
			api.Method(http.MethodGet, "/payment/portal", r.PaymentPortal)
		}
		if r.PaymentCancelSub != nil {
			api.Method(http.MethodDelete, "/payment/subscription", r.PaymentCancelSub)
		}
	})

	// Admin group: RequireAuth (401 on no/invalid token) THEN RequireAdmin
	// (403 'Admin access required' for non-admin tokens).
	mux.Group(func(admin chi.Router) {
		admin.Use(jwtMW)
		admin.Use(RequireAdmin)
		if r.AdminChangePassword != nil {
			admin.Method(http.MethodPost, "/admin/auth/change-password", r.AdminChangePassword)
		}
		if r.AdminMe != nil {
			admin.Method(http.MethodGet, "/admin/auth/me", r.AdminMe)
		}

		// Phase 4c-2 admin content/ops CRUD (Task 21). All nil-guarded,
		// behind the same RequireAuth → RequireAdmin chain.
		if r.AiProvidersList != nil {
			admin.Method(http.MethodGet, "/admin/ai-providers", r.AiProvidersList)
		}
		if r.AiProvidersPaginated != nil {
			admin.Method(http.MethodGet, "/admin/ai-providers/paginated", r.AiProvidersPaginated)
		}
		if r.AiProviderCreate != nil {
			admin.Method(http.MethodPost, "/admin/ai-providers", r.AiProviderCreate)
		}
		if r.AiProviderUpdate != nil {
			admin.Method(http.MethodPatch, "/admin/ai-providers/{id}", r.AiProviderUpdate)
		}
		if r.AiProviderDelete != nil {
			admin.Method(http.MethodDelete, "/admin/ai-providers/{id}", r.AiProviderDelete)
		}
		if r.AiProviderTest != nil {
			admin.Method(http.MethodPost, "/admin/ai-providers/{id}/test", r.AiProviderTest)
		}

		if r.AppSettingsGet != nil {
			admin.Method(http.MethodGet, "/admin/settings", r.AppSettingsGet)
		}
		if r.AppSettingsPatch != nil {
			admin.Method(http.MethodPatch, "/admin/settings", r.AppSettingsPatch)
		}
		if r.AppSettingsTestEmail != nil {
			admin.Method(http.MethodPost, "/admin/settings/test-email", r.AppSettingsTestEmail)
		}

		if r.AdminPlansList != nil {
			admin.Method(http.MethodGet, "/admin/plans", r.AdminPlansList)
		}
		if r.AdminPlanCreate != nil {
			admin.Method(http.MethodPost, "/admin/plans", r.AdminPlanCreate)
		}
		if r.AdminPlanUpdate != nil {
			admin.Method(http.MethodPatch, "/admin/plans/{id}", r.AdminPlanUpdate)
		}
		if r.AdminPlanDelete != nil {
			admin.Method(http.MethodDelete, "/admin/plans/{id}", r.AdminPlanDelete)
		}

		if r.AdminUsersList != nil {
			admin.Method(http.MethodGet, "/admin/users", r.AdminUsersList)
		}
		if r.AdminUserGet != nil {
			admin.Method(http.MethodGet, "/admin/users/{id}", r.AdminUserGet)
		}
		if r.AdminUserUpdate != nil {
			admin.Method(http.MethodPatch, "/admin/users/{id}", r.AdminUserUpdate)
		}

		if r.AdminAccountsList != nil {
			admin.Method(http.MethodGet, "/admin/admin-users", r.AdminAccountsList)
		}
		if r.AdminAccountCreate != nil {
			admin.Method(http.MethodPost, "/admin/admin-users", r.AdminAccountCreate)
		}
		if r.AdminAccountUpdate != nil {
			admin.Method(http.MethodPatch, "/admin/admin-users/{id}", r.AdminAccountUpdate)
		}
		if r.AdminAccountDelete != nil {
			admin.Method(http.MethodDelete, "/admin/admin-users/{id}", r.AdminAccountDelete)
		}

		if r.AdminEmailLogs != nil {
			admin.Method(http.MethodGet, "/admin/email-logs", r.AdminEmailLogs)
		}

		if r.AdminEmailTemplatesList != nil {
			admin.Method(http.MethodGet, "/admin/email-templates", r.AdminEmailTemplatesList)
		}
		if r.AdminEmailTemplateUpdate != nil {
			admin.Method(http.MethodPatch, "/admin/email-templates/{key}", r.AdminEmailTemplateUpdate)
		}
		if r.AdminEmailTemplateReset != nil {
			admin.Method(http.MethodDelete, "/admin/email-templates/{key}", r.AdminEmailTemplateReset)
		}
		if r.AdminEmailTemplatePreview != nil {
			admin.Method(http.MethodGet, "/admin/email-templates/{key}/preview", r.AdminEmailTemplatePreview)
		}

		// Phase 4c-3 admin reporting routes (Task 19). nil-guarded, same
		// RequireAuth → RequireAdmin chain. chi prioritizes static segments
		// over {id} wildcards, so /admin/training-data/stats and
		// /admin/training-data/export resolve before /admin/training-data/{id}
		// regardless of registration order.
		if r.AdminStats != nil {
			admin.Method(http.MethodGet, "/admin/stats", r.AdminStats)
		}
		if r.AdminAnalytics != nil {
			admin.Method(http.MethodGet, "/admin/analytics", r.AdminAnalytics)
		}
		if r.AdminTransactions != nil {
			admin.Method(http.MethodGet, "/admin/transactions", r.AdminTransactions)
		}

		if r.TrainingDataStats != nil {
			admin.Method(http.MethodGet, "/admin/training-data/stats", r.TrainingDataStats)
		}
		if r.TrainingDataExport != nil {
			admin.Method(http.MethodGet, "/admin/training-data/export", r.TrainingDataExport)
		}
		if r.TrainingDataList != nil {
			admin.Method(http.MethodGet, "/admin/training-data", r.TrainingDataList)
		}
		if r.TrainingDataReview != nil {
			admin.Method(http.MethodPatch, "/admin/training-data/{id}", r.TrainingDataReview)
		}

		if r.AdminPaymentsStats != nil {
			admin.Method(http.MethodGet, "/admin/payments/stats", r.AdminPaymentsStats)
		}
		if r.AdminPaymentsList != nil {
			admin.Method(http.MethodGet, "/admin/payments", r.AdminPaymentsList)
		}
		if r.AdminPaymentConfirm != nil {
			admin.Method(http.MethodPost, "/admin/payments/{id}/confirm", r.AdminPaymentConfirm)
		}
		if r.AdminPaymentRefund != nil {
			admin.Method(http.MethodPost, "/admin/payments/{id}/refund", r.AdminPaymentRefund)
		}

		// Phase 4c-4 admin triage routes (Task H1). nil-guarded, same
		// RequireAuth → RequireAdmin chain. Static segments registered
		// before {id} wildcards (chi prioritizes static, but explicit):
		// run-ai-cron before /errors/{id}; /inbox/counts before /inbox.
		// errors — static run-ai-cron before {id} wildcards.
		if r.AdminErrorsRunCron != nil {
			admin.Method(http.MethodPost, "/admin/errors/run-ai-cron", r.AdminErrorsRunCron)
		}
		if r.AdminErrorsList != nil {
			admin.Method(http.MethodGet, "/admin/errors", r.AdminErrorsList)
		}
		if r.AdminErrorGet != nil {
			admin.Method(http.MethodGet, "/admin/errors/{id}", r.AdminErrorGet)
		}
		if r.AdminErrorPatch != nil {
			admin.Method(http.MethodPatch, "/admin/errors/{id}", r.AdminErrorPatch)
		}
		if r.AdminErrorDelete != nil {
			admin.Method(http.MethodDelete, "/admin/errors/{id}", r.AdminErrorDelete)
		}
		if r.AdminErrorSuggest != nil {
			admin.Method(http.MethodPost, "/admin/errors/{id}/suggest-fix", r.AdminErrorSuggest)
		}
		// bug-reports
		if r.AdminBugList != nil {
			admin.Method(http.MethodGet, "/admin/bug-reports", r.AdminBugList)
		}
		if r.AdminBugGet != nil {
			admin.Method(http.MethodGet, "/admin/bug-reports/{id}", r.AdminBugGet)
		}
		if r.AdminBugScreenshot != nil {
			admin.Method(http.MethodGet, "/admin/bug-reports/{id}/screenshot", r.AdminBugScreenshot)
		}
		if r.AdminBugPatch != nil {
			admin.Method(http.MethodPatch, "/admin/bug-reports/{id}", r.AdminBugPatch)
		}
		if r.AdminBugDelete != nil {
			admin.Method(http.MethodDelete, "/admin/bug-reports/{id}", r.AdminBugDelete)
		}
		if r.AdminBugFixProposal != nil {
			admin.Method(http.MethodPost, "/admin/bug-reports/{id}/fix-proposal", r.AdminBugFixProposal)
		}
		// inbox — static counts before /admin/inbox
		if r.AdminInboxCounts != nil {
			admin.Method(http.MethodGet, "/admin/inbox/counts", r.AdminInboxCounts)
		}
		if r.AdminInbox != nil {
			admin.Method(http.MethodGet, "/admin/inbox", r.AdminInbox)
		}
		// releases
		if r.AdminReleasesList != nil {
			admin.Method(http.MethodGet, "/admin/releases", r.AdminReleasesList)
		}
		if r.AdminReleaseUpsert != nil {
			admin.Method(http.MethodPost, "/admin/releases", r.AdminReleaseUpsert)
		}
		if r.AdminReleaseDelete != nil {
			admin.Method(http.MethodDelete, "/admin/releases/{platform}/{channel}", r.AdminReleaseDelete)
		}
		if r.AdminPolicyUpsert != nil {
			admin.Method(http.MethodPost, "/admin/release-policies", r.AdminPolicyUpsert)
		}
		// grant
		if r.AdminGrantSub != nil {
			admin.Method(http.MethodPost, "/admin/subscriptions/grant", r.AdminGrantSub)
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
