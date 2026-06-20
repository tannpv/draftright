// Package main is the composition root for the DraftRight /rewrite Go
// microservice. Per clean architecture this is the ONLY place where
// concrete adapters get wired into use cases. Every other package
// stays free of "if env == prod / if env == dev" branching.
//
// Task 7 state: chi router + SSE handler wired. Adapters fall back to
// in-memory implementations when DATABASE_URL / REDIS_URL / OPENAI_API_KEY
// are empty — gives a zero-config dev loop while preserving production
// fidelity once the env is populated.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tannpv/draftright-rewrite/internal/adapter/redislimit"
	redistrial "github.com/tannpv/draftright-rewrite/internal/adapter/redistrial"
	adminauth "github.com/tannpv/draftright-rewrite/internal/adminauth"
	adminstatspkg "github.com/tannpv/draftright-rewrite/internal/adminstats"
	"github.com/tannpv/draftright-rewrite/internal/aicall"
	aiproviderpkg "github.com/tannpv/draftright-rewrite/internal/aiprovider"
	appsettingspkg "github.com/tannpv/draftright-rewrite/internal/appsettings"
	authpkg "github.com/tannpv/draftright-rewrite/internal/auth"
	bugreportspkg "github.com/tannpv/draftright-rewrite/internal/bugreports"
	corepkg "github.com/tannpv/draftright-rewrite/internal/core"
	emailpkg "github.com/tannpv/draftright-rewrite/internal/email"
	errreportpkg "github.com/tannpv/draftright-rewrite/internal/errreport"
	extractionpkg "github.com/tannpv/draftright-rewrite/internal/extraction"
	exttokenpkg "github.com/tannpv/draftright-rewrite/internal/exttoken"
	feedbackpkg "github.com/tannpv/draftright-rewrite/internal/feedback"
	imepackspkg "github.com/tannpv/draftright-rewrite/internal/imepacks"
	inboxpkg "github.com/tannpv/draftright-rewrite/internal/inbox"
	paymentpkg "github.com/tannpv/draftright-rewrite/internal/payment"
	paymentstrategy "github.com/tannpv/draftright-rewrite/internal/payment/strategy"
	"github.com/tannpv/draftright-rewrite/internal/payment/strategy/lemonsqueezy"
	"github.com/tannpv/draftright-rewrite/internal/payment/strategy/stripe"
	"github.com/tannpv/draftright-rewrite/internal/payment/strategy/vietqr"
	planspkg "github.com/tannpv/draftright-rewrite/internal/plans"
	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/platform/config"
	platformdb "github.com/tannpv/draftright-rewrite/internal/platform/db"
	"github.com/tannpv/draftright-rewrite/internal/platform/metrics"
	"github.com/tannpv/draftright-rewrite/internal/platform/scheduler"
	settingspkg "github.com/tannpv/draftright-rewrite/internal/platform/settings"
	"github.com/tannpv/draftright-rewrite/internal/platform/tracing"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/anthropic"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/chain"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/memory"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/ollama"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/openai"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/pg"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
	parity "github.com/tannpv/draftright-rewrite/internal/rewrite/parity"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/transport"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/usecase"
	rewritelogpkg "github.com/tannpv/draftright-rewrite/internal/rewritelog"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	subpkg "github.com/tannpv/draftright-rewrite/internal/subscription"
	updatespkg "github.com/tannpv/draftright-rewrite/internal/updates"
	usagepkg "github.com/tannpv/draftright-rewrite/internal/usage"
	userpkg "github.com/tannpv/draftright-rewrite/internal/user"
)

const (
	// Read deadline is short — request bodies are small JSON. Write
	// deadline is generous to accommodate SSE streams whose total
	// duration depends on the upstream provider (model tokens/sec).
	readTimeout  = 10 * time.Second
	writeTimeout = 5 * time.Minute
	idleTimeout  = 120 * time.Second

	// Graceful shutdown — long enough for in-flight SSE streams to
	// finish a final token, short enough that prod redeploys don't
	// drag.
	shutdownTimeout = 30 * time.Second
)

