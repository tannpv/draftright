package inbox

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/tannpv/draftright-rewrite/internal/bugreports"
	"github.com/tannpv/draftright-rewrite/internal/errreport"
	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// errorLister is the consumer port for the error_reports admin list. Satisfied
// by *errreport.AdminService.
type errorLister interface {
	List(ctx context.Context, f errreport.AdminListFilter) ([]errreport.ErrorReportEntity, int, error)
}

// bugLister is the consumer port for the bug_reports admin list. Satisfied by
// *bugreports.AdminService.
type bugLister interface {
	List(ctx context.Context, f bugreports.AdminListFilter) ([]bugreports.BugReportEntity, int, error)
}

// Service aggregates the two admin lists into the inbox counts + feed.
type Service struct {
	errors errorLister
	bugs   bugLister
}

// NewService wires the error + bug admin listers (main.go injects the
// concretes).
func NewService(errs errorLister, bugs bugLister) *Service {
	return &Service{errors: errs, bugs: bugs}
}

// Counts mirrors AdminController.inboxCounts: 3 list calls (bug new, feature
// new, error status=0) each with limit 1, taking only the totals. Returns
// { new_bugs, new_features, new_errors, total }.
func (s *Service) Counts(ctx context.Context) (Counts, error) {
	_, newBugs, err := s.bugs.List(ctx, bugFilter(1, "new", "bug"))
	if err != nil {
		return Counts{}, err
	}
	_, newFeatures, err := s.bugs.List(ctx, bugFilter(1, "new", "feature"))
	if err != nil {
		return Counts{}, err
	}
	zero := 0
	_, newErrors, err := s.errors.List(ctx, errreport.AdminListFilter{Status: &zero, Limit: 1, Offset: 0})
	if err != nil {
		return Counts{}, err
	}
	return Counts{
		NewBugs:     newBugs,
		NewFeatures: newFeatures,
		NewErrors:   newErrors,
		Total:       newBugs + newFeatures + newErrors,
	}, nil
}

// FeedQuery is the decoded GET /admin/inbox query: kind (error|bug|empty),
// openOnly (status=='open'), and the capped limit (handler computes it).
type FeedQuery struct {
	Kind     string
	OpenOnly bool
	Limit    int
}

// FeedFor mirrors AdminController.listInbox: conditionally fetch error + bug
// rows (kind filter), map each into its source-specific item shape, concat
// (errors then bugs), stable-sort by created_at DESC, and slice to limit. The
// counts object reports each source's TOTAL (pre-slice) + returned length.
func (s *Service) FeedFor(ctx context.Context, q FeedQuery) (Feed, error) {
	wantErrors := q.Kind != "bug"
	wantBugs := q.Kind != "error"

	var errorEntities []errreport.ErrorReportEntity
	var errorTotal int
	if wantErrors {
		f := errreport.AdminListFilter{Limit: q.Limit, Offset: 0}
		if q.OpenOnly {
			zero := 0
			f.Status = &zero
		}
		rows, total, err := s.errors.List(ctx, f)
		if err != nil {
			return Feed{}, err
		}
		errorEntities, errorTotal = rows, total
	}

	var bugEntities []bugreports.BugReportEntity
	var bugTotal int
	if wantBugs {
		status := ""
		if q.OpenOnly {
			status = "new"
		}
		rows, total, err := s.bugs.List(ctx, bugFilter(q.Limit, status, ""))
		if err != nil {
			return Feed{}, err
		}
		bugEntities, bugTotal = rows, total
	}

	// Map each source into its item shape (errors first, then bugs — the
	// concat order Node uses before the stable sort).
	merged := make([]any, 0, len(errorEntities)+len(bugEntities))
	for _, e := range errorEntities {
		merged = append(merged, mapErrorItem(e))
	}
	for _, b := range bugEntities {
		merged = append(merged, mapBugItem(b))
	}

	// Stable sort by created_at DESC. JS Array.sort is stable, so equal
	// timestamps keep the concat order (errors before bugs).
	sort.SliceStable(merged, func(i, j int) bool {
		return itemTime(merged[i]).After(itemTime(merged[j]))
	})

	// Truncate to limit. The q.Limit >= 0 guard prevents a `merged[:negative]`
	// panic: a negative limit never reaches here in prod (the SQL LIMIT bind
	// rejects it — Postgres "LIMIT must not be negative" → 500, matching Node),
	// but the in-memory fakes in tests can. See issue #40.
	if q.Limit >= 0 && len(merged) > q.Limit {
		merged = merged[:q.Limit]
	}

	return Feed{
		Items: merged,
		Counts: FeedCounts{
			Errors:   errorTotal,
			Bugs:     bugTotal,
			Returned: len(merged),
		},
	}, nil
}

