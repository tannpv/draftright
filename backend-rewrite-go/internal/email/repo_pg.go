package email

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// pgQuerier is the sqlc subset the adapter calls.
type pgQuerier interface {
	IsEmailSuppressed(ctx context.Context, email string) (bool, error)
	InsertEmailLog(ctx context.Context, arg sqlc.InsertEmailLogParams) error
	GetEmailSettings(ctx context.Context) (sqlc.GetEmailSettingsRow, error)
	GetEmailTemplateByKey(ctx context.Context, templateKey string) (sqlc.GetEmailTemplateByKeyRow, error)
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

func (r *PgRepo) GetEmailTemplate(ctx context.Context, key string) (string, string, bool) {
	row, err := r.q.GetEmailTemplateByKey(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return "", "", false
	}
	// subject + html are NOT NULL columns → plain string.
	return row.Subject, row.Html, true
}
