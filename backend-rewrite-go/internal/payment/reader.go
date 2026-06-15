package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Querier is the sqlc subset the payment repo needs (consumer-side port so
// tests fake it without a DB).
type Querier interface {
	GetPaymentByReference(ctx context.Context, referenceCode string) (sqlc.GetPaymentByReferenceRow, error)
	ListPaymentsByUser(ctx context.Context, userID pgtype.UUID) ([]sqlc.ListPaymentsByUserRow, error)
	GetPlanForCheckout(ctx context.Context, id pgtype.UUID) (sqlc.GetPlanForCheckoutRow, error)
	GetUserForCheckout(ctx context.Context, id pgtype.UUID) (sqlc.GetUserForCheckoutRow, error)
	CreatePayment(ctx context.Context, arg sqlc.CreatePaymentParams) (sqlc.CreatePaymentRow, error)
	UpdatePaymentQRData(ctx context.Context, arg sqlc.UpdatePaymentQRDataParams) error
	MarkPaymentFailed(ctx context.Context, arg sqlc.MarkPaymentFailedParams) error
}

// StatusRow is the GetByReference projection feeding the status endpoint.
// PlanName/CompletedAt/ExpiresAt are nil when the underlying column is NULL.
type StatusRow struct {
	Status        string
	Method        string
	Amount        int
	Currency      string
	ReferenceCode string
	PlanName      *string
	CompletedAt   *time.Time
	ExpiresAt     *time.Time
}

// PaymentRow is the full ListByUser projection (every payments column + the
// joined plan), serialised by the history endpoint to match TypeORM's entity
// JSON. Nullable columns are pointers.
type PaymentRow struct {
	ID            string
	UserID        string
	PlanID        string
	Amount        int
	Currency      string
	Method        string
	Status        string
	ProviderRef   *string
	ReferenceCode string
	QRData        *string
	Notes         *string
	ExpiresAt     *time.Time
	CompletedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Plan          *PlanBrief
}

// PlanBrief is the nested plan object on a history row, field-for-field with
// plans.PlanEntity (same column set TypeORM loads via relations:['plan']).
type PlanBrief struct {
	ID            string
	Name          string
	DailyLimit    int
	PriceCents    int
	Currency      *string
	StripePriceID *string
	TrialDays     int
	BillingPeriod string
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Repo is the concrete persistence adapter over the sqlc Querier.
type Repo struct{ q Querier }

// NewRepo wires the querier (accept interface, return struct).
func NewRepo(q Querier) *Repo { return &Repo{q: q} }

// GetByReference returns the payment for a reference code, or (nil,nil) when
// none exists (Node returns null → controller emits {status:"not_found"}).
func (r *Repo) GetByReference(ctx context.Context, ref string) (*StatusRow, error) {
	row, err := r.q.GetPaymentByReference(ctx, ref)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &StatusRow{
		Status:        row.Status,
		Method:        row.Method,
		Amount:        int(row.Amount),
		Currency:      row.Currency,
		ReferenceCode: row.ReferenceCode,
		PlanName:      row.PlanName,
		CompletedAt:   tsPtr(row.CompletedAt),
		ExpiresAt:     tsPtr(row.ExpiresAt),
	}, nil
}

// ListByUser returns the user's 20 newest payments, each with its plan. Always
// returns a non-nil slice ([] not null). A malformed user id yields empty.
func (r *Repo) ListByUser(ctx context.Context, userID string) ([]PaymentRow, error) {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return []PaymentRow{}, nil
	}
	rows, err := r.q.ListPaymentsByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]PaymentRow, 0, len(rows))
	for _, row := range rows {
		pr := PaymentRow{
			ID:            uuid.UUID(row.ID.Bytes).String(),
			UserID:        uuid.UUID(row.UserID.Bytes).String(),
			PlanID:        uuid.UUID(row.PlanID.Bytes).String(),
			Amount:        int(row.Amount),
			Currency:      row.Currency,
			Method:        row.Method,
			Status:        row.Status,
			ProviderRef:   row.ProviderRef,
			ReferenceCode: row.ReferenceCode,
			QRData:        row.QrData,
			Notes:         row.Notes,
			ExpiresAt:     tsPtr(row.ExpiresAt),
			CompletedAt:   tsPtr(row.CompletedAt),
			CreatedAt:     row.CreatedAt.Time,
			UpdatedAt:     row.UpdatedAt.Time,
		}
		if row.PlanPk.Valid {
			pr.Plan = &PlanBrief{
				ID:            uuid.UUID(row.PlanPk.Bytes).String(),
				Name:          derefStr(row.PlanName),
				DailyLimit:    derefInt(row.PlanDailyLimit),
				PriceCents:    derefInt(row.PlanPriceCents),
				Currency:      row.PlanCurrency,
				StripePriceID: row.PlanStripePriceID,
				TrialDays:     derefInt(row.PlanTrialDays),
				BillingPeriod: derefBillingPeriod(row.PlanBillingPeriod),
				IsActive:      derefBool(row.PlanIsActive),
				CreatedAt:     row.PlanCreatedAt.Time,
				UpdatedAt:     row.PlanUpdatedAt.Time,
			}
		}
		out = append(out, pr)
	}
	return out, nil
}

