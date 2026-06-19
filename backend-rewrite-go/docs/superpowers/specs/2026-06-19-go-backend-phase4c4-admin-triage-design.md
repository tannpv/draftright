# Phase 4c-4 — Admin Triage Slice (Go backend port) — Design

> Byte-identical NestJS → Go drop-in. Node is parity authority. Ships nothing
> to prod; shadow-gate before any Caddy flip. Part of the Go backend port
> (`backend-rewrite-go`, module `github.com/tannpv/draftright-rewrite`).

**Date:** 2026-06-19
**Phase:** 4c-4 — the **final admin slice**. Closes the entire admin surface
(55 admin routes total; 36 ported in 4c-1/4c-2/4c-3; **19 here**) plus the
**2 hourly AI fix-proposal crons**. Nothing admin-related remains after this.
Phase 5 = cutover.

---

## 1. Scope

19 routes across 5 capability groups + 2 crons + 1 shared AI fix-proposal path.

| Group | Routes | Node source |
|---|---|---|
| Errors (admin) | 6 | `src/admin/admin.controller.ts` (errors block) + `errors.service.ts` |
| Bug-reports (admin) | 6 | `admin.controller.ts` (bug-reports block) + `bug-reports.service.ts` |
| Inbox | 2 | `admin.controller.ts` (inbox block) |
| Releases | 4 | `updates.controller.ts` (admin block) + `releases.service.ts` + `policies.service.ts` |
| Subscriptions/grant | 1 | `admin.controller.ts` (grant) + `subscriptions.service.ts` |
| Crons | 2 | `fix-proposal.cron.ts` + `bug-fix-proposal.cron.ts` |

**Parity authority** is always the NestJS source above. Every status code,
JSON key, key order, error string, and validation message is copied verbatim.

---

## 2. Module map

Follow the repo rule: **extend existing modules; new flat module only for an
entity that lacks one.**

| Module | Action | What |
|---|---|---|
| `internal/errreport` | EXTEND | admin_usecase.go, admin_handler.go, cron.go, repo methods (list/getOne/setStatus/deleteOne/suggestFix/candidates/updateFixProposal) |
| `internal/bugreports` | EXTEND | admin_usecase.go, admin_handler.go, cron.go, repo methods + screenshot streamer |
| `internal/inbox` | **NEW flat** | composes errreport + bugreports list services via 2 consumer ports |
| `internal/updates` | EXTEND | admin_usecase.go, admin_handler.go, NET-NEW upsert/delete release + policy upsert queries |
| `internal/subscription` | EXTEND | admin grant method on existing admin_handler/usecase + `:one RETURNING` grant query |
| `internal/aiprovider` | EXTEND | `GetDefault` repo method + `GetDefaultAiProvider` sqlc query (shared by both crons + both suggest-fix routes) |
| `internal/platform/scheduler` | EXTEND | NEW `hourly.go` — `NextHourlyAt` + `RunHourly` |
| `internal/shared/router.go` | EXTEND | 19 nil-guarded `http.Handler` fields, mounted in admin group |
| `cmd/server/main.go` | EXTEND | wire repos→services→handlers; start 2 hourly crons (toggle-gated) |

### Clean-arch invariants (unchanged)
- Use cases import NO pgx/chi/net/http. Ports declared consumer-side, 1–3 methods.
- `inbox` imports errreport/bugreports **exported data types** only; its 2 list
  ports are declared in `inbox/usecase.go`, main.go injects the concretes.
- Go struct field order = JSON key order. Every response struct below lists
  fields in Node's exact key order.

---

## 3. The shared AI fix-proposal path

Both `suggest-fix` routes and both crons converge on the same Node logic:
`aiProviders.findDefault()` → if none, **400 "No default AI provider
configured"** → build hardcoded system+user prompts (user text bounded to
**8000 chars**) → `callProvider(provider, system, user)` → store `result.text`.

### 3.1 Default-provider lookup (the GAP)
Node `findDefault()` = `SELECT ... FROM ai_providers WHERE is_default = true AND
is_active = true LIMIT 1` → throws **400 "No default AI provider configured"**
when null. (Note: filters on BOTH `is_default` AND `is_active`.)
Add:
- sqlc query `GetDefaultAiProvider :one` in `queries_aiprovider.sql`
  (`WHERE is_default = true AND is_active = true LIMIT 1`).
