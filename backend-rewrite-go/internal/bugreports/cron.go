package bugreports

import (
	"context"
	"time"
)

// cronSvc is the consumer port the hourly bug fix-proposal cron drives: pick the
// candidate bug ids, then ask the AI provider to propose triage notes / likely
// root cause for each. Satisfied by *AdminService.
type cronSvc interface {
	AdminBugFixCandidates(ctx context.Context, limit int32) ([]string, error)
	SuggestFix(ctx context.Context, id string) (BugReportEntity, error)
}

// CronResult is the per-run tally — Node logs "X succeeded, Y failed".
type CronResult struct {
	Success int
	Failed  int
}

// Cron is the hourly fix-proposal job for bug reports. It mirrors Node's
// BugFixProposalCron (src/bug-reports/bug-fix-proposal.cron.ts): toggle-gated,
// picks the newest un-analyzed bugs (status='new', kind='bug',
// ai_fix_proposal IS NULL; LIMIT 5), and runs SuggestFix per id. A SuggestFix
// failure increments Failed and never aborts the loop. The 5-per-run cap keeps
// AI provider cost bounded — bugs are lower-volume than crash reports.
type Cron struct {
	svc      cronSvc
	disabled func() bool
	sleep    func(time.Duration)
}

// NewCron wires the cron. disabled lets main.go close over the
// DISABLE_FIX_PROPOSAL_CRON env toggle (shared with the errreport cron); sleep
// is injected (time.Sleep in prod) so tests don't wait.
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
	ids, err := c.svc.AdminBugFixCandidates(ctx, 5)
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
