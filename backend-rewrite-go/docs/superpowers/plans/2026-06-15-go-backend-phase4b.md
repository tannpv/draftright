# Go Backend Port — Phase 4b Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port four NestJS modules — extraction (LLM), email (templates + Resend webhook), bug_reports (multipart ingest), feedback (board) — to the Go drop-in as byte-identical endpoints.

**Architecture:** Each module = a flat clean-arch vertical slice under `internal/<feature>/` (domain → usecase → handler → repo_pg), mirroring the shipped `internal/errreport/` template. A new shared blocking-provider port (`Completer`) extends the existing env-driven AI adapters with a non-streaming `Complete`. Routes mounted in `internal/shared/router.go`; wired in `cmd/server/main.go` `composeDeps`.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5 + sqlc v1.31, golang-jwt/jwt/v5 (HS256, zero clock tolerance), crypto/hmac (Svix), slog.

**Parity authority:** NestJS source under `/opt/openAi/DraftRight/backend/src`. Where this plan and Node disagree, **Node wins**. Spec: `docs/superpowers/specs/2026-06-15-go-backend-phase4b-design.md`.

**Standing rules for every task:**
- TDD: failing test → minimal impl → green → commit.
- **Stage ONLY the task's files. NEVER `git add -A`.**
- Response bodies are ordered STRUCTS with explicit json tags in Node's key order — NEVER `map[string]any` for fixed shapes (Go marshals map keys alphabetically).
- Internal 500s use a FIXED opaque noun-phrase string, never `err.Error()`.
- `@MaxLength`/substring/length use UTF-16 semantics via existing `lenUTF16`/`sliceUTF16` helpers in `internal/shared`.
- Validation: replicate ValidationPipe per the sibling pattern in `internal/auth/validate.go` + `internal/payment/handler_checkout.go` (per-field checks in declaration order; messages joined `". "`; code `invalid-input`).
- After SQL edits run `sqlc generate` and commit the regenerated `internal/shared/pg/sqlc/`.
- Gate before every commit: `go build ./...`, `go test ./... -race`, `gofmt -l .` (empty), `go vet ./...`.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/aicall/completer.go` | `Completer` interface doc + `BlockingProvider` port (shared port the consumers reference) |
| `internal/rewrite/adapter/openai/complete.go` | OpenAI non-streaming `Complete` |
| `internal/rewrite/adapter/anthropic/complete.go` | Anthropic non-streaming `Complete` |
| `internal/rewrite/adapter/ollama/complete.go` | Ollama non-streaming `Complete` |
| `internal/rewrite/adapter/memory/complete.go` | scripted `Complete` fake for tests |
| `internal/auth/optional.go` | shared `OptionalUserID(v *Verifier, r) string` (extracted from errreport) |
| `internal/extraction/{domain,usecase,handler,validate}.go` + tests | `POST /extract` |
| `internal/email/templates.go` (modify) | +4 templates, `formatAmount`, HTML-escape fix |
| `internal/email/service.go` (modify) | +4 typed send methods, `sendTestEmail` |
| `internal/email/webhook.go` + test | `POST /webhooks/resend` Svix verify + event handling |
| `internal/email/repo_pg.go` (modify) | `MarkByProviderID`, `Suppress` |
| `internal/bugreports/{domain,usecase,handler,validate,repo_pg,storage}.go` + tests | `POST /bug-reports` |
| `internal/feedback/{domain,usecase,handler,validate,repo_pg}.go` + tests | 3 feedback routes |
| `internal/shared/pg/queries_bugreports.sql`, `queries_feedback.sql`, `queries_email_webhook.sql` | sqlc |
| `internal/shared/router.go` (modify) | mount 7 routes |
| `cmd/server/main.go` (modify) | `composeDeps` wiring |
| `fixtures/*.json` | shadow fixtures |

---

## Task 1: Blocking-provider port (`Completer`) + adapter `Complete` methods

**Files:**
- Create: `internal/aicall/completer.go`, `internal/rewrite/adapter/memory/complete.go`, `internal/rewrite/adapter/memory/complete_test.go`
- Create: `internal/rewrite/adapter/{openai,anthropic,ollama}/complete.go`
- Reference: `internal/rewrite/domain/ports.go:40` (existing streaming `AiProvider`), `backend/src/ai-providers/ai-providers.service.ts:92` (`callProvider` returns `{text, responseTimeMs}`)

- [ ] **Step 1: Define the port.** Create `internal/aicall/completer.go`:

```go
// Package aicall is the consumer-side port for a blocking (non-streaming)
// AI completion. Node's AiProvidersService.callProvider(provider, system,
// user) returns the full assistant text + round-trip ms; the Go rewrite
// adapters are streaming-only, so this port adds the blocking shape.
//
// Parity note: Go resolves the provider from env (AI_PROVIDERS + keys,
// provider-id pinned), NOT from Node's DB `ai_providers WHERE is_default`.
// This follows the existing rewrite divergence (cmd/server/main.go:543).
package aicall

import "context"

// Completer sends a blocking system+user prompt to the configured default
// provider and returns the full assistant text + round-trip milliseconds.
type Completer interface {
	Complete(ctx context.Context, system, user string) (text string, ms int64, err error)
	// Name is the provider name echoed in responses (e.g. "openai").
	Name() string
}
```

- [ ] **Step 2: Write the failing test for the memory fake.** Create `internal/rewrite/adapter/memory/complete_test.go`:

```go
package memory

import (
	"context"
	"testing"
)

func TestProvider_Complete_ReturnsScripted(t *testing.T) {
	p := NewProvider("memory-stub", nil)
	p.SetCompletion(`[{"kind":"address"}]`)
	got, ms, err := p.Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != `[{"kind":"address"}]` {
		t.Errorf("text = %q", got)
	}
	if ms < 0 {
		t.Errorf("ms = %d", ms)
	}
	if p.Name() != "memory-stub" {
		t.Errorf("name = %q", p.Name())
	}
}
```

- [ ] **Step 3: Run it — expect FAIL** (`Complete`/`SetCompletion` undefined). `go test ./internal/rewrite/adapter/memory/ -run TestProvider_Complete`

- [ ] **Step 4: Implement the memory fake.** Create `internal/rewrite/adapter/memory/complete.go`. Add a `completion string` field on `Provider` (declare it in this file via a small setter; if `Provider` is defined in `provider.go`, add the field there instead and keep methods here):

```go
package memory

import "context"

// SetCompletion scripts the text returned by Complete.
func (p *Provider) SetCompletion(text string) { p.completion = text }

// Complete returns the scripted completion (blocking-provider fake).
func (p *Provider) Complete(_ context.Context, _, _ string) (string, int64, error) {
	return p.completion, 1, nil
}
```

Add `completion string` to the `Provider` struct in `provider.go`.

- [ ] **Step 5: Run — expect PASS.**

- [ ] **Step 6: Implement real adapter `Complete` methods.** For each of openai/anthropic/ollama, create `complete.go` that issues the SAME provider's non-streaming HTTP call. Reuse the adapter's existing client/creds/model fields. OpenAI/Anthropic/Ollama blocking shape: POST chat/messages with `stream:false`, read full text. Mirror the request/auth the streaming path already builds (read each adapter's `client.go` for the existing endpoint + headers; the only change is `stream:false` + reading the full content instead of SSE). Return `(content, time.Since(start).Milliseconds(), nil)`. On non-2xx return `("", ms, fmt.Errorf("provider %s: status %d", Name(), code))`. Each adapter's `Name()` already exists.

  Write one unit test per adapter using an `httptest.Server` asserting `stream:false` body is sent and the parsed content is returned. (3 small tests.)

- [ ] **Step 7: Run all adapter tests — expect PASS.**

- [ ] **Step 8: Commit.**

```bash
git add internal/aicall/ internal/rewrite/adapter/
git commit -m "feat(go-port): blocking Completer port + adapter Complete methods (Phase 4b)"
```

---

## Task 2: Shared optional-JWT helper

**Files:**
- Create: `internal/auth/optional.go`, `internal/auth/optional_test.go`
- Modify: `internal/errreport/handler.go` (replace private `optionalUserID` body with a call to the shared helper)
- Reference: `internal/errreport/handler.go` (`optionalUserID`), `backend/src/bug-reports/jwt-user.service.ts:28`

- [ ] **Step 1: Write the failing test.** Create `internal/auth/optional_test.go`:

```go
package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOptionalUserID_NoHeader_Empty(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	if got := OptionalUserID(nil, r); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestOptionalUserID_NonBearer_Empty(t *testing.T) {
	v := NewVerifier("secret")
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("Authorization", "Token abc")
	if got := OptionalUserID(v, r); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`OptionalUserID` undefined).

- [ ] **Step 3: Implement.** Create `internal/auth/optional.go`:

```go
package auth

import (
	"net/http"
	"strings"
)

// OptionalUserID decodes an optional Bearer JWT and returns the user id
// (Claims.UserID), or "" for anonymous/missing/invalid. Never errors.
// Mirrors Node JwtUserService.decodeOptional + errreport.optionalUserID:
// case-sensitive "Bearer " prefix, raw slice(7), any verify error → "".
func OptionalUserID(v *Verifier, r *http.Request) string {
	if v == nil {
		return ""
	}
	const p = "Bearer "
	authz := r.Header.Get("Authorization")
	if !strings.HasPrefix(authz, p) {
		return ""
	}
	claims, err := v.Verify(authz[len(p):])
	if err != nil {
		return ""
	}
	return claims.UserID()
}
```

- [ ] **Step 4: Run — expect PASS.**

- [ ] **Step 5: Refactor errreport to use it.** In `internal/errreport/handler.go`, replace the body of the private `optionalUserID` method with `return auth.OptionalUserID(h.verifier, r)` (keep the method as a thin wrapper to minimise the diff), add the `auth` import. Run `go test ./internal/errreport/ -race` — expect PASS (unchanged behaviour).

- [ ] **Step 6: Commit.**

```bash
git add internal/auth/optional.go internal/auth/optional_test.go internal/errreport/handler.go
git commit -m "refactor(go-port): shared auth.OptionalUserID, reused by errreport (Phase 4b)"
```

---

## Task 3: extraction — domain + output pipeline (use case)

**Files:**
- Create: `internal/extraction/domain.go`, `internal/extraction/usecase.go`, `internal/extraction/usecase_test.go`
- Reference: `backend/src/extraction/extraction.service.ts`, `extraction/dto/extract.dto.ts`

- [ ] **Step 1: Write domain types.** Create `internal/extraction/domain.go`:

```go
package extraction

// EntityKind enumerates extractable entity kinds (extract.dto.ts EntityKind).
type EntityKind string

const (
	KindPhone       EntityKind = "phone"
	KindEmail       EntityKind = "email"
	KindURL         EntityKind = "url"
	KindOTP         EntityKind = "otp"
	KindCreditCard  EntityKind = "creditCard"
	KindAddress     EntityKind = "address"
	KindPersonName  EntityKind = "personName"
	KindDateTime    EntityKind = "dateTime"
	KindBankAccount EntityKind = "bankAccount"
)

// allKinds preserves declaration order for @IsEnum validation messages.
var allKinds = []EntityKind{KindPhone, KindEmail, KindURL, KindOTP, KindCreditCard, KindAddress, KindPersonName, KindDateTime, KindBankAccount}

var regexHandled = map[EntityKind]bool{KindPhone: true, KindEmail: true, KindURL: true, KindOTP: true, KindCreditCard: true}

// Entity is one extracted entity. JSON key order matches Node
// ExtractedEntityDto: kind,value,display,start,end,confidence,meta?.
type Entity struct {
	Kind       EntityKind        `json:"kind"`
	Value      string            `json:"value"`
	Display    string            `json:"display"`
	Start      int               `json:"start"`
	End        int               `json:"end"`
	Confidence float64           `json:"confidence"`
	Meta       map[string]string `json:"meta,omitempty"`
}

// Response key order matches ExtractResponseDto: entities,provider,tokensUsed.
type Response struct {
	Entities   []Entity `json:"entities"`
	Provider   string   `json:"provider"`
	TokensUsed int      `json:"tokensUsed"`
}
```

- [ ] **Step 2: Write the failing use-case test.** Create `internal/extraction/usecase_test.go` covering: (a) hallucinated value (not a substring) dropped; (b) regexHandled kind dropped; (c) dedupe keeps higher confidence + sorts by start; (d) unparseable output → `{entities:[],provider,tokensUsed:0}`; (e) tokensUsed = ceil((len(text)+len(raw))/4) UTF-16. Use a fake `Completer` returning scripted JSON.

```go
package extraction

import (
	"context"
	"testing"
)

type fakeCompleter struct{ out string }

func (f fakeCompleter) Complete(context.Context, string, string) (string, int64, error) { return f.out, 1, nil }
func (f fakeCompleter) Name() string                                                    { return "openai" }

func TestExtract_DropsHallucinationAndRegexKinds_Dedupes(t *testing.T) {
	text := "addr 123 Le Loi and 123 Le Loi again"
	raw := `[{"kind":"address","value":"123 Le Loi","confidence":0.7},{"kind":"address","value":"123 Le Loi","confidence":0.9},{"kind":"phone","value":"123 Le Loi"},{"kind":"address","value":"NOT THERE"}]`
	svc := NewService(fakeCompleter{out: raw})
	got, err := svc.Extract(context.Background(), text, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entities) != 1 {
		t.Fatalf("entities = %d, want 1 (dedup + drop phone + drop hallucination)", len(got.Entities))
	}
	if got.Entities[0].Confidence != 0.9 {
		t.Errorf("confidence = %v, want 0.9 (dedup keeps max)", got.Entities[0].Confidence)
	}
	if got.Provider != "openai" {
		t.Errorf("provider = %q", got.Provider)
	}
}

func TestExtract_Unparseable_EmptyZeroTokens(t *testing.T) {
	svc := NewService(fakeCompleter{out: "not json"})
	got, _ := svc.Extract(context.Background(), "hi", nil)
	if len(got.Entities) != 0 || got.TokensUsed != 0 {
		t.Errorf("want empty + 0 tokens, got %+v", got)
	}
}
```

- [ ] **Step 3: Run — expect FAIL** (`NewService`/`Extract` undefined).

- [ ] **Step 4: Implement the use case.** Create `internal/extraction/usecase.go` porting `extraction.service.ts` verbatim:
  - `Completer` port (consumer copy: re-declare the 2-method interface here, accept it in `NewService` — "accept interfaces").
  - `Extract(ctx, text string, kinds []EntityKind)`: call `Complete(ctx, buildSystemPrompt(kinds), text)`; `stripCodeFences`; `json.Unmarshal` into `[]json.RawMessage` (guard: on error → `{nil-slice→[]…}` return with `Provider: c.Name()`, `TokensUsed: 0`; **entities must marshal as `[]` not `null`** — initialise `out := []Entity{}`); non-array → same empty.
  - `validateEntity`: object guard; kind ∈ allKinds; `regexHandled` drop; `value` non-empty string; `start = indexUTF16(text, value)` (use existing `lenUTF16`/UTF-16 index helper in `internal/shared`; add `IndexUTF16` if absent — start/end are UTF-16 offsets to match JS `String.indexOf`/`.length`); `< 0` → drop; display = trimmed raw.display if non-empty else value; confidence = number or 0.5, clamp 0..1; meta = string→string map or nil.
  - `dedupe`: map key `string(kind)+":"+strings.ToLower(value)`, keep max confidence, then sort by `Start` ascending (stable).
  - `tokensUsed = (lenUTF16(text+raw) + 3) / 4` (== `ceil(n/4)`).
  - `buildSystemPrompt`: port the exact 9-line array `.join("\n")` from `extraction.service.ts:68-77` BYTE-FOR-BYTE (incl. the Vietnamese example line). Default kinds when nil = address,personName,dateTime,bankAccount; filter out regexHandled; `allowed.join("|")`; `[...REGEX_HANDLED].join("|")` must emit insertion order `phone|email|url|otp|creditCard`.

- [ ] **Step 5: Run — expect PASS.**

- [ ] **Step 6: Commit.**

```bash
git add internal/extraction/domain.go internal/extraction/usecase.go internal/extraction/usecase_test.go internal/shared/
git commit -m "feat(go-port): extraction use case — LLM output pipeline parity (Phase 4b)"
```

---

## Task 4: extraction — validation + handler

**Files:**
- Create: `internal/extraction/validate.go`, `internal/extraction/handler.go`, `internal/extraction/handler_test.go`
- Reference: `extraction.controller.ts` (JWT guard, `@HttpCode(200)`), `extract.dto.ts`

- [ ] **Step 1: Write the failing handler test.** Cover: (a) `text` > 8000 → 400 `text must be shorter than or equal to 8000 characters`; (b) bad `kinds` enum → 400; (c) happy path → 200 with `{entities,provider,tokensUsed}` and `entities:[]` (not null) when empty; (d) no-default-provider error → 400 `No default AI provider configured`.

- [ ] **Step 2: Run — expect FAIL.**

- [ ] **Step 3: Implement validation.** Create `internal/extraction/validate.go`:
  - `requestBody{ Text string `json:"text"`; Kinds []string `json:"kinds"` }` — decode with plain `json.Decode` (no DisallowUnknownFields, matching auth fidelity).
  - `validateExtract(b)`: `text` `@MaxLength(8000)` via `lenUTF16` → `text must be shorter than or equal to 8000 characters`; `kinds` `@ArrayMaxSize(20)` → `kinds must contain no more than 20 elements`; each kind ∈ allKinds else `each value in kinds must be one of the following values: phone, email, url, otp, creditCard, address, personName, dateTime, bankAccount` (confirm exact class-validator @IsEnum message at port time against a Node run). Join failures `". "`, code `invalid-input`.

- [ ] **Step 4: Implement handler.** Create `internal/extraction/handler.go`:
  - `Handler{ svc *Service }`; `NewHandler(svc *Service) *Handler`.
  - `Extract(w,r)`: decode → `validateExtract` → on fail `shared.WriteError(w,r,"invalid-input",msg)`; call `svc.Extract`; on `ErrNoDefaultProvider` → `shared.WriteError(w,r,"invalid-input","No default AI provider configured")`; other error → `shared.WriteError(w,r,"internal","extraction failed")`; success → `shared.WriteJSON(w, 200, resp)`.
  - Route is mounted under `RequireAuth` (Task 9), so the handler does NOT re-check JWT.
  - Add `ErrNoDefaultProvider = errors.New("No default AI provider configured")` in `usecase.go`, returned when `Complete` reports no provider (the env chain returns the memory stub in dev; real "no provider" surfaces as the adapter error — map the adapter's no-default error to this sentinel; confirm exact trigger at port time).

- [ ] **Step 5: Run — expect PASS.**

- [ ] **Step 6: Commit.**

```bash
git add internal/extraction/validate.go internal/extraction/handler.go internal/extraction/handler_test.go internal/extraction/usecase.go
git commit -m "feat(go-port): POST /extract handler + ValidationPipe parity (Phase 4b)"
```

---

## Task 5: email — 4 templates + formatAmount + HTML-escape parity fix

**Files:**
- Modify: `internal/email/templates.go`, `internal/email/service.go`
- Create/extend: `internal/email/templates_test.go`
- Reference: `backend/src/email/email-templates.ts:51-87`, `email.service.ts:127-205`

- [ ] **Step 1: Write the failing test.** In `templates_test.go`: (a) `renderTemplate("subscription-activated", {name,plan,amount,expires})` substitutes all tokens; (b) HTML-escape: a var containing `<b>` renders `&lt;b&gt;` in html body but raw in subject; (c) `formatAmount("USD", 999)` == `$9.99`; `formatAmount("VND", 50000)` == `50,000 VND`.

- [ ] **Step 2: Run — expect FAIL.**

- [ ] **Step 3: Add the 4 templates.** In `templates.go` `builtinTemplates`, add `subscription-activated`, `subscription-expired`, `renewal-reminder`, `payment-failed` — copy `subject` + `html` (via `shell(...)`) BYTE-FOR-BYTE from `email-templates.ts:51-87`.

- [ ] **Step 4: Fix the HTML-escape divergence.** Change `substitute` to take an `escape bool` and escape values in html context only (mirror `email.service.ts:190-205`):

```go
func renderTemplate(key string, vars map[string]string) (subject, html string) {
	t, ok := builtinTemplates[key]
	if !ok {
		return "", ""
	}
	return substitute(t.subject, vars, false), substitute(t.html, vars, true)
}

func substitute(s string, vars map[string]string, escape bool) string {
	return tokenRe.ReplaceAllStringFunc(s, func(m string) string {
		k := m[2 : len(m)-2] // strip {{ }}
		v, ok := vars[k]
		if !ok {
			return "" // Node: vars[k] ?? ''
		}
		if escape {
			return escapeHTML(v)
		}
		return v
	})
}

var tokenRe = regexp.MustCompile(`\{\{(\w+)\}\}`)

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}
```

Note: Node `substitute` replaces only `\w+` tokens with a value-or-`''`; unknown `{{x}}` → `''`. The current Go `strings.ReplaceAll` loop leaves unknown tokens intact — the regex version fixes that too (parity). Update the existing `service.go render()` caller if its signature changed.

- [ ] **Step 5: Add `formatAmount` + typed send methods.** In `service.go` add `formatAmount(currency string, amountCents int) string` (USD → `fmt.Sprintf("$%.2f", float64(c)/100)`; else `humanizeThousands(amount)+" "+currency` — match JS `toLocaleString('en-US')` grouping with commas) and the methods `SendRenewalReminder`, `SendPaymentFailed`, `SendSubscriptionActivated`, `SendSubscriptionExpired`, `SendTestEmail` mirroring `email.service.ts:127-163` (vars: `name || 'there'`, `plan`, `expires = expiresAt` as JS `Date.toDateString()` → e.g. `Mon Jun 15 2026`; **port a `dateString` helper matching JS `toDateString()` exactly** — `Www Mmm DD YYYY`). `SendTestEmail` builds the inline html (copy from `email.service.ts:128-138`, including the `new Date().toISOString()` stamp via `shared.ISOMillis(s.now())`).

- [ ] **Step 6: Run — expect PASS.**

- [ ] **Step 7: Commit.**

```bash
git add internal/email/templates.go internal/email/service.go internal/email/templates_test.go
git commit -m "feat(go-port): 4 email templates + formatAmount + HTML-escape parity (Phase 4b)"
```

---

## Task 6: email — Resend webhook (`POST /webhooks/resend`)

**Files:**
- Create: `internal/email/webhook.go`, `internal/email/webhook_test.go`
- Modify: `internal/email/repo_pg.go`, `internal/shared/pg/queries_email_webhook.sql`, `sqlc.yaml`
- Reference: `backend/src/email/email-webhook.controller.ts`

- [ ] **Step 1: Add sqlc queries.** Create `internal/shared/pg/queries_email_webhook.sql`:

```sql
-- name: MarkEmailByProviderID :exec
UPDATE email_logs SET status = $2, error_message = $3 WHERE provider_message_id = $1;

-- name: SuppressEmail :exec
INSERT INTO email_suppressions (email, reason) VALUES ($1, $2)
ON CONFLICT (email) DO NOTHING;
```

Confirm the real column names against `schema.sql` (`email_logs` provider id column; `email_suppressions` columns) and adjust. Register the file in `sqlc.yaml` `queries:`. Run `sqlc generate`.

- [ ] **Step 2: Add repo methods.** In `repo_pg.go` add `MarkByProviderID(ctx, id, status string, reason *string) error` and `Suppress(ctx, email, reason string) error` (lowercase email before insert — Node lowercases; verify). Add to the service's `Querier`/repo port as needed.

- [ ] **Step 3: Write the failing webhook test.** `webhook_test.go`: (a) missing/!`whsec_` secret or bad signature → 400 `{"error":"Invalid webhook signature",...}`; (b) valid signature + bad JSON → 400 `Invalid payload`; (c) valid `email.delivered` → 200 `{"received":true}` + `MarkByProviderID(id,"delivered",nil)` called; (d) `email.bounced` with `bounce.type:"Permanent"` → mark bounced + `Suppress(to,"bounced")`; transient bounce → NO suppress; (e) `email.complained` → mark + suppress. Build a valid Svix signature in-test: `base64(hmac_sha256(base64decode(secret without whsec_), id+"."+ts+"."+body))`, header `v1,<sig>`.

- [ ] **Step 4: Run — expect FAIL.**

- [ ] **Step 5: Implement.** Create `internal/email/webhook.go`:
  - `WebhookHandler{ svc *Service, secret string }`; `NewWebhookHandler(svc *Service, secret string) *WebhookHandler`.
  - `Handle(w,r)`: read **raw body** via `io.ReadAll(r.Body)` (router must NOT pre-consume it); `verify(secret, r.Header, raw)`; on fail `shared.WriteError(w,r,"invalid-input"→400,"Invalid webhook signature")` (confirm Node's BadRequest envelope code maps to status 400; pick the code that yields 400 — likely a generic 400 code; verify against AllExceptionsFilter); `json.Unmarshal` → on fail 400 `Invalid payload`; switch on `event.type` exactly as controller `:53-74`; success → `shared.WriteJSON(w,200,struct{Received bool `json:"received"`}{true})`.
  - `verify`: port `email-webhook.controller.ts:79-94` — headers `svix-id`/`svix-timestamp`/`svix-signature`; `key = base64.StdDecoding(strings.TrimPrefix(secret,"whsec_"))`; `expected = base64.StdEncoding(hmacSHA256(key, id+"."+ts+"."+string(raw)))`; split sig header on space, each part `v1,<sig>`, `hmac.Equal([]byte(part1), []byte(expected))`.

- [ ] **Step 6: Run — expect PASS.**

- [ ] **Step 7: Commit.**

```bash
git add internal/email/webhook.go internal/email/webhook_test.go internal/email/repo_pg.go internal/shared/pg/ sqlc.yaml
git commit -m "feat(go-port): POST /webhooks/resend Svix verify + suppression (Phase 4b)"
```

---

## Task 7: bug_reports — repo + screenshot storage + sqlc

**Files:**
- Create: `internal/bugreports/domain.go`, `internal/bugreports/repo_pg.go`, `internal/bugreports/storage.go`, `internal/bugreports/repo_pg_test.go`, `internal/bugreports/storage_test.go`
- Create: `internal/shared/pg/queries_bugreports.sql` (register in `sqlc.yaml`)
- Reference: `bug-reports.service.ts:62-118,405-418`, `entities/bug-report.entity.ts`

- [ ] **Step 1: Add sqlc queries.** `queries_bugreports.sql`:

```sql
-- name: UserExists :one
SELECT EXISTS(SELECT 1 FROM users WHERE id = $1);

-- name: InsertBugReport :one
INSERT INTO bug_reports
  (source, description, screenshot_path, screenshot_filename, app_version,
   os_info, user_id, user_email, context, status, kind)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'new','bug')
RETURNING id, display_no;
```

Confirm column set/order + `context` jsonb param handling against `schema.sql:223`. Run `sqlc generate`.

- [ ] **Step 2: Write the failing storage test.** `storage_test.go`: `Save(buf, mimetype)` with `image/png` writes `<root>/YYYY-MM-DD/<uuid>.png` and returns the full path + filename; `image/webp` → error `only PNG or JPEG screenshots are accepted` (extensionFor asymmetry). Use a temp dir as root + an injected clock for the date.

- [ ] **Step 3: Run — expect FAIL.**

- [ ] **Step 4: Implement storage.** `storage.go`: `Storage{ root string; now func() time.Time }`; `extensionFor(mime)` → `.png`/`.jpg` else error; `Save(buf []byte, mime, originalName string) (path, filename string, err error)` → `dir = root/now().Format("2006-01-02")`, `os.MkdirAll`, `name = uuid()+ext`, `os.WriteFile`, filename = `originalName` or `name`.

- [ ] **Step 5: Implement repo + domain.** `domain.go`: `Created{ ID string; DisplayNo int64 }`, sentinel errors. `repo_pg.go`: `ResolveUserID(ctx, id string) (pgtype-null)` via `UserExists`; `Insert(ctx, args) (Created, error)`. Write `repo_pg_test.go` only for pure logic (or skip DB-bound test, matching errreport's no-DB test approach).

- [ ] **Step 6: Run — expect PASS.**

- [ ] **Step 7: Commit.**

```bash
git add internal/bugreports/ internal/shared/pg/ sqlc.yaml
git commit -m "feat(go-port): bug_reports repo + screenshot storage + sqlc (Phase 4b)"
```

---

## Task 8: bug_reports — use case + validation + handler (`POST /bug-reports`)

**Files:**
- Create: `internal/bugreports/usecase.go`, `internal/bugreports/validate.go`, `internal/bugreports/handler.go`, `internal/bugreports/handler_test.go`
- Reference: `bug-reports.controller.ts`, `create-bug-report.dto.ts`

- [ ] **Step 1: Write the failing handler test.** Multipart requests via `mime/multipart`. Cover: (a) honeypot `website` non-empty → 201 `{"id":null,"status":"received"}` (exact shape) + NO insert; (b) missing `description` → 400 `description must be longer than or equal to 1 characters` (@MinLength) — confirm exact class-validator message; (c) `source` > 50 → 400; (d) disallowed mime (`image/webp`) → 400 `only PNG or JPEG screenshots are accepted`; (e) happy path PNG → 201 `{"id":...,"ref":"BUG-<n>","message":"Bug report received. Thanks! Reference: BUG-<n>"}`; (f) oversize > 5MB → confirm Node behaviour (likely 400 from multer `LIMIT_FILE_SIZE`, message TBD — verify at port time; default to 400 if multer, NOT the errreport 500 path which was body-parser JSON).

- [ ] **Step 2: Run — expect FAIL.**

- [ ] **Step 3: Implement validation.** `validate.go`: `validateBugReport(fields)` checks in DTO declaration order — `description` @IsString+@MinLength(1); `source` @MaxLength(50); `app_version?` 50; `os_info?` 255; `user_email?` 255; `website?` 255 — UTF-16 lengths, messages joined `". "`, code `invalid-input`. (Honeypot is checked in the handler BEFORE validation? Node: ValidationPipe runs first, then controller honeypot check. So validate THEN honeypot — match order.)

- [ ] **Step 4: Implement use case + handler.** `usecase.go`: `Service{ repo Repo; store *Storage }`; `Create(ctx, in CreateInput, file *FilePart, userID string) (Created, error)` porting `bug-reports.service.ts:62-118` (trim/validate description+source → `description is required`/`source is required` as 400; write file via store; parse `context` JSON → fallback `{"raw":...}`; slice fields to column widths via `sliceUTF16`; resolve user id). `handler.go`: parse multipart (`r.ParseMultipartForm(5<<20)`), enforce mime allow-list at the controller layer (the 7 ALLOWED_MIMES → reject others 400 `only PNG or JPEG screenshots are accepted`), honeypot check, `auth.OptionalUserID`, call service, build `ref = "BUG-"+strconv.FormatInt(displayNo,10)`, respond 201 with ordered struct `{id, ref, message}`. Honeypot returns a SEPARATE ordered struct `{id:null, status:"received"}`.

- [ ] **Step 5: Run — expect PASS.**

- [ ] **Step 6: Commit.**

```bash
git add internal/bugreports/
git commit -m "feat(go-port): POST /bug-reports multipart ingest + honeypot (Phase 4b)"
```

---

## Task 9: feedback — repo + use case + sqlc

**Files:**
- Create: `internal/feedback/domain.go`, `internal/feedback/usecase.go`, `internal/feedback/repo_pg.go`, `internal/feedback/usecase_test.go`
- Create: `internal/shared/pg/queries_feedback.sql` (register in `sqlc.yaml`)
- Reference: `bug-reports.service.ts:120-240`, `entities/{bug-report,feature-vote}.entity.ts`

- [ ] **Step 1: Add sqlc queries.** `queries_feedback.sql`: `InsertFeedback` (kind,title,target_platform,vote_count=0,is_public=true,source,description,app_version,os_info,user_id,user_email,context=null,status='new' RETURNING id, display_no); `CountFeatures` + `ListPublicFeatures` (WHERE kind='feature' AND is_public [+ optional status/target_platform], ORDER BY vote_count DESC, created_at DESC, LIMIT/OFFSET, returning ALL bug_reports columns in entity order); `FindFeature` (by id, kind='feature'); `FindVote`, `InsertVote`, `DeleteVote`, `CountVotes`, `VotedFeatureIDs` (feature_id IN (...) AND user_id=$). Run `sqlc generate`.

- [ ] **Step 2: Write the failing use-case test.** Cover: (a) `CreateFeedback` kind=feature missing title → 400 `title is required for a feature request (1-80 characters)`; (b) feature bad target_platform → 400 `target_platform must be one of: playground, mobile, windows, mac, linux`; (c) `ToggleVote` toggles + recomputes count; (d) `ToggleVote` on non-feature id → NotFound `feature request not found`. Use fakes for the repo.

- [ ] **Step 3: Run — expect FAIL.**

- [ ] **Step 4: Implement domain + use case.** `domain.go`: `FeatureRow` struct with json tags in EXACT entity order: `id, display_no, source, description, screenshot_path, screenshot_filename, app_version, os_info, user_id, user_email, context, status, kind, title, target_platform, vote_count, is_public, admin_notes, ai_fix_proposal, ai_fix_proposed_at, created_at, updated_at, viewerHasVoted` (last). **`display_no` serializes as a STRING** (TypeORM bigint → JS string) — type it `string` with `json:"display_no"`. Timestamps via `shared.ISOMillis`. `usecase.go`: port `createFeedback` (`:126-169`), `toggleVote` (`:182-197`), `listPublicFeatures` (`:205-236`) incl. page≥1, limit clamp 1..100 default 20, `viewerHasVoted` set from `VotedFeatureIDs`.

- [ ] **Step 5: Run — expect PASS.**

- [ ] **Step 6: Commit.**

```bash
git add internal/feedback/ internal/shared/pg/ sqlc.yaml
git commit -m "feat(go-port): feedback repo + use case (create/vote/list) (Phase 4b)"
```

---

## Task 10: feedback — validation + handlers (3 routes)

**Files:**
- Create: `internal/feedback/validate.go`, `internal/feedback/handler.go`, `internal/feedback/handler_test.go`
- Reference: `feedback.controller.ts`, `create-feedback.dto.ts`

- [ ] **Step 1: Write the failing handler test.** Cover: (a) `POST /feedback` honeypot → 201 `{"id":null,"message":"Received. Thanks!"}`; (b) `POST /feedback` bad `kind` → 400 `kind must be one of the following values: bug, feature` (@IsIn); (c) happy feature → 201 `{"id","ref":"FR-<n>","message":"Feature request received. Thanks! Reference: FR-<n>"}`; bug → noun "Bug report", `ref:"BUG-<n>"`; (d) `GET /feedback` → 200 `{"rows":[...],"total":N}` with `viewerHasVoted` last key; (e) `POST /feedback/:id/vote` no JWT → 401 `sign in to vote`; with JWT → 200 `{"vote_count","hasVoted"}`.

- [ ] **Step 2: Run — expect FAIL.**

- [ ] **Step 3: Implement validation.** `validate.go`: `validateFeedback` in DTO order — `kind` @IsIn(['bug','feature']) → `kind must be one of the following values: bug, feature`; `title?` @MaxLength(80); `target_platform?` @IsIn(TARGET_PLATFORMS) → `target_platform must be one of the following values: playground, mobile, windows, mac, linux`; `description` @MinLength(1)+@MaxLength(2000); `source` @MaxLength(50); `app_version?`/`os_info?`/`user_email?`/`website?`. Joined `". "`, code `invalid-input`. (Service-level feature title/target_platform checks live in the use case from Task 9.)

- [ ] **Step 4: Implement handlers.** `handler.go`: `Handler{ svc *Service; verifier *auth.Verifier }`.
  - `Create(w,r)`: decode JSON → `validateFeedback` → honeypot (`website` non-empty → 201 ordered `{id:null, message:"Received. Thanks!"}`) → `auth.OptionalUserID` → `svc.CreateFeedback` (maps service 400s to `invalid-input`) → build `ref` (`FR-`/`BUG-`) + noun → 201 `{id, ref, message}`.
  - `List(w,r)`: parse query (page/limit/status/target_platform) → `auth.OptionalUserID` → `svc.ListPublicFeatures` → 200 `{rows, total}`.
  - `Vote(w,r)`: `id` from chi URL param → `auth.OptionalUserID`; empty → `shared.WriteError(w,r,"unauthorized"→401,"sign in to vote")` (use the code that yields 401 per `StatusForCode`; confirm); non-feature → `not-found`→404 `feature request not found`; success → 200 `{vote_count, hasVoted}`.

- [ ] **Step 5: Run — expect PASS.**

- [ ] **Step 6: Commit.**

```bash
git add internal/feedback/
git commit -m "feat(go-port): feedback handlers — POST/GET /feedback + vote (Phase 4b)"
```

---

## Task 11: Router + main.go wiring + fixtures

**Files:**
- Modify: `internal/shared/router.go`, `cmd/server/main.go`
- Create: `internal/shared/router_phase4b_test.go`, `fixtures/*.json`
- Reference: `internal/shared/router.go` (Phase 4a public/auth mounts), `cmd/server/main.go` `composeDeps`

- [ ] **Step 1: Add router fields + mounts.** In `router.go` add handler fields: `ExtractHandler` (under `RequireAuth`/auth group), `EmailWebhook` (PUBLIC, needs RAW body — ensure no JSON body middleware strips it), `BugReportIngest` (PUBLIC), `FeedbackCreate` (PUBLIC), `FeedbackList` (PUBLIC GET), `FeedbackVote` (PUBLIC route, handler enforces JWT manually). Mount nil-guarded:
  - `POST /extract` inside the auth group (RequireAuth).
  - `POST /webhooks/resend`, `POST /bug-reports`, `POST /feedback`, `GET /feedback`, `POST /feedback/{id}/vote` PUBLIC (before the auth group).

- [ ] **Step 2: Write the failing router test.** `router_phase4b_test.go`: assert public routes reachable WITHOUT JWT (non-401), and `POST /extract` WITHOUT JWT → 401. Mirror `router_phase4a_test.go`.

- [ ] **Step 3: Run — expect FAIL.**

- [ ] **Step 4: Wire `composeDeps`.** In `main.go`:
  - Build the `Completer` from the existing provider chain concrete (it already implements `Complete` after Task 1) — pass it to `extraction.NewService`. Wire `extraction.NewHandler` → `core.extractHandler`.
  - Inside `pool != nil`: build `bugreports` (repo + `Storage{root: cfg.BugReportsDir, now: time.Now}`) + handler; `feedback` repo+service+handler (pass `core.accessVerifier`); `email.NewWebhookHandler(emailSvc, cfg.ResendWebhookSecret)`. Add `BugReportsDir` + `ResendWebhookSecret` to config if absent (env `BUG_REPORTS_DIR` default `/var/lib/draftright/bug-reports`, `RESEND_WEBHOOK_SECRET`).
  - Assign all router fields in the Router literal.

- [ ] **Step 5: Run — expect PASS.** Full gate: `go build ./...`, `go test ./... -race`, `gofmt -l .`, `go vet ./...`.

- [ ] **Step 6: Add fixtures.** Create deterministic shadow fixtures (mirror `fixtures/payment_status.json` schema): `feedback_get.json` (GET `/feedback`, ignore volatile `rows`/timestamps or use empty-DB), `feedback_bad_kind.json` (POST 400), `feedback_honeypot.json` (POST 201 `{id:null,message:"Received. Thanks!"}`), `bugreport_honeypot.json` (multipart honeypot 201 `{id:null,status:"received"}` — if the fixture harness can't do multipart, document the exclusion + cover via unit test). Validate each is valid JSON. DB-writing success paths + `/extract` (LLM) EXCLUDED (non-deterministic).

- [ ] **Step 7: Commit.**

```bash
git add internal/shared/router.go internal/shared/router_phase4b_test.go cmd/server/main.go fixtures/
git commit -m "feat(go-port): wire Phase 4b routes + shadow fixtures (Phase 4b)"
```

---

## Open parity items — confirm against a live Node run before/while implementing

These need a Node response captured to lock the byte-exact string (do NOT guess silently — if a Node run isn't available, implement the best estimate AND leave a `// PARITY-TODO` + flag in the task's commit body):

1. class-validator `@IsEnum` message for `kinds` (extraction) and `@IsIn` messages (feedback `kind`/`target_platform`) — exact wording + value list separator.
2. `@MinLength(1)` message for bug_reports `description` (`must be longer than or equal to 1 characters`).
3. Multipart oversize (>5MB) Node status + body (multer `LIMIT_FILE_SIZE`) — vs the errreport JSON 500 path.
4. Envelope `code` AllExceptionsFilter assigns to a bare `BadRequestException(string)` (no explicit code) → which `code` string + 400; and to `UnauthorizedException`/`NotFoundException` → 401/404 codes. Align `shared.StatusForCode` usage.
5. `GET /feedback` exposes full entity incl. `user_email`, `admin_notes`, `ai_fix_proposal` on a PUBLIC route — replicate for parity, but file a follow-up GitHub issue noting the info-disclosure (port faithfully now; flag separately).
6. JS `Date.toDateString()` exact format (`Mon Jun 15 2026`) for email `expires` var, and `toLocaleString('en-US')` comma grouping for `formatAmount` non-USD.
7. `sub || user_id` claim fallback (tracked auth-module item) — `auth.Claims.UserID()` returns `sub` only; dead edge for DraftRight, but note in Task 2.

---

## Self-Review

**Spec coverage:** Completer (T1) · optional-JWT (T2) · extraction usecase+handler (T3,T4) · email templates+escape (T5) · email webhook (T6) · bug_reports storage+repo+ingest (T7,T8) · feedback repo+usecase+handlers (T9,T10) · wiring+fixtures (T11). All spec sections mapped.

**Placeholder scan:** Long HTML templates intentionally cite exact Node source lines for byte-for-byte copy (mechanical, unambiguous) rather than re-pasting 60 lines ×4. Open parity items (status strings, multer behaviour) are explicitly flagged as "confirm against Node run" with a fallback instruction — not silent TODOs.

**Type consistency:** `Completer` (2 methods Complete+Name) consistent T1↔T3↔T11. `Created{ID,DisplayNo}` consistent T7↔T8. `FeatureRow` json order fixed T9, consumed T10. `auth.OptionalUserID(v,r)` signature consistent T2↔T8↔T10. `ref` built as `PREFIX-<display_no>` inline (matches shipped errreport), no shared formatter (YAGNI).
