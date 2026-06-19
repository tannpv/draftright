package rewritelog_test

// Unit tests for rewritelog.Service — fake Repo, no DB.
// Covers: Stats, ListPending, Review, ExportJSONL (format + empty + html-escape).

import (
	"context"
	"errors"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/rewritelog"
)

// ── fake repo ────────────────────────────────────────────────────────────────

type fakeRepo struct {
	// Stats
	total             int
	countErr          error
	cntPending        int
	cntApproved       int
	cntRejected       int
	countByQualityErr error

	// ListPending
	pendingLogs   []rewritelog.RewriteLog
	pendingTotal  int
	pendingErr    error
	capturedPage  int
	capturedLimit int

	// Review
	reviewErr       error
	capturedID      string
	capturedQuality string

	// ExportJSONL
	approved        []rewritelog.RewriteLog
	findApprovedErr error
}

func (f *fakeRepo) Count(ctx context.Context) (int, error) {
	return f.total, f.countErr
}

func (f *fakeRepo) CountByQuality(ctx context.Context) (pending, approved, rejected int, err error) {
	return f.cntPending, f.cntApproved, f.cntRejected, f.countByQualityErr
}

func (f *fakeRepo) FindPending(ctx context.Context, page, limit int) ([]rewritelog.RewriteLog, int, error) {
	f.capturedPage = page
	f.capturedLimit = limit
	return f.pendingLogs, f.pendingTotal, f.pendingErr
}

func (f *fakeRepo) UpdateQuality(ctx context.Context, id, quality string) error {
	f.capturedID = id
	f.capturedQuality = quality
	return f.reviewErr
}

func (f *fakeRepo) FindApprovedAsc(ctx context.Context) ([]rewritelog.RewriteLog, error) {
	return f.approved, f.findApprovedErr
}

// ── Stats ────────────────────────────────────────────────────────────────────

func TestStats_Aggregates(t *testing.T) {
	f := &fakeRepo{total: 10, cntPending: 3, cntApproved: 5, cntRejected: 2}
	s := rewritelog.NewService(f)
	got, err := s.Stats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Total != 10 {
		t.Errorf("Total: got %d, want 10", got.Total)
	}
	if got.Pending != 3 {
		t.Errorf("Pending: got %d, want 3", got.Pending)
	}
	if got.Approved != 5 {
		t.Errorf("Approved: got %d, want 5", got.Approved)
	}
	if got.Rejected != 2 {
		t.Errorf("Rejected: got %d, want 2", got.Rejected)
	}
}

func TestStats_CountError(t *testing.T) {
	f := &fakeRepo{countErr: errors.New("db down")}
	_, err := rewritelog.NewService(f).Stats(context.Background())
	if err == nil {
		t.Fatal("expected error from Count, got nil")
	}
}

func TestStats_CountByQualityError(t *testing.T) {
	f := &fakeRepo{total: 1, countByQualityErr: errors.New("db down")}
	_, err := rewritelog.NewService(f).Stats(context.Background())
	if err == nil {
		t.Fatal("expected error from CountByQuality, got nil")
	}
}

// ── ListPending ──────────────────────────────────────────────────────────────

