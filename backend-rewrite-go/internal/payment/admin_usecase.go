package payment

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/plans"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// PaymentStats is the GET /admin/payments/stats body. Field order = JSON key
// order, matching Node payment.service getStats return
// ({ total, completed, pending, revenue }).
type PaymentStats struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Pending   int `json:"pending"`
	Revenue   int `json:"revenue"`
}

// FindAllParams carries the GET /admin/payments query inputs. Page/Limit are
// already defaulted by the handler (Node defaults page=1, limit=20).
// SortOrder is already normalised to "ASC"/"DESC" by the handler; the repo
// re-applies the ASC-iff-"ASC" rule defensively. NO clamping of Limit (Node
// payment findAll calls take(limit) with no clamp).
type FindAllParams struct {
	Page, Limit int
	Status      string
	Search      string
	SortBy      string
	SortOrder   string
}

// PaymentsPage is the GET /admin/payments body. Field order = JSON key order,
// matching Node payment.service findAll return ({ payments, total }).
type PaymentsPage struct {
	Payments []AdminPaymentRow `json:"payments"`
	Total    int               `json:"total"`
}

// paymentAdminRepo is the AdminService's consumer-side port. *AdminRepo
// satisfies it. (More methods get added by later 4c-3 tasks.)
type paymentAdminRepo interface {
	Stats(ctx context.Context) (PaymentStats, error)
	FindAll(ctx context.Context, p FindAllParams) ([]AdminPaymentRow, int, error)
	LoadPaymentWithPlan(ctx context.Context, id string) (AdminPaymentDetailRow, error)
	ConfirmPayment(ctx context.Context, id string, completedAt time.Time, notes string) error
}

// subscriptionActivator is the AdminService's port onto the webhook activation
// path. The webhook *Service satisfies it (same package) via its exported
// Activate wrapper, so admin-confirm and webhook-completion funnel through ONE
// activation implementation (no logic fork). A test fake satisfies it too.
type subscriptionActivator interface {
	Activate(ctx context.Context, billing, userID, planID, method string) error
}

// AdminService serves the admin payment routes (stats / findAll / confirm;
// refund added by a later task). Separate from the webhook Service to keep that
// 12-dep struct untouched.
type AdminService struct {
	repo      paymentAdminRepo
	activator subscriptionActivator
	now       func() time.Time
}

// NewAdminService accepts the consumer ports + a now() clock; *AdminRepo and the
// webhook *Service satisfy the interfaces for the composition root, fakes
// satisfy them for tests (accept interfaces, return structs). now is injected so
// completed_at is deterministic in tests.
func NewAdminService(repo paymentAdminRepo, activator subscriptionActivator, now func() time.Time) *AdminService {
	return &AdminService{repo: repo, activator: activator, now: now}
}

// GetStats returns aggregate payment counts + completed revenue.
func (s *AdminService) GetStats(ctx context.Context) (PaymentStats, error) {
	return s.repo.Stats(ctx)
}

// FindAll returns the paginated payments + total, wrapped into PaymentsPage.
// A nil rows slice is normalised to [] so the JSON emits an empty array, never
// null (Node always returns an array). Errors surface with a zero PaymentsPage.
func (s *AdminService) FindAll(ctx context.Context, p FindAllParams) (PaymentsPage, error) {
	rows, total, err := s.repo.FindAll(ctx, p)
	if err != nil {
		return PaymentsPage{}, err
	}
	if rows == nil {
		rows = []AdminPaymentRow{}
	}
	return PaymentsPage{Payments: rows, Total: total}, nil
}

