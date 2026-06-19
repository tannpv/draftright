package errreport

import (
	"context"
	"time"
)

// cronSvc is the consumer port the hourly fix-proposal cron drives: pick the
// candidate error ids, then ask the AI provider to propose a fix for each.
// Satisfied by *AdminService.
type cronSvc interface {
	AdminFixCandidates(ctx context.Context, limit int32) ([]string, error)
	SuggestFix(ctx context.Context, id string) (ErrorReportEntity, error)
}

// CronResult is the per-run tally — Node logs "X succeeded, Y failed".
type CronResult struct {
	Success int
	Failed  int
}

// Cron is the hourly fix-proposal job for errors. It mirrors Node's
// FixProposalCron (src/errors/fix-proposal.cron.ts): toggle-gated, picks the
// top error groups (status=0, ai_fix_proposal IS NULL, count >= 2; LIMIT 10),
// and runs SuggestFix per id. A SuggestFix failure increments Failed and never
// aborts the loop.
type Cron struct {
	svc      cronSvc
	disabled func() bool
	sleep    func(time.Duration)
}

// NewCron wires the cron. disabled lets main.go close over the
// DISABLE_FIX_PROPOSAL_CRON env toggle; sleep is injected (time.Sleep in prod)
// so tests don't wait.
func NewCron(svc cronSvc, disabled func() bool, sleep func(time.Duration)) *Cron {
	return &Cron{svc: svc, disabled: disabled, sleep: sleep}
}

// RunOnce executes a single cron pass. Returns the success/failed tally for the
// caller to log. Mirrors the Node control flow exactly: the 1000ms pause sits
// inside the try block AFTER success++, so a failing SuggestFix produces NO
// sleep.
func (c *Cron) RunOnce(ctx context.Context) CronResult {
	if c.disabled() {
		return CronResult{}
	}
	ids, err := c.svc.AdminFixCandidates(ctx, 10)
	if err != nil || len(ids) == 0 {
		return CronResult{}
	}
	var res CronResult
	for _, id := range ids {
		if _, err := c.svc.SuggestFix(ctx, id); err != nil {
			res.Failed++
			continue
		}
		res.Success++
		// Tiny pause between calls so we don't spike the AI provider.
		// Node: await sleep(1000) is inside try after success++ → only on success.
		c.sleep(1000 * time.Millisecond)
	}
	return res
}
