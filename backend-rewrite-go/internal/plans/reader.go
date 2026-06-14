// Package plans reads plan rows. Phase 1b needs only the free plan
// (assigned on register + social signup). Plan WRITES are admin (Phase 4).
package plans

import (
	"context"

	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Plan is the trimmed projection callers need.
type Plan struct {
	ID         string
	Name       string
	DailyLimit int
}

// Querier is the one-method sqlc subset.
type Querier interface {
	FindFreePlan(ctx context.Context) (sqlc.FindFreePlanRow, error)
}

// Reader resolves plans.
type Reader struct{ q Querier }

// NewReader wires the querier.
func NewReader(q Querier) *Reader { return &Reader{q: q} }

// FindFreePlan returns the active billing_period='none' plan. Node throws
// "Free plan not found. Run seed first." when absent; here pgx.ErrNoRows
// propagates → 500 (seed is a deploy invariant).
func (r *Reader) FindFreePlan(ctx context.Context) (Plan, error) {
	row, err := r.q.FindFreePlan(ctx)
	if err != nil {
		return Plan{}, err
	}
	// row.ID.String() == user.uuidStr: canonical lowercase hyphenated UUID,
	// round-trips through pgtype.UUID.Scan (subscription.CreateFree reparses it).
	return Plan{ID: row.ID.String(), Name: row.Name, DailyLimit: int(row.DailyLimit)}, nil
}
