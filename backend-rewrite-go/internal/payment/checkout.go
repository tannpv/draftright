package payment

import (
	"context"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

// PaymentPendingTTL ports PAYMENT_PENDING_TTL_MS (30 min): how long a pending
// payment's reference stays valid before it expires.
const PaymentPendingTTL = 30 * time.Minute

// DomainError carries an HTTP status + the exact Node message. notFound→404,
// badRequest→400. Task 11's handler maps Status to the response code.
type DomainError struct {
	Status  int
	Message string
}

func (e *DomainError) Error() string { return e.Message }

func notFound(msg string) *DomainError   { return &DomainError{Status: 404, Message: msg} }
func badRequest(msg string) *DomainError { return &DomainError{Status: 400, Message: msg} }

// CheckoutOptions ports CreateCheckoutDto's success/cancel URLs.
type CheckoutOptions struct {
	SuccessURL string
	CancelURL  string
}

// CheckoutResponse is POST /payment/checkout's body: the persisted payment row
// plus the strategy's optional redirect/QR/bank/wallet fields. Empty optional
// fields are omitted to match Node's `undefined`-elided JSON.
type CheckoutResponse struct {
	Payment      PaymentRow             `json:"payment"`
	RedirectURL  *string                `json:"redirect_url,omitempty"`
	QRData       *string                `json:"qr_data,omitempty"`
	BankInfo     *strategy.BankInfo     `json:"bank_info,omitempty"`
	WalletIntent *strategy.WalletIntent `json:"wallet_intent,omitempty"`
}

// CreateCheckout ports PaymentService.createCheckout: validate the plan (exists,
// not free), confirm the method is enabled + supported, load the user, insert a
// pending payment, delegate to the provider strategy, persist any QR payload,
// and assemble the response. Provider failures mark the payment failed and
// surface as a 400 carrying the provider's message (byte-for-byte parity).
func (s *Service) CreateCheckout(ctx context.Context, userID, planID, method string, opts CheckoutOptions) (*CheckoutResponse, error) {
	plan, err := s.checkoutRepo.PlanForCheckout(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, notFound("Plan not found")
	}
	if plan.PriceCents == 0 {
		return nil, badRequest("Cannot purchase a free plan")
	}

	enabled, err := s.EnabledMethods(ctx)
	if err != nil {
		return nil, err
	}
	if !contains(enabled, method) {
		return nil, notFound("Payment method '" + method + "' is not enabled.")
	}
	strat, ok := s.strategies[method]
	if !ok {
		return nil, badRequest("Unsupported payment method: " + method)
	}

	user, err := s.checkoutRepo.UserForCheckout(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, notFound("User not found")
	}

	currency := plan.Currency
	if currency == "" {
		currency = "USD"
	}
	ref := s.genRef()
	expiresAt := s.now().Add(PaymentPendingTTL)
	created, err := s.checkoutRepo.CreatePayment(ctx, userID, planID, plan.PriceCents, currency, method, "pending", ref, expiresAt)
	if err != nil {
		return nil, err
	}

	res, err := strat.CreateCheckout(ctx,
		strategy.Payment{
			ID:            created.ID,
			UserID:        userID,
			PlanID:        planID,
			Amount:        plan.PriceCents,
			Currency:      currency,
			Method:        method,
			ReferenceCode: ref,
		},
		strategy.Plan{
			Name:          plan.Name,
			StripePriceID: plan.StripePriceID,
			TrialDays:     plan.TrialDays,
			BillingPeriod: plan.BillingPeriod,
			PriceCents:    plan.PriceCents,
		},
		strategy.Options{
			SuccessURL:       opts.SuccessURL,
			CancelURL:        opts.CancelURL,
			StripeCustomerID: user.StripeCustomerID,
			UserEmail:        user.Email,
		},
	)
	if err != nil {
		_ = s.checkoutRepo.MarkFailed(ctx, created.ID, err.Error())
		return nil, badRequest(err.Error())
	}
	if res.QRData != "" {
		if uerr := s.checkoutRepo.UpdateQRData(ctx, created.ID, res.QRData); uerr != nil {
			return nil, uerr
		}
	}

	row := PaymentRow{
		ID:            created.ID,
		UserID:        userID,
		PlanID:        planID,
		Amount:        plan.PriceCents,
		Currency:      currency,
		Method:        method,
		Status:        "pending",
		ReferenceCode: ref,
		ExpiresAt:     &expiresAt,
		CreatedAt:     created.CreatedAt,
		UpdatedAt:     created.UpdatedAt,
		// TODO(phase3b-followup): GetPlanForCheckout omits daily_limit/is_active/
		// created_at/updated_at, so this nested plan is partial vs Node's full plan
		// entity. Known gap, documented in the VietQR shadow fixture's ignore_value_of.
		// Widen GetPlanForCheckout + populate fully if exact nested-plan parity needed.
		Plan: &PlanBrief{
			ID:            plan.ID,
			Name:          plan.Name,
			PriceCents:    plan.PriceCents,
			StripePriceID: ptrOrNil(plan.StripePriceID),
			TrialDays:     plan.TrialDays,
			BillingPeriod: plan.BillingPeriod,
			Currency:      ptrOrNil(plan.Currency),
			IsActive:      true,
		},
	}
	if res.QRData != "" {
		row.QRData = &res.QRData
	}

	resp := &CheckoutResponse{Payment: row}
	if res.QRData != "" {
		resp.QRData = &res.QRData
	}
	if res.RedirectURL != "" {
		resp.RedirectURL = &res.RedirectURL
	}
	resp.BankInfo = res.BankInfo
	resp.WalletIntent = res.WalletIntent
	return resp, nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
