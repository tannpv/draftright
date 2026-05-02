# Extension Tokens Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current "extensions read the user's short-lived access JWT and silently fail when it expires" model with a dedicated, scoped, long-lived `extension_token` minted by the main app and consumed by the iOS keyboard, iOS share extension, and Android keyboard.

**Architecture:** Backend issues opaque `dr_ext_<base64>` tokens, stored as SHA-256 hashes in a new `extension_tokens` table, scoped (initially `['rewrite']`), revocable per device, with a sliding 90-day idle expiry. A new `RewriteAuthGuard` accepts either the existing user JWT or an extension token. The Flutter main app mints a token at login, persists it in iOS App Group keychain (new) and Android SharedPreferences, and the extensions read it from those shared stores. The user JWT and refresh-token flow remain unchanged for the main app.

**Tech Stack:**
- Backend: NestJS 10, TypeORM, PostgreSQL 16, ioredis 5, Jest. Existing Passport JWT flow preserved.
- Flutter: dart:convert + http + flutter_secure_storage + shared_preferences + MethodChannel for iOS bridge.
- iOS: Swift, App Group `group.com.draftright.v2`, new keychain-access-group `$(AppIdentifierPrefix)com.draftright.v2.shared`, `SecItemAdd`/`SecItemCopyMatching`.
- Android: Kotlin, `FlutterSharedPreferences` (same UID as main app — already shared implicitly).

---

## Migration Strategy & Compatibility

The plan is **dual-accept throughout**. At every stage, the existing access-JWT path through extensions continues to work. Extensions prefer the extension token if present, fall back to the access JWT otherwise. This means:

- Stage 1 (backend) ships with no client changes required.
- Stage 4 (extensions read new token) ships safely even if some users haven't yet launched the new main app version.
- After everyone has launched the new main app once, the access JWT path becomes dead code and can be deleted in a follow-up release.

There is no flag day. There is no breaking change for any user.

---

## File Structure

### Backend (`/opt/openAi/DraftRight/backend`)

**Create:**
- `migrations/2026-05-02-extension-tokens.sql` — table migration
- `src/auth/extension-token.entity.ts` — TypeORM entity
- `src/auth/extension-token.service.ts` — mint/list/revoke/validate/hash
- `src/auth/extension-token.controller.ts` — endpoints
- `src/auth/rewrite-auth.guard.ts` — accepts JWT or extension token
- `src/auth/dto/mint-extension-token.dto.ts`
- `src/auth/extension-token.service.spec.ts`
- `src/auth/extension-token.controller.spec.ts`
- `src/auth/rewrite-auth.guard.spec.ts`

**Modify:**
- `src/auth/auth.module.ts` — register entity, service, controller, guard
- `src/rewrite/rewrite.controller.ts` — switch decorator from `JwtAuthGuard` to `RewriteAuthGuard`
- `src/users/entities/user.entity.ts` — optional `@OneToMany` relation
- `package.json` — no new deps (we use crypto built-in and existing ioredis)

### Flutter main app (`/opt/openAi/DraftRight/DraftRightMobile/lib`)

**Create:**
- `lib/services/extension_token_service.dart` — mint, sync to shared storage, revoke
- `test/services/extension_token_service_test.dart`

**Modify:**
- `lib/services/auth_service.dart` — call `ExtensionTokenService.ensureMinted()` after login, `revoke()` on logout
- `lib/main.dart` (or wherever app boot wires services) — instantiate `ExtensionTokenService`

### iOS (`/opt/openAi/DraftRight/DraftRightMobile/ios`)

**Create:**
- `Shared/SharedKeychain.swift` — file added to all three targets in Xcode; wraps Keychain access-group
- (Note: requires manual Xcode operations — adding entitlement and target membership)

**Modify:**
- `Runner/Runner.entitlements` — add `keychain-access-groups`
- `DraftRightKeyboard/DraftRightKeyboard.entitlements` — same
- `DraftRightAction/DraftRightAction.entitlements` — same
- `Runner/AppDelegate.swift` — extend MethodChannel to handle `setKeychain` / `getKeychain` / `deleteKeychain`
- `DraftRightKeyboard/SharedSettings.swift` — add `extensionToken` getter from keychain
- `DraftRightKeyboard/BackendClient.swift` — prefer extension token over access token
- `DraftRightAction/SharedSettings.swift` — add `extensionToken` getter from keychain
- `DraftRightAction/ActionViewController.swift` — prefer extension token

### Android (`/opt/openAi/DraftRight/DraftRightMobile/android`)

**Modify:**
- `android/app/src/main/kotlin/com/draftright/keyboard/SharedSettings.kt` — add `extensionToken` getter
- `android/app/src/main/kotlin/com/draftright/keyboard/BackendClient.kt` — prefer extension token

---

## Stages

0. **Preflight** — GitHub issue, test cases in `docs/test-cases.xlsx`, feature branch off `develop`, infra docs review
1. **Backend** — table, service, controller, guard, scope enforcement, dual-accept on `/rewrite`
2. **iOS keychain infrastructure** — entitlements + SharedKeychain.swift bridge + AppDelegate channel methods
3. **Flutter mint flow** — `ExtensionTokenService`, sync into shared storage on login
4. **Extensions** — iOS keyboard, iOS share, Android keyboard read extension token (with fallback)
5. **End-to-end verification** — manual test matrix on real devices
6. **Optional follow-up** — "Active extensions" UI in /account; deprecate fallback path
7. **Deploy + sign-off** — testing → label → E2E → production → label → ✅ How to Verify comment

Stage 0 is mandatory and CRITICAL per the global checklist (`~/.claude/CLAUDE.md` § Development Task Checklist). Stages 1–4 must complete in order at the platform level (each depends on the previous), but most tasks within a stage can be done independently. Stage 7 runs sequentially after the implementation stages and is a CRITICAL pipeline — never deploy to production without testing first.

---

## TC-ID Mapping

Test cases are tracked in `docs/test-cases.xlsx` (sheet: `EXTTOK`). Every code-level test in this plan is annotated with its TC-ID below. Add `// TC: EXTTOK-NNN` (or `# TC: EXTTOK-NNN` for SQL/Dart) as a leading comment in the corresponding test as you implement it.

| TC-ID | Description | Verified by |
|---|---|---|
| EXTTOK-001 | Backend mints token with `dr_ext_` prefix and 43-char base64 body | Task 1.3 unit test `mint returns plaintext token with dr_ext_ prefix` |
| EXTTOK-002 | Backend stores only sha256(token), never plaintext | Task 1.3 unit test `stores only the sha256 hash, not the plaintext` |
| EXTTOK-003 | Re-mint for same `(user, device_id)` revokes the old row | Task 1.3 unit test `revokes the existing active token...` + Task 5.1 row A8 |
| EXTTOK-004 | Validate returns user_id + scopes for active token | Task 1.3 unit test `returns user_id and scopes for valid token` |
| EXTTOK-005 | Validate returns null for revoked token | Task 1.3 unit test + Task 5.1 row A7 |
| EXTTOK-006 | Validate returns null for token without `dr_ext_` prefix | Task 1.3 unit test `returns null for token without dr_ext_ prefix` |
| EXTTOK-007 | Revoke endpoint sets `revoked_at` for matching `(user_id, id)` only | Task 1.3 unit test + Task 1.5 controller test |
| EXTTOK-008 | `POST /rewrite` accepts user JWT (back-compat) | Task 1.6 guard test + Task 5.1 row A1 |
| EXTTOK-009 | `POST /rewrite` accepts extension token with `rewrite` scope | Task 1.6 guard test + Task 5.1 row A3 |
| EXTTOK-010 | Non-rewrite endpoints (`/auth/me`, etc.) reject extension tokens | Task 1.6 guard usage + Task 5.1 row A4 |
| EXTTOK-011 | Flutter mints on login (calls `POST /auth/extension-tokens`) | Task 5.1 rows B1–B2, C1–C2 |
| EXTTOK-012 | Flutter clears stored token on logout | Task 5.1 rows B6, C5 |
| EXTTOK-013 | Flutter `device_id` is generated once and persisted | Task 3.1 unit test `deviceId is generated once and persisted` |
| EXTTOK-014 | iOS keychain item is readable from all three targets via shared access-group | Task 5.1 rows B2, B7 (cross-target read confirmed implicitly when both keyboard and share work after a single login) |
| EXTTOK-015 | iOS keyboard rewrite still works 30 min after main app last used | Task 5.1 row B3 |
| EXTTOK-016 | iOS share extension rewrite still works 30 min after main app last used | Task 5.1 row B4 |
| EXTTOK-017 | Android IME rewrite still works 30 min after main app last used | Task 5.1 row C3 |
| EXTTOK-018 | Upgrade-in-place: old access-JWT continues to work until first main-app open mints the new token | Task 5.1 rows D1–D5 |
| EXTTOK-019 | Re-mint for same device rotates token (old presents as 401) | Task 5.1 row A8 |
| EXTTOK-020 | After logout, server sees presented token as revoked / unrecognized | Task 5.1 row B6 + manual `psql` check that no active row remains for the user |

---

# Stage 0 — Preflight (CRITICAL)

This stage is non-skippable per the project's mandatory Development Task Checklist. Each task here is an action that must complete before Stage 1 begins.

> **Note:** GitHub-issue tracking, status labels, and the `## ✅ How to Verify` issue comment are explicitly skipped for this plan per user direction. The equivalent verification record lives in `docs/superpowers/plans/2026-05-02-extension-tokens-verification.md` (Task 5.1).

## Task 0.1: Add test cases to `docs/test-cases.xlsx`

**Files:**
- Modify: `docs/test-cases.xlsx` — add a new sheet `EXTTOK` (or rows in an existing combined sheet, matching project convention)

- [ ] **Step 1: Open the spreadsheet and add a new sheet `EXTTOK`**

Use the column layout already established by other sheets (likely: `TC-ID | Title | Preconditions | Steps | Expected | Priority | Owner`). If the layout differs, match the existing convention exactly.

- [ ] **Step 2: Populate rows EXTTOK-001 through EXTTOK-020**

