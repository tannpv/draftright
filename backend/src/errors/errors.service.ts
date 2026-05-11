import { Injectable, BadRequestException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { createHash } from 'crypto';
import { ErrorReport } from './entities/error-report.entity';
import { CreateErrorReportDto } from './dto/create-error-report.dto';
import { AiProvidersService } from '../ai-providers/ai-providers.service';

const ALLOWED_PLATFORMS = ['ios', 'android', 'macos', 'windows', 'linux', 'web'];
const ALLOWED_SEVERITY = ['fatal', 'error', 'warning', 'info'];

@Injectable()
export class ErrorsService {
  constructor(
    @InjectRepository(ErrorReport)
    private readonly repo: Repository<ErrorReport>,
    private readonly aiProviders: AiProvidersService,
  ) {}

  /**
   * Ingest an error report. If a row with the same fingerprint already
   * exists, increment count + bump last_seen_at instead of creating
   * a new row (deduplication).
   */
  async ingest(input: CreateErrorReportDto, userId?: string | null): Promise<ErrorReport> {
    if (!ALLOWED_PLATFORMS.includes(input.platform)) {
      throw new BadRequestException(`platform must be one of: ${ALLOWED_PLATFORMS.join(', ')}`);
    }
    const severity = ALLOWED_SEVERITY.includes(input.severity || '') ? input.severity! : 'error';

    // Truncate large fields to keep DB sane
    const message = (input.message || '').slice(0, 5000);
    const stackTrace = (input.stack_trace || '').slice(0, 20000);
    const errorType = (input.error_type || '').slice(0, 200);

    // Scrub obvious secrets from message + stack
    const scrub = (s: string) => s
      .replace(/Bearer\s+[a-zA-Z0-9._\-]+/g, 'Bearer [REDACTED]')
      .replace(/password["':\s=]+["']?[^"'\s,}]+/gi, 'password=[REDACTED]')
      .replace(/[\w._%+-]+@[\w.-]+\.[a-zA-Z]{2,}/g, '[email]');

    const fingerprint = this.fingerprint(errorType, stackTrace);

    // Try to update existing
    const existing = await this.repo.findOne({ where: { fingerprint } });
    if (existing) {
      existing.count = (existing.count || 0) + 1;
      existing.last_seen_at = new Date();
      // Refresh latest-known metadata so admin sees the most recent occurrence
      if (input.app_version) existing.app_version = input.app_version;
      if (userId) existing.user_id = userId;
      if (input.device_id) existing.device_id = input.device_id.slice(0, 100);
      if (input.context) existing.context = input.context;
      return this.repo.save(existing);
    }

    const row = this.repo.create({
      platform: input.platform,
      app_version: input.app_version || null,
      severity,
      error_type: errorType || null,
      message: scrub(message) || null,
      stack_trace: scrub(stackTrace) || null,
      context: input.context || null,
      user_id: userId || null,
      device_id: input.device_id ? input.device_id.slice(0, 100) : null,
      fingerprint,
      count: 1,
      status: 0,
    });
    return this.repo.save(row);
  }

  /**
   * sha256 of (error_type + first 3 stack frames). Stable across runs;
   * different stack lines on same crash still match.
   */
  private fingerprint(errorType: string, stackTrace: string): string {
    const firstFrames = (stackTrace || '')
      .split('\n')
      .slice(0, 3)
      .map(l => l.trim())
      .filter(Boolean)
      .join('|');
    const seed = `${errorType || ''}::${firstFrames}`;
    return createHash('sha256').update(seed).digest('hex');
  }

  /** Admin: paginated list with filters */
  async list(opts: {
    platform?: string;
    status?: number;
    severity?: string;
    limit?: number;
    offset?: number;
  }) {
    const qb = this.repo.createQueryBuilder('e');
    if (opts.platform) qb.andWhere('e.platform = :p', { p: opts.platform });
    if (opts.status !== undefined) qb.andWhere('e.status = :s', { s: opts.status });
    if (opts.severity) qb.andWhere('e.severity = :sv', { sv: opts.severity });
    qb.orderBy('e.last_seen_at', 'DESC')
      .limit(Math.min(opts.limit || 50, 200))
      .offset(opts.offset || 0);
    const [items, total] = await qb.getManyAndCount();
    return { items, total };
  }

  async getOne(id: string): Promise<ErrorReport | null> {
    return this.repo.findOne({ where: { id } });
  }

  async setStatus(id: string, status: number, resolvedBy?: string): Promise<ErrorReport> {
    const row = await this.repo.findOne({ where: { id } });
    if (!row) throw new BadRequestException('not found');
    row.status = status;
    if (status === 4 || status === 5) {
      row.resolved_at = new Date();
      row.resolved_by = resolvedBy || 'admin';
    }
    return this.repo.save(row);
  }

  /**
   * Phase 2: ask the configured AI provider to analyze the error and
   * propose a fix. Stores the response on the error row and sets
   * status=FIX_PROPOSED (3). Admin reviews and either applies the fix
   * (then sets status=4 RESOLVED) or rejects.
   */
  async suggestFix(id: string): Promise<ErrorReport> {
    const row = await this.repo.findOne({ where: { id } });
    if (!row) throw new BadRequestException('not found');

    const provider = await this.aiProviders.findDefault();

    const systemPrompt = `You are a senior software engineer reviewing a production crash report. Analyze the error and propose a concrete fix.

Output format (concise; this is shown to a developer in a triage queue):

ROOT CAUSE
<one sentence — what went wrong, in plain language>

LIKELY FILE / FRAME
<the most likely file:line from the stack trace, or "unknown" if the stack is opaque>

PROPOSED FIX
<2-4 bullet points — what to change. Code snippets if obvious. No fluff.>

CONFIDENCE
<low | medium | high>

If the stack is genuinely insufficient to propose a fix, say so plainly and suggest what additional information would help (e.g., "need source for X", "need a reproduction").

Do not output anything outside this format. Do not pad. Do not apologize.`;

    const userText = [
      `Platform: ${row.platform}`,
      `App version: ${row.app_version || 'unknown'}`,
      `Severity: ${row.severity}`,
      `Occurrences: ${row.count}`,
      `First seen: ${row.first_seen_at?.toISOString?.() ?? 'unknown'}`,
      `Last seen: ${row.last_seen_at?.toISOString?.() ?? 'unknown'}`,
      ``,
      `Error type: ${row.error_type || '(none)'}`,
      `Message: ${row.message || '(none)'}`,
      ``,
      `Stack trace:`,
      row.stack_trace || '(empty)',
      ``,
      `Context: ${JSON.stringify(row.context || {}, null, 2)}`,
    ].join('\n').slice(0, 8000); // bound input to keep latency + cost reasonable

    const result = await this.aiProviders.callProvider(
      provider,
      systemPrompt,
      userText,
    );

    row.ai_fix_proposal = result.text;
    row.status = 3; // FIX_PROPOSED
    return this.repo.save(row);
  }
}
