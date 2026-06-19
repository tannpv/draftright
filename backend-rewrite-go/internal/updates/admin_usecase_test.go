package updates_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tannpv/draftright-rewrite/internal/updates"
)

// fakeRelRepo is an in-memory stand-in for the release/policy repo port.
type fakeRelRepo struct {
	releases []updates.AppRelease
	policies []updates.AppReleasePolicy

	getRelease *updates.AppRelease
	getPolicy  *updates.AppReleasePolicy

	insertedRel updates.AppRelease
	updatedRel  updates.AppRelease
	insertedPol updates.AppReleasePolicy
	updatedPol  updates.AppReleasePolicy

	deleteAffected int
	deletedPlat    string
	deletedChan    string
}

func (f *fakeRelRepo) ListAllReleases(ctx context.Context) ([]updates.AppRelease, error) {
	return f.releases, nil
}
func (f *fakeRelRepo) ListAllPolicies(ctx context.Context) ([]updates.AppReleasePolicy, error) {
	return f.policies, nil
}
func (f *fakeRelRepo) GetReleaseChannel(ctx context.Context, platform, channel string) (*updates.AppRelease, error) {
	return f.getRelease, nil
}
func (f *fakeRelRepo) InsertRelease(ctx context.Context, in updates.AppRelease) (updates.AppRelease, error) {
	f.insertedRel = in
	return in, nil
}
func (f *fakeRelRepo) UpdateRelease(ctx context.Context, in updates.AppRelease) (updates.AppRelease, error) {
	f.updatedRel = in
	return in, nil
}
func (f *fakeRelRepo) DeleteRelease(ctx context.Context, platform, channel string) (int, error) {
	f.deletedPlat, f.deletedChan = platform, channel
	return f.deleteAffected, nil
}
func (f *fakeRelRepo) GetPolicy(ctx context.Context, platform string) (*updates.AppReleasePolicy, error) {
	return f.getPolicy, nil
}
func (f *fakeRelRepo) InsertPolicy(ctx context.Context, in updates.AppReleasePolicy) (updates.AppReleasePolicy, error) {
	f.insertedPol = in
	return in, nil
}
func (f *fakeRelRepo) UpdatePolicy(ctx context.Context, in updates.AppReleasePolicy) (updates.AppReleasePolicy, error) {
	f.updatedPol = in
	return in, nil
}

func TestListAll_SeedsAllPlatformsInOrder(t *testing.T) {
	repo := &fakeRelRepo{
		releases: []updates.AppRelease{
			{Platform: "mac", Channel: "direct", Version: "1.0"},
			{Platform: "windows", Channel: "store", Version: "2.0"},
			{Platform: "bogus", Channel: "direct", Version: "9.0"}, // unknown → skipped
		},
		policies: []updates.AppReleasePolicy{
			{Platform: "mac", Preferred: "direct"},
			{Platform: "nope", Preferred: "store"}, // unknown → skipped
		},
	}
	svc := updates.NewAdminService(repo)
	view, err := svc.ListAll(context.Background())
	require.NoError(t, err)

	// mac filled.
	require.NotNil(t, view.Mac.Policy)
	require.Equal(t, "mac", view.Mac.Policy.Platform)
	require.NotNil(t, view.Mac.Channels.Direct)
	require.Equal(t, "1.0", view.Mac.Channels.Direct.Version)
	require.Nil(t, view.Mac.Channels.Store)

	// windows store filled, policy nil.
	require.Nil(t, view.Windows.Policy)
	require.NotNil(t, view.Windows.Channels.Store)
	require.Equal(t, "2.0", view.Windows.Channels.Store.Version)
	require.Nil(t, view.Windows.Channels.Direct)

	// unfilled platforms remain nil.
	require.Nil(t, view.Linux.Policy)
	require.Nil(t, view.Linux.Channels.Direct)
	require.Nil(t, view.Linux.Channels.Store)
	require.Nil(t, view.Android.Policy)
	require.Nil(t, view.Ios.Policy)
}

func TestUpsertChannel_BadPlatform(t *testing.T) {
	svc := updates.NewAdminService(&fakeRelRepo{})
	_, err := svc.UpsertChannel(context.Background(), updates.UpsertChannelInput{Platform: "nope", Channel: "direct", Version: "1", DownloadURL: "u"})
	require.EqualError(t, err, "platform must be one of: mac, windows, linux, android, ios")
}

