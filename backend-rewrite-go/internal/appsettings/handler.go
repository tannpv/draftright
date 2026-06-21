package appsettings

import (
	"context"
	"errors"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// appSettingsService is the handler's consumer-side port; *Service satisfies
// it. Kept on the consumer side (CLAUDE.md guardrail) so tests inject the real
// Service with faked repo/validator/sender.
type appSettingsService interface {
	Get(ctx context.Context) (AppSettings, error)
	Patch(ctx context.Context, p Patch) (AppSettings, error)
	SendTestEmail(ctx context.Context, to string) error
}

// Handler serves the admin app-settings routes. All routes mount inside the
// admin group (jwtMW → RequireAdmin) wired in the router task; the handler
// itself does no auth.
//
//	GET   /admin/settings             read singleton row → 200 (full 38-key row)
//	PATCH /admin/settings             partial update → 200 (full row)
//	POST  /admin/settings/test-email  send test email → 201 { sent, to }
type Handler struct {
	svc appSettingsService
}

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Get handles GET /admin/settings → 200 with the full settings row.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.Get(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, settings)
}

// patchBody decodes the PATCH body — every field optional (nil = unchanged),
// mirroring Node's `@Body() body: Partial<AppSettings>` (no class-validator
// DTO, so unknown keys are silently ignored — lenient decode, no
// DisallowUnknownFields).
type patchBody struct {
	Environment                *string `json:"environment"`
	TrialLimit                 *int    `json:"trial_limit"`
	TokenExpiryMinutes         *int    `json:"token_expiry_minutes"`
	RefreshTokenExpiryDays     *int    `json:"refresh_token_expiry_days"`
	MaxInputLength             *int    `json:"max_input_length"`
	SupportedLanguages         *string `json:"supported_languages"`
	PaymentMethodsEnabled      *string `json:"payment_methods_enabled"`
	StripeSecretKey            *string `json:"stripe_secret_key"`
	StripeWebhookSecret        *string `json:"stripe_webhook_secret"`
	StripeMode                 *string `json:"stripe_mode"`
	PaypalClientID             *string `json:"paypal_client_id"`
	PaypalClientSecret         *string `json:"paypal_client_secret"`
	PaypalMode                 *string `json:"paypal_mode"`
	MomoPartnerCode            *string `json:"momo_partner_code"`
	MomoAccessKey              *string `json:"momo_access_key"`
	MomoSecretKey              *string `json:"momo_secret_key"`
	MomoMode                   *string `json:"momo_mode"`
	VietqrBankID               *string `json:"vietqr_bank_id"`
	VietqrAccountNumber        *string `json:"vietqr_account_number"`
	VietqrAccountName          *string `json:"vietqr_account_name"`
	CassoAPIKey                *string `json:"casso_api_key"`
	SepayAPIKey                *string `json:"sepay_api_key"`
	SepayMode                  *string `json:"sepay_mode"`
	ResendAPIKey               *string `json:"resend_api_key"`
	EmailFrom                  *string `json:"email_from"`
	GoogleClientID             *string `json:"google_client_id"`
	GoogleClientSecret         *string `json:"google_client_secret"`
	AppleClientID              *string `json:"apple_client_id"`
	AppleTeamID                *string `json:"apple_team_id"`
	AppleKeyID                 *string `json:"apple_key_id"`
	LemonsqueezyAPIKey         *string `json:"lemonsqueezy_api_key"`
	LemonsqueezyStoreID        *string `json:"lemonsqueezy_store_id"`
	LemonsqueezyWebhookSecret  *string `json:"lemonsqueezy_webhook_secret"`
	LemonsqueezyVariantMonthly *string `json:"lemonsqueezy_variant_monthly"`
	LemonsqueezyVariantYearly  *string `json:"lemonsqueezy_variant_yearly"`
	ClientLogLevel             *string `json:"client_log_level"`
}

