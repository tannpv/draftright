package core

import (
	"context"

	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// pgLogLevel reads app_settings.client_log_level via the shared sqlc
// Queries. Returns ("", err) on db error so the handler falls back to
// its cached/default level.
type pgLogLevel struct{ q *sqlc.Queries }

// NewPgLogLevel adapts a sqlc.Queries (built from the shared pool) to
// the LogLevelReader seam.
func NewPgLogLevel(q *sqlc.Queries) LogLevelReader { return pgLogLevel{q: q} }

func (p pgLogLevel) ClientLogLevel(ctx context.Context) (string, error) {
	return p.q.GetClientLogLevel(ctx)
}