Use the descriptions in the **TC-ID Mapping** table at the top of this plan as the `Title` column. Fill `Steps` and `Expected` from the matching row in Stage 5's Verification Record matrix (Task 5.1) for the manual rows; for unit-test-covered rows, write `Verified by automated unit test in <file>`.

- [ ] **Step 3: Save and commit**

```bash
git add docs/test-cases.xlsx
git commit -m "test: add EXTTOK-001..EXTTOK-020 test cases for extension tokens"
```

---

## Task 0.2: Read infrastructure docs and validate environment

**Files:** read-only.

- [ ] **Step 1: Read the production state memory**

Read `~/.claude/projects/-opt-openAi-DraftRight/memory/project_production_live.md`. Note: production URLs, container names, env-var quoting gotchas, deployment commands.

- [ ] **Step 2: Read deploy-scoped CLAUDE.md if it exists**

```bash
ls /opt/openAi/DraftRight/deploy/CLAUDE.md 2>/dev/null && cat /opt/openAi/DraftRight/deploy/CLAUDE.md
ls /opt/openAi/DraftRight/backend/CLAUDE.md 2>/dev/null && cat /opt/openAi/DraftRight/backend/CLAUDE.md
```

Confirm: backend production deploy path, env-file usage, migration application procedure.

- [ ] **Step 3: Verify dev DB matches prod schema baseline**

The migration in Task 1.1 uses `gen_random_uuid()` and array columns — confirm the prod Postgres has the `pgcrypto` extension enabled (it should, since `users.id` is already UUID) and supports `text[]`. Quick check on dev:

```bash
psql "$DATABASE_URL" -c "SELECT gen_random_uuid();"
psql "$DATABASE_URL" -c "SELECT ARRAY['a','b']::text[];"
```

Both should return values, not errors.

- [ ] **Step 4: Note any blocking gotchas**