func main() {
	// Distroless self-probe: `/server -healthcheck` GETs the local /health
	// endpoint and exits 0/1, so the shell-less image can still report
	// container health without wget. Must run before config.Load so a probe
	// works even with a minimal env. See issue #28.
	if len(os.Args) > 1 && os.Args[1] == healthCheckArg {
		os.Exit(healthProbe(os.Getenv("LISTEN_ADDR")))
	}

	cfg, err := config.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("FATAL: " + err.Error() + "\n")
		os.Exit(2)
	}

	log := newLogger(cfg.LogLevel)
	log.Info("boot", "app_env", cfg.AppEnv, "listen", cfg.Listen)

	// Tracing first: install global tracer provider so any spans
	// started during dep wiring get reported. Noop when endpoint
	// empty.
	shutdownTracer, err := tracing.Setup(context.Background(), tracing.Config{
		Endpoint:    cfg.OtelEndpoint,
		ServiceName: "rewrite-go",
		SampleRatio: cfg.OtelSampleRatio,
	})
	if err != nil {
		log.Error("tracing setup failed", "err", err.Error())
		os.Exit(1)
	}
	if cfg.OtelEndpoint != "" {
		log.Info("observability", "tracing", "otlp-http", "endpoint", cfg.OtelEndpoint)
	}

	// Metrics: build the Prometheus sink (or noop) BEFORE composeDeps
	// so the use case picks it up via RewriteDeps.Metrics.
	var (
		metricsSink domain.Metrics
		metricsHTTP http.Handler
	)
	if cfg.MetricsEnabled {
		prom := metrics.NewPrometheus()
		metricsSink = prom
		metricsHTTP = prom.Handler()
		log.Info("observability", "metrics", "prometheus", "path", "/metrics")
	} else {
		metricsSink = metrics.NewNoop()
		log.Info("observability", "metrics", "noop (METRICS_ENABLED unset)")
	}

	deps, core, cleanup, err := composeDeps(context.Background(), cfg, log, metricsSink)
	if err != nil {
		log.Error("dependency wiring failed", "err", err.Error())
		os.Exit(1)
	}
	defer cleanup()

	rt := &shared.Router{
		Log:            log,
		Verifier:       core.accessVerifier, // single shared verifier (also used by errreport)
		MetricsHandler: metricsHTTP,
		EnableTracing:  cfg.OtelEndpoint != "",
		Health:         core.health,
		Me:             core.me,
		Rewrite: &transport.RewriteHandler{
			Deps: deps,
			Log:  log,
		},
		Login:              core.login,
		Refresh:            core.refresh,
		Register:           core.register,
		VerifyEmail:        core.verifyEmail,
		ResendVerification: core.resendVerification,
		ForgotPassword:     core.forgotPassword,
		ResetPassword:      core.resetPassword,
		Social:             core.social,
		Plans:              core.plans,
		ChangePassword:     core.changePassword,
		Account:            core.account,
		DeleteAccount:      core.deleteAccount,
		Subscription:       core.subscription,
		VerifyReceipt:      core.verifyReceipt,
		PaymentMethods:     core.paymentMethods,
		PaymentStatus:      core.paymentStatus,
		PaymentHistory:     core.paymentHistory,
		PaymentCheckout:    core.paymentCheckout,
		PaymentPortal:      core.paymentPortal,
		PaymentCancelSub:   core.paymentCancelSub,

		PaymentWebhookStripe:       core.paymentWebhookStripe,
		PaymentWebhookVietQR:       core.paymentWebhookVietQR,
		PaymentWebhookCasso:        core.paymentWebhookCasso,
		PaymentWebhookSepay:        core.paymentWebhookSepay,
		PaymentWebhookLemonSqueezy: core.paymentWebhookLemonSqueezy,
		MintExtToken:               core.mintExtToken,
		ListExtTokens:              core.listExtTokens,
		RevokeExtToken:             core.revokeExtToken,

		ImePacksManifest: core.imePacksManifest,
		UpdatesLatest:    core.updatesLatest,
		ErrorsIngest:     core.errorsIngest,

		Tones:         core.tones,
		RewriteParity: core.rewriteParity,
		RewriteTrial:  core.rewriteTrial,

		ExtractHandler:  core.extract,
		EmailWebhook:    core.emailWebhook,
		BugReportIngest: core.bugReportIngest,
		FeedbackCreate:  core.feedbackCreate,
		FeedbackList:    core.feedbackList,
		FeedbackVote:    core.feedbackVote,

		AdminLogin:          core.adminLogin,
		AdminChangePassword: core.adminChangePassword,
		AdminMe:             core.adminMe,

		AiProvidersList:      core.aiProvidersList,
		AiProvidersPaginated: core.aiProvidersPaginated,
		AiProviderCreate:     core.aiProviderCreate,
		AiProviderUpdate:     core.aiProviderUpdate,
		AiProviderDelete:     core.aiProviderDelete,
		AiProviderTest:       core.aiProviderTest,

		AppSettingsGet:       core.appSettingsGet,
		AppSettingsPatch:     core.appSettingsPatch,
		AppSettingsTestEmail: core.appSettingsTestEmail,

		AdminPlansList:  core.adminPlansList,
		AdminPlanCreate: core.adminPlanCreate,
		AdminPlanUpdate: core.adminPlanUpdate,
		AdminPlanDelete: core.adminPlanDelete,

		AdminUsersList:  core.adminUsersList,
		AdminUserGet:    core.adminUserGet,
		AdminUserUpdate: core.adminUserUpdate,

		AdminAccountsList:  core.adminAccountsList,
		AdminAccountCreate: core.adminAccountCreate,
		AdminAccountUpdate: core.adminAccountUpdate,
		AdminAccountDelete: core.adminAccountDelete,

		AdminEmailLogs: core.adminEmailLogs,

		AdminEmailTemplatesList:   core.adminEmailTemplatesList,
		AdminEmailTemplateUpdate:  core.adminEmailTemplateUpdate,
		AdminEmailTemplateReset:   core.adminEmailTemplateReset,
		AdminEmailTemplatePreview: core.adminEmailTemplatePreview,

		AdminStats:        core.adminStats,
		AdminAnalytics:    core.adminAnalytics,
		AdminTransactions: core.adminTransactions,

		TrainingDataStats:  core.trainingDataStats,
		TrainingDataList:   core.trainingDataList,
		TrainingDataReview: core.trainingDataReview,
		TrainingDataExport: core.trainingDataExport,

		AdminPaymentsStats:  core.adminPaymentsStats,
		AdminPaymentsList:   core.adminPaymentsList,
		AdminPaymentConfirm: core.adminPaymentConfirm,
		AdminPaymentRefund:  core.adminPaymentRefund,

		AdminErrorsList:    core.adminErrorsList,
		AdminErrorGet:      core.adminErrorGet,
		AdminErrorPatch:    core.adminErrorPatch,
		AdminErrorDelete:   core.adminErrorDelete,
		AdminErrorSuggest:  core.adminErrorSuggest,
		AdminErrorsRunCron: core.adminErrorsRunCron,

		AdminBugList:        core.adminBugList,
		AdminBugGet:         core.adminBugGet,
		AdminBugScreenshot:  core.adminBugScreenshot,
		AdminBugPatch:       core.adminBugPatch,
		AdminBugDelete:      core.adminBugDelete,
		AdminBugFixProposal: core.adminBugFixProposal,

		AdminInboxCounts: core.adminInboxCounts,
		AdminInbox:       core.adminInbox,

		AdminReleasesList:  core.adminReleasesList,
		AdminReleaseUpsert: core.adminReleaseUpsert,
		AdminReleaseDelete: core.adminReleaseDelete,
		AdminPolicyUpsert:  core.adminPolicyUpsert,

		AdminGrantSub: core.adminGrantSub,
	}
	// Enable dual auth on /v1/rewrite (dr_ext_ token OR JWT) only when the
	// extension-token service is wired (DB present). Guarded so the field
	// stays a nil INTERFACE when there's no service — a typed-nil
	// *exttoken.Service in the interface would be non-nil and panic on Verify.
	if core.extSvc != nil {
		rt.ExtVerifier = core.extSvc
	}
	router := rt.Build()

	srv := &http.Server{
		Addr:         cfg.Listen,
		Handler:      router,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	go func() {
		log.Info("listening", "addr", cfg.Listen, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server crashed", "err", err.Error())
			os.Exit(1)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	log.Info("shutdown signal received; draining connections")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", "err", err.Error())
		os.Exit(1)
	}
	if err := shutdownTracer(ctx); err != nil {
		log.Warn("tracer shutdown error", "err", err.Error())
	}
	log.Info("shutdown complete")
}

// composeDeps picks real or in-memory adapters based on which env vars
// are populated. The decision lives here so the use case + handler
// stay unaware of which mode they're running in (Rule #1 — open/closed:
// add a new adapter, edit only this function).
//
// Returns (deps, cleanup, err). Caller MUST invoke cleanup before exit
// even on the happy path — owns the Postgres pool + Redis client.
func composeDeps(ctx context.Context, cfg *config.Config, log *slog.Logger, m domain.Metrics) (usecase.RewriteDeps, coreHandlers, func(), error) {
	var cleanups []func()
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	// --- UserRepo --------------------------------------------------
	// pool is captured here so the Phase 0 core handlers (/health,
	// /auth/me) can share the same Postgres pool as the UserRepo.
	var pool *pgxpool.Pool
	var users domain.UserRepo
	if cfg.DatabaseURL != "" {
		p, err := platformdb.NewPool(ctx, cfg.DatabaseURL)
		if err != nil {
			cleanup()
			return usecase.RewriteDeps{}, coreHandlers{}, nil, fmt.Errorf("postgres pool: %w", err)
		}
		pool = p
		cleanups = append(cleanups, pool.Close)
		users = pg.NewUserRepo(pool)
		log.Info("adapter selected", "port", "users", "impl", "postgres")
	} else {
		users = memory.NewUserRepo(nil)
		log.Warn("adapter selected", "port", "users", "impl", "memory (DATABASE_URL unset — dev fallback)")
	}

	// --- RateLimiter -----------------------------------------------
	// trialLimiter shares the SAME redis client as the streaming rate limiter
	// (or the in-memory fallback). trialLimit mirrors Node's TRIAL_LIMIT
	// (NODE_ENV==='production' ? 3 : 999).
	var limiter domain.RateLimiter
	var trialLimiter parity.TrialLimiter
	if cfg.RedisURL != "" {
		opts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			cleanup()
			return usecase.RewriteDeps{}, coreHandlers{}, nil, fmt.Errorf("parse REDIS_URL: %w", err)
		}
		client := redis.NewClient(opts)
		cleanups = append(cleanups, func() { _ = client.Close() })
		limiter = redislimit.New(client)
		trialLimiter = redistrial.New(client)
		log.Info("adapter selected", "port", "rate_limiter", "impl", "redis")
	} else {
		limiter = memory.NewRateLimiter()
		trialLimiter = memory.NewTrialLimiter()
		log.Warn("adapter selected", "port", "rate_limiter", "impl", "memory (REDIS_URL unset — dev fallback)")
	}
	trialLimit := 999
	if cfg.IsProduction() {
		trialLimit = 3
	}

	// --- AiProvider ------------------------------------------------
	// provider is the ENV-built streaming chain for /v1/rewrite. The
	// non-streaming /rewrite + /extract surfaces no longer use the static
	// blocking completer — they resolve the DB default provider per request
	// (dbDefaultCompleter, wired in the pool block), matching Node's
	// findDefault() call timing. The chain's blocking head is discarded here.
	provider, _ := buildProviderChain(cfg, log)

	// --- Core Phase 0 handlers (/health, /auth/me) -----------------
	// /auth/me reads only verified JWT claims, so it needs no DB and is
	// always available. /health falls back to a static info reader
	// without a DB (it still needs the pool's client_log_level when one
	// exists). appVersion is the value /health reports; matches Node's
	// hardcoded "2.0.0" (health.controller.ts). One const so the two
	// construction branches can't drift.
	const appVersion = "2.0.0"
	var core coreHandlers
	core.me = corepkg.NewMeHandler(cfg.GoBackendRampPercent)

	// The single access-token verifier (JWT_SECRET). Built once here and
	// stashed on core so both the Router's Verifier field (RequireAuth) and
	// the errreport handler's optional best-effort JWT read share ONE
	// instance — no second verifier for the same secret.
	core.accessVerifier = auth.NewVerifier(cfg.JWTSecret)

	// Phase 4a: the IME-pack manifest is a pure in-memory static catalog —
	// no DB, so it's wired unconditionally (available even without a pool).
	core.imePacksManifest = http.HandlerFunc(imepackspkg.NewHandler().Manifest)

	// Phase 4b: /extract resolves the DB default provider per request, so it
	// needs the pool — wired inside the pool block below (was unconditional
	// when it used a static ENV completer; #45 moved it to DB resolution).

	// GET /rewrite/tones is a static catalog (no DB) — wire unconditionally.
	core.tones = http.HandlerFunc(parity.WriteTones)

	if pool != nil {
		q := sqlc.New(pool)
		core.health = corepkg.NewHealthHandler(corepkg.NewPgLogLevel(q), appVersion)

		// AI providers (DB default-provider resolver). Built FIRST in the pool
		// block so a single instance is shared by every consumer: the
		// non-streaming /rewrite + /extract use cases (via dbDefault), the
		// errreport/bugreports fix-proposers (Propose), and the admin CRUD
		// handler below. dbDefault adapts it to the rewrite + extraction
		// completer ports — both resolve the default provider per request (#45).
		aiProviderSvc := aiproviderpkg.NewService(aiproviderpkg.NewPgRepo(q, pool), aiproviderpkg.Factory{})
		dbDefault := dbDefaultCompleter{svc: aiProviderSvc}

		// Phase 4b: POST /extract — DB-resolved default provider per request
		// (Node ExtractionService.findDefault()), reporting provider.name.
		core.extract = http.HandlerFunc(extractionpkg.NewHandler(extractionpkg.NewService(dbDefault)).Extract)

		if cfg.JWTRefreshSecret == "" {
			cleanup()
			return usecase.RewriteDeps{}, coreHandlers{}, nil, errors.New("JWT_REFRESH_SECRET required when auth endpoints are enabled")
		}
		userRepo := userpkg.NewPgRepo(q, pool)
		userSvc := userpkg.NewService(userRepo)
		subReader := subpkg.NewReader(q)
		usageCounter := usagepkg.NewCounter(q)
		ttlReader := settingspkg.NewReader(q)
		// Phase 1b lifecycle collaborators: free-plan reader + free-sub
		// writer (register grant) + email sender (verify/reset). Social
		// verifier stays nil — wired in B9 (Part B); the lifecycle handlers
		// never dereference it.
		plansReader := planspkg.NewReader(q)
		core.plans = http.HandlerFunc(planspkg.NewHandler(planspkg.NewService(plansReader)).List)
		subWriter := subpkg.NewWriter(q)
		emailRepo := emailpkg.NewPgRepo(q)
		emailSvc := emailpkg.NewService(emailRepo, emailpkg.Config{
			EnvAPIKey: cfg.ResendAPIKey,
			EnvFrom:   cfg.EmailFrom,
		})
		socialVer := authpkg.NewHTTPSocialVerifier(cfg.AppleAudiences)
		authSvc := authpkg.NewService(userSvc, subReader, usageCounter, ttlReader, cfg.JWTSecret, cfg.JWTRefreshSecret,
			plansReader, subWriter, emailSvc, socialVer)
		authHandler := authpkg.NewHandler(authSvc)
		core.login = http.HandlerFunc(authHandler.Login)
		core.refresh = http.HandlerFunc(authHandler.Refresh)
		core.register = http.HandlerFunc(authHandler.Register)
		core.verifyEmail = http.HandlerFunc(authHandler.VerifyEmail)
		core.resendVerification = http.HandlerFunc(authHandler.ResendVerification)
		core.forgotPassword = http.HandlerFunc(authHandler.ForgotPassword)
		core.resetPassword = http.HandlerFunc(authHandler.ResetPassword)
		core.social = http.HandlerFunc(authHandler.Social)
		core.changePassword = http.HandlerFunc(authHandler.ChangePassword)
		core.account = http.HandlerFunc(authHandler.Account)
		core.deleteAccount = http.HandlerFunc(authHandler.DeleteAccount)
		subSvc := subpkg.NewService(subReader, usageCounter, plansReader)
		subHandler := subpkg.NewHandler(subSvc)
		core.subscription = http.HandlerFunc(subHandler.Get)
		core.verifyReceipt = http.HandlerFunc(subHandler.VerifyReceipt)

		// Parity rewrite (Node /rewrite controller): authed POST /rewrite + public
		// POST /rewrite/trial. subSvc supplies ResolveDailyLimit; usageCounter
		// supplies CountToday; dbDefault resolves the DB default provider per
		// request (Node RewriteService.callAI → findDefault). Trial seam wired via
		// WithTrial (limiter from Redis/memory, limit from the NODE_ENV-equivalent).
		// WithRewriteLog wires the fire-and-forget training-data capture Node runs
		// on every successful rewrite (authed + trial) for fine-tuning.
		paritySvc := parity.NewService(dbDefault, subSvc, usageCounter).
			WithTrial(trialLimiter, trialLimit, time.Now).
			WithRewriteLog(rewriteLogSink{repo: rewritelogpkg.NewPgRepo(q), log: log})
		parityHandler := parity.NewHandler(paritySvc)
		core.rewriteParity = http.HandlerFunc(parityHandler.Rewrite)
		core.rewriteTrial = http.HandlerFunc(parityHandler.Trial)

		// Extension tokens (T15): mint/list/revoke, JWT-gated. extSvc is also
		// stashed on coreHandlers so T16's dual-auth middleware on /v1/rewrite
		// can reuse the very same *exttoken.Service (verify path).
		extRepo := exttokenpkg.NewRepo(q)
		extSvc := exttokenpkg.NewService(extRepo)
		extHandler := exttokenpkg.NewHandler(extSvc)
		core.extSvc = extSvc
		core.mintExtToken = http.HandlerFunc(extHandler.Mint)
		core.listExtTokens = http.HandlerFunc(extHandler.List)
		core.revokeExtToken = http.HandlerFunc(extHandler.Revoke)

		// Phase 4a ancillary endpoints (DB-backed; public). Updates reads
		// app_releases/app_release_policies; errors writes/dedupes error rows.
		// The errreport handler reuses core.accessVerifier (the SAME access
		// verifier as RequireAuth) for its optional best-effort JWT read.
		updatesHandler := updatespkg.NewHandler(updatespkg.NewService(updatespkg.NewPgRepo(q)))
		core.updatesLatest = http.HandlerFunc(updatesHandler.Latest)
		errHandler := errreportpkg.NewHandler(errreportpkg.NewService(errreportpkg.NewPgRepo(q)), core.accessVerifier)
		core.errorsIngest = http.HandlerFunc(errHandler.Ingest)

		// Phase 4b DB-backed ingest endpoints.
		//
		// bug-reports (public, multipart): screenshots land under
		// cfg.BugReportsDir in date-bucketed subdirs. The clock MUST be UTC —
		// Node's dir name is new Date().toISOString().slice(0,10) (UTC), and a
		// local clock would put a report into the wrong day's folder near
		// midnight (shadow-parity divergence).
		bugStore := bugreportspkg.NewStorage(cfg.BugReportsDir, func() time.Time { return time.Now().UTC() })
		bugHandler := bugreportspkg.NewHandler(
			bugreportspkg.NewService(bugreportspkg.NewPgRepo(q), bugStore),
			core.accessVerifier, // access (session) verifier — never refresh
		)
		core.bugReportIngest = http.HandlerFunc(bugHandler.Create)

		// feedback (public): create + public board feed + JWT-gated vote. The
		// handler reads the optional/required JWT via the SAME access verifier.
		fbHandler := feedbackpkg.NewHandler(
			feedbackpkg.NewService(feedbackpkg.NewPgRepo(q)),
			core.accessVerifier, // access (session) verifier — never refresh
		)
		core.feedbackCreate = http.HandlerFunc(fbHandler.Create)
		core.feedbackList = http.HandlerFunc(fbHandler.List)
		core.feedbackVote = http.HandlerFunc(fbHandler.Vote)

		// email webhook (public, raw body): Resend/Svix delivery events reflect
		// onto email_logs + the suppression list. The *PgRepo satisfies the
		// suppressor port (MarkByProviderID + Suppress).
		core.emailWebhook = http.HandlerFunc(emailpkg.NewWebhookHandler(emailRepo, cfg.ResendWebhookSecret).Handle)

		// Phase 4c-1 admin foundation: admin-auth endpoints.
		adminAuthSvc := adminauth.NewService(
			adminauth.NewPgRepo(q),
			cfg.JWTSecret,
			cfg.JWTRefreshSecret,
			cfg.IsProduction(),
		)
		adminAuthHandler := adminauth.NewHandler(adminAuthSvc)
		core.adminLogin = http.HandlerFunc(adminAuthHandler.Login)
		core.adminChangePassword = http.HandlerFunc(adminAuthHandler.ChangePassword)
		core.adminMe = http.HandlerFunc(adminAuthHandler.Me)

		// Phase 4c-2 admin content/ops CRUD. Each handler method is its own
		// http.Handler route field (no concrete types on the Router — that would
		// cycle, since these packages import internal/shared). Routes are mounted
		// in Task 21; here we only construct + inject.

		// AI providers (6 routes): CRUD + paginated list + test. aiProviderSvc
		// is built once at the top of the pool block (shared DB resolver).
		aiProviderHandler := aiproviderpkg.NewHandler(aiProviderSvc)
		core.aiProvidersList = http.HandlerFunc(aiProviderHandler.List)
		core.aiProvidersPaginated = http.HandlerFunc(aiProviderHandler.Paginated)
		core.aiProviderCreate = http.HandlerFunc(aiProviderHandler.Create)
		core.aiProviderUpdate = http.HandlerFunc(aiProviderHandler.Update)
		core.aiProviderDelete = http.HandlerFunc(aiProviderHandler.Delete)
		core.aiProviderTest = http.HandlerFunc(aiProviderHandler.Test)

		// App settings (3 routes): get/patch + test-email. The payment module's
		// package-level AssertMethodsRegisterable is adapted to the MethodValidator
		// port; emailSvc satisfies EmailSender (SendRaw) directly.
		appSettingsSvc := appsettingspkg.NewService(appsettingspkg.NewPgRepo(q, pool), paymentMethodValidator{}, emailSvc)
		appSettingsHandler := appsettingspkg.NewHandler(appSettingsSvc)
		core.appSettingsGet = http.HandlerFunc(appSettingsHandler.Get)
		core.appSettingsPatch = http.HandlerFunc(appSettingsHandler.Patch)
		core.appSettingsTestEmail = http.HandlerFunc(appSettingsHandler.TestEmail)

		// Plans admin (4 routes): dual-mode list + create/update/delete.
		adminPlansSvc := planspkg.NewAdminService(planspkg.NewAdminRepo(q, pool))
		adminPlansHandler := planspkg.NewAdminHandler(adminPlansSvc)
		core.adminPlansList = http.HandlerFunc(adminPlansHandler.List)
		core.adminPlanCreate = http.HandlerFunc(adminPlansHandler.Create)
		core.adminPlanUpdate = http.HandlerFunc(adminPlansHandler.Update)
		core.adminPlanDelete = http.HandlerFunc(adminPlansHandler.Delete)

		// User admin (3 routes): the SAME *user.AdminRepo satisfies the repo,
		// SubReader (ActiveSubByUser) and RecentUsageReader (RecentUsageByUser)
		// ports; usageCounter is the UsageCounter.
		userAdminRepo := userpkg.NewAdminRepo(q, pool)
		userAdminSvc := userpkg.NewAdminService(userAdminRepo, usageCounter, userAdminRepo, userAdminRepo)
		userAdminHandler := userpkg.NewAdminHandler(userAdminSvc)
		core.adminUsersList = http.HandlerFunc(userAdminHandler.List)
		core.adminUserGet = http.HandlerFunc(userAdminHandler.GetUser)
		core.adminUserUpdate = http.HandlerFunc(userAdminHandler.UpdateUser)

		// Admin-users / portal accounts (4 routes): list/create/update/delete.
		adminAccountsSvc := adminauth.NewAdminUsersService(adminauth.NewAdminUsersRepo(q, pool))
		adminAccountsHandler := adminauth.NewAdminUsersHandler(adminAccountsSvc)
		core.adminAccountsList = http.HandlerFunc(adminAccountsHandler.List)
		core.adminAccountCreate = http.HandlerFunc(adminAccountsHandler.Create)
		core.adminAccountUpdate = http.HandlerFunc(adminAccountsHandler.Update)
		core.adminAccountDelete = http.HandlerFunc(adminAccountsHandler.Delete)

		// Email logs (1 route).
		emailLogsHandler := emailpkg.NewAdminLogsHandler(emailpkg.NewAdminLogsService(emailpkg.NewAdminLogsRepo(pool)))
		core.adminEmailLogs = http.HandlerFunc(emailLogsHandler.List)

		// Email templates (4 routes): list/update/reset/preview.
		emailTemplatesHandler := emailpkg.NewAdminTemplatesHandler(emailpkg.NewAdminTemplatesService(emailpkg.NewAdminTemplatesRepo(q)))
		core.adminEmailTemplatesList = http.HandlerFunc(emailTemplatesHandler.List)
		core.adminEmailTemplateUpdate = http.HandlerFunc(emailTemplatesHandler.Update)
		core.adminEmailTemplateReset = http.HandlerFunc(emailTemplatesHandler.Reset)
		core.adminEmailTemplatePreview = http.HandlerFunc(emailTemplatesHandler.Preview)

		// Payment read-side (Phase 3a): methods registry + status/history reads.
		// Read-only; no provider secrets, no checkout. q satisfies both the
		// payment Querier and the coreSettingsQuerier ports.
		paymentRepo := paymentpkg.NewRepo(q)
		paymentSettings := paymentpkg.NewSettingsAdapter(q)
		creds, err := paymentSettings.Credentials(ctx) // startup load; missing row → zero value (env fallback)
		if err != nil {
			cleanup()
			return usecase.RewriteDeps{}, coreHandlers{}, nil, fmt.Errorf("load payment credentials: %w", err)
		}
		vietqrStrat := vietqr.New(vietqr.Creds{
			BankID:        paymentstrategy.ResolveCredential(creds.VietQRBankID, cfg.VietQRBankID),
			AccountNumber: paymentstrategy.ResolveCredential(creds.VietQRAccountNumber, cfg.VietQRAccountNumber),
			AccountName:   paymentstrategy.ResolveCredential(creds.VietQRAccountName, cfg.VietQRAccountName),
			CassoAPIKey:   paymentstrategy.ResolveCredential(creds.CassoAPIKey, cfg.CassoAPIKey),
			SepayAPIKey:   paymentstrategy.ResolveCredential(creds.SepayAPIKey, cfg.SepayAPIKey),
		})
		stripeStrat := stripe.New(
			stripe.Creds{
				SecretKey:     paymentstrategy.ResolveCredential(creds.StripeSecretKey, cfg.StripeSecretKey),
				WebhookSecret: paymentstrategy.ResolveCredential(creds.StripeWebhookSecret, cfg.StripeWebhookSecret),
			},
			stripe.Env{
				PublishableKey:     cfg.StripePublishableKey,
				ApplePayMerchantID: cfg.ApplePayMerchantID,
				WebsiteURL:         cfg.WebsiteURL,
			},
		)
		lsStrat := lemonsqueezy.New(lemonsqueezy.Creds{
			APIKey:         paymentstrategy.ResolveCredential(creds.LemonSqueezyAPIKey, cfg.LemonSqueezyAPIKey),
			StoreID:        paymentstrategy.ResolveCredential(creds.LemonSqueezyStoreID, cfg.LemonSqueezyStoreID),
			VariantMonthly: creds.LemonSqueezyVariantMonthly,
			VariantYearly:  creds.LemonSqueezyVariantYearly,
			WebhookSecret:  paymentstrategy.ResolveCredential(creds.LemonSqueezyWebhookSecret, cfg.LemonSqueezyWebhookSecret),
		}, cfg.WebsiteURL)
		strategies := map[string]paymentstrategy.Strategy{
			"stripe":        stripeStrat,
			"vietqr":        vietqrStrat,
			"bank_transfer": vietqrStrat,
			"lemonsqueezy":  lsStrat,
			"apple_pay":     stripeStrat,
			"google_pay":    stripeStrat,
		}
		paymentSvc := paymentpkg.NewService(
			paymentRepo, paymentSettings, cfg.PaymentEnabledMethods,
			paymentRepo, // *Repo also satisfies CheckoutRepo
			strategies,
			time.Now,
			paymentpkg.GeneratePaymentReference,
			subReader, // *subpkg.Reader satisfies SubsPort (ActiveByUser) for portal/cancel
		)
		// Phase 3c: wire the webhook collaborators onto the same paymentSvc.
		// subWebhookWriter activates/extends/cancels subscriptions; the mail
		// emailer sends payment-failed/activated mails; paymentSettings resolves
		// LS variants for plan re-resolution. The handler holds the same pointer.
		subWebhookWriter := subpkg.NewWebhookWriter(q)
		paymentSvc.WithWebhook(
			paymentRepo,      // WebhookRepo
			subWebhookWriter, // SubsWriter
			paymentpkg.NewMailWebhookEmailer(emailSvc), // WebhookEmailer
			paymentSettings, // VariantResolver
		)
		paymentHandler := paymentpkg.NewHandler(paymentSvc)
		core.paymentMethods = http.HandlerFunc(paymentHandler.Methods)
		core.paymentStatus = http.HandlerFunc(paymentHandler.Status)
		core.paymentHistory = http.HandlerFunc(paymentHandler.History)
		core.paymentCheckout = http.HandlerFunc(paymentHandler.Checkout)
		core.paymentPortal = http.HandlerFunc(paymentHandler.Portal)
		core.paymentCancelSub = http.HandlerFunc(paymentHandler.CancelSubscription)
		core.paymentWebhookStripe = http.HandlerFunc(paymentHandler.StripeWebhook)
		core.paymentWebhookVietQR = http.HandlerFunc(paymentHandler.VietQRWebhook)
		core.paymentWebhookCasso = http.HandlerFunc(paymentHandler.CassoWebhook)
		core.paymentWebhookSepay = http.HandlerFunc(paymentHandler.SepayWebhook)
		core.paymentWebhookLemonSqueezy = http.HandlerFunc(paymentHandler.LemonSqueezyWebhook)

		// Daily subscription-expiry cron (ports NestJS @Cron("0 09 * * *")).
		// Reuses the same subReader (it satisfies subpkg.CronRepo via
		// DueForRenewal/ExpireLapsed) and the shared email sender. Fires once
		// per day at 09:00 server-local. The goroutine is a background daemon
		// scoped to ctx; it exits with the process.
		expiryCron := subpkg.NewExpiryCron(subReader, subpkg.NewMailCronNotifier(emailSvc, log), log)
		go scheduler.RunDaily(ctx, 9, 0, time.Local, time.Now, func(c context.Context, t time.Time) {
			if err := expiryCron.RunOnce(c, t); err != nil {
				log.Error("expiry cron failed", "err", err)
			}
		})
		log.Info("scheduler started", "job", "subscription-expiry", "at", "09:00 local")

		// ── Phase 4c-3 admin reporting (11 routes) ─────────────────────────
		// Reuses existing collaborators: userAdminRepo (user count), subReader
		// (active-sub aggregates + transaction list + latest-stripe lookup),
		// usageCounter (global usage), adminPlansSvc (plan catalog), paymentSvc
		// (Activate on confirm), subWebhookWriter (cancel on refund),
		// paymentSettings (live Stripe secret resolution).

		// stats + analytics (GET /admin/stats, GET /admin/analytics).
		adminStatsSvc := adminstatspkg.NewService(userAdminRepo, subReader, usageCounter, adminPlansSvc, time.Now)
		adminStatsHandler := adminstatspkg.NewHandler(adminStatsSvc)
		core.adminStats = http.HandlerFunc(adminStatsHandler.GetStats)
		core.adminAnalytics = http.HandlerFunc(adminStatsHandler.GetAnalytics)

		// transactions (GET /admin/transactions). subReader satisfies the
		// adminTransactionsService port (FindAllPaginated).
		subAdminHandler := subpkg.NewAdminHandler(subReader)
		core.adminTransactions = http.HandlerFunc(subAdminHandler.ListTransactions)

		// training-data / rewrite logs (GET stats|list|export, PATCH review).
		rewriteLogHandler := rewritelogpkg.NewHandler(rewritelogpkg.NewService(rewritelogpkg.NewPgRepo(q)))
		core.trainingDataStats = http.HandlerFunc(rewriteLogHandler.Stats)
		core.trainingDataList = http.HandlerFunc(rewriteLogHandler.List)
		core.trainingDataReview = http.HandlerFunc(rewriteLogHandler.Review)
		core.trainingDataExport = http.HandlerFunc(rewriteLogHandler.Export)

		// admin payments (GET stats|list, POST confirm|refund). refunder is nil
		// → NewAdminService defaults to the real Stripe SDK refunder; paymentSvc
		// supplies Activate; subReader the latest-stripe lookup; the two adapters
		// bridge the secret-resolution and cancel shape mismatches.
		paymentAdminSvc := paymentpkg.NewAdminService(
			paymentpkg.NewAdminRepo(q, pool),
			paymentSvc, // subscriptionActivator (Activate)
			nil,        // refunder → real Stripe SDK refunder (NewAdminService default)
			stripeSecretResolver{settings: paymentSettings, envKey: cfg.StripeSecretKey},
			subReader, // stripeSubLookup (FindLatestStripeForUserPlan)
			subCancelAdapter{w: subWebhookWriter},
			log,
			time.Now,
		)
		paymentAdminHandler := paymentpkg.NewAdminHandler(paymentAdminSvc)
		core.adminPaymentsStats = http.HandlerFunc(paymentAdminHandler.Stats)
		core.adminPaymentsList = http.HandlerFunc(paymentAdminHandler.ListPayments)
		core.adminPaymentConfirm = http.HandlerFunc(paymentAdminHandler.Confirm)
		core.adminPaymentRefund = http.HandlerFunc(paymentAdminHandler.Refund)

		// ── Phase 4c-4 admin triage (19 routes) + 2 hourly crons ───────────
		// Reuses aiProviderSvc (built above at the AI-providers wiring) as the
		// fixProposer for BOTH the errreport and bugreports admin services — one
		// default-provider resolver, shared. The bug admin repo needs the pool
		// (dynamic list-query idiom); errreport's admin methods live on the same
		// *errreport.PgRepo as the ingest path, so it only needs q.

		// fixProposalDisabled closes over the DISABLE_FIX_PROPOSAL_CRON toggle.
		// Both crons share it (Node: the same env var gates both schedulers).
		fixProposalDisabled := func() bool { return cfg.DisableFixProposalCron }

		// errreport admin (E1–E6) + its hourly fix-proposal cron.
		errAdminSvc := errreportpkg.NewAdminService(errreportpkg.NewPgRepo(q), aiProviderSvc, time.Now)
		errCron := errreportpkg.NewCron(errAdminSvc, fixProposalDisabled, time.Sleep)
		errAdminHandler := errreportpkg.NewAdminHandler(errAdminSvc, errCron)
		core.adminErrorsList = http.HandlerFunc(errAdminHandler.List)
		core.adminErrorGet = http.HandlerFunc(errAdminHandler.Get)
		core.adminErrorPatch = http.HandlerFunc(errAdminHandler.Patch)
		core.adminErrorDelete = http.HandlerFunc(errAdminHandler.Delete)
		core.adminErrorSuggest = http.HandlerFunc(errAdminHandler.SuggestFix)
		core.adminErrorsRunCron = http.HandlerFunc(errAdminHandler.RunCron)

		// bugreports admin (C1–C6) + its hourly fix-proposal cron.
		bugAdminSvc := bugreportspkg.NewAdminService(bugreportspkg.NewAdminPgRepo(q, pool), aiProviderSvc, time.Now)
		bugCron := bugreportspkg.NewCron(bugAdminSvc, fixProposalDisabled, time.Sleep)
		bugAdminHandler := bugreportspkg.NewAdminHandler(bugAdminSvc)
		core.adminBugList = http.HandlerFunc(bugAdminHandler.List)
		core.adminBugGet = http.HandlerFunc(bugAdminHandler.Get)
		core.adminBugScreenshot = http.HandlerFunc(bugAdminHandler.Screenshot)
		core.adminBugPatch = http.HandlerFunc(bugAdminHandler.Patch)
		core.adminBugDelete = http.HandlerFunc(bugAdminHandler.Delete)
		core.adminBugFixProposal = http.HandlerFunc(bugAdminHandler.FixProposal)

		// inbox (D1–D2): aggregates the two admin listers (errAdminSvc +
		// bugAdminSvc satisfy the inbox List ports).
		inboxHandler := inboxpkg.NewHandler(inboxpkg.NewService(errAdminSvc, bugAdminSvc))
		core.adminInboxCounts = http.HandlerFunc(inboxHandler.Counts)
		core.adminInbox = http.HandlerFunc(inboxHandler.Feed)

		// updates admin (releases CRUD + policy upsert). The SAME *updates.PgRepo
		// satisfies the adminRepo port.
		updatesAdminHandler := updatespkg.NewAdminHandler(updatespkg.NewAdminService(updatespkg.NewPgRepo(q)))
		core.adminReleasesList = http.HandlerFunc(updatesAdminHandler.ListReleases)
		core.adminReleaseUpsert = http.HandlerFunc(updatesAdminHandler.UpsertRelease)
		core.adminReleaseDelete = http.HandlerFunc(updatesAdminHandler.DeleteRelease)
		core.adminPolicyUpsert = http.HandlerFunc(updatesAdminHandler.UpsertPolicy)

		// subscription grant (POST /admin/subscriptions/grant). Reuses subWriter.
		grantHandler := subpkg.NewGrantHandler(subWriter)
		core.adminGrantSub = http.HandlerFunc(grantHandler.Grant)

		// Two hourly fix-proposal crons (ports NestJS @Cron hourly schedulers).
		// Each is a background daemon scoped to ctx; toggle-gating happens INSIDE
		// RunOnce (it short-circuits when fixProposalDisabled() is true), so the
		// goroutines still tick but do no work while disabled — mirroring Node,
		// where the @Cron method early-returns on the env flag.
		go scheduler.RunHourly(ctx, time.Local, time.Now, func(c context.Context) {
			res := errCron.RunOnce(c)
			if res.Success > 0 || res.Failed > 0 {
				log.Info("error fix-proposal cron", "succeeded", res.Success, "failed", res.Failed)
			}
		})
		go scheduler.RunHourly(ctx, time.Local, time.Now, func(c context.Context) {
			res := bugCron.RunOnce(c)
			if res.Success > 0 || res.Failed > 0 {
				log.Info("bug fix-proposal cron", "succeeded", res.Success, "failed", res.Failed)
			}
		})
		log.Info("scheduler started", "job", "fix-proposal", "interval", "hourly", "disabled", cfg.DisableFixProposalCron)
	} else {
		core.health = corepkg.NewHealthHandler(staticInfoReader{}, appVersion)
		log.Warn("auth endpoints disabled", "reason", "no DATABASE_URL")
	}

	return usecase.RewriteDeps{
		Users:     users,
		Provider:  provider,
		RateLimit: limiter,
		Metrics:   m,
		Now:       time.Now,
		Log:       log,
	}, core, cleanup, nil
}

