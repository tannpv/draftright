package payment

import (
	"io"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// readWebhookBody reads the raw request body a webhook needs for signature
// verification (which needs the exact bytes, not a re-marshaled struct) and
// writes the canonical 400 on a read failure. Shared by every webhook route —
// the 6 gated providers via webhook() below, and the ungated AppleWebhook
// (handler_apple.go) — so the "how do we read a webhook body" answer has one
// source of truth (Rule #1).
func readWebhookBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		shared.WriteError(w, r, shared.CodeInvalidInput, "Invalid request body")
		return nil, err
	}
	return payload, nil
}

// webhook is the shared body for the 6 gated public webhook routes. It reads the
// raw payload, dispatches to the Service, and renders 201 on success —
// matching the Node controller, which returns the result from a POST with no
// @HttpCode override.
func (h *Handler) webhook(w http.ResponseWriter, r *http.Request, method string) {
	payload, err := readWebhookBody(w, r)
	if err != nil {
		return
	}
	res, err := h.svc.HandleWebhook(r.Context(), method, payload, r.Header)
	if err != nil {
		if writePaymentErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, shared.CodeInternal, "webhook failed")
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

// PayPalWebhook: POST /payment/webhook/paypal (public).
func (h *Handler) PayPalWebhook(w http.ResponseWriter, r *http.Request) {
	h.webhook(w, r, string(MethodPayPal))
}
