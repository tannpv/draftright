# V2 Backend Rewrite Cache — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Redis-based caching to the V2 backend so that when a user rewrites text with one tone, all 5 rewrite tones are fetched in the background and cached for 5 minutes — making subsequent tone requests instant.

**Architecture:** A new `RewriteCacheService` wraps an `ioredis` client. `RewriteService` checks the cache before calling the AI provider. On cache miss, it fires the tapped tone, caches the result, then fires 4 background tones (fire-and-forget). Translate is excluded from batching. Only the tapped tone counts against usage.

**Tech Stack:** NestJS 10, ioredis, Redis 7, TypeScript, existing AiProvidersService

---

## File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `backend/src/rewrite/rewrite-cache.service.ts` | Redis wrapper — get/set cached rewrites, batch flag management |
| Modify | `backend/src/rewrite/rewrite.service.ts` | Check cache before AI call, fire background batch |
| Modify | `backend/src/rewrite/rewrite.module.ts` | Register Redis provider and RewriteCacheService |
| Modify | `backend/.env.example` | Add REDIS_URL |
| Modify | `docker-compose.yml` | Add Redis service |

---

### Task 1: Add Redis to Docker Compose and env

**Files:**
- Modify: `docker-compose.yml`
- Modify: `backend/.env.example`

- [ ] **Step 1: Add Redis service to docker-compose.yml**

Add the `redis` service and update `backend.depends_on` and `backend.environment`. Replace the entire file:

```yaml
services:
  backend:
    build: ./backend
    ports:
      - "3000:3000"
    environment:
      - DATABASE_URL=postgresql://draftright:password@postgres:5432/draftright
      - REDIS_URL=redis://redis:6379
      - JWT_SECRET=${JWT_SECRET}
      - JWT_REFRESH_SECRET=${JWT_REFRESH_SECRET}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - ADMIN_EMAIL=${ADMIN_EMAIL}
      - ADMIN_PASSWORD=${ADMIN_PASSWORD}
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
    restart: unless-stopped

  postgres:
    image: postgres:16-alpine
    environment:
      - POSTGRES_USER=draftright
      - POSTGRES_PASSWORD=password
      - POSTGRES_DB=draftright
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD", "pg_isready", "-U", "draftright"]
      interval: 5s
      timeout: 3s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    restart: unless-stopped

volumes:
  pgdata:
```

- [ ] **Step 2: Add REDIS_URL to .env.example**

Append to the end of `backend/.env.example`:

```
REDIS_URL=redis://localhost:6379
```

- [ ] **Step 3: Verify Redis starts**

Run: `cd /opt/openAi/DraftRight && docker compose up -d redis && docker compose ps redis`
Expected: redis service shows `running`

- [ ] **Step 4: Commit**

```bash
git add docker-compose.yml backend/.env.example
git commit -m "chore: add Redis service to Docker Compose for rewrite caching"
```

---

### Task 2: Install ioredis dependency

**Files:**
- Modify: `backend/package.json`

- [ ] **Step 1: Install ioredis**

Run: `cd /opt/openAi/DraftRight/backend && npm install ioredis`
Expected: `added X packages` in output, `ioredis` appears in package.json dependencies

- [ ] **Step 2: Commit**

```bash
git add backend/package.json backend/package-lock.json
git commit -m "chore: add ioredis dependency for Redis caching"
```

---

### Task 3: Create RewriteCacheService

**Files:**
- Create: `backend/src/rewrite/rewrite-cache.service.ts`

- [ ] **Step 1: Create rewrite-cache.service.ts**

```typescript
import { Injectable, OnModuleDestroy } from '@nestjs/common';
import Redis from 'ioredis';
import { createHash } from 'crypto';

const CACHE_TTL = 300; // 5 minutes in seconds

@Injectable()
export class RewriteCacheService implements OnModuleDestroy {
  private readonly redis: Redis;

  constructor() {
    this.redis = new Redis(process.env.REDIS_URL || 'redis://localhost:6379', {
      lazyConnect: true,
      maxRetriesPerRequest: 1,
    });
    this.redis.connect().catch(() => {
      // Redis unavailable — degrade gracefully
    });
  }

  onModuleDestroy() {
    this.redis.disconnect();
  }

  async get(userId: string, text: string, tone: string): Promise<string | null> {
    try {
      return await this.redis.get(this.resultKey(userId, text, tone));
    } catch {
      return null;
    }
  }

  async set(userId: string, text: string, tone: string, result: string): Promise<void> {
    try {
      await this.redis.set(this.resultKey(userId, text, tone), result, 'EX', CACHE_TTL);
    } catch {
      // Redis unavailable — skip caching
    }
  }

  async isBatchStarted(userId: string, text: string): Promise<boolean> {
    try {
      const val = await this.redis.get(this.batchKey(userId, text));
      return val === '1';
    } catch {
      return false;
    }
  }

  async markBatchStarted(userId: string, text: string): Promise<void> {
    try {
      await this.redis.set(this.batchKey(userId, text), '1', 'EX', CACHE_TTL);
    } catch {
      // Redis unavailable — skip
    }
  }

  private hashText(text: string): string {
    return createHash('sha256').update(text).digest('hex').substring(0, 16);
  }

  private resultKey(userId: string, text: string, tone: string): string {
    return `rewrite:${userId}:${this.hashText(text)}:${tone}`;
  }

  private batchKey(userId: string, text: string): string {
    return `batch:${userId}:${this.hashText(text)}`;
  }
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /opt/openAi/DraftRight/backend && npx tsc --noEmit 2>&1 | head -20`
Expected: No errors (or only pre-existing warnings)

