package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/platform/scheduler"
)

func TestNextDailyAt(t *testing.T) {
	loc := time.UTC
	cases := []struct {
		name       string
		from, want time.Time
	}{
		{
			name: "before target today rolls to today",
			from: time.Date(2026, 6, 14, 8, 0, 0, 0, loc),
			want: time.Date(2026, 6, 14, 9, 0, 0, 0, loc),
		},
		{
			name: "after target today rolls to tomorrow",
			from: time.Date(2026, 6, 14, 9, 30, 0, 0, loc),
			want: time.Date(2026, 6, 15, 9, 0, 0, 0, loc),
		},
		{
			name: "exactly on target rolls to tomorrow (strictly after)",
			from: time.Date(2026, 6, 14, 9, 0, 0, 0, loc),
			want: time.Date(2026, 6, 15, 9, 0, 0, 0, loc),
		},
		{
			name: "month and year rollover",
			from: time.Date(2026, 12, 31, 23, 0, 0, 0, loc),
			want: time.Date(2027, 1, 1, 9, 0, 0, 0, loc),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := scheduler.NextDailyAt(tc.from, 9, 0, loc)
			if !got.Equal(tc.want) {
				t.Fatalf("NextDailyAt(%v) = %v, want %v", tc.from, got, tc.want)
			}
		})
	}
}

func TestRunDailyReturnsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled — must return without firing the job

	fired := false
	done := make(chan struct{})
	go func() {
		scheduler.RunDaily(ctx, 9, 0, time.UTC,
			func() time.Time { return time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC) },
			func(context.Context, time.Time) { fired = true })
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunDaily did not return on cancelled context")
	}
	if fired {
		t.Fatal("job fired despite cancelled context")
	}
}
