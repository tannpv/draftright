package email

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
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

// PgRepo adapts sqlc to the email.Querier port.
type PgRepo struct{ q pgQuerier }

// NewPgRepo wires a sqlc querier.
func NewPgRepo(q pgQuerier) *PgRepo { return &PgRepo{q: q} }

func (r *PgRepo) IsEmailSuppressed(ctx context.Context, email string) (bool, error) {
	return r.q.IsEmailSuppressed(ctx, email)
}

func (r *PgRepo) InsertEmailLog(ctx context.Context, a InsertEmailLogArgs) error {
	return r.q.InsertEmailLog(ctx, sqlc.InsertEmailLogParams{
		ToEmail: a.To, EmailType: a.Type, Subject: a.Subject, Status: a.Status,
		ProviderID: a.ProviderID, Error: a.Error,
	})
}

func (r *PgRepo) GetEmailSettings(ctx context.Context) (string, string, error) {
	row, err := r.q.GetEmailSettings(ctx)
	if err != nil {
		return "", "", err
	}
	// resend_api_key + email_from are NOT NULL columns → plain string.
	return row.ResendApiKey, row.EmailFrom, nil
}

// MarkByProviderID reflects a Resend delivery event onto email_logs by
// the Resend message id. reason nil → SQL NULL, leaving the existing
// error column untouched (COALESCE in the query).
func (r *PgRepo) MarkByProviderID(ctx context.Context, id, status string, reason *string) error {
	return r.q.MarkEmailByProviderID(ctx, sqlc.MarkEmailByProviderIDParams{
		ProviderID: &id, Status: status, Error: reason,
	})
}

// Suppress adds an address to the suppression list (idempotent). Mirrors
// the NestJS suppress() — the email is lower-cased before insert.
func (r *PgRepo) Suppress(ctx context.Context, email, reason string) error {
	return r.q.SuppressEmail(ctx, sqlc.SuppressEmailParams{
		Email: strings.ToLower(email), Reason: &reason,
	})
}

func (r *PgRepo) GetEmailTemplate(ctx context.Context, key string) (string, string, bool) {
	row, err := r.q.GetEmailTemplateByKey(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return "", "", false
	}
	// subject + html are NOT NULL columns → plain string.
	return row.Subject, row.Html, true
}
