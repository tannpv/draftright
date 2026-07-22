// Package appsettings — Postgres adapter for the single-row app_settings
// admin settings table.
//
// Singleton semantics mirror the NestJS AdminController exactly:
//
//   - getSettings: findOne({ where: {} }) → first/any row; if none,
//     create() (entity defaults) + save(). Go: GetAppSettings (ORDER BY
//     updated_at ASC LIMIT 1) + InsertDefaultAppSettings (DEFAULT VALUES,
//     every column has a DB default so the insert needs no params).
//   - updateSettings: finds the singleton, .update(settings.id, body),
//     re-reads by that id. Go: dynamic UPDATE ... SET <non-nil patch
//     fields>, updated_at = now() WHERE id = (the singleton's id),
//     RETURNING the full row.
//
// Two SQL styles, matching the aiprovider adapter:
//   - STATIC reads/inserts go through sqlc (GetAppSettings,
//     InsertDefaultAppSettings) — compile-time checked.
//   - The DYNAMIC partial UPDATE is assembled here (patchSQL) and run on
//     the pool, because the SET column set isn't known until runtime. The
//     column names are literals owned by this file, never user input.
//
// Physical vs declaration order: the app_settings columns are stored in a
// different physical order than the entity declaration (later-added
// columns sit at the end physically). To stay order-independent we (a) map
// the sqlc named-field row struct into AppSettings field-by-field, and (b)
// use an EXPLICIT column list (selectCols) for the dynamic UPDATE's
// RETURNING, scanned in that same explicit order via scanSettings.
package appsettings

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// pool is the subset of *pgxpool.Pool the dynamic UPDATE needs. *pgxpool.Pool
// satisfies it; kept tiny so a test fake (if ever needed) is cheap.
type pool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PgRepo serves the static get-or-create via sqlc and the dynamic patch via
// the pool. One per process; the pgxpool lifecycle is owned by the caller.
type PgRepo struct {
	q    *sqlc.Queries
	pool pool
}

// NewPgRepo wires the sqlc querier + the shared pool.
func NewPgRepo(q *sqlc.Queries, p pool) *PgRepo { return &PgRepo{q: q, pool: p} }

// selectCols is the explicit projection for the dynamic UPDATE's RETURNING,
// in entity declaration order so scanSettings can scan positionally without
// depending on the table's physical column order.
const selectCols = "id, environment, trial_limit, token_expiry_minutes, refresh_token_expiry_days, " +
	"max_input_length, supported_languages, payment_methods_enabled, stripe_secret_key, " +
	"stripe_webhook_secret, stripe_mode, paypal_client_id, paypal_client_secret, paypal_mode, " +
	"paypal_webhook_id, paypal_plan_monthly, paypal_plan_yearly, " +
	"momo_partner_code, momo_access_key, momo_secret_key, momo_mode, vietqr_bank_id, " +
	"vietqr_account_number, vietqr_account_name, casso_api_key, sepay_api_key, sepay_mode, " +
	"resend_api_key, email_from, google_client_id, google_client_secret, apple_client_id, " +
	"apple_team_id, apple_key_id, lemonsqueezy_api_key, lemonsqueezy_store_id, " +
	"lemonsqueezy_webhook_secret, lemonsqueezy_variant_monthly, lemonsqueezy_variant_yearly, " +
	"client_log_level, updated_at"

// GetOrCreate loads the singleton, creating it with DB defaults when absent
// (Node getSettings parity).
func (r *PgRepo) GetOrCreate(ctx context.Context) (AppSettings, error) {
	row, err := r.q.GetAppSettings(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = r.q.InsertDefaultAppSettings(ctx)
	}
	if err != nil {
		return AppSettings{}, err
	}
	return fromRow(row), nil
}

// Patch applies the non-nil patch fields to the singleton via a dynamic
// UPDATE ... SET ... RETURNING, mirroring TypeORM's partial .update() (only
// provided keys written) followed by the re-read. updated_at = now() is
// always set, so an all-nil patch still touches the row (Node parity).
func (r *PgRepo) Patch(ctx context.Context, p Patch) (AppSettings, error) {
	set, args := patchSQL(p)
	sql := fmt.Sprintf(
		"UPDATE app_settings SET %s WHERE id = (SELECT id FROM app_settings ORDER BY updated_at ASC LIMIT 1) RETURNING %s",
		set, selectCols,
	)
	return scanSettings(r.pool.QueryRow(ctx, sql, args...))
}

