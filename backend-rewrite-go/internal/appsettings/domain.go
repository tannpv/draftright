package appsettings

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// ErrNotFound is returned when the app_settings singleton row does not exist.
var ErrNotFound = errors.New("app settings not found")

// AppSettings mirrors the single-row app_settings admin settings table.
// The 11 payment/SMTP secret columns are MASKED on every read (#30) via
// MarshalJSON; non-secret IDs and config stay raw. Field/JSON order matches
// src/admin/entities/app-settings.entity.ts @Column declaration order exactly.
//
// All string columns are NOT NULL with DB defaults, so they are plain
// strings (not pointers). Four columns are ints (trial_limit,
// token_expiry_minutes, refresh_token_expiry_days, max_input_length).
type AppSettings struct {
	ID                         string
	Environment                string
	TrialLimit                 int
	TokenExpiryMinutes         int
	RefreshTokenExpiryDays     int
	MaxInputLength             int
	SupportedLanguages         string
	PaymentMethodsEnabled      string
	StripeSecretKey            string
	StripeWebhookSecret        string
	StripeMode                 string
	PaypalClientID             string
	PaypalClientSecret         string
	PaypalMode                 string
	PaypalWebhookID            string
	PaypalPlanMonthly          string
	PaypalPlanYearly           string
	MomoPartnerCode            string
	MomoAccessKey              string
	MomoSecretKey              string
	MomoMode                   string
	VietqrBankID               string
	VietqrAccountNumber        string
	VietqrAccountName          string
	CassoAPIKey                string
	SepayAPIKey                string
	SepayMode                  string
	ResendAPIKey               string
	EmailFrom                  string
	GoogleClientID             string
	GoogleClientSecret         string
	AppleClientID              string
	AppleTeamID                string
	AppleKeyID                 string
	LemonsqueezyAPIKey         string
	LemonsqueezyStoreID        string
	LemonsqueezyWebhookSecret  string
	LemonsqueezyVariantMonthly string
	LemonsqueezyVariantYearly  string
	ClientLogLevel             string
	UpdatedAt                  time.Time
}

// Patch is the partial-update input for the admin PATCH settings endpoint.
// Every field is a pointer; nil = "not provided" (column untouched), matching
// TypeORM's partial .update() which only writes supplied keys. No id /
// updated_at (id is fixed; updated_at is set by @UpdateDateColumn).
type Patch struct {
	Environment                *string
	TrialLimit                 *int
	TokenExpiryMinutes         *int
	RefreshTokenExpiryDays     *int
	MaxInputLength             *int
	SupportedLanguages         *string
	PaymentMethodsEnabled      *string
	StripeSecretKey            *string
	StripeWebhookSecret        *string
	StripeMode                 *string
	PaypalClientID             *string
	PaypalClientSecret         *string
	PaypalMode                 *string
	PaypalWebhookID            *string
	PaypalPlanMonthly          *string
	PaypalPlanYearly           *string
	MomoPartnerCode            *string
	MomoAccessKey              *string
	MomoSecretKey              *string
	MomoMode                   *string
	VietqrBankID               *string
	VietqrAccountNumber        *string
	VietqrAccountName          *string
	CassoAPIKey                *string
	SepayAPIKey                *string
	SepayMode                  *string
	ResendAPIKey               *string
	EmailFrom                  *string
	GoogleClientID             *string
	GoogleClientSecret         *string
	AppleClientID              *string
	AppleTeamID                *string
	AppleKeyID                 *string
	LemonsqueezyAPIKey         *string
	LemonsqueezyStoreID        *string
	LemonsqueezyWebhookSecret  *string
	LemonsqueezyVariantMonthly *string
	LemonsqueezyVariantYearly  *string
	ClientLogLevel             *string
}

