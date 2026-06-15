package updates

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Querier is the sqlc subset the updates repo needs.
type Querier interface {
	GetReleasePolicy(ctx context.Context, platform string) (sqlc.AppReleasePolicy, error)
	GetEnabledReleaseByChannel(ctx context.Context, arg sqlc.GetEnabledReleaseByChannelParams) (sqlc.AppRelease, error)
}

// PgRepo reads effective releases. Ports ReleasesService.getEffective.
type PgRepo struct{ q Querier }

// NewPgRepo wires the querier.
func NewPgRepo(q Querier) *PgRepo { return &PgRepo{q: q} }

// PreferredChannel returns the policy's preferred channel for a platform,
// or "direct" when no policy row exists (Node: policy?.preferred ?? 'direct').
func (r *PgRepo) PreferredChannel(ctx context.Context, platform string) (string, error) {
	pol, err := r.q.GetReleasePolicy(ctx, platform)
	if errors.Is(err, pgx.ErrNoRows) {
		return "direct", nil
	}
	if err != nil {
		return "", err
	}
	if pol.Preferred == "" {
		return "direct", nil
	}
	return pol.Preferred, nil
}

// EnabledRelease returns the enabled release for (platform, channel), or
// (nil, nil) when none.
func (r *PgRepo) EnabledRelease(ctx context.Context, platform, channel string) (*Release, error) {
	row, err := r.q.GetEnabledReleaseByChannel(ctx, sqlc.GetEnabledReleaseByChannelParams{Platform: platform, Channel: channel})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rel := &Release{
		Platform:     row.Platform,
		Version:      row.Version,
		DownloadURL:  row.DownloadUrl,
		SHA256:       row.Sha256,
		ReleaseNotes: row.ReleaseNotes,
		Required:     row.Required,
		Channel:      row.Channel,
	}
	if row.UpdatedAt.Valid {
		rel.UpdatedAt = row.UpdatedAt.Time
	}
	return rel, nil
}
