// Package subscription is the read-only entitlement view needed by
// /auth/account in Phase 1a. Subscription WRITES + expiry cron land in
// Phase 2; this reader is the seam they'll extend.
package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tannpv/draftright-rewrite/internal/shared"
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
	CountNewSubsInWindow(ctx context.Context, arg sqlc.CountNewSubsInWindowParams) (int64, error)
	SumRevenueInWindow(ctx context.Context, arg sqlc.SumRevenueInWindowParams) (int64, error)
	CountChurnedInWindow(ctx context.Context, arg sqlc.CountChurnedInWindowParams) (int64, error)
	ListSubscriptionsPaginated(ctx context.Context, arg sqlc.ListSubscriptionsPaginatedParams) ([]sqlc.ListSubscriptionsPaginatedRow, error)
	CountSubscriptionsPaginated(ctx context.Context, search *string) (int64, error)
	FindLatestStripeForUserPlan(ctx context.Context, arg sqlc.FindLatestStripeForUserPlanParams) (sqlc.FindLatestStripeForUserPlanRow, error)
}

// ─── Admin transactions list types (Phase 4c-3) ──────────────────────────────

// TxQuery is the input for FindAllPaginated. Page is 1-based; Limit defaults
// to 20 (bespoke). An empty Search passes NULL to the DB (matches all rows).
type TxQuery struct {
	Search string
	Page   int
	Limit  int
}

// TransactionRow is one row of the admin transactions list. It mirrors Node's
// listTransactions mapped object exactly:
//   - NULL user email/name/plan_name → "—" (U+2014, matching Node's `|| '—'`)
//   - NULL price_cents → 0
//   - store_transaction_id: pass-through (nullable → JSON null, not "—")
//   - timestamps: ISOMillis format (ms precision, trailing Z), expires_at is nullable
type TransactionRow struct {
	ID                 string
	UserEmail          string
	UserName           string
	UserID             string
	PlanName           string
	PriceCents         int
	StoreType          string
	StoreTransactionID *string
	Status             string
	StartedAt          *time.Time
	ExpiresAt          *time.Time
	CreatedAt          *time.Time
}

// MarshalJSON pins TransactionRow to Node's listTransactions field order and
// timestamp format (ISOMillis). expires_at is null when nil.
func (t TransactionRow) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID                 string  `json:"id"`
		UserEmail          string  `json:"user_email"`
		UserName           string  `json:"user_name"`
		UserID             string  `json:"user_id"`
		PlanName           string  `json:"plan_name"`
		PriceCents         int     `json:"price_cents"`
		StoreType          string  `json:"store_type"`
		StoreTransactionID *string `json:"store_transaction_id"`
		Status             string  `json:"status"`
		StartedAt          *string `json:"started_at"`
		ExpiresAt          *string `json:"expires_at"`
		CreatedAt          *string `json:"created_at"`
	}
	w := wire{
		ID:                 t.ID,
		UserEmail:          t.UserEmail,
		UserName:           t.UserName,
		UserID:             t.UserID,
		PlanName:           t.PlanName,
		PriceCents:         t.PriceCents,
		StoreType:          t.StoreType,
		StoreTransactionID: t.StoreTransactionID,
		Status:             t.Status,
	}
	if t.StartedAt != nil {
		s := shared.ISOMillis(*t.StartedAt)
		w.StartedAt = &s
	}
	if t.ExpiresAt != nil {
		s := shared.ISOMillis(*t.ExpiresAt)
		w.ExpiresAt = &s
	}
	if t.CreatedAt != nil {
		s := shared.ISOMillis(*t.CreatedAt)
		w.CreatedAt = &s
	}
	return json.Marshal(w)
}

