// Package core hosts the Phase 0 proof endpoints (GET /health,
// GET /auth/me). These are deliberately thin — they exist to exercise
// the foundation (config, DB, JWT, envelope, shadow-diff) end to end.
// /auth/me moves into a real internal/auth module in Phase 1.
package core

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// LogLevelReader is the seam over app_settings.client_log_level. The
// Postgres implementation lives in this package (adapter on the shared
// sqlc Queries); tests pass a stub. Interface, not a concrete, so the
// handler is unit-testable without a database (Rule #1).
type LogLevelReader interface {
	ClientLogLevel(ctx context.Context) (string, error)
}

// HealthHandler serves GET /health with the same body shape as the
// Node backend: { app, version, status, client_log_level }. The log
// level is cached briefly and DB-tolerant — a DB hiccup never makes
// the liveness probe fail.
type HealthHandler struct {
	reader  LogLevelReader
	version string

	mu       sync.Mutex
	cached   string
	cachedAt time.Time
}

const (
	healthApp        = "draftright"
	defaultLogLevel  = "info"
	logLevelCacheTTL = 30 * time.Second
)

// NewHealthHandler wires the log-level source + the version string the
// endpoint reports (mirror the Node "2.0.0").
func NewHealthHandler(reader LogLevelReader, version string) *HealthHandler {
	return &HealthHandler{reader: reader, version: version, cached: defaultLogLevel}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	shared.WriteJSON(w, http.StatusOK, map[string]string{
		"app":              healthApp,
		"version":          h.version,
		"status":           "ok",
		"client_log_level": h.clientLogLevel(r.Context()),
	})
}

// clientLogLevel returns the cached level, refreshing at most every
// TTL. On a read error it keeps serving the last-known (or default)
// level — identical tolerance to the Node controller.
func (h *HealthHandler) clientLogLevel(ctx context.Context) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.cachedAt.IsZero() && time.Since(h.cachedAt) < logLevelCacheTTL && h.cached != "" {
		return h.cached
	}
	level, err := h.reader.ClientLogLevel(ctx)
	if err != nil || level == "" {
		if h.cached == "" {
			h.cached = defaultLogLevel
		}
		// keep h.cached as-is (last known); refresh the timer so we
		// don't hammer a struggling DB on every probe
		h.cachedAt = time.Now()
		return h.cached
	}
	h.cached = level
	h.cachedAt = time.Now()
	return level
}
