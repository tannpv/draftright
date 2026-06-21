# #34 — Completer factory: route `ollama` through the OpenAI wire (parity) — Design

**Goal:** Make Go's parity `/rewrite` + `/rewrite/trial` build the live AI call
for an `ollama`-type DB provider **byte-identically to Node** — i.e. through the
OpenAI-compatible `/v1/chat/completions` wire POSTed to the provider's
`endpoint_url` — instead of Go's current native Ollama `/api/chat` wire.

**Decision:** Narrow, issue-faithful fix (the wire routing). The broader
`buildBody` quirks Go omits for *every* OpenAI-wire provider (temperature /
gpt-5 `reasoning_effort`, and the ollama `isLocal` message layout) are a
**separate, pre-existing parity gap** filed as its own issue — NOT folded into
#34.

---

## Background — the divergence

Node has **no Ollama strategy**. `OpenAiStrategy.matches` returns true for
`OPENAI || OLLAMA || CUSTOM` (`backend/src/ai-providers/strategies/openai.strategy.ts:27`):
all three speak one OpenAI-compatible wire — `POST <endpoint_url>` with
`{model, messages, …}`, response read from `choices[0].message.content`,
`Authorization: Bearer <api_key>` **only when `api_key` is set**.

Go's `aiprovider.Factory.For` (`internal/aiprovider/completer_factory.go:52`)
instead routes `case "ollama"` to the **native Ollama adapter**
(`internal/rewrite/adapter/ollama`): `POST /api/chat` (default), request
`{model, stream, messages}`, response read from `message.content`, **no auth
header**. Different endpoint, different response JSON shape.

The factory's own doc-comment (lines 16–18) already states the intended
behavior — "one OpenAI-compatible wire serves openai, ollama, AND custom" — so
the `case "ollama"` is a **code/comment contradiction**. This fix makes the
code match its documented intent + the Node authority.

### Why the shadow gate can't catch it

The live gate stubs the AI provider for determinism, so it never exercises the
real upstream wire. `/rewrite`'s **HTTP envelope** parity is already green
(148/148) and stays green. #34 is verified by **reading Node + Go unit tests**,
not the gate. (Memory: "#34 not gate-coverable.")

## Scope — what #34 changes

### Change 1 — factory: merge `ollama` into the OpenAI-wire case

`internal/aiprovider/completer_factory.go`:

```go
case "openai", "ollama", "custom":
	// Node's OpenAiStrategy.matches = openai || ollama || custom: one
	// OpenAI-compatible wire (POST endpoint_url, choices[0].message.content)
	// serves all three. ollama-type providers MUST carry an OpenAI-compatible
	// endpoint_url (e.g. Ollama's own /v1/chat/completions, or Ollama Cloud) —
	// exactly as Node requires, since Node always sends the OpenAI shape.
	opts := []openai.Option{openai.WithModel(p.Model)}
	if p.EndpointURL != "" {
		opts = append(opts, openai.WithEndpoint(p.EndpointURL))
	}
	return openai.New(id, p.APIKey, opts...), nil
```

The separate `case "openai":` and `case "custom":` blocks collapse into this
one. The native-Ollama `import` + `case "ollama"` block are removed from the
factory. (The native adapter package stays — see "Untouched" below.)

### Change 2 — openai client: conditional `Authorization` header

`internal/rewrite/adapter/openai/client.go` `Complete()` currently always sets
`Authorization: Bearer <key>`. Node sets it **only when `api_key` is truthy**
(`openai.strategy.ts:44`). An ollama/local provider has an empty key, so Go
must omit the header entirely when `c.apiKey == ""`:

```go
if c.apiKey != "" {
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
}
```

(Also correct for any keyless `custom` endpoint. The streaming `Stream()` path
already special-cases empty keys — confirm + align if it doesn't.)

## Out of scope → file a NEW issue (broader OpenAI-wire body parity)

Go's `Complete()` builds `[{system}, {user}]` verbatim and sends **no**
`temperature` / `reasoning_effort`, for **every** provider type. Node's
`buildBody` does three things Go omits, none of them ollama-specific in their
machinery:

1. **temperature** — non-gpt-5 sends `temperature: Number(provider.temperature)`.
2. **gpt-5 family** — omits temperature, adds `reasoning_effort: 'minimal'`.
3. **ollama `isLocal` message layout** — a fixed writing-assistant system
   message + the tone prompt folded into the user message
   (`openai.strategy.ts:84-104`).

Items 1–2 affect openai/custom too → this is a wider parity gap than #34's
"wire routing" title. Filing as a separate issue keeps #34 low-risk and
correctly scoped (YAGNI). The new issue will also note item 3 (the ollama
message layout) so ollama parity is fully tracked.

## Untouched

- `internal/rewrite/adapter/ollama` (native `/api/chat`) stays — it still backs
  the **env-configured chain** for the Go-only streaming `/v1/rewrite`
  (`cmd/server/main.go:1208`), which has no Node parity authority.
- The parity prompt resolver (`ResolvePrompt`) and `callAI` flow are unchanged.

## Files

| File | Change |
|---|---|
| `internal/aiprovider/completer_factory.go` | Merge `ollama` into the openai-wire case; drop native-ollama import + case |
| `internal/aiprovider/completer_factory_test.go` | "ollama" → openai-wire Completer: httptest asserts POST to endpoint_url, OpenAI request shape, parses `choices[0].message.content` |
| `internal/rewrite/adapter/openai/client.go` | `Complete()` sets `Authorization` only when `apiKey != ""` |
| `internal/rewrite/adapter/openai/*_test.go` | empty-key → no Authorization header; non-empty → `Bearer <key>` |

## Verification

- `go build ./...` / `go vet ./...` / `gofmt -l .` clean; `go test ./... -race` green.
- New unit tests above (httptest, no live ollama).
- **Full local + live shadow gate still 148/148** — proves the HTTP envelope
  for `/rewrite` is unchanged (AI stubbed), so no parity regression.

## Deploy note

Any ollama-type DB provider row must carry an OpenAI-compatible `endpoint_url`
(it already must, for Node to serve it). No seeded ollama provider exists
(`backend/src/seed.ts` seeds only an OpenAI provider), so default deployments
are unaffected. Before a prod Caddy flip, confirm any admin-created ollama row's
`endpoint_url` is OpenAI-shaped (verifying parity with the already-running Node).
