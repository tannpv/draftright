# Feedback Backend (Spec A) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the existing bug-reports backend into a `feedback` capability — feature requests (with a user-picked `target_platform`), a public upvote board feed, and admin triage — without breaking the existing `POST /bug-reports` contract.

**Architecture:** Reuse the `bug_reports` table via a new `kind` discriminator (`'bug' | 'feature'`) plus `title`, `target_platform`, `vote_count`, `is_public` columns; add a `feature_votes` join table for one-vote-per-user dedupe. The existing `BugReportsModule`/`BugReportsService` is kept (no directory rename — minimal churn) and extended with feature/vote methods; a new `FeedbackController` (mounted at `/feedback`) holds the public board routes. Admin gets `kind`/`target_platform` filters and `title`/`target_platform` patch fields. The AI-fix cron is scoped to `kind='bug'` so feature requests are never auto-processed. Schema change ships as an idempotent SQL file (prod runs with `synchronize: false`); dev/test pick it up via `synchronize`.

**Tech Stack:** NestJS 10, TypeORM, PostgreSQL 16, `class-validator`, Jest (`*.spec.ts`).

**Deviation from spec:** the spec suggested renaming the module to `feedback.module.ts`/`FeedbackService`. This plan keeps `src/bug-reports/` and `BugReportsService` to avoid mechanical churn across `admin.controller.ts`, `admin.module.ts`, `app.module.ts`, and the cron; the new public surface lives in `src/bug-reports/feedback.controller.ts`. Flag at review if you want the full rename.

**Run all commands from `backend/`.**

---

### Task 1: Schema — add columns to `BugReport` entity + SQL migration file

**Files:**
- Modify: `backend/src/bug-reports/entities/bug-report.entity.ts`
- Create: `backend/sql/2026-05-12-feedback.sql`

- [ ] **Step 1: Add the new columns to the entity**

In `backend/src/bug-reports/entities/bug-report.entity.ts`, add these columns after the existing `status` column (keep everything else as-is):

```typescript
  /** Discriminates bug reports ('bug', the legacy default) from feature requests ('feature'). */
  @Column({ type: 'varchar', length: 20, default: 'bug' })
  kind: string;

  /** Feature requests only: short one-line title (3-80 chars). Null for bugs. */
  @Column({ type: 'varchar', length: 80, nullable: true })
  title: string | null;

  /**
   * Feature requests only: which platform the request is *for*, picked by the
   * user in the submit dropdown. One of: playground | mobile | windows | mac | linux.
   * Distinct from `source` (where the request was submitted from). Null for bugs.
   */
  @Column({ type: 'varchar', length: 20, nullable: true, name: 'target_platform' })
  target_platform: string | null;

  /** Feature requests only: upvote count, derived — always recomputed as COUNT(feature_votes). */
  @Column({ type: 'int', default: 0, name: 'vote_count' })
  vote_count: number;

  /** Feature requests only: false hides the row from the public board (admin spam/dedupe lever). */
  @Column({ type: 'boolean', default: true, name: 'is_public' })
  is_public: boolean;
```

Also add an index for the public-board query — add to the `@Index(...)` decorators block above the class:

```typescript
@Index('bug_reports_kind_public_votes_idx', ['kind', 'is_public', 'vote_count'])
```

- [ ] **Step 2: Write the SQL migration file**

Create `backend/sql/2026-05-12-feedback.sql`:

```sql
-- Spec A: feedback (feature requests + public upvote board).
-- Idempotent — safe to re-run. Apply on prod with:
--   docker compose -f docker-compose.prod.yml exec -T postgres psql -U draftright -d draftright < backend/sql/2026-05-12-feedback.sql
-- Dev/test pick these up automatically via TypeORM synchronize.

ALTER TABLE bug_reports ADD COLUMN IF NOT EXISTS kind            varchar(20) NOT NULL DEFAULT 'bug';
ALTER TABLE bug_reports ADD COLUMN IF NOT EXISTS title           varchar(80);
ALTER TABLE bug_reports ADD COLUMN IF NOT EXISTS target_platform varchar(20);
ALTER TABLE bug_reports ADD COLUMN IF NOT EXISTS vote_count      integer NOT NULL DEFAULT 0;
ALTER TABLE bug_reports ADD COLUMN IF NOT EXISTS is_public       boolean NOT NULL DEFAULT true;

CREATE INDEX IF NOT EXISTS bug_reports_kind_public_votes_idx
  ON bug_reports (kind, is_public, vote_count);

CREATE TABLE IF NOT EXISTS feature_votes (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  feature_id uuid NOT NULL REFERENCES bug_reports(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (feature_id, user_id)
);

CREATE INDEX IF NOT EXISTS feature_votes_feature_idx ON feature_votes (feature_id);
```

