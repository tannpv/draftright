// Package paypal implements native recurring subscriptions via the PayPal REST
// API. Ports backend/src/payment/strategies/paypal.strategy.ts.
//
// Why PayPal (in addition to Lemon Squeezy): LS already accepts cards + PayPal
// as a Merchant of Record, but takes a MoR cut on top of processing. A direct
// PayPal integration pays out to the VN business's own PayPal balance at
// PayPal's lower processing rate. USD only — PayPal does not support VND, so
// VN-domestic buyers keep using VietQR.
//
// Flow (Model A, recurring):
//   - CreateCheckout → POST /v1/billing/subscriptions with the billing-plan id
//     (monthly|yearly per plan.BillingPeriod). Our reference code rides along as
//     custom_id; PayPal echoes it on the ACTIVATED webhook so the Service can
//     re-find the pending payment. Returns the "approve" link as the redirect.
//   - VerifyWebhook → verify via PayPal's verify-webhook-signature API (PayPal
//     does NOT sign with HMAC), then dispatch on event_type.
//
// Auth: OAuth2 client-credentials. Access tokens are cached in-memory (mutex —
// unlike Node, VerifyWebhook and CreateCheckout run concurrently here) until a
// 60s safety margin before expiry.
//
// Credentials are read LIVE per call through the injected credsFn — mirroring
// Node's getCredentials() → getSettings(): rotating creds/plans/mode in admin
// Settings takes effect without a restart. (Deliberate divergence from the Go
// LS strategy's boot-frozen creds, in favor of Node-parity behavior.)
package paypal

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

// PayPal REST API hosts, selected by paypal_mode. Fixed infra endpoints —
// named so no literal is repeated (mirrors PayPalStrategy.LIVE_API/SANDBOX_API).
const (
	liveAPI    = "https://api-m.paypal.com"
	sandboxAPI = "https://api-m.sandbox.paypal.com"
)

// Creds is the PayPal credential set, resolved by the caller per call
// (DB-priority with env fallback, mode defaulting to sandbox).
type Creds struct {
	ClientID     string
	ClientSecret string
	WebhookID    string
	PlanMonthly  string
	PlanYearly   string
	Mode         string // "sandbox" | "live"
}

// baseURL selects the API host for the creds' mode.
func (c Creds) baseURL() string {
	if strings.ToLower(c.Mode) == "live" {
		return liveAPI
	}
	return sandboxAPI
}

// Strategy implements strategy.Strategy for PayPal.
type Strategy struct {
	credsFn    func(ctx context.Context) (Creds, error)
	appName    string
	websiteURL string
	http       *http.Client

	// baseOverride (tests only): when non-empty it replaces the live/sandbox
	// host so httptest servers can stand in for PayPal.
	baseOverride string

	mu       sync.Mutex
	tok      string
	tokExpMs int64 // unix ms; mirrors Node's expiresAtMs bookkeeping
}

var _ strategy.Strategy = (*Strategy)(nil)

// New builds a PayPal strategy. credsFn is invoked on every operation (live
// admin-editable credentials); appName brands the checkout + cancel reason;
// websiteURL builds the default return/cancel URLs.
func New(credsFn func(ctx context.Context) (Creds, error), appName, websiteURL string) *Strategy {
	return &Strategy{credsFn: credsFn, appName: appName, websiteURL: websiteURL, http: http.DefaultClient}
}

func (s *Strategy) base(c Creds) string {
	if s.baseOverride != "" {
		return s.baseOverride
	}
	return c.baseURL()
}

