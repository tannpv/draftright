package payment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

// WebhookResult is POST /payment/webhook/:method's response. It always carries
// `success`; `reference_code` is present only when the action resolved a
// reference (Node returns {success} for renewals/cancels/expiries and
// {success, reference_code} for the payment-completing actions).
type WebhookResult struct {
	Success       bool
	ReferenceCode *string
}

// MarshalJSON emits {"success":bool} and adds "reference_code" only when set,
// matching the Node controller's conditional spread.
func (r WebhookResult) MarshalJSON() ([]byte, error) {
	if r.ReferenceCode == nil {
		return json.Marshal(struct {
			Success bool `json:"success"`
		}{Success: r.Success})
	}
	return json.Marshal(struct {
		Success       bool   `json:"success"`
		ReferenceCode string `json:"reference_code"`
	}{Success: r.Success, ReferenceCode: *r.ReferenceCode})
}

// HandleWebhook verifies a provider webhook then dispatches the resulting action
// to the subscription writer / payment repo / emailer. Ports
// PaymentService.handleWebhook + completePayment + activateSubscription.
func (s *Service) HandleWebhook(ctx context.Context, method string, payload []byte, headers http.Header) (WebhookResult, error) {
	enabled, err := s.EnabledMethods(ctx)
	if err != nil {
		return WebhookResult{}, err
	}
	if !contains(enabled, method) {
		return WebhookResult{}, notFound("Payment method '" + method + "' is not enabled.")
	}
	strat, ok := s.strategies[method]
	if !ok {
		return WebhookResult{}, badRequest("Unsupported payment method: " + method)
	}
	action, err := strat.VerifyWebhook(ctx, payload, headers)
	if err != nil {
		var we *strategy.WebhookError
		if errors.As(err, &we) {
			return WebhookResult{}, &DomainError{Status: we.Status, Message: we.Message}
		}
		return WebhookResult{}, err
	}

	switch action.Type {
	case strategy.ActionPaymentCompleted:
		if action.StripeCustomerID != "" {
			if pay, _ := s.webhookRepo.PaymentForWebhook(ctx, action.ReferenceCode); pay != nil && pay.UserID != "" {
				_ = s.webhookRepo.SetStripeCustomerID(ctx, pay.UserID, action.StripeCustomerID)
			}
		}
		done, ref, err := s.completePayment(ctx, action.ReferenceCode, "completed")
		if err != nil {
			return WebhookResult{}, err
		}
		if action.StripeSubscriptionID != "" && done {
			_ = s.subsWriter.StampStoreRef(ctx, action.ReferenceCode, string(StoreStripe), action.StripeSubscriptionID)
		}
		return result(done, ref), nil

	case strategy.ActionPaymentFailed:
		done, ref, err := s.completePayment(ctx, action.ReferenceCode, "failed")
		if err != nil {
			return WebhookResult{}, err
		}
		return result(done, ref), nil

	case strategy.ActionSubscriptionRenewed:
		_, _ = s.subsWriter.ExtendByStoreRef(ctx, string(StoreStripe), action.StripeSubscriptionID, time.Unix(action.CurrentPeriodEnd, 0))
		return WebhookResult{Success: true}, nil

	case strategy.ActionSubscriptionCanceled:
		_, _ = s.subsWriter.CancelByStoreRef(ctx, string(StoreStripe), action.StripeSubscriptionID)
		return WebhookResult{Success: true}, nil

	case strategy.ActionLSPaymentSuccess:
		if action.LSCustomerID != "" {
			if pay, _ := s.webhookRepo.PaymentForWebhook(ctx, action.ReferenceCode); pay != nil && pay.UserID != "" {
				_ = s.webhookRepo.SetLemonSqueezyCustomerID(ctx, pay.UserID, action.LSCustomerID)
			}
		}
		pay, err := s.webhookRepo.PaymentForWebhook(ctx, action.ReferenceCode)
		if err != nil {
			return WebhookResult{}, err
		}
		if pay != nil && pay.Status == "pending" {
			if action.LSVariantID != "" {
				if truePlan := s.resolvePlanIDFromLSVariant(ctx, action.LSVariantID, orUSD(pay.Currency)); truePlan != "" && truePlan != pay.PlanID {
					_ = s.webhookRepo.UpdatePaymentPlan(ctx, pay.ID, truePlan)
				}
			}
			done, ref, err := s.completePayment(ctx, action.ReferenceCode, "completed")
			if err != nil {
				return WebhookResult{}, err
			}
			if done {
				_ = s.subsWriter.StampStoreRef(ctx, action.ReferenceCode, string(StoreLemonSqueezy), action.LSSubscriptionID)
			}
			return result(done, ref), nil
		}
		if action.CurrentPeriodEnd > 0 {
			_, _ = s.subsWriter.ExtendByStoreRef(ctx, string(StoreLemonSqueezy), action.LSSubscriptionID, time.Unix(action.CurrentPeriodEnd, 0))
		}
		ref := action.ReferenceCode
		return WebhookResult{Success: true, ReferenceCode: &ref}, nil

	case strategy.ActionLSPaymentFailed:
		sub, _ := s.subsWriter.FindByStoreRef(ctx, string(StoreLemonSqueezy), action.LSSubscriptionID)
		if sub != nil && sub.UserEmail != "" {
			name := sub.UserName
			if name == "" {
				name = sub.UserEmail
			}
			s.emailer.PaymentFailed(ctx, sub.UserEmail, name, sub.PlanName)
		}
		return WebhookResult{Success: true}, nil

	case strategy.ActionLSSubscriptionCanceled:
		_, _ = s.subsWriter.CancelByStoreRef(ctx, string(StoreLemonSqueezy), action.LSSubscriptionID)
		return WebhookResult{Success: true}, nil

	case strategy.ActionLSSubscriptionExpired:
		_, _ = s.subsWriter.ExpireByStoreRef(ctx, string(StoreLemonSqueezy), action.LSSubscriptionID)
		return WebhookResult{Success: true}, nil

	case strategy.ActionDisputeCreated:
		return WebhookResult{Success: true}, nil

	default: // ActionIgnored
		return WebhookResult{Success: false}, nil
	}
}

