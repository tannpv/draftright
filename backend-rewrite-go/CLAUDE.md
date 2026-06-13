# DraftRight Go Backend (backend-rewrite-go)

Go port of the NestJS backend — byte-identical drop-in replacement.
Goal: collapse the 653MB Node image to a ~40MB Go distroless binary.
Master design: `docs/superpowers/specs/2026-06-13-go-backend-port-design.md`.

**Hard requirement:** identical routes, JSON, status codes, JWT (HS256).
Shadow-compare against Node before any Caddy path flip. Ship nothing to
prod without the live shadow gate passing.

## Tech Stack

| Concern | Choice |
|---|---|
| Language | Go 1.26 |
| Router | chi/v5 |
| DB | pgx/v5 + pgxpool, sqlc v1.31 |
| Auth | golang-jwt/jwt/v5 (HS256), x/crypto/bcrypt (cost 10) |
| Logging | slog |
| Deploy | distroless static binary |

## Architecture — Clean Architecture, package-by-feature

Each feature module under `internal/<feature>/` is a **self-contained
Clean Architecture vertical slice** (its own entities → use case → ports
→ adapters). The app = many small clean-arch modules composed at
`cmd/server/main.go` (the Main component).

```
internal/
  auth/ user/ subscription/ usage/   ← feature modules (clean arch each)
  rewrite/                            ← feature module (layered, N adapters)
  platform/   jwt signer, config, db pool, metrics, settings  ← shared drivers
  shared/     router, error envelope, sqlc-generated queries  ← cross-cutting
cmd/server/main.go                    ← composition root (wires everything)
```

### The two dependency rules (non-negotiable)

1. **Inside a module — dependencies point inward.**
   `handler.go → usecase.go → domain.go`. Never outward. A use case must
   NOT import `pgx`, `chi`, or `net/http`. It works against its own port
   interface, never a concrete adapter.

2. **Between modules — talk through exported interfaces/types only.**
   A module that needs a sibling declares a small port interface in its
   OWN `usecase.go`; `main.go` injects the sibling's concrete. Example:
   `auth` declares `UsageCounter` (1 method) and `main.go` passes the real
   `usage.Counter`. A module NEVER imports another module's constructor or
   unexported guts. `auth → subscription/user/usage` via **exported data
   types** (`user.User`, `subscription.AccountSub`) is allowed; coupling to
   `internal/rewrite` or `internal/core` internals is forbidden.

`main.go` is the ONLY place that names concrete constructors. That is
correct — the composition root is exempt; it knows everyone, is known by
no one.

### Module layout — flat vs layered

Scale the physical shape to the module's size. Same architecture either way.

| Signal | Shape | Example |
|---|---|---|
| Layer has **1 implementation** (one repo, one transport) | **flat files** in one package | `auth`, `user` |
| Layer has **N implementations** (multiple providers/stores) | **subfolder packages** (compiler-enforced boundaries) | `rewrite/adapter/{openai,anthropic,ollama,...}` |

**Flat** = `domain.go` / `usecase.go` / `handler.go` / `repo_pg.go` in one
package. Layers held by discipline (same package = no compiler barrier).

**Layered** = `domain/` `usecase/` `transport/` `adapter/*` each its own
package. Layers held by the compiler (cross-layer poke = build error).

**Project rule: start every new feature FLAT. Promote to layered only when
a layer gains a 2nd implementation OR a test fake that needs isolation.**
Do NOT pre-scaffold empty layer-folders — single-file layer-folders are
ceremony that buys no enforcement (YAGNI). Promotion is a pure packaging
move (no logic change): mv files into sub-packages, keep ports in
`usecase/`, both impls satisfy them, `main.go` picks one.

### Idiomatic Go guardrails (avoid premature abstraction)

- **Accept interfaces, return structs.** Constructors return concrete
  `*PgRepo`; consumers declare the interface.
- **Interfaces on the consumer side, kept small** (1–3 methods). The
  module that USES a behavior owns its interface, not the implementer.
- **Add an interface only when there's a 2nd impl OR a test fake** — not
  before. A single-impl interface with no fake is a smell; collapse it.
- **Hand-wire `main.go`.** No DI framework unless wiring genuinely
  outgrows it.
- `gofmt` decides style — never bikeshed it.

## Adding a new feature (the recipe)

1. `internal/<feature>/domain.go` — exported data types + sentinel errors,
   zero deps.
2. `internal/<feature>/usecase.go` — `Service` + its OWN port interfaces
   (`Repo`, plus any sibling ports it needs). `NewService` accepts
   interfaces.
3. `internal/<feature>/repo_pg.go` — `NewPgRepo` returns a concrete that
   satisfies the `Repo` port.
4. `internal/shared/pg/queries_<feature>.sql` — add queries, run
   `sqlc generate`.
5. `internal/<feature>/handler.go` — HTTP edge; calls the Service only.
6. `*_test.go` beside the code — fakes satisfy the ports, no DB.
7. Wire in `cmd/server/main.go`: build repo → service → handler, **reuse
   existing shared collaborators** (e.g. one `usage.Counter` for all
   features — extend, don't fork), assign the route fields.
8. Add the route field to `coreHandlers` + mount in `shared/router.go`.

## Parity invariants (byte-identical port)

- **JWT clock tolerance MUST be zero** — Node `jsonwebtoken` default. Any
  leeway makes Go accept tokens Node rejects on the expiry boundary = a
  parity BUG.
- **Dual-secret JWT:** access signed `JWT_SECRET`, refresh signed
  `JWT_REFRESH_SECRET`. TTLs from `app_settings` (token_expiry_minutes
  default 15, refresh_token_expiry_days default 90).
- bcryptjs hashes (`$2a`/`$2b`) verify natively in x/crypto/bcrypt — no
  migration.
- Error strings are compared byte-for-byte by the shadow gate — match the
  NestJS `UnauthorizedException` messages exactly (see `auth/errors.go`).
- POST status codes match Node exactly (e.g. login → 201, refresh → 200).
  Don't assume REST conventions; mirror the controller.

## Commands

```bash
go build ./...            # compile all (no -o → strays land in cwd; gitignored)
go test ./... -race       # full suite, race detector
gofmt -l .                # must print nothing
go vet ./...              # must be clean
sqlc generate             # regen internal/shared/pg/sqlc after SQL changes
```

`go build ./...` caches aggressively and can mask a syntax error in a
package whose object is cached. After a suspicious edit, prefer
`go test ./...` (rebuilds for the test binary) or `go clean -cache`.

## Gotchas

- Never commit compiled binaries (`/server`, `/shadowdiff`) — both
  gitignored; they fall out of `go build ./...` / `go build ./cmd/...`.
- Stray non-ASCII / stray chars in `main.go` can be hidden by the build
  cache — `go build ./...` may report OK while `go test ./...` fails the
  package build. Trust the test build.
