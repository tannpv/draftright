# rewrite-go — Production Deploy Runbook

> Companion to `backend-rewrite-go/CLAUDE.md`. Reference when shipping
> the Go /rewrite microservice to the VPS alongside the NestJS backend.

## TL;DR

1. Merge `feature/go-rewrite-microservice-20260529` → `develop` → `main`.
2. CI builds + pushes `ghcr.io/<owner>/draftright-rewrite-go:latest`.
3. Pull on the VPS: `docker compose pull rewrite-go && docker compose up -d rewrite-go`.
4. Smoke-test `:3001/health` and an authed `/v1/rewrite` (see `scripts/smoke.sh`).
5. Flip the Caddy rule for `/v1/rewrite` from `:3000` → `:3001`.
6. Watch `draftright_rewrite_requests_total{outcome="provider_failed"}` for 30 min.
7. Roll back = flip the Caddy line back. No data migration, no client change.

## Architecture (strangler fig)

```
                 ┌─────────────────── Caddy ───────────────────┐
                 │                                             │
client ─────────►│  /v1/rewrite                        :3001   │──► rewrite-go (Go)
                 │  *                                  :3000   │──► backend     (NestJS)
                 └─────────────────────────────────────────────┘
                                  shared:
                            Postgres :5432
                            Redis    :6379
                            JWT_SECRET (env)
```

Both services validate the **same** HS256 JWT signed by NestJS. A user
session lives across both — no double-login, no re-auth, no token
shape divergence. That's the central invariant; do not break it.

## Caddy snippet

Replace your current global `/v1/rewrite` route. Production Caddyfile
edits live on the VPS; SSH then:

```caddy
# === draftright.info ===
draftright.info {
    # Go-served path FIRST so it wins over the catch-all.
    handle /v1/rewrite {
        # Long timeout for SSE streams. Caddy buffers responses by
        # default; flush_interval=-1 disables buffering so each chunk
        # arrives at the client immediately.
        reverse_proxy 127.0.0.1:3001 {
            flush_interval -1
            transport http {
                # SSE responses can run minutes; never timeout the
                # connection prematurely.
                response_header_timeout 10m
                read_timeout 10m
            }
        }
    }

    # Everything else (auth, admin, payments, …) stays on NestJS.
    handle {
        reverse_proxy 127.0.0.1:3000
    }
}
```

Rollback = delete the `/v1/rewrite` handle block and `caddy reload`.

**Never** expose `/metrics` publicly. Either gate at Caddy:

```caddy
# Optional: scrape from inside the VPN only.
@metrics path /metrics
handle @metrics {
    @internal client_ip 10.0.0.0/8 100.64.0.0/10
    handle @internal { reverse_proxy 127.0.0.1:3001 }
    respond 403
}
```

…or set `METRICS_ENABLED=false` and run a sidecar Prometheus exporter
in the private network only.

## Required env (prod)

| Var | Value |
|---|---|
| `JWT_SECRET` | **identical** to NestJS `JWT_SECRET` |
| `DATABASE_URL` | `postgresql://draftright:password@postgres:5432/draftright` |
| `REDIS_URL` | `redis://redis:6379` |
| `OPENAI_API_KEY` | production OpenAI key |
| `ANTHROPIC_API_KEY` | (optional, enables failover) |
| `OLLAMA_URL` | (optional, on-host fallback) |
| `AI_PROVIDERS` | `openai,anthropic,ollama` (priority list) |
| `METRICS_ENABLED` | `true` (publish behind firewall only) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `otel-collector.internal:4318` (optional) |
| `LOG_LEVEL` | `info` (drop to `debug` only during incident) |
| `APP_ENV` | `production` |

Add these to the operator-managed `.env` on the VPS — never check in.

## Cutover plan

### Phase 0 — shadow (optional)
Run both services live, route `/v1/rewrite` to NestJS. Hit the Go
endpoint directly via header trick: `Host: rewrite-go.internal`. Confirm
parity vs NestJS for the same payload.

### Phase 1 — small-percentage shift
Use Caddy's `vars_regexp` to route N% of users to Go by user-id hash.
Start at 5%, hold 24 h, watch metrics:

- `draftright_rewrite_duration_seconds` p95 should be **≤** NestJS baseline.
- `draftright_rewrite_requests_total{outcome="provider_failed"}` should not spike.
- Postgres `usage_logs` row count over 5 min = NestJS count − 5%.

### Phase 2 — full cutover
Flip to 100%. Leave NestJS rewrite path running for 7 days as
emergency rollback. After 7 days clean, delete the NestJS controller.

### Phase 3 — strangler complete
NestJS no longer answers `/v1/rewrite`. Remove the route. Done.

## Rollback

Single line in Caddyfile + `caddy reload`. No DB migration, no client
shipping, no cache warm-up. That is the whole point of the strangler
pattern. Practice the rollback once before relying on it.

## Smoke test

```bash
# From a dev box with prod JWT_SECRET in the shell:
./scripts/smoke.sh https://draftright.info "$JWT"
```

Asserts: `/health` OK, `/v1/rewrite` with the JWT streams ≥1 chunk,
SSE terminator present.

## Known operational gotchas

- **Caddy buffer-by-default kills SSE.** Without `flush_interval -1`,
  every chunk batches until the response completes — users see a 30 s
  hang then the whole rewrite at once. Symptom on the client side:
  "rewrite is laggy" even when latency metrics look fine.
- **JWT clock skew between hosts.** If the VPS and the API origin
  drift, `exp` checks fail intermittently. Run `chrony` / `ntpd` on
  both.
- **`OPENAI_API_KEY` in compose env is process-readable.** Keep
  `chmod 600` on the `.env` file + restrict shell access on the VPS.
- **Redis fail-open.** When Redis is unreachable, the rate limiter
  allows. That's the deliberate posture (see
  `internal/adapter/redislimit/limiter.go`) but it means a downed
  Redis = no rate limiting. Alert on Redis liveness separately.
