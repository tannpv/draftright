package feedback

import (
	"context"
	"strings"
	"unicode/utf16"
)

// CreateInput is the validated POST /feedback body (post-DTO). Title / Platform
// are only consulted when Kind=="feature".
type CreateInput struct {
	Kind        string // "feature" or "bug" (already @IsIn-validated by the DTO)
	Title       string // feature only; "" when absent
	Platform    string // target_platform; feature only; "" when absent
	Description string
	Source      string
	AppVersion  string // "" when absent
	OsInfo      string // "" when absent
	UserEmail   string // "" when absent
}

// NewRow is the validated insert payload handed to the repo. Title / Platform /
// UserID / UserEmail / AppVersion / OsInfo are nil when absent. Context is
// always nil on this route (the feedback route takes no screenshot/context;
// only the legacy multipart /bug-reports route does).
type NewRow struct {
	Kind           string
	Title          *string
	TargetPlatform *string
	Source         string
	Description    string
	AppVersion     *string
	OsInfo         *string
	UserID         *string
	UserEmail      *string
}

// ListParams carries the GET /feedback query after coercion. Page / Limit are
// the raw caller values (0 when absent → defaulted/clamped here). Status /
// Platform are nil to skip the filter (Node only sets the where key when the
// value passes its allow-list).
type ListParams struct {
	Page     int
	Limit    int
	Status   *string
	Platform *string
}

// Repo is the consumer-side port the feedback Service depends on. PgRepo
// satisfies it; tests fake it. All ids are plain strings (the repo parses to
// UUID); counts are ints (Node uses JS numbers).
type Repo interface {
	ResolveUserID(ctx context.Context, id *string) (*string, error)
	Insert(ctx context.Context, n NewRow) (Created, error)

	FeatureExists(ctx context.Context, featureID string) (bool, error)
	VoteExists(ctx context.Context, featureID, userID string) (bool, error)
	InsertVote(ctx context.Context, featureID, userID string) error
	DeleteVote(ctx context.Context, featureID, userID string) error
	CountVotes(ctx context.Context, featureID string) (int, error)
	UpdateVoteCount(ctx context.Context, featureID string, count int) error

	CountFeatures(ctx context.Context, status, platform *string) (int64, error)
	ListFeatures(ctx context.Context, status, platform *string, limit, offset int) ([]FeatureRow, error)
	VotedFeatureIDs(ctx context.Context, ids []string, userID string) (map[string]bool, error)
}

// Service ports the createFeedback / toggleVote / listPublicFeatures methods of
// BugReportsService.
type Service struct{ repo Repo }

// NewService wires the repo.
func NewService(repo Repo) *Service { return &Service{repo: repo} }

// CreateFeedback ports BugReportsService.createFeedback (bug-reports.service.ts:126-169):
// guard description/source, decide kind, for features validate title (1-80
// UTF-16 units) + target_platform, slice fields to their column widths, resolve
// the user id, and insert (context always NULL, status 'new', vote_count 0,
// is_public true). userID is the best-effort JWT subject ("" = anonymous).
func (s *Service) CreateFeedback(ctx context.Context, in CreateInput, userID string) (Created, error) {
	if strings.TrimSpace(in.Description) == "" {
		return Created{}, ErrDescriptionRequired
	}
	if strings.TrimSpace(in.Source) == "" {
		return Created{}, ErrSourceRequired
	}

	// kind = dto.kind === 'feature' ? 'feature' : 'bug'
	kind := "bug"
	if in.Kind == "feature" {
		kind = "feature"
	}

	var title, targetPlatform *string
	if kind == "feature" {
		t := strings.TrimSpace(in.Title)
		if n := lenUTF16(t); n < 1 || n > 80 {
			return Created{}, ErrTitleRequired
		}
		if in.Platform == "" || !PlatformValid(in.Platform) {
			return Created{}, ErrBadTargetPlatform
		}
		title = &t
		p := in.Platform
		targetPlatform = &p
	}

	// resolveUserId: null the FK when the JWT outlives its user.
	resolved, err := s.repo.ResolveUserID(ctx, ptrOrNil(userID))
	if err != nil {
		return Created{}, err
	}

	row := NewRow{
		Kind:           kind,
		Title:          title,
		TargetPlatform: targetPlatform,
		// source → 50, description trimmed (no slice), app_version → 50,
		// os_info → 100, user_email → 255 (Node .slice() widths).
		Source:      sliceUTF16(strings.TrimSpace(in.Source), 50),
		Description: strings.TrimSpace(in.Description),
		AppVersion:  sliceOrNil(in.AppVersion, 50),
		OsInfo:      sliceOrNil(in.OsInfo, 100),
		UserID:      resolved,
		UserEmail:   sliceOrNil(in.UserEmail, 255),
	}
	return s.repo.Insert(ctx, row)
}

