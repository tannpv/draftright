// Package usage is the read-only daily-usage view for /auth/account.
// The full usage module (logging, aggregation) is folded in at Phase 1d;
// this counter is the seam.
package usage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Querier is the one-method sqlc subset.
type Querier interface {
	CountUsageToday(ctx context.Context, arg sqlc.CountUsageTodayParams) (int64, error)
}

// Counter counts a user's rewrites since local midnight.
type Counter struct{ q Querier }

// NewCounter wires the querier.
func NewCounter(q Querier) *Counter { return &Counter{q: q} }

// CountToday mirrors Node usageService.countTodayByUser: rows with
// created_at >= local midnight (server timezone, matching new Date();
// setHours(0,0,0,0)).
func (c *Counter) CountToday(ctx context.Context, userID string) (int, error) {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return 0, nil
	}
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	n, err := c.q.CountUsageToday(ctx, sqlc.CountUsageTodayParams{
		UserID:    uid,
		CreatedAt: pgtype.Timestamp{Time: midnight, Valid: true},
	})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