// getAccessToken ports getAccessToken: OAuth2 client_credentials, cached until
// ~60s before expiry. Every subsequent REST call authorizes with this Bearer.
func (s *Strategy) getAccessToken(ctx context.Context, c Creds) (string, error) {
	now := time.Now().UnixMilli()
	s.mu.Lock()
	if s.tok != "" && s.tokExpMs > now {
		tok := s.tok
		s.mu.Unlock()
		return tok, nil
	}
	s.mu.Unlock()

	if c.ClientID == "" || c.ClientSecret == "" {
		return "", errors.New("PayPal is not configured. Set the client ID + secret in admin Settings → Payment.")
	}
	basic := base64.StdEncoding.EncodeToString([]byte(c.ClientID + ":" + c.ClientSecret))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.base(c)+"/v1/oauth2/token", strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Basic "+basic)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		slog.Error("PayPal token fetch failed", "status", resp.StatusCode, "body", string(body))
		return "", fmt.Errorf("PayPal auth failed (%d)", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", errors.New("PayPal auth returned no access_token")
	}
	ttl := out.ExpiresIn - 60
	if ttl < 0 {
		ttl = 0
	}
	s.mu.Lock()
	s.tok = out.AccessToken
	s.tokExpMs = now + ttl*1000
	s.mu.Unlock()
	return out.AccessToken, nil
}

// CreateCheckout ports createCheckout: POST /v1/billing/subscriptions and
// return the approve link as the redirect URL.
func (s *Strategy) CreateCheckout(ctx context.Context, p strategy.Payment, plan strategy.Plan, opts strategy.Options) (strategy.Result, error) {
	c, err := s.credsFn(ctx)
	if err != nil {
		return strategy.Result{}, err
	}

	isYearly := plan.BillingPeriod == "yearly"
	planID := c.PlanMonthly
	period := "monthly"
	if isYearly {
		planID = c.PlanYearly
		period = "yearly"
	}
	if planID == "" {
		return strategy.Result{}, fmt.Errorf(
			"No PayPal billing plan configured for %s billing. Set paypal_plan_%s in admin Settings → Payment (run scripts/paypal-create-plans.ts to create them).",
			period, period,
		)
	}

	token, err := s.getAccessToken(ctx, c)
	if err != nil {
		return strategy.Result{}, err
	}

	returnURL := opts.SuccessURL
	if returnURL == "" {
		returnURL = fmt.Sprintf("%s/payment/success?ref=%s", s.websiteURL, p.ReferenceCode)
	}
	cancelURL := opts.CancelURL
	if cancelURL == "" {
		cancelURL = fmt.Sprintf("%s/payment/cancel?ref=%s", s.websiteURL, p.ReferenceCode)
	}

	body := map[string]any{
		"plan_id": planID,
		// Echoed back on the ACTIVATED webhook under resource.custom_id.
		"custom_id": p.ReferenceCode,
		"application_context": map[string]any{
			"brand_name":          s.appName,
			"shipping_preference": "NO_SHIPPING",
			"user_action":         "SUBSCRIBE_NOW",
			"return_url":          returnURL,
			"cancel_url":          cancelURL,
		},
	}
	// Node's `subscriber: undefined` is dropped by JSON.stringify — only emit
	// the key when the email is present.
	if opts.UserEmail != "" {
		body["subscriber"] = map[string]any{"email_address": opts.UserEmail}
	}

	var out struct {
		Links []struct {
			Rel  string `json:"rel"`
			Href string `json:"href"`
		} `json:"links"`
	}
	status, err := s.doJSON(ctx, http.MethodPost, s.base(c)+"/v1/billing/subscriptions", token, body, &out)
	if err != nil {
		return strategy.Result{}, err
	}
	if status < 200 || status >= 300 {
		return strategy.Result{}, fmt.Errorf("PayPal checkout failed (%d)", status)
	}
	for _, l := range out.Links {
		if l.Rel == "approve" && l.Href != "" {
			return strategy.Result{RedirectURL: l.Href}, nil
		}
	}
	return strategy.Result{}, errors.New("PayPal checkout returned no approve link")
}

// CustomerPortalURL ports getCustomerPortalUrl: PayPal has no hosted customer
// portal — cancel is handled via CancelSubscription.
func (s *Strategy) CustomerPortalURL(_ context.Context, _ strategy.PortalUser) (string, error) {
	return "", nil
}

