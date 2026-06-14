// Package plans reads plan rows. Phase 1b needs only the free plan
// (assigned on register + social signup). Plan WRITES are admin (Phase 4).
package plans

import (
	"context"

	"github.com/google/uuid"

	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Plan is the trimmed projection callers need.
type Plan struct {
	ID         string
	Name       string
	DailyLimit int
}

// Querier is the sqlc subset the reader needs.
type Querier interface {
	FindFreePlan(ctx context.Context) (sqlc.FindFreePlanRow, error)
	ListActivePlans(ctx context.Context) ([]sqlc.ListActivePlansRow, error)
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

// ListActive returns every active plan, cheapest first, as raw entities
// for GET /plans. Empty → non-nil empty slice (serialises []).
func (r *Reader) ListActive(ctx context.Context) ([]PlanEntity, error) {
	rows, err := r.q.ListActivePlans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlanEntity, 0, len(rows))
	for _, row := range rows {
		out = append(out, PlanEntity{
			ID:            uuid.UUID(row.ID.Bytes).String(),
			Name:          row.Name,
			DailyLimit:    int(row.DailyLimit),
			PriceCents:    int(row.PriceCents),
			Currency:      row.Currency,
			StripePriceID: row.StripePriceID,
			TrialDays:     int(row.TrialDays),
			BillingPeriod: string(row.BillingPeriod),
			IsActive:      row.IsActive,
			CreatedAt:     row.CreatedAt.Time,
			UpdatedAt:     row.UpdatedAt.Time,
		})
	}
	return out, nil
}
