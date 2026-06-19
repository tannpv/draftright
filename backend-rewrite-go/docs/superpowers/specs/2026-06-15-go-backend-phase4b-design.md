# Go Backend Port — Phase 4b Design (extraction · email · bug_reports · feedback)

> **Status:** design / scoping. Approved forks recorded below. Next step after
> user review: `writing-plans` → subagent-driven execution (same as Phase 4a).

**Goal:** Port four LLM/email/ingest modules from the NestJS backend
(`/opt/openAi/DraftRight/backend`) to the Go drop-in
(`/opt/openAi/DraftRight/backend-rewrite-go`) as byte-identical endpoints —
identical routes, JSON keys in Node's insertion order, status codes, error
envelopes, validation messages.

**Parity authority:** Node source. Where this doc and Node disagree, **Node
wins**. Plan snippets are guidance, not authority.

---

## Scope (locked)

### In scope — 7 routes + 1 shared collaborator

| Module | Route | Auth | Status |
|---|---|---|---|
| shared | *(no route)* `Completer` blocking-provider port | — | — |
| extraction | `POST /extract` | JWT required | 200 (`@HttpCode(200)`) |
| email | `POST /webhooks/resend` | Svix HMAC (public) | 200 |
| bug_reports | `POST /bug-reports` | public, JWT optional | 201 |
| feedback | `POST /feedback` | public, JWT optional | 201 |
| feedback | `GET /feedback` | public, JWT optional | 200 |
| feedback | `POST /feedback/:id/vote` | JWT required (manual 401) | 200 (`@HttpCode(200)`) |

Plus: finish `internal/email/` (4 missing templates + `formatAmount` +
HTML-escape parity fix).

### Deferred to Phase 4c (admin)

- `GET /admin/bug-reports`, `GET /admin/bug-reports/:id`,
  `GET /admin/bug-reports/:id/screenshot`, `PATCH /admin/bug-reports/:id`,
  `DELETE /admin/bug-reports/:id`
- `POST /admin/bug-reports/:id/fix-proposal` (admin-guarded LLM fix proposal)
- hourly `bug-fix-proposal.cron` (`@Cron(EVERY_HOUR)`)
- admin email-logs / email-templates CRUD / `settings/test-email`

Rationale: all admin-guarded → belong with the rest of admin in 4c. The
blocking-provider `Completer` they need is still built in 4b (extraction
requires it), so 4c only adds the bug-flavored prompt + cron on top.

---

## Cross-cutting parity invariants (all modules)

Already established in Phases 0–4a — reaffirmed here:

1. **Error envelope** `{error, code, request_id}` in that key order via
   `shared.WriteError(w, r, code, message)`; status from `StatusForCode`.
2. **ValidationPipe** (`whitelist + forbidNonWhitelisted + transform`): unknown
   body key → 400; failed constraints → flat humanized message array joined
   `". "`, code `invalid-input`. Replicate per the sibling pattern in
   `auth/validate.go` + `payment/handler_checkout.go` (per-field checks in
   declaration order, `lenUTF16` for `@MaxLength`).
3. **Go marshals struct fields in declaration order; JS preserves insertion
   order** → every response is an ordered STRUCT with explicit json tags in
   Node's key order. Never `map[string]any` for a fixed-shape body.
4. **POST default 201 in Nest** — honor each route's `@HttpCode` override
   exactly (table above).
5. **Internal 500s** use a FIXED opaque noun-phrase string, never
   `err.Error()` (`"extraction failed"`, etc.).
6. **JWT clock tolerance zero.** Optional-JWT routes: case-sensitive `Bearer `
   prefix, raw `slice(7)`, any verify error → anonymous (mirror
   `errreport.optionalUserID`).
7. **Per-route throttling** (bug_reports + feedback POST = 5/min + 30/hr per
   IP) — note in plan; implement only if the Go port already has a throttle
   middleware, else document as a deferred infra item (Caddy/throttle layer),
   do NOT silently drop.

---

## Shared collaborator — `Completer` (blocking provider call)

**Problem:** Go rewrite providers are **streaming-only, per-tone**
(`rewrite/domain/ports.go:40` `Stream(...) (<-chan string, <-chan error)`).
Extraction (and 4c bug-fix) need a **blocking** call with an arbitrary
system+user prompt returning full text.

