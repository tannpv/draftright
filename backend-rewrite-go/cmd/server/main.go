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
	authpkg "github.com/tannpv/draftright-rewrite/internal/auth"
	corepkg "github.com/tannpv/draftright-rewrite/internal/core"
	emailpkg "github.com/tannpv/draftright-rewrite/internal/email"
	exttokenpkg "github.com/tannpv/draftright-rewrite/internal/exttoken"
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
	"github.com/tannpv/draftright-rewrite/internal/rewrite/transport"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/usecase"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	subpkg "github.com/tannpv/draftright-rewrite/internal/subscription"
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
		Verifier:       auth.NewVerifier(cfg.JWTSecret),
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
		MintExtToken:       core.mintExtToken,
		ListExtTokens:      core.listExtTokens,
		RevokeExtToken:     core.revokeExtToken,
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
	var limiter domain.RateLimiter
	if cfg.RedisURL != "" {
		opts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			cleanup()
			return usecase.RewriteDeps{}, coreHandlers{}, nil, fmt.Errorf("parse REDIS_URL: %w", err)
		}
		client := redis.NewClient(opts)
		cleanups = append(cleanups, func() { _ = client.Close() })
		limiter = redislimit.New(client)
		log.Info("adapter selected", "port", "rate_limiter", "impl", "redis")
	} else {
		limiter = memory.NewRateLimiter()
		log.Warn("adapter selected", "port", "rate_limiter", "impl", "memory (REDIS_URL unset — dev fallback)")
	}

	// --- AiProvider ------------------------------------------------
	provider := buildProviderChain(cfg, log)

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
	if pool != nil {
		q := sqlc.New(pool)
		core.health = corepkg.NewHealthHandler(corepkg.NewPgLogLevel(q), appVersion)

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
		emailSvc := emailpkg.NewService(emailpkg.NewPgRepo(q), emailpkg.Config{
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
		subHandler := subpkg.NewHandler(subpkg.NewService(subReader, usageCounter, plansReader))
		core.subscription = http.HandlerFunc(subHandler.Get)
		core.verifyReceipt = http.HandlerFunc(subHandler.VerifyReceipt)

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
		paymentHandler := paymentpkg.NewHandler(paymentSvc)
		core.paymentMethods = http.HandlerFunc(paymentHandler.Methods)
		core.paymentStatus = http.HandlerFunc(paymentHandler.Status)
		core.paymentHistory = http.HandlerFunc(paymentHandler.History)
		core.paymentCheckout = http.HandlerFunc(paymentHandler.Checkout)
		core.paymentPortal = http.HandlerFunc(paymentHandler.Portal)
		core.paymentCancelSub = http.HandlerFunc(paymentHandler.CancelSubscription)

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

	// Extension-token handlers (set when pool != nil; all JWT-gated).
	mintExtToken   http.Handler // POST   /auth/extension-tokens
	listExtTokens  http.Handler // GET    /auth/extension-tokens
	revokeExtToken http.Handler // DELETE /auth/extension-tokens/{id}

	// extSvc is the live *exttoken.Service (verify path). Stashed so T16's
	// dual-auth middleware on /v1/rewrite can authorize presented ext tokens
	// without re-building the service. Nil when pool == nil.
	extSvc *exttokenpkg.Service
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
func buildProviderChain(cfg *config.Config, log *slog.Logger) domain.AiProvider {
	raw := strings.TrimSpace(cfg.AIProviders)
	if raw == "" {
		log.Warn("adapter selected", "port", "ai_provider", "impl", "memory (AI_PROVIDERS unset — dev fallback)")
		return memory.NewProvider("memory-stub",
			[]string{"[", "stub", " ", "rewrite", "]"})
	}

	var providers []domain.AiProvider
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
			providers = append(providers, openai.New(resolveProviderID(cfg.OpenAIProviderID, log, "openai"), cfg.OpenAIKey))
		case "anthropic":
			if cfg.AnthropicKey == "" {
				log.Warn("chain: skipping provider, missing credential", "provider", name, "env", "ANTHROPIC_API_KEY")
				continue
			}
			providers = append(providers, anthropic.New(resolveProviderID(cfg.AnthropicProviderID, log, "anthropic"), cfg.AnthropicKey))
		case "ollama":
			if cfg.OllamaURL == "" {
				log.Warn("chain: skipping provider, missing endpoint", "provider", name, "env", "OLLAMA_URL")
				continue
			}
			providers = append(providers, ollama.New(resolveProviderID(cfg.OllamaProviderID, log, "ollama"), ollama.WithEndpoint(cfg.OllamaURL)))
		default:
			log.Warn("chain: unknown provider name; ignoring", "provider", name)
			continue
		}
		picked = append(picked, name)
	}

	if len(providers) == 0 {
		log.Warn("adapter selected", "port", "ai_provider", "impl", "memory (no usable entries in AI_PROVIDERS — dev fallback)")
		return memory.NewProvider("memory-stub",
			[]string{"[", "stub", " ", "rewrite", "]"})
	}

	// Single-provider config doesn't need the failover wrapper — and
	// shouldn't have one, because chain.Provider exposes its own
	// uuid.New() id which would NOT match the operator-pinned
	// PROVIDER_ID, causing usage_logs FK violations.  Return the lone
	// provider unwrapped so its pinned id reaches usage_logs unmodified.
	if len(providers) == 1 {
		log.Info("adapter selected", "port", "ai_provider", "impl", picked[0])
		return providers[0]
	}

	chainName := "chain:" + strings.Join(picked, ">")
	log.Info("adapter selected", "port", "ai_provider", "impl", chainName)
	return chain.New(chainName, providers, chain.WithLogger(log))
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
