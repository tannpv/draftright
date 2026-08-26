package email

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/tannpv/draftright-rewrite/internal/platform/secretcipher"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	"github.com/tannpv/emailkit"
)

// pgQuerier is the sqlc subset the adapter calls.
type pgQuerier interface {
	IsEmailSuppressed(ctx context.Context, email string) (bool, error)
	InsertEmailLog(ctx context.Context, arg sqlc.InsertEmailLogParams) error
	GetEmailSettings(ctx context.Context) (sqlc.GetEmailSettingsRow, error)
	GetEmailTemplateByKey(ctx context.Context, templateKey string) (sqlc.GetEmailTemplateByKeyRow, error)
	MarkEmailByProviderID(ctx context.Context, arg sqlc.MarkEmailByProviderIDParams) error
	SuppressEmail(ctx context.Context, arg sqlc.SuppressEmailParams) error
}

// PgRepo adapts sqlc to emailkit.Store.
type PgRepo struct{ q pgQuerier }

// NewPgRepo wires a sqlc querier.
func NewPgRepo(q pgQuerier) *PgRepo { return &PgRepo{q: q} }

func (r *PgRepo) IsSuppressed(ctx context.Context, email string) (bool, error) {
	return r.q.IsEmailSuppressed(ctx, email)
}

func (r *PgRepo) LogSend(ctx context.Context, a emailkit.SendRecord) error {
	return r.q.InsertEmailLog(ctx, sqlc.InsertEmailLogParams{
		ToEmail: a.To, EmailType: a.Type, Subject: a.Subject, Status: a.Status,
		ProviderID: a.ProviderID, Error: a.Error,
	})
}

// Credentials backs emailkit's Config.Resolve. The key is decrypted at rest
// (#50); Decrypt passes legacy plaintext through unchanged.
//
// resend_api_key + email_from are NOT NULL columns → plain string.
func (r *PgRepo) Credentials(ctx context.Context) (string, string, error) {
	row, err := r.q.GetEmailSettings(ctx)
	if err != nil {
		// No app_settings row means "not configured yet", not "resolution
		// failed": emailkit's Config.Resolve contract skips the send (no env
		// fallback) only on a genuine failure — a dead DB, a bad decrypt.
		// Treating an absent row the same way stopped every email on a fresh
		// deployment, since app_settings starts empty. Every other error
		// (including a row that exists but fails to decrypt) still fails
		// closed below. Same shape as Template's identical check.
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", nil
		}
		return "", "", err
	}
	apiKey, err := secretcipher.Decrypt(row.ResendApiKey)
	if err != nil {
		return "", "", err
	}
	return apiKey, row.EmailFrom, nil
}

// MarkByProviderID reflects a Resend delivery event onto email_logs by
// the Resend message id. reason nil → SQL NULL, leaving the existing
// error column untouched (COALESCE in the query).
func (r *PgRepo) MarkByProviderID(ctx context.Context, id, status string, reason *string) error {
	return r.q.MarkEmailByProviderID(ctx, sqlc.MarkEmailByProviderIDParams{
		ProviderID: &id, Status: status, Error: reason,
	})
}

// Suppress adds an address to the suppression list. The SQL is
// ON CONFLICT DO NOTHING, which is what lets emailkit answer a failed store
// write with a retryable error and have the redelivery converge.
//
// The address arrives already lowercased and trimmed by emailkit. Re-casing it
// here would be a second definition of that policy, and the two would drift.
func (r *PgRepo) Suppress(ctx context.Context, email, reason string) error {
	return r.q.SuppressEmail(ctx, sqlc.SuppressEmailParams{
		Email: email, Reason: &reason,
	})
}

func (r *PgRepo) Template(ctx context.Context, key string) (string, string, bool) {
	row, err := r.q.GetEmailTemplateByKey(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return "", "", false
	}
	// subject + html are NOT NULL columns → plain string.
	return row.Subject, row.Html, true
}