// CancelSubscription ports cancelSubscription (POST
// /v1/billing/subscriptions/{id}/cancel). PayPal keeps access billed-through
// the current period; subscriptions.status flips via the CANCELLED webhook.
// 204 No Content on success; 422 = already cancelled → treat as success.
func (s *Strategy) CancelSubscription(ctx context.Context, subscriptionID string) (bool, error) {
	c, err := s.credsFn(ctx)
	if err != nil {
		return false, err
	}
	token, err := s.getAccessToken(ctx, c)
	if err != nil {
		return false, err
	}
	body := map[string]any{"reason": "Cancelled from " + s.appName}
	status, err := s.doJSON(ctx, http.MethodPost,
		s.base(c)+"/v1/billing/subscriptions/"+subscriptionID+"/cancel", token, body, nil)
	if err != nil {
		return false, err
	}
	if status == http.StatusNoContent || status == http.StatusUnprocessableEntity {
		return true, nil
	}
	slog.Error("PayPal cancel failed", "status", status, "sub", subscriptionID)
	return false, errors.New("Could not cancel the subscription")
}

// fetchNextBillingUnix ports fetchNextBillingUnix: a subscription's
// next_billing_time as unix seconds, or 0 whenever it can't be determined
// (best-effort — a renewal with cpe=0 simply skips the extend).
func (s *Strategy) fetchNextBillingUnix(ctx context.Context, c Creds, subscriptionID string) int64 {
	token, err := s.getAccessToken(ctx, c)
	if err != nil {
		slog.Warn("PayPal next_billing_time fetch failed", "sub", subscriptionID, "err", err)
		return 0
	}
	var out struct {
		BillingInfo struct {
			NextBillingTime string `json:"next_billing_time"`
		} `json:"billing_info"`
	}
	status, err := s.doJSON(ctx, http.MethodGet,
		s.base(c)+"/v1/billing/subscriptions/"+subscriptionID, token, nil, &out)
	if err != nil || status < 200 || status >= 300 {
		if err != nil {
			slog.Warn("PayPal next_billing_time fetch failed", "sub", subscriptionID, "err", err)
		}
		return 0
	}
	return isoToUnix(out.BillingInfo.NextBillingTime)
}

