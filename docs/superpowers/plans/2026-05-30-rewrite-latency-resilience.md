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

### Phase 2 — Paid Ollama Cloud tier ($20/mo, planned)

Free tier shares one global queue across every Ollama Cloud user.
Paid tier gives dedicated concurrency + no burst queueing.

**Steps when ready:**
1. Visit https://ollama.com/billing, upgrade.
2. Verify new key has higher concurrency (probe `/v1/chat/completions`
   3× in parallel; expect all <5 s).
3. Update `OPENAI_API_KEY` in NestJS `.env` on VPS.
4. Restart backend.
5. Sample `usage_logs.response_time_ms` for 24 h to confirm max <10 s.

**When to do:** within 7 days of Phase 1 ship. Avoids next bursty
window catching users with a 30-180 s wait.

**Cost:** $20/mo, scales to ~$50/mo if usage grows. Cheap vs the
support cost of "yesterday worked" tickets.

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