// ToggleVote ports BugReportsService.toggleVote (bug-reports.service.ts:182-197):
// load the feature (kind='feature') or NotFound; delete or insert the caller's
// vote; recompute vote_count = COUNT(feature_votes) and persist it on the row.
// Idempotent.
func (s *Service) ToggleVote(ctx context.Context, featureID, userID string) (VoteResult, error) {
	exists, err := s.repo.FeatureExists(ctx, featureID)
	if err != nil {
		return VoteResult{}, err
	}
	if !exists {
		return VoteResult{}, ErrFeatureNotFound
	}

	voted, err := s.repo.VoteExists(ctx, featureID, userID)
	if err != nil {
		return VoteResult{}, err
	}
	var hasVoted bool
	if voted {
		if err := s.repo.DeleteVote(ctx, featureID, userID); err != nil {
			return VoteResult{}, err
		}
		hasVoted = false
	} else {
		if err := s.repo.InsertVote(ctx, featureID, userID); err != nil {
			return VoteResult{}, err
		}
		hasVoted = true
	}

	count, err := s.repo.CountVotes(ctx, featureID)
	if err != nil {
		return VoteResult{}, err
	}
	if err := s.repo.UpdateVoteCount(ctx, featureID, count); err != nil {
		return VoteResult{}, err
	}
	return VoteResult{VoteCount: count, HasVoted: hasVoted}, nil
}

// ListPublicFeatures ports BugReportsService.listPublicFeatures (bug-reports.service.ts:205-236):
// page>=1 (clamp), limit clamped 1..100 (default 20), offset=(page-1)*limit;
// fetch the page + total; when userID is set, fetch the caller's votes for the
// page's ids and stamp viewerHasVoted per row (always false when anonymous).
// Status / Platform filters are pre-validated by the handler (nil = skip).
func (s *Service) ListPublicFeatures(ctx context.Context, p ListParams, userID string) (ListResult, error) {
	// page = Math.max(1, Number(query.page) || 1) — 0/absent/negative → 1.
	page := p.Page
	if page < 1 {
		page = 1
	}
	// limit = Math.min(100, Math.max(1, Number(query.limit) || 20)) — 0/absent
	// defaults to 20 BEFORE the 1..100 clamp; a negative value skips the
	// default (truthy in JS) and is clamped up to 1.
	limit := p.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	total, err := s.repo.CountFeatures(ctx, p.Status, p.Platform)
	if err != nil {
		return ListResult{}, err
	}
	rows, err := s.repo.ListFeatures(ctx, p.Status, p.Platform, limit, offset)
	if err != nil {
		return ListResult{}, err
	}

	if userID != "" && len(rows) > 0 {
		ids := make([]string, len(rows))
		for i, r := range rows {
			ids[i] = r.ID
		}
		voted, err := s.repo.VotedFeatureIDs(ctx, ids, userID)
		if err != nil {
			return ListResult{}, err
		}
		for i := range rows {
			rows[i].ViewerHasVoted = voted[rows[i].ID]
		}
	}

	return ListResult{Rows: rows, Total: total}, nil
}

// ptrOrNil returns nil for "", else &s. Mirrors Node's `x ? ... : null`.
func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// sliceOrNil returns nil for "" (Node `x ? x.slice(0,n) : null`), else the
// value capped to n UTF-16 code units.
func sliceOrNil(s string, n int) *string {
	if s == "" {
		return nil
	}
	v := sliceUTF16(s, n)
	return &v
}

// sliceUTF16 caps s to n UTF-16 code units, mirroring JS String.slice(0, n).
// Package-local copy of the errreport/bugreports helper (per CLAUDE.md: keep
// per-package, do not promote to shared).
func sliceUTF16(s string, n int) string {
	u := utf16.Encode([]rune(s))
	if len(u) <= n {
		return s
	}
	return string(utf16.Decode(u[:n]))
}

// lenUTF16 returns the length of s in UTF-16 code units, matching JS
// String.length (what `t.length` measures in the 1-80 title check).
func lenUTF16(s string) int {
	return len(utf16.Encode([]rune(s)))
}