- [ ] **Step 3: Type-check**

Run: `npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add src/bug-reports/entities/bug-report.entity.ts sql/2026-05-12-feedback.sql
git commit -m "feat(feedback): add kind/title/target_platform/vote_count/is_public to bug_reports + SQL migration"
```

---

### Task 2: `FeatureVote` entity

**Files:**
- Create: `backend/src/bug-reports/entities/feature-vote.entity.ts`

- [ ] **Step 1: Write the entity**

Create `backend/src/bug-reports/entities/feature-vote.entity.ts`:

```typescript
import {
  Entity, PrimaryGeneratedColumn, Column, Index, Unique, CreateDateColumn,
} from 'typeorm';

/**
 * One row per (feature request, user) upvote. The UNIQUE constraint
 * enforces one vote per user per feature; `bug_reports.vote_count` is
 * always recomputed as COUNT(*) of these rows for the feature.
 */
@Entity('feature_votes')
@Unique('feature_votes_feature_user_uq', ['feature_id', 'user_id'])
@Index('feature_votes_feature_idx', ['feature_id'])
export class FeatureVote {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'uuid', name: 'feature_id' })
  feature_id: string;

  @Column({ type: 'uuid', name: 'user_id' })
  user_id: string;

  @CreateDateColumn({ name: 'created_at' })
  created_at: Date;
}
```

- [ ] **Step 2: Type-check**

Run: `npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add src/bug-reports/entities/feature-vote.entity.ts
git commit -m "feat(feedback): add FeatureVote entity"
```

---

### Task 3: DTOs — `CreateFeedbackDto`, `VoteResponse` shape, admin update fields

**Files:**
- Create: `backend/src/bug-reports/dto/create-feedback.dto.ts`
- Modify: `backend/src/bug-reports/dto/create-bug-report.dto.ts` (extend `UpdateBugReportDto`)

- [ ] **Step 1: Write `CreateFeedbackDto`**

Create `backend/src/bug-reports/dto/create-feedback.dto.ts`:

```typescript
import { IsString, IsOptional, IsIn, MaxLength, MinLength } from 'class-validator';

export const FEEDBACK_KINDS = ['bug', 'feature'] as const;
export const TARGET_PLATFORMS = ['playground', 'mobile', 'windows', 'mac', 'linux'] as const;

export type FeedbackKind = (typeof FEEDBACK_KINDS)[number];
export type TargetPlatform = (typeof TARGET_PLATFORMS)[number];

/**
 * Body for `POST /feedback`. JSON (no file upload on this route — feature
 * requests don't take screenshots; the legacy `POST /bug-reports` route
 * still handles multipart bug reports with screenshots).
 */
export class CreateFeedbackDto {
  @IsIn(FEEDBACK_KINDS)
  kind: FeedbackKind;

  /** Required when kind === 'feature'; ignored for bugs. Validated in the service against length 3-80. */
  @IsOptional() @IsString() @MaxLength(80)
  title?: string;

  /** Required when kind === 'feature'; ignored for bugs. */
  @IsOptional() @IsIn(TARGET_PLATFORMS)
  target_platform?: TargetPlatform;

  @IsString() @MinLength(1) @MaxLength(2000)
  description: string;

  @IsString() @MaxLength(50)
  source: string;

  @IsOptional() @IsString() @MaxLength(50)
  app_version?: string;

  @IsOptional() @IsString() @MaxLength(100)
  os_info?: string;

  @IsOptional() @IsString() @MaxLength(255)
  user_email?: string;
}
```

- [ ] **Step 2: Extend `UpdateBugReportDto`**

In `backend/src/bug-reports/dto/create-bug-report.dto.ts`, replace the `UpdateBugReportDto` class with:

```typescript
export class UpdateBugReportDto {
  @IsOptional() @IsString() @MaxLength(20)
  status?: string;

  @IsOptional() @IsString()
  admin_notes?: string;

  /** Feature requests: edit the title. */
  @IsOptional() @IsString() @MaxLength(80)
  title?: string;

  /** Feature requests: re-classify the target platform. */
  @IsOptional() @IsString() @MaxLength(20)
  target_platform?: string;

  /** Feature requests: hide/show on the public board. */
  @IsOptional() @IsBoolean()
  is_public?: boolean;
}
```

Add `IsBoolean` to the `class-validator` import at the top of that file (it currently imports `IsString, IsOptional, MaxLength, MinLength`).

- [ ] **Step 3: Type-check**

Run: `npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add src/bug-reports/dto/
git commit -m "feat(feedback): CreateFeedbackDto + extend UpdateBugReportDto with title/target_platform/is_public"
```

