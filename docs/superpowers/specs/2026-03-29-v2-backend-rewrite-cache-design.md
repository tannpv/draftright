# V2 Backend Rewrite Cache — Design Spec

**Date:** 2026-03-29
**Scope:** NestJS backend — server-side caching of AI rewrite results per user
**Goal:** When a user rewrites text with one tone, batch-fetch all 5 rewrite tones in the background and cache results in Redis. Subsequent requests for the same text return instantly from cache without hitting the AI provider.

## Requirements

- Cache stored in Redis with 5-minute TTL
- Per-user cache — no sharing between users
- When a tone is requested: check cache first, return immediately if hit
- On cache miss: call AI provider, cache result, then fire 4 background rewrite tones (Simple, Natural, Polished, Concise, Technical minus the tapped tone)
- Translate is excluded from batching — always on-demand
- Only the user-tapped tone counts against daily usage limit; background pre-fetches are free
- Cache hits skip usage logging entirely (already counted on first request)
- Background tone failures are silently ignored

## Architecture

### New Infrastructure: Redis

Add Redis to `docker-compose.yml`:
- Image: `redis:7-alpine`
- Port: 6379 (internal only, no host mapping needed)
- No persistence needed (cache is ephemeral)
- Environment variable: `REDIS_URL=redis://redis:6379`

### New File: `src/rewrite/rewrite-cache.service.ts`

Wraps `ioredis` client. Responsible for all Redis interactions.

**Key format:**
- Result cache: `rewrite:{userId}:{sha256(text)}:{tone}` → stores the rewritten text string
- Batch flag: `batch:{userId}:{sha256(text)}` → stores `"1"` to indicate batch was started

**Methods:**
- `get(userId: string, text: string, tone: string): Promise<string | null>` — Returns cached rewrite or null
- `set(userId: string, text: string, tone: string, result: string): Promise<void>` — Stores result with 300s TTL
- `isBatchStarted(userId: string, text: string): Promise<boolean>` — Checks batch flag
- `markBatchStarted(userId: string, text: string): Promise<void>` — Sets batch flag with 300s TTL

Text is hashed with SHA-256 before use in keys to keep key length bounded and avoid special characters.

### Modified File: `src/rewrite/rewrite.service.ts`

Inject `RewriteCacheService`. Modify `rewrite()` method:

```
rewrite(userId, text, tone, targetLanguage):
  1. cached = await rewriteCacheService.get(userId, text, tone)
  2. IF cached != null:
       → return { rewritten_text: cached, usage_today: countToday, daily_limit }
       → NO usage log, NO daily limit check (already counted)
  3. ELSE (cache miss — existing flow):
       → check subscription
       → check daily limit
       → build system prompt
       → call AI provider
       → log usage
       → cache result: rewriteCacheService.set(userId, text, tone, result)
  4. IF tone != 'translate' AND !rewriteCacheService.isBatchStarted(userId, text):
       → rewriteCacheService.markBatchStarted(userId, text)
       → define rewriteTones = ['simple','natural','polished','concise','technical'] minus tapped tone
       → for each otherTone: fire async (no await):
           - build system prompt for otherTone
           - call AI provider
           - rewriteCacheService.set(userId, text, otherTone, result)
           - NO usage logging for background tones
       → do NOT await these — fire and forget
  5. return { rewritten_text, usage_today, daily_limit }
```

### Modified File: `src/rewrite/rewrite.module.ts`

- Import `ioredis`
- Register Redis client as provider using `REDIS_URL` env var
- Register `RewriteCacheService`

### Modified File: `docker-compose.yml`

- Add `redis` service
- Add `REDIS_URL` env var to backend service
- Backend `depends_on` includes redis

### Modified File: `.env.example`

- Add `REDIS_URL=redis://localhost:6379`

### Unchanged Files

- `rewrite.controller.ts` — no changes, same endpoint signature
- `rewrite.dto.ts` — no changes
- `ai-providers.service.ts` — no changes, used as-is
- `usage.service.ts` — no changes, called only for tapped tone
- `subscriptions.service.ts` — no changes

## Cache Hit Response

On cache hit, the response still needs `usage_today` and `daily_limit` for the client. The cache hit path still calls `usageService.countTodayByUser()` and fetches the subscription's daily limit, but skips the AI call and usage logging.

## Edge Cases

- **Text changes between requests:** Different SHA-256 hash = different cache key, old entries expire via TTL.
- **User changes subscription mid-cache:** Cache hit skips subscription check, but the original request was already validated. 5-minute TTL limits exposure.
- **AI provider error on background tone:** Silently caught, that tone simply won't be cached. User can request it later for a fresh attempt.
- **Redis unavailable:** `RewriteCacheService` methods should catch connection errors and return null/false — graceful degradation to uncached behavior.
- **Concurrent requests for same text:** Possible duplicate AI calls. Acceptable — Redis SET is idempotent, both results get cached.

## Token Cost Impact

- First tap: 5 AI calls (1 tapped + 4 background) instead of 1. 5x upfront cost.
- Subsequent taps on same text within 5 minutes: 0 AI calls.
- Net saving depends on usage: saves if users try multiple tones. Costs more if users only ever use one tone per text.
- Only 1 usage logged per tap regardless of batching.

## NPM Dependencies

- `ioredis` — Redis client for Node.js (mature, TypeScript types, NestJS-compatible)
