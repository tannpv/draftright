// Admin email-templates Postgres adapter. Static read via the sqlc-generated
// *sqlc.Queries (ListEmailTemplates → all email_templates rows). The use case
// merges these customizations onto the builtin defaults; this repo only loads
// them, keyed by template_key (mirroring Node's `new Map(o.template_key → o)`).
package email

import (
	"context"

	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// AdminTemplatesRepo wraps the sqlc querier. The *sqlc.Queries is owned by the
// caller (wired in main.go in a later task).
type AdminTemplatesRepo struct {
	q *sqlc.Queries
}

// NewAdminTemplatesRepo wires the sqlc querier.
func NewAdminTemplatesRepo(q *sqlc.Queries) *AdminTemplatesRepo {
	return &AdminTemplatesRepo{q: q}
}

// ListCustomizations loads all email_templates rows into a map keyed by
// template_key. The map is non-nil even when empty (so the merge always
// ranges cleanly).
func (r *AdminTemplatesRepo) ListCustomizations(ctx context.Context) (map[string]DBTemplate, error) {
	rows, err := r.q.ListEmailTemplates(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]DBTemplate, len(rows))
	for _, row := range rows {
		out[row.TemplateKey] = DBTemplate{Subject: row.Subject, HTML: row.Html}
	}
	return out, nil
}

// Upsert inserts or updates the customization row for template_key (PK), mirroring
// Node's emailTemplateRepo.save(create({template_key, subject, html})).
func (r *AdminTemplatesRepo) Upsert(ctx context.Context, key, subject, html string) error {
	return r.q.UpsertEmailTemplate(ctx, sqlc.UpsertEmailTemplateParams{
		TemplateKey: key,
		Subject:     subject,
		Html:        html,
	})
}

// Delete removes the customization row for template_key (reset to builtin),
// mirroring Node's emailTemplateRepo.delete({template_key}). Idempotent.
func (r *AdminTemplatesRepo) Delete(ctx context.Context, key string) error {
	return r.q.DeleteEmailTemplate(ctx, key)
}
