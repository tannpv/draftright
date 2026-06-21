# #34 — Ollama→OpenAI Wire Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route `ollama`-type DB providers through the OpenAI `/v1/chat/completions` wire in the completer factory (matching Node's `OpenAiStrategy`), and send the `Authorization` header only when an API key is present.

**Architecture:** Two surgical changes. (1) `aiprovider.Factory.For` merges the `ollama` case into the existing `openai`/`custom` OpenAI-wire case — dropping the native-Ollama adapter from the parity path. (2) The openai client sets `Authorization: Bearer …` only when `apiKey != ""` (in both `Complete()` and `Stream()`), mirroring Node's `if (provider.api_key)`. The native ollama adapter is untouched (still backs the Go-only `/v1/rewrite` env chain).

**Tech Stack:** Go 1.26, `net/http`, `net/http/httptest`, `testify/require`. Tests are pure unit (no live ollama, no DB).

Spec: `docs/superpowers/specs/2026-06-21-ollama-openai-wire-parity-design.md`

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `internal/aiprovider/completer_factory.go` | Map a provider row → live Completer | Merge `ollama` into the openai-wire case; drop native-ollama import + case |
| `internal/aiprovider/completer_factory_test.go` | Factory construction + wire assertions | Add a test proving `type=="ollama"` builds an OpenAI-wire Completer that POSTs OpenAI shape to `endpoint_url` |
| `internal/rewrite/adapter/openai/complete.go` | Blocking OpenAI call (`Complete`) | `Authorization` header only when `apiKey != ""` |
| `internal/rewrite/adapter/openai/client.go` | Streaming OpenAI call (`Stream`) | `Authorization` header only when `apiKey != ""` |
| `internal/rewrite/adapter/openai/complete_test.go` | `Complete` unit tests | Add empty-key → no-header test |
| `internal/rewrite/adapter/openai/client_test.go` | `Stream` unit tests | Add empty-key → no-header test |

---

## Task 1: Factory routes `ollama` through the OpenAI wire

**Files:**
- Modify: `internal/aiprovider/completer_factory.go:30-61`
- Test: `internal/aiprovider/completer_factory_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/aiprovider/completer_factory_test.go`. This proves an
`ollama`-type provider now produces a Completer that speaks the **OpenAI wire**
(POSTs to `endpoint_url`, sends `{model, messages:[{role:system},{role:user}]}`,
reads `choices[0].message.content`) — the byte-shape Node's `OpenAiStrategy`
uses — instead of the native Ollama `/api/chat` shape (`{messages}` →
`message.content`).

