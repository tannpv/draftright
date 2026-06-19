package updates

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// platforms/channels/storeStatuses mirror the Node service-level enums
// (releases.service.ts + policies.service.ts). Their join order drives the
// verbatim validation messages, so they must match Node byte-for-byte.
var (
	platforms     = []string{"mac", "windows", "linux", "android", "ios"}
	channels      = []string{"direct", "store"}
	storeStatuses = []string{"not_submitted", "in_review", "approved", "rejected", "n/a"}
)

// ErrReleaseNotFound is the sentinel for "deleteChannel matched no row". The
// handler maps it to 404 NotFoundException. DeleteChannel returns a
// releaseNotFoundError whose Error() is the verbatim Node message
// ("No release row for {platform}/{channel}") and which Is()-matches this
// sentinel so the handler can branch on it.
var ErrReleaseNotFound = errors.New("release not found")

// releaseNotFoundError carries the verbatim 404 message while matching the
// ErrReleaseNotFound sentinel via errors.Is.
type releaseNotFoundError struct{ msg string }

func (e releaseNotFoundError) Error() string { return e.msg }
func (e releaseNotFoundError) Is(target error) bool {
	return target == ErrReleaseNotFound
}

var sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)

func contains(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

// ReleasesView is the GET /admin/releases body. Field order = PLATFORMS order
// so JSON keys serialize mac, windows, linux, android, ios.
type ReleasesView struct {
	Mac     PlatformReleases `json:"mac"`
	Windows PlatformReleases `json:"windows"`
	Linux   PlatformReleases `json:"linux"`
	Android PlatformReleases `json:"android"`
	Ios     PlatformReleases `json:"ios"`
}

// PlatformReleases is one platform card: the policy plus both channels.
type PlatformReleases struct {
	Policy   *AppReleasePolicy `json:"policy"`
	Channels Channels          `json:"channels"`
}

// Channels holds the direct and store release rows for one platform.
type Channels struct {
	Direct *AppRelease `json:"direct"`
	Store  *AppRelease `json:"store"`
}

// adminRepo is the persistence port for admin release/policy operations (E1).
type adminRepo interface {
	ListAllReleases(ctx context.Context) ([]AppRelease, error)
	ListAllPolicies(ctx context.Context) ([]AppReleasePolicy, error)
	GetReleaseChannel(ctx context.Context, platform, channel string) (*AppRelease, error)
	InsertRelease(ctx context.Context, in AppRelease) (AppRelease, error)
	UpdateRelease(ctx context.Context, in AppRelease) (AppRelease, error)
	DeleteRelease(ctx context.Context, platform, channel string) (int, error)
	GetPolicy(ctx context.Context, platform string) (*AppReleasePolicy, error)
	InsertPolicy(ctx context.Context, in AppReleasePolicy) (AppReleasePolicy, error)
	UpdatePolicy(ctx context.Context, in AppReleasePolicy) (AppReleasePolicy, error)
}

// AdminService implements the admin release/policy use cases (R1–R4 logic).
type AdminService struct {
	repo adminRepo
}

// NewAdminService wires the release/policy repo.
func NewAdminService(repo adminRepo) *AdminService { return &AdminService{repo: repo} }

// UpsertChannelInput is the upsertChannel argument; optional fields are
// pointers so "absent" (nil) differs from "" / false the way Node's
// `dto.x !== undefined` checks do.
type UpsertChannelInput struct {
	Platform     string
	Channel      string
	Version      string
	DownloadURL  string
	Sha256       *string
	ReleaseNotes *string
	Required     *bool
	Enabled      *bool
}

// UpsertPolicyInput is the policy upsert argument; optional fields are
// pointers to distinguish "not provided" from a zero value.
type UpsertPolicyInput struct {
	Platform    string
	Preferred   *string
	StoreStatus *string
	Notes       *string
}

// ListAll ports ReleasesService.listAll(): seed every platform in PLATFORMS
// order, then fill each platform's channels and policy from the rows/policies
// (skipping any whose platform isn't one of PLATFORMS).
func (s *AdminService) ListAll(ctx context.Context) (ReleasesView, error) {
	rows, err := s.repo.ListAllReleases(ctx)
	if err != nil {
		return ReleasesView{}, err
	}
	policies, err := s.repo.ListAllPolicies(ctx)
	if err != nil {
		return ReleasesView{}, err
	}

	byPlatform := map[string]*PlatformReleases{
		"mac":     {},
		"windows": {},
		"linux":   {},
		"android": {},
		"ios":     {},
	}
	for i := range rows {
		row := rows[i]
		pr, ok := byPlatform[row.Platform]
		if !ok {
			continue
		}
		switch row.Channel {
		case "direct":
			pr.Channels.Direct = &row
		case "store":
			pr.Channels.Store = &row
		}
	}
	for i := range policies {
		pol := policies[i]
		pr, ok := byPlatform[pol.Platform]
		if !ok {
			continue
		}
		pr.Policy = &pol
	}

	return ReleasesView{
		Mac:     *byPlatform["mac"],
		Windows: *byPlatform["windows"],
		Linux:   *byPlatform["linux"],
		Android: *byPlatform["android"],
		Ios:     *byPlatform["ios"],
	}, nil
}

// UpsertChannel ports ReleasesService.upsertChannel: validate (verbatim
// messages), lowercase sha, then load-then-branch overwriting optional fields
// only when provided (mirrors Node's per-field `if (x !== undefined)`).
func (s *AdminService) UpsertChannel(ctx context.Context, in UpsertChannelInput) (AppRelease, error) {
	channel := in.Channel
	if channel == "" {
		channel = "direct"
	}
	if !contains(platforms, in.Platform) {
		return AppRelease{}, fmt.Errorf("platform must be one of: %s", strings.Join(platforms, ", "))
	}
	if !contains(channels, channel) {
		return AppRelease{}, fmt.Errorf("channel must be one of: %s", strings.Join(channels, ", "))
	}
	if in.Version == "" || in.DownloadURL == "" {
		return AppRelease{}, errors.New("version and download_url are required")
	}
	if in.Sha256 != nil && *in.Sha256 != "" && !sha256Re.MatchString(strings.ToLower(*in.Sha256)) {
		return AppRelease{}, errors.New("sha256 must be a 64-char hex string (or empty)")
	}

	// sha256 = input.sha256?.toLowerCase() — undefined stays "undefined";
	// the `?? ''` below substitutes '' when not provided.
	sha := ""
	if in.Sha256 != nil {
		sha = strings.ToLower(*in.Sha256)
	}

	existing, err := s.repo.GetReleaseChannel(ctx, in.Platform, channel)
	if err != nil {
		return AppRelease{}, err
	}
	if existing != nil {
		existing.Version = in.Version
		existing.DownloadURL = in.DownloadURL
		// A new artifact replaces the hash; an absent hash clears the stale one.
		existing.SHA256 = sha
		if in.ReleaseNotes != nil {
			existing.ReleaseNotes = *in.ReleaseNotes
		}
		if in.Required != nil {
			existing.Required = *in.Required
		}
		if in.Enabled != nil {
			existing.Enabled = *in.Enabled
		}
		return s.repo.UpdateRelease(ctx, *existing)
	}

	row := AppRelease{
		Platform:     in.Platform,
		Channel:      channel,
		Version:      in.Version,
		DownloadURL:  in.DownloadURL,
		SHA256:       sha,
		ReleaseNotes: derefOr(in.ReleaseNotes, ""),
		Required:     derefBool(in.Required, false),
		Enabled:      derefBool(in.Enabled, true),
	}
	return s.repo.InsertRelease(ctx, row)
}

// DeleteChannel ports ReleasesService.deleteChannel: 0 rows affected →
// ErrReleaseNotFound carrying the "No release row for {platform}/{channel}"
// message Node throws as NotFoundException.
func (s *AdminService) DeleteChannel(ctx context.Context, platform, channel string) error {
	affected, err := s.repo.DeleteRelease(ctx, platform, channel)
	if err != nil {
		return err
	}
	if affected == 0 {
		return releaseNotFoundError{msg: fmt.Sprintf("No release row for %s/%s", platform, channel)}
	}
	return nil
}

// UpsertPolicy ports PoliciesService.upsert: validate (verbatim messages),
// then load-then-branch overwriting optional fields only when provided.
func (s *AdminService) UpsertPolicy(ctx context.Context, in UpsertPolicyInput) (AppReleasePolicy, error) {
	if !contains(platforms, in.Platform) {
		return AppReleasePolicy{}, fmt.Errorf("platform must be one of: %s", strings.Join(platforms, ", "))
	}
	if in.Preferred != nil && !contains(channels, *in.Preferred) {
		return AppReleasePolicy{}, fmt.Errorf("preferred must be one of: %s", strings.Join(channels, ", "))
	}
	if in.StoreStatus != nil && !contains(storeStatuses, *in.StoreStatus) {
		return AppReleasePolicy{}, fmt.Errorf("store_status must be one of: %s", strings.Join(storeStatuses, ", "))
	}

	existing, err := s.repo.GetPolicy(ctx, in.Platform)
	if err != nil {
		return AppReleasePolicy{}, err
	}
	if existing != nil {
		if in.Preferred != nil {
			existing.Preferred = *in.Preferred
		}
		if in.StoreStatus != nil {
			existing.StoreStatus = *in.StoreStatus
		}
		if in.Notes != nil {
			existing.Notes = *in.Notes
		}
		return s.repo.UpdatePolicy(ctx, *existing)
	}

	row := AppReleasePolicy{
		Platform:    in.Platform,
		Preferred:   derefOr(in.Preferred, "direct"),
		StoreStatus: derefOr(in.StoreStatus, "not_submitted"),
		Notes:       derefOr(in.Notes, ""),
	}
	return s.repo.InsertPolicy(ctx, row)
}

func derefOr(p *string, def string) string {
	if p != nil {
		return *p
	}
	return def
}

func derefBool(p *bool, def bool) bool {
	if p != nil {
		return *p
	}
	return def
}
