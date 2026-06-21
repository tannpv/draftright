# #57 — top-level JSON `null` body 400 parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Make every JSON-request-body handler reject a top-level literal `null`
body with Node's Express strict-body-parser 400, instead of Go's current
proceed-with-zero-struct (which surfaces as 401 on `/auth/login`).

**Architecture:** One shared decoder `shared.DecodeJSON(w, r, dst, mode)` that
buffers the body once, intercepts a literal `null` body with the pinned Node
400, and otherwise **replays the exact original `json.NewDecoder(...).Decode`**
so empty/malformed/trailing-data semantics stay byte-identical. A `DecodeMode`
enum preserves each call site's pre-existing empty/malformed tolerance. Adopt at
all 21 request-body decode sites. The change is **additive: only the `null` case
changes behavior.**

**Tech Stack:** Go 1.26, chi, `encoding/json`, shadow-gate fixtures.

**Parity authority:** Node `:3200` in the live shadow gate. The hardcoded null
message is pinned to the running prod Node version; if the gate reports a
mismatch, replace the constant with whatever Node actually emits (capture it
from the gate's Node side — do NOT guess).

---

## Why a shared decoder, not a middleware

Node's body-parser is global middleware that runs **before** route guards, but
the webhook routes (`/webhooks/resend`, `/payment/webhook/*`) use
`express.raw()` and bypass json body-parser. A global Go middleware would
(a) buffer raw-body signature routes (violating the router.go:328-330 invariant)
and (b) wrongly reject `null` on those webhook routes where Node does **not**.
A per-decode-site helper affects exactly the routes Node json-body-parses —
the faithful match. Residual: `null` + unauthenticated on a guarded route still
returns Go 401 vs Node 400 (guard runs before the handler's decode); doubly
theoretical, documented as out of scope.

## Mode mapping (all 21 sites, all currently emit `invalid-input` / "Invalid request body")

**DecodeStrict** — `err != nil` → 400 (empty body also 400, via EOF):
- `internal/auth/handler.go:77` (the `decodeJSON` wrapper — covers Login/Refresh/Register/ChangePassword/VerifyEmail/ResendVerification/ForgotPassword/ResetPassword/Social)
- `internal/rewritelog/handler.go:138`
- `internal/plans/admin_handler.go:127`, `:174`
- `internal/payment/handler_checkout.go:76`
- `internal/adminauth/handler.go:67`, `:91`
- `internal/adminauth/admin_users_handler.go:125`, `:159`

**DecodeOptional** — `err != nil && err != io.EOF` → 400 (empty body proceeds):
- `internal/payment/admin_handler.go:107`, `:127`
- `internal/updates/admin_handler.go:98`, `:140`
- `internal/bugreports/admin_handler.go:208`
- `internal/errreport/admin_handler.go:147`
- `internal/appsettings/handler.go:108`, `:195`

**DecodeLenient** — `_ = Decode(...)` (all errors ignored, proceed):
- `internal/email/admin_templates_handler.go:78`
- `internal/exttoken/handler.go:57`

---

## Task 1: shared.DecodeJSON + DecodeMode (TDD)

**Files:**
- Create: `internal/shared/decode.go`
- Test: `internal/shared/decode_test.go`

- [ ] **Step 1: Write failing table-driven test** in `decode_test.go`. Cover the
  matrix `{null, empty, malformed, valid, valid+trailing} × {Strict, Optional,
  Lenient}` plus whitespace-wrapped null (`"  null  "`). Assert:
  - literal `null` (any mode) → returns false, response 400, body
    `{"error":"Unexpected token 'n', \"null\" is not valid JSON","code":"invalid-input","request_id":...}` (assert error+code; request_id present).
  - empty body: Strict → false+400 `"Invalid request body"`; Optional/Lenient → true, dst untouched.
  - malformed (`{`): Strict/Optional → false+400 `"Invalid request body"`; Lenient → true.
  - valid (`{"email":"a"}`): all modes → true, dst populated.
  - valid+trailing (`{}{}`): all modes → true (Decoder stops after first value — proves we did NOT switch to Unmarshal).
  Use `httptest.NewRecorder` + `httptest.NewRequest`. RED first.

- [ ] **Step 2: Run test, verify it fails** (`go test ./internal/shared/ -run DecodeJSON`). Expected: undefined `DecodeJSON`.

- [ ] **Step 3: Implement `internal/shared/decode.go`:**

```go
package shared

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// DecodeMode controls tolerance for empty / malformed request bodies.
// A top-level literal `null` body is ALWAYS rejected (Node Express
// strict-body-parser parity), regardless of mode.
type DecodeMode int

const (
	// DecodeStrict: empty OR malformed body → 400. Mirrors a call site
	// that did `if err != nil { 400 }`.
	DecodeStrict DecodeMode = iota
	// DecodeOptional: empty body proceeds (zero value); malformed → 400.
	// Mirrors `if err != nil && err != io.EOF { 400 }`.
	DecodeOptional
	// DecodeLenient: empty OR malformed body proceeds. Mirrors `_ = Decode(...)`.
	DecodeLenient
)

// nullBodyMessage is the exact SyntaxError Express's strict body-parser
// surfaces for a top-level literal `null` body, mapped through Nest's
// AllExceptionsFilter to 400 invalid-input. Pinned to the running prod
// Node version — the live shadow gate (Node :3200) is the authority. If
// the gate reports a mismatch, replace this with Node's actual bytes.
const nullBodyMessage = `Unexpected token 'n', "null" is not valid JSON`

// DecodeJSON reads the request body and unmarshals JSON into dst. It
// intercepts a top-level literal `null` body — which Go's encoding/json
// silently no-ops into a struct pointer (no error), but Node rejects —
// and emits Node's 400. Every other path replays the original
// json.NewDecoder(r.Body).Decode(dst) byte-for-byte so empty / malformed
// / trailing-data behavior is unchanged; only mode decides how the
// replayed error is handled. Returns true to continue, false if an error
// response was already written.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, mode DecodeMode) bool {
	buf, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		if mode == DecodeLenient {
			return true
		}
		WriteError(w, r, "invalid-input", "Invalid request body")
		return false
	}

	// Top-level literal null: the one case Go's decoder accepts but Node
	// rejects. Whitespace-tolerant (Node trims), matches Node's strict mode.
	if string(bytes.TrimSpace(buf)) == "null" {
		WriteError(w, r, "invalid-input", nullBodyMessage)
		return false
	}

	// Replay the exact original decode (Decoder, NOT Unmarshal — Decoder
	// stops after the first JSON value, tolerating trailing data exactly
	// as the prior call sites did).
	err := json.NewDecoder(bytes.NewReader(buf)).Decode(dst)
	switch mode {
	case DecodeStrict:
		if err != nil {
			WriteError(w, r, "invalid-input", "Invalid request body")
			return false
		}
	case DecodeOptional:
		if err != nil && err != io.EOF {
			WriteError(w, r, "invalid-input", "Invalid request body")
			return false
		}
	case DecodeLenient:
		// errors ignored — proceed with whatever decoded (or zero value).
	}
	return true
}
```

- [ ] **Step 4: Run test, verify PASS.**
- [ ] **Step 5: Commit** — `git add internal/shared/decode.go internal/shared/decode_test.go` →
  `feat(shared): DecodeJSON rejects top-level null body (Node body-parser parity, #57)`

---

## Task 2: adopt DecodeJSON at all 21 sites + gate fixture

**Files:** the 13 handler files in the mode table above; new fixture
`cmd/shadowdiff/fixtures/auth/login_null_body.json`; touch any package tests
that assert the old null behavior.

- [ ] **Step 1:** For each site, replace the inline
  `json.NewDecoder(r.Body).Decode(&body)` + its `if err ...` block with a single
  `if !shared.DecodeJSON(w, r, &body, shared.<Mode>) { return }`, picking the
  mode from the table. For `auth/handler.go`, change the body of the local
  `decodeJSON` wrapper to delegate: `return shared.DecodeJSON(w, r, dst, shared.DecodeStrict)`
  (keeps all auth callers untouched). For the two Lenient `_ = Decode` sites,
  replace with `_ = shared.DecodeJSON(w, r, &body, shared.DecodeLenient)` (ignore
  the bool — null still 400s because DecodeJSON writes + returns false, but the
  caller must STOP on false; so use `if !shared.DecodeJSON(...) { return }` even
  for Lenient, since null now legitimately short-circuits). Remove now-unused
  `io` / `encoding/json` imports where they fall out.

- [ ] **Step 2:** Add gate fixture `cmd/shadowdiff/fixtures/auth/login_null_body.json`:

```json
{
  "name": "auth_login_null_body",
  "method": "POST",
  "path": "/auth/login",
  "headers": { "Content-Type": "application/json" },
  "raw_body": "null",
  "ignore_value_of": ["request_id"]
}
```

- [ ] **Step 3:** `go build ./...`, `go vet ./...`, `gofmt -l .` (empty),
  `go test ./... -race` (green). Fix any package test that hard-coded the old
  null→401 behavior (there should be none; null bodies aren't currently tested).

- [ ] **Step 4: Commit** — staged handler files + fixture →
  `fix(go-port): reject top-level null JSON body at all 21 decode sites (#57)`

---

## After both tasks

1. Full gate: `go build ./...` / `go vet ./...` / `gofmt -l .` / `go test ./... -race` / `sqlc-check.sh` (no SQL changed → trivially clean).
2. Live shadow gate on dev VPS (the authority for the null message). If
   `auth_login_null_body` FAILs on the message, capture Node's actual bytes and
   update `nullBodyMessage`, re-run.
3. `superpowers:finishing-a-development-branch`. Do NOT push/merge until the user
   authorizes (GitFlow + per-batch prod authorization).
