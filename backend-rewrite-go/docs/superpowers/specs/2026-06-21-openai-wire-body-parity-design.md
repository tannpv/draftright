# #60 — OpenAI-wire request body parity (temperature / gpt-5 / ollama isLocal) — Design

**Goal:** Make Go's non-streaming `openai.Client.Complete` build the
chat-completions **request body byte-identically to Node's
`OpenAiStrategy.buildBody`** for every parity caller — closing the
pre-existing body gap that #34 deliberately left out of scope.

**Decision:** Mirror Node's self-contained strategy — thread `temperature`
and an `isLocal` flag into `openai.Client` via functional options; centralize
body assembly in a private `buildBody`. Narrow to the **non-streaming
`Complete`** path (the only Node-parity-bound one).

---

## Authority

Node `backend/src/ai-providers/strategies/openai.strategy.ts` `buildBody()`.
Go `internal/rewrite/adapter/openai/complete.go` `Complete()`.

`OpenAiStrategy.matches = openai || ollama || custom`; `.call` is
**non-streaming** (`fetch` + `json()`), reads `choices[0].message.content`.
Go's parity callers all route through `aiprovider.Factory.For → openai.New`:

- `internal/rewrite/parity/service.go:268` — `/rewrite` + `/rewrite/trial`
- `internal/aiprovider/usecase.go:127` — admin test-provider (POST `/admin/ai-providers/:id/test`)
- `internal/aiprovider/usecase.go:165` — admin configured rewrite

`Stream()` (Go-only streaming `/v1/rewrite`) has **no Node counterpart** → its
body is not parity-bound and is **out of scope**.

## The four body divergences (Complete only)

| # | Node `buildBody` | Go `Complete` now | Fix |
|---|---|---|---|
| 1 | no `stream` key | `"stream": false` | drop the key |
| 2 | non-gpt-5 → `temperature: Number(provider.temperature)` | absent | parse `Temperature` → number, emit |
| 3 | gpt-5 (`model.startsWith('gpt-5')`) → `reasoning_effort:'minimal'`, **no** temperature | absent | mirror |
| 4 | ollama (`type===OLLAMA`) → fixed assistant system message + systemPrompt folded into the **user** message | always `[{system},{user}]` | mirror the isLocal layout |

### Node message layouts (copy verbatim)

**isLocal (ollama) — system content:**
```
You are a writing assistant. If the user provides existing text, rewrite it as instructed. If the user provides a request or instruction, generate the requested content. You ONLY output the result text. Never answer questions about yourself, never explain your process, never add commentary. CRITICAL: You must reply in the SAME LANGUAGE as the input text. If the input is Vietnamese, reply in Vietnamese. If French, reply in French. Never switch to English unless the input is in English.
```

**isLocal — user content template** (`systemPrompt` = the resolved system arg,
`userText` = the user arg):
```
${systemPrompt}\n\nIMPORTANT: Your response MUST be in the same language as the text below.\n\nUser input:\n"${userText}"\n\nOutput:
```

**non-local:** `[{role:"system",content:system},{role:"user",content:user}]`.

### temperature semantics

Node `Number(provider.temperature)`; `provider.temperature` is a
`decimal(3,2)` string (e.g. `"0.30"`). `Number("0.30") === 0.3`;
`JSON.stringify(0.3) === "0.3"`. Go: `strconv.ParseFloat("0.30",64) → 0.3`;
`json.Marshal(0.3) → "0.3"`. Match. The column is `NOT NULL` with a numeric
default, so a parse failure is unreachable in practice; defensively treat a
ParseFloat error as `0` (closest to Node `Number("") === 0`).

### gpt-5 detection

Node `provider.model.startsWith('gpt-5')` → Go `strings.HasPrefix(c.model,
"gpt-5")`. gpt-5 omits `temperature` and adds `reasoning_effort: "minimal"`.

## Threading (the design choice)

Go's `openai.Client` carries only `id/apiKey/endpoint/model/http/systemFmt` —
not `temperature` or provider type. After #34 the factory collapsed
openai/ollama/custom into one `openai.New()`. Add two functional options,
mirroring Node's self-contained strategy:

```go
// WithTemperature stores the raw decimal(3,2) string (e.g. "0.30"); parsed
// at body-build time, mirroring Node's Number(provider.temperature).
func WithTemperature(t string) Option { return func(c *Client) { c.temperature = t } }

// WithLocalLayout selects the ollama isLocal message layout (Node
// type===OLLAMA): a fixed assistant system message + the system prompt folded
// into the user message.
func WithLocalLayout(local bool) Option { return func(c *Client) { c.isLocal = local } }
```

`Client` gains `temperature string` + `isLocal bool`. `Complete` delegates body
assembly to a new private `buildBody(system, user string) (map[string]any,
error)` mirroring Node's `buildBody`.

Factory (`completer_factory.go`, openai/ollama/custom case) appends:
```go
opts = append(opts, openai.WithTemperature(p.Temperature))
opts = append(opts, openai.WithLocalLayout(p.Type == "ollama"))
```

**Rejected alternatives:** (B) build the body at the factory/usecase layer and
pass a prepared struct into `Complete` — disrupts the `Complete(system, user)`
signature shared by all callers. (C) pass the full `aiprovider.AiProvider` into
the adapter — breaks layering (`rewrite/adapter/openai` would import
`internal/aiprovider`). (A) functional options is idiomatic, matches the
existing `WithModel/WithEndpoint` pattern, and keeps the adapter dependency-free.

## Untouched

- `Stream()` body (Go-only `/v1/rewrite`, no parity authority).
- Node 429 retry/backoff (`MAX_429_RETRIES`/`RETRY_BACKOFF_MS`) — pre-existing,
  independent absence in Go; filed/handled separately if ever needed (YAGNI).
- The conditional `Authorization` header (#34) and response parsing — unchanged.

## Files

| File | Change |
|---|---|
| `internal/rewrite/adapter/openai/client.go` | add `temperature`/`isLocal` fields + `WithTemperature`/`WithLocalLayout` options |
| `internal/rewrite/adapter/openai/complete.go` | `Complete` calls new private `buildBody`; drop `stream:false`; add temperature / gpt-5 / isLocal branches |
| `internal/rewrite/adapter/openai/complete_test.go` | httptest asserts exact POSTed JSON body for non-gpt-5, gpt-5, ollama-isLocal |
| `internal/aiprovider/completer_factory.go` | append `WithTemperature(p.Temperature)` + `WithLocalLayout(p.Type=="ollama")` |
| `internal/aiprovider/completer_factory_test.go` | assert the options reach the wire (temperature present, ollama → isLocal layout) |

## Verification

- New httptest unit tests assert the exact request body per case.
- `go build ./...` / `go vet ./...` / `gofmt -l .` clean; `go test ./... -race` green.
- **Not gate-coverable** — the live gate stubs the AI provider (memory), so the
  request body never reaches a real upstream. Verified by unit tests + reading
  Node. Full local + live shadow gate stays 148/148 (envelope unaffected — the
  body change is invisible to the stubbed provider).

## Deploy note

Body-only change on a stubbed-in-gate path → no Caddy-flip risk by itself.
Ships with the next go-port batch; live gate 148/148 confirmed before any flip.