// coreHandlers bundles the Phase 0 proof endpoints so composeDeps can
// return them alongside the rewrite deps without widening the call site
// into a long positional tuple.
type coreHandlers struct {
	health  http.Handler // GET /health  (always set)
	me      http.Handler // GET /auth/me (always set — JWT claims only, no DB)
	login   http.Handler // POST /auth/login (set when pool != nil)
	refresh http.Handler // POST /auth/refresh (set when pool != nil)

	// Phase 1b lifecycle handlers (set when pool != nil; all public).
	register           http.Handler // POST /auth/register
	verifyEmail        http.Handler // POST /auth/verify-email
	resendVerification http.Handler // POST /auth/resend-verification
	forgotPassword     http.Handler // POST /auth/forgot-password
	resetPassword      http.Handler // POST /auth/reset-password
	social             http.Handler // POST /auth/social

	plans http.Handler // GET /plans (set when pool != nil)

	changePassword http.Handler // POST /auth/change-password (set when pool != nil)
	account        http.Handler // GET /auth/account (set when pool != nil)
	deleteAccount  http.Handler // DELETE /auth/account (set when pool != nil)
	subscription   http.Handler // GET /subscription (set when pool != nil)
	verifyReceipt  http.Handler // POST /subscription/verify-receipt (set when pool != nil)

	// Payment read-side (Phase 3a; set when pool != nil).
	paymentMethods http.Handler // GET /payment/methods (public)
	paymentStatus  http.Handler // GET /payment/status/{ref} (public)
	paymentHistory http.Handler // GET /payment/history (auth)

	paymentCheckout  http.Handler // POST /payment/checkout      (auth)
	paymentPortal    http.Handler // GET /payment/portal         (auth)
	paymentCancelSub http.Handler // DELETE /payment/subscription (auth)

	// Payment webhooks (Phase 3c; set when pool != nil; all public).
	paymentWebhookStripe       http.Handler // POST /payment/webhook/stripe
	paymentWebhookVietQR       http.Handler // POST /payment/webhook/vietqr
	paymentWebhookCasso        http.Handler // POST /payment/webhook/casso
	paymentWebhookSepay        http.Handler // POST /payment/webhook/sepay
	paymentWebhookLemonSqueezy http.Handler // POST /payment/webhook/lemonsqueezy

	// Extension-token handlers (set when pool != nil; all JWT-gated).
	mintExtToken   http.Handler // POST   /auth/extension-tokens
	listExtTokens  http.Handler // GET    /auth/extension-tokens
	revokeExtToken http.Handler // DELETE /auth/extension-tokens/{id}

	// extSvc is the live *exttoken.Service (verify path). Stashed so T16's
	// dual-auth middleware on /v1/rewrite can authorize presented ext tokens
	// without re-building the service. Nil when pool == nil.
	extSvc *exttokenpkg.Service

	// Phase 4a ancillary handlers (all public).
	//   imePacksManifest — GET /ime-packs/manifest (static; always set).
	//   updatesLatest    — GET /updates/latest     (DB; set when pool != nil).
	//   errorsIngest     — POST /errors            (DB; set when pool != nil).
	imePacksManifest http.Handler
	updatesLatest    http.Handler
	errorsIngest     http.Handler

	// Parity /rewrite controller handlers.
	//   tones         — GET  /rewrite/tones (static; always set).
	//   rewriteParity — POST /rewrite       (dual-auth; set when pool != nil).
	//   rewriteTrial  — POST /rewrite/trial (public; set when pool != nil).
	tones         http.Handler
	rewriteParity http.Handler
	rewriteTrial  http.Handler

	// Phase 4b handlers.
	//   extract         — POST /extract (auth; no DB — always set).
	//   bugReportIngest — POST /bug-reports (public; DB — set when pool != nil).
	//   feedbackCreate  — POST /feedback (public; DB — set when pool != nil).
	//   feedbackList    — GET  /feedback (public; DB — set when pool != nil).
	//   feedbackVote    — POST /feedback/{id}/vote (public; DB — set when pool != nil).
	//   emailWebhook    — POST /webhooks/resend (public, raw body; DB — set when pool != nil).
	extract         http.Handler
	bugReportIngest http.Handler
	feedbackCreate  http.Handler
	feedbackList    http.Handler
	feedbackVote    http.Handler
	emailWebhook    http.Handler

	// Phase 4c-1 admin foundation (set when pool != nil).
	adminLogin          http.Handler // POST /admin/auth/login           (public)
	adminChangePassword http.Handler // POST /admin/auth/change-password (admin)
	adminMe             http.Handler // GET  /admin/auth/me              (admin)

	// Phase 4c-2 admin content/ops CRUD (set when pool != nil; all admin).
	// One http.Handler per route; mounted in Task 21.
	aiProvidersList      http.Handler // GET    /admin/ai-providers           (admin)
	aiProvidersPaginated http.Handler // GET    /admin/ai-providers/paginated (admin)
	aiProviderCreate     http.Handler // POST   /admin/ai-providers           (admin)
	aiProviderUpdate     http.Handler // PATCH  /admin/ai-providers/{id}      (admin)
	aiProviderDelete     http.Handler // DELETE /admin/ai-providers/{id}      (admin)
	aiProviderTest       http.Handler // POST   /admin/ai-providers/{id}/test (admin)

	appSettingsGet       http.Handler // GET   /admin/settings            (admin)
	appSettingsPatch     http.Handler // PATCH /admin/settings            (admin)
	appSettingsTestEmail http.Handler // POST  /admin/settings/test-email (admin)

	adminPlansList  http.Handler // GET    /admin/plans      (admin)
	adminPlanCreate http.Handler // POST   /admin/plans      (admin)
	adminPlanUpdate http.Handler // PATCH  /admin/plans/{id} (admin)
	adminPlanDelete http.Handler // DELETE /admin/plans/{id} (admin)

	adminUsersList  http.Handler // GET   /admin/users      (admin)
	adminUserGet    http.Handler // GET   /admin/users/{id} (admin)
	adminUserUpdate http.Handler // PATCH /admin/users/{id} (admin)

	adminAccountsList  http.Handler // GET    /admin/admin-users      (admin)
	adminAccountCreate http.Handler // POST   /admin/admin-users      (admin)
	adminAccountUpdate http.Handler // PATCH  /admin/admin-users/{id} (admin)
	adminAccountDelete http.Handler // DELETE /admin/admin-users/{id} (admin)

	adminEmailLogs http.Handler // GET /admin/email-logs (admin)

	adminEmailTemplatesList   http.Handler // GET    /admin/email-templates               (admin)
	adminEmailTemplateUpdate  http.Handler // PATCH  /admin/email-templates/{key}         (admin)
	adminEmailTemplateReset   http.Handler // DELETE /admin/email-templates/{key}         (admin)
	adminEmailTemplatePreview http.Handler // GET    /admin/email-templates/{key}/preview (admin)

	// Phase 4c-3 admin reporting (set when pool != nil; all admin).
	adminStats        http.Handler // GET  /admin/stats
	adminAnalytics    http.Handler // GET  /admin/analytics
	adminTransactions http.Handler // GET  /admin/transactions

	trainingDataStats  http.Handler // GET   /admin/training-data/stats
	trainingDataList   http.Handler // GET   /admin/training-data
	trainingDataReview http.Handler // PATCH /admin/training-data/{id}
	trainingDataExport http.Handler // GET   /admin/training-data/export

	adminPaymentsStats  http.Handler // GET  /admin/payments/stats
	adminPaymentsList   http.Handler // GET  /admin/payments
	adminPaymentConfirm http.Handler // POST /admin/payments/{id}/confirm
	adminPaymentRefund  http.Handler // POST /admin/payments/{id}/refund

	// Phase 4c-4 admin triage (set when pool != nil; all admin). 19 routes:
	// errreport admin (E1–E6), bugreports admin (C1–C6), inbox (D1–D2),
	// updates admin (releases/policy), subscription grant.
	adminErrorsList    http.Handler // GET    /admin/errors
	adminErrorGet      http.Handler // GET    /admin/errors/{id}
	adminErrorPatch    http.Handler // PATCH  /admin/errors/{id}
	adminErrorDelete   http.Handler // DELETE /admin/errors/{id}
	adminErrorSuggest  http.Handler // POST   /admin/errors/{id}/suggest-fix
	adminErrorsRunCron http.Handler // POST   /admin/errors/run-ai-cron

	adminBugList        http.Handler // GET    /admin/bug-reports
	adminBugGet         http.Handler // GET    /admin/bug-reports/{id}
	adminBugScreenshot  http.Handler // GET    /admin/bug-reports/{id}/screenshot
	adminBugPatch       http.Handler // PATCH  /admin/bug-reports/{id}
	adminBugDelete      http.Handler // DELETE /admin/bug-reports/{id}
	adminBugFixProposal http.Handler // POST   /admin/bug-reports/{id}/fix-proposal

	adminInboxCounts http.Handler // GET /admin/inbox/counts
	adminInbox       http.Handler // GET /admin/inbox

	adminReleasesList  http.Handler // GET    /admin/releases
	adminReleaseUpsert http.Handler // POST   /admin/releases
	adminReleaseDelete http.Handler // DELETE /admin/releases/{platform}/{channel}
	adminPolicyUpsert  http.Handler // POST   /admin/release-policies

	adminGrantSub http.Handler // POST /admin/subscriptions/grant

	// accessVerifier is the single *auth.Verifier for JWT_SECRET. Shared by
	// the Router's Verifier field (RequireAuth) and the errreport handler's
	// optional best-effort JWT read — exactly one instance per access secret.
	accessVerifier *auth.Verifier
}