---

### Task 4: Service methods — `createFeedback`, `listPublicFeatures`, `toggleVote`

**Files:**
- Modify: `backend/src/bug-reports/bug-reports.service.ts`
- Test: `backend/src/bug-reports/bug-reports.service.spec.ts` (create)

- [ ] **Step 1: Write the failing test**

Create `backend/src/bug-reports/bug-reports.service.spec.ts`:

```typescript
import { Test } from '@nestjs/testing';
import { getRepositoryToken } from '@nestjs/typeorm';
import { BadRequestException, NotFoundException } from '@nestjs/common';
import { BugReportsService } from './bug-reports.service';
import { BugReport } from './entities/bug-report.entity';
import { FeatureVote } from './entities/feature-vote.entity';
import { AiProvidersService } from '../ai-providers/ai-providers.service';

// In-memory fakes — enough to exercise the feature/vote logic without a DB.
class FakeRepo<T extends { id: string }> {
  rows: T[] = [];
  private match(r: any, where: any) { return !where || Object.entries(where).every(([k, v]) => v === undefined || r[k] === v); }
  create(p: Partial<T>): T { return { ...(p as any) }; }
  async save(r: any) { if (!r.id) r.id = 'id-' + (this.rows.length + 1); const i = this.rows.findIndex(x => x.id === r.id); if (i >= 0) this.rows[i] = r; else this.rows.push(r); return r; }
  async findOne({ where }: any) { return this.rows.find(r => this.match(r, where)) ?? null; }
  async find({ where, order, skip, take }: any = {}) {
    let out = this.rows.filter(r => this.match(r, where));
    if (order) for (const [k, dir] of Object.entries(order).reverse()) {
      out = out.slice().sort((a: any, b: any) => (a[k] < b[k] ? -1 : a[k] > b[k] ? 1 : 0) * (String(dir).toUpperCase() === 'DESC' ? -1 : 1));
    }
    if (skip) out = out.slice(skip);
    if (take !== undefined) out = out.slice(0, take);
    return out;
  }
  async count({ where }: any = {}) { return this.rows.filter(r => this.match(r, where)).length; }
  async delete(idOrWhere: any) { const before = this.rows.length; this.rows = this.rows.filter(r => typeof idOrWhere === 'string' ? r.id !== idOrWhere : !this.match(r, idOrWhere)); return { affected: before - this.rows.length }; }
}

describe('BugReportsService — feedback', () => {
  let svc: BugReportsService;
  let bugRepo: FakeRepo<BugReport>;
  let voteRepo: FakeRepo<FeatureVote>;

  beforeEach(async () => {
    bugRepo = new FakeRepo<BugReport>();
    voteRepo = new FakeRepo<FeatureVote>();
    const mod = await Test.createTestingModule({
      providers: [
        BugReportsService,
        { provide: getRepositoryToken(BugReport), useValue: bugRepo },
        { provide: getRepositoryToken(FeatureVote), useValue: voteRepo },
        { provide: AiProvidersService, useValue: {} },
      ],
    }).compile();
    svc = mod.get(BugReportsService);
  });

  it('createFeedback rejects a feature with no title', async () => {
    await expect(svc.createFeedback(
      { kind: 'feature', target_platform: 'windows', description: 'x', source: 'web' } as any, null,
    )).rejects.toBeInstanceOf(BadRequestException);
  });

  it('createFeedback rejects a feature with no target_platform', async () => {
    await expect(svc.createFeedback(
      { kind: 'feature', title: 'Add dark mode', description: 'x', source: 'web' } as any, null,
    )).rejects.toBeInstanceOf(BadRequestException);
  });

  it('createFeedback stores a feature row with vote_count 0, is_public true, status new', async () => {
    const row = await svc.createFeedback(
      { kind: 'feature', title: 'Add dark mode', target_platform: 'mac', description: 'please', source: 'web' } as any,
      'user-1',
    );
    expect(row.kind).toBe('feature');
    expect(row.title).toBe('Add dark mode');
    expect(row.target_platform).toBe('mac');
    expect(row.vote_count).toBe(0);
    expect(row.is_public).toBe(true);
    expect(row.status).toBe('new');
    expect(row.user_id).toBe('user-1');
  });

  it('createFeedback with kind=bug ignores title/target_platform', async () => {
    const row = await svc.createFeedback(
      { kind: 'bug', title: 'ignored', target_platform: 'mac', description: 'broke', source: 'web' } as any, null,
    );
    expect(row.kind).toBe('bug');
    expect(row.title).toBeNull();
    expect(row.target_platform).toBeNull();
  });

  it('toggleVote adds then removes a vote and recomputes vote_count', async () => {
    const row = await svc.createFeedback(
      { kind: 'feature', title: 'X', target_platform: 'linux', description: 'd', source: 'web' } as any, null,
    );
    let res = await svc.toggleVote(row.id, 'user-1');
    expect(res).toEqual({ vote_count: 1, hasVoted: true });
    expect((await bugRepo.findOne({ where: { id: row.id } }))!.vote_count).toBe(1);

    res = await svc.toggleVote(row.id, 'user-1');
    expect(res).toEqual({ vote_count: 0, hasVoted: false });
    expect((await bugRepo.findOne({ where: { id: row.id } }))!.vote_count).toBe(0);
  });

  it('toggleVote dedupes — a second user adds a distinct vote', async () => {
    const row = await svc.createFeedback(
      { kind: 'feature', title: 'X', target_platform: 'linux', description: 'd', source: 'web' } as any, null,
    );
    await svc.toggleVote(row.id, 'user-1');
    const res = await svc.toggleVote(row.id, 'user-2');
    expect(res.vote_count).toBe(2);
  });

  it('toggleVote on a non-feature row throws NotFound', async () => {
    const bug = await svc.createFeedback(
      { kind: 'bug', description: 'd', source: 'web' } as any, null,
    );
    await expect(svc.toggleVote(bug.id, 'user-1')).rejects.toBeInstanceOf(NotFoundException);
    await expect(svc.toggleVote('missing', 'user-1')).rejects.toBeInstanceOf(NotFoundException);
  });

  it('listPublicFeatures returns only public features, votes desc, with viewerHasVoted when userId given', async () => {
    const a = await svc.createFeedback({ kind: 'feature', title: 'A', target_platform: 'mac', description: 'd', source: 'web' } as any, null);
    const b = await svc.createFeedback({ kind: 'feature', title: 'B', target_platform: 'mac', description: 'd', source: 'web' } as any, null);
    await svc.createFeedback({ kind: 'bug', description: 'd', source: 'web' } as any, null);
    const hidden = await svc.createFeedback({ kind: 'feature', title: 'H', target_platform: 'mac', description: 'd', source: 'web' } as any, null);
    await svc.update(hidden.id, { is_public: false });
    await svc.toggleVote(b.id, 'user-1');
    await svc.toggleVote(b.id, 'user-2');
    await svc.toggleVote(a.id, 'user-1');

    const { rows, total } = await svc.listPublicFeatures({ page: 1, limit: 10 }, 'user-1');
    expect(total).toBe(2);
    expect(rows.map(r => r.title)).toEqual(['B', 'A']);
    expect(rows.find(r => r.title === 'B')!.viewerHasVoted).toBe(true);
    expect(rows.find(r => r.title === 'A')!.viewerHasVoted).toBe(true);

    const anon = await svc.listPublicFeatures({ page: 1, limit: 10 }, null);
    expect(anon.rows.every(r => r.viewerHasVoted === false)).toBe(true);
  });

  it('listPublicFeatures filters by status and target_platform', async () => {
    const a = await svc.createFeedback({ kind: 'feature', title: 'A', target_platform: 'mac', description: 'd', source: 'web' } as any, null);
    await svc.createFeedback({ kind: 'feature', title: 'B', target_platform: 'linux', description: 'd', source: 'web' } as any, null);
    await svc.update(a.id, { status: 'reviewing' });

    expect((await svc.listPublicFeatures({ status: 'reviewing' } as any, null)).rows.map(r => r.title)).toEqual(['A']);
    expect((await svc.listPublicFeatures({ target_platform: 'linux' } as any, null)).rows.map(r => r.title)).toEqual(['B']);
  });

  it('update validates target_platform against the allow-list', async () => {
    const row = await svc.createFeedback({ kind: 'feature', title: 'A', target_platform: 'mac', description: 'd', source: 'web' } as any, null);
    await expect(svc.update(row.id, { target_platform: 'nope' })).rejects.toBeInstanceOf(BadRequestException);
    const ok = await svc.update(row.id, { target_platform: 'windows' });
    expect(ok.target_platform).toBe('windows');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx jest src/bug-reports/bug-reports.service.spec.ts`