- `(*aiprovider.PgRepo).GetDefault(ctx) (AiProvider, error)`; add `GetDefault`
  to the module's `Repo` interface.
- Reuse `aiprovider.Factory.For(p)` → `aicall.Completer` (the 4c-2 template at
  `internal/aiprovider/usecase.go:109` `Test()`), call
  `completer.Complete(ctx, system, user)` → `(text, ms, err)`.

Both errreport and bugreports declare a tiny consumer port for this — exact
shape TBD-in-plan, but ≤2 methods: `DefaultProvider(ctx)` + `Complete(...)`,
or one `Propose(ctx, system, user) (text string, err error)` facade in
aiprovider. **Plan decides;** simplest is a single facade method
`aiprovider.Service.Propose(ctx, system, user)` returning text or the verbatim
400 sentinel when no default. Reused by errreport, bugreports, both crons.

### 3.2 Error prompt (verbatim — transcribe exactly in the plan)
- **System:** senior engineer reviewing a production crash; instruct output
  sections `ROOT CAUSE / LIKELY FILE / FRAME / PROPOSED FIX / CONFIDENCE`.
- **User:** labelled block — `Platform`, `App version`, `Severity`,
  `Occurrences`, `First seen`, `Last seen`, `Error type`, `Message`,
  `Stack trace`, `Context`. (Exact strings live in
  `src/errors/fix-proposal.cron.ts` / `errors.service.ts suggestFix` — copy
  byte-for-byte; do NOT paraphrase.)
- On success: set `ai_fix_proposal = result.text`, `status = 3`. Returns entity.

### 3.3 Bug prompt (verbatim)
- **System:** senior engineer reviewing a user bug report, **NO stack trace**;
  output sections `LIKELY ROOT CAUSE / LIKELY AREA OF CODE / PROPOSED FIX OR
  INVESTIGATION / CONFIDENCE / REPRODUCTION QUESTIONS`.
- **User:** labelled block — `Source`, `Platform·OS`, `App version`,
  `Submitted`, `User email`, `Has screenshot`, `User's description`, `Context`.
- On success: set `ai_fix_proposal = result.text`,
  `ai_fix_proposed_at = now()`, and `status = 'fix_proposed'` **only if status
  was `'new'`**. Returns entity.

> **GOTCHA:** `error_reports` table has NO `ai_fix_proposed_at` column — do not
> add it. Only `bug_reports` has `ai_fix_proposed_at`.

---

## 4. ERRORS — 6 routes (all admin-guarded)

Not-found in this group → **400 BadRequestException**, message `"not found"`
(errors do NOT use 404).

ErrorReport row key order (verbatim):
`id, display_no, platform, app_version, severity, error_type, message,
stack_trace, context, user_id, device_id, fingerprint, count, status,
ai_fix_proposal, resolved_by, resolved_at, first_seen_at, last_seen_at`.

| # | Route | Status | Notes |
|---|---|---|---|
| E1 | `GET /admin/errors` | 200 | **bespoke parse, NOT parseListQuery**: `platform?`, `status?`(Number), `severity?`, `limit?`(def 50, cap 200), `offset?`(def 0). Sort `last_seen_at DESC`. Response `{items, total}`. |
| E2 | `GET /admin/errors/:id` | 200 | `if(!row) throw BadRequestException('not found')` → **400 "not found"**. |
| E3 | `PATCH /admin/errors/:id` | 200 | **inline body** `{status:number (req), resolved_by?:string}`, no class-validator. 400 `'status required'` if `status` undefined. `status∈{4,5}` → set `resolved_at=now`, `resolved_by = resolved_by||'admin'`. Returns entity. |
| E4 | `DELETE /admin/errors/:id` | 200 | response `{id, deleted}`, idempotent (`deleted:false` if absent). |
| E5 | `POST /admin/errors/:id/suggest-fix` | 201 | 400 "not found"; 400 "No default AI provider configured"; LLM error → 500. Sets `ai_fix_proposal`, `status=3`. Returns entity. (§3.2) |
| E6 | `POST /admin/errors/run-ai-cron` | 201 | awaits the ERROR cron only, returns `{ok:true}`. |

---

## 5. BUG-REPORTS — 6 routes

Not-found in this group → **404 NotFoundException**, message
`"bug report not found"` (bugs DO use 404).