// paymentMethodValidator adapts payment.AssertMethodsRegisterable (a
// package-level func) to appsettings.MethodValidator (a 1-method interface),
// so PATCH /admin/settings rejects enabling a payment method with no backend
// strategy. A tiny method-value adapter — the composition root is the only
// place that bridges the two modules' shapes.
type paymentMethodValidator struct{}

func (paymentMethodValidator) AssertMethodsRegisterable(csv string) error {
	return paymentpkg.AssertMethodsRegisterable(csv)
}

// dbDefaultCompleter routes the non-streaming rewrite + extraction use cases
// through the DB-configured default AI provider, resolved PER REQUEST via
// aiprovider.Service.DefaultComplete (which reads ai_providers.is_default and
// reports provider.name). This mirrors Node, where both RewriteService.callAI
// and ExtractionService.extract call aiProviders.findDefault() at call time —
// not a process-static ENV provider. It satisfies parity.completer (Complete)
// and extraction.DefaultProvider (DefaultComplete), translating aiprovider's
// no-default sentinel into each consumer's own sentinel so the HTTP edges map
// it to the right status (rewrite/extract → 400 invalid-input).
type dbDefaultCompleter struct{ svc *aiproviderpkg.Service }

func (d dbDefaultCompleter) Complete(ctx context.Context, system, user string) (parity.Completion, error) {
	text, _, model, ptype, ms, err := d.svc.DefaultCompleteFull(ctx, system, user)
	if errors.Is(err, aiproviderpkg.ErrNoDefaultProvider) {
		return parity.Completion{}, parity.ErrNoDefaultProvider
	}
	if err != nil {
		return parity.Completion{}, err
	}
	return parity.Completion{
		Text:           text,
		Model:          model,
		ProviderType:   ptype,
		ResponseTimeMs: ms,
	}, nil
}

