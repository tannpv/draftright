package usage_test

import (
	"context"
	"testing"
	"time"

	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	"github.com/tannpv/draftright-rewrite/internal/usage"
)

type fakeQ struct {
	gotSince time.Time
	count    int64
}

func (f *fakeQ) CountUsageToday(ctx context.Context, arg sqlc.CountUsageTodayParams) (int64, error) {
	f.gotSince = arg.CreatedAt.Time
	return f.count, nil
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