// AdminPaymentDetailRow is a Payment serialized with its plan relation loaded
// but WITHOUT the user relation — adminConfirm (and refund) use
// relations:['plan'], so TypeORM/JSON.stringify leave `user` undefined and the
// key is OMITTED entirely (not null). Field order = Payment entity property
// order MINUS user. Nested plan via *plans.PlanEntity (nil → null).
//
// NOTE (parity / naming): the master plan calls this type `PaymentRow`, but that
// name is already taken in this package by the user-facing /payment/history
// projection in reader.go (whose nested plan is a *PlanBrief). The JSON shape
// here matches the plan's spec exactly — 16 keys, no `user`, nested
// *plans.PlanEntity; only the Go identifier differs to avoid the collision.
type AdminPaymentDetailRow struct {
	ID            string
	UserID        string
	PlanID        string
	Plan          *plans.PlanEntity
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
}

// MarshalJSON pins the key order to id,user_id,plan_id,plan,amount,currency,
// method,status,provider_ref,reference_code,qr_data,notes,expires_at,
// completed_at,created_at,updated_at — 16 keys, NO `user`. Timestamps are
// ISO-millis; nullable scalars → null; nil plan → null.
func (p AdminPaymentDetailRow) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID            string            `json:"id"`
		UserID        string            `json:"user_id"`
		PlanID        string            `json:"plan_id"`
		Plan          *plans.PlanEntity `json:"plan"`
		Amount        int               `json:"amount"`
		Currency      string            `json:"currency"`
		Method        string            `json:"method"`
		Status        string            `json:"status"`
		ProviderRef   *string           `json:"provider_ref"`
		ReferenceCode string            `json:"reference_code"`
		QRData        *string           `json:"qr_data"`
		Notes         *string           `json:"notes"`
		ExpiresAt     *string           `json:"expires_at"`
		CompletedAt   *string           `json:"completed_at"`
		CreatedAt     string            `json:"created_at"`
		UpdatedAt     string            `json:"updated_at"`
	}{
		ID:            p.ID,
		UserID:        p.UserID,
		PlanID:        p.PlanID,
		Plan:          p.Plan,
		Amount:        p.Amount,
		Currency:      p.Currency,
		Method:        p.Method,
		Status:        p.Status,
		ProviderRef:   p.ProviderRef,
		ReferenceCode: p.ReferenceCode,
		QRData:        p.QRData,
		Notes:         p.Notes,
		ExpiresAt:     isoPtr(p.ExpiresAt),
		CompletedAt:   isoPtr(p.CompletedAt),
		CreatedAt:     shared.ISOMillis(p.CreatedAt),
		UpdatedAt:     shared.ISOMillis(p.UpdatedAt),
	})
}

// AdminConfirm flips a pending payment to completed (stamping completed_at +
// notes) and activates its subscription. Ports payment.service adminConfirm:
// load by id (relations:['plan']) → 404 if missing → 400 if not pending →
// status=completed, completed_at=now, notes=adminNotes||'Manually confirmed by
// admin', save → activateSubscription(payment) → return the (in-memory mutated)
// payment, plan loaded, user undefined. The activation reuses the webhook path.
func (s *AdminService) AdminConfirm(ctx context.Context, id, notes string) (AdminPaymentDetailRow, error) {
	row, err := s.repo.LoadPaymentWithPlan(ctx, id)
	if err != nil {
		return AdminPaymentDetailRow{}, err // ErrPaymentNotFound propagates
	}
	if row.Status != string(StatusPending) {
		return AdminPaymentDetailRow{}, ErrPaymentNotPending
	}
	if notes == "" {
		notes = "Manually confirmed by admin"
	}
	completedAt := s.now()
	if err := s.repo.ConfirmPayment(ctx, id, completedAt, notes); err != nil {
		return AdminPaymentDetailRow{}, err
	}
	billing := ""
	if row.Plan != nil {
		billing = row.Plan.BillingPeriod
	}
	if err := s.activator.Activate(ctx, billing, row.UserID, row.PlanID, row.Method); err != nil {
		return AdminPaymentDetailRow{}, err
	}
	// Mirror Node's in-memory mutation of the loaded entity (no reload).
	row.Status = string(StatusCompleted)
	row.CompletedAt = &completedAt
	row.Notes = &notes
	return row, nil
}
