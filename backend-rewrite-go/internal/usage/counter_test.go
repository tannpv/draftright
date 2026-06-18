package usage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	"github.com/tannpv/draftright-rewrite/internal/usage"
)

type fakeQ struct {
	gotSince         time.Time
	count            int64
	gotTodayBoundary time.Time
	today            int64
	gotMonthBoundary time.Time
	month            int64
}

func (f *fakeQ) CountUsageToday(ctx context.Context, arg sqlc.CountUsageTodayParams) (int64, error) {
	f.gotSince = arg.CreatedAt.Time
	return f.count, nil
}

func (f *fakeQ) CountUsageTodayAll(ctx context.Context, createdAt pgtype.Timestamp) (int64, error) {
	f.gotTodayBoundary = createdAt.Time
	return f.today, nil
}

func (f *fakeQ) CountUsageThisMonthAll(ctx context.Context, createdAt pgtype.Timestamp) (int64, error) {
	f.gotMonthBoundary = createdAt.Time
	return f.month, nil
}

func TestCountToday_PassesLocalMidnight(t *testing.T) {
	f := &fakeQ{count: 7}
	c := usage.NewCounter(f)
	n, err := c.CountToday(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Fatalf("count = %d, want 7", n)
	}
	now := time.Now()
	wantMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if !f.gotSince.Equal(wantMidnight) {
		t.Fatalf("since = %v, want local midnight %v", f.gotSince, wantMidnight)
	}
}

func TestCountToday_BadUUIDReturnsZero(t *testing.T) {
	f := &fakeQ{count: 99}
	c := usage.NewCounter(f)
	n, err := c.CountToday(context.Background(), "not-a-uuid")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("bad uuid should count 0, got %d", n)
	}
}

func TestCountTodayAll(t *testing.T) {
	now := time.Date(2026, 6, 18, 14, 30, 0, 0, time.UTC)
	want := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	f := &fakeQ{today: 42}
	c := usage.NewCounter(f)
	got, err := c.CountTodayAllAt(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, 42, got)
	require.Equal(t, want, f.gotTodayBoundary)
}

func TestCountThisMonthAll(t *testing.T) {
	now := time.Date(2026, 6, 18, 14, 30, 0, 0, time.UTC)
	want := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	f := &fakeQ{month: 99}
	c := usage.NewCounter(f)
	got, err := c.CountThisMonthAllAt(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, 99, got)
	require.Equal(t, want, f.gotMonthBoundary)
}
