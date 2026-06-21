# Streaming `/v1/rewrite` training-data capture (#58) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Write a `rewrite_logs` row on clean finish of the Go-only streaming
`POST /v1/rewrite`, carrying the real served model + provider_type of the winning
failover-chain leaf.

**Architecture:** Per-request `provenance.Provenance` carrier in `context.Context`;
each leaf adapter stamps its `(model, Name())` synchronously at the top of
`Stream()`; the use case wraps ctx, reads the carrier after a clean stream close,
and fires a nil-safe fire-and-forget `RewriteLogger.LogRewrite`.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5, sqlc, testify, `go test -race`.

**Design doc:** `docs/superpowers/specs/2026-06-21-streaming-rewritelog-design.md`

**Working dir for all commands:** `/opt/openAi/DraftRight/backend-rewrite-go`

---

### Task 1: `provenance` carrier package

**Files:**
- Create: `internal/rewrite/provenance/provenance.go`
- Test: `internal/rewrite/provenance/provenance_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package provenance_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/provenance"
)

func TestNewContext_AttachesReadableCarrier(t *testing.T) {
	ctx, p := provenance.NewContext(context.Background())
	require.NotNil(t, p)
	require.Same(t, p, provenance.From(ctx))
	m, ty := p.Read()
	require.Equal(t, "", m)
	require.Equal(t, "", ty)
}

func TestFrom_NoCarrier_ReturnsNil(t *testing.T) {
	require.Nil(t, provenance.From(context.Background()))
}

func TestStamp_LastWriteWins(t *testing.T) {
	ctx, p := provenance.NewContext(context.Background())
	provenance.Stamp(ctx, "gpt-4o-mini", "openai")
	provenance.Stamp(ctx, "claude-3-5-sonnet-20241022", "anthropic")
	m, ty := p.Read()
	require.Equal(t, "claude-3-5-sonnet-20241022", m)
	require.Equal(t, "anthropic", ty)
}

func TestPackageStamp_NoCarrier_NoPanic(t *testing.T) {
	require.NotPanics(t, func() {
		provenance.Stamp(context.Background(), "m", "t")
	})
}

func TestStamp_RaceClean(t *testing.T) {
	ctx, p := provenance.NewContext(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); provenance.Stamp(ctx, "m", "t") }()
	}
	wg.Wait()
	m, ty := p.Read()
	require.Equal(t, "m", m)
	require.Equal(t, "t", ty)
}
```

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/rewrite/provenance/ -race` → FAIL (package does not compile / not found).

- [ ] **Step 3: Write the implementation**

```go
// Package provenance carries, per request, the model + provider_type of the
// leaf AI provider that actually served a streamed rewrite. The streaming use
// case fans out through the failover chain adapter, whose domain.AiProvider
// port exposes only Name()+ID() — there is no return-value channel for "which
// leaf won". A per-request carrier on context.Context surfaces it without
// changing the Stream signature and without shared state on the singleton chain.
package provenance

import (
	"context"
	"sync"
)

// Provenance is a per-request, mutation-safe carrier. Stamped by the serving
// leaf adapter; read by the use case after the stream closes cleanly.
type Provenance struct {
	mu    sync.Mutex
	model string
	ptype string
}

type ctxKey struct{}

// NewContext attaches a fresh carrier to ctx and returns both. Call once per
// request, before invoking the provider's Stream.
func NewContext(ctx context.Context) (context.Context, *Provenance) {
	p := &Provenance{}
	return context.WithValue(ctx, ctxKey{}, p), p
}

// From returns the carrier attached to ctx, or nil if none.
func From(ctx context.Context) *Provenance {
	p, _ := ctx.Value(ctxKey{}).(*Provenance)
	return p
}

// Set records the serving leaf's model + provider_type (last write wins).
func (p *Provenance) Set(model, ptype string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.model = model
	p.ptype = ptype
}

// Read returns the stamped model + provider_type.
func (p *Provenance) Read() (model, ptype string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.model, p.ptype
}

