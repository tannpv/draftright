# Go Backend Port — Master Design

**Date:** 2026-06-13
**Status:** Design approved (architecture + decomposition). Phase specs to follow.
**Visual companion:** `2026-06-13-go-backend-architecture.html` (open in a browser)

## 1. Motivation

The NestJS backend image is **653 MB** (279 MB `node_modules`); the existing Go
`draftright-rewrite` service is **~40 MB** on distroless — ~16× smaller, with
lower memory and instant cold start. We will port the whole backend to Go to
collapse the footprint and unify on one runtime.

## 2. Decisions (locked)

| Decision | Choice |
|---|---|
| Cutover style | **Big-bang** end state, but **built incrementally and run in parallel** with Node, flipped per-path once shadow-compared to parity. |
| Where it lives | **Extend the existing `draftright-rewrite` service** (reuse its clean-arch foundation + adapters — Rule #1). |
| API/DB contract | **Exact drop-in replacement** — identical routes, JSON, status codes, JWT (HS256); reuse the live Postgres schema as-is via `sqlc`. All 7 clients unchanged. |
| Top-level structure | **Modular / package-by-feature** (one package per NestJS module), clean-architecture *inside* each module. |

## 3. Architecture

**Modular monolith with clean architecture per module.** Top level is sliced by
feature, not by layer. Inside every module the dependency rule holds: `domain`
at the center; `usecase` orchestrates; `handler` / `repo` / external `adapter`s
sit on the outside; dependencies point inward only. `cmd/server/main.go` is the
single composition root where concretes are wired.

A module maps 1:1 to a NestJS module, so the strangler-fig port lifts one folder
at a time.

### Shared packages (deliberate)

1. **`platform/`** — infra every module uses: config, DB pool, Redis, JWT auth,
   logger, metrics, tracing, cron scheduler. No feature logic.
2. **`shared/`** — cross-cutting app concerns: NestJS-shaped error envelope
   (`{error, code, request_id}`), request-id + auth middleware, and the
   `sqlc`-generated DB types as one package over the whole schema (FKs cross
   module boundaries).
3. **Reusable adapters** — LLM providers (`anthropic`/`openai`/`ollama`/`chain`)
   reused by `rewrite` and `bug_reports`; the Resend email adapter reused by
   `auth`, `payment`, `subscription`.

### Dependency rules (the anti-spaghetti guardrail)

- ✅ `module → platform`, `module → shared` (downward, stable)
- ✅ `module A → an interface that module B satisfies` (wired in the composition root)
- ❌ `module A → module B/internal` (no direct cross-feature imports)

### Directory layout

```
draftright-rewrite/
├── cmd/server/main.go         # composition root
├── internal/
│   ├── rewrite/               # ✓ done — the template
│   │     domain.go usecase.go handler.go adapter/{anthropic,openai,ollama,chain}
│   ├── auth/  user/  plan/  usage/  ai_providers/
│   ├── subscription/          # + expiry/nudge cron
│   ├── extension_token/
│   ├── payment/               # domain usecase handler registry webhook
│   │     adapter/{stripe,lemonsqueezy,sepay,paypal,banktransfer}
│   ├── admin/ bug_reports/ email/ errors/ updates/ ime_packs/ extraction/
│   ├── platform/   config db redis auth logger metrics tracing scheduler
│   └── shared/     errenvelope.go middleware.go requestid.go pg/sqlc
```

## 4. Module decomposition (5 phases)

Foundation-first, then identity → entitlements → money → admin. Each phase is its
own spec → plan → build → shadow-verify cycle. `rewrite` already ships.

- **Phase 0 — Parity foundation:** sqlc over the live schema; error envelope +
  status map; JWT compat (verify Node tokens, issue identical claims); typed
  config from the same env; request-id middleware; **shadow-compare harness**
  (mirror a request to Node + Go, diff responses).
- **Phase 1 — Identity & core:** `health`, `users`, `auth` (register/login/
  refresh/me/verify-email/forgot+reset/delete, social Google/Apple/Facebook),
  `ai_providers`, `usage`; fold in `rewrite`.
- **Phase 2 — Entitlements:** `plans` (currency-aware), `subscription` (assign,
  expiry cron, nudges), `extension_token` (`dr_ext_*`).
- **Phase 3 — Payment (highest risk):** payment core + registry; 5 strategy
  adapters (Stripe, LemonSqueezy, VietQR/SePay, PayPal, bank-transfer); webhooks
  with signature verification + idempotency; portal + cancel.
- **Phase 4 — Admin & ancillary:** `admin` (CRUD + analytics + settings),
  `bug_reports`/feedback (+ AI fix-proposal cron), `email` (Resend + templates +
  webhook + suppressions), `errors`, `updates` (`/updates/latest` + sha256),
  `ime_packs`, `extraction`.
- **Phase 5 — Cutover:** Caddy routes all paths → Go once parity is green; Node
  decommissioned.

## 5. Cutover strategy (de-risking the big bang)

Caddy already path-routes `/v1/rewrite` → Go. Go runs alongside Node. Before
flipping each path we **shadow-compare**: mirror real requests to both, diff
JSON/status, fix mismatches until clean. The "big bang" becomes flipping the
Caddy routes once the parity harness is green across all modules — instantly
reversible by reverting the Caddy block. Node stays warm until confidence, then
is decommissioned.

## 6. Cross-cutting concerns

- **Cron jobs** (subscription expiry, AI fix-proposal) → scheduler goroutines in
  `platform/scheduler`, not the app request path.
- **Secrets/config** stay in the same `.env`, read through typed config — no
  hardcoding (Rule #1).
- **JWT** stays HS256 with the same secret + claim shape so Node- and Go-issued
  tokens are mutually valid during the parallel window.
- **DB** is one shared Postgres; `sqlc` reads/writes the existing tables. No
  schema migration is introduced by the port itself.

## 7. Out of scope / non-goals

- No API redesign — drop-in only.
- No schema changes driven by the port.
- The `draftright-rewrite` healthcheck fix (issue #28) is independent.

## 8. Risks

- **Payment parity** is the hardest (webhooks, idempotency, 5 providers) — it
  gets the most shadow-compare time and ships last among core modules.
- **Subtle JSON/status mismatches** could break a client — the shadow-compare
  harness is the mitigation; nothing flips without a clean diff.
- **Cron correctness** (expiry, nudges) — verify against Node behavior on dev
  before flipping.

## 9. Next step

Write the **Phase 0 spec** (parity foundation) as the first implementation
document, then turn it into a step-by-step plan. Build and verify Phase 0 before
any money/auth module.