- [ ] **Step 3: Commit**

```bash
git add backend/src/rewrite/rewrite-cache.service.ts
git commit -m "feat: add RewriteCacheService with Redis get/set and batch flag"
```

---

### Task 4: Register RewriteCacheService in the module

**Files:**
- Modify: `backend/src/rewrite/rewrite.module.ts`

- [ ] **Step 1: Update rewrite.module.ts**

Replace the entire file:

```typescript
import { Module } from '@nestjs/common';
import { RewriteController } from './rewrite.controller';
import { RewriteService } from './rewrite.service';
import { RewriteCacheService } from './rewrite-cache.service';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';
import { UsageModule } from '../usage/usage.module';
import { AiProvidersModule } from '../ai-providers/ai-providers.module';

@Module({
  imports: [SubscriptionsModule, UsageModule, AiProvidersModule],
  controllers: [RewriteController],
  providers: [RewriteService, RewriteCacheService],
})
export class RewriteModule {}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /opt/openAi/DraftRight/backend && npx tsc --noEmit 2>&1 | head -20`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add backend/src/rewrite/rewrite.module.ts
git commit -m "chore: register RewriteCacheService in RewriteModule"
```

---

### Task 5: Integrate cache into RewriteService

**Files:**
- Modify: `backend/src/rewrite/rewrite.service.ts`

- [ ] **Step 1: Replace rewrite.service.ts**

Replace the entire file:

```typescript
import { Injectable, HttpException } from '@nestjs/common';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';
import { UsageService } from '../usage/usage.service';
import { AiProvidersService } from '../ai-providers/ai-providers.service';
import { RewriteCacheService } from './rewrite-cache.service';

const TONE_PROMPTS: Record<string, string> = {
  simple: 'Rewrite the following text using simple, easy-to-understand language. Use short sentences and common words. Preserve the original meaning. Return only the rewritten text, no explanations.',
  natural: 'Rewrite the following text to sound more natural and conversational, as if spoken by a real person. Remove awkward phrasing and make it flow smoothly. Preserve the original meaning. Return only the rewritten text, no explanations.',
  polished: 'Rewrite the following text to be more polished and professional. Improve grammar, word choice, and sentence structure for a refined, workplace-appropriate tone. Preserve the original meaning. Return only the rewritten text, no explanations.',
  concise: 'Rewrite the following text to be as concise as possible. Remove unnecessary words, redundancy, and filler while preserving the key meaning. Return only the rewritten text, no explanations.',
  technical: 'Rewrite the following text in a technical specification style. Use precise, unambiguous language suitable for documentation, specs, or technical communication. Preserve the original meaning. Return only the rewritten text, no explanations.',
};

const REWRITE_TONES = ['simple', 'natural', 'polished', 'concise', 'technical'];

function getTranslatePrompt(targetLanguage: string): string {
  return `Translate the following text into ${targetLanguage}. If the text is already in ${targetLanguage}, translate it into English instead. Preserve the original meaning and tone. Return only the translated text, no explanations.`;
}

@Injectable()
export class RewriteService {
  constructor(
    private readonly subscriptionsService: SubscriptionsService,
    private readonly usageService: UsageService,
    private readonly aiProvidersService: AiProvidersService,
    private readonly rewriteCache: RewriteCacheService,
  ) {}

