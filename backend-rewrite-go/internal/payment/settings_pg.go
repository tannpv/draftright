package payment

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// coreSettingsQuerier is the sqlc subset for the settings reads.
// Satisfied by the live *sqlc.Queries.
type coreSettingsQuerier interface {
	GetPaymentMethodsEnabled(ctx context.Context) (string, error)
	GetPaymentCredentials(ctx context.Context) (sqlc.GetPaymentCredentialsRow, error)
}

// SettingsAdapter maps app_settings.payment_methods_enabled to the Service's
// SettingsReader port. The column is NOT NULL (default ”); an empty string
// means unconfigured → found=false so the Service degrades to env → default.
// No app_settings row at all → pgx.ErrNoRows → found=false.
type SettingsAdapter struct{ q coreSettingsQuerier }

// NewSettingsAdapter wires the querier.
func NewSettingsAdapter(q coreSettingsQuerier) *SettingsAdapter { return &SettingsAdapter{q: q} }

// PaymentMethodsEnabled returns (csv, found, err). found is false on an empty
// column or an absent row, so the Service falls back to env then default.
func (a *SettingsAdapter) PaymentMethodsEnabled(ctx context.Context) (string, bool, error) {
	v, err := a.q.GetPaymentMethodsEnabled(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, v != "", nil
}

// Credentials is the checkout-time provider credential set (DB priority side
// of resolveCredential). Empty strings when the column / row is absent.
type Credentials struct {
	StripeSecretKey            string
	StripeWebhookSecret        string
	VietQRBankID               string
	VietQRAccountNumber        string
	VietQRAccountName          string
	CassoAPIKey                string
	SepayAPIKey                string
	LemonSqueezyAPIKey         string
	LemonSqueezyStoreID        string
	LemonSqueezyVariantMonthly string
	LemonSqueezyVariantYearly  string
	LemonSqueezyWebhookSecret  string
}

// Credentials reads the singleton app_settings credential row. A missing row
// is not an error — it yields the zero value, and the caller falls back to env
// per resolveCredential (Node getSettings() returns null the same way).
func (a *SettingsAdapter) Credentials(ctx context.Context) (Credentials, error) {
	row, err := a.q.GetPaymentCredentials(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return Credentials{}, nil
	}
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{
		StripeSecretKey:            row.StripeSecretKey,
		StripeWebhookSecret:        row.StripeWebhookSecret,
		VietQRBankID:               row.VietqrBankID,
		VietQRAccountNumber:        row.VietqrAccountNumber,
		VietQRAccountName:          row.VietqrAccountName,
		CassoAPIKey:                row.CassoApiKey,
		SepayAPIKey:                row.SepayApiKey,
		LemonSqueezyAPIKey:         row.LemonsqueezyApiKey,
		LemonSqueezyStoreID:        row.LemonsqueezyStoreID,
		LemonSqueezyVariantMonthly: row.LemonsqueezyVariantMonthly,
		LemonSqueezyVariantYearly:  row.LemonsqueezyVariantYearly,
		LemonSqueezyWebhookSecret:  row.LemonsqueezyWebhookSecret,
	}, nil
}