// Patch handles PATCH /admin/settings → 200 with the re-read full row.
//
// Status mapping mirrors Node updateSettings exactly:
//   - JSON-decode failure on the body                → 400 invalid-input
//   - payment_methods_enabled validation failure     → 400 invalid-input
//     (Node assertMethodsRegisterable throws BadRequestException)
//   - repo / DB failure                              → 500 internal
//     (Node TypeORM throws a plain error → AllExceptionsFilter 500)
//
// svc.Patch wraps a payment-method validation failure in InvalidSettingsError
// (Error() forwards the validator's verbatim message). The handler recognises
// it via errors.As → 400 invalid-input. Any other Patch error is a repo
// failure → 500 internal. Matching on the typed error (not a message prefix)
// means a future wording change in the validator can't silently flip the
// status code.
func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	var body patchBody
	// Empty body (io.EOF) = no fields supplied = all columns untouched, which
	// Node's `Partial<AppSettings>` also accepts. Only malformed JSON is 400.
	if !shared.DecodeJSON(w, r, &body, shared.DecodeOptional) {
		return
	}

	// #30 write guard: drop masked echoes so a portal re-save can't overwrite the
	// stored secret with its mask. Applies to the 11 secret-bearing columns only;
	// an empty string still clears (explicit), nil leaves the column untouched.
	for _, f := range []**string{
		&body.StripeSecretKey, &body.StripeWebhookSecret, &body.PaypalClientSecret,
		&body.MomoAccessKey, &body.MomoSecretKey, &body.CassoAPIKey, &body.SepayAPIKey,
		&body.ResendAPIKey, &body.GoogleClientSecret, &body.LemonsqueezyAPIKey,
		&body.LemonsqueezyWebhookSecret,
	} {
		if *f != nil && shared.ContainsMaskMarker(**f) {
			*f = nil
		}
	}

	p := Patch{
		Environment:                body.Environment,
		TrialLimit:                 body.TrialLimit,
		TokenExpiryMinutes:         body.TokenExpiryMinutes,
		RefreshTokenExpiryDays:     body.RefreshTokenExpiryDays,
		MaxInputLength:             body.MaxInputLength,
		SupportedLanguages:         body.SupportedLanguages,
		PaymentMethodsEnabled:      body.PaymentMethodsEnabled,
		StripeSecretKey:            body.StripeSecretKey,
		StripeWebhookSecret:        body.StripeWebhookSecret,
		StripeMode:                 body.StripeMode,
		PaypalClientID:             body.PaypalClientID,
		PaypalClientSecret:         body.PaypalClientSecret,
		PaypalMode:                 body.PaypalMode,
		MomoPartnerCode:            body.MomoPartnerCode,
		MomoAccessKey:              body.MomoAccessKey,
		MomoSecretKey:              body.MomoSecretKey,
		MomoMode:                   body.MomoMode,
		VietqrBankID:               body.VietqrBankID,
		VietqrAccountNumber:        body.VietqrAccountNumber,
		VietqrAccountName:          body.VietqrAccountName,
		CassoAPIKey:                body.CassoAPIKey,
		SepayAPIKey:                body.SepayAPIKey,
		SepayMode:                  body.SepayMode,
		ResendAPIKey:               body.ResendAPIKey,
		EmailFrom:                  body.EmailFrom,
		GoogleClientID:             body.GoogleClientID,
		GoogleClientSecret:         body.GoogleClientSecret,
		AppleClientID:              body.AppleClientID,
		AppleTeamID:                body.AppleTeamID,
		AppleKeyID:                 body.AppleKeyID,
		LemonsqueezyAPIKey:         body.LemonsqueezyAPIKey,
		LemonsqueezyStoreID:        body.LemonsqueezyStoreID,
		LemonsqueezyWebhookSecret:  body.LemonsqueezyWebhookSecret,
		LemonsqueezyVariantMonthly: body.LemonsqueezyVariantMonthly,
		LemonsqueezyVariantYearly:  body.LemonsqueezyVariantYearly,
		ClientLogLevel:             body.ClientLogLevel,
	}

	settings, err := h.svc.Patch(r.Context(), p)
	if err != nil {
		var inv InvalidSettingsError
		if errors.As(err, &inv) {
			shared.WriteError(w, r, "invalid-input", err.Error())
			return
		}
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, settings)
}

// testEmailBody decodes the test-email POST body { to }.
type testEmailBody struct {
	To string `json:"to"`
}

// TestEmail handles POST /admin/settings/test-email → 201 { sent:true, to }.
//
// Bad recipient: Node throws a PLAIN `new Error('Valid recipient email
// required')` (NOT a NestException), which AllExceptionsFilter classifies as
// 500 / internal. Byte-identical parity therefore returns 500, NOT 400.
func (h *Handler) TestEmail(w http.ResponseWriter, r *http.Request) {
	var body testEmailBody
	// Node's `@Body() body: { to }` tolerates an absent/empty body (Express
	// yields {}), then the `!body?.to` guard throws → 500. Mirror that: an
	// empty body (io.EOF) is NOT a decode error here — it leaves To="" so the
	// service's '@' guard fires. Only malformed JSON is a 400.
	if !shared.DecodeJSON(w, r, &body, shared.DecodeOptional) {
		return
	}
	if err := h.svc.SendTestEmail(r.Context(), body.To); err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusCreated, struct {
		Sent bool   `json:"sent"`
		To   string `json:"to"`
	}{true, body.To})
}
