# Go Backend Port — Phase 0: Parity Foundation

**Date:** 2026-06-13
**Status:** Design approved. Plan to follow (writing-plans).
**Parent:** `2026-06-13-go-backend-port-design.md` (master design)
**Service:** `backend-rewrite-go/` (module `github.com/tannpv/draftright-rewrite`)

## 1. Purpose

Phase 0 establishes the foundation every later module copies: the modular
package layout, the shared infrastructure (config, DB, JWT, error envelope,
request-id), and the **shadow-compare harness** that proves Go responses are
byte-identical to Node before any path is flipped.

Phase 0 ships no business feature. It proves the plumbing end-to-end through two
endpoints — `GET /health` (no auth, no DB-required) and `GET /auth/me` (JWT +
DB) — so config → DB → JWT → handler → envelope → shadow-diff is exercised on a
real authed request before any money/auth module is built.

No production traffic exists yet (no public launch), so the harness replays
**author-controlled fixtures** against Node + Go on dev; there is no live mirror
and no double-write risk.

## 2. Starting point (what already exists)

The Go `rewrite` service is already built, but **layer-first** (`internal/domain`,
`internal/usecase`, `internal/http`, `internal/adapter`, `internal/platform`).
It already provides, reusable as-is:

- chi router, pgx pool, `sqlc` (config `sqlc.yaml`, `internal/platform/db/schema.sql`,
  generated `internal/adapter/pg/sqlc/`), golang-jwt v5.
- `internal/platform/auth` — HS256 `Verifier` that accepts Node-signed tokens
  (same `JWT_SECRET`), rejects `alg=none`/expired/malformed via sentinels.
- `internal/platform/config` — typed `Config` mirroring the Node `.env` names.
- `internal/http/errors.go` — `httpError{error, code}` + `statusForDomainErr`.
- OTel tracing + Prometheus metrics, request logging.

Phase 0 is therefore **parity-fixing + restructuring**, not greenfield.

## 3. Parity gaps to close (measured against live Node)

These are facts derived from the Node source, not choices. Drop-in requires each
to match exactly.

| Piece | Node (`backend/src`) | Go today | Phase 0 fix |
|---|---|---|---|
| Error envelope | `{error, code, request_id}` (`common/all-exceptions.filter.ts`) | `{error, code}` — no `request_id` | add `request_id` field + request-id middleware |
| Status map | `httpStatusForCode` (`common/error-codes.ts`): invalid-input 400, invalid-token 401, user-not-found 401, quota-exceeded 402, forbidden 403, not-found 404, conflict 409, rate-limited 429, provider-failed 502, provider-unavailable 503, else 500 | partial; `errors.go` comment **falsely claims** quota diverges to 429 — Node also uses 402 | reconcile `statusForDomainErr` to the Node table byte-for-byte; delete stale comment; add missing sentinels (invalid-token, forbidden, not-found, conflict) |
| JWT claims | `{sub, email, role}` (`auth/auth.service.ts` `generateTokens`) | `{sub, role}` — no `email` | add `Email` to `auth.Claims` |
| `/health` | `{app:"draftright", version, status:"ok", client_log_level}` (`health/health.controller.ts`); DB read of `app_settings.client_log_level`, cached 30s, DB-tolerant | rewrite-only health | match shape incl. `client_log_level`, same DB-tolerant fallback |
| `/auth/me` | `{id, email, role, flags:{use_go_backend}}` (`auth/auth.controller.ts`); `use_go_backend` from `FeatureFlagsService` bucket over `GO_BACKEND_RAMP_PERCENT` | none | build via JWT → sqlc user → flags; mirror the bucket algorithm |

### JWT lifetimes / issuance

Node signs access tokens with `expiresIn = app_settings.token_expiry_minutes`
(default 15m) and refresh with `refresh_token_expiry_days` (default 90d), payload
`{sub, email, role}`. Phase 0 only **verifies** Node tokens on the request path
(`/auth/me`). It also adds a **signer util** that issues claims identical to
Node (same payload + exp sourced from `app_settings`) so Phase 1 login drops in —
but no login endpoint ships in Phase 0. The signer is unit-tested by round-trip
(sign in Go → verify in Go) and by Node-verifies-Go / Go-verifies-Node parity.

## 4. Architecture decision — restructure in Phase 0

The master design's target is **modular / package-by-feature**. The existing code
is layer-first. **Decision (approved): restructure in Phase 0, as the first
task**, so `internal/rewrite/` becomes the real template every later module
copies. The move is mechanical (no behavior change); the existing `rewrite` unit
tests staying green is the proof of correctness.

### Target layout after Phase 0

```
backend-rewrite-go/
├── cmd/
│   ├── server/main.go          # composition root
│   └── shadowdiff/main.go      # NEW — shadow-compare replay tool
├── internal/
│   ├── rewrite/                # MOVED from layer-first → the template
│   │     domain.go usecase.go handler.go
│   │     adapter/{anthropic,openai,ollama,chain,memory}
│   ├── core/                   # NEW — Phase 0 proof endpoints
│   │     health.go             #   GET /health
│   │     me.go                 #   GET /auth/me (→ moves to internal/auth/ in Phase 1)
│   ├── platform/               # unchanged — config db auth logger metrics tracing redis
│   └── shared/                 # NEW — cross-module app concerns
│         errenvelope.go        #   {error, code, request_id} + status map
│         requestid.go          #   request-id middleware
│         middleware_auth.go    #   bearer → Claims → context (moved from http/)
│         pg/sqlc/              #   generated DB types over the whole schema
```

