// Package usercontext is the Go read-adapter for per-user rewrite
// personalization (#173). The profile is WRITTEN by the NestJS /me/context
// endpoints; Go only READS it to inject the preamble at rewrite time. It
// satisfies the parity.Service contextProvider seam (a Preamble method), wired
// in cmd/server/main.go via .WithUserContext(...).
package usercontext

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tannpv/draftright-rewrite/internal/platform/secretcipher"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/parity"
	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// contextQuerier is the consumer-side port — just the one sqlc query this
// adapter needs, so a test can fake it without a database.
type contextQuerier interface {
	GetUserContext(ctx context.Context, userID pgtype.UUID) (sqlc.GetUserContextRow, error)
}

// Provider reads user_contexts and builds the injection preamble.
type Provider struct {
	q contextQuerier
}

// NewProvider wraps the sqlc queries (main.go passes sqlc.New(pool)).
func NewProvider(q contextQuerier) *Provider {
	return &Provider{q: q}
}

// Preamble returns the personalization preamble for userID, or "" when there is
// nothing to inject (no row, disabled, empty) OR on ANY error — a lookup/parse/
// decrypt failure must never break the rewrite (mirrors Node
// UserContextService.getPreamble's catch → null). BuildContextPreamble is the
// single shared builder (kept byte-identical to Node), so Go and Node produce
// the same preamble for the same profile.
func (p *Provider) Preamble(ctx context.Context, userID string) string {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ""
	}
	pgUID := pgtype.UUID{Bytes: uid, Valid: true}

	row, err := p.q.GetUserContext(ctx, pgUID)
	if err != nil {
		return "" // pgx.ErrNoRows (never set a context) or any DB error
	}

	// style_notes is encrypted at rest (#50); decrypt for injection. A decrypt
	// failure degrades to no personalization rather than a failed rewrite.
	notes, err := secretcipher.Decrypt(row.StyleNotes)
	if err != nil {
		return ""
	}

	return parity.BuildContextPreamble(parity.UserContextProfile{
		Enabled:    row.Enabled,
		JobTitle:   row.JobTitle,
		Industry:   row.Industry,
		Audience:   row.Audience,
		StyleNotes: notes,
	})
}
