# Rewrite Node-Parity Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Make the Go backend serve Node's `/rewrite*` contract byte-identically so
the Phase 5 whole-upstream Caddy flip doesn't 404 the core product.

**Architecture:** Node serves `GET /rewrite/tones`, `POST /rewrite` (non-streaming
JSON, 201), `POST /rewrite/trial` (201). Go currently serves ONLY `POST /v1/rewrite`
(SSE/stub envelope `{text,service}`). This plan adds the three Node routes as a flat
parity module mirroring `internal/extraction` (Completer-based, non-streaming), and
leaves the existing streaming `/v1/rewrite` mounted untouched (macOS Go-toggle dev
path; excluded from the parity inventory because Node 404s it).

**Tech Stack:** Go 1.26, chi/v5, existing `aicall.Completer`, `rewrite/domain.UserRepo`,
`redis/go-redis/v9`, `config.IsProduction()`.

---

## The Node contract (parity authority — `/opt/openAi/DraftRight/backend`)

### `GET /rewrite/tones` — public, no guard, status **200**
`rewrite.controller.ts:16` → `{ tones: TONES }`. TONES (`tones.ts`), key order `id,label,icon,kind`:
```json
{ "tones": [
  { "id":"simple",        "label":"Simple",        "icon":"✎",  "kind":"rewrite" },
  { "id":"natural",       "label":"Natural",       "icon":"💬", "kind":"rewrite" },
  { "id":"polished",      "label":"Polished",      "icon":"✨", "kind":"rewrite" },
  { "id":"concise",       "label":"Concise",       "icon":"⊖",  "kind":"rewrite" },
  { "id":"technical",     "label":"Technical",     "icon":"🔧", "kind":"rewrite" },
  { "id":"claude",        "label":"Claude",        "icon":"✦",  "kind":"rewrite" },
  { "id":"grammar_check", "label":"Grammar Check", "icon":"✓",  "kind":"grammar" },
  { "id":"translate",     "label":"Translate",     "icon":"🌐", "kind":"translate" }
] }
```

### `POST /rewrite` — `RewriteAuthGuard` (JWT **or** `dr_ext_` token w/ `rewrite` scope), status **201**
Global `ValidationPipe({whitelist,forbidNonWhitelisted})` over `RewriteDto`:
`text @IsString` (NO length cap, NO trim), `tone @IsIn(TONE_IDS)`,
`target_language @IsOptional @IsString`, `source_language @IsOptional @IsString`.
`TONE_IDS = [simple,natural,polished,concise,technical,claude,grammar_check,translate]`.
Validation failure → **400** `invalid-input`, message = class-validator strings via
`humanizeValidation` joined `". "` (property order = DTO decl order text,tone,target_language,source_language):
- unknown key → `property <key> should not exist`
- `tone` not in set → `tone must be one of the following values: simple, natural, polished, concise, technical, claude, grammar_check, translate`
- `text` not string → `text must be a string`

Service (`rewrite.service.ts:rewrite`): resolve entitlement (`dailyLimit`), count today's usage.
- quota: `dailyLimit !== -1 && usageToday >= dailyLimit` → **429** `{ "error":"Daily limit reached", "usage_today":N, "daily_limit":N }`
- call provider; provider error → **502** `{ "error":"Rewrite service is temporarily unavailable. Please try again shortly.", "code":"provider-failed" }`
- success (rewrite tones) → **201** `{ "rewritten_text":"…", "usage_today":usedToday+1, "daily_limit":N }`
- `grammar_check` tone → `{ "grammar":{…}, "usage_today":usedToday+1, "daily_limit":N }` (parse LLM JSON; on parse fail `{score:0,issues:[],error:'Failed to parse grammar analysis'}`)
- auth failure (RewriteAuthGuard): missing/!Bearer → **401** `Missing bearer token`; `dr_ext_` invalid → `Invalid extension token`; missing scope → `Token missing rewrite scope`. (Go `shared.ExtOrJWT` already emits these.)

