package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tannpv/draftright-rewrite/internal/platform/scheduler"
)

func TestNextHourlyAt(t *testing.T) {
	loc := time.UTC
	// 12:30 → next :00 is 13:00
	from := time.Date(2026, 6, 19, 12, 30, 0, 0, loc)
	require.Equal(t, time.Date(2026, 6, 19, 13, 0, 0, 0, loc), scheduler.NextHourlyAt(from, loc))
	// exactly on :00 rolls to next hour
	on := time.Date(2026, 6, 19, 13, 0, 0, 0, loc)
	require.Equal(t, time.Date(2026, 6, 19, 14, 0, 0, 0, loc), scheduler.NextHourlyAt(on, loc))
}

func TestRunHourlyReturnsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled — must return without firing the job

	fired := false
	done := make(chan struct{})
	go func() {
		scheduler.RunHourly(ctx, time.UTC,
			func() time.Time { return time.Date(2026, 6, 19, 12, 30, 0, 0, time.UTC) },
			func(context.Context) { fired = true })
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunHourly did not return on cancelled context")
	}
	if fired {
		t.Fatal("job fired despite cancelled context")
	}
}