`platform/` (infra, no feature logic) and `shared/` (envelope, request-id, auth
middleware, sqlc types) are the two deliberate shared packages from the master
design. Dependency rule holds: `module → platform`, `module → shared` (downward);
no cross-module `internal` imports.

The Phase 0 endpoints `/health` and `/auth/me` are small enough that they live in
a thin `internal/core/` (or `internal/health` + folded into a future `auth`
module) — to avoid pre-creating a half-empty `auth` module in Phase 0, `/auth/me`
ships in `internal/core/` and **moves into `internal/auth/` in Phase 1** when the
real auth module lands. `/health` stays in `internal/core/`. This keeps Phase 0
from inventing module boundaries it can't yet fill.

## 5. Deliverables

1. **Restructure** layer-first → `internal/rewrite/` + create `internal/shared/`;
   all existing `rewrite` tests green (proof the move is behavior-preserving).
2. **sqlc** regenerated over the live schema for Phase 0 tables (`users`,
   `app_settings`); `sqlc.yaml` pinned; generated types in `shared/pg/sqlc`.
3. **Error envelope + request-id** — `request_id` added to the wire struct; a
   request-id middleware mints a UUID per request, stamps the response, threads
   it into logs; `statusForDomainErr` reconciled to the Node status table
   (table-driven test asserts every code→status pair).
4. **JWT compat** — `Email` added to `Claims`; signer util issuing Node-identical
   claims (payload + exp from `app_settings`); parity tests both directions.
5. **`GET /health`** — `{app, version, status, client_log_level}`, DB-tolerant
   30s-cached `client_log_level`, throttle-exempt.
6. **`GET /auth/me`** — JWT → sqlc user load → `{id, email, role, flags:{use_go_backend}}`;
   `use_go_backend` mirrors the Node `GO_BACKEND_RAMP_PERCENT` bucket.
7. **`cmd/shadowdiff`** — standalone replay tool: reads fixture request files,
   fires each at the Node base URL and the Go base URL, deep-diffs HTTP status +
   JSON body, prints mismatches and exits non-zero on any diff. Ships with
   `/health` and `/auth/me` fixtures (the `me` fixture carries a real
   Node-issued bearer token).

## 6. Shadow-compare harness detail

- **Input:** a directory of fixture files, each describing one request — method,
  path, headers (incl. a bearer token where needed), optional body.
- **Run:** `shadowdiff --node=https://api.dev.draftright.info --go=http://localhost:3001 --fixtures=./fixtures`.
- **Compare:** status code must match; JSON bodies deep-equal after normalising
  fields that legitimately differ per request (`request_id` is compared for
  *presence + shape* — a non-empty UUID — not value, since it's per-request).
- **Output:** per-fixture PASS/FAIL with a unified diff of the mismatch; process
  exits non-zero if any fixture fails. This is the Phase 0 done-gate.
- **Determinism:** fixtures are authored + checked in; no live traffic tap. Seed
  the corpus from real dev access logs later if useful.

## 7. Testing strategy

- **Restructure:** existing `rewrite` unit tests must pass unchanged post-move.
- **Unit:** request-id middleware (UUID minted + echoed on response), envelope
  (table-driven code→status parity), JWT (verify Node token; round-trip
  sign→verify; reject `alg=none`, expired, missing `sub`), sqlc queries against a
  dev Postgres.
- **Integration gate:** `shadowdiff` green for `/health` + `/auth/me` against live
  Node on dev. Authed `/auth/me` fixture proves cross-issuer JWT validity in the
  same run.

## 8. Task order

1. Restructure → `internal/rewrite/` + `internal/shared/`; rewrite tests green.
2. sqlc regen over live schema (`users`, `app_settings`).
3. Envelope `request_id` + request-id middleware + reconciled status map (+ table test).
4. JWT: add `email` claim + signer util (+ parity tests).
5. `GET /health` (DB-tolerant, `client_log_level`).
6. `GET /auth/me` (JWT → sqlc user → flags).
7. `cmd/shadowdiff` + `/health` & `/auth/me` fixtures → green diff vs Node.

## 9. Done criteria

- All existing rewrite tests + new unit tests green.
- `go vet` / build clean; modular layout in place; no cross-module `internal` imports.
- `shadowdiff` reports PASS for `/health` and `/auth/me` against dev Node.
- No schema migration introduced; no production change; nothing flipped in Caddy.

## 10. Out of scope (deferred to later phases)

- Any login / register / refresh endpoint (Phase 1 `auth`).
- Moving `/auth/me` into a real `internal/auth/` module (Phase 1).
- Redis, rate-limiting, AI providers on the request path beyond the existing
  rewrite wiring (Phase 1+).
- The `draftright-rewrite` healthcheck fix (issue #28 — independent).