### `POST /rewrite/trial` — public, no guard, status **201**
IP rate-limit: key `trial:{clientIp}:{YYYY-MM-DD}`, INCR with 86400s expiry, limit =
`IsProduction() ? 3 : 999`. `count > limit` → **429** `{ "error":"Trial limit reached. Sign up for unlimited rewrites!" }`.
clientIp = first `X-Forwarded-For` hop else remote addr. Truncate text to 500 chars,
call provider. `grammar_check` → `{grammar:{…}}`; else → `{ "rewritten_text":"…" }`. Same
ValidationPipe 400 rules as `/rewrite`.

### TONE_PROMPTS (system prompts — copy verbatim from `rewrite.service.ts:15-25`)
`simple,natural,polished,concise,technical,claude` rewrite prompts + `GRAMMAR_CHECK_PROMPT`
+ `translate` template (`resolvePrompt`). Reuse Node's strings byte-for-byte (the LLM text
is non-deterministic and ignored by the gate, but the prompts must be faithful for prod).

---

## File Structure

New flat package `internal/rewrite/parity/` (mirrors `internal/extraction`):
- `tones.go` — `ToneMeta` + `Tones` catalog (single source for handler + DTO validation).
- `prompts.go` — `TONE_PROMPTS`, `GRAMMAR_CHECK_PROMPT`, `resolvePrompt(tone,target,source)`.
- `service.go` — `Service` (Completer + UserRepo) → `Rewrite(...)` / `TrialRewrite(...)`; quota, usage, grammar parse.
- `triallimit.go` — `TrialLimiter` interface + `RedisTrialLimiter` (INCR+EXPIRE) + memory fake.
- `handler.go` — `GET /rewrite/tones`, `POST /rewrite`, `POST /rewrite/trial`; validation, status, error mapping.
- `validate.go` — DTO whitelist+type validation → class-validator messages (mirror `extraction/validate.go` + `user` validator idiom).
- `*_test.go` beside each.

Modify:
- `internal/shared/router.go` — mount the 3 routes (`/rewrite/tones` public; `/rewrite` via `ExtOrJWT`; `/rewrite/trial` public). Add handler fields to `coreHandlers`.
- `cmd/server/main.go` — build parity service from `completer` + `userRepo`; wire trial limiter (redis when `REDIS_URL`, else memory); assign route fields.
- `deploy/shadow/routes.txt` — add `GET /rewrite/tones`, `POST /rewrite`, `POST /rewrite/trial`; annotate `POST /v1/rewrite` as Go-only (exclude from coverage) OR drop it from the inventory with a comment.

---

## Task R1: Tones catalog + `GET /rewrite/tones`

**Files:** Create `internal/rewrite/parity/tones.go`, `internal/rewrite/parity/tones_test.go`.

- [ ] **Step 1 (RED):** Test asserts `json.Marshal(map[string]any{"tones":Tones})` equals the
  exact byte string from the contract above (all 8 entries, key order id,label,icon,kind, UTF-8 icons).
- [ ] **Step 2:** Run `go test ./internal/rewrite/parity/ -run Tones -v` → FAIL (undefined).
- [ ] **Step 3:** Implement `type ToneMeta struct{ ID,Label,Icon,Kind string }` with json tags
  `id,label,icon,kind`; `var Tones = []ToneMeta{…8 entries…}`; `var ToneIDs = []string{…}` derived.
- [ ] **Step 4:** Test PASS.
- [ ] **Step 5: Commit** `git add internal/rewrite/parity/tones.go internal/rewrite/parity/tones_test.go && git commit -m "feat(rewrite): tones catalog (Node parity)"`

## Task R2: Prompt registry

**Files:** Create `internal/rewrite/parity/prompts.go`, `…/prompts_test.go`.