// Stamp is the leaf-side convenience: stamps the carrier on ctx if one is
// attached, else a no-op (parity path / tests with no carrier).
func Stamp(ctx context.Context, model, ptype string) {
	if p := From(ctx); p != nil {
		p.Set(model, ptype)
	}
}
```

> Note: the test uses `provenance.Stamp` (package fn) + `p.Read()`. The method is
> named `Set` internally; the test does not call `p.Set` directly — keep `Set`
> exported so leaf tests can construct a carrier and assert. (Test above only
> uses `Stamp` + `Read`; do not rename.)

- [ ] **Step 4: Run to verify pass** — `go test ./internal/rewrite/provenance/ -race -v` → all PASS.

- [ ] **Step 5: Commit** — `git add internal/rewrite/provenance && git commit` (message below).

```
feat(rewrite): per-request provenance carrier for served model/provider_type
```

---

### Task 2: openai leaf stamps provenance in `Stream()`

**Files:**
- Modify: `internal/rewrite/adapter/openai/client.go` (top of `Stream`)
- Test: `internal/rewrite/adapter/openai/client_test.go` (add one test)

- [ ] **Step 1: Write the failing test.** Add a test that calls `c.Stream(ctx, req)`
  with a `provenance.NewContext` ctx, drains the channels, then asserts
  `p.Read()` returns `("gpt-4o-mini", "openai")`. Use the existing test harness
  in the file (an httptest server returning a minimal SSE body); if the file
  already has a streaming success test, model the new one on it. Construct the
  client with the default model. Example assertion core:

```go
ctx, p := provenance.NewContext(context.Background())
tokens, errs := c.Stream(ctx, mustReq(t))
drain(t, tokens, errs) // reuse/define a local drain
m, ty := p.Read()
require.Equal(t, "openai", ty)
require.Equal(t, "gpt-4o-mini", m)
```

- [ ] **Step 2: Run → FAIL** (`ty`/`m` empty). `go test ./internal/rewrite/adapter/openai/ -race -run Provenance -v`.

- [ ] **Step 3: Implement.** At the very top of `func (c *Client) Stream(...)`, BEFORE
  allocating channels / launching the goroutine, add:

```go
	provenance.Stamp(ctx, c.model, c.Name())
