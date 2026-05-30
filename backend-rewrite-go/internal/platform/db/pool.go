// Package db owns the Postgres connection pool + the bridge from
// pgxpool to the sqlc-generated `Queries` type. Every adapter that
// talks to Postgres goes through these helpers (Rule #1 — one place
// owns connection settings, retries, and the timezone clamp).
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Sensible defaults for a single rewrite-go instance. Pool size of 25
// matches NestJS TypeORM default (10) + 15 headroom for SSE bursts;
// shared Postgres needs the two services to keep their pools' sum
// under postgres.max_connections (typically 100).
const (
	defaultMaxConns = 25
	defaultMinConns = 5
	// 30s is enough for steady-state Postgres + most contended cases;
	// shorter timeouts cause spurious 5xxs under burst load.
	defaultConnectTimeout = 30 * time.Second
	// Postgres kills idle connections after server-side configured
	// limits; we recycle proactively so we never hand a dead conn to
	// a query.
	defaultMaxConnLifetime = 30 * time.Minute
	defaultMaxConnIdleTime = 5 * time.Minute
)

// NewPool returns a configured pgxpool ready for queries. Caller owns
// the *pgxpool.Pool — call .Close() at shutdown to drain in-flight
// queries cleanly.
//
// Wraps pgxpool.New() with our defaults so individual adapters don't
// re-tune the same knobs (Rule #1 — reusable infra config).
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, errors.New("db: DATABASE_URL not set")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("db: parse DSN: %w", err)
	}
	cfg.MaxConns = defaultMaxConns
	cfg.MinConns = defaultMinConns
	cfg.MaxConnLifetime = defaultMaxConnLifetime
	cfg.MaxConnIdleTime = defaultMaxConnIdleTime

	// ApplicationName tags every session in pg_stat_activity so we can
	// distinguish rewrite-go connections from NestJS connections + audit
	// + kill rogue queries surgically.
	cfg.ConnConfig.RuntimeParams["application_name"] = "draftright-rewrite-go"

	// Lock to UTC so any timestamp arithmetic in queries is unambiguous.
	// NestJS also stores TIMESTAMP WITHOUT TIME ZONE so this matters.
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"

	connectCtx, cancel := context.WithTimeout(ctx, defaultConnectTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(connectCtx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: connect pool: %w", err)
	}
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping after connect: %w", err)
	}
	return pool, nil
}
