package subscription

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// WriteQuerier is the sqlc subset the writer needs.
type WriteQuerier interface {
	CreateFreeSubscription(ctx context.Context, arg sqlc.CreateFreeSubscriptionParams) error
	CancelActiveSubsByUser(ctx context.Context, id pgtype.UUID) error
	InsertGrantedSubscription(ctx context.Context, arg sqlc.InsertGrantedSubscriptionParams) (sqlc.Subscription, error)
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

// Grant cancels any prior active subscription for the user then inserts a new
// active admin_granted one, returning the saved row. Mirrors
// subscriptionsService.grant (expiresAt nil → NULL). started_at = now() in SQL.
func (w *Writer) Grant(ctx context.Context, userID, planID string, expiresAt *time.Time) (GrantedSub, error) {
	var uid, pid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return GrantedSub{}, err
	}
	if err := pid.Scan(planID); err != nil {
		return GrantedSub{}, err
	}
	if err := w.q.CancelActiveSubsByUser(ctx, uid); err != nil {
		return GrantedSub{}, err
	}
	var exp pgtype.Timestamp
	if expiresAt != nil {
		exp = pgtype.Timestamp{Time: *expiresAt, Valid: true}
	}
	row, err := w.q.InsertGrantedSubscription(ctx, sqlc.InsertGrantedSubscriptionParams{
		UserID:    uid,
		PlanID:    pid,
		StoreType: sqlc.SubscriptionsStoreTypeEnumAdminGranted,
		ExpiresAt: exp,
	})
	if err != nil {
		return GrantedSub{}, err
	}
	return GrantedSub{
		ID:                 uuid.UUID(row.ID.Bytes).String(),
		UserID:             uuid.UUID(row.UserID.Bytes).String(),
		PlanID:             uuid.UUID(row.PlanID.Bytes).String(),
		Status:             string(row.Status),
		StoreType:          string(row.StoreType),
		StoreTransactionID: row.StoreTransactionID,
		StartedAt:          row.StartedAt.Time,
		ExpiresAt:          toTimePtr(row.ExpiresAt),
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}, nil
}
