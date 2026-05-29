// Package main is the composition root for the DraftRight /rewrite Go
// microservice. Per clean architecture, this is the ONLY place where
// concrete adapters get wired into use cases. Everything else stays
// pluggable behind the ports in internal/domain.
//
// Task 1 scaffold: HTTP server with /health + /rewrite stubs only.
// Real adapters land in Tasks 4-7 (see README + plan).
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
)

const (
	// Listen address. Override via LISTEN_ADDR env when the prod
	// container is also serving on 3001 already — but Task 1 ships with
	// a fixed default so the quick-start instructions in README work
	// without extra env wiring.
	defaultListenAddr = ":3001"

	// Generous deadlines for Task 1; tightened in Task 7 once SSE
	// streaming is wired (write deadline becomes per-request).
	readTimeout  = 10 * time.Second
	writeTimeout = 60 * time.Second
	idleTimeout  = 120 * time.Second
	// Graceful shutdown window — long enough for SSE streams in flight
	// (Task 7) to finish a token, short enough that prod redeploys
	// don't drag.
	shutdownTimeout = 15 * time.Second
)

func main() {
	log := newLogger()
	addr := envOr("LISTEN_ADDR", defaultListenAddr)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/rewrite", handleRewriteStub)

	srv := &http.Server{
		Addr:         addr,
		Handler:      withRequestLogging(log, mux),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	// Run the server on a goroutine so the main goroutine can listen for
	// signals + drive graceful shutdown. Without this split, SIGTERM
	// would either be ignored or kill in-flight requests mid-stream.
	go func() {
		log.Info("listening", "addr", addr)
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
// (Loki/CloudWatch/etc.) Level defaults to INFO; override via LOG_LEVEL.
//
// Kept in main.go for Task 1 simplicity; moves to
// internal/platform/logger/ in Task 0.2-equivalent (see plan Task 2).
func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch os.Getenv("LOG_LEVEL") {
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
// request (method, path, status, duration). Kept minimal for Task 1;
// gets replaced by a proper middleware chain in Task 7 (chi router) with
// correlation-id, recover-on-panic, and Prometheus metrics added.
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
// middleware can record it. Standard pattern — http.ResponseWriter
// itself doesn't expose the status after WriteHeader is called.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// handleHealth is the kubelet/Docker-healthcheck probe target.
// Returns 200 unconditionally for Task 1; will check Postgres + Redis
// + AI-provider reachability in a future task.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "rewrite-go",
	})
}

// handleRewriteStub returns a placeholder until Task 7 wires the real
// SSE streaming pipeline. Documented so anyone hitting it during the
// scaffold phase doesn't think prod is broken.
func handleRewriteStub(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"text":    "Hello from Go!",
		"tone":    "placeholder",
		"service": "rewrite-go",
		"note":    "Task 1 scaffold — real rewrite pipeline lands in Task 7. See README.",
	})
}

// writeJSON encodes a value as JSON with the correct headers. Centralised
// so future handlers don't repeat the boilerplate (Rule #1 — reusable).
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// Last-resort log; the response is already partially written so
		// we can't change the status anymore.
		slog.Error("write json failed", "err", err)
	}
}

// envOr returns the env value or the fallback when unset/empty.
// Will move to internal/platform/config in Task 2.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
