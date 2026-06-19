// Package scheduler runs jobs on a daily wall-clock schedule (server-local
// time), porting NestJS @Cron("0 09 * * *"). Pure NextDailyAt is unit-tested;
// RunDaily is the thin time.Timer loop around it.
package scheduler

import (
	"context"
	"time"
)

// NextDailyAt returns the next instant strictly after `from` whose local
// clock reads hh:mm in loc. If from is already past today's hh:mm (or exactly
// on it), it rolls to tomorrow.
func NextDailyAt(from time.Time, hh, mm int, loc *time.Location) time.Time {
	f := from.In(loc)
	next := time.Date(f.Year(), f.Month(), f.Day(), hh, mm, 0, 0, loc)
	if !next.After(f) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// RunDaily blocks, firing job(ctx, fireTime) once per day at hh:mm local until
// ctx is cancelled. `now` is injected for testability (prod passes time.Now).
// The first fire is at the next hh:mm strictly after now(). A panic in job is
// the caller's concern — wrap if needed.
func RunDaily(ctx context.Context, hh, mm int, loc *time.Location, now func() time.Time, job func(context.Context, time.Time)) {
	for {
		next := NextDailyAt(now(), hh, mm, loc)
		timer := time.NewTimer(next.Sub(now()))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case t := <-timer.C:
			job(ctx, t)
		}
	}
}
