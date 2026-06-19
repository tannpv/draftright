package updates

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// adminFakeQuerier extends the read-path querier with the admin write/list
// methods E1 adds. It records params and returns canned rows so the mappers +
// nil-handling are exercised without a DB (the live shadow gate covers the SQL).
type adminFakeQuerier struct {
	allReleases  []sqlc.AppRelease
	allPolicies  []sqlc.AppReleasePolicy
	getRelRow    sqlc.AppRelease
	getRelErr    error
	getRelParams sqlc.GetReleaseChannelParams
	insRelParams sqlc.InsertReleaseChannelParams
	updRelParams sqlc.UpdateReleaseChannelParams
	delRelParams sqlc.DeleteReleaseChannelParams
	deleted      int64
	getPolRow    sqlc.AppReleasePolicy
	getPolErr    error
	insPolParams sqlc.InsertReleasePolicyParams
	updPolParams sqlc.UpdateReleasePolicyParams
}

func (f *adminFakeQuerier) GetReleasePolicy(ctx context.Context, platform string) (sqlc.AppReleasePolicy, error) {
	return f.getPolRow, f.getPolErr
}
func (f *adminFakeQuerier) GetEnabledReleaseByChannel(ctx context.Context, arg sqlc.GetEnabledReleaseByChannelParams) (sqlc.AppRelease, error) {
	return sqlc.AppRelease{}, nil
}
func (f *adminFakeQuerier) ListAllReleases(ctx context.Context) ([]sqlc.AppRelease, error) {
	return f.allReleases, nil
}
func (f *adminFakeQuerier) ListAllReleasePolicies(ctx context.Context) ([]sqlc.AppReleasePolicy, error) {
	return f.allPolicies, nil
}
func (f *adminFakeQuerier) GetReleaseChannel(ctx context.Context, arg sqlc.GetReleaseChannelParams) (sqlc.AppRelease, error) {
	f.getRelParams = arg
	return f.getRelRow, f.getRelErr
}
func (f *adminFakeQuerier) InsertReleaseChannel(ctx context.Context, arg sqlc.InsertReleaseChannelParams) (sqlc.AppRelease, error) {
	f.insRelParams = arg
	return sqlc.AppRelease{
		Platform: arg.Platform, Version: arg.Version, DownloadUrl: arg.DownloadUrl,
		Sha256: arg.Sha256, ReleaseNotes: arg.ReleaseNotes, Required: arg.Required,
		Channel: arg.Channel, Enabled: arg.Enabled,
	}, nil
}
func (f *adminFakeQuerier) UpdateReleaseChannel(ctx context.Context, arg sqlc.UpdateReleaseChannelParams) (sqlc.AppRelease, error) {
	f.updRelParams = arg
	return sqlc.AppRelease{
		Platform: arg.Platform, Version: arg.Version, DownloadUrl: arg.DownloadUrl,
		Sha256: arg.Sha256, ReleaseNotes: arg.ReleaseNotes, Required: arg.Required,
		Channel: arg.Channel, Enabled: arg.Enabled,
	}, nil
}
func (f *adminFakeQuerier) DeleteReleaseChannel(ctx context.Context, arg sqlc.DeleteReleaseChannelParams) (int64, error) {
	f.delRelParams = arg
	return f.deleted, nil
}
func (f *adminFakeQuerier) InsertReleasePolicy(ctx context.Context, arg sqlc.InsertReleasePolicyParams) (sqlc.AppReleasePolicy, error) {
	f.insPolParams = arg
	return sqlc.AppReleasePolicy{
		Platform: arg.Platform, Preferred: arg.Preferred, StoreStatus: arg.StoreStatus, Notes: arg.Notes,
	}, nil
}
func (f *adminFakeQuerier) UpdateReleasePolicy(ctx context.Context, arg sqlc.UpdateReleasePolicyParams) (sqlc.AppReleasePolicy, error) {
	f.updPolParams = arg
	return sqlc.AppReleasePolicy{
		Platform: arg.Platform, Preferred: arg.Preferred, StoreStatus: arg.StoreStatus, Notes: arg.Notes,
	}, nil
}

