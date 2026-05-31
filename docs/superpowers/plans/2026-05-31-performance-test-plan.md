# DraftRight Performance Test Plan

> **Goal**: produce defensible numbers for "how does the stack behave
> under load" without spending money, tripping prod alarms, or
> generating false confidence from synthetic workloads.

**Date:** 2026-05-31
**Author:** Tan (with Claude)
**Status:** DRAFT — execute on approval

---

## 1. Why a plan, not a script

Naive "1M requests" against `/rewrite` would:
- Burn ~$140 in OpenAI gpt-5-nano fees.
- Hit OpenAI Tier-1 RPM cap (500/min) → 30+ h serial elapsed.
- Trigger Caddy `rate_limit` + DigitalOcean DOS heuristics.
- Tell us nothing useful — upstream LLM dominates the latency.

The right shape is **multiple small, targeted tests** that isolate
each layer of the stack and surface the actual bottleneck.

---

## 2. What we want to learn

| Q | Answer reveals |
|---|---|
| Q1: Single-NestJS-container throughput on `/health`? | TLS + Express overhead alone (no DB, no LLM) |
| Q2: Single-Go-container throughput on `/v1/rewrite` (stub)? | Provider-pipeline overhead without LLM cost |
| Q3: Caddy + NestJS prod `/health` p95 from outside? | Internet + TLS + reverse-proxy budget |
| Q4: Real `/rewrite` p50/p95 under modest concurrency? | OpenAI gpt-5-nano latency tail in production |
| Q5: At what load does NestJS start dropping (5xx > 1%)? | Capacity headroom before horizontal scale needed |
| Q6: Does `/v1/rewrite` SSE hold up at high concurrent connections? | Goroutine memory + flush overhead |

---

## 3. Hypotheses

| # | Statement | If wrong, action |
|---|---|---|
| H1 | NestJS `/health` sustains ≥ 3000 req/s on local Mac | If <1000 → look at Express middleware stack (validation pipe? throttler eating CPU?) |
| H2 | Go `/v1/rewrite` sustains ≥ 5000 req/s on local Mac | If <2000 → chi router or our SSE writer is bottlenecking |
| H3 | Prod `/health` p95 < 80 ms from a Vietnam IP | If >200 ms → Caddy or DO network |
| H4 | Real `/rewrite` p95 < 4 s at 5 concurrent | If >8 s → OpenAI degraded OR our reasoning_effort=minimal isn't applying |
| H5 | NestJS prod 5xx rate < 0.1% under sustained 50 req/s for 60 s | If >1% → Postgres pool too small or Redis flap |
| H6 | Go `/v1/rewrite` SSE memory stays < 100 MB at 1000 concurrent streams | If >300 MB → goroutine leak |

---

## 4. Scope NOT covered

- Single-flight de-duplication (cache-stampede).
- Database write-heavy workloads (subscription/payment flows).
- Multi-region latency.
- Cold-start (Docker container restart) under load.
- iOS / Android keyboard share-extension throughput.

Out-of-scope items are listed so a follow-up plan can pick them up.

---

## 5. Tools

| Tool | Purpose | Why |
|---|---|---|
| `oha` | HTTP load generator | Modern wrk2 replacement, single binary, JSON output, supports rate limiting (`--query-per-second`) |
| `htop` / `docker stats` | Resource observation | Live CPU/RAM per container |
| Prometheus + ad-hoc Grafana query | Server-side stats | Our `/metrics` endpoint already emits `draftright_rewrite_*` series |
| `psql` against prod | Verify usage_logs rows for `/rewrite` runs | Confirms no silent drops |

Install:
```bash
brew install oha   # one-time
```

---

## 6. Test scenarios

Each scenario has: **target**, **method**, **duration**, **expected**, **kill switch**.

### S1 — Local NestJS `/health`
- **Target**: `http://localhost:3000/health`
- **Method**: `oha -n 100000 -c 100 --no-tui`
- **Duration**: until 100k complete (~30 s expected)
- **Expected**: avg ≥ 3000 req/s, p99 < 50 ms
- **Kill switch**: if 5xx > 1% → ctrl-c, log issue, skip remaining

### S2 — Local Go `/v1/rewrite` (memory stub)
- **Target**: `http://localhost:3001/v1/rewrite`
- **Method**: POST with JWT, body `{"text":"hi","tone":"polished"}`, `oha -n 50000 -c 200`
- **Pre-req**: Go service running with `AI_PROVIDERS=` empty so memory stub is selected
- **Expected**: avg ≥ 5000 req/s, p99 < 100 ms (memory stub returns 5 tokens immediately)
- **Kill switch**: same as S1

### S3 — Local Go `/v1/rewrite` SSE
- **Target**: same as S2 but `Accept: text/event-stream`
- **Method**: 1000 concurrent connections held open for 60 s
- **Pre-req**: same as S2
- **Expected**: container RAM < 150 MB, no goroutine leak (check `/metrics` Go runtime gauge)
- **Kill switch**: container RAM > 500 MB → kill

### S4 — Prod `/health` from Vietnam IP
- **Target**: `https://api.draftright.info/health`
- **Method**: `oha -n 50000 -c 50 --query-per-second 500` (rate-limited to be polite)
- **Duration**: ~100 s
- **Expected**: p95 < 80 ms, 0% 5xx
- **Kill switch**: 4xx > 5% (means Caddy is rate-limiting → back off + reduce -c)

