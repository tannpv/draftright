// Package settings reads admin-configurable app_settings values that
// other modules need at request time. Read-only in Phase 1a; the admin
// module (Phase 4) owns writes.
package settings

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Default token lifetimes — applied when the app_settings row is absent,
// matching Node's `settings?.token_expiry_minutes ?? 15` / `?? 90`.
const (
	defaultAccessMinutes = 15
	defaultRefreshDays   = 90
)

// Querier is the sqlc subset this reader needs (one method) — narrow
// interface so tests fake it without a DB.
type Querier interface {
	GetAuthTokenSettings(ctx context.Context) (sqlc.GetAuthTokenSettingsRow, error)
}

// Reader resolves token TTLs from app_settings.
type Reader struct{ q Querier }

// NewReader wires the sqlc querier (or any Querier).
func NewReader(q Querier) *Reader { return &Reader{q: q} }

// TokenTTLs returns (accessTTL, refreshTTL). Missing row → defaults.
func (r *Reader) TokenTTLs(ctx context.Context) (time.Duration, time.Duration, error) {
	row, err := r.q.GetAuthTokenSettings(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultAccessMinutes * time.Minute, defaultRefreshDays * 24 * time.Hour, nil
	}
	if err != nil {
		return 0, 0, err
	}
	accMin := int(row.TokenExpiryMinutes)
	if accMin <= 0 {
		accMin = defaultAccessMinutes
	}
	refDays := int(row.RefreshTokenExpiryDays)
	if refDays <= 0 {
		refDays = defaultRefreshDays
	}
	return time.Duration(accMin) * time.Minute, time.Duration(refDays) * 24 * time.Hour, nil
}
