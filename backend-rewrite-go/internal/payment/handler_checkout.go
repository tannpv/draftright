package payment

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// paymentMethods is the full PaymentMethod enum in NestJS declaration order
// (@IsIn(PAYMENT_METHODS)). The checkout validator and its humanized message
// both depend on this order.
var paymentMethods = []string{
	"stripe", "paypal", "momo", "vietqr", "bank_transfer", "lemonsqueezy", "apple_pay", "google_pay",
}

// checkoutBody mirrors NestJS CreateCheckoutDto.
type checkoutBody struct {
	PlanID     string `json:"plan_id"`
	Method     string `json:"method"`
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

// validateCheckout reproduces the ValidationPipe humanizer for CreateCheckoutDto
// in property-declaration order (plan_id, then method). success_url/cancel_url
// are @IsOptional @IsString — a JSON string body can't violate them, so they're
// not validated. Messages join with ". "; empty result = valid.
func validateCheckout(b checkoutBody) string {
	var msgs []string
	if b.PlanID == "" {
		msgs = append(msgs, "plan_id must be a string")
	}
	if !contains(paymentMethods, b.Method) {
		msgs = append(msgs, "method must be one of the following values: "+strings.Join(paymentMethods, ", "))
	}
	return strings.Join(msgs, ". ")
}

// writePaymentErr renders a known *DomainError as the canonical envelope:
// 404 → not-found, 400 → invalid-input. Returns false (unhandled) for any
// other status or non-DomainError so callers fall through to a 500. Mirrors
// auth's writeDomainErr pattern.
func writePaymentErr(w http.ResponseWriter, r *http.Request, err error) bool {
	var derr *DomainError
	if !errors.As(err, &derr) {
		return false
	}
	var code string
	switch derr.Status {
	case 404:
		code = "not-found"
	case 400:
		code = "invalid-input"
	case 500:
		code = "internal"
	default:
		return false
	}
	shared.WriteError(w, r, code, derr.Message)
	return true
}

// Checkout: POST /payment/checkout (JWT) → 201 CheckoutResponse.
func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	var body checkoutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	if msg := validateCheckout(body); msg != "" {
		shared.WriteError(w, r, "invalid-input", msg)
		return
	}
	resp, err := h.svc.CreateCheckout(r.Context(), claims.Sub, body.PlanID, body.Method, CheckoutOptions{
		SuccessURL: body.SuccessURL, CancelURL: body.CancelURL,
	})
	if err != nil {
		if writePaymentErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "checkout failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, resp)
}

// Portal: GET /payment/portal (JWT) → 200 {"url": <string>}.
func (h *Handler) Portal(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	url, err := h.svc.CustomerPortalURL(r.Context(), claims.Sub)
	if err != nil {
		if writePaymentErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "portal failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"url": url})
}

// CancelSubscription: DELETE /payment/subscription (JWT) → 200
// {cancelled, expires_at} (CancelResult.MarshalJSON pins the shape).
func (h *Handler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	out, err := h.svc.CancelActiveSubscription(r.Context(), claims.Sub)
	if err != nil {
		if writePaymentErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "cancel failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, out)
}