### S5 — Prod `/rewrite` real upstream
- **Target**: `https://api.draftright.info/rewrite` with valid JWT (signed for admin user)
- **Method**: `oha -n 100 -c 5` with body `{"text":"Short test sentence.","tone":"polished"}`
- **Duration**: ~3-5 min
- **Expected**: p50 < 2 s, p95 < 4 s, 0 errors
- **Cost**: ~100 × $0.0001 = ~$0.01
- **Kill switch**: any 5xx → stop, inspect logs

### S6 — Prod sustained burst (capacity)
- **Target**: `https://api.draftright.info/health` (cheap endpoint)
- **Method**: `oha -c 200 --query-per-second 200 -z 60s`
- **Duration**: 60 s
- **Expected**: backend container stays < 80% CPU, no 5xx
- **Kill switch**: CPU > 95% sustained → stop; this means current droplet is too small

---

## 7. Pre-flight checklist

- [ ] Branch from `develop`: `chore/perf-baseline-20260531`.
- [ ] Local stack running: `cd backend && npm run start:dev` + `cd backend-rewrite-go && go run ./cmd/server`.
- [ ] `oha` installed (`brew install oha`).
- [ ] Sign a fresh JWT for prod tests (15-min expiry).
- [ ] `docker stats` running in side terminal to observe local containers.
- [ ] Prod baseline: snapshot `draftright_rewrite_*` metrics before S5/S6 so deltas isolate the test.
- [ ] Tell ops (you) you're about to load prod for 5 min.

---

## 8. Execution order

1. **S1** → S2 → S3 (all local, fast, no risk).
2. Review local numbers. If H1/H2/H3 are violated, pause + investigate before touching prod.
3. **S4** → S5 → S6 (prod, escalating). 30 s cooldown between each.

---

## 9. Recording

For each scenario, capture into a single markdown report (`docs/perf-baseline-20260531.md`):

```
## Scenario: SN
- Command run:
- Total requests:
- Wall clock:
- req/s avg:
- p50 / p95 / p99 latency:
- 2xx %, 4xx %, 5xx %:
- Observed CPU / RAM:
- Notable findings:
```

---

## 10. Pass/fail per hypothesis

After all scenarios run:

| Hypothesis | Pass criterion | Fail action |
|---|---|---|
| H1 (NestJS ≥ 3000 req/s) | from S1 | profile Express middleware |
| H2 (Go ≥ 5000 req/s) | from S2 | profile chi router |
| H3 (prod /health p95 < 80 ms) | from S4 | inspect Caddy log timing |
| H4 (real /rewrite p95 < 4 s) | from S5 | verify gpt-5-nano + reasoning_effort=minimal in DB row |
| H5 (5xx < 0.1% sustained) | from S6 | bump Postgres pool / look at Redis |
| H6 (SSE RAM < 100 MB at 1000 conn) | from S3 | look for goroutine leak; check `runtime.NumGoroutine()` |

---

## 11. Time + cost budget

| | Wall time | $ cost |
|---|---|---|
| S1 local /health | 30 s | $0 |
| S2 local Go stub | 10 s | $0 |
| S3 local SSE | 60 s | $0 |
| S4 prod /health | 100 s | $0 |
| S5 prod /rewrite | 3-5 min | ~$0.01 |
| S6 prod burst | 60 s | $0 |
| **Total** | **~10 min** | **~$0.01** |

---

## 12. Risks

| Risk | Mitigation |
|---|---|
| Prod Caddy rate-limits the test client → false fail | Pre-allow my dev IP in Caddyfile for the test window OR use `-q` to stay below threshold |
| OpenAI charges spike from runaway loop | S5 is hard-capped at `-n 100`; oha doesn't retry on timeout |
| `usage_logs` table grows ~100 rows / S5 run | Acceptable; cleanup in `--admin-notes` script later |
| Token expires mid-test (15 min) | S5+S6 total well under 15 min; refresh script ready if needed |
| Local Mac M-series perf ≠ prod x86 droplet | Numbers are RELATIVE; track ratio not absolute |

---

## 13. Stretch goals (only if time allows after S1-S6)

- **S7 — Soak test**: 20 req/s sustained for 1 hour against `/rewrite`. Detects slow leaks. Cost ~$7. Not in baseline; budget separately.
- **S8 — Capacity walk-up**: ramp from 10 to 200 req/s in 10-req/s steps, find the knee in p99 latency. Reveals scaling pain points.

---

## 14. Output artifact

After execution, commit:

- `docs/perf-baseline-20260531.md` — the recording above + a 3-sentence summary at the top.
- Update `docs/architecture-standards.md` Tier-1/Tier-2 thresholds with measured numbers (no more hand-waving "supports 10k DAU").

---

## 15. Approval gate

Don't execute until:

- [ ] Plan reviewed.
- [ ] Time window confirmed (off-peak ideally — late evening Vietnam time).
- [ ] OpenAI billing alert configured for low threshold ($1 spend alert).
- [ ] Caddy access log rotation isn't about to fire during the window (would lose data).

When green-lit, run scenarios in section 7 order, capture per section 9 template.
