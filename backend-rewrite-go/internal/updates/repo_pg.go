package updates

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Querier is the sqlc subset the updates repo needs.
type Querier interface {
	GetReleasePolicy(ctx context.Context, platform string) (sqlc.AppReleasePolicy, error)
	GetEnabledReleaseByChannel(ctx context.Context, arg sqlc.GetEnabledReleaseByChannelParams) (sqlc.AppRelease, error)
	ListAllReleases(ctx context.Context) ([]sqlc.AppRelease, error)
	ListAllReleasePolicies(ctx context.Context) ([]sqlc.AppReleasePolicy, error)
	GetReleaseChannel(ctx context.Context, arg sqlc.GetReleaseChannelParams) (sqlc.AppRelease, error)
	InsertReleaseChannel(ctx context.Context, arg sqlc.InsertReleaseChannelParams) (sqlc.AppRelease, error)
	UpdateReleaseChannel(ctx context.Context, arg sqlc.UpdateReleaseChannelParams) (sqlc.AppRelease, error)
	DeleteReleaseChannel(ctx context.Context, arg sqlc.DeleteReleaseChannelParams) (int64, error)
	InsertReleasePolicy(ctx context.Context, arg sqlc.InsertReleasePolicyParams) (sqlc.AppReleasePolicy, error)
	UpdateReleasePolicy(ctx context.Context, arg sqlc.UpdateReleasePolicyParams) (sqlc.AppReleasePolicy, error)
}

// mapRelease maps a sqlc app_releases row to the admin AppRelease domain
// struct. updated_at serializes as Date.toISOString() to match Node/TypeORM.
func mapRelease(row sqlc.AppRelease) AppRelease {
	r := AppRelease{
		Platform:     row.Platform,
		Channel:      row.Channel,
		Version:      row.Version,
		DownloadURL:  row.DownloadUrl,
		SHA256:       row.Sha256,
		ReleaseNotes: row.ReleaseNotes,
		Required:     row.Required,
		Enabled:      row.Enabled,
	}
	if row.UpdatedAt.Valid {
		r.UpdatedAt = shared.ISOMillis(row.UpdatedAt.Time)
	}
	return r
}

// mapPolicy maps a sqlc app_release_policies row to the admin
// AppReleasePolicy domain struct.
func mapPolicy(row sqlc.AppReleasePolicy) AppReleasePolicy {
	p := AppReleasePolicy{
		Platform:    row.Platform,
		Preferred:   row.Preferred,
		StoreStatus: row.StoreStatus,
		Notes:       row.Notes,
	}
	if row.UpdatedAt.Valid {
		p.UpdatedAt = shared.ISOMillis(row.UpdatedAt.Time)
	}
	return p
}

// ListAllReleases returns every (platform, channel) row (Node releaseRepo.find()).
func (r *PgRepo) ListAllReleases(ctx context.Context) ([]AppRelease, error) {
	rows, err := r.q.ListAllReleases(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AppRelease, len(rows))
	for i, row := range rows {
		out[i] = mapRelease(row)
	}
	return out, nil
}

// ListAllPolicies returns every policy row (Node policyRepo.find()).
func (r *PgRepo) ListAllPolicies(ctx context.Context) ([]AppReleasePolicy, error) {
	rows, err := r.q.ListAllReleasePolicies(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AppReleasePolicy, len(rows))
	for i, row := range rows {
		out[i] = mapPolicy(row)
	}
	return out, nil
}

// GetReleaseChannel loads one (platform, channel) row, or (nil, nil) when
// none exists (Node releaseRepo.findOne → null). The use case load-then-
// branches on this to mirror Node's partial-overwrite upsert.
func (r *PgRepo) GetReleaseChannel(ctx context.Context, platform, channel string) (*AppRelease, error) {
	row, err := r.q.GetReleaseChannel(ctx, sqlc.GetReleaseChannelParams{Platform: platform, Channel: channel})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rel := mapRelease(row)
	return &rel, nil
}

// InsertRelease creates a new (platform, channel) row with the merged values
// the use case computed.
func (r *PgRepo) InsertRelease(ctx context.Context, in AppRelease) (AppRelease, error) {
	row, err := r.q.InsertReleaseChannel(ctx, sqlc.InsertReleaseChannelParams{
		Platform:     in.Platform,
		Channel:      in.Channel,
		Version:      in.Version,
		DownloadUrl:  in.DownloadURL,
		Sha256:       in.SHA256,
		ReleaseNotes: in.ReleaseNotes,
		Required:     in.Required,
		Enabled:      in.Enabled,
	})
	if err != nil {
		return AppRelease{}, err
	}
	return mapRelease(row), nil
}

// UpdateRelease overwrites an existing (platform, channel) row with the merged
// values the use case computed.
func (r *PgRepo) UpdateRelease(ctx context.Context, in AppRelease) (AppRelease, error) {
	row, err := r.q.UpdateReleaseChannel(ctx, sqlc.UpdateReleaseChannelParams{
		Platform:     in.Platform,
		Channel:      in.Channel,
		Version:      in.Version,
		DownloadUrl:  in.DownloadURL,
		Sha256:       in.SHA256,
		ReleaseNotes: in.ReleaseNotes,
		Required:     in.Required,
		Enabled:      in.Enabled,
	})
	if err != nil {
		return AppRelease{}, err
	}
	return mapRelease(row), nil
}

// DeleteRelease removes a (platform, channel) row, returning the affected
// count (0 → use case raises ErrReleaseNotFound).
func (r *PgRepo) DeleteRelease(ctx context.Context, platform, channel string) (int, error) {
	n, err := r.q.DeleteReleaseChannel(ctx, sqlc.DeleteReleaseChannelParams{Platform: platform, Channel: channel})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// GetPolicy loads one platform's policy row, or (nil, nil) when none exists
// (Node policyRepo.findOne → null). The use case load-then-branches on this.
func (r *PgRepo) GetPolicy(ctx context.Context, platform string) (*AppReleasePolicy, error) {
	row, err := r.q.GetReleasePolicy(ctx, platform)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	pol := mapPolicy(row)
	return &pol, nil
}

// InsertPolicy creates a new policy row with the merged values the use case
// computed.
func (r *PgRepo) InsertPolicy(ctx context.Context, in AppReleasePolicy) (AppReleasePolicy, error) {
	row, err := r.q.InsertReleasePolicy(ctx, sqlc.InsertReleasePolicyParams{
		Platform:    in.Platform,
		Preferred:   in.Preferred,
		StoreStatus: in.StoreStatus,
		Notes:       in.Notes,
	})
	if err != nil {
		return AppReleasePolicy{}, err
	}
	return mapPolicy(row), nil
}

// UpdatePolicy overwrites an existing policy row with the merged values the
// use case computed.
func (r *PgRepo) UpdatePolicy(ctx context.Context, in AppReleasePolicy) (AppReleasePolicy, error) {
	row, err := r.q.UpdateReleasePolicy(ctx, sqlc.UpdateReleasePolicyParams{
		Platform:    in.Platform,
		Preferred:   in.Preferred,
		StoreStatus: in.StoreStatus,
		Notes:       in.Notes,
	})
	if err != nil {
		return AppReleasePolicy{}, err
	}
	return mapPolicy(row), nil
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