If you find anything that materially affects the plan (e.g. prod uses a different Postgres version that doesn't support `gen_random_uuid` without extension), pause execution and surface the finding to the user before proceeding to Stage 1.

---

## Task 0.3: Create the feature branch off `develop`

**Files:** none — git operation.

- [ ] **Step 1: Confirm current state**

```bash
cd /opt/openAi/DraftRight
git status --short
git branch --show-current
```

The repo is currently on `feature/customer-registration-20260430` with uncommitted changes. Do NOT carry those changes into the new branch.

- [ ] **Step 2: Stash or set aside any in-progress work**

If `git status` shows uncommitted modifications, decide with the user whether to commit, stash, or discard them. Do not silently bring them along.

```bash
# Option A: stash
git stash push -m "WIP on customer-registration before extension-tokens"

# Option B: confirm with user, then leave them in place if they're truly orphan
```

- [ ] **Step 3: Update develop and branch off**

```bash
git fetch origin
git checkout develop
git pull --ff-only origin develop
git checkout -b feature/extension-tokens-20260502
```

- [ ] **Step 4: Verify the new branch base**

```bash
git log --oneline -5
git rev-parse --abbrev-ref HEAD  # expect: feature/extension-tokens-20260502
```

- [ ] **Step 5: Push the empty branch**

```bash
git push -u origin feature/extension-tokens-20260502
```

**Stage 0 complete. Test cases are tracked, infra docs are reviewed, branch is on `feature/extension-tokens-20260502` off `develop`. Now safe to proceed to Stage 1.**

---

# Stage 1 — Backend

## Task 1.1: Create migration SQL

**Files:**
- Create: `backend/migrations/2026-05-02-extension-tokens.sql`

- [ ] **Step 1: Write the SQL migration**

```sql
-- 2026-05-02-extension-tokens.sql
-- Long-lived, scoped tokens for keyboard/share extensions.
-- Each row represents a token currently issued to a single user/device pair.
-- Plaintext tokens are never stored; we keep only sha256(token).

CREATE TABLE IF NOT EXISTS extension_tokens (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash    CHAR(64) NOT NULL,
  scopes        TEXT[] NOT NULL DEFAULT ARRAY['rewrite'],
  device_id     UUID NOT NULL,
  device_name   VARCHAR(64) NOT NULL DEFAULT 'mobile',
  last_used_at  TIMESTAMP WITHOUT TIME ZONE,
  created_at    TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
  revoked_at    TIMESTAMP WITHOUT TIME ZONE
);

-- Active token lookup (used per request).
CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_tokens_hash_active
  ON extension_tokens (token_hash)
  WHERE revoked_at IS NULL;

-- Re-mint replaces the existing active token for a (user, device).
CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_tokens_user_device_active
  ON extension_tokens (user_id, device_id)
  WHERE revoked_at IS NULL;

-- Audit lookup.
CREATE INDEX IF NOT EXISTS idx_ext_tokens_user
  ON extension_tokens (user_id);
```

- [ ] **Step 2: Apply against dev database**

Run: `psql "$DATABASE_URL" -f backend/migrations/2026-05-02-extension-tokens.sql`
Expected: `CREATE TABLE`, three `CREATE INDEX` lines.

- [ ] **Step 3: Verify with `\d extension_tokens`**

Run: `psql "$DATABASE_URL" -c '\d extension_tokens'`
Expected: column list matches the SQL; three indexes shown.

- [ ] **Step 4: Commit**

```bash
git add backend/migrations/2026-05-02-extension-tokens.sql
git commit -m "feat(backend): add extension_tokens migration"
```

---

## Task 1.2: Create the entity

**Files:**
- Create: `backend/src/auth/extension-token.entity.ts`

- [ ] **Step 1: Write the entity**

```typescript
import {
  Column,
  CreateDateColumn,
  Entity,
  Index,
  JoinColumn,
  ManyToOne,
  PrimaryGeneratedColumn,
} from 'typeorm';
import { User } from '../users/entities/user.entity';

@Entity('extension_tokens')
@Index(['user_id'])
export class ExtensionToken {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'uuid' })
  user_id: string;

  @ManyToOne(() => User, { onDelete: 'CASCADE' })
  @JoinColumn({ name: 'user_id' })
  user: User;

  @Column({ type: 'char', length: 64 })
  token_hash: string;

  @Column({ type: 'text', array: true, default: () => "ARRAY['rewrite']" })
  scopes: string[];

  @Column({ type: 'uuid' })
  device_id: string;

  @Column({ type: 'varchar', length: 64, default: 'mobile' })
  device_name: string;

  @Column({ type: 'timestamp', nullable: true })
  last_used_at: Date | null;

  @CreateDateColumn({ type: 'timestamp' })
  created_at: Date;

  @Column({ type: 'timestamp', nullable: true })
  revoked_at: Date | null;
}
```

- [ ] **Step 2: Type-check**

Run: `cd backend && npx tsc --noEmit`
Expected: no new errors related to this file.

- [ ] **Step 3: Commit**

```bash
git add backend/src/auth/extension-token.entity.ts
git commit -m "feat(backend): add ExtensionToken entity"
```

---

## Task 1.3: Service — mint, hash, validate

**Files:**
- Create: `backend/src/auth/extension-token.service.ts`
- Create: `backend/src/auth/extension-token.service.spec.ts`

- [ ] **Step 1: Write the failing test for `mint`**

```typescript
// backend/src/auth/extension-token.service.spec.ts
import { Test } from '@nestjs/testing';
import { getRepositoryToken } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { ExtensionToken } from './extension-token.entity';
import { ExtensionTokenService } from './extension-token.service';
import * as crypto from 'crypto';

describe('ExtensionTokenService', () => {
  let service: ExtensionTokenService;
  let repo: jest.Mocked<Repository<ExtensionToken>>;

  beforeEach(async () => {
    const repoMock = {
      findOne: jest.fn(),
      find: jest.fn(),
      create: jest.fn((x) => x),
      save: jest.fn(async (x) => ({ ...x, id: 'id-1', created_at: new Date() })),
      update: jest.fn(),
    } as any;

    const module = await Test.createTestingModule({
      providers: [
        ExtensionTokenService,
        { provide: getRepositoryToken(ExtensionToken), useValue: repoMock },
      ],
    }).compile();

    service = module.get(ExtensionTokenService);
    repo = module.get(getRepositoryToken(ExtensionToken));
  });

  describe('mint', () => {
    it('returns plaintext token with dr_ext_ prefix', async () => {
      repo.findOne.mockResolvedValue(null);
      const result = await service.mint('user-1', 'device-1', 'iPhone Keyboard');
      expect(result.token).toMatch(/^dr_ext_[A-Za-z0-9_-]{43}$/);
    });

    it('stores only the sha256 hash, not the plaintext', async () => {
      repo.findOne.mockResolvedValue(null);
      const result = await service.mint('user-1', 'device-1', 'iPhone Keyboard');
      const expectedHash = crypto.createHash('sha256').update(result.token).digest('hex');
      expect(repo.save).toHaveBeenCalledWith(
        expect.objectContaining({ token_hash: expectedHash }),
      );
      expect(repo.save).not.toHaveBeenCalledWith(
        expect.objectContaining({ token_hash: result.token }),
      );
    });

    it('revokes the existing active token for the same (user, device)', async () => {
      repo.findOne.mockResolvedValue({ id: 'existing-id' } as any);
      await service.mint('user-1', 'device-1', 'iPhone Keyboard');
      expect(repo.update).toHaveBeenCalledWith(
        { id: 'existing-id' },
        { revoked_at: expect.any(Date) },
      );
    });
  });

  describe('validate', () => {
    it('returns null for unknown token', async () => {
      repo.findOne.mockResolvedValue(null);
      const result = await service.validate('dr_ext_unknown');
      expect(result).toBeNull();
    });

    it('returns null for revoked token', async () => {
      repo.findOne.mockResolvedValue(null); // findOne uses revoked_at IS NULL filter
      const result = await service.validate('dr_ext_revoked');
      expect(result).toBeNull();
    });

    it('returns user_id and scopes for valid token', async () => {
      repo.findOne.mockResolvedValue({
        id: 'tok-1',
        user_id: 'user-1',
        scopes: ['rewrite'],
        revoked_at: null,
      } as any);
      const result = await service.validate('dr_ext_valid');
      expect(result).toEqual({ tokenId: 'tok-1', userId: 'user-1', scopes: ['rewrite'] });
    });

    it('returns null for token without dr_ext_ prefix', async () => {
      const result = await service.validate('not-an-extension-token');
      expect(result).toBeNull();
      expect(repo.findOne).not.toHaveBeenCalled();
    });
  });

  describe('revoke', () => {
    it('sets revoked_at on the row', async () => {
      await service.revoke('user-1', 'tok-1');
      expect(repo.update).toHaveBeenCalledWith(
        { id: 'tok-1', user_id: 'user-1' },
        { revoked_at: expect.any(Date) },
      );
    });
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && npx jest src/auth/extension-token.service.spec.ts`
Expected: FAIL — `ExtensionTokenService` cannot be imported.

- [ ] **Step 3: Write the service**

```typescript
// backend/src/auth/extension-token.service.ts
import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { IsNull, Repository } from 'typeorm';
import * as crypto from 'crypto';
import { ExtensionToken } from './extension-token.entity';

const TOKEN_PREFIX = 'dr_ext_';

export interface ValidatedExtensionToken {
  tokenId: string;
  userId: string;
  scopes: string[];
}

@Injectable()
export class ExtensionTokenService {
  constructor(
    @InjectRepository(ExtensionToken)
    private readonly repo: Repository<ExtensionToken>,
  ) {}

  async mint(
    userId: string,
    deviceId: string,
    deviceName: string,
  ): Promise<{ token: string; id: string }> {
    // Revoke existing active token for this (user, device) pair.
    const existing = await this.repo.findOne({
      where: { user_id: userId, device_id: deviceId, revoked_at: IsNull() },
    });
    if (existing) {
      await this.repo.update({ id: existing.id }, { revoked_at: new Date() });
    }

    // 32 random bytes → 43 url-safe base64 chars.
    const raw = crypto.randomBytes(32).toString('base64url');
    const token = `${TOKEN_PREFIX}${raw}`;
    const tokenHash = crypto.createHash('sha256').update(token).digest('hex');

    const row = this.repo.create({
      user_id: userId,
      token_hash: tokenHash,
      scopes: ['rewrite'],
      device_id: deviceId,
      device_name: deviceName,
    });
    const saved = await this.repo.save(row);
    return { token, id: saved.id };
  }

  async validate(presentedToken: string): Promise<ValidatedExtensionToken | null> {
    if (!presentedToken.startsWith(TOKEN_PREFIX)) return null;
    const tokenHash = crypto.createHash('sha256').update(presentedToken).digest('hex');
    const row = await this.repo.findOne({
      where: { token_hash: tokenHash, revoked_at: IsNull() },
    });
    if (!row) return null;
    // Update last_used_at write-behind. Don't await; failures here are non-fatal.
    this.repo
      .update({ id: row.id }, { last_used_at: new Date() })
      .catch(() => undefined);
    return { tokenId: row.id, userId: row.user_id, scopes: row.scopes };
  }

  async list(userId: string): Promise<ExtensionToken[]> {
    return this.repo.find({
      where: { user_id: userId, revoked_at: IsNull() },
      order: { created_at: 'DESC' },
    });
  }

  async revoke(userId: string, tokenId: string): Promise<void> {
    await this.repo.update(
      { id: tokenId, user_id: userId },
      { revoked_at: new Date() },
    );
  }

  async revokeAll(userId: string): Promise<void> {
    await this.repo.update(
      { user_id: userId, revoked_at: IsNull() },
      { revoked_at: new Date() },
    );
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && npx jest src/auth/extension-token.service.spec.ts`
Expected: PASS — all assertions green.

- [ ] **Step 5: Commit**

```bash
git add backend/src/auth/extension-token.service.ts backend/src/auth/extension-token.service.spec.ts
git commit -m "feat(backend): add ExtensionTokenService with mint/validate/revoke"
```

---

## Task 1.4: Mint DTO

**Files:**
- Create: `backend/src/auth/dto/mint-extension-token.dto.ts`

- [ ] **Step 1: Write the DTO**

```typescript
// backend/src/auth/dto/mint-extension-token.dto.ts
import { IsString, IsUUID, Length, Matches } from 'class-validator';

export class MintExtensionTokenDto {
  @IsUUID('4')
  device_id: string;

  @IsString()
  @Length(1, 64)
  @Matches(/^[A-Za-z0-9 _.\-]+$/, {
    message: 'device_name must be alphanumeric/space/_/./- only',
  })
  device_name: string;
}
```

- [ ] **Step 2: Type-check**

Run: `cd backend && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/src/auth/dto/mint-extension-token.dto.ts
git commit -m "feat(backend): add MintExtensionTokenDto"
```

---

## Task 1.5: Controller — mint, list, revoke

**Files:**
- Create: `backend/src/auth/extension-token.controller.ts`
- Create: `backend/src/auth/extension-token.controller.spec.ts`

- [ ] **Step 1: Write the failing test**

```typescript
// backend/src/auth/extension-token.controller.spec.ts
import { Test } from '@nestjs/testing';
import { ExtensionTokenController } from './extension-token.controller';
import { ExtensionTokenService } from './extension-token.service';

describe('ExtensionTokenController', () => {
  let controller: ExtensionTokenController;
  let service: jest.Mocked<ExtensionTokenService>;

  beforeEach(async () => {
    const serviceMock = {
      mint: jest.fn(),
      list: jest.fn(),
      revoke: jest.fn(),
    } as any;

    const module = await Test.createTestingModule({
      controllers: [ExtensionTokenController],
      providers: [{ provide: ExtensionTokenService, useValue: serviceMock }],
    }).compile();

    controller = module.get(ExtensionTokenController);
    service = module.get(ExtensionTokenService);
  });

  it('mint returns the plaintext token (only time it is exposed)', async () => {
    service.mint.mockResolvedValue({ token: 'dr_ext_abc', id: 'tok-1' });
    const req = { user: { id: 'user-1' } };
    const result = await controller.mint(req, { device_id: 'd1-uuid', device_name: 'iPhone' });
    expect(result).toEqual({ token: 'dr_ext_abc', id: 'tok-1' });
    expect(service.mint).toHaveBeenCalledWith('user-1', 'd1-uuid', 'iPhone');
  });

  it('list returns rows for current user, never exposes token_hash', async () => {
    service.list.mockResolvedValue([
      {
        id: 'tok-1',
        user_id: 'user-1',
        token_hash: 'secret',
        scopes: ['rewrite'],
        device_id: 'd1',
        device_name: 'iPhone',
        last_used_at: null,
        created_at: new Date('2026-05-02'),
        revoked_at: null,
      } as any,
    ]);
    const result = await controller.list({ user: { id: 'user-1' } });
    expect(result[0]).not.toHaveProperty('token_hash');
    expect(result[0]).toMatchObject({ id: 'tok-1', device_name: 'iPhone' });
  });

  it('revoke calls service with current user id and token id', async () => {
    await controller.revoke({ user: { id: 'user-1' } }, 'tok-1');
    expect(service.revoke).toHaveBeenCalledWith('user-1', 'tok-1');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && npx jest src/auth/extension-token.controller.spec.ts`
Expected: FAIL — controller cannot be imported.

- [ ] **Step 3: Write the controller**

```typescript
// backend/src/auth/extension-token.controller.ts
import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  Param,
  Post,
  Req,
  UseGuards,
} from '@nestjs/common';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { JwtAuthGuard } from './jwt-auth.guard';
import { MintExtensionTokenDto } from './dto/mint-extension-token.dto';
import { ExtensionTokenService } from './extension-token.service';

@ApiTags('auth')
@ApiBearerAuth()
@UseGuards(JwtAuthGuard)
@Controller('auth/extension-tokens')
export class ExtensionTokenController {
  constructor(private readonly service: ExtensionTokenService) {}

  @Post()
  @HttpCode(200)
  async mint(@Req() req: any, @Body() dto: MintExtensionTokenDto) {
    return this.service.mint(req.user.id, dto.device_id, dto.device_name);
  }

  @Get()
  async list(@Req() req: any) {
    const rows = await this.service.list(req.user.id);
    return rows.map(({ token_hash, user_id, ...rest }) => rest);
  }

  @Delete(':id')
  @HttpCode(204)
  async revoke(@Req() req: any, @Param('id') id: string) {
    await this.service.revoke(req.user.id, id);
  }
}
```

Note: `@HttpCode(200)` on POST mirrors the project convention from the customer-registration work — the macOS client treats 201 as a non-success on auto-refresh paths. See `feedback_nest_post_status.md`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && npx jest src/auth/extension-token.controller.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/src/auth/extension-token.controller.ts backend/src/auth/extension-token.controller.spec.ts
git commit -m "feat(backend): add ExtensionTokenController endpoints"
```

---

## Task 1.6: Rewrite auth guard — accept JWT or extension token

**Files:**
- Create: `backend/src/auth/rewrite-auth.guard.ts`
- Create: `backend/src/auth/rewrite-auth.guard.spec.ts`

- [ ] **Step 1: Write the failing test**

```typescript
// backend/src/auth/rewrite-auth.guard.spec.ts
import { ExecutionContext, UnauthorizedException } from '@nestjs/common';
import { RewriteAuthGuard } from './rewrite-auth.guard';
import { ExtensionTokenService } from './extension-token.service';
import { JwtAuthGuard } from './jwt-auth.guard';

function makeCtx(headers: Record<string, string>): ExecutionContext {
  const req: any = { headers };
  return {
    switchToHttp: () => ({ getRequest: () => req }),
  } as any;
}

describe('RewriteAuthGuard', () => {
  let extService: jest.Mocked<ExtensionTokenService>;
  let jwtGuard: jest.Mocked<JwtAuthGuard>;
  let guard: RewriteAuthGuard;

  beforeEach(() => {
    extService = { validate: jest.fn() } as any;
    jwtGuard = { canActivate: jest.fn() } as any;
    guard = new RewriteAuthGuard(extService, jwtGuard);
  });

  it('rejects requests with no Authorization header', async () => {
    const ctx = makeCtx({});
    await expect(guard.canActivate(ctx)).rejects.toThrow(UnauthorizedException);
  });

  it('uses extension-token path when Authorization is dr_ext_*', async () => {
    extService.validate.mockResolvedValue({
      tokenId: 'tok-1',
      userId: 'user-1',
      scopes: ['rewrite'],
    });
    const ctx = makeCtx({ authorization: 'Bearer dr_ext_abc' });
    const result = await guard.canActivate(ctx);
    expect(result).toBe(true);
    const req = ctx.switchToHttp().getRequest();
    expect(req.user).toEqual({
      id: 'user-1',
      email: '',
      role: '',
      isAdmin: false,
      via: 'extension_token',
      tokenId: 'tok-1',
    });
    expect(jwtGuard.canActivate).not.toHaveBeenCalled();
  });

  it('rejects extension token without rewrite scope', async () => {
    extService.validate.mockResolvedValue({
      tokenId: 'tok-1',
      userId: 'user-1',
      scopes: ['something-else'],
    });
    const ctx = makeCtx({ authorization: 'Bearer dr_ext_abc' });
    await expect(guard.canActivate(ctx)).rejects.toThrow(UnauthorizedException);
  });

  it('rejects invalid extension token', async () => {
    extService.validate.mockResolvedValue(null);
    const ctx = makeCtx({ authorization: 'Bearer dr_ext_bogus' });
    await expect(guard.canActivate(ctx)).rejects.toThrow(UnauthorizedException);
  });

  it('falls through to JWT guard for non-prefixed tokens', async () => {
    jwtGuard.canActivate.mockResolvedValue(true);
    const ctx = makeCtx({ authorization: 'Bearer regular.jwt.value' });
    const result = await guard.canActivate(ctx);
    expect(result).toBe(true);
    expect(jwtGuard.canActivate).toHaveBeenCalledWith(ctx);
    expect(extService.validate).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && npx jest src/auth/rewrite-auth.guard.spec.ts`
Expected: FAIL — guard not implemented.

- [ ] **Step 3: Write the guard**

```typescript
// backend/src/auth/rewrite-auth.guard.ts
import {
  CanActivate,
  ExecutionContext,
  Injectable,
  UnauthorizedException,
} from '@nestjs/common';
import { ExtensionTokenService } from './extension-token.service';
import { JwtAuthGuard } from './jwt-auth.guard';

const REQUIRED_SCOPE = 'rewrite';

@Injectable()
export class RewriteAuthGuard implements CanActivate {
  constructor(
    private readonly extService: ExtensionTokenService,
    private readonly jwtGuard: JwtAuthGuard,
  ) {}

  async canActivate(context: ExecutionContext): Promise<boolean> {
    const req = context.switchToHttp().getRequest();
    const header = req.headers.authorization;
    if (!header || !header.startsWith('Bearer ')) {
      throw new UnauthorizedException('Missing bearer token');
    }
    const token = header.slice('Bearer '.length);

    if (token.startsWith('dr_ext_')) {
      const validated = await this.extService.validate(token);
      if (!validated) throw new UnauthorizedException('Invalid extension token');
      if (!validated.scopes.includes(REQUIRED_SCOPE)) {
        throw new UnauthorizedException('Token missing rewrite scope');
      }
      // Match the shape downstream code expects from JwtStrategy.validate.
      req.user = {
        id: validated.userId,
        email: '',
        role: '',
        isAdmin: false,
        via: 'extension_token',
        tokenId: validated.tokenId,
      };
      return true;
    }

    return (await this.jwtGuard.canActivate(context)) as boolean;
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && npx jest src/auth/rewrite-auth.guard.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/src/auth/rewrite-auth.guard.ts backend/src/auth/rewrite-auth.guard.spec.ts
git commit -m "feat(backend): add RewriteAuthGuard accepting JWT or extension token"
```

---

## Task 1.7: Wire up the module

**Files:**
- Modify: `backend/src/auth/auth.module.ts`

- [ ] **Step 1: Open `backend/src/auth/auth.module.ts` and add the new pieces**

Add the imports at the top:

```typescript
import { TypeOrmModule } from '@nestjs/typeorm';
import { ExtensionToken } from './extension-token.entity';
import { ExtensionTokenService } from './extension-token.service';
import { ExtensionTokenController } from './extension-token.controller';
import { RewriteAuthGuard } from './rewrite-auth.guard';
```

In the `@Module({...})` decorator:

- Add `TypeOrmModule.forFeature([ExtensionToken])` to `imports` (alongside existing entities; if there's no existing TypeOrmModule.forFeature here, add it).
- Add `ExtensionTokenController` to `controllers`.
- Add `ExtensionTokenService`, `RewriteAuthGuard` to `providers`.
- Add `ExtensionTokenService`, `RewriteAuthGuard` to `exports` so the rewrite module can use the guard.

- [ ] **Step 2: Type-check**

Run: `cd backend && npx tsc --noEmit`
Expected: no new errors.

- [ ] **Step 3: Boot smoke test**

Run: `cd backend && npm run start:dev`
Watch logs for ~10 seconds. Expected: app boots without errors. Stop with Ctrl-C.

- [ ] **Step 4: Commit**

```bash
git add backend/src/auth/auth.module.ts
git commit -m "feat(backend): wire ExtensionToken pieces into AuthModule"
```

---

## Task 1.8: Switch rewrite endpoint to RewriteAuthGuard

**Files:**
- Modify: `backend/src/rewrite/rewrite.controller.ts`
- Modify: `backend/src/rewrite/rewrite.module.ts` (if needed for AuthModule import)

- [ ] **Step 1: Update the rewrite controller**

Open `backend/src/rewrite/rewrite.controller.ts`. Change:

```typescript
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
// ...
@UseGuards(JwtAuthGuard)
@ApiBearerAuth()
@Post()
async rewrite(@Req() req: any, @Body() dto: RewriteDto) {
  return this.rewriteService.rewrite(req.user.id, dto.text, dto.tone, dto.target_language, dto.source_language);
}
```

to:

```typescript
import { RewriteAuthGuard } from '../auth/rewrite-auth.guard';
// ...
@UseGuards(RewriteAuthGuard)
@ApiBearerAuth()
@Post()
async rewrite(@Req() req: any, @Body() dto: RewriteDto) {
  return this.rewriteService.rewrite(req.user.id, dto.text, dto.tone, dto.target_language, dto.source_language);
}
```

- [ ] **Step 2: Ensure RewriteModule imports AuthModule**

Open `backend/src/rewrite/rewrite.module.ts`. If it doesn't already import `AuthModule`, add:

```typescript
import { AuthModule } from '../auth/auth.module';

@Module({
  imports: [/* existing imports */, AuthModule],
  // ...
})
```

- [ ] **Step 3: Type-check + boot**

Run: `cd backend && npx tsc --noEmit && npm run start:dev`
Expected: clean type check; app boots. Stop with Ctrl-C.

- [ ] **Step 4: End-to-end smoke test against running backend**

While `start:dev` is running, in another shell:

```bash
# Get a regular JWT (existing flow)
JWT=$(curl -s -X POST http://localhost:3000/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"<existing-test-user>","password":"<password>"}' | jq -r .access_token)

# Verify rewrite still works with regular JWT
curl -s -X POST http://localhost:3000/rewrite \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -d '{"text":"hello there","tone":"professional"}'

# Mint an extension token
EXT=$(curl -s -X POST http://localhost:3000/auth/extension-tokens \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -d '{"device_id":"00000000-0000-0000-0000-000000000001","device_name":"plan-test"}' \
  | jq -r .token)
echo "EXT=$EXT"  # should start with dr_ext_

# Verify rewrite works with extension token
curl -s -X POST http://localhost:3000/rewrite \
  -H "Authorization: Bearer $EXT" \
  -H 'Content-Type: application/json' \
  -d '{"text":"hello there","tone":"casual"}'

# Verify a non-rewrite endpoint REJECTS the extension token
curl -s -o /dev/null -w '%{http_code}\n' -X GET http://localhost:3000/auth/me \
  -H "Authorization: Bearer $EXT"
# Expected: 401
```

Expected: rewrite calls return JSON; the `/auth/me` call returns `401`.

- [ ] **Step 5: Commit**

```bash
git add backend/src/rewrite/rewrite.controller.ts backend/src/rewrite/rewrite.module.ts
git commit -m "feat(backend): /rewrite accepts JWT or extension token"
```

---

## Task 1.9: Backend stage closeout

- [ ] **Step 1: Run the full unit test suite**

Run: `cd backend && npx jest`
Expected: all auth-related tests pass.

- [ ] **Step 2: Document the new endpoints in the existing API docs**

If `backend/CLAUDE.md` documents endpoints, add:

```
POST   /auth/extension-tokens         (mint or rotate) — body { device_id, device_name }
GET    /auth/extension-tokens         (list active tokens for user) — no plaintext
DELETE /auth/extension-tokens/:id     (revoke a single token)
```

- [ ] **Step 3: Commit docs**

```bash
git add backend/CLAUDE.md
git commit -m "docs(backend): document extension-token endpoints"
```

**Stage 1 complete. Backend now mints, validates, and accepts extension tokens. No client changes have been made yet — existing access-JWT path through extensions continues to work unchanged.**

---

# Stage 2 — iOS keychain infrastructure

This stage exists because **storing a 90-day token in `UserDefaults` is unacceptable** — `UserDefaults` is an unencrypted plist on disk. We move shared-token storage to the App Group keychain via a `keychain-access-groups` entitlement shared across all three iOS targets.

## Task 2.1: Add keychain-access-groups entitlement to all three targets

**Files:**
- Modify: `DraftRightMobile/ios/Runner/Runner.entitlements`
- Modify: `DraftRightMobile/ios/DraftRightKeyboard/DraftRightKeyboard.entitlements`
- Modify: `DraftRightMobile/ios/DraftRightAction/DraftRightAction.entitlements`

- [ ] **Step 1: Update Runner.entitlements**

Replace the file contents with:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>com.apple.security.application-groups</key>
	<array>
		<string>group.com.draftright.v2</string>
	</array>
	<key>keychain-access-groups</key>
	<array>
		<string>$(AppIdentifierPrefix)com.draftright.v2.shared</string>
	</array>
</dict>
</plist>
```

- [ ] **Step 2: Update DraftRightKeyboard.entitlements**

Same change — replace contents with the identical XML above.

- [ ] **Step 3: Update DraftRightAction.entitlements**

Same change.

- [ ] **Step 4: Apple Developer portal — register the keychain access group**

Manual step (cannot be done from CLI):
- Visit https://developer.apple.com/account → Identifiers → App IDs.
- For each of the three App IDs (`com.draftright.draftrightMobile.v2`, `...v2.DraftRightKeyboard`, `...v2.DraftRightAction`), enable the **Keychain Sharing** capability and add the access group `com.draftright.v2.shared`.
- Regenerate provisioning profiles for all three.

- [ ] **Step 5: Verify Xcode build**

Run: `cd DraftRightMobile && flutter build ios --no-codesign`
Expected: build succeeds. Codesign is skipped because we don't yet have updated profiles in the build environment.

- [ ] **Step 6: Commit**

```bash
git -C DraftRightMobile add ios/Runner/Runner.entitlements ios/DraftRightKeyboard/DraftRightKeyboard.entitlements ios/DraftRightAction/DraftRightAction.entitlements
git -C DraftRightMobile commit -m "feat(ios): add keychain-access-groups entitlement for shared token storage"
```

---

## Task 2.2: Create SharedKeychain.swift

**Files:**
- Create: `DraftRightMobile/ios/Shared/SharedKeychain.swift`

- [ ] **Step 1: Create the directory if needed**

Run: `mkdir -p /opt/openAi/DraftRight/DraftRightMobile/ios/Shared`

- [ ] **Step 2: Write the file**

```swift
// DraftRightMobile/ios/Shared/SharedKeychain.swift
import Foundation
import Security

/// A thin wrapper over the Keychain that uses a shared access group so the
/// main app and the keyboard/action extensions all read/write the same items.
///
/// Add this file to the Runner, DraftRightKeyboard, and DraftRightAction
/// targets in Xcode (target membership checkboxes). Do not duplicate.
public enum SharedKeychain {
    public static let accessGroup = "com.draftright.v2.shared"
    private static let service = "com.draftright.v2"

    @discardableResult
    public static func set(_ key: String, _ value: String?) -> Bool {
        guard let value = value else {
            return delete(key)
        }
        let data = Data(value.utf8)

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key,
            kSecAttrAccessGroup as String: accessGroup,
        ]

        let attributes: [String: Any] = [
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlock,
        ]

        let updateStatus = SecItemUpdate(query as CFDictionary, attributes as CFDictionary)
        if updateStatus == errSecSuccess { return true }

        var insert = query
        insert[kSecValueData as String] = data
        insert[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlock
        let addStatus = SecItemAdd(insert as CFDictionary, nil)
        return addStatus == errSecSuccess
    }

    public static func get(_ key: String) -> String? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key,
            kSecAttrAccessGroup as String: accessGroup,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne,
        ]
        var ref: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &ref)
        guard status == errSecSuccess, let data = ref as? Data else { return nil }
        return String(data: data, encoding: .utf8)
    }

    @discardableResult
    public static func delete(_ key: String) -> Bool {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key,
            kSecAttrAccessGroup as String: accessGroup,
        ]
        let status = SecItemDelete(query as CFDictionary)
        return status == errSecSuccess || status == errSecItemNotFound
    }
}
```

- [ ] **Step 3: Add target membership in Xcode**

Manual step. Open `DraftRightMobile/ios/Runner.xcworkspace` in Xcode:
- Select `Shared/SharedKeychain.swift` in the navigator.
- In the File Inspector pane (right side), check **Target Membership** for: Runner, DraftRightKeyboard, DraftRightAction.
- Save the project (Cmd-S).

- [ ] **Step 4: Build to confirm linker is happy across all targets**

Run: `cd DraftRightMobile && flutter build ios --no-codesign`
Expected: all three targets compile.

- [ ] **Step 5: Commit**

```bash
git -C DraftRightMobile add ios/Shared/SharedKeychain.swift ios/Runner.xcodeproj/project.pbxproj
git -C DraftRightMobile commit -m "feat(ios): add SharedKeychain helper for App Group token storage"
```

---

## Task 2.3: Extend AppDelegate platform channel with keychain methods

**Files:**
- Modify: `DraftRightMobile/ios/Runner/AppDelegate.swift`

- [ ] **Step 1: Add keychain methods to the existing channel handler**

Open `DraftRightMobile/ios/Runner/AppDelegate.swift`. The existing handler has a `switch call.method` for `set` / `get`. Extend it with the three new cases. The full updated `setMethodCallHandler` block is:

```swift
channel.setMethodCallHandler { (call, result) in
  switch call.method {
  case "set":
    if let args = call.arguments as? [String: Any],
       let key = args["key"] as? String {
      if let value = args["value"] as? String {
        defaults?.set(value, forKey: key)
      } else {
        defaults?.removeObject(forKey: key)
      }
      defaults?.synchronize()
      result(true)
    } else {
      result(FlutterError(code: "INVALID_ARGS", message: "key required", details: nil))
    }
  case "get":
    if let args = call.arguments as? [String: Any],
       let key = args["key"] as? String {
      result(defaults?.string(forKey: key))
    } else {
      result(FlutterError(code: "INVALID_ARGS", message: "key required", details: nil))
    }
  case "setKeychain":
    if let args = call.arguments as? [String: Any],
       let key = args["key"] as? String {
      let value = args["value"] as? String
      result(SharedKeychain.set(key, value))
    } else {
      result(FlutterError(code: "INVALID_ARGS", message: "key required", details: nil))
    }
  case "getKeychain":
    if let args = call.arguments as? [String: Any],
       let key = args["key"] as? String {
      result(SharedKeychain.get(key))
    } else {
      result(FlutterError(code: "INVALID_ARGS", message: "key required", details: nil))
    }
  case "deleteKeychain":
    if let args = call.arguments as? [String: Any],
       let key = args["key"] as? String {
      result(SharedKeychain.delete(key))
    } else {
      result(FlutterError(code: "INVALID_ARGS", message: "key required", details: nil))
    }
  default:
    result(FlutterMethodNotImplemented)
  }
}
```

- [ ] **Step 2: Build to verify**

Run: `cd DraftRightMobile && flutter build ios --no-codesign`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git -C DraftRightMobile add ios/Runner/AppDelegate.swift
git -C DraftRightMobile commit -m "feat(ios): expose keychain set/get/delete via app_group MethodChannel"
```

**Stage 2 complete. iOS now has shared keychain infrastructure available, but nothing reads or writes through it yet.**

---

# Stage 3 — Flutter mint flow

## Task 3.1: ExtensionTokenService

**Files:**
- Create: `DraftRightMobile/lib/services/extension_token_service.dart`
- Create: `DraftRightMobile/test/services/extension_token_service_test.dart`

- [ ] **Step 1: Write the failing test**

```dart
// DraftRightMobile/test/services/extension_token_service_test.dart
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:draftright_mobile/services/extension_token_service.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() async {
    SharedPreferences.setMockInitialValues({});
  });

  test('deviceId is generated once and persisted', () async {
    final svc = ExtensionTokenService(baseUrl: 'http://localhost:3000');
    final first = await svc.deviceId();
    final second = await svc.deviceId();
    expect(first, equals(second));
    expect(first, matches(RegExp(r'^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$')));
  });

  test('storeToken writes to SharedPreferences', () async {
    final svc = ExtensionTokenService(baseUrl: 'http://localhost:3000');
    await svc.storeToken('dr_ext_abc');
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString('flutter.draftright.extensionToken'), 'dr_ext_abc');
  });

  test('clearToken removes the token from SharedPreferences', () async {
    final svc = ExtensionTokenService(baseUrl: 'http://localhost:3000');
    await svc.storeToken('dr_ext_abc');
    await svc.clearToken();
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString('flutter.draftright.extensionToken'), isNull);
  });
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd DraftRightMobile && flutter test test/services/extension_token_service_test.dart`
Expected: FAIL — `extension_token_service.dart` doesn't exist.

- [ ] **Step 3: Write the service**

```dart
// DraftRightMobile/lib/services/extension_token_service.dart
import 'dart:convert';
import 'dart:io' show Platform;
import 'dart:math';

import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

import 'logger.dart';

class ExtensionTokenService {
  ExtensionTokenService({required String baseUrl}) : _baseUrl = baseUrl;

  static const _channel = MethodChannel('com.draftright.v2/app_group');
  static const _kDeviceId = 'draftright.deviceId';
  static const _kExtensionToken = 'draftright.extensionToken';
  static const _kSharedPrefsExtensionToken = 'draftright.extensionToken';

  final String _baseUrl;

  Future<String> deviceId() async {
    final prefs = await SharedPreferences.getInstance();
    var id = prefs.getString(_kDeviceId);
    if (id != null && id.isNotEmpty) return id;
    id = _uuidv4();
    await prefs.setString(_kDeviceId, id);
    return id;
  }

  /// Mint an extension token from the backend using the user's session JWT,
  /// then persist it to all relevant shared stores so the extensions can read it.
  Future<void> ensureMinted({
    required String accessToken,
  }) async {
    final id = await deviceId();
    final name = _deviceName();

    final response = await http
        .post(
          Uri.parse('$_baseUrl/auth/extension-tokens'),
          headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer $accessToken',
          },
          body: jsonEncode({'device_id': id, 'device_name': name}),
        )
        .timeout(const Duration(seconds: 15));

    if (response.statusCode >= 400) {
      DRLogger.log(
        'Mint extension token failed: ${response.statusCode} ${response.body}',
        category: 'AUTH',
      );
      return;
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    final token = data['token'] as String;
    await storeToken(token);
    DRLogger.log('Extension token minted and stored', category: 'AUTH');
  }

  /// Revoke the current extension token server-side, then clear local copies.
  Future<void> revoke({
    required String accessToken,
    required String tokenId,
  }) async {
    try {
      await http
          .delete(
            Uri.parse('$_baseUrl/auth/extension-tokens/$tokenId'),
            headers: {'Authorization': 'Bearer $accessToken'},
          )
          .timeout(const Duration(seconds: 10));
    } catch (e) {
      DRLogger.log('Revoke extension token failed: $e', category: 'AUTH');
    }
    await clearToken();
  }

  Future<void> storeToken(String token) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_kSharedPrefsExtensionToken, token);
    await _syncToKeychain(_kExtensionToken, token);
  }

  Future<void> clearToken() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_kSharedPrefsExtensionToken);
    await _syncToKeychain(_kExtensionToken, null);
  }

  Future<void> _syncToKeychain(String key, String? value) async {
    if (!Platform.isIOS) return;
    try {
      if (value == null) {
        await _channel.invokeMethod('deleteKeychain', {'key': key});
      } else {
        await _channel.invokeMethod('setKeychain', {'key': key, 'value': value});
      }
    } catch (_) {
      // Channel not available (web/desktop test runs).
    }
  }

  String _deviceName() {
    if (Platform.isIOS) return 'iOS';
    if (Platform.isAndroid) return 'Android';
    return 'Mobile';
  }

  String _uuidv4() {
    final rng = Random.secure();
    final bytes = List<int>.generate(16, (_) => rng.nextInt(256));
    bytes[6] = (bytes[6] & 0x0f) | 0x40; // version 4
    bytes[8] = (bytes[8] & 0x3f) | 0x80; // variant
    String hex(int b) => b.toRadixString(16).padLeft(2, '0');
    final s = bytes.map(hex).join();
    return '${s.substring(0, 8)}-${s.substring(8, 12)}-${s.substring(12, 16)}-${s.substring(16, 20)}-${s.substring(20)}';
  }
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd DraftRightMobile && flutter test test/services/extension_token_service_test.dart`
Expected: PASS — all three assertions green. (Network calls are not exercised in this test file; mint/revoke are tested via Stage 5 manual verification.)

- [ ] **Step 5: Commit**

```bash
git -C DraftRightMobile add lib/services/extension_token_service.dart test/services/extension_token_service_test.dart
git -C DraftRightMobile commit -m "feat(mobile): add ExtensionTokenService for mint/store/clear"
```

---

## Task 3.2: Wire ExtensionTokenService into AuthService

**Files:**
- Modify: `DraftRightMobile/lib/services/auth_service.dart`

- [ ] **Step 1: Open auth_service.dart and add the field + method calls**

In the `AuthService` class, near the top where `_keyAccess` etc. are declared, add:

```dart
late final ExtensionTokenService _extension =
    ExtensionTokenService(baseUrl: _baseUrl);
```

Add the import at the top of the file:

```dart
import 'extension_token_service.dart';
```

Update `_storeTokens` (currently lines 217-227). After `notifyListeners();` add an unawaited mint:

```dart
// Mint or rotate the extension token in the background. Failures here
// must not block login — extensions will fall back to the access JWT.
unawaited(_extension.ensureMinted(accessToken: access));
```

Add the import for `unawaited` at the top:

```dart
import 'dart:async';
```

Update `logout` (currently lines 155-164). After clearing the access token tracks, add:

```dart
// Best-effort revoke. Continue even if this fails.
await _extension.clearToken();
```

(We don't call the backend revoke here because we no longer have a token ID to address; we simply clear the client state. The backend row goes inert — its hash is never presented again. Consider a `revokeAll` endpoint as a follow-up.)

- [ ] **Step 2: Type-check by analyzing the file**

Run: `cd DraftRightMobile && flutter analyze lib/services/auth_service.dart`
Expected: no new errors.

- [ ] **Step 3: Commit**

```bash
git -C DraftRightMobile add lib/services/auth_service.dart
git -C DraftRightMobile commit -m "feat(mobile): mint extension token on login, clear on logout"
```

**Stage 3 complete. Main app now mints + persists an extension token whenever a user logs in. Nothing reads it yet.**

---

# Stage 4 — Extensions read the new token

## Task 4.1: iOS DraftRightKeyboard reads extension token

**Files:**
- Modify: `DraftRightMobile/ios/DraftRightKeyboard/SharedSettings.swift`
- Modify: `DraftRightMobile/ios/DraftRightKeyboard/BackendClient.swift`

- [ ] **Step 1: Add `extensionToken` getter to SharedSettings**

Replace the file content with:

```swift
// DraftRightMobile/ios/DraftRightKeyboard/SharedSettings.swift
import Foundation

struct SharedSettings {
    private let defaults: UserDefaults?

    init() {
        defaults = UserDefaults(suiteName: "group.com.draftright.v2")
    }

    /// Long-lived extension token (preferred). Stored in App Group keychain.
    var extensionToken: String {
        SharedKeychain.get("draftright.extensionToken") ?? ""
    }

    /// Short-lived user JWT (legacy fallback). Will be removed in a follow-up
    /// release once everyone has launched the new main app version at least once.
    var accessToken: String {
        defaults?.string(forKey: "draftright.accessToken") ?? ""
    }

    /// The token to actually present in Authorization headers.
    var bearerToken: String {
        let ext = extensionToken
        return ext.isEmpty ? accessToken : ext
    }

    var backendUrl: String {
        #if DEBUG
        return defaults?.string(forKey: "draftright.backendUrl") ?? "http://localhost:3000"
        #else
        return defaults?.string(forKey: "draftright.backendUrl") ?? "https://api.draftright.info"
        #endif
    }

    var translateLanguage: String {
        defaults?.string(forKey: "draftright.translateLanguage") ?? "Vietnamese"
    }
}
```

- [ ] **Step 2: Update BackendClient to use bearerToken**

In `DraftRightMobile/ios/DraftRightKeyboard/BackendClient.swift`, in the `rewrite(text:tone:settings:completion:)` method, change:

```swift
let accessToken = settings.accessToken
guard !accessToken.isEmpty else {
    completion(.failure(NSError(
        domain: "BackendClient", code: -1,
        userInfo: [NSLocalizedDescriptionKey: "Please login in DraftRight app"])))
    return
}
```

to:

```swift
let bearerToken = settings.bearerToken
guard !bearerToken.isEmpty else {
    completion(.failure(NSError(
        domain: "BackendClient", code: -1,
        userInfo: [NSLocalizedDescriptionKey: "Please login in DraftRight app"])))
    return
}
```

And further down change:

```swift
request.addValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
```

to:

```swift
request.addValue("Bearer \(bearerToken)", forHTTPHeaderField: "Authorization")
```

- [ ] **Step 3: Build the keyboard target**

Run: `cd DraftRightMobile && flutter build ios --no-codesign`
Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
git -C DraftRightMobile add ios/DraftRightKeyboard/SharedSettings.swift ios/DraftRightKeyboard/BackendClient.swift
git -C DraftRightMobile commit -m "feat(ios-keyboard): prefer extension token from keychain"
```

---

## Task 4.2: iOS DraftRightAction reads extension token

**Files:**
- Modify: `DraftRightMobile/ios/DraftRightAction/SharedSettings.swift`
- Modify: `DraftRightMobile/ios/DraftRightAction/ActionViewController.swift`
- Modify: `DraftRightMobile/ios/DraftRightAction/BackendClient.swift` (if there's a separate one)

- [ ] **Step 1: Update SharedSettings.swift**

Apply the same change as Task 4.1 Step 1 to `DraftRightMobile/ios/DraftRightAction/SharedSettings.swift`. The file content should be identical.

- [ ] **Step 2: Update the auth check in ActionViewController**

In `DraftRightMobile/ios/DraftRightAction/ActionViewController.swift`, find the existing check (around line 221):

```swift
if settings.accessToken.isEmpty {
  showError("Please login in the DraftRight app first")
  return
}
```

Change `accessToken` to `bearerToken`:

```swift
if settings.bearerToken.isEmpty {
  showError("Please login in the DraftRight app first")
  return
}
```

- [ ] **Step 3: Update BackendClient if separate from keyboard's**

If `DraftRightMobile/ios/DraftRightAction/BackendClient.swift` exists as a separate file, apply the same `accessToken → bearerToken` substitution as Task 4.1 Step 2. If the action target shares the keyboard's BackendClient via Xcode target membership, no further change needed.

- [ ] **Step 4: Build**

Run: `cd DraftRightMobile && flutter build ios --no-codesign`
Expected: build succeeds.

- [ ] **Step 5: Commit**

```bash
git -C DraftRightMobile add ios/DraftRightAction/
git -C DraftRightMobile commit -m "feat(ios-action): prefer extension token from keychain"
```

---

## Task 4.3: Android keyboard reads extension token

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/SharedSettings.kt`
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/BackendClient.kt`

- [ ] **Step 1: Update SharedSettings.kt**

Replace contents with:

```kotlin
package com.draftright.keyboard

import android.content.Context
import android.content.SharedPreferences

class SharedSettings(context: Context) {
    private val prefs: SharedPreferences =
        context.getSharedPreferences("FlutterSharedPreferences", Context.MODE_PRIVATE)

    /** Long-lived extension token (preferred). */
    val extensionToken: String
        get() = prefs.getString("flutter.draftright.extensionToken", "") ?: ""

    /** Short-lived user JWT (legacy fallback). */
    val accessToken: String
        get() = prefs.getString("flutter.draftright.accessToken", "") ?: ""

    /** The token to actually present in Authorization headers. */
    val bearerToken: String
        get() = if (extensionToken.isNotEmpty()) extensionToken else accessToken

    val backendUrl: String
        get() = prefs.getString("flutter.draftright.backendUrl", "https://api.draftright.info")
            ?: "https://api.draftright.info"

    val translateLanguage: String
        get() = prefs.getString("flutter.draftright.translateLanguage", "Vietnamese") ?: "Vietnamese"
}
```

- [ ] **Step 2: Update BackendClient.kt**

In `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/BackendClient.kt`, change:

```kotlin
val accessToken = settings.accessToken
if (accessToken.isEmpty()) {
    onResult(Result.failure(Exception("Please login in DraftRight app")))
    return@thread
}
```

to:

```kotlin
val bearerToken = settings.bearerToken
if (bearerToken.isEmpty()) {
    onResult(Result.failure(Exception("Please login in DraftRight app")))
    return@thread
}
```

And change:

```kotlin
conn.setRequestProperty("Authorization", "Bearer $accessToken")
```

to:

```kotlin
conn.setRequestProperty("Authorization", "Bearer $bearerToken")
```

- [ ] **Step 3: Build the Android app**

Run: `cd DraftRightMobile && flutter build apk --debug`
Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
git -C DraftRightMobile add android/app/src/main/kotlin/com/draftright/keyboard/
git -C DraftRightMobile commit -m "feat(android-keyboard): prefer extension token from SharedPreferences"
```

**Stage 4 complete. All three extensions now prefer the long-lived extension token, falling back to the access JWT if the new token isn't present yet.**

---

# Stage 5 — End-to-end verification

There are no automated tests for the iOS or Android extensions in this codebase. This stage is a deliberate, structured manual test plan that must produce evidence of every claim.

## Task 5.1: Write the verification record file

**Files:**
- Create: `docs/superpowers/plans/2026-05-02-extension-tokens-verification.md`

- [ ] **Step 1: Create the file with the matrix template**

```markdown
# Extension Tokens — Verification Record

For each row, after performing the test, fill in: PASS / FAIL, the date/time,
and a 1-line note. If FAIL, include the observed error.

## A. Backend dual-accept (run with backend on dev machine)

| # | Test | Result |
|---|------|--------|
| A1 | `POST /rewrite` with valid user JWT returns rewritten text | |
| A2 | `POST /auth/extension-tokens` with user JWT returns `{ token, id }` and token starts with `dr_ext_` | |
| A3 | `POST /rewrite` with extension token returns rewritten text | |
| A4 | `GET /auth/me` with extension token returns 401 | |
| A5 | `GET /auth/extension-tokens` with user JWT lists rows; `token_hash` field is NOT present in JSON | |
| A6 | `DELETE /auth/extension-tokens/:id` with user JWT returns 204 | |
| A7 | After A6, `POST /rewrite` with the revoked extension token returns 401 | |
| A8 | Re-mint with same `device_id` invalidates the old token (re-running A3 with the old token returns 401) | |

## B. iOS — fresh install, real device

| # | Test | Result |
|---|------|--------|
| B1 | Install new build, log in with test account | |
| B2 | After login, the keyboard's first rewrite call succeeds within 5 minutes | |
| B3 | Wait 30 minutes (longer than access JWT TTL of 15m), then invoke keyboard rewrite — it succeeds | |
| B4 | Wait 30 minutes, then invoke share extension — it succeeds | |
| B5 | After 24 hours of no main-app activity, keyboard still works | |
| B6 | Log out in main app — keyboard rewrite shows "Please login in DraftRight app" | |
| B7 | Log back in — keyboard works again on first attempt (no need to "warm" by visiting playground) | |

## C. Android — fresh install, real device

| # | Test | Result |
|---|------|--------|
| C1 | Install new build, log in with test account | |
| C2 | After login, the IME's first rewrite call succeeds within 5 minutes | |
| C3 | Wait 30 minutes, then invoke IME rewrite — it succeeds | |
| C4 | After 24 hours of no main-app activity, IME still works | |
| C5 | Log out in main app — IME rewrite shows the auth-required error | |
| C6 | Log back in — IME works again on first attempt | |

## D. Migration safety (existing user upgrades from old build)

| # | Test | Result |
|---|------|--------|
| D1 | Install OLD build, log in. Confirm keyboard works. Note: old access token is in shared storage. | |
| D2 | Install new build OVER the old install (no fresh install). Do NOT open main app. | |
| D3 | Invoke keyboard. Within the access-JWT lifetime, it should still work via the fallback path. | |
| D4 | Open main app once. ExtensionTokenService mints. Confirm via Charles/mitmproxy that mint endpoint was called. | |
| D5 | Wait 30 minutes. Keyboard now uses the extension token (Authorization header starts with `Bearer dr_ext_`). | |
```

- [ ] **Step 2: Run all tests in matrix A** (backend tests can be done immediately)

Mark each row with the result. Tests A1–A8 should all PASS at this point.

- [ ] **Step 3: Run matrix B and C against TestFlight / internal Android distribution**

These require deploying the new build. If you have not yet deployed the new mobile build, mark these rows as "pending deployment" and circle back.

- [ ] **Step 4: Run matrix D**

Requires keeping a copy of the previous main-app build for the upgrade test.

- [ ] **Step 5: Commit the verification record (whatever rows are filled)**

```bash
git add docs/superpowers/plans/2026-05-02-extension-tokens-verification.md
git commit -m "docs: extension-tokens verification matrix (partial — backend rows complete)"
```

---

# Stage 6 — Optional follow-up (deferrable)

These are deliberately separate so Stages 1–5 can ship independently.

## Task 6.1: "Active extensions" UI in /account

**Files:**
- Modify: `admin/src/pages/AccountPage.tsx` (or wherever the customer account page lives)
- Or: a new section in the mobile app's settings screen

Use the new `GET /auth/extension-tokens` endpoint to render rows like:

```
DEVICES SIGNED IN
- iPhone (created 2026-05-02, last used 14 minutes ago) [Revoke]
- Pixel 7 (created 2026-04-30, last used 3 days ago)    [Revoke]
```

Wire the "Revoke" button to `DELETE /auth/extension-tokens/:id`.

(Detailed task breakdown deferred until you're ready to ship this UI.)

## Task 6.2: Drop the access-JWT fallback in extensions

After at least one full release cycle where users have had a chance to upgrade and re-login:

- Remove the `accessToken` getter from each extension's `SharedSettings`.
- Replace `bearerToken` with a direct `extensionToken` reference.
- Update the error message: "Please open DraftRight and sign in again."
- (If desired) tighten the backend `RewriteAuthGuard` to reject `dr_ext_*` calls only — currently it accepts both.

## Task 6.3: Audit `last_used_at` write-behind cost

Each `validate()` call updates `last_used_at`. At scale this is row contention on a hot table. If this becomes a problem:

- Move the update to an async queue.
- Or sample (only update if `last_used_at` is more than 1 hour old).
- Or front it with Redis: `SET ext_tok_last_used:<id> NOW EX 86400` and flush periodically.

Don't pre-optimize. Add observability in Stage 5 first.

---

# Stage 7 — Deploy + sign-off (CRITICAL pipeline)

This stage enforces the global Development Task Checklist's deploy steps: TESTING first, run the test matrix on testing, only then PROMOTE to production, then health-check, then write the verification record. **Never deploy backend changes straight to production.**

> **Note on issue labels and "## ✅ How to Verify" issue comments:** skipped per user direction. The deploy log + verification record file (Task 5.1) is the equivalent artifact for this work.

## Task 7.1: Merge feature branch into `develop` (no fast-forward)

**Files:** none — git operation.

- [ ] **Step 1: Confirm all Stage 1–4 commits are pushed and CI green**

```bash
cd /opt/openAi/DraftRight
git -C . fetch origin
git -C . log --oneline origin/develop..feature/extension-tokens-20260502
```

Expected: a list of all commits made during Stages 1–4. If the list is empty, you're already merged or on the wrong branch.

- [ ] **Step 2: Type-check the full backend before merging**

```bash
cd /opt/openAi/DraftRight/backend && npx tsc --noEmit
cd /opt/openAi/DraftRight/backend && npx jest
```

Expected: 0 type errors; all jest specs (including the new `extension-token.*.spec.ts` and `rewrite-auth.guard.spec.ts`) pass.

- [ ] **Step 3: Type-check / analyze Flutter**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter analyze
cd /opt/openAi/DraftRight/DraftRightMobile && flutter test
```

Expected: no analyzer errors; `extension_token_service_test.dart` passes.

- [ ] **Step 4: Merge with `--no-ff`**

```bash
cd /opt/openAi/DraftRight
git checkout develop
git pull --ff-only origin develop
git merge --no-ff feature/extension-tokens-20260502 -m "Merge branch 'feature/extension-tokens-20260502' into develop

Adds scoped 90-day extension tokens for iOS keyboard, iOS share, Android keyboard.
Backend: new /auth/extension-tokens endpoints + RewriteAuthGuard (dual-accept).
Mobile: ExtensionTokenService mints on login, syncs to iOS App Group keychain
and Android SharedPreferences. Extensions read new token, fall back to access JWT."
git push origin develop
```

---

## Task 7.2: Deploy backend to TESTING

**Files:** none — deploy operation against the testing environment.

> **Pre-deploy validation** (per project Deployment Validation Rule): re-read `~/.claude/projects/-opt-openAi-DraftRight/memory/project_production_live.md` to confirm container names, env-file path, and the migration application command. The same approach applies to testing — adapt names where they differ.

- [ ] **Step 1: Apply the SQL migration on the testing database**

```bash
# Replace <TESTING_DB_URL> with the actual testing DSN from the infra docs.
psql "<TESTING_DB_URL>" -f /opt/openAi/DraftRight/backend/migrations/2026-05-02-extension-tokens.sql
psql "<TESTING_DB_URL>" -c '\d extension_tokens'
```

Expected: `CREATE TABLE`, three `CREATE INDEX`, then column listing matches the SQL. If `synchronize` is on in testing, the migration is idempotent (`IF NOT EXISTS`).

- [ ] **Step 2: Build and deploy the backend image to testing**

Use whatever existing testing-deploy procedure the project documents (likely `docker compose -f docker-compose.testing.yml up -d --build backend` on the testing host, or a CI-driven deploy via push to a tag). Do not invent a new procedure here.

- [ ] **Step 3: Smoke-check testing**

```bash
curl -fsS https://<TESTING_HOST>/health  # or whatever the existing health endpoint is
curl -fsS -o /dev/null -w '%{http_code}\n' https://<TESTING_HOST>/api/docs
```

Expected: `health` returns 200; Swagger docs reachable.

- [ ] **Step 4: Run the EXTTOK matrix A on testing**

Re-run the smoke commands from Task 1.8 Step 4 against the testing host (replace `localhost:3000` with the testing URL). Update the rows in `docs/superpowers/plans/2026-05-02-extension-tokens-verification.md` matrix A with PASS/FAIL.

- [ ] **Step 5: STOP if any matrix A row fails**

If any A row fails, do NOT proceed to mobile builds or production deploy. Diagnose, fix on `feature/extension-tokens-20260502`, re-merge to develop with another `--no-ff` merge commit, redeploy testing, re-run matrix A. Only proceed when every A row is PASS.

---

## Task 7.3: Distribute mobile builds to testers

**Files:** none — release operation.

- [ ] **Step 1: Bump versions per project convention**

Match the existing version-bump approach (per memory `cca0fb72 chore(android): bump to 2.1.7, separate per-platform versions`). Bump iOS and Android build numbers as appropriate; do not change marketing version unless the user asks.

- [ ] **Step 2: Build & upload iOS to TestFlight**

Use the existing TestFlight build/upload pipeline. Notify testers (or yourself) that build N is available and references the EXTTOK matrix B.

- [ ] **Step 3: Build & upload Android internal track**

Use the existing internal-track upload pipeline. Notify testers, reference matrix C.

- [ ] **Step 4: Run matrix B and C on a real iPhone and a real Android phone**

Walk through every row of matrices B, C, and D in `2026-05-02-extension-tokens-verification.md`. Critical rows:
- B3, B4, C3 (the **30-minute idle test** — this is the actual bug we're fixing).
- D1–D5 (upgrade-in-place safety).

Update the file with results. Commit the updated verification file.

- [ ] **Step 5: STOP if any matrix B/C/D row fails**

Same rule as 7.2 step 5 — fix on the feature branch, re-merge, re-distribute, re-test.

---

## Task 7.4: Merge `develop` → `main`

**Files:** none — git operation.

- [ ] **Step 1: Confirm matrices A, B, C, D are all green**

Read the verification file. Every row must be PASS (or marked as a known limitation with user acknowledgement).

- [ ] **Step 2: Merge with `--no-ff`**

```bash
cd /opt/openAi/DraftRight
git checkout main
git pull --ff-only origin main
git merge --no-ff develop -m "Merge develop into main: extension tokens"
git push origin main
```

---

## Task 7.5: Deploy backend to PRODUCTION

**Files:** none — deploy operation against production.

> **Pre-deploy validation:** read `project_production_live.md` again. Confirm the production env-file path, the production DB URL, and the container restart command. Validate quoting on any env vars containing special characters before invoking `docker run`.

- [ ] **Step 1: Apply the SQL migration on production database**

```bash
psql "$PROD_DATABASE_URL" -f /opt/openAi/DraftRight/backend/migrations/2026-05-02-extension-tokens.sql
psql "$PROD_DATABASE_URL" -c '\d extension_tokens'
```

Expected: same as testing.

- [ ] **Step 2: Deploy backend image to production**

Use the documented production deploy procedure from `project_production_live.md`. Watch the rollout; do not background it.

- [ ] **Step 3: Post-deploy health check (CRITICAL)**

```bash
ssh <PROD_HOST> 'docker ps --format "table {{.Names}}\t{{.Status}}"'
curl -fsS https://api.draftright.info/health
ssh <PROD_HOST> 'docker logs --tail=200 <BACKEND_CONTAINER>'
```

Expected: backend container `Up X minutes (healthy)`; `/health` returns 200; logs show no boot errors.

- [ ] **Step 4: Production smoke test (regular JWT path — back-compat)**

```bash
JWT=$(curl -fsS -X POST https://api.draftright.info/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"<prod-test-user>","password":"<password>"}' | jq -r .access_token)

curl -fsS -X POST https://api.draftright.info/rewrite \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -d '{"text":"hello there","tone":"professional"}' | jq .
```

Expected: 200 response with `rewritten_text`. **No existing user is broken.**

- [ ] **Step 5: Production smoke test (new extension-token path)**

```bash
EXT=$(curl -fsS -X POST https://api.draftright.info/auth/extension-tokens \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -d '{"device_id":"00000000-0000-0000-0000-0000000000aa","device_name":"prod-smoke"}' \
  | jq -r .token)
echo "$EXT" | head -c 8  # must print: dr_ext_

curl -fsS -X POST https://api.draftright.info/rewrite \
  -H "Authorization: Bearer $EXT" \
  -H 'Content-Type: application/json' \
  -d '{"text":"hello there","tone":"casual"}' | jq .

# Confirm scope rejection: extension token must NOT work on /auth/me
curl -s -o /dev/null -w '%{http_code}\n' https://api.draftright.info/auth/me \
  -H "Authorization: Bearer $EXT"
# Expected: 401
```

Expected: rewrite call returns 200; `/auth/me` returns 401.

- [ ] **Step 6: Clean up the smoke-test token**

The smoke-test token is real and long-lived. Revoke it now.

```bash
# Get its ID
TOK_ID=$(curl -fsS https://api.draftright.info/auth/extension-tokens \
  -H "Authorization: Bearer $JWT" | jq -r '.[] | select(.device_name=="prod-smoke") | .id')

curl -fsS -X DELETE "https://api.draftright.info/auth/extension-tokens/$TOK_ID" \
  -H "Authorization: Bearer $JWT" -o /dev/null -w '%{http_code}\n'
# Expected: 204
```

---

## Task 7.6: Mobile production releases

**Files:** none — release operation.

- [ ] **Step 1: iOS App Store submission**

Promote the TestFlight build that passed matrix B from "Internal Testing" to "App Store" submission. Use the existing release-notes template; mention "Improves reliability of the keyboard and share extension after the app has been idle for a while" in user-facing notes.

- [ ] **Step 2: Android Play Store submission**

Promote the internal-track build that passed matrix C to closed/open testing or production track per the existing release cadence.

- [ ] **Step 3: Note the release dates in the verification file**

Append to `2026-05-02-extension-tokens-verification.md`:

```
## Production releases
- Backend: <YYYY-MM-DD HH:MM TZ>, build <commit-sha>
- iOS App Store: submitted <YYYY-MM-DD>, expected review <YYYY-MM-DD>
- Android Play Store: rolled out <YYYY-MM-DD>
```

---

## Task 7.7: Self-verify on production (CRITICAL)

This is the project's mandatory "test your own verification steps" rule. Do this before declaring the work done.

- [ ] **Step 1: Open the verification record file**

Read `docs/superpowers/plans/2026-05-02-extension-tokens-verification.md` end-to-end. Every row is PASS, or has an explicit user-acknowledged exception.

- [ ] **Step 2: On a real iPhone with the App Store / TestFlight build**

Walk through:
1. Fresh login.
2. Use the keyboard. Confirm it works.
3. Background the app. Wait 30 minutes (real wall-clock).
4. Use the keyboard again. Confirm it works without re-opening the main app.
5. Use the share extension on a Safari page. Confirm it works.

If any step fails, the production deploy is broken. Roll back the migration is not necessary (back-compat is preserved), but investigate and fix before declaring success.

- [ ] **Step 3: On a real Android phone**

Walk through:
1. Fresh login.
2. Use the IME. Confirm it works.
3. Background the app. Wait 30 minutes.
4. Use the IME again. Confirm it works.

- [ ] **Step 4: Write the final outcome line in the verification file**

Append:

```
## Final outcome (Stage 7.7)
- iPhone keyboard 30-min idle test: PASS / FAIL on <date>
- iPhone share extension 30-min idle test: PASS / FAIL on <date>
- Android IME 30-min idle test: PASS / FAIL on <date>
- Production back-compat smoke (Task 7.5 step 4): PASS / FAIL on <date>
- Verified by: Tan
```

- [ ] **Step 5: Commit the final verification record**

```bash
git checkout main
git add docs/superpowers/plans/2026-05-02-extension-tokens-verification.md
git commit -m "docs: extension-tokens production verification complete"
git push origin main
```

**Stage 7 complete. The work is shipped, verified on production, and recorded.**

---

## Self-Review Notes

- **Compliance with global Development Task Checklist:** Stage 0 covers test cases first + branch off develop + infra docs review. Stage 7 covers the testing → production pipeline with health checks and self-verification. GitHub issue creation, status labels, and the `## ✅ How to Verify` issue comment are intentionally **skipped per user direction** — the verification record file (Task 5.1) is the equivalent artifact.
- **Spec coverage:** Backend: covered (Tasks 1.1–1.9). Flutter: covered (Tasks 3.1–3.2). iOS: covered (Tasks 2.1–2.3 + 4.1–4.2). Android: covered (Task 4.3). Verification: covered (Stage 5). Deploy: covered (Stage 7).
- **TC-ID coverage:** Every test case EXTTOK-001 through EXTTOK-020 maps to a specific task and test (see TC-ID Mapping table).
- **Manual steps explicitly called out:** Apple Developer portal capability registration (2.1.4); Xcode target membership for SharedKeychain.swift (2.2.3); spreadsheet edit (0.1); production migration (7.5.1). These cannot be automated and the plan flags them as such.
- **Asymmetric storage:** iOS uses keychain, Android uses SharedPreferences. This is intentional — Android SharedPreferences is per-UID-private, providing the same isolation as keychain on iOS for this threat model. Documented in plan opener.
- **No new dependencies introduced** — uses Node `crypto`, dart `dart:convert`, ioredis already in tree, no new packages on iOS/Android.
- **Single-flight refresh concerns from Option A do not apply** — long-lived tokens don't need refresh, so concurrent extensions cannot race.
- **Branch hygiene** is handled in Task 0.3, not left to the executor's discretion.