// rewriteLogSink adapts the rewritelog repo to parity.rewriteLogger. It honors
// the port's fire-and-forget contract: LogRewrite returns immediately, detaching
// the INSERT onto a goroutine with a cancellation-detached context (the request
// ctx is cancelled once the response is written) and swallowing failures with a
// warn log — mirroring Node's `this.rewriteLogService.log({...}).catch(()=>{})`.
type rewriteLogSink struct {
	repo *rewritelogpkg.PgRepo
	log  *slog.Logger
}

func (s rewriteLogSink) LogRewrite(ctx context.Context, e parity.RewriteLogEntry) {
	ctx = context.WithoutCancel(ctx)
	go func() {
		err := s.repo.Insert(ctx, rewritelogpkg.RewriteLogInput{
			Tone:           e.Tone,
			InputText:      e.InputText,
			OutputText:     e.OutputText,
			Model:          e.Model,
			ProviderType:   e.ProviderType,
			ResponseTimeMs: e.ResponseTimeMs,
		})
		if err != nil {
			s.log.Warn("rewrite_logs insert failed", "err", err)
		}
	}()
}

func (d dbDefaultCompleter) DefaultComplete(ctx context.Context, system, user string) (string, string, error) {
	text, name, _, err := d.svc.DefaultComplete(ctx, system, user)
	if errors.Is(err, aiproviderpkg.ErrNoDefaultProvider) {
		return "", "", extractionpkg.ErrNoDefaultProvider
	}
	return text, name, err
}