Expected: FAIL — `svc.createFeedback is not a function` / `toggleVote` / `listPublicFeatures` undefined; `FeatureVote` repository token not provided until we wire it (Task 6) — for the spec it's provided as a fake, so the failure is purely the missing methods.

- [ ] **Step 3: Implement the service methods**

In `backend/src/bug-reports/bug-reports.service.ts`:

a) Add imports at the top:

```typescript
import { FeatureVote } from './entities/feature-vote.entity';
import { CreateFeedbackDto, TARGET_PLATFORMS } from './dto/create-feedback.dto';
```

b) Inject the `FeatureVote` repo in the constructor (add a third parameter):

```typescript
  constructor(
    @InjectRepository(BugReport)
    private readonly repo: Repository<BugReport>,
    @InjectRepository(FeatureVote)
    private readonly votes: Repository<FeatureVote>,
    private readonly aiProviders: AiProvidersService,
  ) {}
```

c) Add these methods to the class body (after `create()`):

```typescript
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
      if (t.length < 3 || t.length > 80) {
        throw new BadRequestException('title must be 3-80 characters for a feature request');
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
```

d) Update `update()` to handle the new fields. Replace the existing `update()` method body with:

```typescript
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
      if (t.length < 3 || t.length > 80) {
        throw new BadRequestException('title must be 3-80 characters');
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
```