// bugFilter builds the bugreports AdminListFilter the inbox needs: no search,
// ORDER BY br.created_at DESC, the given limit (page 1), and optional
// status/kind custom filters. status/kind == "" means no predicate.
func bugFilter(limit int, status, kind string) bugreports.AdminListFilter {
	built := listquery.Build(
		listquery.Query{Limit: &limit, SortBy: "created_at", SortOrder: "DESC"},
		nil, // no search columns — inbox never searches
		nil, // no sort allow-list — default br.created_at applies
		"br.created_at",
		"", // no is_active status column
	)
	return bugreports.AdminListFilter{Built: built, Status: status, Kind: kind}
}

// mapErrorItem mirrors the Node errorItems map. title = [error_type,
// message].filter(Boolean).join(': ').slice(0,200) || '(no message)'.
// created_at = last_seen_at ?? first_seen_at ?? created_at (the entity holds
// only the seen-at ISO strings, both non-empty in practice).
func mapErrorItem(e errreport.ErrorReportEntity) ErrorItem {
	parts := make([]string, 0, 2)
	if e.ErrorType != nil && *e.ErrorType != "" {
		parts = append(parts, *e.ErrorType)
	}
	if e.Message != nil && *e.Message != "" {
		parts = append(parts, *e.Message)
	}
	title := sliceUTF16(strings.Join(parts, ": "), 200)
	if title == "" {
		title = "(no message)"
	}
	createdAt := e.LastSeenAt
	if createdAt == "" {
		createdAt = e.FirstSeenAt
	}
	// platform/severity are NOT-NULL string columns on error_reports; Node's
	// `e.platform ?? null` / `e.severity ?? null` are no-ops (the value is
	// always a string), so emit the string directly via a pointer.
	platform := e.Platform
	severity := e.Severity
	return ErrorItem{
		Kind:            "error",
		ID:              e.ID,
		Title:           title,
		Platform:        &platform,
		AppVersion:      e.AppVersion,
		Status:          errorStatusLabel(e.Status),
		CreatedAt:       createdAt,
		AiFixProposal:   e.AiFixProposal,
		ErrorType:       e.ErrorType,
		Severity:        &severity,
		OccurrenceCount: e.Count,
	}
}

// mapBugItem mirrors the Node bugItems map. title = description.slice(0,200) ||
// '(no description)'. has_screenshot = !!screenshot_path.
func mapBugItem(b bugreports.BugReportEntity) BugItem {
	title := sliceUTF16(b.Description, 200)
	if title == "" {
		title = "(no description)"
	}
	return BugItem{
		Kind:          "bug",
		ID:            b.ID,
		Title:         title,
		Platform:      b.OsInfo,
		AppVersion:    b.AppVersion,
		Status:        b.Status,
		CreatedAt:     b.CreatedAt,
		AiFixProposal: b.AiFixProposal,
		UserEmail:     b.UserEmail,
		HasScreenshot: b.ScreenshotPath != nil && *b.ScreenshotPath != "",
	}
}

// itemTime parses an item's created_at ISO string for the DESC sort. The
// strings come from shared.ISOMillis (RFC3339 with millis + Z); an unparseable
// value sorts as the zero time (oldest), mirroring `new Date(bad).getTime()`
// being NaN — which JS treats as last in a descending comparator.
func itemTime(v any) time.Time {
	var s string
	switch it := v.(type) {
	case ErrorItem:
		s = it.CreatedAt
	case BugItem:
		s = it.CreatedAt
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

// sliceUTF16 caps s to n UTF-16 code units, mirroring JS String.slice(0, n).
func sliceUTF16(s string, n int) string {
	u := utf16.Encode([]rune(s))
	if len(u) <= n {
		return s
	}
	return string(utf16.Decode(u[:n]))
}