- [ ] **Step 1 (RED):** Test `resolvePrompt("polished","","")` == the exact polished prompt;
  `resolvePrompt("grammar_check","","")` == GRAMMAR_CHECK_PROMPT; `resolvePrompt("translate","Vietnamese","English")`
  == `"The source text is written in English. Translate the following text into Vietnamese. If the text is already in Vietnamese, translate it into English instead. Preserve the original meaning and tone. Return only the translated text, no explanations."`;
  `resolvePrompt("nope","","")` == "" (caller 400s before this in practice).
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Port `TONE_PROMPTS` map + `GRAMMAR_CHECK_PROMPT` + `resolvePrompt` from `rewrite.service.ts:15-40` byte-for-byte.
- [ ] **Step 4:** PASS.
- [ ] **Step 5: Commit** `…&& git commit -m "feat(rewrite): tone prompt registry (Node parity)"`

## Task R3: Parity rewrite service + `POST /rewrite`

**Files:** Create `internal/rewrite/parity/service.go`, `validate.go`, `handler.go` + tests.
Reuse `aicall.Completer` (own copy of the 2-method port like `extraction.Completer`) and
`rewrite/domain.UserRepo` (`Find`→`User{Plan.DailyLimit,UsedToday}`, `LogUsage`).

- [ ] **Step 1 (RED) validate:** table test over invalid bodies asserting exact message + order:
  unknown key → `property X should not exist`; `{"tone":5,...}` → `tone must be one of the following values: …`;
  `{"text":5,...}` → `text must be a string`; valid `{"text":"hi","tone":"polished"}` → no error.
- [ ] **Step 2 (RED) service:** with a fake Completer returning `("Hello.",12,nil)` and a fake
  UserRepo (`Plan.DailyLimit=500, UsedToday=0`): `Rewrite` returns `rewritten_text="Hello.", usage_today=1, daily_limit=500`.
  Quota case (`UsedToday=500`) → `ErrQuotaExceeded`-equivalent carrying `{usage_today:500,daily_limit:500}`.
  Provider-error case → provider-failed sentinel. grammar_check tone → `{grammar:…}` envelope.
- [ ] **Step 3 (RED) handler:** httptest — no Claims in ctx is impossible (mounted under ExtOrJWT);
  inject claims, assert 201 + body keys + `Content-Type`. Bad tone → 400 message. Quota → 429 body.
  Provider fail → 502 `{error:PROVIDER_UNAVAILABLE_MESSAGE,code:provider-failed}`.
- [ ] **Step 4:** Implement. Envelope structs with pinned json key order
  (`rewritten_text,usage_today,daily_limit`; grammar variant `grammar,usage_today,daily_limit`).
  Status `http.StatusCreated`. Resolve user via `UserRepo.Find`; `daily_limit = Plan.DailyLimit`;
  quota mirrors Node `dailyLimit !== -1 && usageToday >= dailyLimit`. On success call `UserRepo.LogUsage`
  then return `usedToday+1`. (Skip Node's Redis result-cache + background batch pre-gen — value-level
  only, ignored by the gate; note the deviation in a comment.)
- [ ] **Step 5:** All parity tests PASS; `go test ./internal/rewrite/... -race`.
- [ ] **Step 6: Commit** `…&& git commit -m "feat(rewrite): POST /rewrite non-streaming Node-parity envelope"`

## Task R4: Trial rewrite + IP limiter + `POST /rewrite/trial`

**Files:** Create `internal/rewrite/parity/triallimit.go`, add `TrialRewrite` to `service.go`,
trial handler to `handler.go` + tests.

- [ ] **Step 1 (RED):** memory `TrialLimiter` over limit → 429 body
  `{error:"Trial limit reached. Sign up for unlimited rewrites!"}`. Under limit → 201 `{rewritten_text}`.
  clientIp extraction: `X-Forwarded-For: "1.2.3.4, 5.6.7.8"` → `1.2.3.4`; absent → RemoteAddr.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** `TrialLimiter interface { Incr(ctx, key string, ttlSec int) (int64, error) }`;
  `RedisTrialLimiter` (go-redis INCR + EXPIRE on first incr); memory fake. Limit = `prod?3:999`
  passed in from config. `TrialRewrite` truncates text to 500, calls Completer, grammar/else envelope, status 201.
- [ ] **Step 4:** PASS.
- [ ] **Step 5: Commit** `…&& git commit -m "feat(rewrite): POST /rewrite/trial + IP rate limiter (Node parity)"`