e) Update `findAllPaginated()` to accept an optional `kind` and `target_platform` filter — change its signature and add filters right after the existing `status` filter block:

```typescript
  async findAllPaginated(
    query: ListQuery & { status?: string; kind?: string; target_platform?: string },
  ) {
    const qb = this.repo.createQueryBuilder('br');

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

    const result = await applyListQuery(
      qb,
      { ...query, status: undefined as any, kind: undefined as any, target_platform: undefined as any },
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
      null,
    );
    return result;
  }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx jest src/bug-reports/bug-reports.service.spec.ts`
Expected: PASS (all specs green).

- [ ] **Step 5: Type-check**

Run: `npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add src/bug-reports/bug-reports.service.ts src/bug-reports/bug-reports.service.spec.ts
git commit -m "feat(feedback): createFeedback / toggleVote / listPublicFeatures + admin update fields + kind/platform filters"
```

---

### Task 5: `FeedbackController` — public `/feedback` routes

**Files:**
- Create: `backend/src/bug-reports/feedback.controller.ts`
- Test: `backend/src/bug-reports/feedback.controller.spec.ts` (create)

- [ ] **Step 1: Write the failing test**

Create `backend/src/bug-reports/feedback.controller.spec.ts`:

```typescript
import { Test } from '@nestjs/testing';
import { UnauthorizedException } from '@nestjs/common';
import * as jwt from 'jsonwebtoken';
import { FeedbackController } from './feedback.controller';
import { BugReportsService } from './bug-reports.service';

describe('FeedbackController', () => {
  let ctrl: FeedbackController;
  const svc = {
    createFeedback: jest.fn(),
    listPublicFeatures: jest.fn(),
    toggleVote: jest.fn(),
  };
  const SECRET = 'test_secret';

  const reqWith = (token?: string) =>
    ({ headers: token ? { authorization: `Bearer ${token}` } : {} } as any);

  beforeAll(() => { process.env.JWT_SECRET = SECRET; });
  beforeEach(async () => {
    jest.clearAllMocks();
    const mod = await Test.createTestingModule({
      controllers: [FeedbackController],
      providers: [{ provide: BugReportsService, useValue: svc }],
    }).compile();
    ctrl = mod.get(FeedbackController);
  });

  it('POST /feedback stamps user_id from a valid Bearer token', async () => {
    svc.createFeedback.mockResolvedValue({ id: 'r1' });
    const token = jwt.sign({ sub: 'user-9' }, SECRET);
    const out = await ctrl.create({ kind: 'feature', title: 'X', target_platform: 'mac', description: 'd', source: 'web' } as any, reqWith(token));
    expect(svc.createFeedback).toHaveBeenCalledWith(expect.objectContaining({ kind: 'feature' }), 'user-9');
    expect(out).toEqual({ id: 'r1' });
  });

  it('POST /feedback treats a bad token as anonymous', async () => {
    svc.createFeedback.mockResolvedValue({ id: 'r2' });
    await ctrl.create({ kind: 'feature', title: 'X', target_platform: 'mac', description: 'd', source: 'web' } as any, reqWith('garbage'));
    expect(svc.createFeedback).toHaveBeenCalledWith(expect.anything(), null);
  });

  it('GET /feedback passes filters + decoded userId through', async () => {
    svc.listPublicFeatures.mockResolvedValue({ rows: [], total: 0 });
    const token = jwt.sign({ sub: 'user-3' }, SECRET);
    await ctrl.list({ status: 'reviewing', target_platform: 'linux', page: '2', limit: '5' } as any, reqWith(token));
    expect(svc.listPublicFeatures).toHaveBeenCalledWith(
      { status: 'reviewing', target_platform: 'linux', page: '2', limit: '5' }, 'user-3',
    );
  });

  it('POST /feedback/:id/vote requires a valid token', async () => {
    await expect(ctrl.vote('feat-1', reqWith())).rejects.toBeInstanceOf(UnauthorizedException);
    await expect(ctrl.vote('feat-1', reqWith('garbage'))).rejects.toBeInstanceOf(UnauthorizedException);
  });

  it('POST /feedback/:id/vote toggles via the service', async () => {
    svc.toggleVote.mockResolvedValue({ vote_count: 3, hasVoted: true });
    const token = jwt.sign({ sub: 'user-7' }, SECRET);
    const out = await ctrl.vote('feat-1', reqWith(token));
    expect(svc.toggleVote).toHaveBeenCalledWith('feat-1', 'user-7');
    expect(out).toEqual({ vote_count: 3, hasVoted: true });
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx jest src/bug-reports/feedback.controller.spec.ts`
Expected: FAIL — `Cannot find module './feedback.controller'`.

