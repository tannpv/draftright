// Package stripe implements the Stripe payment strategy: subscription Checkout
// Sessions for the `stripe` method and PaymentIntents for the apple_pay /
// google_pay wallet methods, plus the billing-portal + cancel flows. Uses the
// official stripe-go client.API.
//
// Ports backend/src/payment/strategies/stripe.strategy.ts (Node) 1:1. All
// user-visible strings (error messages, the U+00B7 middle dot in displayLabel)
// are byte-identical to the Node source.
package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	stripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/client"
	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

// Creds ports the AppSettings-resolved Stripe credentials (env fallback applied
// upstream by the Service via strategy.ResolveCredential).
type Creds struct {
	SecretKey     string
	WebhookSecret string
}

// Env ports the env-only wallet config (no DB column yet in Node).
type Env struct {
	PublishableKey     string
	ApplePayMerchantID string
	WebsiteURL         string
}

// Strategy is the Stripe provider. newClient is a test seam so unit tests never
// hit the network.
type Strategy struct {
	creds     Creds
	env       Env
	newClient func(key string) *client.API
}

var _ strategy.Strategy = (*Strategy)(nil)

func New(c Creds, e Env) *Strategy {
	return &Strategy{creds: c, env: e, newClient: func(key string) *client.API {
		return client.New(key, nil)
	}}
}

func (s *Strategy) websiteURL() string {
	if s.env.WebsiteURL != "" {
		return s.env.WebsiteURL
	}
	return "http://localhost:4000"
}

// CreateCheckout ports createCheckout. Wallet methods (apple_pay/google_pay)
// branch to a PaymentIntent before the price check (wallets bill an amount, not
// a pre-created Price).
func (s *Strategy) CreateCheckout(ctx context.Context, p strategy.Payment, plan strategy.Plan, opts strategy.Options) (strategy.Result, error) {
	if s.creds.SecretKey == "" {
		// Node line 56 — byte-identical.
		return strategy.Result{}, fmt.Errorf("Stripe is not configured. Set STRIPE_SECRET_KEY in admin Settings → Payment.")
	}
	sc := s.newClient(s.creds.SecretKey)

	if p.Method == "apple_pay" || p.Method == "google_pay" {
		return s.walletIntent(ctx, sc, p, plan, opts)
	}

	if plan.StripePriceID == "" {
		// Node line 79 — `Plan ${plan.id} has no stripe_price_id...`. The Go port's
		// Plan has no ID field; plan.id == payment.plan_id, so we interpolate PlanID.
		return strategy.Result{}, fmt.Errorf("Plan %s has no stripe_price_id. Run scripts/sync-stripe-prices.ts.", p.PlanID)
	}

	successURL := opts.SuccessURL
	if successURL == "" {
		successURL = fmt.Sprintf("%s/payment/success?ref=%s", s.websiteURL(), p.ReferenceCode)
	}
	cancelURL := opts.CancelURL
	if cancelURL == "" {
		cancelURL = s.websiteURL() + "/payment/cancel"
	}

	params := &stripe.CheckoutSessionParams{
		Mode:               stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(plan.StripePriceID), Quantity: stripe.Int64(1)},
		},
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		ClientReferenceID: stripe.String(p.UserID),
		SubscriptionData:  &stripe.CheckoutSessionSubscriptionDataParams{},
	}
	// Session metadata (Node lines 87-92).
	params.AddMetadata("reference_code", p.ReferenceCode)
	params.AddMetadata("payment_id", p.ID)
	params.AddMetadata("user_id", p.UserID)
	params.AddMetadata("plan_id", p.PlanID)
	// Subscription metadata (Node lines 94-98) — no payment_id, matching Node.
	params.SubscriptionData.AddMetadata("reference_code", p.ReferenceCode)
	params.SubscriptionData.AddMetadata("user_id", p.UserID)
	params.SubscriptionData.AddMetadata("plan_id", p.PlanID)
	if plan.TrialDays > 0 {
		params.SubscriptionData.TrialPeriodDays = stripe.Int64(int64(plan.TrialDays))
	}

	// Customer reuse (Node lines 113-117).
	if opts.StripeCustomerID != "" {
		params.Customer = stripe.String(opts.StripeCustomerID)
	} else if opts.UserEmail != "" {
		params.CustomerEmail = stripe.String(opts.UserEmail)
	}

	sess, err := sc.CheckoutSessions.New(params)
	if err != nil {
		return strategy.Result{}, err
	}
	return strategy.Result{RedirectURL: sess.URL}, nil
}