// stripeSecretResolver adapts the payment SettingsAdapter + env fallback to the
// payment AdminService's stripeSecretSource port. It resolves the Stripe secret
// live per refund (settings override → env), so an operator key rotation in
// app_settings takes effect without a restart — the same ResolveCredential
// policy the strategy wiring applies at startup.
type stripeSecretResolver struct {
	settings *paymentpkg.SettingsAdapter
	envKey   string
}

func (r stripeSecretResolver) StripeSecretKey(ctx context.Context) (string, error) {
	creds, err := r.settings.Credentials(ctx)
	if err != nil {
		return "", err
	}
	return paymentstrategy.ResolveCredential(creds.StripeSecretKey, r.envKey), nil
}

// subCancelAdapter narrows subscription.WebhookWriter.CancelByStoreRef
// (rows int64, err error) → error for the payment AdminService's
// subscriptionCanceller port; the affected-row count is irrelevant to refund.
type subCancelAdapter struct {
	w *subpkg.WebhookWriter
}

func (a subCancelAdapter) CancelByStoreRef(ctx context.Context, storeType, storeRef string) error {
	_, err := a.w.CancelByStoreRef(ctx, storeType, storeRef)
	return err
}

// staticInfoReader is the dev-fallback LogLevelReader used when no
// Postgres pool exists. It always reports "info" so /health stays
// healthy without a database.
type staticInfoReader struct{}

