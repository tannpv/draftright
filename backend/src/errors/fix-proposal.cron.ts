import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { Cron, CronExpression } from '@nestjs/schedule';
import { InjectRepository } from '@nestjs/typeorm';
import { IsNull, Repository } from 'typeorm';
import { ErrorReport } from './entities/error-report.entity';
import { ErrorsService } from './errors.service';
import { EnvSchema } from '../config/env.schema';

/**
 * Hourly cron that asks the configured AI provider to propose fixes
 * for new errors that haven't been triaged yet. Converts the error
 * triage queue from "human-clicks-each-row" to "self-driving" — by the
 * time a developer opens the admin portal in the morning, the
 * common errors already have AI analyses attached.
 *
 * Throttling: only processes errors with count >= 2 (avoid one-offs)
 * and status = 0 (NEW) and ai_fix_proposal IS NULL. Caps at 10 per
 * run to keep AI provider cost bounded.
 *
 * Toggle: set DISABLE_FIX_PROPOSAL_CRON=1 in .env to skip (useful for
 * dev environments or when the AI provider is misconfigured).
 */
@Injectable()
export class FixProposalCron {
  private readonly logger = new Logger(FixProposalCron.name);

  constructor(
    @InjectRepository(ErrorReport)
    private readonly repo: Repository<ErrorReport>,
    private readonly errors: ErrorsService,
    private readonly cfg: ConfigService<EnvSchema, true>,
  ) {}

  @Cron(CronExpression.EVERY_HOUR)
  async run(): Promise<void> {
    // Standard S14 — env reads go through the typed ConfigService.
    const disabled = this.cfg.get('DISABLE_FIX_PROPOSAL_CRON', { infer: true });
    if (disabled === '1' || disabled === 'true') {
      this.logger.debug('FixProposalCron skipped (DISABLE_FIX_PROPOSAL_CRON set)');
      return;
    }

    // Pick errors that:
    // - are still NEW (status=0)
    // - haven't been analyzed yet (ai_fix_proposal IS NULL)
    // - have happened at least 2 times (signal vs noise)
    // Top-N by count so we attack the worst first.
    const candidates = await this.repo
      .createQueryBuilder('e')
      .where('e.status = :status', { status: 0 })
      .andWhere('e.ai_fix_proposal IS NULL')
      .andWhere('e.count >= :minCount', { minCount: 2 })
      .orderBy('e.count', 'DESC')
      .addOrderBy('e.last_seen_at', 'DESC')
      .limit(10)
      .getMany();

    if (candidates.length === 0) {
      this.logger.debug('FixProposalCron: no candidates this run');
      return;
    }

    this.logger.log(
      `FixProposalCron: analyzing ${candidates.length} error(s) — top counts: ${candidates.slice(0, 3).map(c => c.count).join(', ')}`,
    );

    let success = 0;
    let failed = 0;
    for (const err of candidates) {
      try {
        await this.errors.suggestFix(err.id);
        success++;
        // Tiny pause between calls so we don't spike the AI provider
        await new Promise(r => setTimeout(r, 1000));
      } catch (e) {
        failed++;
        this.logger.warn(`FixProposalCron: ${err.id} failed — ${(e as Error).message}`);
      }
    }
    this.logger.log(`FixProposalCron: ${success} succeeded, ${failed} failed`);
  }
}