```go
func TestFactory_OllamaUsesOpenAIWire(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		// OpenAI response shape: choices[0].message.content (NOT message.content)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"  ok  "}}]}`))
	}))
	defer srv.Close()

	p := AiProvider{
		ID:          "11111111-1111-1111-1111-111111111111",
		Name:        "Local Ollama",
		Type:        "ollama",
		EndpointURL: srv.URL + "/v1/chat/completions",
		APIKey:      "", // ollama/local has no key
		Model:       "llama3.2",
	}
	c, err := Factory{}.For(p)
	require.NoError(t, err)
	require.NotNil(t, c)

	text, _, err := c.Complete(context.Background(), "sys", "user")
	require.NoError(t, err)
	require.Equal(t, "ok", text) // proves it parsed choices[0].message.content + trimmed

	require.Equal(t, "/v1/chat/completions", gotPath) // POSTed to endpoint_url, not /api/chat
	// OpenAI request shape: messages array with system+user roles
	msgs, ok := gotBody["messages"].([]any)
	require.True(t, ok, "body must carry a messages array")
	require.Len(t, msgs, 2)
	require.Equal(t, "system", msgs[0].(map[string]any)["role"])
	require.Equal(t, "user", msgs[1].(map[string]any)["role"])
	require.Equal(t, "llama3.2", gotBody["model"])
}
```

Ensure the test file imports are present (add any missing): `context`,
`encoding/json`, `io`, `net/http`, `net/http/httptest`, `testing`, and
`github.com/stretchr/testify/require`. The file currently uses only `testing`,
so add the rest:

```go
import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/aiprovider/ -run TestFactory_OllamaUsesOpenAIWire -v`
Expected: FAIL — the native ollama adapter posts to `/api/chat` (so `gotPath`
is `/v1/chat/completions` only if the URL had no path… it POSTs to the raw
endpoint, but reads `message.content`, so `text` is empty → `require.Equal "ok"`
fails). The assertion on response parsing (`choices[0].message.content`) fails
because the native adapter reads `message.content`.

- [ ] **Step 3: Write minimal implementation**

In `internal/aiprovider/completer_factory.go`, replace the three separate
`case "openai":`, `case "ollama":`, `case "custom":` blocks with a single
merged OpenAI-wire case. Final `For` body:

```go
// For maps a provider row to its OpenAI-compatible / Anthropic adapter.
func (Factory) For(p AiProvider) (Completer, error) {
	id, err := uuid.Parse(p.ID)
	if err != nil {
		id = uuid.New()
	}

	switch p.Type {
	case "openai", "ollama", "custom":
		// Node's OpenAiStrategy.matches = openai || ollama || custom: ONE
		// OpenAI-compatible wire (POST endpoint_url, choices[0].message.content)
		// serves all three. An ollama-type provider MUST carry an
		// OpenAI-compatible endpoint_url (Ollama's own /v1/chat/completions, or
		// Ollama Cloud) — exactly as Node requires, since Node always sends the
		// OpenAI shape (#34).
		opts := []openai.Option{openai.WithModel(p.Model)}
		if p.EndpointURL != "" {
			opts = append(opts, openai.WithEndpoint(p.EndpointURL))
		}
		return openai.New(id, p.APIKey, opts...), nil
	case "anthropic":
		opts := []anthropic.Option{anthropic.WithModel(p.Model)}
		if p.EndpointURL != "" {
			opts = append(opts, anthropic.WithEndpoint(p.EndpointURL))
		}
		return anthropic.New(id, p.APIKey, opts...), nil
	default:
		return nil, fmt.Errorf("No provider strategy registered for type %q (provider id=%s, name=%s)", p.Type, p.ID, p.Name)
	}
}
```

Then remove the now-unused native-ollama import. The import block becomes:

```go
import (
	"fmt"

	"github.com/google/uuid"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/anthropic"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/openai"
)
```

Also update the type doc-comment so it no longer implies a distinct ollama
adapter — change the `For maps …` / struct comment lines that reference the
native Ollama adapter to state ollama shares the OpenAI wire. (The existing
lines 16–18 already say this; just ensure no comment still says "ollama" maps
to a native adapter.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/aiprovider/ -run TestFactory_OllamaUsesOpenAIWire -v`
Expected: PASS.

Also run the whole package to confirm the existing `TestFactory_BuildsKnownTypes`
(which includes an `"ollama"` case) still passes — it asserts only non-nil
construction, which still holds:

Run: `go test ./internal/aiprovider/ -v`
Expected: PASS (all tests).

- [ ] **Step 5: Commit**

```bash
git add internal/aiprovider/completer_factory.go internal/aiprovider/completer_factory_test.go
git commit -m "fix(aiprovider): route ollama through the OpenAI wire in completer factory (#34 Node parity)"
```

---

## Task 2: OpenAI client sends Authorization only when a key is present

**Files:**
- Modify: `internal/rewrite/adapter/openai/complete.go:39`
- Modify: `internal/rewrite/adapter/openai/client.go:133`
- Test: `internal/rewrite/adapter/openai/complete_test.go`
- Test: `internal/rewrite/adapter/openai/client_test.go`

Node sets the header only when `provider.api_key` is truthy
(`backend/src/ai-providers/strategies/openai.strategy.ts:43-45`). An
ollama/local provider has no key, so Go must omit the header entirely — not
send `Authorization: Bearer ` (empty). Rule #1: same rule in both call paths.

- [ ] **Step 1: Write the failing test (Complete)**

Add to `internal/rewrite/adapter/openai/complete_test.go`:

```go
func TestClient_Complete_OmitsAuthWhenNoKey(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasAuth := r.Header["Authorization"]
		require.False(t, hasAuth, "no Authorization header when api key is empty")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "", openai.WithEndpoint(srv.URL))
	_, _, err := c.Complete(context.Background(), "sys", "user")
	require.NoError(t, err)
}
```

- [ ] **Step 2: Write the failing test (Stream)**

Add to `internal/rewrite/adapter/openai/client_test.go` (it already imports
`httptest`, `http`, `uuid`, `require` — reuse them; mirror the existing
streaming-test server shape, returning a minimal SSE `[DONE]`):