func TestListPending_PassesThrough(t *testing.T) {
	want := []rewritelog.RewriteLog{{ID: "abc", Tone: "polished"}}
	f := &fakeRepo{pendingLogs: want, pendingTotal: 42}
	s := rewritelog.NewService(f)

	logs, total, err := s.ListPending(context.Background(), 3, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 42 {
		t.Errorf("total: got %d, want 42", total)
	}
	if len(logs) != 1 || logs[0].ID != "abc" {
		t.Errorf("logs: got %v, want %v", logs, want)
	}
	if f.capturedPage != 3 {
		t.Errorf("page: got %d, want 3", f.capturedPage)
	}
	if f.capturedLimit != 10 {
		t.Errorf("limit: got %d, want 10", f.capturedLimit)
	}
}

func TestListPending_PropagatesError(t *testing.T) {
	f := &fakeRepo{pendingErr: errors.New("repo fail")}
	_, _, err := rewritelog.NewService(f).ListPending(context.Background(), 1, 20)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── Review ───────────────────────────────────────────────────────────────────

func TestReview_DelegatesArguments(t *testing.T) {
	f := &fakeRepo{}
	if err := rewritelog.NewService(f).Review(context.Background(), "id-123", "approved"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.capturedID != "id-123" {
		t.Errorf("id: got %q, want %q", f.capturedID, "id-123")
	}
	if f.capturedQuality != "approved" {
		t.Errorf("quality: got %q, want %q", f.capturedQuality, "approved")
	}
}

func TestReview_PropagatesError(t *testing.T) {
	f := &fakeRepo{reviewErr: errors.New("update failed")}
	err := rewritelog.NewService(f).Review(context.Background(), "x", "rejected")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── ExportJSONL ──────────────────────────────────────────────────────────────

func TestExportJSONL_Format(t *testing.T) {
	f := &fakeRepo{approved: []rewritelog.RewriteLog{
		{Tone: "polished", InputText: "hi", OutputText: "Hello."},
	}}
	s := rewritelog.NewService(f)
	out, err := s.ExportJSONL(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"messages":[{"role":"system","content":"Rewrite the following text in a polished tone. Return only the rewritten text."},{"role":"user","content":"hi"},{"role":"assistant","content":"Hello."}]}`
	if out != want {
		t.Errorf("ExportJSONL output mismatch\ngot:  %s\nwant: %s", out, want)
	}
}

func TestExportJSONL_MultipleLines(t *testing.T) {
	f := &fakeRepo{approved: []rewritelog.RewriteLog{
		{Tone: "simple", InputText: "a", OutputText: "b"},
		{Tone: "concise", InputText: "c", OutputText: "d"},
	}}
	out, err := rewritelog.NewService(f).ExportJSONL(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// must contain exactly one '\n' joining two JSON objects
	line1 := `{"messages":[{"role":"system","content":"Rewrite the following text in a simple tone. Return only the rewritten text."},{"role":"user","content":"a"},{"role":"assistant","content":"b"}]}`
	line2 := `{"messages":[{"role":"system","content":"Rewrite the following text in a concise tone. Return only the rewritten text."},{"role":"user","content":"c"},{"role":"assistant","content":"d"}]}`
	want := line1 + "\n" + line2
	if out != want {
		t.Errorf("multi-line mismatch\ngot:  %s\nwant: %s", out, want)
	}
}

func TestExportJSONL_Empty(t *testing.T) {
	s := rewritelog.NewService(&fakeRepo{})
	out, err := s.ExportJSONL(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty string, got %q", out)
	}
}

func TestExportJSONL_NoHTMLEscape(t *testing.T) {
	// Go's json.Marshal (HTML mode) emits < as the 6-char sequence \u003c,
	// > as \u003e, & as \u0026. Node's JSON.stringify does NOT escape these.
	// Our encoder must produce literal characters, not \u-escaped forms.
	f := &fakeRepo{approved: []rewritelog.RewriteLog{{Tone: "casual", InputText: "a < b & c > d", OutputText: "x"}}}
	out, err := rewritelog.NewService(f).ExportJSONL(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Literal characters must appear (correctly unescaped).
	if !containsLiteral(out, "a < b & c > d") {
		t.Errorf("ExportJSONL mangled input_text; got: %s", out)
	}
	// The JSON \u-escaped forms produced by html mode must NOT appear.
	// "\\u003c" in Go source = the 6-byte sequence: \ u 0 0 3 c in the string.
	for _, htmlForm := range []string{"\\u003c", "\\u003e", "\\u0026"} {
		if containsLiteral(out, htmlForm) {
			t.Errorf("ExportJSONL contains HTML-escaped form %q in: %s", htmlForm, out)
		}
	}
}

func TestExportJSONL_FindApprovedError(t *testing.T) {
	f := &fakeRepo{findApprovedErr: errors.New("db fail")}
	_, err := rewritelog.NewService(f).ExportJSONL(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// containsLiteral is a plain byte-string contains check (no regex, no encoding).
func containsLiteral(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
