# Go Backend Port — Phase 5 Cutover Design

**Status:** approved (brainstorm 2026-06-19), pending implementation plan.
**Master design:** `/opt/openAi/DraftRight/docs/superpowers/specs/2026-06-13-go-backend-port-design.md` §5.
**Predecessors:** Phases 0–4c-4 all DONE + merged + pushed to `develop`. Every
route is ported; this phase proves parity end-to-end and flips production.

---

## 1. Goal

Prove the Go backend is byte-identical to Node across **all ~100 routes** under
real DB writes, then cut `api.draftright.info` over to Go and decommission the
653 MB Node image — keeping instant rollback throughout.

## 2. Locked decisions (from brainstorm)

| Decision | Choice |
|---|---|
| Spec scope | Full cutover A+B+C+D; the prod flip (D) is a **manual, gated runbook** — never run autonomously. |
| Write parity | **Two isolated DBs, reset per fixture.** Node and Go each own a DB; both cloned from one frozen template before every fixture → identical start state → real write parity. |
| Snapshot source | **Clone current `draftright_dev` once**, freeze as template, plus a small idempotent *augment* SQL for known auth creds + ≥1 representative row per entity. |
| Prod flip style | **All-at-once**, single Caddyfile edit + reload; rollback = revert the one block + reload (seconds). |
| Node teardown | **Time-boxed: 7-day clean soak**, Node stopped-but-present, then removed from compose. |

## 3. Non-goals

- No API redesign, no schema change (drop-in port).
- No live prod traffic mirroring (rejected in brainstorm — extra infra, the
  per-fixture write-parity gate already covers all paths).
- No automation of the prod Caddy flip or Node teardown — those are manual
  runbook steps gated on a green gate and human go-ahead.

---

## 4. Architecture

Three-layer shadow rig on the dev VPS (`129.212.208.248`), then a gated prod flip.

```
                  ┌─ Node-shadow  → DB: draftright_shadow_node ┐
  shadowdiff ──── ┤                                             ├── both DROP+CREATE
  (reset+replay)  └─ Go-shadow    → DB: draftright_shadow_go   ┘    per-fixture from
                                                                    draftright_shadow_tmpl
                       maintenance conn → postgres DB (terminate/drop/create)
```

- **Template DB** `draftright_shadow_tmpl` = `draftright_dev` clone + `augment.sql`,
  built once by `deploy/shadow/make-template.sh`. Frozen for the run.
- **Node-shadow** and **Go-shadow** = two dev containers, identical env to
  `backend-dev` except `DATABASE_URL` points at their own DB and a distinct host
  port. Same `JWT_SECRET`/`JWT_REFRESH_SECRET` → one minted token valid on both.
- **Reset per fixture:** on a maintenance connection to the `postgres` database,
  for each target DB:
  ```sql
  SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1;
  DROP DATABASE IF EXISTS <db>;
  CREATE DATABASE <db> TEMPLATE draftright_shadow_tmpl;
  ```
  Backends' pools (pgxpool / Node pg) auto-reconnect on the next query. Identical
  template → identical pre-state for both backends on every fixture, so a POST/
  PATCH/DELETE applies from the same starting point on each side and the
  responses are directly comparable.

### Why DROP/CREATE works while backends are connected

`DROP DATABASE` requires zero active connections. The maintenance step
`pg_terminate_backend`s any open sessions to the target DB first; the backend's
connection pool then re-dials lazily against the freshly created DB. This is the
single riskiest infra assumption and gets a dedicated verification task (§7,
build step 2).

---

## 5. Components

| Unit | Responsibility | New / extend |
|---|---|---|
| `cmd/shadowdiff/main.go` | + token bootstrap, + template-var substitution, + `--reset` orchestration, + route-coverage assertion | extend |
| `cmd/shadowdiff/reset.go` | reset-SQL generation + maintenance-conn executor (terminate/drop/create) | new |
| `cmd/shadowdiff/subst.go` | replace `{{user_token}}`/`{{admin_token}}`/`{{ext_token}}` in fixture headers/body | new |
| `cmd/shadowdiff/fixtures/<module>/*.json` | one fixture per route, grouped by module | new |
| `deploy/shadow/docker-compose.shadow.yml` | `backend-node-shadow` + `backend-go-shadow` services (own DB + port) | new |
| `deploy/shadow/make-template.sh` | clone `draftright_dev` → apply `augment.sql` → create `draftright_shadow_tmpl` | new |
| `deploy/shadow/augment.sql` | idempotent: known user/admin creds, one ext-token row, ≥1 row per entity asserted by read fixtures | new |
| `deploy/shadow/run-gate.sh` | build template → up both shadow containers → run shadowdiff → report | new |
| `deploy/phase5-cutover-runbook.md` | prod flip + soak + teardown, manual/gated | new |

### Harness extensions in detail

- **Token bootstrap** (run once, before fixtures): POST `/auth/login` and
  `/admin/auth/login` against **Node-shadow** with the augment creds; mint an
  extension token via `/auth/extension-tokens`. Capture the access tokens into a
  substitution map. Tokens are minted fresh each run (never committed) so expiry
  is a non-issue. Same secret means they verify on Go-shadow too.
- **Substitution:** a fixture header `"Authorization": "Bearer {{user_token}}"`
  (or `{{admin_token}}` / `{{ext_token}}`) is expanded before send. Body
  substitution supported for the rare authed-id-in-body case.
- **Reset mode:** `--reset` enables per-fixture DROP/CREATE of both DBs from the
  template via the maintenance DSN (`--maint-dsn`, `--template`, `--db-node`,
  `--db-go`). Without `--reset`, behavior is the legacy stateless replay (kept
  for read-only smoke runs).