// walletIntent ports createWalletPaymentIntent (Node lines 138-210): reuse or
// create a Stripe Customer keyed by email, then create an off_session
// PaymentIntent and return the wallet handshake block.
func (s *Strategy) walletIntent(_ context.Context, sc *client.API, p strategy.Payment, plan strategy.Plan, opts strategy.Options) (strategy.Result, error) {
	// Reuse existing Customer or create one keyed by user_email (Node 146-153).
	customerID := opts.StripeCustomerID
	if customerID == "" {
		custParams := &stripe.CustomerParams{}
		if opts.UserEmail != "" {
			custParams.Email = stripe.String(opts.UserEmail)
		}
		custParams.AddMetadata("user_id", p.UserID)
		cust, err := sc.Customers.New(custParams)
		if err != nil {
			return strategy.Result{}, err
		}
		customerID = cust.ID
	}

	piParams := &stripe.PaymentIntentParams{
		Amount:             stripe.Int64(int64(p.Amount)),
		Currency:           stripe.String(strings.ToLower(p.Currency)),
		Customer:           stripe.String(customerID),
		SetupFutureUsage:   stripe.String("off_session"),
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
	}
	piParams.AddMetadata("reference_code", p.ReferenceCode)
	piParams.AddMetadata("payment_id", p.ID)
	piParams.AddMetadata("user_id", p.UserID)
	piParams.AddMetadata("plan_id", p.PlanID)
	piParams.AddMetadata("method", p.Method)
	pi, err := sc.PaymentIntents.New(piParams)
	if err != nil {
		return strategy.Result{}, err
	}

	wi := &strategy.WalletIntent{
		ClientSecret:   pi.ClientSecret,
		PublishableKey: s.env.PublishableKey,
		// Node uses plan?.country_code || 'US'; the Go Plan has no country_code → "US".
		CountryCode:   "US",
		CurrencyCode:  strings.ToUpper(p.Currency),
		DisplayAmount: displayAmount(p.Currency, p.Amount),
		DisplayLabel:  displayLabel(plan.Name, plan.BillingPeriod),
	}
	if s.env.ApplePayMerchantID != "" {
		wi.MerchantIdentifier = s.env.ApplePayMerchantID
	}
	return strategy.Result{WalletIntent: wi}, nil
}

// CustomerPortalURL ports getCustomerPortalUrl: "" when the user has no Stripe
// customer, else a one-shot billing-portal URL.
func (s *Strategy) CustomerPortalURL(_ context.Context, u strategy.PortalUser) (string, error) {
	if u.StripeCustomerID == "" {
		return "", nil
	}
	if s.creds.SecretKey == "" {
		// Node line 221 — `throw new Error('Stripe is not configured.')`.
		return "", fmt.Errorf("Stripe is not configured.")
	}
	sc := s.newClient(s.creds.SecretKey)
	sess, err := sc.BillingPortalSessions.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(u.StripeCustomerID),
		ReturnURL: stripe.String(s.websiteURL() + "/account?subscribed=1"),
	})
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

