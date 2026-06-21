# OpenAI-wire request body parity (#60) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `openai.Client.Complete` build the chat-completions request body byte-identically to Node `OpenAiStrategy.buildBody` — drop `stream:false`, add temperature (non-gpt-5), `reasoning_effort:"minimal"` (gpt-5), and the ollama `isLocal` message layout — threaded via two new functional options.

**Architecture:** Mirror Node's self-contained strategy. `openai.Client` gains `temperature string` + `isLocal bool` (set by `WithTemperature`/`WithLocalLayout`). A private `buildBody(system,user)` centralizes assembly. The factory passes the provider's temperature + `type=="ollama"`. Only the non-streaming `Complete` path (parity-bound) changes; `Stream` (Go-only) is untouched.

**Tech Stack:** Go 1.26, net/http, encoding/json, strconv, httptest, testify.

**Authority:** `backend/src/ai-providers/strategies/openai.strategy.ts` `buildBody()`. Design: `docs/superpowers/specs/2026-06-21-openai-wire-body-parity-design.md`.

---

### Task 1: Add `temperature` + `isLocal` options to the Client

**Files:**
- Modify: `internal/rewrite/adapter/openai/client.go`
- Test: `internal/rewrite/adapter/openai/client_test.go`

- [ ] **Step 1: Write the failing test**

Add to `client_test.go` (mirror the existing option tests in the file):

```go
func TestNew_TemperatureAndLocalLayoutOptions(t *testing.T) {
	t.Parallel()
	c := openai.New(uuid.New(), "k",
		openai.WithTemperature("0.30"),
		openai.WithLocalLayout(true),
	)
	require.NotNil(t, c)
}
```

- [ ] **Step 2: Run — expect FAIL (undefined options)**

Run: `go test ./internal/rewrite/adapter/openai/ -run TestNew_TemperatureAndLocalLayoutOptions`
Expected: build error — `undefined: openai.WithTemperature` / `openai.WithLocalLayout`.

- [ ] **Step 3: Add the fields + options**

In `client.go`, add two fields to `Client`:
```go
	temperature string // raw decimal(3,2) string, parsed at body-build (Node Number())
	isLocal     bool   // ollama type → isLocal message layout
```

Add two options near `WithModel`:
```go
// WithTemperature stores the raw decimal(3,2) string (e.g. "0.30"), parsed at
// body-build time to mirror Node's Number(provider.temperature).
func WithTemperature(t string) Option { return func(c *Client) { c.temperature = t } }

// WithLocalLayout selects the ollama isLocal message layout (Node
// type===OLLAMA): a fixed assistant system message + the system prompt folded
// into the user message.
func WithLocalLayout(local bool) Option { return func(c *Client) { c.isLocal = local } }
```

- [ ] **Step 4: Run — expect PASS**

Run: `go test ./internal/rewrite/adapter/openai/ -run TestNew_TemperatureAndLocalLayoutOptions`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/rewrite/adapter/openai/client.go internal/rewrite/adapter/openai/client_test.go
git add internal/rewrite/adapter/openai/client.go internal/rewrite/adapter/openai/client_test.go
git commit -m "feat(openai): add WithTemperature + WithLocalLayout options to Client

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: `Complete` builds the body via a Node-parity `buildBody`

**Files:**
- Modify: `internal/rewrite/adapter/openai/complete.go`
- Test: `internal/rewrite/adapter/openai/complete_test.go`

This is the core task. Read `complete.go` and the existing `complete_test.go` first to reuse the httptest pattern (the existing `TestClient_Complete_NonStreaming` / `TestClient_Complete_OmitsAuthWhenNoKey` capture the request).

- [ ] **Step 1: Write the failing tests**

Add three table-style httptest tests to `complete_test.go`. Each captures the POSTed JSON body into a `map[string]any` and asserts the exact shape. Reuse the existing server-capture idiom in the file (a handler that reads `r.Body`, stores it, writes a minimal `{"choices":[{"message":{"content":"x"}}]}` response).