**Approved fork:** Go rewrite already resolves its provider from **env**
(`AI_PROVIDERS` list + per-provider keys, provider-id pinned via env to satisfy
`usage_logs` FK — documented at `cmd/server/main.go:543`), NOT from Node's
DB-driven `ai_providers WHERE is_default AND is_active`. Phase 4b **follows the
existing env-driven convention** — it does NOT reintroduce DB resolution. The
byte-parity target is the **response shape + output-cleaning pipeline + token
heuristic**, not which physical key answers (rewrite already env-driven; this
keeps consistency + Rule #1 extend-don't-fork).

**Design:** add a non-streaming capability to the existing provider, consumed
through a small consumer-side port:

```go
// internal/extraction (consumer owns the interface, kept small)
type Completer interface {
    // Complete sends a blocking system+user prompt to the configured
    // default provider, returns the full assistant text + round-trip ms.
    Complete(ctx context.Context, system, user string) (text string, ms int64, err error)
}
```

Implementation reuses the env-selected provider's HTTP client. Options for the
plan to pick (mechanical, no logic change):
- (a) extend each adapter (`openai`/`anthropic`/`ollama`) with a `Complete`
  method alongside `Stream`, expose via a `BlockingProvider` interface; OR
- (b) a thin `aiprovider.Completer` adapter that does a non-stream POST using
  the same creds/model the chain already resolved.

Recommendation: **(a)** — one method per existing adapter, same creds path,
satisfied by the same concrete already built in `composeDeps`. `chain` wrapper
forwards to its primary. Memory fake returns a scripted string for tests.

No-default / no-usable-provider → mirror Node `findDefault` throw:
`BadRequestException('No default AI provider configured')` → 400 envelope code
`invalid-input` (verify exact Node code/message at port time).

---

## Module 1 — extraction

**Files:** `internal/extraction/{domain,usecase,handler,validate}.go` + tests.
No DB writes. No new sqlc.

**Route:** `POST /extract`, `RequireAuth` (session JWT, not ext-token), 200.

**Request DTO** (`extract.dto.ts`): `text` (`@IsString @MaxLength(8000)`,
required); `kinds?` (`@IsOptional @IsArray @ArrayMaxSize(20)
@IsEnum(EntityKind,{each:true})`). `EntityKind` =
`phone,email,url,otp,creditCard,address,personName,dateTime,bankAccount`.

**Response** (`ExtractResponseDto`, key order):
`{ entities: [...], provider, tokensUsed }`. Each entity, key order:
`kind, value, display, start, end, confidence, meta?` (`meta` omitted when nil).

**Byte-exact output pipeline** (port `extraction.service.ts` verbatim):
- `stripCodeFences` (`:80`).
- JSON.parse guard → `{entities:[], provider, tokensUsed:0}` on failure
  (+ sha1 log). Array guard.
- `validateEntity` (`:91`): drop `REGEX_HANDLED` kinds
  (phone/email/url/otp/creditCard) from LLM output; `value` MUST be a literal
  substring of `text` via index lookup (UTF-16 semantics) else drop
  ("hallucination"); confidence clamp 0..1; `meta` coerced string→string map;
  recompute `start/end/display`.
- `dedupe` keyed `kind + ":" + lower(value)`, keep max confidence; sort by
  `start`.
- `tokensUsed = ceil((len(text)+len(rawOutput)) / 4)` — char heuristic, UTF-16
  length, replicate exactly.
- System prompt hardcoded (`:61`, incl. the Vietnamese example + `kinds`
  filter excluding REGEX_HANDLED). Copy byte-for-byte.

**Collaborators:** `Completer` (above), `auth.Verifier` (RequireAuth — already
wired). 500 → `"extraction failed"`.

---

## Module 2 — email (finish existing `internal/email/`)

**Already done:** deliver/Resend/log/render, `verification` +
`password-reset` templates, `SendVerification`/`SendPasswordReset`/`SendRaw`,
creds fallback, `defaultFrom`, fire-and-forget.

**4b adds:**

1. **4 missing templates** (`email-templates.ts:24`): `subscription-activated`,
   `subscription-expired`, `renewal-reminder`, `payment-failed` + their typed
   send methods + `sendTestEmail`. Plus `formatAmount` helper.
2. **HTML-escape parity FIX (real divergence):** Node `substitute` HTML-escapes
   values in html context (`email.service.ts:190`); Go `templates.go:53` does
   NOT. Fix Go to escape in html body, raw in subject — match Node exactly.
3. **`POST /webhooks/resend` controller** (`email-webhook.controller.ts`):
   - Public, `@HttpCode(200)`, response `{ received: true }` (single key).
   - **Svix HMAC** (`:78`): needs `svix-id`/`svix-timestamp`/`svix-signature`;
     key = base64-decode(secret minus `whsec_`); HMAC-SHA256 of
     `${id}.${ts}.${rawBody}`; constant-time compare against space-separated
     `v1,<sig>` parts. **Requires RAW request body** — Go route must read raw
     bytes before any JSON decode (Nest uses `rawBody:true`).
   - Bad sig → `BadRequestException('Invalid webhook signature')`; bad JSON →
     `('Invalid payload')` → 400 envelope.
   - Events (`:53`): `email.delivered` → mark delivered;
     `email.bounced` → mark bounced + suppress **only if** bounce type/subType
     contains "permanent"/"hard"; `email.complained` → mark complained +
     suppress; else ignore. Suppression lowercased + idempotent (orIgnore).
   - DB: `email_logs` (mark by provider id), `email_suppressions` (insert
     ignore). Reuse existing `email/repo_pg.go`; add `MarkByProviderID` +
     `Suppress` queries if absent.

**Collaborators:** existing email service/repo; settings (`app_settings`) for
resend key already wired.

---

## Module 3 — bug_reports (`POST /bug-reports` only; full file-upload parity)

**Files:** `internal/bugreports/{domain,usecase,handler,validate,repo_pg}.go` +
tests. New sqlc `queries_bugreports.sql`. Use `internal/errreport/` as the
clean-arch template (closest sibling: public JWT-optional ingest + honeypot).

**Route:** `POST /bug-reports`, public, JWT optional, **201**,
**multipart/form-data**.

**Request DTO** (`CreateBugReportDto`, multipart): `description`
(`@IsString @MinLength(1)`); `source` (`@IsString @MaxLength(50)`);
`app_version?`/`user_email?` (`@MaxLength(50)`/`(255)`); `os_info?`
(`@MaxLength(255)`, service re-slices to 100); `context?` (JSON string, service
parses, fallback `{raw}`); `website?` (`@MaxLength(255)`, **honeypot**); file
field `screenshot` via `FileInterceptor` — max **5MB**, mime allow-list
png/jpeg/jpg/webp/heic/heif/gif; rejected mime →
`BadRequestException('only PNG or JPEG screenshots are accepted')`.

**Screenshot persistence** (full parity, `bug-reports.service.ts:81`):
- write to `<BUG_REPORTS_DIR>/YYYY-MM-DD/<uuid><ext>`; `BUG_REPORTS_DIR` env,
  default `/var/lib/draftright/bug-reports`.
- `extensionFor` only emits `.png`/`.jpg` (`:414`) — a webp/heic that passes
  the controller mime filter **throws here** (replicate the asymmetry exactly).
- date bucket = report date; uuid filename.

**Response** (`:96`, key order): `{ id, ref, message }`;
`ref = "BUG-" + display_no` (`display-number.ts`). **Honeypot** (`:76`) returns
a DIFFERENT shape `{ id: null, status: 'received' }` with 201, no row written.

**Service write** (`:66`): trim/validate description+source (empty →
BadRequest), parse context JSON, `resolveUserId` nulls orphan FK, slice fields
to column widths, `status:'new'`, `kind:'bug'`. Multipart body-size: 5MB
(`http.MaxBytesReader` analog; oversize → match Node behavior — likely the
PayloadTooLarge → 500 path seen in errreport unless multipart differs; verify
at port time).

**DB:** `bug_reports` table (entity column order at `bug-report.entity.ts:21`).
Reads `users` for FK resolve. Filesystem write for screenshot.

**Collaborators:** optional-JWT decoder (shared `jwt-user` equivalent =
`errreport.optionalUserID` pattern), `display-number` formatter (shared — also
used by feedback; build once in `shared/` or a small `displaynum` pkg).

---

## Module 4 — feedback (3 routes; shares bug_reports table)

**Files:** `internal/feedback/{domain,usecase,handler,validate,repo_pg}.go` +
tests (or co-locate with bugreports since same table). New sqlc
`queries_feedback.sql`. No LLM, no email, no file.

**`POST /feedback`** (JSON, public JWT-opt, 201):
DTO (`CreateFeedbackDto`): `kind` (`@IsIn(['bug','feature'])`); `title?`
(`@MaxLength(80)`, re-validated 1–80 when kind=feature); `target_platform?`
(`@IsIn(['playground','mobile','windows','mac','linux'])`); `description`
(`@IsString @MinLength(1) @MaxLength(2000)`); `source` (`@MaxLength(50)`);
`app_version?`/`os_info?`/`user_email?`; `website?` honeypot.
Response (`:50`): `{ id, ref, message }`, `ref = "BUG-n"/"FR-n"`, noun
"Bug report"/"Feature request". Honeypot (`:43`) → `{ id: null, message:
'Received. Thanks!' }`. Service (`:126`): feature requires title 1–80 +
target_platform in set (else BadRequest, allowed-list in message — match
verbatim); sets `vote_count:0, is_public:true, screenshot_*:null, context:null,
status:'new'`.

**`GET /feedback`** (public JWT-opt, 200): `listPublicFeatures` (`:205`) →
`{ rows, total }`. Each row = full `BugReport` entity columns **+ trailing
`viewerHasVoted`** (Object.assign → LAST key; Go struct must place
`ViewerHasVoted` last). Filter `kind='feature' AND is_public`, optional `status`
(must be in ALLOWED_STATUSES) + `target_platform`; order
`vote_count DESC, created_at DESC`; page≥1, limit clamp 1–100 default 20.

**`POST /feedback/:id/vote`** (JWT REQUIRED but **manual**, 200): decode-optional
→ if nil throw `UnauthorizedException('sign in to vote')` (401, NON-standard
message — match exactly, do NOT use the guard's generic "Unauthorized").
`toggleVote` (`:182`): toggle `feature_votes` row (UNIQUE `(feature_id,user_id)`),
recompute `vote_count = COUNT(*)`, persist on BugReport. Non-feature id →
`NotFoundException('feature request not found')`. Response: `{ vote_count,
hasVoted }`.

**DB:** `bug_reports` + `feature_votes`. Shared `display-number` + optional-JWT
decoder.

---

## Build order (hardest deps first)

1. **`Completer`** shared blocking-provider port + adapter `Complete` methods +
   memory fake. (Unblocks extraction.)
2. **extraction** — `POST /extract` + output pipeline.
3. **email** — 4 templates + escape fix + Svix webhook (raw-body route).
4. **shared `displaynum`** + optional-JWT helper (extract from errreport if
   reusable).
5. **bug_reports** — multipart + screenshot FS + honeypot.
6. **feedback** — 3 routes (reuse #4 helpers, bug_reports repo).
7. **Wiring** — router public/auth mounts + `composeDeps` + fixtures.

## Fixtures (shadow gate, deterministic only)

- `extract.json` — POST, requires JWT + a live provider → likely EXCLUDE from
  static fixtures (non-deterministic LLM); cover via unit tests instead.
- `feedback_get.json` — GET `/feedback` (ignore volatile timestamps/ids).
- `feedback_bad_kind.json` — POST 400 validation.
- `bugreport_honeypot.json` — POST honeypot 201 `{id:null,status:'received'}`.
- `feedback_honeypot.json` — POST honeypot 201 `{id:null,message:...}`.
- DB-writing success paths excluded (non-deterministic ref/ids), per 4a.

## Out of scope / ships nothing to prod

No Caddy flip, no cutover. Live shadow gate operator-pending (Phase 5). Per-route
throttle parity flagged but gated on whether Go has a throttle layer yet.

## Open parity items to confirm at port time

- Exact Node `code` for `findDefault` throw (BadRequest → which envelope code).
- Multipart oversize behavior vs errreport's PayloadTooLarge→500 (multipart
  parser may differ).
- Whether a Go throttle middleware exists; if not, document the deferral.
- `bug_reports` / `feature_votes` exact column serialization order from the
  prod schema dump (`internal/platform/db/schema.sql`).
