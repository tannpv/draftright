package scheduler

import (
	"context"
	"time"
)

// NextHourlyAt returns the next top-of-hour strictly after `from` in loc.
func NextHourlyAt(from time.Time, loc *time.Location) time.Time {
	f := from.In(loc)
	next := time.Date(f.Year(), f.Month(), f.Day(), f.Hour(), 0, 0, 0, loc)
	if !next.After(f) {
		next = next.Add(time.Hour)
	}
	return next
}

// RunHourly blocks, firing job(ctx) at every top-of-hour local until ctx is
// cancelled. `now` is injected for testability (prod passes time.Now).
func RunHourly(ctx context.Context, loc *time.Location, now func() time.Time, job func(context.Context)) {
	for {
		next := NextHourlyAt(now(), loc)
		timer := time.NewTimer(next.Sub(now()))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			job(ctx)
		}
	}
}