func TestUpsertChannel_BadChannel(t *testing.T) {
	svc := updates.NewAdminService(&fakeRelRepo{})
	_, err := svc.UpsertChannel(context.Background(), updates.UpsertChannelInput{Platform: "mac", Channel: "beta", Version: "1", DownloadURL: "u"})
	require.EqualError(t, err, "channel must be one of: direct, store")
}

func TestUpsertChannel_MissingVersionOrURL(t *testing.T) {
	svc := updates.NewAdminService(&fakeRelRepo{})
	_, err := svc.UpsertChannel(context.Background(), updates.UpsertChannelInput{Platform: "mac", Channel: "direct", Version: "", DownloadURL: "u"})
	require.EqualError(t, err, "version and download_url are required")
	_, err = svc.UpsertChannel(context.Background(), updates.UpsertChannelInput{Platform: "mac", Channel: "direct", Version: "1", DownloadURL: ""})
	require.EqualError(t, err, "version and download_url are required")
}

func TestUpsertChannel_BadSha(t *testing.T) {
	svc := updates.NewAdminService(&fakeRelRepo{})
	sha := "xyz"
	_, err := svc.UpsertChannel(context.Background(), updates.UpsertChannelInput{Platform: "mac", Channel: "direct", Version: "1", DownloadURL: "u", Sha256: &sha})
	require.EqualError(t, err, "sha256 must be a 64-char hex string (or empty)")
}

func TestUpsertChannel_ShaLowercasedAndDefaults(t *testing.T) {
	repo := &fakeRelRepo{} // no existing → insert
	svc := updates.NewAdminService(repo)
	sha := "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789"
	out, err := svc.UpsertChannel(context.Background(), updates.UpsertChannelInput{Platform: "mac", Channel: "direct", Version: "1", DownloadURL: "u", Sha256: &sha})
	require.NoError(t, err)
	require.Equal(t, "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", out.SHA256)
	// defaults
	require.Equal(t, "", out.ReleaseNotes)
	require.Equal(t, false, out.Required)
	require.Equal(t, true, out.Enabled)
	require.Equal(t, "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", repo.insertedRel.SHA256)
}

// An explicit empty channel must FAIL validation at the usecase layer. Node
// defaults only on null/undefined (`?? 'direct'`, applied in the handler when
// the request key is absent); an explicit "" is not one of {direct, store} so
// the service rejects it with the verbatim message → 400. The legitimate
// nil→"direct" default lives in the handler, not the usecase.
func TestUpsertChannel_EmptyChannelRejected(t *testing.T) {
	repo := &fakeRelRepo{}
	svc := updates.NewAdminService(repo)
	_, err := svc.UpsertChannel(context.Background(), updates.UpsertChannelInput{Platform: "mac", Version: "1", DownloadURL: "u"})
	require.EqualError(t, err, "channel must be one of: direct, store")
}

func TestUpsertChannel_EmptyShaDefaultsEmpty(t *testing.T) {
	repo := &fakeRelRepo{}
	svc := updates.NewAdminService(repo)
	out, err := svc.UpsertChannel(context.Background(), updates.UpsertChannelInput{Platform: "mac", Channel: "direct", Version: "1", DownloadURL: "u"})
	require.NoError(t, err)
	require.Equal(t, "", out.SHA256)
}

func TestUpsertChannel_ExistingOverwritesOptionalOnlyWhenProvided(t *testing.T) {
	existing := updates.AppRelease{
		Platform: "mac", Channel: "direct", Version: "old", DownloadURL: "old",
		SHA256: "oldsha", ReleaseNotes: "old notes", Required: true, Enabled: false,
	}
	repo := &fakeRelRepo{getRelease: &existing}
	svc := updates.NewAdminService(repo)
	// No release_notes/required/enabled provided → keep existing.
	out, err := svc.UpsertChannel(context.Background(), updates.UpsertChannelInput{Platform: "mac", Channel: "direct", Version: "new", DownloadURL: "newurl"})
	require.NoError(t, err)
	require.Equal(t, "new", out.Version)
	require.Equal(t, "newurl", out.DownloadURL)
	require.Equal(t, "", out.SHA256) // sha cleared when not supplied
	require.Equal(t, "old notes", out.ReleaseNotes)
	require.Equal(t, true, out.Required)
	require.Equal(t, false, out.Enabled)
	require.Equal(t, "new", repo.updatedRel.Version)
}