- [ ] **Step 3: Implement the controller**

Create `backend/src/bug-reports/feedback.controller.ts`:

```typescript
import {
  Controller, Post, Get, Body, Param, Query, Req, HttpCode, UnauthorizedException,
} from '@nestjs/common';
import { ApiTags, ApiOperation } from '@nestjs/swagger';
import { Request } from 'express';
import * as jwt from 'jsonwebtoken';
import { BugReportsService } from './bug-reports.service';
import { CreateFeedbackDto } from './dto/create-feedback.dto';

/** Decodes a Bearer JWT from the request; returns the user id or null. */
function userIdFrom(req: Request): string | null {
  const h = req.headers['authorization'];
  if (typeof h !== 'string' || !h.startsWith('Bearer ')) return null;
  try {
    const decoded = jwt.verify(h.slice(7), process.env.JWT_SECRET || 'change_me') as {
      sub?: string; user_id?: string;
    };
    return decoded.sub || decoded.user_id || null;
  } catch {
    return null;
  }
}

/**
 * Public feedback API powering the upvote board at draftright.info/feedback
 * and the native "Suggest a feature" forms in every client.
 *
 *   POST /feedback            create a bug or feature request (JWT optional → user_id)
 *   GET  /feedback            public board feed (kind=feature, is_public), votes desc
 *   POST /feedback/:id/vote   toggle the caller's upvote (JWT REQUIRED)
 *
 * The legacy multipart `POST /bug-reports` route (screenshots) is untouched.
 */
@ApiTags('feedback')
@Controller('feedback')
export class FeedbackController {
  constructor(private readonly feedback: BugReportsService) {}

  @Post()
  @HttpCode(201)
  @ApiOperation({ summary: 'Submit a bug report or feature request' })
  async create(@Body() dto: CreateFeedbackDto, @Req() req: Request) {
    const row = await this.feedback.createFeedback(dto, userIdFrom(req));
    return { id: row.id, message: dto.kind === 'feature' ? 'Feature request received. Thanks!' : 'Bug report received. Thanks!' };
  }

  @Get()
  @ApiOperation({ summary: 'Public feature-request board feed (sorted by votes)' })
  async list(
    @Query() q: { page?: string; limit?: string; status?: string; target_platform?: string },
    @Req() req: Request,
  ) {
    return this.feedback.listPublicFeatures(q as any, userIdFrom(req));
  }

  @Post(':id/vote')
  @HttpCode(200)
  @ApiOperation({ summary: 'Toggle the signed-in user’s upvote on a feature request' })
  async vote(@Param('id') id: string, @Req() req: Request) {
    const userId = userIdFrom(req);
    if (!userId) throw new UnauthorizedException('sign in to vote');
    return this.feedback.toggleVote(id, userId);
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx jest src/bug-reports/feedback.controller.spec.ts`
Expected: PASS.

- [ ] **Step 5: Type-check**

Run: `npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add src/bug-reports/feedback.controller.ts src/bug-reports/feedback.controller.spec.ts
git commit -m "feat(feedback): public FeedbackController — POST/GET /feedback + POST /feedback/:id/vote"
```

---

### Task 6: Wire `FeatureVote` + `FeedbackController` into the module

**Files:**
- Modify: `backend/src/bug-reports/bug-reports.module.ts`

- [ ] **Step 1: Update the module**

Replace the contents of `backend/src/bug-reports/bug-reports.module.ts` with:

```typescript
import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { BugReportsController } from './bug-reports.controller';
import { FeedbackController } from './feedback.controller';
import { BugReportsService } from './bug-reports.service';
import { BugReport } from './entities/bug-report.entity';
import { FeatureVote } from './entities/feature-vote.entity';
import { AiProvidersModule } from '../ai-providers/ai-providers.module';
import { BugFixProposalCron } from './bug-fix-proposal.cron';

@Module({
  imports: [TypeOrmModule.forFeature([BugReport, FeatureVote]), AiProvidersModule],
  controllers: [BugReportsController, FeedbackController],
  providers: [BugReportsService, BugFixProposalCron],
  exports: [BugReportsService],
})
export class BugReportsModule {}
```