```

  Add the import `"github.com/tannpv/draftright-rewrite/internal/rewrite/provenance"`.

- [ ] **Step 4: Run → PASS.** `go test ./internal/rewrite/adapter/openai/ -race -v`.

- [ ] **Step 5: Commit** — `feat(rewrite/openai): stamp served model/provider_type on Stream`.

---

### Task 3: anthropic leaf stamps provenance in `Stream()`

**Files:**
- Modify: `internal/rewrite/adapter/anthropic/client.go`
- Test: `internal/rewrite/adapter/anthropic/client_test.go`

- [ ] **Step 1: Write the failing test** — same shape as Task 2, expecting
  `("claude-3-5-sonnet-20241022", "anthropic")`.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** — `provenance.Stamp(ctx, c.model, c.Name())` at top of `Stream`, add import.
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** — `feat(rewrite/anthropic): stamp served model/provider_type on Stream`.

---

### Task 4: ollama leaf stamps provenance in `Stream()`

**Files:**
- Modify: `internal/rewrite/adapter/ollama/client.go`
- Test: `internal/rewrite/adapter/ollama/client_test.go`

- [ ] **Step 1: Write the failing test** — same shape, expecting `("llama3.2", "ollama")`.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** — `provenance.Stamp(ctx, c.model, c.Name())` at top of `Stream`, add import.
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** — `feat(rewrite/ollama): stamp served model/provider_type on Stream`.

---

### Task 5: memory stub stamps provenance + chain failover stamps the winner

**Files:**
- Modify: `internal/rewrite/adapter/memory/provider.go`
- Test: `internal/rewrite/adapter/chain/provider_test.go` (add failover-winner test)

- [ ] **Step 1: Write the failing test** in `provider_test.go`:

```go
func TestChain_StampsWinningProviderProvenance(t *testing.T) {
	t.Parallel()
	primary := memory.NewProvider("openai", nil).
		WithModel("gpt-4o-mini").
		WithFinalError(errors.New("primary down"))
	fallback := memory.NewProvider("anthropic", []string{"rescue"}).
		WithModel("claude-3-5-sonnet-20241022")
	c := chain.New("test", []domain.AiProvider{primary, fallback})

	ctx, p := provenance.NewContext(context.Background())
	tokens, errs := c.Stream(ctx, mustReq(t))
	got, err := drain(t, tokens, errs)
	require.NoError(t, err)
	require.Equal(t, []string{"rescue"}, got)
	m, ty := p.Read()
	require.Equal(t, "claude-3-5-sonnet-20241022", m) // winner, not primary
	require.Equal(t, "anthropic", ty)
}
```

  This requires `memory.Provider` to (a) stamp in `Stream` and (b) carry a model
  via a new `WithModel` option.

- [ ] **Step 2: Run → FAIL** (`WithModel` undefined / provenance empty).

- [ ] **Step 3: Implement.** In `internal/rewrite/adapter/memory/provider.go`:
  - add a `model string` field + `WithModel(m string) *Provider` chainable option
    (match the existing `WithFinalError` builder style on `*Provider`);
  - at the top of `Stream`, add `provenance.Stamp(ctx, p.model, p.Name())` BEFORE
    launching the goroutine; add the provenance import.

- [ ] **Step 4: Run → PASS.** `go test ./internal/rewrite/adapter/chain/ ./internal/rewrite/adapter/memory/ -race -v`.

- [ ] **Step 5: Commit** — `feat(rewrite/memory): WithModel + stamp provenance; test chain stamps winner`.

---

### Task 6: use case writes `rewrite_logs` on clean finish

**Files:**
- Modify: `internal/rewrite/usecase/rewrite.go`
- Test: `internal/rewrite/usecase/rewrite_test.go`

- [ ] **Step 1: Write the failing tests.** Add a fake logger + a provider that
  stamps provenance, then assert:
  1. clean finish → exactly one `LogRewrite` with
     `{Tone:"polished", InputText:<req text>, OutputText:<joined tokens>,
       Model:"m", ProviderType:"t", ResponseTimeMs:>=0}`;
  2. provider error mid-stream → NO `LogRewrite`;
  3. ctx cancel → NO `LogRewrite`;
  4. nil `RewriteLog` dep → no panic, clean finish still streams.

```go
type captureLogger struct {
	mu      sync.Mutex
	entries []usecase.RewriteLogEntry
}
func (c *captureLogger) LogRewrite(_ context.Context, e usecase.RewriteLogEntry) {
	c.mu.Lock(); defer c.mu.Unlock(); c.entries = append(c.entries, e)
}
```

  Use the existing usecase test harness (fake UserRepo, fake RateLimiter, a
  provider fake). For the provider fake, stamp provenance at the top of its
  `Stream` (`provenance.Stamp(ctx, "m", "t")`) so the carrier is populated. Pin
  `Now` for a deterministic `ResponseTimeMs`. Because `LogRewrite` fires on a
  detached goroutine in production but the fake is synchronous, assert directly
  after draining; if the production sink is async, the in-use-case call is
  synchronous (the use case calls `deps.RewriteLog.LogRewrite` directly — the
  goroutine detach lives in the main.go sink, not the use case), so the capture
  is synchronous in the test. Guard reads with the mutex.

- [ ] **Step 2: Run → FAIL** (`RewriteLogEntry`/`RewriteLogger` undefined; no log emitted).

- [ ] **Step 3: Implement** in `rewrite.go`:
  - add the port + entry type near `RewriteDeps`:

```go
// RewriteLogEntry is the training-data row captured on a clean streamed finish.
type RewriteLogEntry struct {
	Tone, InputText, OutputText, Model, ProviderType string
	ResponseTimeMs int64
}

// RewriteLogger is the consumer-side fire-and-forget training-data sink. nil
// disables capture (the use case never panics on a nil sink).
type RewriteLogger interface {
	LogRewrite(ctx context.Context, e RewriteLogEntry)
}
```

  - add `RewriteLog RewriteLogger` to `RewriteDeps`;
  - in `Rewrite`, replace `provTokens, provErrs := deps.Provider.Stream(ctx, req)`
    with:

```go
	ctx, prov := provenance.NewContext(ctx)
	provTokens, provErrs := deps.Provider.Stream(ctx, req)