// patchSQL walks the 36 settable fields in entity declaration order; for
// each non-nil pointer it appends "<col> = $N" and the deref'd value. The
// fixed-order walk (not a map range) keeps placeholder numbering and the SET
// string deterministic. updated_at = now() is always appended last with no
// placeholder.
func patchSQL(p Patch) (set string, args []any) {
	var sets []string
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if p.Environment != nil {
		add("environment", *p.Environment)
	}
	if p.TrialLimit != nil {
		add("trial_limit", *p.TrialLimit)
	}
	if p.TokenExpiryMinutes != nil {
		add("token_expiry_minutes", *p.TokenExpiryMinutes)
	}
	if p.RefreshTokenExpiryDays != nil {
		add("refresh_token_expiry_days", *p.RefreshTokenExpiryDays)
	}
	if p.MaxInputLength != nil {
		add("max_input_length", *p.MaxInputLength)
	}
	if p.SupportedLanguages != nil {
		add("supported_languages", *p.SupportedLanguages)
	}
	if p.PaymentMethodsEnabled != nil {
		add("payment_methods_enabled", *p.PaymentMethodsEnabled)
	}
	if p.StripeSecretKey != nil {
		add("stripe_secret_key", *p.StripeSecretKey)
	}
	if p.StripeWebhookSecret != nil {
		add("stripe_webhook_secret", *p.StripeWebhookSecret)
	}
	if p.StripeMode != nil {
		add("stripe_mode", *p.StripeMode)
	}
	if p.PaypalClientID != nil {
		add("paypal_client_id", *p.PaypalClientID)
	}
	if p.PaypalClientSecret != nil {
		add("paypal_client_secret", *p.PaypalClientSecret)
	}
	if p.PaypalMode != nil {
		add("paypal_mode", *p.PaypalMode)
	}
	if p.PaypalWebhookID != nil {
		add("paypal_webhook_id", *p.PaypalWebhookID)
	}
	if p.PaypalPlanMonthly != nil {
		add("paypal_plan_monthly", *p.PaypalPlanMonthly)
	}
	if p.PaypalPlanYearly != nil {
		add("paypal_plan_yearly", *p.PaypalPlanYearly)
	}
	if p.MomoPartnerCode != nil {
		add("momo_partner_code", *p.MomoPartnerCode)
	}
	if p.MomoAccessKey != nil {
		add("momo_access_key", *p.MomoAccessKey)
	}
	if p.MomoSecretKey != nil {
		add("momo_secret_key", *p.MomoSecretKey)
	}
	if p.MomoMode != nil {
		add("momo_mode", *p.MomoMode)
	}
	if p.VietqrBankID != nil {
		add("vietqr_bank_id", *p.VietqrBankID)
	}
	if p.VietqrAccountNumber != nil {
		add("vietqr_account_number", *p.VietqrAccountNumber)
	}
	if p.VietqrAccountName != nil {
		add("vietqr_account_name", *p.VietqrAccountName)
	}
	if p.CassoAPIKey != nil {
		add("casso_api_key", *p.CassoAPIKey)
	}
	if p.SepayAPIKey != nil {
		add("sepay_api_key", *p.SepayAPIKey)
	}
	if p.SepayMode != nil {
		add("sepay_mode", *p.SepayMode)
	}
	if p.ResendAPIKey != nil {
		add("resend_api_key", *p.ResendAPIKey)
	}
	if p.EmailFrom != nil {
		add("email_from", *p.EmailFrom)
	}
	if p.GoogleClientID != nil {
		add("google_client_id", *p.GoogleClientID)
	}
	if p.GoogleClientSecret != nil {
		add("google_client_secret", *p.GoogleClientSecret)
	}
	if p.AppleClientID != nil {
		add("apple_client_id", *p.AppleClientID)
	}
	if p.AppleTeamID != nil {
		add("apple_team_id", *p.AppleTeamID)
	}
	if p.AppleKeyID != nil {
		add("apple_key_id", *p.AppleKeyID)
	}
	if p.LemonsqueezyAPIKey != nil {
		add("lemonsqueezy_api_key", *p.LemonsqueezyAPIKey)
	}
	if p.LemonsqueezyStoreID != nil {
		add("lemonsqueezy_store_id", *p.LemonsqueezyStoreID)
	}
	if p.LemonsqueezyWebhookSecret != nil {
		add("lemonsqueezy_webhook_secret", *p.LemonsqueezyWebhookSecret)
	}
	if p.LemonsqueezyVariantMonthly != nil {
		add("lemonsqueezy_variant_monthly", *p.LemonsqueezyVariantMonthly)
	}
	if p.LemonsqueezyVariantYearly != nil {
		add("lemonsqueezy_variant_yearly", *p.LemonsqueezyVariantYearly)
	}
	if p.ClientLogLevel != nil {
		add("client_log_level", *p.ClientLogLevel)
	}
	sets = append(sets, "updated_at = now()")
	return strings.Join(sets, ", "), args
}

