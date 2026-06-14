package subscription

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// WriteQuerier is the sqlc subset the writer needs.
type WriteQuerier interface {
	CreateFreeSubscription(ctx context.Context, arg sqlc.CreateFreeSubscriptionParams) error
}

// Writer creates subscription rows. Phase 1b: the free grant on signup.
// Paid grants + lifecycle land in Phase 2 on this same seam.
type Writer struct{ q WriteQuerier }

// NewWriter wires the querier.
func NewWriter(q WriteQuerier) *Writer { return &Writer{q: q} }

// CreateFree inserts an ACTIVE admin_granted free subscription
// (started_at=now, expires_at=null), mirroring
// subscriptionsService.createFreeSubscription.
func (w *Writer) CreateFree(ctx context.Context, userID, planID string) error {
	var uid, pid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return err
	}
	if err := pid.Scan(planID); err != nil {
		return err
	}
	return w.q.CreateFreeSubscription(ctx, sqlc.CreateFreeSubscriptionParams{UserID: uid, PlanID: pid})
}