// CheckoutPlan is the plan projection createCheckout needs (free-plan guard +
// payment.amount + strategy inputs). Nullable columns flatten to "".
type CheckoutPlan struct {
	ID, Name      string
	DailyLimit    int
	PriceCents    int
	Currency      string // "" when NULL
	StripePriceID string // "" when NULL
	TrialDays     int
	BillingPeriod string
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CheckoutUser is the user projection createCheckout / portal / cancel need.
// Nullable provider customer ids flatten to "".
type CheckoutUser struct {
	ID, Email              string
	StripeCustomerID       string // "" when NULL
	LemonSqueezyCustomerID string // "" when NULL
}

// CreatedPayment is the RETURNING projection of CreatePayment (server defaults).
type CreatedPayment struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PlanForCheckout loads the plan by id, or (nil,nil) when the id is malformed
// or no row exists (Node throws NotFound → controller maps; callers treat nil
// as "plan not found").
func (r *Repo) PlanForCheckout(ctx context.Context, id string) (*CheckoutPlan, error) {
	uid, ok := parseUUID(id)
	if !ok {
		return nil, nil
	}
	row, err := r.q.GetPlanForCheckout(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &CheckoutPlan{
		ID:            uuid.UUID(row.ID.Bytes).String(),
		Name:          row.Name,
		DailyLimit:    int(row.DailyLimit),
		PriceCents:    int(row.PriceCents),
		Currency:      derefStr(row.Currency),
		StripePriceID: derefStr(row.StripePriceID),
		TrialDays:     int(row.TrialDays),
		BillingPeriod: string(row.BillingPeriod),
		IsActive:      row.IsActive,
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}, nil
}

// UserForCheckout loads the user by id, or (nil,nil) when the id is malformed
// or no row exists.
func (r *Repo) UserForCheckout(ctx context.Context, id string) (*CheckoutUser, error) {
	uid, ok := parseUUID(id)
	if !ok {
		return nil, nil
	}
	row, err := r.q.GetUserForCheckout(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &CheckoutUser{
		ID:                     uuid.UUID(row.ID.Bytes).String(),
		Email:                  row.Email,
		StripeCustomerID:       derefStr(row.StripeCustomerID),
		LemonSqueezyCustomerID: derefStr(row.LemonsqueezyCustomerID),
	}, nil
}

// CreatePayment inserts a pending payment and returns the server-assigned id +
// timestamps. method/status/currency are plain varchar columns (NOT enums).
func (r *Repo) CreatePayment(ctx context.Context, userID, planID string, amount int, currency, method, status, ref string, expiresAt time.Time) (*CreatedPayment, error) {
	uid, ok := parseUUID(userID)
	if !ok {
		return nil, fmt.Errorf("payment: invalid user id %q", userID)
	}
	pid, ok := parseUUID(planID)
	if !ok {
		return nil, fmt.Errorf("payment: invalid plan id %q", planID)
	}
	row, err := r.q.CreatePayment(ctx, sqlc.CreatePaymentParams{
		UserID:        uid,
		PlanID:        pid,
		Amount:        int32(amount),
		Currency:      currency,
		Method:        method,
		Status:        status,
		ReferenceCode: ref,
		ExpiresAt:     pgtype.Timestamp{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	return &CreatedPayment{
		ID:        uuid.UUID(row.ID.Bytes).String(),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

// UpdateQRData stores the rendered QR payload on a pending payment.
func (r *Repo) UpdateQRData(ctx context.Context, paymentID, qr string) error {
	pid, ok := parseUUID(paymentID)
	if !ok {
		return fmt.Errorf("payment: invalid payment id %q", paymentID)
	}
	return r.q.UpdatePaymentQRData(ctx, sqlc.UpdatePaymentQRDataParams{ID: pid, QrData: &qr})
}

// MarkFailed flips a payment to status='failed' with the given notes.
func (r *Repo) MarkFailed(ctx context.Context, paymentID, notes string) error {
	pid, ok := parseUUID(paymentID)
	if !ok {
		return fmt.Errorf("payment: invalid payment id %q", paymentID)
	}
	return r.q.MarkPaymentFailed(ctx, sqlc.MarkPaymentFailedParams{ID: pid, Notes: &notes})
}

// parseUUID parses a string id into a pgtype.UUID; ok=false on a malformed id.
func parseUUID(s string) (pgtype.UUID, bool) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}, false
	}
	return u, true
}

// --- pgtype / pointer helpers --------------------------------------------

func tsPtr(ts pgtype.Timestamp) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int32) int {
	if i == nil {
		return 0
	}
	return int(*i)
}

func derefBool(b *bool) bool {
	return b != nil && *b
}

func derefBillingPeriod(p *sqlc.PlansBillingPeriodEnum) string {
	if p == nil {
		return ""
	}
	return string(*p)
}

// MarshalJSON pins PaymentRow to the TypeORM Payment entity JSON: every column
// in declaration order, ms-precision timestamps, nested plan last. The `user`
// relation is NOT loaded by findByUser (relations:['plan'] only), so it is
// absent here too.
func (p PaymentRow) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID            string     `json:"id"`
		UserID        string     `json:"user_id"`
		PlanID        string     `json:"plan_id"`
		Amount        int        `json:"amount"`
		Currency      string     `json:"currency"`
		Method        string     `json:"method"`
		Status        string     `json:"status"`
		ProviderRef   *string    `json:"provider_ref"`
		ReferenceCode string     `json:"reference_code"`
		QRData        *string    `json:"qr_data"`
		Notes         *string    `json:"notes"`
		ExpiresAt     *string    `json:"expires_at"`
		CompletedAt   *string    `json:"completed_at"`
		CreatedAt     string     `json:"created_at"`
		UpdatedAt     string     `json:"updated_at"`
		Plan          *PlanBrief `json:"plan"`
	}{
		ID: p.ID, UserID: p.UserID, PlanID: p.PlanID, Amount: p.Amount,
		Currency: p.Currency, Method: p.Method, Status: p.Status,
		ProviderRef: p.ProviderRef, ReferenceCode: p.ReferenceCode, QRData: p.QRData,
		Notes: p.Notes, ExpiresAt: isoPtr(p.ExpiresAt), CompletedAt: isoPtr(p.CompletedAt),
		CreatedAt: shared.ISOMillis(p.CreatedAt), UpdatedAt: shared.ISOMillis(p.UpdatedAt),
		Plan: p.Plan,
	})
}

// MarshalJSON pins PlanBrief to plans.PlanEntity's shape (same column order).
func (p PlanBrief) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID            string  `json:"id"`
		Name          string  `json:"name"`
		DailyLimit    int     `json:"daily_limit"`
		PriceCents    int     `json:"price_cents"`
		Currency      *string `json:"currency"`
		StripePriceID *string `json:"stripe_price_id"`
		TrialDays     int     `json:"trial_days"`
		BillingPeriod string  `json:"billing_period"`
		IsActive      bool    `json:"is_active"`
		CreatedAt     string  `json:"created_at"`
		UpdatedAt     string  `json:"updated_at"`
	}{
		ID: p.ID, Name: p.Name, DailyLimit: p.DailyLimit, PriceCents: p.PriceCents,
		Currency: p.Currency, StripePriceID: p.StripePriceID, TrialDays: p.TrialDays,
		BillingPeriod: p.BillingPeriod, IsActive: p.IsActive,
		CreatedAt: shared.ISOMillis(p.CreatedAt), UpdatedAt: shared.ISOMillis(p.UpdatedAt),
	})
}

func isoPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := shared.ISOMillis(*t)
	return &s
}
