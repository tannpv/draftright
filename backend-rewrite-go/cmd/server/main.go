// Package main is the composition root for the DraftRight /rewrite Go
// microservice. Per clean architecture, this is the ONLY place where
// concrete adapters get wired into use cases. Everything else stays
// pluggable behind the ports in internal/domain.
//
// Task 2 state: HTTP server with /health (public) + /rewrite (JWT-protected).
// Real rewrite pipeline (SSE + provider call + quota) lands in Task 7.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	internalhttp "github.com/tannpv/draftright-rewrite/internal/http"
	authpkg "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/platform/config"
)

const (
	// Generous deadlines for Tasks 2-6; tightened in Task 7 once SSE
	// streaming is wired (write deadline becomes per-request).
	readTimeout  = 10 * time.Second
	writeTimeout = 60 * time.Second
	idleTimeout  = 120 * time.Second
	// Graceful shutdown window — long enough for in-flight SSE streams
	// (Task 7) to finish a token, short enough that prod redeploys
	// don't drag.
	shutdownTimeout = 15 * time.Second
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// We can't use slog here yet — logger needs the level from
		// config. Fall back to stderr + exit 1 with a clear message
		// so a misconfigured deploy fails loudly.
		_, _ = os.Stderr.WriteString("FATAL: " + err.Error() + "\n")
		os.Exit(2)
	}

	log := newLogger(cfg.LogLevel)
	verifier := authpkg.NewVerifier(cfg.JWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	// /rewrite now requires a valid JWT. RequireAuth wraps the
	// stub handler so a real handler in Task 7 plugs in without
	// touching the wiring.
	mux.Handle("/rewrite", internalhttp.RequireAuth(verifier, log)(http.HandlerFunc(handleRewriteStub)))

	srv := &http.Server{
		Addr:         cfg.Listen,
		Handler:      withRequestLogging(log, mux),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	// Run the server on a goroutine so the main goroutine can listen
	// for signals + drive graceful shutdown. Without this split,
	// SIGTERM would either be ignored or kill in-flight requests
	// mid-stream.
	go func() {
		log.Info("listening", "addr", cfg.Listen, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server crashed", "err", err)
			os.Exit(1)
		}
	}()

	// Wait for SIGINT / SIGTERM. Docker stops containers with SIGTERM;
	// Ctrl-C from a dev terminal sends SIGINT. Catch both.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	log.Info("shutdown signal received; draining connections")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	log.Info("shutdown complete")
}

// newLogger returns a JSON-output slog suitable for prod log aggregation
// (Loki/CloudWatch/etc.) Level threshold parsed from config.LogLevel.
//
// Kept in main.go for Task 2 simplicity; moves to
// internal/platform/logger/ once a second consumer exists.
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

// withRequestLogging wraps the handler with one structured log line per
// request. Replaced by the chi middleware chain in Task 7 (adds
// correlation-id, recover-on-panic, and Prometheus metrics).
func withRequestLogging(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

// statusRecorder captures the response status code so the logging
// middleware can record it.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// handleHealth is the kubelet/Docker-healthcheck probe target.
// Returns 200 unconditionally for Task 2; will check Postgres + Redis
// + AI-provider reachability in a future task.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "rewrite-go",
	})
}

// handleRewriteStub returns the verified user id from the JWT, proving
// the auth middleware works end-to-end. Real SSE pipeline lands in Task 7.
func handleRewriteStub(w http.ResponseWriter, r *http.Request) {
	claims, ok := internalhttp.ClaimsFromContext(r.Context())
	if !ok {
		// Middleware misconfigured — route wrapped at registration
		// time, so this branch only fires on a programmer error.
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "auth middleware missing",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"text":    "Hello from Go!",
		"tone":    "placeholder",
		"service": "rewrite-go",
		"user_id": claims.UserID(),
		"role":    claims.Role,
		"note":    "Task 2 — JWT verified. Real SSE pipeline lands in Task 7.",
	})
}

// writeJSON encodes a value as JSON with the correct headers.
// Centralised so future handlers don't repeat the boilerplate (Rule #1).
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("write json failed", "err", err)
	}
}
