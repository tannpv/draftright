# DraftRight Architecture Standards

> Canonical reference for how DraftRight services are built, deployed,
> and talk to each other. Updated alongside the codebase — when a
> standard changes, the PR touches both the code and this file.

## Status

Initial draft 2026-05-30, written after the architecture review that
shipped items 1, 2, 3, 4 of the review (error envelope, request-id,
strategy pattern, post-deploy smoke).

## 1. Scope

Two backend services in active development:

| Service | Stack | Path on host | Repo path |
|---|---|---|---|
| `backend` | NestJS 10, TypeScript, TypeORM | `/opt/draftright/` | `backend/` |
| `rewrite-go` | Go 1.25, chi, pgx, sqlc | `/opt/draftright-rewrite-go/` | `backend-rewrite-go/` |

Plus 5 client apps (macOS, iOS, Android, Windows, Linux) consuming the
backend, and 2 static sites (admin SPA, marketing).

## 2. Layering

### S1. New services use clean architecture (hexagonal / ports-and-adapters)

The Go service is the reference:

```
internal/
├── domain/      entities + ports (interfaces). Zero external deps.
├── usecase/     orchestration. Depends only on domain.
├── adapter/     concrete impls of ports. Imports domain.
├── platform/    cross-cutting infra (auth, config, db, metrics, tracing).
└── http/        transport layer (outermost).
cmd/server/main.go  composition root — wires concrete adapters.
```

Inner layers don't know outer layers exist. Compiler enforces it
through Go's import cycle prohibition.

### S2. NestJS modules stay feature-organized

Each NestJS module mirrors a business capability (`auth`, `users`,
`subscriptions`, etc.) and contains:

```
src/<feature>/
├── entities/                 TypeORM models
├── dto/                      class-validator request shapes
├── strategies/  (optional)   strategy-pattern impls
├── <feature>.controller.ts   HTTP routes
├── <feature>.service.ts      business logic
└── <feature>.module.ts       DI wiring
```

New modules follow this layout.

### S3. No business logic in controllers, no HTTP in domain

- Controllers delegate to services and return shaped responses.
- Services own use-case orchestration.
- Domain entities have no awareness of HTTP, frameworks, or transport.

## 3. Service communication

### S4. Star topology through Caddy

```
client → Caddy → (NestJS | Go)
```

- **No service-to-service shortcuts** at the container layer.
- Both services read the shared database; **neither writes the
  other's domain tables**.
- When inter-service communication becomes necessary, it goes through
  Caddy (uniform observability + auth + rate limiting).

### S5. Sync HTTP is the default

REST + JSON for everything below 100ms tail-latency budget (excluding
the LLM stream itself, which is bounded by the upstream provider).

### S6. Async messaging is opt-in

Move to async (Redis Streams / NATS / SQS) when one of:

- Sync handler p99 latency exceeds 200ms for a reason other than the
  LLM upstream.
- Operation can be retried + idempotent.
- Operation is part of a batch / fan-out (cron, email blast, log
  rollup).

Don't pre-emptively async an endpoint just because "it could be."

## 4. Authentication

### S7. Single HS256 JWT, NestJS signs, every other service verifies

- Same `JWT_SECRET` env var across all services.
- Claims shape: `{ "sub": "<uuid>", "role": "user|admin", "exp", "iat" }`.
- Go service has its own JWT verifier (`internal/platform/auth/jwt.go`)
  but never issues tokens.
- Extension tokens (`dr_ext_*`, sliding 90-day) live in NestJS only
  today; can extend Go's `Verify()` to recognize them when needed.

### S8. JWT secret rotation is dual-window

1. Add new secret as `JWT_SECRET_NEXT`.
2. Both services accept tokens signed with either for a 24 h window.
3. After 24 h, promote `JWT_SECRET_NEXT` to `JWT_SECRET`, remove old.

Not yet implemented in code; documented as the target rotation flow.

## 5. Error handling

### S9. Canonical error envelope on every service

```json
{
  "error": "<human-readable message>",
  "code":  "kebab-case-machine-id",
  "request_id": "<uuid>"
}
```

- Mirrors the Go service shape (its `internal/http/errors.go`).
- Registered codes: `backend/src/common/error-codes.ts`. Sync any new
  code into Go's `internal/http/errors.go` if it can arise there.
- `error` stays a string at the top level for backward compatibility
  with macOS clients that surface `bodyText`.

### S10. Status codes for known failures

| Code | Status |
|---|---|
| `invalid-input` | 400 |
| `invalid-token` | 401 |
| `user-not-found` | 401 |
| `quota-exceeded` | 402 |
| `forbidden` | 403 |
| `not-found` | 404 |
| `conflict` | 409 |
| `rate-limited` | 429 |
| `provider-failed` | 502 |
| `provider-unavailable` | 503 |
| `internal` | 500 |

