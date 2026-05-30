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
	"github.com/redis/go-redis/v9"

	"github.com/tannpv/draftright-rewrite/internal/adapter/anthropic"
	"github.com/tannpv/draftright-rewrite/internal/adapter/chain"
	"github.com/tannpv/draftright-rewrite/internal/adapter/memory"
	"github.com/tannpv/draftright-rewrite/internal/adapter/ollama"
	"github.com/tannpv/draftright-rewrite/internal/adapter/openai"
	"github.com/tannpv/draftright-rewrite/internal/adapter/pg"
	"github.com/tannpv/draftright-rewrite/internal/adapter/redislimit"
	"github.com/tannpv/draftright-rewrite/internal/domain"
	internalhttp "github.com/tannpv/draftright-rewrite/internal/http"
	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/platform/config"
	platformdb "github.com/tannpv/draftright-rewrite/internal/platform/db"
	"github.com/tannpv/draftright-rewrite/internal/platform/metrics"
	"github.com/tannpv/draftright-rewrite/internal/platform/tracing"
	"github.com/tannpv/draftright-rewrite/internal/usecase"
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
		metricsSink   domain.Metrics
		metricsHTTP   http.Handler
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

	deps, cleanup, err := composeDeps(context.Background(), cfg, log, metricsSink)
	if err != nil {
		log.Error("dependency wiring failed", "err", err.Error())
		os.Exit(1)
	}
	defer cleanup()

	router := (&internalhttp.Router{
		Log:            log,
		Verifier:       auth.NewVerifier(cfg.JWTSecret),
		MetricsHandler: metricsHTTP,
		EnableTracing:  cfg.OtelEndpoint != "",
		Rewrite: &internalhttp.RewriteHandler{
			Deps: deps,
			Log:  log,
		},
	}).Build()

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
func composeDeps(ctx context.Context, cfg *config.Config, log *slog.Logger, m domain.Metrics) (usecase.RewriteDeps, func(), error) {
	var cleanups []func()
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	// --- UserRepo --------------------------------------------------
	var users domain.UserRepo
	if cfg.DatabaseURL != "" {
		pool, err := platformdb.NewPool(ctx, cfg.DatabaseURL)
		if err != nil {
			cleanup()
			return usecase.RewriteDeps{}, nil, fmt.Errorf("postgres pool: %w", err)
		}
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
			return usecase.RewriteDeps{}, nil, fmt.Errorf("parse REDIS_URL: %w", err)
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

	return usecase.RewriteDeps{
		Users:     users,
		Provider:  provider,
		RateLimit: limiter,
		Metrics:   m,
		Now:       time.Now,
		Log:       log,
	}, cleanup, nil
}

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
