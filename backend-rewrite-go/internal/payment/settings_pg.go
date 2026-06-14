package payment

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// coreSettingsQuerier is the one-method sqlc subset for the settings read.
// Satisfied by the live *sqlc.Queries.
type coreSettingsQuerier interface {
	GetPaymentMethodsEnabled(ctx context.Context) (string, error)
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
