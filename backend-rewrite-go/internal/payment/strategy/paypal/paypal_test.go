package paypal

// Port of backend/src/payment/strategies/paypal.strategy.spec.ts — the same
// scenarios exercised against an httptest stand-in for the PayPal REST API
// (baseOverride replaces the live/sandbox host).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

// fixedCreds returns a credsFn yielding the same creds every call.
func fixedCreds(c Creds) func(context.Context) (Creds, error) {
	return func(context.Context) (Creds, error) { return c, nil }
}

// newTestStrategy builds a Strategy pointed at the given httptest server.
func newTestStrategy(t *testing.T, srv *httptest.Server, c Creds) *Strategy {
	t.Helper()
	s := New(fixedCreds(c), "DraftRight", "https://draftright.info")
	s.baseOverride = srv.URL
	return s
}

// paypalAPI is a configurable fake PayPal server. tokenCalls counts OAuth
// hits (for the cache test).
type paypalAPI struct {
	tokenCalls   atomic.Int64
	verifyStatus string // verification_status returned by verify endpoint
	verifyHTTP   int    // HTTP status of verify endpoint (0 → 200)
	cancelHTTP   int    // HTTP status of cancel endpoint
	nextBilling  string // billing_info.next_billing_time on GET subscription
	approveHref  string // approve link returned on create subscription
	createHTTP   int    // HTTP status of create subscription (0 → 201)
}

func (f *paypalAPI) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		f.tokenCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok_1", "expires_in": 3600})
	})
	mux.HandleFunc("/v1/notifications/verify-webhook-signature", func(w http.ResponseWriter, r *http.Request) {
		if f.verifyHTTP != 0 {
			w.WriteHeader(f.verifyHTTP)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"verification_status": f.verifyStatus})
	})
	mux.HandleFunc("/v1/billing/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		if f.createHTTP != 0 {
			w.WriteHeader(f.createHTTP)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "I-NEW", "links": []map[string]string{
				{"rel": "self", "href": "https://x/self"},
				{"rel": "approve", "href": f.approveHref},
			},
		})
	})
	mux.HandleFunc("/v1/billing/subscriptions/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/cancel") {
			w.WriteHeader(f.cancelHTTP)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"billing_info": map[string]any{"next_billing_time": f.nextBilling},
		})
	})
	return mux
}

var baseCreds = Creds{
	ClientID: "cid", ClientSecret: "csec", WebhookID: "wh_1",
	PlanMonthly: "P-MONTH", PlanYearly: "P-YEAR", Mode: "sandbox",
}