// completePayment flips a pending payment to completed/failed by reference and,
// for the completed path, activates the subscription. Returns (handled, ref,
// err): handled=false only when the reference matched no payment; an
// already-settled payment is a successful no-op (handled=true).
func (s *Service) completePayment(ctx context.Context, ref, status string) (bool, string, error) {
	pay, err := s.webhookRepo.PaymentForWebhook(ctx, ref)
	if err != nil {
		return false, ref, err
	}
	if pay == nil {
		return false, ref, nil
	}
	if pay.Status != "pending" {
		return true, ref, nil
	}
	if status == "completed" {
		if err := s.webhookRepo.MarkPaymentCompleted(ctx, ref); err != nil {
			return false, ref, err
		}
		if err := s.activateSubscription(ctx, pay.BillingPeriod, pay.UserID, pay.PlanID, pay.Method); err != nil {
			return false, ref, err
		}
	} else {
		if err := s.webhookRepo.MarkPaymentFailedByRef(ctx, ref); err != nil {
			return false, ref, err
		}
	}
	return true, ref, nil
}

// activateSubscription grants the Pro subscription for a completed payment and
// fires the activation email. expires_at = now + 1mo (monthly) or 1yr (yearly).
// It takes primitives (not a *WebhookPayment) so both the webhook completion
// path and the admin-confirm path funnel through ONE function — no logic fork.
func (s *Service) activateSubscription(ctx context.Context, billing, userID, planID, method string) error {
	if billing == "" {
		billing = "monthly"
	}
	now := s.now()
	exp := now.AddDate(0, 1, 0)
	if billing == "yearly" {
		exp = now.AddDate(1, 0, 0)
	}
	if err := s.subsWriter.Grant(ctx, userID, planID, string(StoreTypeForMethod(PaymentMethod(method))), &exp); err != nil {
		return err
	}
	if email, name, err := s.webhookRepo.UserEmailName(ctx, userID); err == nil && email != "" {
		s.emailer.SubscriptionActivated(ctx, email, name, "Pro")
	}
	return nil
}

// Activate is the exported wrapper the payment AdminService consumes (it lives
// in the same package but talks to the webhook Service through this small
// surface). It delegates to the unexported activateSubscription so the webhook
// and admin-confirm paths share one activation implementation.
func (s *Service) Activate(ctx context.Context, billing, userID, planID, method string) error {
	return s.activateSubscription(ctx, billing, userID, planID, method)
}

// resolvePlanIDFromLSVariant maps a Lemon Squeezy variant id to the active plan
// id for its billing period + currency, or "" when the variant doesn't match a
// configured monthly/yearly variant. Ports the LS variant→plan re-resolution.
func (s *Service) resolvePlanIDFromLSVariant(ctx context.Context, variantID, currency string) string {
	monthly, yearly, err := s.variants.LemonSqueezyVariants(ctx)
	if err != nil {
		return ""
	}
	billing := ""
	switch {
	case variantID == monthly && monthly != "":
		billing = "monthly"
	case variantID == yearly && yearly != "":
		billing = "yearly"
	}
	if billing == "" {
		return ""
	}
	id, _ := s.webhookRepo.FindFirstActivePlanID(ctx, billing, currency)
	return id
}

// result builds a WebhookResult that always carries the reference code (the
// payment-completing actions echo it back).
func result(success bool, ref string) WebhookResult {
	return WebhookResult{Success: success, ReferenceCode: &ref}
}

// orUSD defaults a blank currency to "USD" (Node's `currency || 'USD'`).
func orUSD(c string) string {
	if c == "" {
		return "USD"
	}
	return c
}
