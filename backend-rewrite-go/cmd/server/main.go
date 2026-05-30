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
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/tannpv/draftright-rewrite/internal/adapter/memory"
	"github.com/tannpv/draftright-rewrite/internal/adapter/openai"
	"github.com/tannpv/draftright-rewrite/internal/adapter/pg"
	"github.com/tannpv/draftright-rewrite/internal/adapter/redislimit"
	"github.com/tannpv/draftright-rewrite/internal/domain"
	internalhttp "github.com/tannpv/draftright-rewrite/internal/http"
	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/platform/config"
	platformdb "github.com/tannpv/draftright-rewrite/internal/platform/db"
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

	deps, cleanup, err := composeDeps(context.Background(), cfg, log)
	if err != nil {
		log.Error("dependency wiring failed", "err", err.Error())
		os.Exit(1)
	}
	defer cleanup()

	router := (&internalhttp.Router{
		Log:      log,
		Verifier: auth.NewVerifier(cfg.JWTSecret),
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
	log.Info("shutdown complete")
}

// composeDeps picks real or in-memory adapters based on which env vars
// are populated. The decision lives here so the use case + handler
// stay unaware of which mode they're running in (Rule #1 — open/closed:
// add a new adapter, edit only this function).
//
// Returns (deps, cleanup, err). Caller MUST invoke cleanup before exit
// even on the happy path — owns the Postgres pool + Redis client.
func composeDeps(ctx context.Context, cfg *config.Config, log *slog.Logger) (usecase.RewriteDeps, func(), error) {
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
	var provider domain.AiProvider
	if cfg.OpenAIKey != "" {
		provider = openai.New(uuid.New(), cfg.OpenAIKey)
		log.Info("adapter selected", "port", "ai_provider", "impl", "openai")
	} else {
		// Memory provider streams a canned response so a smoke test
		// without API keys still produces something visible.
		provider = memory.NewProvider("memory-stub",
			[]string{"[", "stub", " ", "rewrite", "]"})
		log.Warn("adapter selected", "port", "ai_provider", "impl", "memory (OPENAI_API_KEY unset — dev fallback)")
	}

	return usecase.RewriteDeps{
		Users:     users,
		Provider:  provider,
		RateLimit: limiter,
		Now:       time.Now,
		Log:       log,
	}, cleanup, nil
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
