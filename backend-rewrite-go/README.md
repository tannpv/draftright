# backend-rewrite-go

Go microservice that owns the `/rewrite` endpoint, running alongside the
existing NestJS backend during the Strangler-Fig migration. Architecture
+ rationale: `docs/superpowers/plans/2026-05-29-go-rewrite-microservice.md`
in the parent repo.

**Current state:** Task 1 — scaffold + Hello World + Docker. No JWT, no DB,
no AI provider, no SSE yet.

## Quick start

```bash
# Build + run via Docker (recommended — matches production)
make dev

# In another shell
curl http://localhost:3001/health
# {"service":"rewrite-go","status":"ok"}

curl -X POST http://localhost:3001/rewrite
# {"note":"Task 1 scaffold...","service":"rewrite-go","text":"Hello from Go!","tone":"placeholder"}
```

## Boundary rules (Rule #1 — clean code / reusable / extendable)

1. **NestJS owns the schema** until Task 12 cutover. This service uses
   `sqlc`-generated read bindings (Task 3) — never raw migrations from
   here.
2. **JWT_SECRET shared via env** with NestJS. Same HS256 hash on both
   sides (Task 2).
3. **No cross-service HTTP in the hot path.** This service talks
   straight to Postgres + Redis. Calling NestJS for quota would defeat
   the purpose.
4. **`internal/` boundary** keeps domain free of framework imports.
   Domain depends on nothing outside `internal/domain/`.
5. **`domain/ports.go` defines interfaces; adapters implement them.**
   Hexagonal architecture, compile-time enforced via Go's package
   visibility.
6. **No `.unwrap()` equivalent.** Every error returned. `slog.Error`
   for non-fatal, return for fatal.
7. **Schema changes follow this order:** NestJS migration → deploy
   NestJS → `sqlc generate` → deploy Go.

## Repository layout

```
cmd/server/main.go             composition root
internal/
  domain/                      entities + ports (no external deps)
  usecase/                     orchestration (depends only on domain)
  adapter/                     concrete I/O (postgres, openai, redis, ...)
  http/                        HTTP handlers (chi router, Task 7)
  platform/                    cross-cutting infra (config, logger, ...)
```

See the plan for task-by-task progression.

## Tasks (high level)

- [x] **Task 1:** Scaffold + Hello World + Docker
- [ ] **Task 2:** JWT auth middleware (shared HS256)
- [ ] **Task 3:** Postgres via pgx + sqlc bindings
- [ ] **Task 4:** Domain layer (entities + ports)
- [ ] **Task 5:** Use case (orchestration only)
- [ ] **Task 6:** Adapters — pg, openai, redis_rate_limiter
- [ ] **Task 7:** HTTP handler with SSE streaming
- [ ] **Task 8:** Provider failover (Anthropic + Ollama)
- [ ] **Task 9:** Observability (slog + Prometheus + tracing)
- [ ] **Task 10:** Caddy side-by-side + production deploy
- [ ] **Task 11:** Client feature flag (opt-in A/B)
- [ ] **Task 12:** Full cutover

Full task detail in `docs/superpowers/plans/2026-05-29-go-rewrite-microservice.md`.