func (staticInfoReader) ClientLogLevel(context.Context) (string, error) { return "info", nil }

// buildProviderChain reads cfg.AIProviders (comma-separated priority
// list) and assembles a chain.Provider. Filtering rules:
//
//   - "openai"    requires OpenAIKey;    skipped + warned otherwise.
//   - "anthropic" requires AnthropicKey; skipped + warned otherwise.
//   - "ollama"    requires OllamaURL;    skipped + warned otherwise.
//   - empty list (or all entries filtered) → memory stub fallback so
//     the dev binary still produces visible output.
//
// Unknown tokens are warned then ignored — a typo in the env doesn't
// crash boot, but it's visible in logs.
//
// Why an env-driven list + chain wrapper (vs hardcoded chain):
// operators want to flip the priority order without a rebuild
// (incident response: "OpenAI is down, push Anthropic to head").
//
// Returns (chain, completer): the streaming chain for /v1/rewrite and the
// blocking Completer (the priority HEAD provider) for /extract. Both come
// from the SAME selection pass — one source of truth for provider order so
// the streaming + blocking surfaces can never disagree on the head. Every
// concrete adapter (openai/anthropic/ollama/memory) satisfies BOTH
// domain.AiProvider (Stream) and aicall.Completer (Complete) after Task 1.
func buildProviderChain(cfg *config.Config, log *slog.Logger) (domain.AiProvider, aicall.Completer) {
	raw := strings.TrimSpace(cfg.AIProviders)
	if raw == "" {
		log.Warn("adapter selected", "port", "ai_provider", "impl", "memory (AI_PROVIDERS unset — dev fallback)")
		stub := memory.NewProvider("memory-stub",
			[]string{"[", "stub", " ", "rewrite", "]"})
		return stub, stub
	}

	// completers parallels providers element-for-element (same concrete
	// adapter, viewed through its blocking port). completers[0] is the head.
	var providers []domain.AiProvider
	var completers []aicall.Completer
	var picked []string
	for _, name := range strings.Split(raw, ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		switch name {
		case "":
			continue
		case "openai":
			if cfg.OpenAIKey == "" {
				log.Warn("chain: skipping provider, missing credential", "provider", name, "env", "OPENAI_API_KEY")
				continue
			}
			p := openai.New(resolveProviderID(cfg.OpenAIProviderID, log, "openai"), cfg.OpenAIKey)
			providers = append(providers, p)
			completers = append(completers, p)
		case "anthropic":
			if cfg.AnthropicKey == "" {
				log.Warn("chain: skipping provider, missing credential", "provider", name, "env", "ANTHROPIC_API_KEY")
				continue
			}
			p := anthropic.New(resolveProviderID(cfg.AnthropicProviderID, log, "anthropic"), cfg.AnthropicKey)
			providers = append(providers, p)
			completers = append(completers, p)
		case "ollama":
			if cfg.OllamaURL == "" {
				log.Warn("chain: skipping provider, missing endpoint", "provider", name, "env", "OLLAMA_URL")
				continue
			}
			p := ollama.New(resolveProviderID(cfg.OllamaProviderID, log, "ollama"), ollama.WithEndpoint(cfg.OllamaURL))
			providers = append(providers, p)
			completers = append(completers, p)
		default:
			log.Warn("chain: unknown provider name; ignoring", "provider", name)
			continue
		}
		picked = append(picked, name)
	}

	if len(providers) == 0 {
		log.Warn("adapter selected", "port", "ai_provider", "impl", "memory (no usable entries in AI_PROVIDERS — dev fallback)")
		stub := memory.NewProvider("memory-stub",
			[]string{"[", "stub", " ", "rewrite", "]"})
		return stub, stub
	}

	// Single-provider config doesn't need the failover wrapper — and
	// shouldn't have one, because chain.Provider exposes its own
	// uuid.New() id which would NOT match the operator-pinned
	// PROVIDER_ID, causing usage_logs FK violations.  Return the lone
	// provider unwrapped so its pinned id reaches usage_logs unmodified.
	if len(providers) == 1 {
		log.Info("adapter selected", "port", "ai_provider", "impl", picked[0])
		return providers[0], completers[0]
	}

	chainName := "chain:" + strings.Join(picked, ">")
	log.Info("adapter selected", "port", "ai_provider", "impl", chainName)
	// /extract uses the priority-HEAD provider's blocking port (completers[0]);
	// it does not failover (the chain wrapper is streaming-only, and Node's
	// extraction calls the single default provider, not a chain).
	return chain.New(chainName, providers, chain.WithLogger(log)), completers[0]
}

