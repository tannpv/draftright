package payment

import (
	"io"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// webhook is the shared body for the 5 public webhook routes. It reads the raw
// payload (signature verification needs the exact bytes), dispatches to the
// Service, and renders 201 on success — matching the Node controller, which
// returns the result from a POST with no @HttpCode override.
func (h *Handler) webhook(w http.ResponseWriter, r *http.Request, method string) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	res, err := h.svc.HandleWebhook(r.Context(), method, payload, r.Header)
	if err != nil {
		if writePaymentErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "webhook failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, res)
}

// StripeWebhook: POST /payment/webhook/stripe (public).
func (h *Handler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	h.webhook(w, r, string(MethodStripe))
}

// VietQRWebhook: POST /payment/webhook/vietqr (public).
func (h *Handler) VietQRWebhook(w http.ResponseWriter, r *http.Request) {
	h.webhook(w, r, string(MethodVietQR))
}

// CassoWebhook: POST /payment/webhook/casso (public). Casso is a vietqr
// auto-confirm source — same handler method as vietqr.
func (h *Handler) CassoWebhook(w http.ResponseWriter, r *http.Request) {
	h.webhook(w, r, string(MethodVietQR))
}

// SepayWebhook: POST /payment/webhook/sepay (public). SePay is a vietqr
// auto-confirm source — same handler method as vietqr.
func (h *Handler) SepayWebhook(w http.ResponseWriter, r *http.Request) {
	h.webhook(w, r, string(MethodVietQR))
}

// LemonSqueezyWebhook: POST /payment/webhook/lemonsqueezy (public).
func (h *Handler) LemonSqueezyWebhook(w http.ResponseWriter, r *http.Request) {
	h.webhook(w, r, string(MethodLemonSqueezy))
}