// TxPage is the paginated result for FindAllPaginated.
type TxPage struct {
	Transactions []TransactionRow
	Total        int
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

// MonthStat is one month bucket for GET /admin/analytics monthly_stats.
// JSON keys match Node getMonthlyStats exactly: month, new_subscriptions,
// revenue_cents, churned.
type MonthStat struct {
	Month            string `json:"month"`
	NewSubscriptions int    `json:"new_subscriptions"`
	RevenueCents     int    `json:"revenue_cents"`
	Churned          int    `json:"churned"`
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

// GetMonthlyStatsAt mirrors Node subscriptionsService.getMonthlyStats(months):
// returns `months` entries oldest→newest, each covering one calendar month
// [mStart, mEnd) in now's timezone.
//
// Node loop: for i = months-1; i >= 0; i--
//
//	monthStart = new Date(year, month-i, 1)   ← JS 0-based month
//	monthEnd   = new Date(year, month-i+1, 1)
//
// time.Date normalizes month underflow/overflow identically (e.g. month 0
// → December of prior year), so Go mirrors JS new Date(y, m, 1) exactly.
func (r *Reader) GetMonthlyStatsAt(ctx context.Context, now time.Time, months int) ([]MonthStat, error) {
	out := make([]MonthStat, 0, months)
	for i := months - 1; i >= 0; i-- {
		// JS: new Date(now.getFullYear(), now.getMonth()-i, 1)
		// Go Month() is 1-based; JS getMonth() is 0-based. The difference cancels:
		// Month(6)-0 == month(5)+1 == June. time.Date normalizes underflow
		// identically to JS new Date, so going back i months is just Month()-i.
		mStart := time.Date(now.Year(), time.Month(int(now.Month())-i), 1, 0, 0, 0, 0, now.Location())
		mEnd := time.Date(mStart.Year(), mStart.Month()+1, 1, 0, 0, 0, 0, now.Location())

		mStartTS := pgtype.Timestamp{Time: mStart, Valid: true}
		mEndTS := pgtype.Timestamp{Time: mEnd, Valid: true}

		newSubs, err := r.q.CountNewSubsInWindow(ctx, sqlc.CountNewSubsInWindowParams{
			StartedAt:   mStartTS,
			StartedAt_2: mEndTS,
		})
		if err != nil {
			return nil, err
		}

		revenue, err := r.q.SumRevenueInWindow(ctx, sqlc.SumRevenueInWindowParams{
			StartedAt:   mStartTS,
			StartedAt_2: mEndTS,
		})
		if err != nil {
			return nil, err
		}

		churned, err := r.q.CountChurnedInWindow(ctx, sqlc.CountChurnedInWindowParams{
			UpdatedAt:   mStartTS,
			UpdatedAt_2: mEndTS,
		})
		if err != nil {
			return nil, err
		}

		out = append(out, MonthStat{
			Month:            mStart.Format("2006-01"),
			NewSubscriptions: int(newSubs),
			RevenueCents:     int(revenue),
			Churned:          int(churned),
		})
	}
	return out, nil
}

// ─── FindAllPaginated (Phase 4c-3) ───────────────────────────────────────────

// emDash is the Unicode em-dash character (U+2014), matching Node's
// `sub.user?.email || '—'` fallback in listTransactions.
const emDash = "—"

// derefOrEmDash returns the dereferenced string or emDash when nil.
func derefOrEmDash(s *string) string {
	if s == nil {
		return emDash
	}
	return *s
}

// toTimePtr converts a pgtype.Timestamp to a *time.Time, returning nil when
// the Timestamp is not valid (NULL column).
func toTimePtr(ts pgtype.Timestamp) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

// FindAllPaginated mirrors subscriptionsService.findAllPaginated + the
// listTransactions row mapping. Two queries (list + count = getManyAndCount).
// Default limit is 20 (bespoke — caller must set TxQuery.Limit).
// When Search is empty, nil is passed to the DB so the IS NULL branch fires
// (matches all rows, parity with Node omitting the WHERE clause).
func (r *Reader) FindAllPaginated(ctx context.Context, q TxQuery) (TxPage, error) {
	var searchPtr *string
	if q.Search != "" {
		searchPtr = &q.Search
	}
	offset := int32((q.Page - 1) * q.Limit)
	limit := int32(q.Limit)

	rows, err := r.q.ListSubscriptionsPaginated(ctx, sqlc.ListSubscriptionsPaginatedParams{
		Limit:  limit,
		Offset: offset,
		Search: searchPtr,
	})
	if err != nil {
		return TxPage{}, err
	}

	total, err := r.q.CountSubscriptionsPaginated(ctx, searchPtr)
	if err != nil {
		return TxPage{}, err
	}

	out := make([]TransactionRow, 0, len(rows))
	for _, row := range rows {
		tx := TransactionRow{
			ID:                 uuid.UUID(row.ID.Bytes).String(),
			UserEmail:          derefOrEmDash(row.UserEmail),
			UserName:           derefOrEmDash(row.UserName),
			UserID:             uuid.UUID(row.UserID.Bytes).String(),
			PlanName:           derefOrEmDash(row.PlanName),
			PriceCents:         derefInt32(row.PriceCents),
			StoreType:          string(row.StoreType),
			StoreTransactionID: row.StoreTransactionID,
			Status:             string(row.Status),
			StartedAt:          toTimePtr(row.StartedAt),
			ExpiresAt:          toTimePtr(row.ExpiresAt),
			CreatedAt:          toTimePtr(row.CreatedAt),
		}
		out = append(out, tx)
	}
	return TxPage{Transactions: out, Total: int(total)}, nil
}

// ─── FindLatestStripeForUserPlan (Phase 4c-3) ────────────────────────────────

// FindLatestStripeForUserPlan mirrors subscriptionsService.findLatestStripeForUserPlan:
// returns the store_transaction_id of the most-recent Stripe subscription for
// a (user, plan) pair. Used by the admin refund flow.
// No row → ("", false, nil). Other DB errors propagate.
func (r *Reader) FindLatestStripeForUserPlan(ctx context.Context, userID, planID string) (storeTxID string, found bool, err error) {
	var uid, pid pgtype.UUID
	if scanErr := uid.Scan(userID); scanErr != nil {
		return "", false, nil
	}
	if scanErr := pid.Scan(planID); scanErr != nil {
		return "", false, nil
	}
	row, err := r.q.FindLatestStripeForUserPlan(ctx, sqlc.FindLatestStripeForUserPlanParams{
		UserID: uid,
		PlanID: pid,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	txID := derefStr(row.StoreTransactionID)
	return txID, true, nil
}