```go
func TestComplete_BodyParity_NonGpt5(t *testing.T) {
	t.Parallel()
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "k",
		openai.WithEndpoint(srv.URL),
		openai.WithModel("gpt-4o-mini"),
		openai.WithTemperature("0.30"),
	)
	_, _, err := c.Complete(context.Background(), "SYS", "USER")
	require.NoError(t, err)

	// no stream key
	_, hasStream := got["stream"]
	require.False(t, hasStream, "Node buildBody omits the stream key")
	require.Equal(t, "gpt-4o-mini", got["model"])
	require.InDelta(t, 0.3, got["temperature"], 1e-9)
	_, hasReasoning := got["reasoning_effort"]
	require.False(t, hasReasoning)
	msgs := got["messages"].([]any)
	require.Len(t, msgs, 2)
	require.Equal(t, map[string]any{"role": "system", "content": "SYS"}, msgs[0])
	require.Equal(t, map[string]any{"role": "user", "content": "USER"}, msgs[1])
}

func TestComplete_BodyParity_Gpt5(t *testing.T) {
	t.Parallel()
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "k",
		openai.WithEndpoint(srv.URL),
		openai.WithModel("gpt-5-nano"),
		openai.WithTemperature("0.30"),
	)
	_, _, err := c.Complete(context.Background(), "SYS", "USER")
	require.NoError(t, err)

	require.Equal(t, "minimal", got["reasoning_effort"])
	_, hasTemp := got["temperature"]
	require.False(t, hasTemp, "gpt-5 omits temperature")
}

func TestComplete_BodyParity_OllamaIsLocal(t *testing.T) {
	t.Parallel()
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "",
		openai.WithEndpoint(srv.URL),
		openai.WithModel("llama3.2"),
		openai.WithTemperature("0.30"),
		openai.WithLocalLayout(true),
	)
	_, _, err := c.Complete(context.Background(), "SYS", "USER")
	require.NoError(t, err)

	msgs := got["messages"].([]any)
	require.Len(t, msgs, 2)
	sys := msgs[0].(map[string]any)
	require.Equal(t, "system", sys["role"])
	require.Contains(t, sys["content"], "You are a writing assistant.")
	require.Contains(t, sys["content"], "SAME LANGUAGE")
	usr := msgs[1].(map[string]any)
	require.Equal(t, "user", usr["role"])
	require.Equal(t,
		"SYS\n\nIMPORTANT: Your response MUST be in the same language as the text below.\n\nUser input:\n\"USER\"\n\nOutput:",
		usr["content"])
	// isLocal still carries temperature (llama3.2 is non-gpt-5)
	require.InDelta(t, 0.3, got["temperature"], 1e-9)
}
```

Ensure imports include `io`, `encoding/json`, `net/http`, `net/http/httptest`, `context`, `github.com/google/uuid`, `github.com/stretchr/testify/require`.

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./internal/rewrite/adapter/openai/ -run TestComplete_BodyParity`
Expected: FAIL — `stream` key present, no `temperature`/`reasoning_effort`, non-local messages for the ollama case.

- [ ] **Step 3: Implement `buildBody` + rewire `Complete`**

In `complete.go`, add the private method (mirror Node `buildBody` exactly):

```go
// localSystemPrompt is Node's fixed isLocal (ollama) system message —
// openai.strategy.ts buildBody. Kept byte-identical for wire parity.
const localSystemPrompt = "You are a writing assistant. If the user provides existing text, rewrite it as instructed. If the user provides a request or instruction, generate the requested content. You ONLY output the result text. Never answer questions about yourself, never explain your process, never add commentary. CRITICAL: You must reply in the SAME LANGUAGE as the input text. If the input is Vietnamese, reply in Vietnamese. If French, reply in French. Never switch to English unless the input is in English."

// buildBody assembles the chat-completions request body byte-identically to
// Node OpenAiStrategy.buildBody: isLocal (ollama) message layout, gpt-5
// reasoning_effort vs temperature, no `stream` key.
func (c *Client) buildBody(system, user string) map[string]any {
	var messages []map[string]string
	if c.isLocal {
		messages = []map[string]string{
			{"role": "system", "content": localSystemPrompt},
			{"role": "user", "content": system +
				"\n\nIMPORTANT: Your response MUST be in the same language as the text below.\n\nUser input:\n\"" +
				user + "\"\n\nOutput:"},
		}
	} else {
		messages = []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		}
	}

	body := map[string]any{
		"model":    c.model,
		"messages": messages,
	}
	if strings.HasPrefix(c.model, "gpt-5") {
		// gpt-5 family: temperature omitted (only default supported);
		// reasoning_effort=minimal disables hidden reasoning tokens.
		body["reasoning_effort"] = "minimal"
	} else {
		// Node: Number(provider.temperature). decimal(3,2) NOT NULL → always
		// numeric; ParseFloat failure is unreachable, defensively → 0
		// (matches Number("") === 0).
		temp, err := strconv.ParseFloat(c.temperature, 64)
		if err != nil {
			temp = 0
		}
		body["temperature"] = temp
	}
	return body
}
```

Replace the body construction in `Complete`:
```go
	body, err := json.Marshal(c.buildBody(system, user))