func TestUpsertChannel_ExistingProvidedOptionalOverwrites(t *testing.T) {
	existing := updates.AppRelease{Platform: "mac", Channel: "direct", ReleaseNotes: "old", Required: false, Enabled: true}
	repo := &fakeRelRepo{getRelease: &existing}
	svc := updates.NewAdminService(repo)
	notes := "fresh"
	req := true
	en := false
	out, err := svc.UpsertChannel(context.Background(), updates.UpsertChannelInput{
		Platform: "mac", Channel: "direct", Version: "1", DownloadURL: "u",
		ReleaseNotes: &notes, Required: &req, Enabled: &en,
	})
	require.NoError(t, err)
	require.Equal(t, "fresh", out.ReleaseNotes)
	require.Equal(t, true, out.Required)
	require.Equal(t, false, out.Enabled)
}

func TestDeleteChannel_NotFound(t *testing.T) {
	repo := &fakeRelRepo{deleteAffected: 0}
	svc := updates.NewAdminService(repo)
	err := svc.DeleteChannel(context.Background(), "windows", "store")
	require.Error(t, err)
	require.ErrorIs(t, err, updates.ErrReleaseNotFound)
	require.EqualError(t, err, "No release row for windows/store")
}

func TestDeleteChannel_OK(t *testing.T) {
	repo := &fakeRelRepo{deleteAffected: 1}
	svc := updates.NewAdminService(repo)
	err := svc.DeleteChannel(context.Background(), "mac", "direct")
	require.NoError(t, err)
	require.Equal(t, "mac", repo.deletedPlat)
	require.Equal(t, "direct", repo.deletedChan)
}

func TestUpsertPolicy_BadPlatform(t *testing.T) {
	svc := updates.NewAdminService(&fakeRelRepo{})
	_, err := svc.UpsertPolicy(context.Background(), updates.UpsertPolicyInput{Platform: "nope"})
	require.EqualError(t, err, "platform must be one of: mac, windows, linux, android, ios")
}

func TestUpsertPolicy_BadPreferred(t *testing.T) {
	svc := updates.NewAdminService(&fakeRelRepo{})
	pref := "beta"
	_, err := svc.UpsertPolicy(context.Background(), updates.UpsertPolicyInput{Platform: "mac", Preferred: &pref})
	require.EqualError(t, err, "preferred must be one of: direct, store")
}

func TestUpsertPolicy_BadStoreStatus(t *testing.T) {
	svc := updates.NewAdminService(&fakeRelRepo{})
	ss := "bogus"
	_, err := svc.UpsertPolicy(context.Background(), updates.UpsertPolicyInput{Platform: "mac", StoreStatus: &ss})
	require.EqualError(t, err, "store_status must be one of: not_submitted, in_review, approved, rejected, n/a")
}

func TestUpsertPolicy_InsertDefaults(t *testing.T) {
	repo := &fakeRelRepo{} // no existing → insert
	svc := updates.NewAdminService(repo)
	out, err := svc.UpsertPolicy(context.Background(), updates.UpsertPolicyInput{Platform: "mac"})
	require.NoError(t, err)
	require.Equal(t, "direct", out.Preferred)
	require.Equal(t, "not_submitted", out.StoreStatus)
	require.Equal(t, "", out.Notes)
}

func TestUpsertPolicy_ExistingOverwritesOnlyProvided(t *testing.T) {
	existing := updates.AppReleasePolicy{Platform: "mac", Preferred: "store", StoreStatus: "approved", Notes: "keep"}
	repo := &fakeRelRepo{getPolicy: &existing}
	svc := updates.NewAdminService(repo)
	ss := "in_review"
	out, err := svc.UpsertPolicy(context.Background(), updates.UpsertPolicyInput{Platform: "mac", StoreStatus: &ss})
	require.NoError(t, err)
	require.Equal(t, "store", out.Preferred)       // unchanged
	require.Equal(t, "in_review", out.StoreStatus) // overwritten
	require.Equal(t, "keep", out.Notes)            // unchanged
}