BugReport row key order (verbatim):
`id, display_no, source, description, screenshot_path, screenshot_filename,
app_version, os_info, user_id, user_email, context, status, kind, title,
target_platform, vote_count, is_public, admin_notes, ai_fix_proposal,
ai_fix_proposed_at, created_at, updated_at`.

| # | Route | Status | Notes |
|---|---|---|---|
| B1 | `GET /admin/bug-reports` | 200 | **parseListQuery** (def 10, cap 100) + custom filters `status` (`['new','reviewing','fix_proposed','resolved','wont_fix']` or `'all'`), `kind` (`bug`/`feature`), `target_platform`. Search cols `br.description, br.title, br.user_email, br.source`. Sort allow-list `created_at/updated_at/status/source/kind/vote_count` → `br.*`, default `br.created_at DESC`. Response `{rows, total}`. |
| B2 | `GET /admin/bug-reports/:id` | 200 | 404 `"bug report not found"`. |
| B3 | `GET /admin/bug-reports/:id/screenshot` | 200 | Content-Type `image/png` if filename ends `.png` (case-insensitive) else `image/jpeg`; `Content-Disposition: inline; filename="<name, non-word chars → _>"`; streams from `screenshot_path`. 400 `"no screenshot for this report"` (path null); 400 `"screenshot file missing on disk"` (file absent). |
| B4 | `PATCH /admin/bug-reports/:id` | 200 | DTO `{status?, admin_notes?, title?(1-80), target_platform?, is_public?:bool}`. Validation messages **verbatim**: `"status must be one of: new, reviewing, fix_proposed, resolved, wont_fix"`, `"title is required (1-80 characters)"`, `"target_platform must be one of: playground, mobile, windows, mac, linux"`. 404 if absent. |
| B5 | `DELETE /admin/bug-reports/:id` | 200 | response `{success:true}`, 404 if absent, best-effort screenshot file unlink. |
| B6 | `POST /admin/bug-reports/:id/fix-proposal` | 201 | 404 "bug report not found"; 400 "No default AI provider configured"; LLM → 500. Sets `ai_fix_proposal`, `ai_fix_proposed_at=now`, `status='fix_proposed'` ONLY if was `'new'`. Returns entity. (§3.3) |

target_platform allow-list (B4): `['playground','mobile','windows','mac','linux']`.

---

## 6. INBOX — 2 routes

`internal/inbox` NEW flat module. Composes errreport list + bugreports list via
2 consumer ports. No DB of its own.

### I1 `GET /admin/inbox/counts` → 200
Node: `Promise.all` of
- bug `findAllPaginated(status:'new', kind:'bug', limit:1)` → `.total` → `new_bugs`
- bug `findAllPaginated(status:'new', kind:'feature', limit:1)` → `.total` → `new_features`
- errors `list(status:0, limit:1)` → `.total` → `new_errors`

Response `{new_bugs, new_features, new_errors, total}` where `total = new_bugs +
new_features + new_errors`.

### I2 `GET /admin/inbox` → 200
Query `kind?`, `status?`, `limit?`.
- `limit = Math.min(parseInt(limit||'50')||50, 100)`.
- `wantErrors = kind !== 'bug'`; `wantBugs = kind !== 'error'`;
  `openOnly = status === 'open'`.
- if wantErrors: errors `list(status: openOnly?0:undefined, limit, offset:0)`.
- if wantBugs: bug `findAllPaginated(page:1, limit, status: openOnly?'new':undefined,
  sortBy:'created_at', sortOrder:'DESC')`.
- map each to `InboxItem`, merge errorItems+bugItems, **sort by created_at DESC**,
  `slice(limit)`.

Response `{items:[InboxItem], counts:{errors, bugs, returned}}` where
`errors = errorItems.length`, `bugs = bugItems.length`, `returned = items.length`.

**InboxItem key order:** `kind, id, title, platform, app_version, status,
created_at, ai_fix_proposal` then error extras `error_type, severity,
occurrence_count` OR bug extras `user_email, has_screenshot`.

errorItem mapping:
- `kind:'error'`
- `title = [error_type, message].filter(Boolean).join(': ').slice(0,200) || '(no message)'`
- `platform = e.platform`
- `status = errorStatusLabel(e.status)` where
  `errorStatusLabel(s) = ['new','reviewing','resolved','fix_proposed','resolved','wont_fix'][s] ?? 'new'`