func tz(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func TestListAllReleases_Mapper(t *testing.T) {
	ts := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	f := &adminFakeQuerier{allReleases: []sqlc.AppRelease{
		{Platform: "mac", Channel: "direct", Version: "1.2.3", DownloadUrl: "u", Sha256: "ab", ReleaseNotes: "n", Required: true, Enabled: true, UpdatedAt: tz(ts)},
	}}
	repo := NewPgRepo(f)
	rows, err := repo.ListAllReleases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	r := rows[0]
	if r.Platform != "mac" || r.Channel != "direct" || r.Version != "1.2.3" || r.DownloadURL != "u" ||
		r.SHA256 != "ab" || r.ReleaseNotes != "n" || !r.Required || !r.Enabled {
		t.Fatalf("release mapper mismatch: %+v", r)
	}
	if r.UpdatedAt != "2026-06-19T12:00:00.000Z" {
		t.Fatalf("updated_at = %q", r.UpdatedAt)
	}
}

func TestListAllPolicies_Mapper(t *testing.T) {
	ts := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	f := &adminFakeQuerier{allPolicies: []sqlc.AppReleasePolicy{
		{Platform: "ios", Preferred: "store", StoreStatus: "approved", Notes: "ok", UpdatedAt: tz(ts)},
	}}
	repo := NewPgRepo(f)
	rows, err := repo.ListAllPolicies(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	p := rows[0]
	if p.Platform != "ios" || p.Preferred != "store" || p.StoreStatus != "approved" || p.Notes != "ok" {
		t.Fatalf("policy mapper mismatch: %+v", p)
	}
	if p.UpdatedAt != "2026-06-19T12:00:00.000Z" {
		t.Fatalf("updated_at = %q", p.UpdatedAt)
	}
}

func TestGetReleaseChannel_FoundAndNotFound(t *testing.T) {
	f := &adminFakeQuerier{getRelRow: sqlc.AppRelease{Platform: "mac", Channel: "store", Version: "9"}}
	repo := NewPgRepo(f)
	got, err := repo.GetReleaseChannel(context.Background(), "mac", "store")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Version != "9" {
		t.Fatalf("expected found release, got %+v", got)
	}
	if f.getRelParams.Platform != "mac" || f.getRelParams.Channel != "store" {
		t.Fatalf("params = %+v", f.getRelParams)
	}

	f2 := &adminFakeQuerier{getRelErr: pgx.ErrNoRows}
	got2, err := NewPgRepo(f2).GetReleaseChannel(context.Background(), "mac", "store")
	if err != nil {
		t.Fatalf("expected nil error on no rows, got %v", err)
	}
	if got2 != nil {
		t.Fatalf("expected nil release on no rows, got %+v", got2)
	}
}

func TestInsertAndUpdateRelease_Passthrough(t *testing.T) {
	f := &adminFakeQuerier{}
	repo := NewPgRepo(f)
	in := AppRelease{Platform: "linux", Channel: "direct", Version: "2", DownloadURL: "d", SHA256: "h", ReleaseNotes: "rn", Required: true, Enabled: false}
	out, err := repo.InsertRelease(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Platform != "linux" || out.Channel != "direct" || out.Version != "2" || !out.Required || out.Enabled {
		t.Fatalf("insert out mismatch: %+v", out)
	}
	if f.insRelParams.DownloadUrl != "d" || f.insRelParams.Sha256 != "h" || f.insRelParams.ReleaseNotes != "rn" {
		t.Fatalf("insert params mismatch: %+v", f.insRelParams)
	}
	if _, err := repo.UpdateRelease(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if f.updRelParams.Platform != "linux" || f.updRelParams.Channel != "direct" || f.updRelParams.Version != "2" {
		t.Fatalf("update params mismatch: %+v", f.updRelParams)
	}
}

func TestDeleteRelease_Affected(t *testing.T) {
	f := &adminFakeQuerier{deleted: 1}
	n, err := NewPgRepo(f).DeleteRelease(context.Background(), "windows", "store")
	if err != nil || n != 1 {
		t.Fatalf("expected affected=1, got %d %v", n, err)
	}
	if f.delRelParams.Platform != "windows" || f.delRelParams.Channel != "store" {
		t.Fatalf("delete params = %+v", f.delRelParams)
	}
	n2, _ := NewPgRepo(&adminFakeQuerier{deleted: 0}).DeleteRelease(context.Background(), "windows", "store")
	if n2 != 0 {
		t.Fatalf("expected affected=0, got %d", n2)
	}
}

func TestGetPolicy_FoundAndNotFound(t *testing.T) {
	ts := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	f := &adminFakeQuerier{getPolRow: sqlc.AppReleasePolicy{Platform: "android", Preferred: "direct", StoreStatus: "in_review", Notes: "x", UpdatedAt: tz(ts)}}
	got, err := NewPgRepo(f).GetPolicy(context.Background(), "android")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Platform != "android" || got.StoreStatus != "in_review" || got.UpdatedAt != "2026-06-19T12:00:00.000Z" {
		t.Fatalf("policy mismatch: %+v", got)
	}
	f2 := &adminFakeQuerier{getPolErr: pgx.ErrNoRows}
	got2, err := NewPgRepo(f2).GetPolicy(context.Background(), "android")
	if err != nil {
		t.Fatalf("expected nil error on no rows, got %v", err)
	}
	if got2 != nil {
		t.Fatalf("expected nil policy on no rows, got %+v", got2)
	}
}

func TestInsertAndUpdatePolicy_Passthrough(t *testing.T) {
	f := &adminFakeQuerier{}
	repo := NewPgRepo(f)
	in := AppReleasePolicy{Platform: "mac", Preferred: "store", StoreStatus: "approved", Notes: "n"}
	out, err := repo.InsertPolicy(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Preferred != "store" || out.StoreStatus != "approved" || out.Notes != "n" {
		t.Fatalf("insert policy out mismatch: %+v", out)
	}
	if f.insPolParams.Platform != "mac" || f.insPolParams.Preferred != "store" {
		t.Fatalf("insert policy params mismatch: %+v", f.insPolParams)
	}
	if _, err := repo.UpdatePolicy(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if f.updPolParams.Platform != "mac" || f.updPolParams.StoreStatus != "approved" {
		t.Fatalf("update policy params mismatch: %+v", f.updPolParams)
	}
}
