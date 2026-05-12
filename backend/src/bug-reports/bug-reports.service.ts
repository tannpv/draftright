import { Injectable, BadRequestException, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { randomUUID } from 'crypto';
import { promises as fs } from 'fs';
import * as path from 'path';
import { BugReport } from './entities/bug-report.entity';
import { CreateBugReportDto, UpdateBugReportDto } from './dto/create-bug-report.dto';
import { FeatureVote } from './entities/feature-vote.entity';
import { CreateFeedbackDto, TARGET_PLATFORMS } from './dto/create-feedback.dto';
import { applyListQuery, ListQuery } from '../common/list-query';
import { AiProvidersService } from '../ai-providers/ai-providers.service';

/**
 * Root directory for bug-report screenshots on disk.
 * In production this is bind-mounted into the backend container at the
 * same path. Override via `BUG_REPORTS_DIR` for local dev or tests.
 */
const STORAGE_ROOT =
  process.env.BUG_REPORTS_DIR || '/var/lib/draftright/bug-reports';

const ALLOWED_STATUSES = ['new', 'reviewing', 'fix_proposed', 'resolved', 'wont_fix'];

interface UploadedScreenshot {
  buffer: Buffer;
  originalname: string;
  mimetype: string;
  size: number;
}

@Injectable()
export class BugReportsService {
  constructor(
    @InjectRepository(BugReport)
    private readonly repo: Repository<BugReport>,
    @InjectRepository(FeatureVote)
    private readonly votes: Repository<FeatureVote>,
    private readonly aiProviders: AiProvidersService,
  ) {}

  /**
   * Create a bug report row. If `file` is provided it's written to
   * `STORAGE_ROOT/YYYY-MM-DD/<uuid>.<ext>` and the path/filename are
   * stamped on the row.
   */
  async create(
    dto: CreateBugReportDto,
    file: UploadedScreenshot | undefined,
    userId: string | null,
  ): Promise<BugReport> {
    if (!dto.description || dto.description.trim().length === 0) {
      throw new BadRequestException('description is required');
    }
    if (!dto.source || dto.source.trim().length === 0) {
      throw new BadRequestException('source is required');
    }

    let screenshotPath: string | null = null;
    let screenshotFilename: string | null = null;

    if (file) {
      const ext = this.extensionFor(file.mimetype);
      const id = randomUUID();
      const today = new Date().toISOString().slice(0, 10); // YYYY-MM-DD
      const dir = path.join(STORAGE_ROOT, today);
      await fs.mkdir(dir, { recursive: true });
      const filename = `${id}${ext}`;
      const fullPath = path.join(dir, filename);
      await fs.writeFile(fullPath, file.buffer);
      screenshotPath = fullPath;
      screenshotFilename = file.originalname || filename;
    }

    let parsedContext: Record<string, any> | null = null;
    if (dto.context) {
      try {
        parsedContext = JSON.parse(dto.context);
      } catch {
        // If context isn't valid JSON, store it as a single-key blob so
        // we don't throw away the data.
        parsedContext = { raw: dto.context };
      }
    }

    const row = this.repo.create({
      source: dto.source.trim().slice(0, 50),
      description: dto.description.trim(),
      screenshot_path: screenshotPath,
      screenshot_filename: screenshotFilename,
      app_version: dto.app_version ? dto.app_version.slice(0, 50) : null,
      os_info: dto.os_info ? dto.os_info.slice(0, 100) : null,
      user_id: userId,
      user_email: dto.user_email ? dto.user_email.slice(0, 255) : null,
      context: parsedContext,
      status: 'new',
    });
    return this.repo.save(row);
  }

  /**
   * Create a feedback row from `POST /feedback`. For kind='feature',
   * `title` (3-80 chars) and `target_platform` (∈ TARGET_PLATFORMS) are
   * required and stamped; for kind='bug' they're forced null (the legacy
   * `POST /bug-reports` route handles screenshots; this route does not).
   */
  async createFeedback(dto: CreateFeedbackDto, userId: string | null): Promise<BugReport> {
    if (!dto.description || dto.description.trim().length === 0) {
      throw new BadRequestException('description is required');
    }
    if (!dto.source || dto.source.trim().length === 0) {
      throw new BadRequestException('source is required');
    }
    const kind = dto.kind === 'feature' ? 'feature' : 'bug';

    let title: string | null = null;
    let targetPlatform: string | null = null;
    if (kind === 'feature') {
      const t = (dto.title ?? '').trim();
      if (t.length < 1 || t.length > 80) {
        throw new BadRequestException('title is required for a feature request (1-80 characters)');
      }
      if (!dto.target_platform || !TARGET_PLATFORMS.includes(dto.target_platform as any)) {
        throw new BadRequestException(
          `target_platform must be one of: ${TARGET_PLATFORMS.join(', ')}`,
        );
      }
      title = t;
      targetPlatform = dto.target_platform;
    }

    const row = this.repo.create({
      kind,
      title,
      target_platform: targetPlatform,
      vote_count: 0,
      is_public: true,
      source: dto.source.trim().slice(0, 50),
      description: dto.description.trim(),
      screenshot_path: null,
      screenshot_filename: null,
      app_version: dto.app_version ? dto.app_version.slice(0, 50) : null,
      os_info: dto.os_info ? dto.os_info.slice(0, 100) : null,
      user_id: userId,
      user_email: dto.user_email ? dto.user_email.slice(0, 255) : null,
      context: null,
      status: 'new',
    });
    return this.repo.save(row);
  }

  /** Loads a feature row (kind='feature') or throws NotFound. */
  private async findFeatureById(id: string): Promise<BugReport> {
    const row = await this.repo.findOne({ where: { id } });
    if (!row || row.kind !== 'feature') throw new NotFoundException('feature request not found');
    return row;
  }

  /**
   * Toggle the caller's upvote on a feature, then recompute and persist
   * `vote_count = COUNT(feature_votes for this feature)`. Idempotent.
   */
  async toggleVote(featureId: string, userId: string): Promise<{ vote_count: number; hasVoted: boolean }> {
    const row = await this.findFeatureById(featureId);
    const existing = await this.votes.findOne({ where: { feature_id: featureId, user_id: userId } });
    let hasVoted: boolean;
    if (existing) {
      await this.votes.delete({ feature_id: featureId, user_id: userId });
      hasVoted = false;
    } else {
      await this.votes.save(this.votes.create({ feature_id: featureId, user_id: userId }));
      hasVoted = true;
    }
    const count = await this.votes.count({ where: { feature_id: featureId } });
    row.vote_count = count;
    await this.repo.save(row);
    return { vote_count: count, hasVoted };
  }

  /**
   * Public board feed: kind='feature' AND is_public, sorted by
   * vote_count DESC then created_at DESC, optional status / target_platform
   * filters, paginated. Each row carries `viewerHasVoted` (always false
   * when userId is null).
   */
  async listPublicFeatures(
    query: { page?: number | string; limit?: number | string; status?: string; target_platform?: string },
    userId: string | null,
  ): Promise<{ rows: Array<BugReport & { viewerHasVoted: boolean }>; total: number }> {
    const where: Record<string, any> = { kind: 'feature', is_public: true };
    if (query.status && query.status !== 'all' && ALLOWED_STATUSES.includes(query.status)) {
      where.status = query.status;
    }
    if (query.target_platform && TARGET_PLATFORMS.includes(query.target_platform as any)) {
      where.target_platform = query.target_platform;
    }
    const page = Math.max(1, Number(query.page) || 1);
    const limit = Math.min(100, Math.max(1, Number(query.limit) || 20));

    const total = await this.repo.count({ where });
    const rows = await this.repo.find({
      where,
      order: { vote_count: 'DESC', created_at: 'DESC' },
      skip: (page - 1) * limit,
      take: limit,
    });

    let votedIds = new Set<string>();
    if (userId && rows.length > 0) {
      const myVotes = await this.votes.find({ where: { user_id: userId } });
      votedIds = new Set(myVotes.map(v => v.feature_id));
    }
    return {
      rows: rows.map(r => Object.assign(r, { viewerHasVoted: votedIds.has(r.id) })),
      total,
    };
  }

  /** Admin: paginated list with search/sort/status/kind/target_platform filter. */
  async findAllPaginated(
    query: ListQuery & { status?: string; kind?: string; target_platform?: string },
  ) {
    const qb = this.repo.createQueryBuilder('br');

    // Status filter is custom (not is_active) — apply it before
    // applyListQuery so the helper's status filter doesn't fight us.
    if (query.status && query.status !== 'all' &&
        ALLOWED_STATUSES.includes(query.status)) {
      qb.andWhere('br.status = :st', { st: query.status });
    }
    if (query.kind === 'bug' || query.kind === 'feature') {
      qb.andWhere('br.kind = :k', { k: query.kind });
    }
    if (query.target_platform && TARGET_PLATFORMS.includes(query.target_platform as any)) {
      qb.andWhere('br.target_platform = :tp', { tp: query.target_platform });
    }

    // Build a clean ListQuery — omit the custom fields so applyListQuery
    // doesn't try to apply them as is_active.
    const listQ: ListQuery = {
      search: query.search,
      sort_by: query.sort_by,
      sort_order: query.sort_order,
      page: query.page,
      limit: query.limit,
    };
    const result = await applyListQuery(
      qb,
      listQ,
      ['br.description', 'br.title', 'br.user_email', 'br.source'],
      {
        created_at: 'br.created_at',
        updated_at: 'br.updated_at',
        status: 'br.status',
        source: 'br.source',
        kind: 'br.kind',
        vote_count: 'br.vote_count',
      },
      'br.created_at',
      null, // no is_active column on this entity
    );
    return result;
  }

  async findById(id: string): Promise<BugReport> {
    const row = await this.repo.findOne({ where: { id } });
    if (!row) throw new NotFoundException('bug report not found');
    return row;
  }

  async update(id: string, dto: UpdateBugReportDto): Promise<BugReport> {
    const row = await this.findById(id);
    if (dto.status !== undefined) {
      if (!ALLOWED_STATUSES.includes(dto.status)) {
        throw new BadRequestException(
          `status must be one of: ${ALLOWED_STATUSES.join(', ')}`,
        );
      }
      row.status = dto.status;
    }
    if (dto.admin_notes !== undefined) row.admin_notes = dto.admin_notes;
    if (dto.title !== undefined) {
      const t = (dto.title ?? '').trim();
      if (t.length < 1 || t.length > 80) {
        throw new BadRequestException('title is required (1-80 characters)');
      }
      row.title = t;
    }
    if (dto.target_platform !== undefined) {
      if (!TARGET_PLATFORMS.includes(dto.target_platform as any)) {
        throw new BadRequestException(
          `target_platform must be one of: ${TARGET_PLATFORMS.join(', ')}`,
        );
      }
      row.target_platform = dto.target_platform;
    }
    if (dto.is_public !== undefined) row.is_public = dto.is_public;
    return this.repo.save(row);
  }

  /** Delete row + screenshot file (best-effort on the file). */
  async delete(id: string): Promise<void> {
    const row = await this.findById(id);
    if (row.screenshot_path) {
      try {
        await fs.unlink(row.screenshot_path);
      } catch {
        // Best-effort — file might already be missing on disk.
      }
    }
    await this.repo.delete(id);
  }

  /** Returns the absolute on-disk path for the screenshot, or null. */
  async getScreenshotPath(id: string): Promise<{ path: string; filename: string } | null> {
    const row = await this.findById(id);
    if (!row.screenshot_path) return null;
    return {
      path: row.screenshot_path,
      filename: row.screenshot_filename || path.basename(row.screenshot_path),
    };
  }

  /**
   * Asks the configured AI provider to read the bug description, screenshot
   * filename, and platform context, then propose a fix. Stores the response
   * on the row and bumps status to 'fix_proposed'. Same UX as
   * ErrorsService.suggestFix but with a bug-report-flavored prompt
   * (free-text description, no stack trace, possibly a screenshot reference).
   */
  async suggestFix(id: string): Promise<BugReport> {
    const row = await this.findById(id);

    const provider = await this.aiProviders.findDefault();

    const systemPrompt = `You are a senior software engineer reviewing a user-submitted bug report from a production app. The user described what's wrong in their own words; you have NO stack trace and possibly no reproduction steps. Propose a triage plan.

Output format (concise; this is shown to a developer in a triage queue):

LIKELY ROOT CAUSE
<one or two sentences — your best guess based on the description, platform, and version. Cite parts of the description that support it.>

LIKELY AREA OF CODE
<which module, screen, or feature this most likely lives in. "unknown" if too vague.>

PROPOSED FIX OR INVESTIGATION
<2-4 bullet points — either a concrete code change OR specific things the developer should check / questions to ask the user.>

CONFIDENCE
<low | medium | high>

REPRODUCTION QUESTIONS
<2-3 short questions to ask the user IF the report is too vague to act on. Skip this section if confidence is medium or high.>

Do not output anything outside this format. Do not pad. Do not apologize.`;

    const userText = [
      `Source: ${row.source}`,
      `Platform / OS: ${row.os_info || 'unknown'}`,
      `App version: ${row.app_version || 'unknown'}`,
      `Submitted: ${row.created_at?.toISOString?.() ?? 'unknown'}`,
      `User email: ${row.user_email || '(anonymous)'}`,
      `Has screenshot: ${row.screenshot_path ? 'yes' : 'no'}`,
      ``,
      `User's description:`,
      row.description,
      ``,
      `Context: ${JSON.stringify(row.context || {}, null, 2)}`,
    ].join('\n').slice(0, 8000);

    const result = await this.aiProviders.callProvider(
      provider,
      systemPrompt,
      userText,
    );

    row.ai_fix_proposal = result.text;
    row.ai_fix_proposed_at = new Date();
    if (row.status === 'new') row.status = 'fix_proposed';
    return this.repo.save(row);
  }

  /** Cron uses this to find candidates needing AI analysis. */
  async findFixProposalCandidates(limit: number = 5): Promise<BugReport[]> {
    return this.repo
      .createQueryBuilder('br')
      .where('br.status = :st', { st: 'new' })
      .andWhere('br.ai_fix_proposal IS NULL')
      .orderBy('br.created_at', 'DESC')
      .limit(limit)
      .getMany();
  }

  private extensionFor(mimetype: string): string {
    if (mimetype === 'image/png') return '.png';
    if (mimetype === 'image/jpeg' || mimetype === 'image/jpg') return '.jpg';
    throw new BadRequestException('only PNG or JPEG screenshots are accepted');
  }
}