- `created_at = e.last_seen_at ?? e.first_seen_at`
- `ai_fix_proposal = e.ai_fix_proposal`
- extras: `error_type = e.error_type`, `severity = e.severity`,
  `occurrence_count = e.count`

bugItem mapping:
- `kind:'bug'`
- `title = b.description.slice(0,200) || '(no description)'`
- `platform = b.os_info`
- `status = b.status` (raw)
- `created_at = b.created_at`
- `ai_fix_proposal = b.ai_fix_proposal`
- extras: `user_email = b.user_email`, `has_screenshot = !!b.screenshot_path`

> `errorStatusLabel` is defined/used ONLY in the inbox block (NOT the error CRUD
> routes — those return raw int `status`). Index 2 and 4 both map to `'resolved'`
> intentionally — copy the array verbatim.

---

## 7. RELEASES — 4 routes

`internal/updates` EXTEND. Existing public read queries (`GetReleasePolicy`,
`GetEnabledReleaseByChannel`) stay; add NET-NEW admin queries.

PLATFORMS order (verbatim, drives GET response key order):
`['mac','windows','linux','android','ios']`. Channels per platform:
`{direct, store}`.

AppRelease fields/defaults: `platform`, `channel`(def `'direct'`), `version`,
`download_url`, `sha256`(def `''`), `release_notes`(def `''`), `required`(def
`false`), `enabled`(def `true`), `updated_at`.
AppReleasePolicy fields/defaults: `platform`, `preferred`(def `'direct'`),
`store_status`(def `'not_submitted'`), `notes`(def `''`), `updated_at`.

| # | Route | Status | Notes |
|---|---|---|---|
| R1 | `GET /admin/releases` | 200 | `listAll()`. Build nested map seeded over PLATFORMS in order: each `byPlatform[p] = {policy:null, channels:{direct:null, store:null}}`; fill `channels[row.channel]=row` and `policy` from rows/policies (skip rows whose platform not in PLATFORMS). Response `{mac:{policy,channels:{direct,store}}, windows:{...}, linux:{...}, android:{...}, ios:{...}}`. |
| R2 | `POST /admin/releases` | 201 | body `{platform, channel?(def 'direct'), version, download_url, sha256?, release_notes?, required?, enabled?}`. `upsertChannel`. Validation (BadRequestException, verbatim): platform∈PLATFORMS else `"platform must be one of: mac, windows, linux, android, ios"`; channel∈['direct','store'] else `"channel must be one of: direct, store"`; `version && download_url` else `"version and download_url are required"`; sha256 (if non-empty) must match `^[0-9a-f]{64}$` else `"sha256 must be a 64-char hex string (or empty)"`. Returns upserted AppRelease. |
| R3 | `DELETE /admin/releases/:platform/:channel` | 200 | `deleteChannel`. Response `{ok:true}`. 404 NotFoundException `"No release row for {platform}/{channel}"` if affected=0. |
| R4 | `POST /admin/release-policies` | 201 | body `{platform, preferred?, store_status?, notes?}`. `upsert`. Validation: platform enum (same msg); preferred∈['direct','store'] else `"preferred must be one of: direct, store"`; store_status∈['not_submitted','in_review','approved','rejected','n/a'] else `"store_status must be one of: not_submitted, in_review, approved, rejected, n/a"`. Returns AppReleasePolicy. |

NET-NEW sqlc queries (names TBD-in-plan): upsert release (by platform+channel,
`ON CONFLICT ... DO UPDATE`), delete release (`:execrows` → affected count for
404), upsert policy (by platform), `find()` all releases, `find()` all policies.

---

## 8. SUBSCRIPTIONS/GRANT — 1 route

### G1 `POST /admin/subscriptions/grant` → 201
DTO `GrantSubscriptionDto`: `@IsUUID user_id`, `@IsUUID plan_id`, `@IsOptional
@IsDateString expires_at?`.

`subscriptionsService.grant(user_id, plan_id, expiresAt)`:
1. cancel prior ACTIVE sub for user → `cancelled` (existing
   `CancelActiveSubsByUser`).
2. create new ACTIVE sub `{user_id, plan_id, status:'active',
   store_type:'admin_granted', started_at:now, expires_at: expiresAt||null}`.
3. **return the inserted Subscription row.**