func TestVerifyWebhook_UnsetWebhookIDIgnores(t *testing.T) {
	api := &paypalAPI{verifyStatus: "SUCCESS"}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	c := baseCreds
	c.WebhookID = ""
	s := newTestStrategy(t, srv, c)

	action, err := s.VerifyWebhook(context.Background(), []byte(`{}`), http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != strategy.ActionIgnored {
		t.Fatalf("unset webhook id must ignore, got %q", action.Type)
	}
	if n := api.tokenCalls.Load(); n != 0 {
		t.Fatalf("must not call PayPal at all, token calls=%d", n)
	}
}

func TestVerifyWebhook_FailureStatusRejects(t *testing.T) {
	api := &paypalAPI{verifyStatus: "FAILURE"}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	s := newTestStrategy(t, srv, baseCreds)

	_, err := s.VerifyWebhook(context.Background(), []byte(`{"event_type":"BILLING.SUBSCRIPTION.ACTIVATED"}`), http.Header{})
	var we *strategy.WebhookError
	if !errors.As(err, &we) || we.Status != 400 || we.Message != "Invalid PayPal webhook signature" {
		t.Fatalf("want 400 invalid-signature WebhookError, got %v", err)
	}
}

func TestVerifyWebhook_VerifyEndpointErrorRejects(t *testing.T) {
	api := &paypalAPI{verifyHTTP: http.StatusServiceUnavailable}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	s := newTestStrategy(t, srv, baseCreds)

	_, err := s.VerifyWebhook(context.Background(), []byte(`{"event_type":"X"}`), http.Header{})
	var we *strategy.WebhookError
	if !errors.As(err, &we) || we.Status != 400 || we.Message != "PayPal webhook verification failed" {
		t.Fatalf("want 400 verification-failed WebhookError, got %v", err)
	}
}

func TestVerifyWebhook_ActivatedMapsToPaymentSuccess(t *testing.T) {
	api := &paypalAPI{verifyStatus: "SUCCESS"}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	s := newTestStrategy(t, srv, baseCreds)

	payload := []byte(`{
		"event_type": "BILLING.SUBSCRIPTION.ACTIVATED",
		"resource": {
			"id": "I-SUB-9", "custom_id": "DR-PRO-PP",
			"billing_info": {"next_billing_time": "2027-01-01T00:00:00Z"}
		}
	}`)
	action, err := s.VerifyWebhook(context.Background(), payload, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != strategy.ActionPayPalPaymentSuccess ||
		action.ReferenceCode != "DR-PRO-PP" ||
		action.PayPalSubscriptionID != "I-SUB-9" ||
		action.CurrentPeriodEnd != 1798761600 {
		t.Fatalf("bad action: %+v", action)
	}
}

func TestVerifyWebhook_ActivatedMissingCustomIDIgnores(t *testing.T) {
	api := &paypalAPI{verifyStatus: "SUCCESS"}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	s := newTestStrategy(t, srv, baseCreds)

	payload := []byte(`{"event_type":"BILLING.SUBSCRIPTION.ACTIVATED","resource":{"id":"I-SUB-9"}}`)
	action, err := s.VerifyWebhook(context.Background(), payload, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != strategy.ActionIgnored {
		t.Fatalf("missing custom_id must ignore, got %q", action.Type)
	}
}

func TestVerifyWebhook_SaleCompletedFetchesNextBilling(t *testing.T) {
	api := &paypalAPI{verifyStatus: "SUCCESS", nextBilling: "2027-06-01T00:00:00Z"}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	s := newTestStrategy(t, srv, baseCreds)

	payload := []byte(`{"event_type":"PAYMENT.SALE.COMPLETED","resource":{"billing_agreement_id":"I-SUB-1"}}`)
	action, err := s.VerifyWebhook(context.Background(), payload, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != strategy.ActionPayPalPaymentSuccess ||
		action.ReferenceCode != "" ||
		action.PayPalSubscriptionID != "I-SUB-1" ||
		action.CurrentPeriodEnd != 1811808000 {
		t.Fatalf("bad renewal action: %+v", action)
	}
}

func TestVerifyWebhook_LifecycleEvents(t *testing.T) {
	api := &paypalAPI{verifyStatus: "SUCCESS"}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	s := newTestStrategy(t, srv, baseCreds)

	cases := []struct {
		event string
		want  string
	}{
		{"BILLING.SUBSCRIPTION.PAYMENT.FAILED", strategy.ActionPayPalPaymentFailed},
		{"BILLING.SUBSCRIPTION.CANCELLED", strategy.ActionPayPalSubCanceled},
		{"BILLING.SUBSCRIPTION.EXPIRED", strategy.ActionPayPalSubExpired},
		{"BILLING.SUBSCRIPTION.SUSPENDED", strategy.ActionPayPalSubExpired},
		{"SOMETHING.ELSE", strategy.ActionIgnored},
	}
	for _, tc := range cases {
		payload := []byte(`{"event_type":"` + tc.event + `","resource":{"id":"I-SUB-X"}}`)
		action, err := s.VerifyWebhook(context.Background(), payload, http.Header{})
		if err != nil {
			t.Fatalf("%s: %v", tc.event, err)
		}
		if action.Type != tc.want {
			t.Fatalf("%s → %q, want %q", tc.event, action.Type, tc.want)
		}
		if tc.want != strategy.ActionIgnored && action.PayPalSubscriptionID != "I-SUB-X" {
			t.Fatalf("%s: missing sub id: %+v", tc.event, action)
		}
	}
}

func TestCreateCheckout_ReturnsApproveLink(t *testing.T) {
	api := &paypalAPI{approveHref: "https://www.sandbox.paypal.com/approve/ABC"}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	s := newTestStrategy(t, srv, baseCreds)

	res, err := s.CreateCheckout(context.Background(),
		strategy.Payment{ReferenceCode: "DR-PRO-XY"},
		strategy.Plan{BillingPeriod: "monthly"},
		strategy.Options{UserEmail: "u@x.com"})
	if err != nil {
		t.Fatal(err)
	}
	if res.RedirectURL != "https://www.sandbox.paypal.com/approve/ABC" {
		t.Fatalf("bad redirect: %+v", res)
	}
}

func TestCreateCheckout_MissingPlanIDErrors(t *testing.T) {
	api := &paypalAPI{}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	c := baseCreds
	c.PlanYearly = ""
	s := newTestStrategy(t, srv, c)

	_, err := s.CreateCheckout(context.Background(),
		strategy.Payment{ReferenceCode: "DR-PRO-XY"},
		strategy.Plan{BillingPeriod: "yearly"}, strategy.Options{})
	want := "No PayPal billing plan configured for yearly billing. Set paypal_plan_yearly in admin Settings → Payment (run scripts/paypal-create-plans.ts to create them)."
	if err == nil || err.Error() != want {
		t.Fatalf("want exact Node message, got %v", err)
	}
	if n := api.tokenCalls.Load(); n != 0 {
		t.Fatalf("must fail before hitting PayPal, token calls=%d", n)
	}
}

func TestCreateCheckout_UnconfiguredErrors(t *testing.T) {
	api := &paypalAPI{}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	c := baseCreds
	c.ClientID, c.ClientSecret = "", ""
	s := newTestStrategy(t, srv, c)

	_, err := s.CreateCheckout(context.Background(),
		strategy.Payment{ReferenceCode: "DR-PRO-XY"},
		strategy.Plan{BillingPeriod: "monthly"}, strategy.Options{})
	want := "PayPal is not configured. Set the client ID + secret in admin Settings → Payment."
	if err == nil || err.Error() != want {
		t.Fatalf("want not-configured message, got %v", err)
	}
}

func TestCancelSubscription_StatusSemantics(t *testing.T) {
	cases := []struct {
		httpStatus int
		ok         bool
		wantErr    string
	}{
		{http.StatusNoContent, true, ""},
		{http.StatusUnprocessableEntity, true, ""}, // already cancelled → success
		{http.StatusInternalServerError, false, "Could not cancel the subscription"},
	}
	for _, tc := range cases {
		api := &paypalAPI{cancelHTTP: tc.httpStatus}
		srv := httptest.NewServer(api.handler())
		s := newTestStrategy(t, srv, baseCreds)
		ok, err := s.CancelSubscription(context.Background(), "I-SUB-2")
		srv.Close()
		if ok != tc.ok {
			t.Fatalf("status %d: ok=%v want %v", tc.httpStatus, ok, tc.ok)
		}
		if tc.wantErr == "" && err != nil {
			t.Fatalf("status %d: unexpected err %v", tc.httpStatus, err)
		}
		if tc.wantErr != "" && (err == nil || err.Error() != tc.wantErr) {
			t.Fatalf("status %d: want %q, got %v", tc.httpStatus, tc.wantErr, err)
		}
	}
}

func TestAccessToken_CachedAcrossCalls(t *testing.T) {
	api := &paypalAPI{cancelHTTP: http.StatusNoContent}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	s := newTestStrategy(t, srv, baseCreds)

	for i := 0; i < 3; i++ {
		if _, err := s.CancelSubscription(context.Background(), "I-SUB-2"); err != nil {
			t.Fatal(err)
		}
	}
	if n := api.tokenCalls.Load(); n != 1 {
		t.Fatalf("token must be fetched once and cached, got %d calls", n)
	}
}
