package rewritelog

// Unit tests for PgRepo — use a fake querier (no real DB).
// The fake satisfies the unexported rewriteQuerier interface defined in repo_pg.go.
//
// Tests cover the 5 public methods:
//   Count, CountByQuality, FindPending, UpdateQuality, FindApprovedAsc

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// fakeQuerier is a test-only implementation of rewriteQuerier.
// Each field is a func so individual tests can inject exactly the behaviour
// they need without inheriting a shared state struct.
type fakeQuerier struct {
	countFn              func(ctx context.Context) (int64, error)
	countByQualityFn     func(ctx context.Context) ([]sqlc.CountRewriteLogsByQualityRow, error)
	listPendingFn        func(ctx context.Context, arg sqlc.ListPendingRewriteLogsParams) ([]sqlc.RewriteLog, error)
	countPendingFn       func(ctx context.Context) (int64, error)
	updateQualityFn      func(ctx context.Context, arg sqlc.UpdateRewriteLogQualityParams) error
	listApprovedAscFn    func(ctx context.Context) ([]sqlc.RewriteLog, error)
	insertFn             func(ctx context.Context, arg sqlc.InsertRewriteLogParams) error
	listPendingCallArgs  *sqlc.ListPendingRewriteLogsParams // capture last call
	updateQualityCallArg *sqlc.UpdateRewriteLogQualityParams
	insertCallArg        *sqlc.InsertRewriteLogParams
}

func (f *fakeQuerier) CountRewriteLogs(ctx context.Context) (int64, error) {
	return f.countFn(ctx)
}
func (f *fakeQuerier) CountRewriteLogsByQuality(ctx context.Context) ([]sqlc.CountRewriteLogsByQualityRow, error) {
	return f.countByQualityFn(ctx)
}
func (f *fakeQuerier) ListPendingRewriteLogs(ctx context.Context, arg sqlc.ListPendingRewriteLogsParams) ([]sqlc.RewriteLog, error) {
	f.listPendingCallArgs = &arg
	return f.listPendingFn(ctx, arg)
}
func (f *fakeQuerier) CountPendingRewriteLogs(ctx context.Context) (int64, error) {
	return f.countPendingFn(ctx)
}
func (f *fakeQuerier) UpdateRewriteLogQuality(ctx context.Context, arg sqlc.UpdateRewriteLogQualityParams) error {
	f.updateQualityCallArg = &arg
	return f.updateQualityFn(ctx, arg)
}
func (f *fakeQuerier) ListApprovedRewriteLogsAsc(ctx context.Context) ([]sqlc.RewriteLog, error) {
	return f.listApprovedAscFn(ctx)
}
func (f *fakeQuerier) InsertRewriteLog(ctx context.Context, arg sqlc.InsertRewriteLogParams) error {
	f.insertCallArg = &arg
	return f.insertFn(ctx, arg)
}

// makeUUID builds a pgtype.UUID from a canonical string (panic on parse error —
// only call with known-good values in tests).
func makeUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		panic("makeUUID: " + err.Error())
	}
	return u
}

// makeTS builds a valid pgtype.Timestamp from a time.Time.
func makeTS(t time.Time) pgtype.Timestamp {
	return pgtype.Timestamp{Time: t, Valid: true}
}

// ── Test 1: Count ────────────────────────────────────────────────────────────

