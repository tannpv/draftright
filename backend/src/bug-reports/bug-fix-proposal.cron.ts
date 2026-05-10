import { Injectable, Logger } from '@nestjs/common';
import { Cron, CronExpression } from '@nestjs/schedule';
import { BugReportsService } from './bug-reports.service';

/**
 * Hourly cron that asks the configured AI provider to propose triage
 * notes / likely root cause for new bug reports that haven't been
 * analyzed yet. Mirrors the FixProposalCron used for error_reports.
 *
 * Throttling: only processes status='new' + ai_fix_proposal IS NULL.
 * Caps at 5 per run to keep AI provider cost bounded — bugs are
 * usually lower-volume than crash reports, so a smaller cap is fine.
 *
 * Toggle: set DISABLE_FIX_PROPOSAL_CRON=1 in .env to skip both this
 * and the error-report cron (shared toggle, intentional — when
 * disabling AI triage you usually want both off).
 */
@Injectable()
export class BugFixProposalCron {
  private readonly logger = new Logger(BugFixProposalCron.name);

  constructor(private readonly bugs: BugReportsService) {}

  @Cron(CronExpression.EVERY_HOUR)
  async run(): Promise<void> {
    if (process.env.DISABLE_FIX_PROPOSAL_CRON === '1') {
      this.logger.debug('BugFixProposalCron skipped (DISABLE_FIX_PROPOSAL_CRON=1)');
      return;
    }

    const candidates = await this.bugs.findFixProposalCandidates(5);
    if (candidates.length === 0) {
      this.logger.debug('BugFixProposalCron: no candidates this run');
      return;
    }

    this.logger.log(`BugFixProposalCron: analyzing ${candidates.length} bug(s)`);

    let success = 0;
    let failed = 0;
    for (const bug of candidates) {
      try {
        await this.bugs.suggestFix(bug.id);
        success++;
        await new Promise(r => setTimeout(r, 1000));
      } catch (e) {
        failed++;
        this.logger.warn(`BugFixProposalCron: ${bug.id} failed — ${(e as Error).message}`);
      }
    }
    this.logger.log(`BugFixProposalCron: ${success} succeeded, ${failed} failed`);
  }
}
