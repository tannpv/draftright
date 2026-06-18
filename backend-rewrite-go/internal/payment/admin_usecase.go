package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
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
	RefundPayment(ctx context.Context, id, notes string) error
}

// The refund() consumer-side ports. Kept small (1 method each); main.go injects
// the concretes (payment settings adapter, subscription Reader, subscription
// WebhookWriter) — that wiring is a later task. stripeRefunder is declared in
// refund_stripe.go (Task 16) and reused here.
type stripeSecretSource interface {
	// StripeSecretKey resolves the active Stripe secret key (settings override
	// then env fallback). "" means "not configured".
	StripeSecretKey(ctx context.Context) (string, error)
}
type stripeSubLookup interface {
	// FindLatestStripeForUserPlan mirrors
	// subscriptionsService.findLatestStripeForUserPlan: the store_transaction_id
	// (Stripe sub id) of the latest stripe subscription for (user, plan). found
	// is false when no row matches.
	FindLatestStripeForUserPlan(ctx context.Context, userID, planID string) (subID string, found bool, err error)
}
type subscriptionCanceller interface {
	// CancelByStoreRef mirrors subscriptionsService.cancelByStripeSubId — marks
	// our DB subscription cancelled so the quota guard demotes the user.
	CancelByStoreRef(ctx context.Context, storeType, storeRef string) error
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
	repo         paymentAdminRepo
	activator    subscriptionActivator
	refunder     stripeRefunder
	secretSource stripeSecretSource
	subLookup    stripeSubLookup
	subCanceller subscriptionCanceller
	log          *slog.Logger
	now          func() time.Time
}

// NewAdminService accepts the consumer ports + a logger + a now() clock;
// *AdminRepo and the webhook *Service satisfy the interfaces for the composition
// root, fakes satisfy them for tests (accept interfaces, return structs). now is
// injected so completed_at is deterministic in tests. A nil logger defaults to
// slog.Default(). The refund-only deps (refunder/secretSource/subLookup/
// subCanceller) may be nil for paths that never call Refund (e.g. confirm tests).
func NewAdminService(repo paymentAdminRepo, activator subscriptionActivator, refunder stripeRefunder, secretSource stripeSecretSource, subLookup stripeSubLookup, subCanceller subscriptionCanceller, log *slog.Logger, now func() time.Time) *AdminService {
	if log == nil {
		log = slog.Default()
	}
	return &AdminService{
		repo:         repo,
		activator:    activator,
		refunder:     refunder,
		secretSource: secretSource,
		subLookup:    subLookup,
		subCanceller: subCanceller,
		log:          log,
		now:          now,
	}
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

// Refund refunds a completed Stripe payment and cancels its subscription so the
// user loses access immediately. Ports payment.service refund():
//
//	load by id (relations:['plan']) → 404 if missing
//	already-refunded → no-op return (no persist, no Stripe)
//	not completed → 400 'Only completed payments can be refunded'
//	non-stripe → 400 `Refund not supported for method '<method>'`
//	resolve secret key (settings || env) → 400 if empty
//	find latest stripe sub → if present: refund latest invoice charge (skip if
//	  none), then cancel the Stripe sub (non-fatal); else metadata-only
//	mutate status=refunded + compose notes, persist, cancel our DB sub
//	return the in-memory mutated payment (plan loaded, no reload).
func (s *AdminService) Refund(ctx context.Context, id, reason string) (AdminPaymentDetailRow, error) {
	row, err := s.repo.LoadPaymentWithPlan(ctx, id)
	if err != nil {
		return AdminPaymentDetailRow{}, err // ErrPaymentNotFound propagates (404)
	}
	if row.Status == string(StatusRefunded) {
		return row, nil // no-op: already refunded (no persist, no Stripe)
	}
	if row.Status != string(StatusCompleted) {
		return AdminPaymentDetailRow{}, ErrRefundNotCompleted
	}
	if row.Method != string(MethodStripe) {
		return AdminPaymentDetailRow{}, badRequest(fmt.Sprintf("Refund not supported for method '%s'", row.Method))
	}

	key, err := s.secretSource.StripeSecretKey(ctx)
	if err != nil {
		return AdminPaymentDetailRow{}, err // settings lookup error → 500
	}
	if key == "" {
		return AdminPaymentDetailRow{}, ErrStripeKeyMissing
	}

	subID, found, err := s.subLookup.FindLatestStripeForUserPlan(ctx, row.UserID, row.PlanID)
	if err != nil {
		return AdminPaymentDetailRow{}, err
	}
	hasSub := found && subID != ""

	refundedCharge := ""
	if hasSub {
		chargeID, err := s.refunder.LatestInvoiceCharge(ctx, key, subID)
		if err != nil {
			return AdminPaymentDetailRow{}, err // Node retrieve throw → 500
		}
		if chargeID != "" {
			refundID, err := s.refunder.CreateRefund(ctx, key, chargeID, reason)
			if err != nil {
				return AdminPaymentDetailRow{}, err // Node refunds.create throw → 500
			}
			refundedCharge = refundID
			s.log.Info("refunded stripe charge", "charge", chargeID, "refund", refundID, "payment", id)
		} else {
			s.log.Warn("no charge found on latest invoice; skipping Stripe refund", "sub", subID)
		}
		// Cancel the Stripe subscription IMMEDIATELY. Runs even when no charge
		// was found, and is NON-FATAL (Node wraps it in try/catch).
		if err := s.refunder.CancelSubscription(ctx, key, subID); err != nil {
			s.log.Warn("stripe sub cancel failed", "sub", subID, "err", err)
		}
	} else {
		s.log.Warn("no Stripe subscription linked to payment; refunding metadata only", "payment", id)
	}

	// Compose the note: ['<existing>', 'Refunded by admin (reason) — Stripe refund <id>']
	// .filter(Boolean).join(' | '). The em-dash is U+2014 with surrounding spaces.
	note := "Refunded by admin"
	if reason != "" {
		note += " (" + reason + ")"
	}
	if refundedCharge != "" {
		note += " — Stripe refund " + refundedCharge
	}
	parts := []string{}
	if row.Notes != nil && *row.Notes != "" {
		parts = append(parts, *row.Notes)
	}
	parts = append(parts, note)
	final := strings.Join(parts, " | ")

	row.Status = string(StatusRefunded)
	row.Notes = &final

	if err := s.repo.RefundPayment(ctx, id, final); err != nil {
		return AdminPaymentDetailRow{}, err
	}

	// Mark our DB subscription cancelled (Node cancelByStripeSubId — NOT wrapped
	// in try/catch, so an error propagates → 500).
	if hasSub {
		if err := s.subCanceller.CancelByStoreRef(ctx, string(StoreStripe), subID); err != nil {
			return AdminPaymentDetailRow{}, err
		}
	}

	return row, nil
}
