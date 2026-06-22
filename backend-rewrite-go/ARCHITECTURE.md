# DraftRight Go Backend — Architecture & Reading Guide

A map over the maze. The code is Clean Architecture (package-by-feature);
that buys modularity at the cost of indirection. This doc buys back the
*local* readability so a new dev can fix a bug without grepping blind.

Read this once, then every module is predictable.

---

## 1. The layer model

Dependencies point **inward only** (Uncle Bob's Dependency Rule).

```
  Drivers (chi, pgx, smtp)         ← outermost, swappable tech
    └─ Adapters (handler.go, repo_pg.go)
         └─ Use Cases (usecase.go — Service)
              └─ Entities (domain.go)   ← innermost, zero deps
```

| Layer | File | May import | Must NOT import |
|---|---|---|---|
| Entities | `domain.go` | stdlib only | pgx, chi, net/http |
| Use Cases | `usecase.go` (`Service` + ports) | domain + own port interfaces | pgx, chi, net/http |
| Adapters | `handler.go` (in), `repo_pg.go` (out) | use case + framework | — |
| Drivers | chi router, pgxpool | — | — |

**Enforced, not aspirational.** The audit greps prove it: no `usecase.go`
imports pgx/chi/http; every cross-module call goes through an exported
interface; concretes are named only in `main.go`.

---

## 2. Directory roles

```
backend-rewrite-go/
  cmd/server/main.go     ← composition root (the ONLY place naming concretes)
  internal/              ← Go compiler-enforced privacy wall (app-private)
    auth/ user/ payment/ subscription/ usage/ …  ← FEATURE modules
    rewrite/             ← feature module, LAYERED (N AI providers)
    core/                ← Phase-0 migration scaffolding (health, /auth/me, feature-flag ramp)
    platform/            ← shared DRIVERS (jwt signer, config, db pool, metrics, settings)
    shared/              ← cross-cutting plumbing (router, error envelope, sqlc queries)
```

| Dir | Role | Shared? |
|---|---|---|
| `internal/<feature>/` | one self-contained clean-arch slice | no — talk via exported types/interfaces |
| `platform/` | technical drivers every feature may use | yes |
| `shared/` | router, error envelope, sqlc-generated SQL | yes |
| `core/` | transitional port scaffolding — **do NOT import from features** | no (fenced) |
| `cmd/server/main.go` | wires everyone; known by no one | — |

`internal/` is a **Go language privacy boundary**, not a clean-arch ring:
code under it is importable only within this module. The rings live
*inside each feature folder*.

---

## 3. Flat vs layered — when a feature grows directories

Same architecture, two physical shapes. Scale the shape to the module.

| Signal | Shape | Boundary held by | Example |
|---|---|---|---|
| 1 impl per layer (one repo, one transport) | **flat files**, one package | discipline | `auth`, `payment`, `user` |
| N impls of a layer (multi-provider) | **sub-package per impl** | the compiler | `rewrite/adapter/{openai,anthropic,ollama,…}` |

**Trigger to promote = a layer gains a 2nd implementation (or a test fake
needing package isolation). NOT file count.** `auth` has 8 files in one
flat package — still flat, because it has one repo. Don't split a
single-impl layer into a folder: a one-file package buys a compiler wall
around nothing (ceremony, YAGNI).

Promotion is a pure packaging move: `mv` files into sub-packages, keep
ports in `usecase/`, `main.go` picks the impl. No logic change.

---

## 4. "Where do I fix X?" — request-trace maps

### POST /auth/login
```
shared/router.go            route mounted
  → auth/handler.go         Login()          parse body, call service, map errors
    → auth/usecase.go       Service.Login()  THE business flow (verify pw, sign JWTs)
      → UserService port    → user/repo_pg.go (DB read)
      → TTLReader port      → platform/settings (token TTLs)
  error strings             auth/errors.go   (byte-identical to Node)
```

### POST /v1/rewrite
```
shared/router.go
  → rewrite/transport/handler_rewrite.go     HTTP edge, SSE streaming
    → rewrite/usecase/                        orchestration + trial limit
      → rewrite/adapter/{openai|anthropic|ollama}/   provider picked by config
  prompts/tones                rewrite/parity/
```

### POST /payment/checkout
```
shared/router.go
  → payment/handler_checkout.go              parse {plan_id, method}
    → payment/usecase.go      Service         resolve plan, dispatch by method
      → payment/strategy/<method>/            stripe | lemonsqueezy | vietqr | …
        (each strategy = one CreateCheckout impl behind strategy.Strategy port)
```

**Rule of thumb:** start at `shared/router.go` to find the handler, then
follow `handler.go → usecase.go → port → adapter`. Business logic always
lives in `usecase.go`. The edge (`handler.go`) only parses + maps.

---

## 5. Adding a feature (the recipe)

1. `domain.go` — exported types + sentinel errors, zero deps.
2. `usecase.go` — `Service` + its OWN port interfaces (`Repo` + any sibling
   ports). `NewService` accepts interfaces.
3. `repo_pg.go` — `NewPgRepo` returns a concrete satisfying `Repo`.
4. `shared/pg/queries_<feature>.sql` — add queries, `sqlc generate`.
5. `handler.go` — HTTP edge; calls the Service only.
6. `*_test.go` — fakes satisfy the ports, no DB.
7. Wire in `main.go`: repo → service → handler; **reuse existing shared
   collaborators** (one `usage.Counter` for all features — extend, don't fork).
8. Mount the route in `shared/router.go`.

Start FLAT. Promote to layered only when step 3 gains a 2nd impl.

---

## 6. Rule #1 conventions (clean / extendable / reusable / no-hardcoding)

**No hardcoding — config is the single source of truth.**
- `os.Getenv` lives in **`platform/config/config.go` only** (+ `main.go`).
  No other package reads env. Adapters receive injected values.
- Per-env endpoints are **named consts beside their override var**, never
  inlined: `config.DefaultWebsiteURL` (override `WEBSITE_URL`),
  `config.DefaultIMEPackBase` (override `IME_PACK_BASE`). One literal, one place.
- Vendor API URLs (`api.openai.com`, `api.lemonsqueezy.com/v1`) are
  named `default*` consts in their adapter — constant by nature, override
  where the product allows.
- **Adapters never re-default an injected value.** config guarantees
  non-empty; a strategy that falls back to its own literal = duplication.

**Extendable / reusable.**
- Accept interfaces, return structs. Constructors return concrete
  `*PgRepo`; consumers declare the interface.
- Ports live on the **consumer** side, kept small (1–3 methods). The
  module that USES a behavior owns its interface.
- Add an interface only when there's a 2nd impl OR a test fake — never
  pre-abstract. A single-impl interface with no fake is a smell; collapse it.
- Adding a payment method / AI provider = 1 new strategy/adapter package +
  1 line in `main.go`. The use case never branches on the concrete.

**Verify these hold (CI-able):**
```bash
# config is the only env reader
grep -rl "os.Getenv" --include=*.go internal cmd | grep -v _test   # → only platform/config + main.go
# no usecase touches the framework
grep -lE '"github.com/jackc/pgx|go-chi|"net/http"' internal/*/usecase.go   # → empty
```

---

## 7. Why it reads "hard" — and the deal

Clean arch trades *local* readability (one file = whole story) for
*modular* readability (each piece swappable + DB-free testable). You can't
have both in the code. So the local readability is bought back **here** —
docs, the trace maps above, consistent file order — a map over the maze,
not a smaller maze.

Don't "simplify" by merging layers or deleting ports: that throws away the
testability and the compiler boundaries you paid for. Add a signpost, not
a shortcut.