## Task R5: Router + main wiring + routes.txt

**Files:** Modify `internal/shared/router.go`, `cmd/server/main.go`, `deploy/shadow/routes.txt`.

- [ ] **Step 1:** Add `coreHandlers` fields `tones, rewriteParity, rewriteTrial http.Handler`.
- [ ] **Step 2:** Mount: `GET /rewrite/tones` (public, before the JWT group, like other public routes);
  `POST /rewrite` via `ExtOrJWT(r.ExtVerifier, jwtMW, r.Log)(r.rewriteParity)` (same dual-auth as `/v1/rewrite`);
  `POST /rewrite/trial` (public). Keep `/v1/rewrite` exactly as-is.
- [ ] **Step 3:** In `main.go` build `parity.NewService(completer, userRepo)` (tones+trial available
  even without DB? — `/rewrite` needs userRepo so it's inside the `pool != nil` block; `/rewrite/tones`
  is static → wire unconditionally like `imePacksManifest`; `/rewrite/trial` needs only Completer +
  limiter → wire unconditionally). Wire trial limiter: redis client from `REDIS_URL` else memory.
- [ ] **Step 4:** Update `routes.txt`: add the 3 routes; mark `POST /v1/rewrite` with a trailing
  `# go-only (Node 404s; excluded from parity gate)` and make `missingRoutes`/coverage treat it as
  non-required — OR remove it from routes.txt with a comment block explaining why. Pick whichever the
  coverage loader supports cleanly (read `cmd/shadowdiff/routes.go`).
- [ ] **Step 5:** `go build ./... && go test ./... -race && gofmt -l . && go vet ./...` all clean.
- [ ] **Step 6: Commit** `…&& git commit -m "feat(rewrite): mount /rewrite, /rewrite/tones, /rewrite/trial; routes.txt"`

## Task R6: Shadow fixtures (the original Task 14, now authorable)

**Files:** Create `cmd/shadowdiff/fixtures/rewrite/*.json`.

- [ ] `tones.json` — `GET /rewrite/tones` → deterministic, byte-exact (NO ignore_value_of).
- [ ] `rewrite_no_auth.json` — `POST /rewrite` no Authorization → 401 (status + body).
- [ ] `rewrite_bad_tone.json` — `POST /rewrite` `{{user_token}}` + `{"text":"hi","tone":"nope"}` → 400 exact message.
- [ ] `rewrite_jwt_ok.json` / `rewrite_ext_ok.json` — `{{user_token}}` and `{{ext_token}}`,
  valid body → status 201; `ignore_value_of:["rewritten_text"]` (LLM text non-deterministic).
  Keep `usage_today`/`daily_limit` asserted only if deterministic post-reset; if Node's Redis cache
  perturbs `usage_today`, add it to `ignore_value_of` and note why in the fixture name/comment.
- [ ] `trial_bad_body.json` — `POST /rewrite/trial` `{}` → 400.
- [ ] `extract_no_auth.json` / `extract_ok.json` — `POST /extract` (already implemented).
- [ ] **Verify** via `./deploy/shadow/verify-module.sh cmd/shadowdiff/fixtures/rewrite` → all PASS.
- [ ] **Commit** `git add cmd/shadowdiff/fixtures/rewrite && git commit -m "test(shadow): rewrite + extract fixtures"`

---

## After R1–R6
1. Full gate `go build ./... && go vet ./... && gofmt -l . && go test ./... -race` clean.
2. File a GitHub issue documenting the parity defect found + fixed (standing rule).
3. Resume Phase 5 Tasks 15–20.

## Deviations (intentional, value-level only — invisible to the byte gate)
- No Redis result-cache / no background batch pre-generation on `/rewrite` (Node has both; they
  affect only `rewritten_text` value + warm-cache `usage_today`, which the gate ignores for LLM paths).
- Provider resolved from env (`AI_PROVIDERS`), not Node's `ai_providers WHERE is_default` — pre-existing
  rewrite divergence (see `aicall` package doc).