## 6. Observability

### S11. Every service emits `X-Request-Id`

- Caddy generates one if absent, forwards otherwise.
- NestJS: `RequestIdMiddleware` (`backend/src/common/request-id.middleware.ts`).
- Go: chi `middleware.RequestID`.
- Every log line stamps it.
- Clients surface it on error so support tickets can quote a traceable id.

### S12. Every service exposes `/metrics`

- Prometheus text format.
- Route accessible only from internal CIDR or behind auth at Caddy.
- Go service: wired via `internal/platform/metrics`.
- NestJS: wired via `@willsoto/nestjs-prometheus` →
  `backend/src/common/metrics/`. Same series names as Go
  (`draftright_rewrite_requests_total`,
  `draftright_rewrite_duration_seconds`) so a single Grafana panel
  joins both backends.

### S13. Structured JSON logs

- Go: `slog` JSON handler.
- NestJS: default logger today; switch to Pino with `{ timestamp, level,
  msg, request_id, ... }` shape when central log aggregation lands.

## 7. Configuration

### S14. Typed, validated, fail-fast at boot

- Go: `internal/platform/config.Load()` returns `(*Config, error)`.
  Required env vars enumerated; service refuses to start without them.
- NestJS: `@nestjs/config` with a Zod schema at
  `backend/src/config/env.schema.ts`. `ConfigModule.forRoot({ validate:
  validateEnv })` rejects boot if any required field is missing or
  malformed; one error message lists every offending field at once.
  Consumers inject `ConfigService<EnvSchema, true>` for strongly-typed
  access — `cfg.get('JWT_SECRET')` returns `string`, not
  `string | undefined`. Direct `process.env.*` reads outside
  `env.schema.ts` are a code-review red flag.

### S15. `.env` files never in git

- `.env.example` committed with placeholders.
- Real `.env` on the host, `chmod 600`, root-owned.
- CI pulls from a vault (future — not needed at solo-dev scale).

## 8. Data layer

### S16. Postgres is the source of truth for schema

- All schema changes start with a SQL migration in `backend/sql/`.
- Both services read from the same `draftright` database.
- Each service "owns" only the tables it writes to; both can read any
  table.

### S17. Go's sqlc bindings auto-refresh from prod schema

- `backend-rewrite-go/internal/platform/db/schema.sql` is the dump.
- `backend-rewrite-go/scripts/dump-prod-schema.sh` refreshes from
  `ssh draftright pg_dump`.
- CI runs `sqlc generate` + `git diff --exit-code` to gate drift.

### S18. NestJS: `synchronize: true` in dev, migrations in prod

- Dev: `synchronize: true` (auto-sync on entity changes).
- Prod: explicit migrations under `/opt/draftright/migrations/`.
- Adding a `@Column` to an entity in prod = manual SQL migration
  FIRST, then deploy. (See `feedback_prod_synchronize_off_migrations`.)

## 9. Deployment

### S19. One container per service, immutable images

- Each service builds a single Docker image.
- Images are content-addressed (tag = git SHA).
- `latest` tag = current main.
- Rollback = redeploy a previous SHA tag.

### S20. Per-host single replica until ≥10k DAU

- Compose stack with one replica each.
- Vertical scale (larger droplet) before horizontal.
- Switch to `replicas: N` + Caddy round-robin when CPU sustained >50%.

### S21. Deploy is one command

```bash
ssh draftright 'cd /opt/<service> && sudo docker compose -f docker-compose.prod.yml up -d --build <service>'
```

### S22. Every deploy followed by smoke check

```bash
./scripts/post-deploy-smoke.sh https://api.draftright.info --jwt "$JWT"
```

Non-zero exit triggers immediate rollback.

### S23. Caddyfile changes go through `caddy validate` + `caddy reload`

- Edit local; `caddy validate --config <file>`.
- Replace live file; `systemctl reload caddy`.
- Never edit live without validate.
- Backup live file under `Caddyfile.bak.YYYYMMDD` before replace.

## 10. Provider abstraction (cross-service standard)

### S24. Strategy pattern for any swappable upstream

Already adopted in:

- `backend/src/payment/strategies/` (Stripe, PayPal, VietQR, …).
- `backend/src/ai-providers/strategies/` (OpenAI-compat, Anthropic).
- `backend-rewrite-go/internal/adapter/` (openai, anthropic, ollama,
  chain, memory).

When adding a new provider:

1. New file in the corresponding `strategies/` (NestJS) or `adapter/`
   (Go) directory implementing the interface / port.
2. Register in the module / composition root.
3. Zero edits to existing strategies or the service / use case layer.

## 11. Testing

### S25. Three test layers per service

| Layer | Coverage |
|---|---|
| Unit | Pure functions + in-memory fakes |
| Integration | Real Postgres + Redis containers (testcontainers) |
| Smoke | Real deploy, hits live endpoints, asserts shapes |