// fromRow maps a sqlc AppSetting row (named fields, physical-order-agnostic)
// into the domain AppSettings, narrowing int32 → int.
func fromRow(row sqlc.AppSetting) AppSettings {
	return AppSettings{
		ID:                         uuidStr(row.ID),
		Environment:                row.Environment,
		TrialLimit:                 int(row.TrialLimit),
		TokenExpiryMinutes:         int(row.TokenExpiryMinutes),
		RefreshTokenExpiryDays:     int(row.RefreshTokenExpiryDays),
		MaxInputLength:             int(row.MaxInputLength),
		SupportedLanguages:         row.SupportedLanguages,
		PaymentMethodsEnabled:      row.PaymentMethodsEnabled,
		StripeSecretKey:            row.StripeSecretKey,
		StripeWebhookSecret:        row.StripeWebhookSecret,
		StripeMode:                 row.StripeMode,
		PaypalClientID:             row.PaypalClientID,
		PaypalClientSecret:         row.PaypalClientSecret,
		PaypalMode:                 row.PaypalMode,
		PaypalWebhookID:            row.PaypalWebhookID,
		PaypalPlanMonthly:          row.PaypalPlanMonthly,
		PaypalPlanYearly:           row.PaypalPlanYearly,
		MomoPartnerCode:            row.MomoPartnerCode,
		MomoAccessKey:              row.MomoAccessKey,
		MomoSecretKey:              row.MomoSecretKey,
		MomoMode:                   row.MomoMode,
		VietqrBankID:               row.VietqrBankID,
		VietqrAccountNumber:        row.VietqrAccountNumber,
		VietqrAccountName:          row.VietqrAccountName,
		CassoAPIKey:                row.CassoApiKey,
		SepayAPIKey:                row.SepayApiKey,
		SepayMode:                  row.SepayMode,
		ResendAPIKey:               row.ResendApiKey,
		EmailFrom:                  row.EmailFrom,
		GoogleClientID:             row.GoogleClientID,
		GoogleClientSecret:         row.GoogleClientSecret,
		AppleClientID:              row.AppleClientID,
		AppleTeamID:                row.AppleTeamID,
		AppleKeyID:                 row.AppleKeyID,
		LemonsqueezyAPIKey:         row.LemonsqueezyApiKey,
		LemonsqueezyStoreID:        row.LemonsqueezyStoreID,
		LemonsqueezyWebhookSecret:  row.LemonsqueezyWebhookSecret,
		LemonsqueezyVariantMonthly: row.LemonsqueezyVariantMonthly,
		LemonsqueezyVariantYearly:  row.LemonsqueezyVariantYearly,
		ClientLogLevel:             row.ClientLogLevel,
		UpdatedAt:                  tsTime(row.UpdatedAt),
	}
}

// scanSettings scans the dynamic UPDATE's RETURNING row. The Scan arg order
// MUST match selectCols (entity declaration order), NOT physical order.
func scanSettings(row pgx.Row) (AppSettings, error) {
	var (
		id        pgtype.UUID
		updatedAt pgtype.Timestamp
		s         AppSettings
	)
	if err := row.Scan(
		&id,
		&s.Environment,
		&s.TrialLimit,
		&s.TokenExpiryMinutes,
		&s.RefreshTokenExpiryDays,
		&s.MaxInputLength,
		&s.SupportedLanguages,
		&s.PaymentMethodsEnabled,
		&s.StripeSecretKey,
		&s.StripeWebhookSecret,
		&s.StripeMode,
		&s.PaypalClientID,
		&s.PaypalClientSecret,
		&s.PaypalMode,
		&s.PaypalWebhookID,
		&s.PaypalPlanMonthly,
		&s.PaypalPlanYearly,
		&s.MomoPartnerCode,
		&s.MomoAccessKey,
		&s.MomoSecretKey,
		&s.MomoMode,
		&s.VietqrBankID,
		&s.VietqrAccountNumber,
		&s.VietqrAccountName,
		&s.CassoAPIKey,
		&s.SepayAPIKey,
		&s.SepayMode,
		&s.ResendAPIKey,
		&s.EmailFrom,
		&s.GoogleClientID,
		&s.GoogleClientSecret,
		&s.AppleClientID,
		&s.AppleTeamID,
		&s.AppleKeyID,
		&s.LemonsqueezyAPIKey,
		&s.LemonsqueezyStoreID,
		&s.LemonsqueezyWebhookSecret,
		&s.LemonsqueezyVariantMonthly,
		&s.LemonsqueezyVariantYearly,
		&s.ClientLogLevel,
		&updatedAt,
	); err != nil {
		return AppSettings{}, err
	}
	s.ID = uuidStr(id)
	s.UpdatedAt = tsTime(updatedAt)
	return s, nil
}

// tsTime unwraps a pgtype.Timestamp; a NULL yields the zero time. updated_at
// is NOT NULL, so Valid is always true in practice.
func tsTime(t pgtype.Timestamp) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// uuidStr renders a pgtype.UUID as canonical lowercase hyphenated form.
func uuidStr(u pgtype.UUID) string { return u.String() }