// resolveProviderID parses an env-supplied ai_providers.id (UUID
// string) and falls back to a freshly minted UUID when the env var
// is unset OR malformed.  Pinning the ID lets the Go service write
// usage_logs.ai_provider_id rows that satisfy the existing FK
// constraint against ai_providers — so NestJS + Go served calls
// share one provider row for analytics joins.
//
// One helper for every provider type so the resolve-or-mint policy
// lives in a single place (Rule #1 — extendable: adding a new
// provider = call the same helper).
func resolveProviderID(raw string, log *slog.Logger, name string) uuid.UUID {
	if raw == "" {
		minted := uuid.New()
		log.Info("provider id minted (env unset; usage_logs FK may fail)",
			"provider", name, "id", minted.String())
		return minted
	}
	parsed, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		minted := uuid.New()
		log.Warn("provider id env malformed; falling back to mint",
			"provider", name, "env_val", raw, "err", err.Error(),
			"id", minted.String())
		return minted
	}
	log.Info("provider id pinned from env", "provider", name, "id", parsed.String())
	return parsed
}

// newLogger returns a JSON-output slog suitable for production log
// aggregation (Loki / CloudWatch / etc.). Level threshold parsed from
// config.LogLevel.
func newLogger(levelStr string) *slog.Logger {
	level := slog.LevelInfo
	switch levelStr {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