Today:

- Go: unit + smoke (`scripts/smoke.sh`, `scripts/post-deploy-smoke.sh`).
- NestJS: unit + integration + smoke.
- Integration entry point: `npm run test:integration` from
  `backend/`. Suite lives under `backend/test/integration/`, named
  `*.int-spec.ts`. `globalSetup` boots one Postgres + one Redis
  container per Jest run (`@testcontainers/postgresql`,
  `@testcontainers/redis`) and publishes their connection URLs onto
  `process.env`. Each `.int-spec.ts` boots a NestJS TestingModule
  that picks the env up via the same `ConfigService` path production
  uses, so a schema typo or invalid env value fails the same way at
  test time and at boot.
  `maxWorkers: 1` so the container set is shared across tests
  instead of forked. 2-minute test timeout absorbs first-time image
  pulls on a cold Docker cache.

### S26. `-race -count=10` on Go test runs

Catches data races + tests are flake-free at 10× iterations.

### S26b. OpenAPI spec is checked in + drift-detected

- `backend/openapi.json` (50 KB at this writing) is generated from the
  NestJS Swagger config and committed to the repo.
- `npm run openapi:generate` — boots an ephemeral Postgres via
  testcontainers, builds the AppModule, dumps Swagger's document to
  disk, exits.
- `npm run openapi:check` — regenerates + `git diff --exit-code`
  fails the check when a controller / DTO / decorator change drifts
  the wire shape. Designed to gate PRs in CI; intentional API
  changes show up as a reviewable diff in `openapi.json`.
- Adding a new endpoint = run `npm run openapi:generate` + commit
  the updated spec alongside the controller change. No drift = no
  surprise.
- Clients (admin SPA, Flutter, Swift, Windows, Linux) regenerate
  types from this file when they want updated bindings.

## 12. Naming + path conventions

### S27. Container naming

```
draftright-<role>-<replica-index>
```

Examples:
- `draftright-backend-1` (NestJS)
- `draftright-rewrite-go-1` (Go)
- `draftright-postgres-1`
- `draftright-redis-1`

When horizontally scaling, indices grow (`-2`, `-3`).

### S28. Docker network

Every service joins `draftright_default`. External services join via
`networks: { draftright_default: { external: true } }`.

### S29. Host paths

```
/opt/draftright/             NestJS backend + .env + Dockerfile
/opt/draftright-rewrite-go/  Go rewrite + .env + Dockerfile
/etc/caddy/Caddyfile         Edge config
/var/log/caddy/              Caddy access logs
/var/lib/draftright/         Per-service data dirs (bug-report uploads, etc.)
```

### S30. Endpoint conventions

- `/health` — public probe, returns JSON `{ status, service, version }`.
- `/metrics` — Prometheus, internally gated.
- `/v1/` — versioned API surface.
- Unversioned legacy routes (`/rewrite`, `/auth/*`) remain operational
  during the strangler-fig migration.

## 13. Scaling tier playbook

### Tier 1: 1–500 DAU (today)

Compose + Caddy. Single host. **Don't add anything else.**

### Tier 2: 5k DAU

Add:
- Loki + Promtail for log shipping.
- Prometheus scraping `/metrics`.
- Per-route Caddy rate limit (`caddy-rate-limit` plugin).
- Compose `replicas: 2` for backend + rewrite-go.
- `app.enableShutdownHooks()` on NestJS for graceful SIGTERM.

### Tier 3: 50k DAU / multi-host

Migrate to:
- k3s cluster (3-5 nodes).
- Traefik or NGINX Ingress (replace Caddy).
- Longhorn / Ceph persistent volumes.
- Linkerd or Cilium service mesh.
- Redis Streams / NATS for async work.

### Tier 4: 500k+ DAU / regional

Multi-region, Spanner-class DB, service mesh mandatory. Different
playbook.

## 14. Anti-recommendations (don't do these now)

- **Kong / Envoy / Istio** — operational tax exceeds value below ~5
  services. Caddy covers 70 % of the gateway role.
- **Microservices for the sake of it** — extracting a NestJS module
  into its own service is only worth it when the module needs
  independent scaling, deployment cadence, or a different stack.
- **Custom RPC protocols** — REST + JSON until protocol overhead
  becomes >10 % of latency. Then evaluate gRPC.
- **Premature ORM swap** — TypeORM in NestJS, sqlc in Go. Each fits
  its stack. Don't unify.

## 15. Change protocol

Standards in this file get updated when:

1. A pull request introduces a new pattern that future code should
   follow.
2. A scaling tier crosses (e.g. 1k DAU → 5k DAU triggers Tier 2
   additions).
3. A retrospective identifies a recurring footgun that a standard
   would prevent.

PRs touching this file MUST also update the corresponding code (or
explicitly justify documentation-only changes).
