# Streaming `/v1/rewrite` training-data capture (#58) — Design

**Goal:** Make the Go-only streaming endpoint `POST /v1/rewrite` write a
`rewrite_logs` training-data row on clean stream finish, with the **real served
model + provider_type** of the winning failover-chain link — closing the
fine-tuning-dataset gap that opens when `flags.use_go_backend` routes macOS
rewrites to the streaming path.

**Decision:** Approach **B (full provenance)** — chosen over the minimal
empty-metadata insert. The route is Go-only (no Node parity authority), so the
only bar is *useful training data + don't crash + don't break the stream*. The
owner wants accurate `model`/`provider_type` for analytics + dataset quality.

---

## Background

- `#36` restored `rewrite_logs` capture on the **parity** paths (`POST /rewrite`,
  `POST /rewrite/trial`) via a fire-and-forget insert in
  `internal/rewrite/parity/service.go:284`. Those paths get `model` +
  `provider_type` + `response_time_ms` **synchronously** from the blocking
  `completer.Complete() → Completion`.
- The streaming use case `internal/rewrite/usecase/rewrite.go` fans out through
  the **failover `chain` adapter** (`internal/rewrite/adapter/chain`). The
  `domain.AiProvider` port exposes only `Name()` + `ID()` — there is **no
  per-request "which leaf actually served this stream"** signal. On clean finish
  it writes only `usage_logs` (`deps.Users.LogUsage`), never `rewrite_logs`.

## Constraint

The `domain.AiProvider.Stream(ctx, req) (<-chan string, <-chan error)` signature
is fixed and shared by every provider + the chain + the use case. We must surface
the winning leaf's provenance **without changing that signature** and **without
shared mutable state on the singleton chain** (concurrent requests would clobber).

## Approach B — per-request provenance carrier in `context.Context`

A new tiny package `internal/rewrite/provenance` holds a per-request carrier:

```go
package provenance

// Provenance is a per-request, mutation-safe carrier for the model +
// provider_type of the leaf provider that actually served the stream.
type Provenance struct {
    mu    sync.Mutex
    model string
    ptype string
}

type ctxKey struct{}

// NewContext attaches a fresh carrier and returns it alongside the derived ctx.
func NewContext(ctx context.Context) (context.Context, *Provenance)

// From returns the carrier on ctx, or nil if none is attached.
func From(ctx context.Context) *Provenance

// Stamp records the serving leaf's model + provider_type (last write wins).
func (p *Provenance) Stamp(model, ptype string)

// Read returns the stamped model + provider_type.
func (p *Provenance) Read() (model, ptype string)

// Stamp is the package-level convenience: looks up the carrier on ctx and
// stamps it if present; a no-op when no carrier is attached (parity path,
// tests with no carrier). Leaf adapters call this.
func Stamp(ctx context.Context, model, ptype string)
```

### Why this is correct under failover

Each leaf adapter calls `provenance.Stamp(ctx, c.model, c.Name())`
**synchronously at the top of its `Stream()`, before launching the streaming
goroutine.** The chain tries leaves **sequentially** (it calls `leaf2.Stream`
only after `leaf1` returned an error from `tryOne`). Sequential synchronous
stamps ⇒ the **last** stamp is always the committed (winning) leaf:

```
leaf1.Stream()  → Stamp(leaf1)  → fails before first token
leaf2.Stream()  → Stamp(leaf2)  → streams to completion
carrier.Read()  → (leaf2.model, "anthropic")   ✓ winner
```

Happy path (no failover): one leaf, one stamp, carrier = that leaf. The mutex
keeps `go test -race` clean even though the stamp write (chain/leaf goroutine)
and the `Read` (use-case goroutine after the token channel closes) are ordered by
the channel-close happens-before.

### Use-case wiring

In `usecase.Rewrite`, wrap the ctx once before streaming, read after clean close:

```go
ctx, prov := provenance.NewContext(ctx)
provTokens, provErrs := deps.Provider.Stream(ctx, req)
// ... clean-finish branch, AFTER the LogUsage success:
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

The insert fires **only on clean finish** — never on provider error, ctx cancel,
or client-gone (mirrors #36's "log only on success"). It reuses the existing
`elapsedMs` + `output` already accumulated for `usage_logs`.

### New use-case port (consumer-owned, nil-safe)

`usecase` declares its OWN small port + entry type (per CLAUDE.md: interfaces
belong to the consumer; mirrors how `parity` declares `rewriteLogger`):

```go
type RewriteLogEntry struct {
    Tone, InputText, OutputText, Model, ProviderType string
    ResponseTimeMs int64
}
type RewriteLogger interface { LogRewrite(ctx context.Context, e RewriteLogEntry) }
```

`RewriteDeps` gains `RewriteLog RewriteLogger` (nil = capture disabled; the
use case never panics on a nil sink — same guard style as `Metrics`/`Log`).

### Composition root

`cmd/server/main.go` adds a thin adapter `usecaseRewriteLogSink` mirroring the
existing `rewriteLogSink`: fire-and-forget, `context.WithoutCancel`, detached
goroutine, swallow+warn on error, mapping `usecase.RewriteLogEntry →
rewritelog.RewriteLogInput → repo.Insert`. Wired into `RewriteDeps.RewriteLog`
from the same `rewritelogpkg.NewPgRepo(q)` already built for the parity sink.

## Files

| File | Change |
|---|---|
| `internal/rewrite/provenance/provenance.go` | **Create** — carrier + ctx helpers |
| `internal/rewrite/provenance/provenance_test.go` | **Create** — unit + race tests |
| `internal/rewrite/adapter/openai/client.go` | Stamp at top of `Stream()` |
| `internal/rewrite/adapter/anthropic/client.go` | Stamp at top of `Stream()` |
| `internal/rewrite/adapter/ollama/client.go` | Stamp at top of `Stream()` |
| `internal/rewrite/adapter/memory/provider.go` | Stamp at top of `Stream()` (model "", type = Name) |
| `internal/rewrite/usecase/rewrite.go` | Wrap ctx, add port + `RewriteLogEntry`, log on clean finish |
| `internal/rewrite/usecase/rewrite_test.go` | Capture-fake assertions (logs on success only) |
| `internal/rewrite/adapter/chain/provider_test.go` | Failover stamps the winner |
| `cmd/server/main.go` | `usecaseRewriteLogSink` adapter + `RewriteDeps.RewriteLog` wiring |

## Scope / risk

- **Go-only route** → cannot affect the shadow gate (no Node counterpart). The
  full gate must still pass unchanged (148/148) — this adds capture without
  altering any parity response byte.
- Fire-and-forget, invisible HTTP side-effect → streamed response bytes
  unchanged; the stream is never blocked on the insert.
- Stamp = one mutex `Set` per `Stream` call; `Read` = one mutex `Get` after
  close. Negligible hot-path cost.
- `memory` stub stamps `model=""`, `provider_type="memory-stub"` — dev/test only.

## Out of scope (YAGNI)

- No change to `usage_logs` (already stamps `chain.ID()` as the logical provider).
- No schema migration — `rewrite_logs` columns already exist (#36).
- No new admin UI — the existing training-data page surfaces these rows already.
