package payment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

// TestWebhookHandler_Returns201AndBody: a completing payment webhook renders 201
// with {success:true, reference_code:...} (the conditional WebhookResult shape).
func TestWebhookHandler_Returns201AndBody(t *testing.T) {
	repo := &fakeWebhookRepo{pay: &WebhookPayment{
		ID: "p", UserID: "u", PlanID: "pl", Status: "pending",
		Currency: "USD", Method: "stripe", BillingPeriod: "monthly",
	}}
	svc := &Service{
		settings:   fakeSettings{csv: "stripe", found: true},
		strategies: map[string]strategy.Strategy{"stripe": fakeVerifier{action: strategy.WebhookAction{Type: strategy.ActionPaymentCompleted, ReferenceCode: "DR-PRO-AA"}}},
		now:        func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}
	svc.WithWebhook(repo, &fakeSubsWriter{}, &noopEmailer{}, fakeVariants{})
	h := NewHandler(svc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/payment/webhook/stripe", strings.NewReader("{}"))
	h.StripeWebhook(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Success       bool   `json:"success"`
		ReferenceCode string `json:"reference_code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Success || body.ReferenceCode != "DR-PRO-AA" {
		t.Fatalf("bad body: %s", rec.Body.String())
	}
}

// TestWebhookHandler_ProviderErr401Envelope: a 401 WebhookError from the strategy
// renders the canonical invalid-token envelope (status 401), via the new
// writePaymentErr case.
func TestWebhookHandler_ProviderErr401Envelope(t *testing.T) {
	svc := &Service{
		settings:   fakeSettings{csv: "vietqr", found: true},
		strategies: map[string]strategy.Strategy{"vietqr": fakeVerifier{err: &strategy.WebhookError{Status: 401, Message: "Missing webhook authorization"}}},
	}
	svc.WithWebhook(&fakeWebhookRepo{}, &fakeSubsWriter{}, &noopEmailer{}, fakeVariants{})
	h := NewHandler(svc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/payment/webhook/vietqr", strings.NewReader("{}"))
	h.VietQRWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d (%s)", rec.Code, rec.Body.String())
	}
	var env struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != "invalid-token" || env.Error != "Missing webhook authorization" {
		t.Fatalf("bad envelope: %s", rec.Body.String())
	}
}

// TestWebhookHandler_DisabledMethod404: a method not in the enabled set yields a
// 404 (assertEnabled inside HandleWebhook → not-found envelope).
func TestWebhookHandler_DisabledMethod404(t *testing.T) {
	svc := &Service{
		settings:   fakeSettings{csv: "stripe", found: true},
		envCSV:     "stripe",
		strategies: map[string]strategy.Strategy{"stripe": fakeVerifier{}},
	}
	svc.WithWebhook(&fakeWebhookRepo{}, &fakeSubsWriter{}, &noopEmailer{}, fakeVariants{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/payment/webhook/lemonsqueezy", strings.NewReader("{}"))
	NewHandler(svc).LemonSqueezyWebhook(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", rec.Code, rec.Body.String())
	}
}
