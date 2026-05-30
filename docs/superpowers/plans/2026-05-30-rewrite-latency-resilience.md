# Rewrite-Latency Resilience Plan

**Date:** 2026-05-30
**Branch this is filed under:** `develop`
**Status:** Phase 1 SHIPPING NOW (hotfix), Phases 2-3 PLANNED

## Problem

User reports macOS app showing "Request timed out" for every rewrite
tone on 2026-05-30 02:30-02:38 GMT, even though the same app worked
yesterday and the day before with the same code.

## Root cause (evidence-based)

Postgres `usage_logs.response_time_ms` shows the regression:

| Hour window | Avg | Max | Notes |
|---|---|---|---|
| Last 3 days (baseline) | 1.4–3.0 s | 5.1 s | normal |
| 2026-05-30 02:00 hr | **30.1 s** | **124.1 s** | **30× slower** |

Specific outlier rows (all model `gpt-oss:20b-cloud`, all input ~116
chars):
- 02:34:26 → `simple` → 51.7 s, 201 OK
- 02:37:48 → `natural` → 124.1 s, 201 OK (but client already gone)

Caddy access log for the same window shows
`duration=60.023s, status=0, size=0` — macOS killed the connection at
exactly 60 s. Matches `BackendClient.swift` `timeoutIntervalForRequest = 60`.

**Conclusion:** Ollama Cloud free-tier shared queue depth fluctuates.
On heavy worldwide load it bursts 50-120 s per call. The macOS 60 s
ceiling is too short to ride those bursts → user sees an error while
the backend is still producing a correct answer.

Nothing changed in DraftRight code. Nothing changed in the backend.
Upstream variance only.

## Three-phase fix

### Phase 1 — macOS URLSession headroom (HOTFIX, ~5 min)

Ship in `fix/macos-timeout-bump-20260530` → develop → testing → main →
release `2.3.x+patch`.

```swift
// DraftRight/AI/BackendClient.swift:57
config.timeoutIntervalForRequest = 30   // idle gap
config.timeoutIntervalForResource = 180 // total ceiling
```

- **30 s idle gap:** dead TCP / wedged proxy fails fast.
- **180 s total:** covers Ollama Cloud bursts up to 3× yesterday's max.
- **Tradeoff:** real broken network = user waits 180 s before fail
  (acceptable; the old 60 s ceiling lost real successful responses).

**Effect:** 99% of cloud bursts ride through. UX restored.
**Status:** in-flight.

### Phase 2 — Switch default provider to OpenAI gpt-5-nano (decided, ~5 min)

User chose this over Ollama Cloud Pro after costing both. At current
volume (32 calls/day) → **$0.04/mo**. At 1M users → **$1.8k/mo**, of
which a public rewriteCache (text + tone keyed) cuts ~50% → **$900/mo**
at full scale.

Pricing verified 2026-05-30: gpt-5-nano = $0.05/M input + $0.40/M
output (vs gpt-4o-mini $0.15/$0.60, vs Anthropic Haiku $1/$5).

**Why this beats Ollama Cloud Pro:**
- Pro is $20/mo flat, caps at 50× Free volume → blown at ~10k DAU.
- gpt-5-nano scales linearly + costs ~$0 at our current scale + has
  no shared global queue.
- 1.7s typical latency vs Ollama Cloud's 1-120s variance.

**Steps:**
1. Get OpenAI key from https://platform.openai.com/api-keys.
2. Top up $5 prepaid (`/settings/organization/billing`).
3. Admin portal → AI Providers → Add row:
   - name: `OpenAI Nano`
   - type: `openai`
   - endpoint_url: `https://api.openai.com/v1/chat/completions`
   - model: `gpt-5-nano`
   - api_key: paste `sk-…`
   - temperature: `0.5`
   - is_active: true, is_default: false (first)
4. Smoke-test the row via admin "Test" button or a dev rewrite call.
5. Flip is_default to true once smoke passes — the existing
   `update()` logic in AiProvidersService demotes the old default
   atomically.
6. Sample `usage_logs.response_time_ms` for 24 h; expect max < 5 s,
   avg < 2 s.

**Rollback:** one SQL — flip `is_default` back to the old row.
Documented in the saved plan + memory.

**Cost evolution:**
- Today: $0.04/mo
- 1k users:  $9/mo
- 10k users: $89/mo
- 100k users: $893/mo (~50% with public cache)
- 1M users: $1,785/mo (~$900/mo with cache)

### Phase 3 — Streaming via Go service (root cure, planned)

Already coded as `feature/go-rewrite-microservice-20260529` (Tasks
1-10). Streams SSE tokens — first chunk hits the client in <1 s
regardless of total generation length. **User never sees a hang
again** because they see tokens arrive immediately.

**Steps (= Task 11 from the Go service plan):**
1. Merge `feature/go-rewrite-microservice-20260529` → develop → main.
2. Build + push container; deploy alongside NestJS.
3. Flip Caddyfile rule for `/v1/rewrite` to `:3001` (Go service).
4. Client patch: macOS / iOS / Android / Windows / Linux clients add
   SSE consumer (parse `data: {"delta":"..."}` lines, append to view
   as they arrive).
5. Server-controlled rollout via `/auth/me` `flags.use_go_backend`
   percentage ramp (5% → 20% → 50% → 100% over a week).

**When to do:** after Phase 1 stable + Phase 2 budget approved.
Estimated 1.5 dev days.

**Effect:** terminates the entire class of "upstream slow = client
timeout" bugs. SSE means slow != timed-out.

## Why "yesterday worked, today broke" feels arbitrary (but isn't)

Bursty upstream is stochastic. Same code, same config, same input —
different outcomes depending on global Ollama Cloud load.
Yesterday's load = low → all responses <5 s, well under 60 s ceiling.
Today's spike = high → some responses 50-120 s → ceiling fired.

**Lesson:** never cap a synchronous client deadline below the worst
observed upstream latency. Either widen the deadline (Phase 1) or
switch to streaming (Phase 3).

## Acceptance criteria

| Phase | Criterion |
|---|---|
| 1 | macOS app surfaces no "Request timed out" for any tone over a 24 h burst window post-deploy |
| 2 | `select max(response_time_ms) from usage_logs where created_at > now() - interval '7 days'` < 10000 |
| 3 | mobile + desktop clients render first token within 2 s of POST; no synchronous wait for full response |

## Out of scope

- Switching the default model to a smaller/faster one (`ministral-3:3b`,
  `gemma3:4b`). Considered + rejected for now — output quality matters
  more than speed at our scale. Re-evaluate if Phase 2 is delayed.
- Adding a server-side queue / circuit breaker — Phase 3 SSE makes
  this unnecessary because the user is no longer waiting blind.
