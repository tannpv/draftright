package payment

import (
	"context"

	stripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/client"
)

// stripeRefunder performs the Stripe-side refund operations for admin refunds.
// Consumer-side port (the refund use case owns it). The real impl builds a
// fresh Stripe client per call from the secret key (mirrors Node's
// `new Stripe(secretKey)` per request).
type stripeRefunder interface {
	// LatestInvoiceCharge retrieves the subscription's latest invoice and
	// returns its charge id. Mirrors Node:
	//   sub = stripe.subscriptions.retrieve(subID, {expand:['latest_invoice']})
	//   chargeID = sub.latest_invoice.charge || sub.latest_invoice.payments.data[0].payment.charge
	// Returns ("", nil) when no charge is resolvable (Node logs a warn + skips refund).
	LatestInvoiceCharge(ctx context.Context, secretKey, subID string) (chargeID string, err error)
	// CreateRefund creates a refund for the charge. Mirrors
	//   stripe.refunds.create({charge, reason})
	// Returns the refund id.
	CreateRefund(ctx context.Context, secretKey, chargeID, reason string) (refundID string, err error)
	// CancelSubscription cancels the subscription IMMEDIATELY (access ends now).
	// Mirrors Node `stripe.subscriptions.cancel(subID)` — NOT cancel_at_period_end.
	CancelSubscription(ctx context.Context, secretKey, subID string) error
}

// stripeSDKRefunder is the real stripeRefunder. It builds a fresh Stripe client
// per call from the secret key (mirrors Node's `new Stripe(secretKey)`).
// newClient is a test seam mirroring the strategy package's pattern.
type stripeSDKRefunder struct {
	newClient func(key string) *client.API
}

var _ stripeRefunder = (*stripeSDKRefunder)(nil)

func newStripeSDKRefunder() *stripeSDKRefunder {
	return &stripeSDKRefunder{newClient: func(key string) *client.API {
		return client.New(key, nil)
	}}
}

// LatestInvoiceCharge ports Node payment.service.ts:
//
//	const subObj = await stripe.subscriptions.retrieve(stripeSubId, { expand: ['latest_invoice'] });
//	const invoiceObj = subObj.latest_invoice;
//	const chargeId = invoiceObj?.charge || invoiceObj?.payments?.data?.[0]?.payment?.charge;
//
// v82 fields read:
//   - sub.LatestInvoice              ← mirrors subObj.latest_invoice
//   - invoice.Payments.Data[0].Payment.Charge.ID
//     ← mirrors invoiceObj.payments.data[0].payment.charge
//
// v82 removed the top-level typed Invoice.Charge field (the new Stripe API
// moved the charge under invoice payments), so Node's first `invoiceObj?.charge`
// branch has no typed equivalent and we only follow the payments path. When no
// charge is resolvable we return ("", nil) — NOT an error — so the usecase
// logs-and-skips exactly like Node's warn branch.
func (r *stripeSDKRefunder) LatestInvoiceCharge(_ context.Context, secretKey, subID string) (string, error) {
	sc := r.newClient(secretKey)
	params := &stripe.SubscriptionParams{}
	params.AddExpand("latest_invoice")
	sub, err := sc.Subscriptions.Get(subID, params)
	if err != nil {
		return "", err
	}
	inv := sub.LatestInvoice
	if inv == nil || inv.Payments == nil || len(inv.Payments.Data) == 0 {
		return "", nil
	}
	pay := inv.Payments.Data[0].Payment
	if pay == nil || pay.Charge == nil {
		return "", nil
	}
	return pay.Charge.ID, nil
}

// CreateRefund ports `stripe.refunds.create({ charge: chargeId, reason })`.
// reason is only set when non-empty: Stripe rejects an empty reason enum, and
// Node passes `reason` which is undefined when absent.
func (r *stripeSDKRefunder) CreateRefund(_ context.Context, secretKey, chargeID, reason string) (string, error) {
	sc := r.newClient(secretKey)
	params := &stripe.RefundParams{Charge: stripe.String(chargeID)}
	if reason != "" {
		params.Reason = stripe.String(reason)
	}
	refund, err := sc.Refunds.New(params)
	if err != nil {
		return "", err
	}
	return refund.ID, nil
}

// CancelSubscription ports `stripe.subscriptions.cancel(stripeSubId)` — an
// immediate cancel (access ends now), NOT cancel_at_period_end.
func (r *stripeSDKRefunder) CancelSubscription(_ context.Context, secretKey, subID string) error {
	sc := r.newClient(secretKey)
	_, err := sc.Subscriptions.Cancel(subID, nil)
	return err
}
