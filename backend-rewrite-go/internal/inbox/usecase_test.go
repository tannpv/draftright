package inbox

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/bugreports"
	"github.com/tannpv/draftright-rewrite/internal/errreport"
)

type fakeErrors struct {
	rows     []errreport.ErrorReportEntity
	total    int
	lastFilt errreport.AdminListFilter
	calls    int
}

func (f *fakeErrors) List(_ context.Context, filt errreport.AdminListFilter) ([]errreport.ErrorReportEntity, int, error) {
	f.lastFilt = filt
	f.calls++
	return f.rows, f.total, nil
}

type fakeBugs struct {
	rows   []bugreports.BugReportEntity
	total  int
	filts  []bugreports.AdminListFilter
	totals map[string]int // keyed by status+"/"+kind for per-call totals
}

func (f *fakeBugs) List(_ context.Context, filt bugreports.AdminListFilter) ([]bugreports.BugReportEntity, int, error) {
	f.filts = append(f.filts, filt)
	if f.totals != nil {
		return nil, f.totals[filt.Status+"/"+filt.Kind], nil
	}
	return f.rows, f.total, nil
}

func sp(s string) *string { return &s }

func TestCounts(t *testing.T) {
	errs := &fakeErrors{total: 7}
	bugs := &fakeBugs{totals: map[string]int{"new/bug": 3, "new/feature": 5}}
	svc := NewService(errs, bugs)
	c, err := svc.Counts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if c.NewBugs != 3 || c.NewFeatures != 5 || c.NewErrors != 7 || c.Total != 15 {
		t.Fatalf("counts wrong: %+v", c)
	}
	// error count call uses status=0, limit=1
	if errs.lastFilt.Status == nil || *errs.lastFilt.Status != 0 || errs.lastFilt.Limit != 1 {
		t.Fatalf("error count filter wrong: %+v", errs.lastFilt)
	}
	// bug calls: new/bug then new/feature, each Built limit 1
	if len(bugs.filts) != 2 {
		t.Fatalf("want 2 bug calls, got %d", len(bugs.filts))
	}
	if bugs.filts[0].Status != "new" || bugs.filts[0].Kind != "bug" {
		t.Fatalf("first bug filter wrong: %+v", bugs.filts[0])
	}
	if bugs.filts[0].Built.Limit != 1 {
		t.Fatalf("bug count limit should be 1, got %d", bugs.filts[0].Built.Limit)
	}
}

