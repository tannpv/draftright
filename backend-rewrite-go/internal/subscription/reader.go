// Package subscription is the read-only entitlement view needed by
// /auth/account in Phase 1a. Subscription WRITES + expiry cron land in
// Phase 2; this reader is the seam they'll extend.
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

// AccountSub is the subscription shape /auth/account serializes.
// ExpiresAt is nil when the row's expires_at is NULL.
type AccountSub struct {
	PlanName           string
	Status             string
	StoreType          string
	StoreTransactionID string // "" when NULL — needed by in-app cancel
	StartedAt          time.Time
	ExpiresAt          *time.Time
	DailyLimit         int
}

// PlanBreakdown is one row returned by GetPlansBreakdown.
// PlanName is "" and PriceCents is 0 when the plan row is missing (LEFT JOIN NULL).
type PlanBreakdown struct {
	PlanName    string `json:"plan_name"`
	ActiveCount int    `json:"active_count"`
	PriceCents  int    `json:"price_cents"`
}

// Querier is the sqlc subset this reader needs.
type Querier interface {
	GetActiveSubscriptionByUserID(ctx context.Context, id pgtype.UUID) (sqlc.GetActiveSubscriptionByUserIDRow, error)
	GetActiveSubWithPlan(ctx context.Context, userID pgtype.UUID) (sqlc.GetActiveSubWithPlanRow, error)
	GetLastExpiredAt(ctx context.Context, userID pgtype.UUID) (pgtype.Timestamp, error)
	SubsDueForRenewal(ctx context.Context, arg sqlc.SubsDueForRenewalParams) ([]sqlc.SubsDueForRenewalRow, error)
	ExpireLapsedSubs(ctx context.Context, expiresAt pgtype.Timestamp) ([]sqlc.ExpireLapsedSubsRow, error)
	CountActiveSubscriptions(ctx context.Context) (int64, error)
	PlansBreakdown(ctx context.Context) ([]sqlc.PlansBreakdownRow, error)
}

// Reader resolves the active subscription.
type Reader struct{ q Querier }

// NewReader wires the querier.
func NewReader(q Querier) *Reader { return &Reader{q: q} }

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

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
		PlanName:           row.PlanName,
		Status:             string(row.Status),
		StoreType:          string(row.StoreType),
		StoreTransactionID: derefStr(row.StoreTransactionID),
		StartedAt:          row.StartedAt.Time,
		DailyLimit:         int(row.DailyLimit),
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

// DueForRenewal implements CronRepo pass 1: active subs whose expiry falls in
// [lo, hi]. Bounds are caller-computed (now+2.5d / now+3.5d) so timezone math
// stays in the Go process.
func (r *Reader) DueForRenewal(ctx context.Context, lo, hi time.Time) ([]RenewalRow, error) {
	rows, err := r.q.SubsDueForRenewal(ctx, sqlc.SubsDueForRenewalParams{
		ExpiresAt:   pgtype.Timestamp{Time: lo, Valid: true},
		ExpiresAt_2: pgtype.Timestamp{Time: hi, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	out := make([]RenewalRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, RenewalRow{
			UserID:    uuid.UUID(row.UserID.Bytes).String(),
			Email:     row.Email,
			PlanName:  row.PlanName,
			ExpiresAt: row.ExpiresAt.Time,
		})
	}
	return out, nil
}

// ExpireLapsed implements CronRepo pass 2: flip active|cancelled subs past
// expiry to expired (one bulk UPDATE), returning the affected rows.
func (r *Reader) ExpireLapsed(ctx context.Context, now time.Time) ([]ExpiredRow, error) {
	rows, err := r.q.ExpireLapsedSubs(ctx, pgtype.Timestamp{Time: now, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]ExpiredRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ExpiredRow{
			UserID:    uuid.UUID(row.UserID.Bytes).String(),
			Email:     row.Email,
			ExpiresAt: row.ExpiresAt.Time,
		})
	}
	return out, nil
}

// CountActive mirrors subscriptionsService.countActive(): total active subscriptions.
func (r *Reader) CountActive(ctx context.Context) (int, error) {
	n, err := r.q.CountActiveSubscriptions(ctx)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// derefInt32 returns 0 when p is nil.
func derefInt32(p *int32) int {
	if p == nil {
		return 0
	}
	return int(*p)
}

// GetPlansBreakdown mirrors subscriptionsService.getPlansBreakdown(): active subs
// grouped by plan, returning plan name, price, and count. Empty DB result → non-nil
// empty slice. NULL plan_name/price_cents (orphaned sub) maps to "" / 0.
func (r *Reader) GetPlansBreakdown(ctx context.Context) ([]PlanBreakdown, error) {
	rows, err := r.q.PlansBreakdown(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlanBreakdown, 0, len(rows))
	for _, row := range rows {
		out = append(out, PlanBreakdown{
			PlanName:    derefStr(row.PlanName),
			ActiveCount: int(row.ActiveCount),
			PriceCents:  derefInt32(row.PriceCents),
		})
	}
	return out, nil
}