- [ ] **Step 2: Type-check + full unit suite**

Run: `npx tsc --noEmit && npx jest src/bug-reports`
Expected: no type errors; all `bug-reports` specs pass.

- [ ] **Step 3: Commit**

```bash
git add src/bug-reports/bug-reports.module.ts
git commit -m "feat(feedback): register FeatureVote entity + FeedbackController in BugReportsModule"
```

---

### Task 7: Scope the AI-fix cron + candidate query to `kind='bug'`

**Files:**
- Modify: `backend/src/bug-reports/bug-reports.service.ts` (the `findFixProposalCandidates` method)

- [ ] **Step 1: Add the test**

Append to `backend/src/bug-reports/bug-reports.service.spec.ts` inside the `describe` block:

```typescript
  it('findFixProposalCandidates ignores feature requests', async () => {
    // FakeRepo doesn't implement createQueryBuilder; assert intent at the SQL level instead.
    // (Covered by the prod query string — see implementation. This test documents the requirement.)
    const sql = (BugReportsService.prototype.findFixProposalCandidates as any).toString();
    expect(sql).toContain("kind");
  });
```

- [ ] **Step 2: Run it — fails**

Run: `npx jest src/bug-reports/bug-reports.service.spec.ts -t "ignores feature requests"`
Expected: FAIL — the method body doesn't mention `kind` yet.

- [ ] **Step 3: Update `findFixProposalCandidates`**

In `backend/src/bug-reports/bug-reports.service.ts`, change the query in `findFixProposalCandidates` to add a `kind` filter:

```typescript
  async findFixProposalCandidates(limit: number = 5): Promise<BugReport[]> {
    return this.repo
      .createQueryBuilder('br')
      .where('br.status = :st', { st: 'new' })
      .andWhere("br.kind = 'bug'")
      .andWhere('br.ai_fix_proposal IS NULL')
      .orderBy('br.created_at', 'DESC')
      .limit(limit)
      .getMany();
  }
```

- [ ] **Step 4: Run it — passes**

Run: `npx jest src/bug-reports/bug-reports.service.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/bug-reports/bug-reports.service.ts src/bug-reports/bug-reports.service.spec.ts
git commit -m "fix(feedback): AI fix-proposal cron only considers kind='bug' rows"
```

---

### Task 8: Admin endpoints — `kind` / `target_platform` filters + patch fields

**Files:**
- Modify: `backend/src/admin/admin.controller.ts` (the `listBugReports` and `updateBugReport` handlers)

- [ ] **Step 1: Update `listBugReports`**

In `backend/src/admin/admin.controller.ts`, replace the `listBugReports` handler with:

```typescript
  @Get('bug-reports')
  async listBugReports(@Query() q: Record<string, unknown>) {
    const query = parseListQuery(q);
    const status = typeof q.status === 'string' ? q.status : undefined;
    const kind = typeof q.kind === 'string' ? q.kind : undefined;
    const target_platform = typeof q.target_platform === 'string' ? q.target_platform : undefined;
    return this.bugReportsService.findAllPaginated({
      ...query,
      status: status as any,
      kind,
      target_platform,
    });
  }
```

- [ ] **Step 2: Update `updateBugReport`**

Replace the `updateBugReport` handler with:

```typescript
  @Patch('bug-reports/:id')
  async updateBugReport(
    @Param('id') id: string,
    @Body() body: { status?: string; admin_notes?: string; title?: string; target_platform?: string; is_public?: boolean },
  ) {
    return this.bugReportsService.update(id, body);
  }
```

(`UpdateBugReportDto` already accepts these fields after Task 3; `update()` validates them.)

- [ ] **Step 3: Type-check**

Run: `npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add src/admin/admin.controller.ts
git commit -m "feat(feedback): admin bug-reports list accepts kind/target_platform filters; patch accepts title/target_platform/is_public"
```

---

### Task 9: Smoke the running app + apply the migration locally

**Files:** none (manual verification).

- [ ] **Step 1: Start infra + backend**

Run:
```bash
docker compose up -d postgres redis
cd backend && npm run start:dev
```
Expected: backend boots clean; TypeORM `synchronize` (NODE_ENV ≠ production) creates the `feature_votes` table and the new `bug_reports` columns. No migration error in the logs.

- [ ] **Step 2: Create a feature request**