```
(Drop the inline `map[string]any{... "stream": false ...}`.)

Add `"strconv"` to the imports; keep `strings` (already imported).

- [ ] **Step 4: Run — expect PASS**

Run: `go test ./internal/rewrite/adapter/openai/ -run TestComplete_BodyParity -v`
Expected: all three PASS.

- [ ] **Step 5: Run the whole adapter package with race**

Run: `go test ./internal/rewrite/adapter/openai/ -race`
Expected: PASS — existing `Complete` tests (non-streaming, omit-auth) still green (they assert response parsing + headers, not the dropped `stream` key).

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/rewrite/adapter/openai/complete.go internal/rewrite/adapter/openai/complete_test.go
git add internal/rewrite/adapter/openai/complete.go internal/rewrite/adapter/openai/complete_test.go
git commit -m "fix(openai): build Complete body byte-identical to Node (temperature, gpt-5 reasoning_effort, ollama isLocal, no stream key)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Factory threads temperature + isLocal

**Files:**
- Modify: `internal/aiprovider/completer_factory.go`
- Test: `internal/aiprovider/completer_factory_test.go`

- [ ] **Step 1: Write the failing test**

Extend `completer_factory_test.go`. The existing `TestFactory_OllamaUsesOpenAIWire` already drives a wire round-trip; add assertions (or a sibling test) proving the factory-built Completer sends the temperature and, for an ollama row, the isLocal layout. Capture the POSTed body like Task 2.

```go
func TestFactory_PassesTemperatureAndLocalLayout(t *testing.T) {
	t.Parallel()
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	comp, err := aiprovider.Factory{}.For(aiprovider.AiProvider{
		ID:          uuid.New().String(),
		Type:        "ollama",
		Model:       "llama3.2",
		EndpointURL: srv.URL,
		Temperature: "0.30",
	})
	require.NoError(t, err)

	_, _, err = comp.Complete(context.Background(), "SYS", "USER")
	require.NoError(t, err)

	require.InDelta(t, 0.3, got["temperature"], 1e-9)
	msgs := got["messages"].([]any)
	require.Contains(t, msgs[0].(map[string]any)["content"], "You are a writing assistant.")
}
```

(Match the import list the test file already uses; add `io`/`encoding/json`/`net/http`/`net/http/httptest`/`context` if not present — Task 1 of #34 already added several.)

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./internal/aiprovider/ -run TestFactory_PassesTemperatureAndLocalLayout`
Expected: FAIL — temperature absent, messages in non-local layout (factory not yet passing the options).

- [ ] **Step 3: Implement**

In `completer_factory.go`, the openai/ollama/custom case, after the endpoint append:
```go
		opts = append(opts, openai.WithTemperature(p.Temperature))
		opts = append(opts, openai.WithLocalLayout(p.Type == "ollama"))
```

- [ ] **Step 4: Run — expect PASS**

Run: `go test ./internal/aiprovider/ -run TestFactory_PassesTemperatureAndLocalLayout -v`
Expected: PASS.

- [ ] **Step 5: Whole package + race**

Run: `go test ./internal/aiprovider/ -race`
Expected: PASS — existing factory tests unaffected.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/aiprovider/completer_factory.go internal/aiprovider/completer_factory_test.go
git add internal/aiprovider/completer_factory.go internal/aiprovider/completer_factory_test.go
git commit -m "fix(aiprovider): factory threads temperature + ollama isLocal layout to the openai wire (#60 Node parity)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Full gate + safety check (verification only)

**Files:** none (verification).

- [ ] **Step 1: Build / vet / fmt**

Run: `go build ./... && go vet ./... && gofmt -l .`
Expected: build+vet clean; `gofmt -l .` prints nothing.

- [ ] **Step 2: Full race suite**

Run: `go test ./... -race`
Expected: all packages PASS, no race.

- [ ] **Step 3: Confirm no `stream` key remains in `Complete`'s body**

Run: `grep -n '"stream"' internal/rewrite/adapter/openai/complete.go`
Expected: no match (Stream's `client.go` may still set stream:true — that's the Go-only path, fine).

- [ ] **Step 4: Confirm `Stream` body untouched**

Run: `git diff origin/develop...HEAD -- internal/rewrite/adapter/openai/client.go | grep -c '"stream"'`
Expected: `0` (Task 1 only added fields/options to client.go; no body change there).

- [ ] **Step 5: Note on the live shadow gate**

Not gate-coverable (AI provider stubbed). The live shadow gate must still read **148/148** before any prod Caddy flip — proving the HTTP envelope is unchanged. Controller-run at deploy time, not here.

---

## After all tasks

1. Dispatch a final whole-branch reviewer.
2. Invoke `superpowers:finishing-a-development-branch`.
3. Apply `status: developed` to #60 on merge-to-develop; do NOT close it.
4. Push/merge only when the user asks.
