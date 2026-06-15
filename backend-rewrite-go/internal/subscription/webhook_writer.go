package subscription

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// StoreRefSub is the FindByStoreRef projection a webhook handler needs to email
// the affected user (LS payment-failed).
type StoreRefSub struct {
	UserID    string
	UserEmail string
	UserName  string
	PlanName  string
}

// WebhookQuerier is the sqlc subset the webhook writer needs.
type WebhookQuerier interface {
	CancelActiveSubsByUser(ctx context.Context, id pgtype.UUID) error
	InsertGrantedSubscription(ctx context.Context, arg sqlc.InsertGrantedSubscriptionParams) error
	StampStoreRefByReference(ctx context.Context, arg sqlc.StampStoreRefByReferenceParams) (int64, error)
	ExtendByStoreRef(ctx context.Context, arg sqlc.ExtendByStoreRefParams) (int64, error)
	CancelByStoreRef(ctx context.Context, arg sqlc.CancelByStoreRefParams) (int64, error)
	ExpireByStoreRef(ctx context.Context, arg sqlc.ExpireByStoreRefParams) (int64, error)
	FindByStoreRef(ctx context.Context, arg sqlc.FindByStoreRefParams) (sqlc.FindByStoreRefRow, error)
}

// WebhookWriter mutates subscriptions on provider webhook events. Ports the
// store-ref helpers in subscriptions.service.ts.
type WebhookWriter struct{ q WebhookQuerier }

// NewWebhookWriter wires the querier.
func NewWebhookWriter(q WebhookQuerier) *WebhookWriter { return &WebhookWriter{q: q} }

// Grant cancels any prior active subscription for the user then inserts a new
// active one (ports SubscriptionsService.grant). expiresAt nil → NULL.
func (w *WebhookWriter) Grant(ctx context.Context, userID, planID, storeType string, expiresAt *time.Time) error {
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}
	pid, err := parseUUID(planID)
	if err != nil {
		return err
	}
	if err := w.q.CancelActiveSubsByUser(ctx, uid); err != nil {
		return err
	}
	var exp pgtype.Timestamp
	if expiresAt != nil {
		exp = pgtype.Timestamp{Time: *expiresAt, Valid: true}
	}
	return w.q.InsertGrantedSubscription(ctx, sqlc.InsertGrantedSubscriptionParams{
		UserID:    uid,
		PlanID:    pid,
		StoreType: sqlc.SubscriptionsStoreTypeEnum(storeType),
		ExpiresAt: exp,
	})
}

// StampStoreRef writes (store_type, store_transaction_id) onto the most-recent
// active sub of the user who paid referenceCode. No match → no-op.
func (w *WebhookWriter) StampStoreRef(ctx context.Context, referenceCode, storeType, transactionID string) error {
	_, err := w.q.StampStoreRefByReference(ctx, sqlc.StampStoreRefByReferenceParams{
		ReferenceCode:      referenceCode,
		StoreType:          sqlc.SubscriptionsStoreTypeEnum(storeType),
		StoreTransactionID: &transactionID,
	})
	return err
}

// ExtendByStoreRef bumps expires_at on a renewal. Returns rows affected.
func (w *WebhookWriter) ExtendByStoreRef(ctx context.Context, storeType, transactionID string, expiresAt time.Time) (int64, error) {
	return w.q.ExtendByStoreRef(ctx, sqlc.ExtendByStoreRefParams{
		StoreType:          sqlc.SubscriptionsStoreTypeEnum(storeType),
		StoreTransactionID: &transactionID,
		ExpiresAt:          pgtype.Timestamp{Time: expiresAt, Valid: true},
	})
}

// CancelByStoreRef marks the matching active sub cancelled. Returns rows affected.
func (w *WebhookWriter) CancelByStoreRef(ctx context.Context, storeType, transactionID string) (int64, error) {
	return w.q.CancelByStoreRef(ctx, sqlc.CancelByStoreRefParams{
		StoreType: sqlc.SubscriptionsStoreTypeEnum(storeType), StoreTransactionID: &transactionID,
	})
}

// ExpireByStoreRef revokes access now. Returns rows affected.
func (w *WebhookWriter) ExpireByStoreRef(ctx context.Context, storeType, transactionID string) (int64, error) {
	return w.q.ExpireByStoreRef(ctx, sqlc.ExpireByStoreRefParams{
		StoreType: sqlc.SubscriptionsStoreTypeEnum(storeType), StoreTransactionID: &transactionID,
	})
}

// FindByStoreRef loads the sub + user + plan projection, or (nil,nil) when none.
func (w *WebhookWriter) FindByStoreRef(ctx context.Context, storeType, transactionID string) (*StoreRefSub, error) {
	row, err := w.q.FindByStoreRef(ctx, sqlc.FindByStoreRefParams{
		StoreType: sqlc.SubscriptionsStoreTypeEnum(storeType), StoreTransactionID: &transactionID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &StoreRefSub{
		UserID:    uuid.UUID(row.UserID.Bytes).String(),
		UserEmail: row.UserEmail,
		UserName:  row.UserName,
		PlanName:  row.PlanName,
	}, nil
}

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}