// CancelSubscription ports cancelSubscription: cancel_at_period_end = true.
func (s *Strategy) CancelSubscription(_ context.Context, subscriptionID string) (bool, error) {
	if s.creds.SecretKey == "" {
		// Node line 240 — `throw new Error('Stripe is not configured.')`.
		return false, fmt.Errorf("Stripe is not configured.")
	}
	sc := s.newClient(s.creds.SecretKey)
	_, err := sc.Subscriptions.Update(subscriptionID, &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// VerifyWebhook ports StripeStrategy.verifyWebhook. Unset secret_key/webhook_secret
// → Ignored. A present-but-invalid signature → 400 "Invalid Stripe webhook
// signature". Recognised events map to actions; everything else → Ignored.
//
// Uses stripe.ConstructEvent with WithIgnoreAPIVersionMismatch() so the handler
// stays forward-compatible across Stripe API version upgrades (Node's
// constructEvent has the same behaviour — it never checks the API version).
func (s *Strategy) VerifyWebhook(ctx context.Context, payload []byte, headers http.Header) (strategy.WebhookAction, error) {
	if s.creds.SecretKey == "" || s.creds.WebhookSecret == "" {
		return strategy.Ignored(), nil
	}
	sig := headers.Get("Stripe-Signature")
	event, err := stripe.ConstructEvent(payload, sig, s.creds.WebhookSecret, stripe.WithIgnoreAPIVersionMismatch())
	if err != nil {
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 400, Message: "Invalid Stripe webhook signature"}
	}

	// event.Data.Raw is tagged json:"object" in EventData — it contains the
	// raw JSON of data.object (the API resource), not the full data envelope.
	// This mirrors Node's `event.data.object`.
	var obj map[string]any
	_ = json.Unmarshal(event.Data.Raw, &obj)

	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		// Initial checkout success — provisions subscription for the first time.
		// Node lines 286-293: session.metadata?.reference_code, session.subscription, session.customer.
		return strategy.WebhookAction{
			Type:                 strategy.ActionPaymentCompleted,
			ReferenceCode:        digStr(obj, "metadata", "reference_code"),
			StripeSubscriptionID: str(obj["subscription"]),
			StripeCustomerID:     str(obj["customer"]),
		}, nil

	case stripe.EventTypePaymentIntentSucceeded:
		// Native-wallet checkout success (Apple Pay / Google Pay path).
		// Node lines 304-314: ignore if no reference_code.
		ref := digStr(obj, "metadata", "reference_code")
		if ref == "" {
			return strategy.Ignored(), nil
		}
		return strategy.WebhookAction{
			Type:             strategy.ActionPaymentCompleted,
			ReferenceCode:    ref,
			StripeCustomerID: str(obj["customer"]),
		}, nil

	case stripe.EventTypeInvoicePaymentSucceeded:
		// Subscription renewal — extends expiry.
		// Node lines 322-343: new path (invoice.parent.subscription_details.subscription)
		// then legacy path (invoice.subscription); same dual-path for period_end.
		subID := firstStr(
			digStr(obj, "parent", "subscription_details", "subscription"),
			str(obj["subscription"]),
		)
		line := firstLine(obj)
		periodEnd := firstInt(
			digInt(line, "period", "end"),
			digInt(line, "parent", "subscription_item_details", "subscription_period", "end"),
			toInt(obj["period_end"]),
		)
		if subID == "" || periodEnd == 0 {
			return strategy.Ignored(), nil
		}
		return strategy.WebhookAction{
			Type:                 strategy.ActionSubscriptionRenewed,
			StripeSubscriptionID: subID,
			CurrentPeriodEnd:     int64(periodEnd),
		}, nil

	case stripe.EventTypeInvoicePaymentFailed:
		// Renewal charge failed — Node lines 350-358.
		// New path: invoice.parent.subscription_details.metadata.reference_code
		// Legacy path: invoice.subscription_details.metadata.reference_code
		// Fallback: invoice.id
		ref := firstStr(
			digStr(obj, "parent", "subscription_details", "metadata", "reference_code"),
			digStr(obj, "subscription_details", "metadata", "reference_code"),
			str(obj["id"]),
		)
		return strategy.WebhookAction{Type: strategy.ActionPaymentFailed, ReferenceCode: ref}, nil

	case stripe.EventTypeCustomerSubscriptionDeleted:
		// Final cancellation — Node lines 361-366: sub.id.
		return strategy.WebhookAction{Type: strategy.ActionSubscriptionCanceled, StripeSubscriptionID: str(obj["id"])}, nil

	case stripe.EventTypeChargeDisputeCreated:
		// Chargeback — Node lines 370-376: dispute.charge, dispute.amount.
		return strategy.WebhookAction{
			Type:           strategy.ActionDisputeCreated,
			StripeChargeID: str(obj["charge"]),
			Amount:         toInt(obj["amount"]),
		}, nil

	default:
		return strategy.Ignored(), nil
	}
}

// str safely type-asserts any to string; returns "" for non-strings.
func str(v any) string { s, _ := v.(string); return s }

// toInt converts a JSON-unmarshalled numeric (float64) or int to int.
func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

// firstStr returns the first non-empty string from xs.
func firstStr(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}

// firstInt returns the first non-zero int from xs.
func firstInt(xs ...int) int {
	for _, x := range xs {
		if x != 0 {
			return x
		}
	}
	return 0
}

// digStr walks nested map[string]any by keys, returning the leaf string or "".
func digStr(m map[string]any, keys ...string) string {
	cur := any(m)
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = mm[k]
	}
	return str(cur)
}

// digInt walks nested map[string]any by keys, returning the leaf int or 0.
func digInt(m map[string]any, keys ...string) int {
	cur := any(m)
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return 0
		}
		cur = mm[k]
	}
	return toInt(cur)
}

// firstLine returns invoice.lines.data[0] as a map, or nil.
func firstLine(obj map[string]any) map[string]any {
	lines, ok := obj["lines"].(map[string]any)
	if !ok {
		return nil
	}
	data, ok := lines["data"].([]any)
	if !ok || len(data) == 0 {
		return nil
	}
	m, _ := data[0].(map[string]any)
	return m
}

var zeroDecimal = map[string]bool{"VND": true, "JPY": true, "KRW": true}

// displayAmount ports the isZeroDecimal block (Node 189-192): zero-decimal
// currencies emit the integer amount; others emit amount/100 to 2 decimals.
func displayAmount(currency string, amount int) string {
	if zeroDecimal[strings.ToUpper(currency)] {
		return strconv.Itoa(amount)
	}
	return strconv.FormatFloat(float64(amount)/100, 'f', 2, 64)
}

// displayLabel ports the cadence label (Node 193-196): empty name → "Pro"; a
// non-empty cadence is title-cased and joined with " · " (U+00B7). The Go port
// uses "none" as the no-cadence sentinel where Node uses an empty string.
func displayLabel(name, billingPeriod string) string {
	if name == "" {
		name = "Pro"
	}
	if billingPeriod == "" || billingPeriod == "none" {
		return name
	}
	cadence := strings.ToUpper(billingPeriod[:1]) + billingPeriod[1:]
	return name + " · " + cadence
}