> **PARITY GAP:** the existing `InsertGrantedSubscription :exec` returns nothing,
> but Node returns the full row. Change the query to **`:one`** with
> `RETURNING id, user_id, plan_id, status, store_type, store_transaction_id,
> started_at, expires_at, created_at, updated_at`. (Also pass the
> `store_type='admin_granted'` literal — the existing query already takes
> store_type as `$3`.)

Response Subscription key order (verbatim):
`id, user_id, plan_id, status, store_type, store_transaction_id, started_at,
expires_at, created_at, updated_at`.

---

## 9. HOURLY CRONS — 2

Both fire on the wall-clock hour (`:00`). Shared env toggle:
`DISABLE_FIX_PROPOSAL_CRON == '1' || 'true'` skips BOTH. Each cron: never throws
(catches per-candidate), sleeps **1000ms between calls**, logs
`"${success} succeeded, ${failed} failed"` at the end.

### 9.1 Scheduler (NEW `internal/platform/scheduler/hourly.go`)
Mirror existing `daily.go` (`NextDailyAt` + `RunDaily`, ctx-cancellable select
loop). Add:
- `NextHourlyAt(now time.Time) time.Time` — next top-of-hour after `now`.
- `RunHourly(ctx, now func() time.Time, job func(context.Context))` — sleep
  until next `:00`, run `job`, repeat until ctx cancelled.

Wire in `cmd/server/main.go` mirroring the expiry-cron template (main.go:607-615):
build both jobs, gate on the toggle, `go scheduler.RunHourly(...)` each.

### 9.2 Errors cron (`FixProposalCron`, `EVERY_HOUR`)
Candidates: `status=0 AND ai_fix_proposal IS NULL AND count >= 2` ORDER BY
`count DESC, last_seen_at DESC` LIMIT **10**. Per candidate:
`errreport.suggestFix(id)` (the §3.2 path). Tally success/fail, sleep 1000ms
between.

### 9.3 Bugs cron (`BugFixProposalCron`, `EVERY_HOUR`)
Candidates: `status='new' AND kind='bug' AND ai_fix_proposal IS NULL` ORDER BY
`created_at DESC` LIMIT **5**. Per candidate: `bugreports.suggestFix(id)` (the
§3.3 path). Tally, sleep 1000ms between.

Both expose their candidate query as a repo method
(`ErrorCandidates`/`BugCandidates`) and reuse the same suggestFix use case the
HTTP routes call (E5/B6). The `run-ai-cron` route (E6) invokes the errors cron's
job directly.

---

## 10. Parity landmines (the 10 + new)

1. **Not-found split:** errors → **400 "not found"**; bugs → **404 "bug report not found"**.
2. **List wrappers differ:** errors `{items,total}` (bespoke parse); bugs `{rows,total}` (parseListQuery).
3. **DELETE shapes differ:** errors `{id, deleted}` (idempotent); bugs `{success:true}` (404 if absent).
4. **PATCH shapes differ:** errors inline body, 400 `"status required"`; bugs DTO with verbatim validation messages.
5. **POST = 201; GET/PATCH/DELETE = 200.** (suggest-fix, fix-proposal, run-ai-cron, releases POST, release-policies POST, grant = 201.)
6. **Screenshot:** content-type by `.png` suffix (case-insensitive) else jpeg; `Content-Disposition: inline; filename="<non-word→_>"`.
7. **suggest-fix side-effects differ:** error → `status=3`; bug → `ai_fix_proposed_at` + `status='fix_proposed'` ONLY if was `'new'`.
8. **`error_reports` has NO `ai_fix_proposed_at`** column — do not add.
9. **Inbox shapes verbatim:** InboxItem key order, `errorStatusLabel` array (idx 2 & 4 both `'resolved'`), `created_at = last_seen_at ?? first_seen_at`, `(no message)`/`(no description)` fallbacks.
10. **AI prompts hardcoded verbatim** — transcribe byte-for-byte, user text bounded 8000 chars.
11. **Grant query must become `:one RETURNING`** — existing `:exec` cannot return the row Node returns.
12. **Releases GET key order = PLATFORMS order** (`mac, windows, linux, android, ios`), each `{policy, channels:{direct, store}}`.
13. **Crons: toggle gates BOTH; never throw; 1000ms between calls; log `"X succeeded, Y failed"`; LIMIT 10 (errors) / 5 (bugs).**

---

## 11. Go reuse verdict (confirmed)

