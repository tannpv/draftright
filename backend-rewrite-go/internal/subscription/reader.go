// Package subscription is the read-only entitlement view needed by
// /auth/account in Phase 1a. Subscription WRITES + expiry cron land in
// Phase 2; this reader is the seam they'll extend.
package subscription

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// AccountSub is the subscription shape /auth/account serializes.
// ExpiresAt is nil when the row's expires_at is NULL.
type AccountSub struct {
	PlanName   string
	Status     string
	StoreType  string
	StartedAt  time.Time
	ExpiresAt  *time.Time
	DailyLimit int
}

// Querier is the sqlc subset this reader needs.
type Querier interface {
	GetActiveSubscriptionByUserID(ctx context.Context, id pgtype.UUID) (sqlc.GetActiveSubscriptionByUserIDRow, error)
	GetActiveSubWithPlan(ctx context.Context, userID pgtype.UUID) (sqlc.GetActiveSubWithPlanRow, error)
	GetLastExpiredAt(ctx context.Context, userID pgtype.UUID) (pgtype.Timestamp, error)
}

// Reader resolves the active subscription.
type Reader struct{ q Querier }

// NewReader wires the querier.
func NewReader(q Querier) *Reader { return &Reader{q: q} }

// ActiveByUser returns the newest active subscription, or (nil, nil)
// when the user has none.
func (r *Reader) ActiveByUser(ctx context.Context, userID string) (*AccountSub, error) {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return nil, nil // malformed id → no sub (account still renders)
	}
	row, err := r.q.GetActiveSubscriptionByUserID(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := &AccountSub{
		PlanName:   row.PlanName,
		Status:     string(row.Status),
		StoreType:  string(row.StoreType),
		StartedAt:  row.StartedAt.Time,
		DailyLimit: int(row.DailyLimit),
	}
	if row.ExpiresAt.Valid {
		t := row.ExpiresAt.Time
		s.ExpiresAt = &t
	}
	return s, nil
}

// SubView is the active-sub projection GET /subscription needs.
type SubView struct {
	Status        string
	ExpiresAt     *time.Time
	PlanName      string
	DailyLimit    int
	BillingPeriod string
}

// ActiveWithPlan returns the newest active sub + plan, or (nil,nil) when none.
func (r *Reader) ActiveWithPlan(ctx context.Context, userID string) (*SubView, error) {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return nil, nil
	}
	row, err := r.q.GetActiveSubWithPlan(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v := &SubView{
		Status:        string(row.Status),
		PlanName:      row.PlanName,
		DailyLimit:    int(row.DailyLimit),
		BillingPeriod: string(row.BillingPeriod),
	}
	if row.ExpiresAt.Valid {
		t := row.ExpiresAt.Time
		v.ExpiresAt = &t
	}
	return v, nil
}

// LastExpiredAt returns updated_at of the newest expired sub, or (nil,nil).
func (r *Reader) LastExpiredAt(ctx context.Context, userID string) (*time.Time, error) {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return nil, nil
	}
	ts, err := r.q.GetLastExpiredAt(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !ts.Valid {
		return nil, nil
	}
	t := ts.Time
	return &t, nil
}