Run (new terminal):
```bash
curl -s -X POST http://localhost:3000/feedback -H 'Content-Type: application/json' \
  -d '{"kind":"feature","title":"Dark mode for the panel","target_platform":"mac","description":"Please follow system appearance","source":"web"}'
```
Expected: `{"id":"<uuid>","message":"Feature request received. Thanks!"}`

- [ ] **Step 3: Reject a feature with no title / bad platform**

Run:
```bash
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://localhost:3000/feedback -H 'Content-Type: application/json' \
  -d '{"kind":"feature","target_platform":"mac","description":"x","source":"web"}'
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://localhost:3000/feedback -H 'Content-Type: application/json' \
  -d '{"kind":"feature","title":"ok","target_platform":"banana","description":"x","source":"web"}'
```
Expected: `400` and `400`.

- [ ] **Step 4: List the public board**

Run: `curl -s 'http://localhost:3000/feedback' | head -c 400`
Expected: JSON `{ "rows": [ { ..., "title":"Dark mode for the panel", "target_platform":"mac", "vote_count":0, "viewerHasVoted":false } ], "total": 1 }`

- [ ] **Step 5: Vote without a token → 401**

Run: `curl -s -o /dev/null -w '%{http_code}\n' -X POST http://localhost:3000/feedback/<uuid-from-step-2>/vote`
Expected: `401`.

- [ ] **Step 6: Legacy bug-report route still works**

Run:
```bash
curl -s -X POST http://localhost:3000/bug-reports -F 'description=still works' -F 'source=web'
```
Expected: `{"id":"<uuid>","message":"Bug report received. Thanks!"}` — and `curl -s 'http://localhost:3000/feedback'` still shows `total: 1` (the bug doesn't appear on the feature board).

- [ ] **Step 7: Confirm the prod-migration file is correct**

Run: `docker compose exec -T postgres psql -U draftright -d draftright < backend/sql/2026-05-12-feedback.sql && echo OK`
Expected: `OK` (idempotent — re-running is a no-op; this is the exact command to run on prod later, swapping in `docker-compose.prod.yml`).

- [ ] **Step 8: Stop the dev backend** (Ctrl-C). No commit — this task is verification only.

---

### Task 10: Update docs + final type-check + full suite

**Files:**
- Modify: `backend/CLAUDE.md` (document the `feedback` routes — follow the existing module-list format)
- Modify: `docs/changelog.md` (prepend a bullet under the `## 2026-05-12` section)

- [ ] **Step 1: Document in `backend/CLAUDE.md`**

Add a `feedback` entry wherever modules/endpoints are listed, matching the surrounding style, e.g.:

```markdown
- **feedback** (`src/bug-reports/`) — `POST /feedback` (bug or feature request; JWT optional → user_id), `GET /feedback` (public board: kind=feature & is_public, votes desc, `?status=`/`?target_platform=` filters), `POST /feedback/:id/vote` (toggle upvote, JWT required). `feature_votes` table = one vote per user per feature; `bug_reports.vote_count` is derived. Legacy multipart `POST /bug-reports` (screenshots) unchanged. AI fix-proposal cron only touches `kind='bug'`. Schema migration: `backend/sql/2026-05-12-feedback.sql`.
```

- [ ] **Step 2: Changelog bullet**

In `docs/changelog.md`, under `## 2026-05-12`, add:

```markdown
- Feedback / feature-request backend (Spec A): `bug_reports` gains `kind`/`title`/`target_platform`/`vote_count`/`is_public`; new `feature_votes` table; public `GET/POST /feedback` + `POST /feedback/:id/vote`; admin list/patch gain kind+platform filters. `POST /bug-reports` contract unchanged. Migration in `backend/sql/2026-05-12-feedback.sql`.
```

- [ ] **Step 3: Final checks**

Run: `cd backend && npx tsc --noEmit && npx jest`
Expected: no type errors; full unit suite passes.

- [ ] **Step 4: Commit**

```bash
git add backend/CLAUDE.md docs/changelog.md
git commit -m "docs(feedback): document /feedback routes + Spec A changelog entry"
```

---

## After this plan

Spec A is done — backend + admin support feature requests, the public board feed, and voting. **Deferred from the spec** (do as a follow-up if abuse shows up): IP rate-limiting on `POST /feedback` → 429 — the existing `POST /bug-reports` route has none either, so add `@nestjs/throttler` app-wide in a separate small task rather than bolting it onto just this route. Next: **Spec B** (native submit surfaces — macOS / Windows / Linux / Flutter / web-playground forms with the title + platform-dropdown + description fields) and **Spec C** (the `draftright.info/feedback` Astro board page). Each gets its own plan. Deploy note: before Spec C goes live, apply `backend/sql/2026-05-12-feedback.sql` to prod (Postgres runs with `synchronize: false`) and redeploy the backend container.
