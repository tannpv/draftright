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
// All secret fields are returned UNMASKED on every read (Node parity — the
// NestJS AppSettings entity exposes raw values). Field/JSON order matches
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
		ID:                         s.ID,
		Environment:                s.Environment,
		TrialLimit:                 s.TrialLimit,
		TokenExpiryMinutes:         s.TokenExpiryMinutes,
		RefreshTokenExpiryDays:     s.RefreshTokenExpiryDays,
		MaxInputLength:             s.MaxInputLength,
		SupportedLanguages:         s.SupportedLanguages,
		PaymentMethodsEnabled:      s.PaymentMethodsEnabled,
		StripeSecretKey:            s.StripeSecretKey,
		StripeWebhookSecret:        s.StripeWebhookSecret,
		StripeMode:                 s.StripeMode,
		PaypalClientID:             s.PaypalClientID,
		PaypalClientSecret:         s.PaypalClientSecret,
		PaypalMode:                 s.PaypalMode,
		MomoPartnerCode:            s.MomoPartnerCode,
		MomoAccessKey:              s.MomoAccessKey,
		MomoSecretKey:              s.MomoSecretKey,
		MomoMode:                   s.MomoMode,
		VietqrBankID:               s.VietqrBankID,
		VietqrAccountNumber:        s.VietqrAccountNumber,
		VietqrAccountName:          s.VietqrAccountName,
		CassoAPIKey:                s.CassoAPIKey,
		SepayAPIKey:                s.SepayAPIKey,
		SepayMode:                  s.SepayMode,
		ResendAPIKey:               s.ResendAPIKey,
		EmailFrom:                  s.EmailFrom,
		GoogleClientID:             s.GoogleClientID,
		GoogleClientSecret:         s.GoogleClientSecret,
		AppleClientID:              s.AppleClientID,
		AppleTeamID:                s.AppleTeamID,
		AppleKeyID:                 s.AppleKeyID,
		LemonsqueezyAPIKey:         s.LemonsqueezyAPIKey,
		LemonsqueezyStoreID:        s.LemonsqueezyStoreID,
		LemonsqueezyWebhookSecret:  s.LemonsqueezyWebhookSecret,
		LemonsqueezyVariantMonthly: s.LemonsqueezyVariantMonthly,
		LemonsqueezyVariantYearly:  s.LemonsqueezyVariantYearly,
		ClientLogLevel:             s.ClientLogLevel,
		UpdatedAt:                  shared.ISOMillis(s.UpdatedAt),
	})
}