- **Coverage assertion:** harness loads the canonical route list
  (`deploy/shadow/routes.txt`, generated from `router.go`) and fails loudly if
  any route lacks a fixture — prevents a silent gap masquerading as a green gate.

### Fixture shape (unchanged schema, existing `fixture` struct)

```json
{
  "name": "auth_login_ok",
  "method": "POST",
  "path": "/auth/login",
  "headers": { "Content-Type": "application/json" },
  "body": { "email": "shadow-user@draftright.info", "password": "ShadowPass123" },
  "ignore_value_of": ["request_id", "access_token", "refresh_token"]
}
```

`ignore_value_of` covers request-time values (tokens, `request_id`, server-set
`created_at`/`updated_at`/`now()` timestamps) — compared for presence, not value.

---

## 6. Data flow (per fixture)

1. Reset `draftright_shadow_node` + `draftright_shadow_go` from the template.
2. Substitute `{{…}}` tokens in the fixture.
3. Replay against Node-shadow and Go-shadow.
4. Diff: status code, then JSON deep-compare honoring `ignore_value_of`.
5. PASS / FAIL line; non-zero process exit on any FAIL (existing behavior).

---

## 7. Build order (one implementation plan)

1. **Harness extensions** — `subst.go`, `reset.go`, bootstrap, coverage
   assertion, `--reset`/`--maint-dsn` flags. Unit tests: substitution,
   reset-SQL generation, coverage-gap detection, bootstrap parsing. No infra.
2. **Dev shadow infra** — `docker-compose.shadow.yml`, `make-template.sh`,
   `augment.sql`, `run-gate.sh`. **Verify the DROP/CREATE-while-connected
   assumption** on the real dev Postgres here (smoke: stand up Go-shadow, reset
   its DB mid-run, confirm pool reconnects and a follow-up request succeeds).
3. **Fixture authoring** — ~100 routes, batched per module: auth, plans,
   payment(public+authed+5 webhooks), updates/ime-packs, errors-ingest,
   email-webhook, bug-reports-ingest, feedback, rewrite(ext+jwt), extract,
   subscription, auth-account/ext-tokens, admin-auth, admin ai-providers,
   admin settings, admin plans, admin users, admin-users, admin email,
   admin stats/analytics/transactions, admin training-data, admin payments,
   admin errors, admin bug-reports, admin inbox, admin releases/policies,
   admin grant. Parallelizable per module (independent fixture files).
4. **Run gate → green** — `run-gate.sh`, fix every diff. Payment webhooks need
   valid per-provider signatures replayed (Stripe/LS/Casso/SePay/VietQR).
5. **Prod cutover runbook** — write `deploy/phase5-cutover-runbook.md` (manual,
   gated; see §8). No code; the flip is operator-executed.

## 8. Prod cutover runbook (section D — manual, gated, never autonomous)

Preconditions: gate 100% green on dev, route-coverage assertion passes, human
go-ahead.

1. **Build + ship Go prod image** — build `backend-rewrite-go` image, add a
   `backend-go` service to the prod compose pointed at the **prod** DB + secrets
   (via `--env-file`, not inline — special chars), on an unused host port. Bring
   it up **alongside** Node; smoke `/health`.
2. **Flip** — edit the `api.draftright.info` Caddy block `reverse_proxy` target
   from the Node port to the Go port; `systemctl reload caddy`.
3. **Payment webhooks** — confirm provider webhook URLs are path-only
   (`/payment/webhook/*`) so the Caddy flip re-points them automatically; if any
   provider targets a port/host directly, update it.
4. **Soak** — watch Go logs + `/health` + container status. **Rollback** at any
   sign of trouble: revert the Caddy block to the Node port + reload (seconds);
   Node never stopped during soak.
5. **Teardown** — after **7 days** with zero rollbacks: stop Node, confirm Go
   steady, then remove the Node service from prod compose and prune the image.

Post-flip: apply the GitHub issue-lifecycle label updates per `~/.claude/CLAUDE.md`
and add a `## ✅ How to Verify` comment to the tracking issue.

---

## 9. Risks & mitigations

| Risk | Mitigation |
|---|---|
| `DROP DATABASE` blocked by live connections | `pg_terminate_backend` before drop; dedicated verification in build step 2. |
| Payment parity (webhooks, signatures, idempotency, 5 providers) | Most fixtures + valid signed payloads per provider; flips under the same gate, no special-casing. |
| Token expiry mid-run | Mint fresh at gate start; never commit static tokens. |
| Silent route-coverage gap → false green | Coverage assertion fails the run if any router route lacks a fixture. |
| Reset perf (≈200 DROP/CREATE per run) | Dev DB is small; template clone is fast. If too slow, gate reads first under one reset, then writes each reset (optimization, not correctness). |
| Cron jobs (expiry, AI fix-proposal) differ | Out of the request-path gate; verify on dev against Node behavior before flip (separate manual check noted in runbook). |

## 10. Testing

- **Harness unit tests** beside the code: substitution, reset-SQL generation,
  coverage-gap detection, bootstrap token parsing (extend existing
  `diff_test.go` / `token_verify_test.go`).
- **The gate is the integration test:** green = parity proven across all routes.
- **Infra smoke** (build step 2): reset-while-connected reconnect proof.

## 11. Standing rules (carried from prior phases)

- GitFlow: branch from `develop`, `--no-ff` merges only, never commit direct to
  main/develop, push/merge only when asked.
- No prod hits, no invented creds, no real JWT/bearer committed. Shadow tokens
  minted at runtime from augment creds; augment creds are dev-only test values.
- Subagent-driven execution: fresh implementer per task → spec review → quality
  review → fix loop. Fixture authoring parallelizable per module. Every
  implementer prompt: "stage ONLY this task's files, NEVER `git add -A`".
- Node remains the parity authority — Node wins every disagreement.
