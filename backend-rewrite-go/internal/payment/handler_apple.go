package payment

import (
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// appleRedeemBody mirrors the StoreKit client's redeem POST: the signed
// transaction JWS from Transaction.jsonRepresentation, base64/JWS-encoded.
type appleRedeemBody struct {
	SignedTransaction string `json:"signedTransaction"`
}

// AppleRedeem: POST /payment/apple/redeem (JWT) → 200 on grant. Verifies the
// client-supplied StoreKit transaction and grants Pro through
// RedeemAppleTransaction (the same Grant chokepoint every other provider
// uses). Errors from verification/plan-resolution/persistence all surface as
// 502 provider-failed — the client's only recourse on any of them is retry,
// same as any other upstream-dependent write.
func (h *Handler) AppleRedeem(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.CodeInternal, "auth context missing")
		return
	}
	var body appleRedeemBody
	if !shared.DecodeJSON(w, r, &body, shared.DecodeStrict) {
		return
	}
	if body.SignedTransaction == "" {
		shared.WriteError(w, r, shared.CodeInvalidInput, "signedTransaction must be a string")
		return
	}
	if err := h.svc.RedeemAppleTransaction(r.Context(), claims.Sub, body.SignedTransaction); err != nil {
		shared.WriteError(w, r, shared.CodeProviderFailed, "apple transaction redeem failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// AppleWebhook: POST /payment/webhook/apple (public) → 201 WebhookResult.
// App Store Server Notifications V2. Apple IAP is redemption-only and
// deliberately absent from registeredMethods/EnabledMethods (see
// webhook.go), so this goes through the UNGATED HandleProviderNotification
// rather than the shared `webhook` helper the other providers use — the JWS
// signature check inside VerifyWebhook is the actual authentication, same
// trust model as every other provider's webhook signature. Unauthenticated
// at transport is correct: Apple does not attach anything RequireAuth could
// verify.
func (h *Handler) AppleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := readWebhookBody(w, r)
	if err != nil {
		return
	}
	res, err := h.svc.HandleProviderNotification(r.Context(), string(MethodAppleIAP), payload, r.Header)
	if err != nil {
		if writePaymentErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, shared.CodeInternal, "webhook failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, res)
}