- **AI call:** `internal/aicall/completer.go:15` `Completer{Complete(ctx,system,user)(text,ms,err); Name()}`; `internal/aiprovider/completer_factory.go:30 Factory.For(p)`; test template `internal/aiprovider/usecase.go:109`. **GAP:** add `GetDefaultAiProvider :one` + `GetDefault` repo method + Repo-interface entry.
- **Scheduler:** `internal/platform/scheduler/daily.go` has `NextDailyAt`+`RunDaily`. **NEW** `hourly.go`. Expiry-cron wiring template `cmd/server/main.go:607-615`.
- **sqlc models READY:** `ErrorReport` (ai_fix_proposal, status, count, fingerprint; **NO ai_fix_proposed_at**), `BugReport` (ai_fix_proposal, ai_fix_proposed_at, status, kind), `AppRelease`, `AppReleasePolicy`, subscription enums + `InsertGrantedSubscriptionParams`.
- **updates module** `internal/updates/`: has `GetReleasePolicy`, `GetEnabledReleaseByChannel` (public read). NET-NEW upsert/delete release + upsert policy + find-all queries.
- **subscription grant:** `CancelActiveSubsByUser` exists; `InsertGrantedSubscription` exists as `:exec` → change to `:one RETURNING`.
- **errreport/bugreports** = ingest only today; add admin_usecase/admin_handler/cron + repo methods per module.
- **inbox** = NEW flat module, 2 consumer ports.

---

## 12. Router + main.go wiring

19 new nil-guarded `http.Handler` fields on `internal/shared/router.go`,
mounted under the admin group (after RequireAuth → RequireAdmin), static paths
over wildcards (chi). A `router_admin4c4_test.go` mirrors the 4c-3 guard test:
mount all 19 with 200 handlers, assert no-auth → 401 and non-admin → 403.

Static-vs-wildcard care:
- `/admin/errors/run-ai-cron` (static POST) vs `/admin/errors/{id}` — chi
  routes static over wildcard, both reachable.
- `/admin/errors/{id}/suggest-fix`, `/admin/bug-reports/{id}/screenshot`,
  `/admin/bug-reports/{id}/fix-proposal` are sub-paths — distinct.
- `/admin/inbox/counts` (static) vs `/admin/inbox` (static) — distinct.
- `/admin/releases/{platform}/{channel}` (DELETE) — 2-segment wildcard.

main.go: build each repo → service → handler; reuse the one shared
`aiprovider` default-provider path + `aicall` factory across both suggest-fix
services and both crons; start the 2 hourly crons gated on
`DISABLE_FIX_PROPOSAL_CRON`.

---

## 13. Testing strategy

Per the byte-parity discipline + repo convention (`*_test.go` beside code,
fakes satisfy ports, no DB):
- **Use-case tests** with fakes: errors not-found→400, bugs not-found→404,
  PATCH validation messages verbatim, DELETE idempotency vs 404, suggest-fix
  side-effects (status=3 vs ai_fix_proposed_at+conditional status), inbox
  mapping (errorStatusLabel array, title slice/fallback, created_at coalesce,
  merge+sort+slice), grant cancel-then-insert + returned row, releases
  validation messages + GET nested-map key order, cron candidate selection +
  toggle skip + "X succeeded, Y failed" log + 1000ms spacing (inject clock/sleep).
- **Handler tests:** status codes (201 vs 200), error envelopes
  `{error,code,request_id}`, screenshot content-type/disposition, JSON key
  order (golden-string compare where order matters).
- **Scheduler test:** `NextHourlyAt` returns next `:00`; `RunHourly` fires job
  then respects ctx cancel.
- **Router guard test:** `router_admin4c4_test.go` (19 routes, 401/403).
- Full gate green: `go build ./...`, `go vet ./...`, `gofmt -l .` (empty),
  `go test ./... -race`, `sqlc generate` + no drift.

---

## 14. Out of scope / follow-ups

- No prod deploy, no Caddy flip (Phase 5).
- AI prompt verbatim strings: transcribed in the PLAN from
  `src/errors/fix-proposal.cron.ts`, `src/errors/errors.service.ts`,
  `src/bug-reports/*` — not duplicated here to avoid drift; the plan is the
  single source for the exact bytes.
- Pre-existing hardening follow-ups (#29-#36) unchanged; do not fix here
  (would break byte-parity until Node changes too).