// VerifyWebhook ports verifyWebhook: authenticate via PayPal's
// verify-webhook-signature API, then dispatch on event_type.
func (s *Strategy) VerifyWebhook(ctx context.Context, payload []byte, headers http.Header) (strategy.WebhookAction, error) {
	c, err := s.credsFn(ctx)
	if err != nil {
		return strategy.WebhookAction{}, err
	}
	if c.WebhookID == "" {
		slog.Warn("PayPal webhook called but webhook ID is unset.")
		return strategy.Ignored(), nil
	}

	var event struct {
		EventType string `json:"event_type"`
		Resource  struct {
			ID                 string `json:"id"`
			CustomID           string `json:"custom_id"`
			BillingAgreementID string `json:"billing_agreement_id"`
			BillingInfo        struct {
				NextBillingTime string `json:"next_billing_time"`
			} `json:"billing_info"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		slog.Error("PayPal webhook body is not valid JSON.")
		return strategy.Ignored(), nil
	}
	// PayPal's verify API needs the event echoed verbatim, not our trimmed
	// struct — round-trip the raw payload as opaque JSON.
	var rawEvent json.RawMessage = payload

	verifyBody := map[string]any{
		"auth_algo":         headers.Get("paypal-auth-algo"),
		"cert_url":          headers.Get("paypal-cert-url"),
		"transmission_id":   headers.Get("paypal-transmission-id"),
		"transmission_sig":  headers.Get("paypal-transmission-sig"),
		"transmission_time": headers.Get("paypal-transmission-time"),
		"webhook_id":        c.WebhookID,
		"webhook_event":     rawEvent,
	}
	token, err := s.getAccessToken(ctx, c)
	if err != nil {
		return strategy.WebhookAction{}, err
	}
	var verifyOut struct {
		VerificationStatus string `json:"verification_status"`
	}
	status, err := s.doJSON(ctx, http.MethodPost,
		s.base(c)+"/v1/notifications/verify-webhook-signature", token, verifyBody, &verifyOut)
	if err != nil {
		return strategy.WebhookAction{}, err
	}
	if status < 200 || status >= 300 {
		slog.Error("PayPal verify-webhook-signature rejected", "status", status)
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 400, Message: "PayPal webhook verification failed"}
	}
	if verifyOut.VerificationStatus != "SUCCESS" {
		slog.Error("PayPal webhook signature not verified", "status", verifyOut.VerificationStatus)
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 400, Message: "Invalid PayPal webhook signature"}
	}

	res := event.Resource
	switch event.EventType {
	case "BILLING.SUBSCRIPTION.ACTIVATED":
		// First-cycle activation. custom_id carries our reference code.
		if res.CustomID == "" || res.ID == "" {
			slog.Warn("PayPal ACTIVATED missing custom_id or id — cannot match payment.")
			return strategy.Ignored(), nil
		}
		return strategy.WebhookAction{
			Type:                 strategy.ActionPayPalPaymentSuccess,
			ReferenceCode:        res.CustomID,
			PayPalSubscriptionID: res.ID,
			CurrentPeriodEnd:     isoToUnix(res.BillingInfo.NextBillingTime),
		}, nil

	case "PAYMENT.SALE.COMPLETED":
		// A charge cleared — first cycle OR renewal. billing_agreement_id is
		// the subscription id. No reference code here, so the Service always
		// takes the renewal branch; on the first cycle ACTIVATED already
		// completed the payment, making the extend an idempotent no-op.
		if res.BillingAgreementID == "" {
			return strategy.Ignored(), nil
		}
		return strategy.WebhookAction{
			Type:                 strategy.ActionPayPalPaymentSuccess,
			PayPalSubscriptionID: res.BillingAgreementID,
			CurrentPeriodEnd:     s.fetchNextBillingUnix(ctx, c, res.BillingAgreementID),
		}, nil

	case "BILLING.SUBSCRIPTION.PAYMENT.FAILED":
		if res.ID == "" {
			return strategy.Ignored(), nil
		}
		return strategy.WebhookAction{Type: strategy.ActionPayPalPaymentFailed, PayPalSubscriptionID: res.ID}, nil

	case "BILLING.SUBSCRIPTION.CANCELLED":
		if res.ID == "" {
			return strategy.Ignored(), nil
		}
		return strategy.WebhookAction{Type: strategy.ActionPayPalSubCanceled, PayPalSubscriptionID: res.ID}, nil

	case "BILLING.SUBSCRIPTION.EXPIRED", "BILLING.SUBSCRIPTION.SUSPENDED":
		if res.ID == "" {
			return strategy.Ignored(), nil
		}
		return strategy.WebhookAction{Type: strategy.ActionPayPalSubExpired, PayPalSubscriptionID: res.ID}, nil

	default:
		return strategy.Ignored(), nil
	}
}

// doJSON issues an authorized JSON request and decodes the response into out
// (when non-nil and the body decodes). It returns the HTTP status; callers own
// per-endpoint status semantics (204/422 success on cancel, etc.). Transport
// errors are returned as-is.
func (s *Strategy) doJSON(ctx context.Context, method, url, bearer string, body, out any) (int, error) {
	var rdr io.Reader = bytes.NewReader(nil)
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil && !errors.Is(err, io.EOF) {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

// isoToUnix ports `Math.floor(new Date(iso).getTime()/1000)`; "" → 0.
func isoToUnix(iso string) int64 {
	if iso == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339Nano, iso)
	if err != nil {
		return 0
	}
	return t.Unix()
}