```go
func TestClient_Stream_OmitsAuthWhenNoKey(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasAuth := r.Header["Authorization"]
		require.False(t, hasAuth, "no Authorization header when api key is empty")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "", openai.WithEndpoint(srv.URL))
	tokens, errs := c.Stream(context.Background(), drReq(t))
	for range tokens {
	}
	// drain errs; a clean [DONE] stream yields no error
	for err := range errs {
		require.NoError(t, err)
	}
}
```

If `client_test.go` has no existing helper that builds a
`domain.RewriteRequest`, reuse whatever the existing streaming tests in that
file use to construct the request argument to `Stream` (match the existing
test's exact constructor call — do NOT invent a new one). Name the local helper
`drReq` only if no equivalent exists; otherwise inline the existing pattern.

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/rewrite/adapter/openai/ -run OmitsAuthWhenNoKey -v`
Expected: FAIL — both currently send `Authorization: Bearer ` (empty value), so
`r.Header["Authorization"]` is present → `require.False` fails.

- [ ] **Step 4: Write minimal implementation**

In `internal/rewrite/adapter/openai/complete.go`, replace line 39:

```go
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
```

with:

```go
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
```

In `internal/rewrite/adapter/openai/client.go`, replace line 133 (inside
`Stream`) identically:

```go
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/rewrite/adapter/openai/ -v`
Expected: PASS — the new empty-key tests pass AND the existing
`Bearer test-key` assertions (non-empty key) still pass.

- [ ] **Step 6: Commit**

```bash
git add internal/rewrite/adapter/openai/complete.go internal/rewrite/adapter/openai/client.go internal/rewrite/adapter/openai/complete_test.go internal/rewrite/adapter/openai/client_test.go
git commit -m "fix(openai): send Authorization header only when api key is set (Node parity, ollama/local has none)"
```

---

## Task 3: Full gate + parity safety check

**Files:** none (verification only)

- [ ] **Step 1: Build, vet, fmt**

Run:
```bash
go build ./...
go vet ./...
gofmt -l .
```
Expected: build OK, vet clean, `gofmt -l .` prints nothing.

- [ ] **Step 2: Full race suite**

Run: `go test ./... -race`
Expected: all packages `ok` / `no test files`. Pay attention to
`internal/aiprovider`, `internal/rewrite/...`, and the parity service package —
none should regress.

- [ ] **Step 3: Confirm the native ollama adapter is still referenced (not dead)**

Run: `grep -rn 'adapter/ollama' cmd/server/main.go`
Expected: still imported + used in the env-chain `case "ollama"` (the Go-only
`/v1/rewrite` streaming path). The native adapter must remain compiled and used
there — only the **factory** stopped using it. If this grep is empty, the
native adapter became dead code and the env-chain wiring was broken — STOP and
investigate.

- [ ] **Step 4: Confirm no remaining unconditional empty-Bearer in the openai adapter**

Run: `grep -n 'Authorization' internal/rewrite/adapter/openai/*.go`
Expected: both `complete.go` and `client.go` occurrences are now guarded by
`if c.apiKey != ""`. No bare `Set("Authorization", "Bearer "+c.apiKey)` outside
a key check.

- [ ] **Step 5: (Controller-run) live shadow gate**

This is run by the controller on the dev VPS, not the implementer subagent. The
full + live shadow gate must still report **148/148** — `/rewrite` envelope
parity is unchanged because the gate stubs the AI provider; this change only
alters the real upstream wire (uncovered by the gate). Record the result before
any merge.

---

## Self-Review notes

- **Spec coverage:** Change 1 (factory merge) → Task 1. Change 2 (conditional
  auth, both `Complete` + `Stream`) → Task 2. "Untouched native adapter" +
  "gate still 148/148" → Task 3 steps 3 & 5. Out-of-scope follow-up issue is
  filed by the controller after merge (not a code task).
- **No placeholders:** every code step shows the exact final code.
- **Type consistency:** `AiProvider`, `Factory{}.For`, `openai.New(id, apiKey,
  opts...)`, `openai.WithModel`, `openai.WithEndpoint`, `c.apiKey` all match the
  existing signatures read from the tree. `Complete(ctx, system, user) (string,
  int64, error)` matches the parity `Completer` port.