func TestFeed_MergeSortSlice(t *testing.T) {
	errs := &fakeErrors{
		rows: []errreport.ErrorReportEntity{
			{ID: "e1", Platform: "ios", Severity: "error", LastSeenAt: "2026-06-19T10:00:00.000Z", ErrorType: sp("TypeError"), Message: sp("boom"), Count: 4},
		},
		total: 1,
	}
	bugs := &fakeBugs{
		rows: []bugreports.BugReportEntity{
			{ID: "b1", Description: "app crash", Status: "new", CreatedAt: "2026-06-19T11:00:00.000Z", OsInfo: sp("Android 14"), ScreenshotPath: sp("/x.png")},
		},
		total: 1,
	}
	svc := NewService(errs, bugs)
	feed, err := svc.FeedFor(context.Background(), FeedQuery{Kind: "", OpenOnly: false, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 2 {
		t.Fatalf("want 2 items, got %d", len(feed.Items))
	}
	// b1 (11:00) is newer than e1 (10:00) → bug first after DESC sort.
	if bi, ok := feed.Items[0].(BugItem); !ok || bi.ID != "b1" {
		t.Fatalf("first item should be bug b1, got %#v", feed.Items[0])
	}
	if ei, ok := feed.Items[1].(ErrorItem); !ok || ei.ID != "e1" {
		t.Fatalf("second item should be error e1, got %#v", feed.Items[1])
	}
	if feed.Counts.Errors != 1 || feed.Counts.Bugs != 1 || feed.Counts.Returned != 2 {
		t.Fatalf("counts wrong: %+v", feed.Counts)
	}
}

func TestFeed_KindBugOnly(t *testing.T) {
	errs := &fakeErrors{rows: []errreport.ErrorReportEntity{{ID: "e1", LastSeenAt: "2026-06-19T10:00:00.000Z"}}, total: 1}
	bugs := &fakeBugs{rows: []bugreports.BugReportEntity{{ID: "b1", CreatedAt: "2026-06-19T09:00:00.000Z"}}, total: 1}
	svc := NewService(errs, bugs)
	feed, err := svc.FeedFor(context.Background(), FeedQuery{Kind: "bug", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if errs.calls != 0 {
		t.Fatalf("errors should not be fetched for kind=bug")
	}
	if len(feed.Items) != 1 {
		t.Fatalf("want 1 bug item, got %d", len(feed.Items))
	}
	if feed.Counts.Errors != 0 {
		t.Fatalf("error total should be 0 when not fetched")
	}
}

func TestFeed_OpenOnlyFiltersStatus(t *testing.T) {
	errs := &fakeErrors{}
	bugs := &fakeBugs{}
	svc := NewService(errs, bugs)
	_, err := svc.FeedFor(context.Background(), FeedQuery{Kind: "", OpenOnly: true, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if errs.lastFilt.Status == nil || *errs.lastFilt.Status != 0 {
		t.Fatalf("openOnly error status should be 0, got %v", errs.lastFilt.Status)
	}
	if len(bugs.filts) == 0 || bugs.filts[len(bugs.filts)-1].Status != "new" {
		t.Fatalf("openOnly bug status should be 'new', got %+v", bugs.filts)
	}
}

func TestFeed_LimitSlice(t *testing.T) {
	errs := &fakeErrors{
		rows: []errreport.ErrorReportEntity{
			{ID: "e1", LastSeenAt: "2026-06-19T10:00:00.000Z"},
			{ID: "e2", LastSeenAt: "2026-06-19T09:00:00.000Z"},
		},
		total: 2,
	}
	bugs := &fakeBugs{rows: []bugreports.BugReportEntity{{ID: "b1", CreatedAt: "2026-06-19T08:00:00.000Z"}}, total: 1}
	svc := NewService(errs, bugs)
	feed, err := svc.FeedFor(context.Background(), FeedQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 2 {
		t.Fatalf("want sliced to 2, got %d", len(feed.Items))
	}
	if feed.Counts.Returned != 2 {
		t.Fatalf("returned should be 2")
	}
	// totals are pre-slice
	if feed.Counts.Errors != 2 || feed.Counts.Bugs != 1 {
		t.Fatalf("totals should be pre-slice: %+v", feed.Counts)
	}
}

// TestFeed_NegativeLimitNoPanic guards the slice against a negative limit.
// In prod a negative limit never reaches the slice — Node and Go both pass it
// straight to the SQL LIMIT bind and Postgres rejects `LIMIT -5`
// ("LIMIT must not be negative") → both return 500. So negative limit is NOT a
// byte-parity divergence; this test only protects against the latent
// `merged[:negative]` panic that the in-memory fakes (no DB) would otherwise
// hit. See issue #40.
func TestFeed_NegativeLimitNoPanic(t *testing.T) {
	errs := &fakeErrors{
		rows:  []errreport.ErrorReportEntity{{ID: "e1", LastSeenAt: "2026-06-19T10:00:00.000Z"}},
		total: 1,
	}
	bugs := &fakeBugs{rows: []bugreports.BugReportEntity{{ID: "b1", CreatedAt: "2026-06-19T09:00:00.000Z"}}, total: 1}
	svc := NewService(errs, bugs)
	feed, err := svc.FeedFor(context.Background(), FeedQuery{Limit: -5})
	if err != nil {
		t.Fatal(err)
	}
	// No panic; with a negative limit the truncation is skipped, all rows pass.
	if len(feed.Items) != 2 {
		t.Fatalf("want 2 items (no truncation on negative limit), got %d", len(feed.Items))
	}
}

func TestErrorItem_JSONKeySet(t *testing.T) {
	it := mapErrorItem(errreport.ErrorReportEntity{
		ID: "e1", Platform: "ios", Severity: "error", LastSeenAt: "2026-06-19T10:00:00.000Z",
		ErrorType: sp("TypeError"), Message: sp("boom"), Count: 3,
	})
	b, _ := json.Marshal(it)
	s := string(b)
	for _, want := range []string{`"kind":"error"`, `"error_type":"TypeError"`, `"severity":"error"`, `"occurrence_count":3`, `"title":"TypeError: boom"`} {
		if !contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
	for _, bad := range []string{"user_email", "has_screenshot"} {
		if contains(s, bad) {
			t.Fatalf("error item must NOT have %s: %s", bad, s)
		}
	}
}

func TestBugItem_JSONKeySet(t *testing.T) {
	it := mapBugItem(bugreports.BugReportEntity{
		ID: "b1", Description: "crash", Status: "new", CreatedAt: "2026-06-19T10:00:00.000Z",
		OsInfo: sp("Android"), UserEmail: sp("u@x.com"), ScreenshotPath: sp("/p.png"),
	})
	b, _ := json.Marshal(it)
	s := string(b)
	for _, want := range []string{`"kind":"bug"`, `"user_email":"u@x.com"`, `"has_screenshot":true`, `"title":"crash"`} {
		if !contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
	for _, bad := range []string{"error_type", "severity", "occurrence_count"} {
		if contains(s, bad) {
			t.Fatalf("bug item must NOT have %s: %s", bad, s)
		}
	}
}

func TestErrorItem_EmptyTitleFallback(t *testing.T) {
	it := mapErrorItem(errreport.ErrorReportEntity{ID: "e", LastSeenAt: "2026-06-19T10:00:00.000Z"})
	if it.Title != "(no message)" {
		t.Fatalf("want (no message), got %q", it.Title)
	}
}

func TestBugItem_EmptyTitleFallback(t *testing.T) {
	it := mapBugItem(bugreports.BugReportEntity{ID: "b", CreatedAt: "2026-06-19T10:00:00.000Z"})
	if it.Title != "(no description)" {
		t.Fatalf("want (no description), got %q", it.Title)
	}
}

func TestErrorStatusLabel(t *testing.T) {
	cases := map[int]string{0: "new", 1: "reviewing", 2: "resolved", 3: "fix_proposed", 4: "resolved", 5: "wont_fix", 9: "new", -1: "new"}
	for in, want := range cases {
		if got := errorStatusLabel(in); got != want {
			t.Fatalf("label(%d)=%q want %q", in, got, want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