```

  - in the clean-finish branch, AFTER the existing `LogUsage` block and BEFORE
    `recordOutcome(domain.OutcomeOK)`, add:

```go
		if deps.RewriteLog != nil {
			model, ptype := prov.Read()
			deps.RewriteLog.LogRewrite(ctx, RewriteLogEntry{
				Tone:           req.Tone().String(),
				InputText:      req.Text(),
				OutputText:     output.String(),
				Model:          model,
				ProviderType:   ptype,
				ResponseTimeMs: elapsedMs,
			})
		}
```

  - add the provenance import. `elapsedMs` is already computed in that branch.

- [ ] **Step 4: Run → PASS.** `go test ./internal/rewrite/usecase/ -race -v`.

- [ ] **Step 5: Commit** — `feat(rewrite): capture rewrite_logs on clean streamed finish (#58)`.

---

### Task 7: wire the sink in the composition root

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Implement** (no unit test — composition root; covered by build +
  the gate). Add a thin adapter mirroring `rewriteLogSink` (around line 1041):

```go
// usecaseRewriteLogSink adapts the rewritelog repo to usecase.RewriteLogger for
// the streaming /v1/rewrite path. Same fire-and-forget contract as
// rewriteLogSink: detach onto a cancellation-free ctx, swallow + warn on error.
type usecaseRewriteLogSink struct {
	repo *rewritelogpkg.PgRepo
	log  *slog.Logger
}

func (s usecaseRewriteLogSink) LogRewrite(ctx context.Context, e usecase.RewriteLogEntry) {
	ctx = context.WithoutCancel(ctx)
	go func() {
		err := s.repo.Insert(ctx, rewritelogpkg.RewriteLogInput{
			Tone:           e.Tone,
			InputText:      e.InputText,
			OutputText:     e.OutputText,
			Model:          e.Model,
			ProviderType:   e.ProviderType,
			ResponseTimeMs: e.ResponseTimeMs,
		})
		if err != nil {
			s.log.Warn("rewrite_logs insert failed (streaming)", "err", err)
		}
	}()
}
```

  Then in the `RewriteDeps` literal (around line 824), add the field:

```go
		RewriteLog: usecaseRewriteLogSink{repo: rewritelogpkg.NewPgRepo(q), log: log},
```

  Reuse the existing `q` (sqlc queries) in scope. Only attach it when the DB/auth
  path is live (the same block that builds the parity `rewriteLogSink`); if
  `RewriteDeps` is built once unconditionally, guard with the same condition that
  guards the parity sink (`WithRewriteLog(...)` at ~line 486) so a no-DB dev run
  leaves `RewriteLog` nil. Match the surrounding wiring exactly.

- [ ] **Step 2: Build + vet + fmt.**

```
go build ./... && go vet ./... && gofmt -l . && go test ./... -race
```

  Expected: build clean, vet clean, `gofmt -l` prints nothing, all tests PASS.

- [ ] **Step 3: Commit** — `feat(server): wire streaming rewrite_logs sink into RewriteDeps (#58)`.

---

## After all tasks

1. Full local gate: `go build ./...`, `go vet ./...`, `gofmt -l .` (empty),
   `go test ./... -race` (green), `sqlc generate` (no drift — no SQL changed, so
   expect none).
2. Run the **live shadow gate** on the dev VPS — must stay **148/148** (this is a
   Go-only addition; no parity byte may change). The `57P01` per-fixture DB-reset
   flake is transient; re-run once if it appears.
3. `superpowers:finishing-a-development-branch`. Branch NOT pushed/merged until
   the user authorizes (GitFlow: feature → develop → main).

## Verification

- `go test ./internal/rewrite/... -race -v` → all PASS incl. provenance, leaf
  stamps, chain-winner, use-case capture-only-on-success.
- Manual: with `AI_PROVIDERS` set + DB up, a `POST /v1/rewrite` that streams to
  completion inserts one `rewrite_logs` row whose `model`/`provider_type` match
  the serving leaf; an aborted/errored stream inserts nothing.