  async rewrite(userId: string, text: string, tone: string, targetLanguage?: string) {
    // Check cache first
    const cached = await this.rewriteCache.get(userId, text, tone);
    if (cached) {
      const sub = await this.subscriptionsService.findActiveByUserId(userId);
      const dailyLimit = sub?.plan?.daily_limit ?? 0;
      const usageToday = await this.usageService.countTodayByUser(userId);
      return { rewritten_text: cached, usage_today: usageToday, daily_limit: dailyLimit };
    }

    // Cache miss — existing flow: check subscription, limits, call AI
    const sub = await this.subscriptionsService.findActiveByUserId(userId);
    if (!sub || !sub.plan) {
      throw new HttpException({ error: 'No active subscription', usage_today: 0, daily_limit: 0 }, 403);
    }

    const dailyLimit = sub.plan.daily_limit;
    const usageToday = await this.usageService.countTodayByUser(userId);

    if (dailyLimit !== -1 && usageToday >= dailyLimit) {
      throw new HttpException({
        error: 'Daily limit reached', usage_today: usageToday, daily_limit: dailyLimit,
      }, 429);
    }

    const systemPrompt = tone === 'translate'
      ? getTranslatePrompt(targetLanguage || 'English')
      : TONE_PROMPTS[tone];

    if (!systemPrompt) {
      throw new HttpException({ error: `Unknown tone: ${tone}` }, 400);
    }

    const provider = await this.aiProvidersService.findDefault();

    let result: { text: string; responseTimeMs: number };
    try {
      result = await this.aiProvidersService.callProvider(provider, systemPrompt, text);
    } catch (error: any) {
      throw new HttpException({ error: `AI provider error: ${error.message}` }, 502);
    }

    // Log usage only for the tapped tone
    await this.usageService.log({
      user_id: userId, tone, input_length: text.length, output_length: result.text.length,
      ai_provider_id: provider.id, response_time_ms: result.responseTimeMs,
    });

    // Cache the result
    await this.rewriteCache.set(userId, text, tone, result.text);

    // Fire background batch for other rewrite tones (excluding translate)
    if (tone !== 'translate' && !(await this.rewriteCache.isBatchStarted(userId, text))) {
      await this.rewriteCache.markBatchStarted(userId, text);

      const otherTones = REWRITE_TONES.filter(t => t !== tone);
      for (const otherTone of otherTones) {
        this.fetchAndCacheTone(userId, text, otherTone, provider).catch(() => {
          // Silently ignore background errors
        });
      }
    }

    return { rewritten_text: result.text, usage_today: usageToday + 1, daily_limit: dailyLimit };
  }

  private async fetchAndCacheTone(
    userId: string, text: string, tone: string, provider: any,
  ): Promise<void> {
    const systemPrompt = TONE_PROMPTS[tone];
    if (!systemPrompt) return;

    const result = await this.aiProvidersService.callProvider(provider, systemPrompt, text);
    await this.rewriteCache.set(userId, text, tone, result.text);
  }
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /opt/openAi/DraftRight/backend && npx tsc --noEmit 2>&1 | head -20`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add backend/src/rewrite/rewrite.service.ts
git commit -m "feat: integrate RewriteCacheService — cache check, background batch, usage skip on hit"
```

---

### Task 6: Test end-to-end

**Files:** None changed — manual testing.

- [ ] **Step 1: Start Redis and backend**

```bash
cd /opt/openAi/DraftRight
docker compose up -d redis postgres
cd backend
REDIS_URL=redis://localhost:6379 npm run start:dev
```

Expected: NestJS starts on port 3000 with no Redis connection errors in logs.

- [ ] **Step 2: Login to get JWT token**

```bash
curl -s -X POST http://localhost:3000/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@draftright.com","password":"DraftRight2026"}' \
  | jq -r '.access_token'
```

Save the token as `TOKEN` for subsequent requests.

- [ ] **Step 3: Test cache miss — first rewrite**

```bash
curl -s -X POST http://localhost:3000/rewrite \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"text":"The quick brown fox jumps over the lazy dog","tone":"simple"}' \
  | jq .
```

Expected: Response with `rewritten_text`, `usage_today: 1`. Takes a few seconds (AI call).

- [ ] **Step 4: Verify background tones were cached**

```bash
redis-cli KEYS "rewrite:*"
```

Expected: Multiple keys — one for the tapped tone (`simple`) plus up to 4 background tones (`natural`, `polished`, `concise`, `technical`). Background tones may still be populating.

Wait 5 seconds, then run again — should see all 5 tones cached.

- [ ] **Step 5: Test cache hit — second rewrite (different tone)**

```bash
curl -s -X POST http://localhost:3000/rewrite \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"text":"The quick brown fox jumps over the lazy dog","tone":"polished"}' \
  | jq .
```

Expected: Instant response (< 100ms). `usage_today` should still be `1` (cache hit doesn't increment).

- [ ] **Step 6: Test translate is not cached from batch**

```bash
curl -s -X POST http://localhost:3000/rewrite \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"text":"The quick brown fox jumps over the lazy dog","tone":"translate","target_language":"Vietnamese"}' \
  | jq .
```

Expected: Takes a few seconds (not cached). `usage_today` increments to `2`.

- [ ] **Step 7: Test cache expiry**

```bash
redis-cli TTL "$(redis-cli KEYS 'rewrite:*' | head -1)"
```

Expected: TTL value between 0 and 300 seconds.

- [ ] **Step 8: Commit tag**

```bash
git tag -a v2-cache -m "V2 backend rewrite cache with Redis — batch prefetch, per-user, 5-min TTL"
```