func (s AppSettings) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID                         string `json:"id"`
		Environment                string `json:"environment"`
		TrialLimit                 int    `json:"trial_limit"`
		TokenExpiryMinutes         int    `json:"token_expiry_minutes"`
		RefreshTokenExpiryDays     int    `json:"refresh_token_expiry_days"`
		MaxInputLength             int    `json:"max_input_length"`
		SupportedLanguages         string `json:"supported_languages"`
		PaymentMethodsEnabled      string `json:"payment_methods_enabled"`
		StripeSecretKey            string `json:"stripe_secret_key"`
		StripeWebhookSecret        string `json:"stripe_webhook_secret"`
		StripeMode                 string `json:"stripe_mode"`
		PaypalClientID             string `json:"paypal_client_id"`
		PaypalClientSecret         string `json:"paypal_client_secret"`
		PaypalMode                 string `json:"paypal_mode"`
		PaypalWebhookID            string `json:"paypal_webhook_id"`
		PaypalPlanMonthly          string `json:"paypal_plan_monthly"`
		PaypalPlanYearly           string `json:"paypal_plan_yearly"`
		MomoPartnerCode            string `json:"momo_partner_code"`
		MomoAccessKey              string `json:"momo_access_key"`
		MomoSecretKey              string `json:"momo_secret_key"`
		MomoMode                   string `json:"momo_mode"`
		VietqrBankID               string `json:"vietqr_bank_id"`
		VietqrAccountNumber        string `json:"vietqr_account_number"`
		VietqrAccountName          string `json:"vietqr_account_name"`
		CassoAPIKey                string `json:"casso_api_key"`
		SepayAPIKey                string `json:"sepay_api_key"`
		SepayMode                  string `json:"sepay_mode"`
		ResendAPIKey               string `json:"resend_api_key"`
		EmailFrom                  string `json:"email_from"`
		GoogleClientID             string `json:"google_client_id"`
		GoogleClientSecret         string `json:"google_client_secret"`
		AppleClientID              string `json:"apple_client_id"`
		AppleTeamID                string `json:"apple_team_id"`
		AppleKeyID                 string `json:"apple_key_id"`
		LemonsqueezyAPIKey         string `json:"lemonsqueezy_api_key"`
		LemonsqueezyStoreID        string `json:"lemonsqueezy_store_id"`
		LemonsqueezyWebhookSecret  string `json:"lemonsqueezy_webhook_secret"`
		LemonsqueezyVariantMonthly string `json:"lemonsqueezy_variant_monthly"`
		LemonsqueezyVariantYearly  string `json:"lemonsqueezy_variant_yearly"`
		ClientLogLevel             string `json:"client_log_level"`
		UpdatedAt                  string `json:"updated_at"`
	}{
		ID:                     s.ID,
		Environment:            s.Environment,
		TrialLimit:             s.TrialLimit,
		TokenExpiryMinutes:     s.TokenExpiryMinutes,
		RefreshTokenExpiryDays: s.RefreshTokenExpiryDays,
		MaxInputLength:         s.MaxInputLength,
		SupportedLanguages:     s.SupportedLanguages,
		PaymentMethodsEnabled:  s.PaymentMethodsEnabled,
		// #30: payment + SMTP secret columns masked (first3…last4). Public IDs
		// (paypal_client_id, apple_team_id/key_id, google_client_id, store/variant
		// IDs, *_mode, vietqr_*, email_from, *_partner_code) stay raw. Node mirror:
		// src/admin maskSettings util.
		StripeSecretKey:            shared.MaskSecret(s.StripeSecretKey),
		StripeWebhookSecret:        shared.MaskSecret(s.StripeWebhookSecret),
		StripeMode:                 s.StripeMode,
		PaypalClientID:             s.PaypalClientID,
		PaypalClientSecret:         shared.MaskSecret(s.PaypalClientSecret),
		PaypalMode:                 s.PaypalMode,
		PaypalWebhookID:            s.PaypalWebhookID,
		PaypalPlanMonthly:          s.PaypalPlanMonthly,
		PaypalPlanYearly:           s.PaypalPlanYearly,
		MomoPartnerCode:            s.MomoPartnerCode,
		MomoAccessKey:              shared.MaskSecret(s.MomoAccessKey),
		MomoSecretKey:              shared.MaskSecret(s.MomoSecretKey),
		MomoMode:                   s.MomoMode,
		VietqrBankID:               s.VietqrBankID,
		VietqrAccountNumber:        s.VietqrAccountNumber,
		VietqrAccountName:          s.VietqrAccountName,
		CassoAPIKey:                shared.MaskSecret(s.CassoAPIKey),
		SepayAPIKey:                shared.MaskSecret(s.SepayAPIKey),
		SepayMode:                  s.SepayMode,
		ResendAPIKey:               shared.MaskSecret(s.ResendAPIKey),
		EmailFrom:                  s.EmailFrom,
		GoogleClientID:             s.GoogleClientID,
		GoogleClientSecret:         shared.MaskSecret(s.GoogleClientSecret),
		AppleClientID:              s.AppleClientID,
		AppleTeamID:                s.AppleTeamID,
		AppleKeyID:                 s.AppleKeyID,
		LemonsqueezyAPIKey:         shared.MaskSecret(s.LemonsqueezyAPIKey),
		LemonsqueezyStoreID:        s.LemonsqueezyStoreID,
		LemonsqueezyWebhookSecret:  shared.MaskSecret(s.LemonsqueezyWebhookSecret),
		LemonsqueezyVariantMonthly: s.LemonsqueezyVariantMonthly,
		LemonsqueezyVariantYearly:  s.LemonsqueezyVariantYearly,
		ClientLogLevel:             s.ClientLogLevel,
		UpdatedAt:                  shared.ISOMillis(s.UpdatedAt),
	})
}