func TestCount_ReturnsTotalRowCount(t *testing.T) {
	q := &fakeQuerier{
		countFn: func(_ context.Context) (int64, error) { return 42, nil },
	}
	repo := NewPgRepo(q)
	got, err := repo.Count(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42 {
		t.Fatalf("Count() = %d, want 42", got)
	}
}

// ── Test 2: CountByQuality ───────────────────────────────────────────────────

// CountByQuality maps [{quality:"pending",n:5},{quality:"approved",n:3}] →
// (5, 3, 0) with 0 for missing "rejected", and ignores unknown quality values.
func TestCountByQuality_MapsKnownIgnoresUnknown(t *testing.T) {
	q := &fakeQuerier{
		countByQualityFn: func(_ context.Context) ([]sqlc.CountRewriteLogsByQualityRow, error) {
			return []sqlc.CountRewriteLogsByQualityRow{
				{Quality: "pending", N: 5},
				{Quality: "approved", N: 3},
				{Quality: "unknown_future_value", N: 99}, // must be ignored
			}, nil
		},
	}
	repo := NewPgRepo(q)
	pending, approved, rejected, err := repo.CountByQuality(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pending != 5 {
		t.Errorf("pending = %d, want 5", pending)
	}
	if approved != 3 {
		t.Errorf("approved = %d, want 3", approved)
	}
	if rejected != 0 {
		t.Errorf("rejected = %d, want 0 (missing from DB)", rejected)
	}
}

// ── Test 3: FindPending ──────────────────────────────────────────────────────

// FindPending(page=2, limit=10) must compute offset=10 and return (logs, total).
func TestFindPending_OffsetAndTotal(t *testing.T) {
	refID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	sqlcRow := sqlc.RewriteLog{
		ID:             makeUUID(refID),
		Tone:           "polished",
		InputText:      "hello",
		OutputText:     "world",
		Model:          "llama3.2",
		ProviderType:   "ollama",
		ResponseTimeMs: 150,
		Quality:        "pending",
		CreatedAt:      makeTS(now),
	}

	q := &fakeQuerier{
		listPendingFn: func(_ context.Context, _ sqlc.ListPendingRewriteLogsParams) ([]sqlc.RewriteLog, error) {
			return []sqlc.RewriteLog{sqlcRow}, nil
		},
		countPendingFn: func(_ context.Context) (int64, error) { return 25, nil },
	}
	repo := NewPgRepo(q)
	logs, total, err := repo.FindPending(context.Background(), 2, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Offset must be (page-1)*limit = (2-1)*10 = 10
	if q.listPendingCallArgs == nil {
		t.Fatal("ListPendingRewriteLogs was never called")
	}
	if q.listPendingCallArgs.Limit != 10 {
		t.Errorf("Limit = %d, want 10", q.listPendingCallArgs.Limit)
	}
	if q.listPendingCallArgs.Offset != 10 {
		t.Errorf("Offset = %d, want 10 (page=2,limit=10)", q.listPendingCallArgs.Offset)
	}

	if total != 25 {
		t.Errorf("total = %d, want 25", total)
	}
	if len(logs) != 1 {
		t.Fatalf("len(logs) = %d, want 1", len(logs))
	}
	if logs[0].ID != refID {
		t.Errorf("ID = %q, want %q", logs[0].ID, refID)
	}
}

// ── Test 4: UpdateQuality bad UUID ───────────────────────────────────────────

// UpdateQuality with a malformed id must return a non-nil error without calling
// the querier. Node 500s because Postgres throws "invalid input syntax for type
// uuid"; Go must surface an error so the handler can 500 too.
func TestUpdateQuality_BadUUID_ReturnsError(t *testing.T) {
	called := false
	q := &fakeQuerier{
		updateQualityFn: func(_ context.Context, _ sqlc.UpdateRewriteLogQualityParams) error {
			called = true
			return nil
		},
	}
	repo := NewPgRepo(q)
	err := repo.UpdateQuality(context.Background(), "not-a-uuid", "approved")
	if err == nil {
		t.Fatal("expected non-nil error for malformed UUID, got nil")
	}
	if called {
		t.Fatal("querier must NOT be called for a malformed UUID")
	}
}

// ── Test 5: FindApprovedAsc ──────────────────────────────────────────────────

// FindApprovedAsc maps all 9 fields correctly: uuid→string, Timestamp→time.Time,
// int32 ResponseTimeMs → int.
func TestFindApprovedAsc_MapsAllFields(t *testing.T) {
	refID := "11111111-2222-3333-4444-555555555555"
	ts := time.Date(2026, 1, 15, 8, 30, 0, 0, time.UTC)
	row := sqlc.RewriteLog{
		ID:             makeUUID(refID),
		Tone:           "casual",
		InputText:      "input text",
		OutputText:     "output text",
		Model:          "gpt-4",
		ProviderType:   "openai",
		ResponseTimeMs: 300,
		Quality:        "approved",
		CreatedAt:      makeTS(ts),
	}
	q := &fakeQuerier{
		listApprovedAscFn: func(_ context.Context) ([]sqlc.RewriteLog, error) {
			return []sqlc.RewriteLog{row}, nil
		},
	}
	repo := NewPgRepo(q)
	logs, err := repo.FindApprovedAsc(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("len(logs) = %d, want 1", len(logs))
	}
	got := logs[0]
	if got.ID != refID {
		t.Errorf("ID = %q, want %q", got.ID, refID)
	}
	if got.Tone != "casual" {
		t.Errorf("Tone = %q, want casual", got.Tone)
	}
	if got.InputText != "input text" {
		t.Errorf("InputText = %q", got.InputText)
	}
	if got.OutputText != "output text" {
		t.Errorf("OutputText = %q", got.OutputText)
	}
	if got.Model != "gpt-4" {
		t.Errorf("Model = %q", got.Model)
	}
	if got.ProviderType != "openai" {
		t.Errorf("ProviderType = %q", got.ProviderType)
	}
	if got.ResponseTimeMs != 300 {
		t.Errorf("ResponseTimeMs = %d, want 300", got.ResponseTimeMs)
	}
	if got.Quality != "approved" {
		t.Errorf("Quality = %q", got.Quality)
	}
	if !got.CreatedAt.Equal(ts) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, ts)
	}
}

// ── Test: Insert ─────────────────────────────────────────────────────────────

// Insert maps RewriteLogInput → InsertRewriteLogParams (response_time_ms int64
// → int32) and forwards to the querier. id/quality/created_at are DB defaults.
func TestInsert_MapsInputToParams(t *testing.T) {
	q := &fakeQuerier{
		insertFn: func(_ context.Context, _ sqlc.InsertRewriteLogParams) error { return nil },
	}
	repo := NewPgRepo(q)
	err := repo.Insert(context.Background(), RewriteLogInput{
		Tone:           "polished",
		InputText:      "hi",
		OutputText:     "Hello.",
		Model:          "gpt-4o",
		ProviderType:   "openai",
		ResponseTimeMs: 123,
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if q.insertCallArg == nil {
		t.Fatal("InsertRewriteLog not called")
	}
	got := *q.insertCallArg
	want := sqlc.InsertRewriteLogParams{
		Tone: "polished", InputText: "hi", OutputText: "Hello.",
		Model: "gpt-4o", ProviderType: "openai", ResponseTimeMs: 123,
	}
	if got != want {
		t.Fatalf("params = %+v, want %+v", got, want)
	}
}

// Insert surfaces the querier error unchanged (caller decides fire-and-forget).
func TestInsert_PropagatesError(t *testing.T) {
	q := &fakeQuerier{
		insertFn: func(_ context.Context, _ sqlc.InsertRewriteLogParams) error {
			return errSentinel
		},
	}
	repo := NewPgRepo(q)
	if err := repo.Insert(context.Background(), RewriteLogInput{}); err != errSentinel {
		t.Fatalf("err = %v, want errSentinel", err)
	}
}

var errSentinel = errorString("boom")

type errorString string

func (e errorString) Error() string { return string(e) }
