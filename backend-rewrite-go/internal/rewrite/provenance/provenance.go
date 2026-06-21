// Package provenance carries, per request, which leaf AI provider actually
// served a streaming rewrite (its model and provider_type).
//
// Why a context carrier rather than a return value or struct field?
// The streaming rewrite fans out through a failover chain of providers. Each
// link implements the fixed domain.AiProvider.Stream signature, which cannot
// grow an out-parameter for provenance without breaking every implementation,
// and the chain itself is wired as a singleton (shared across all requests),
// so it cannot hold per-request mutable state on its own fields. The only
// per-request channel available to every link is the context. So each leaf
// stamps the model/provider_type it served onto a carrier riding on ctx; the
// use case reads it back after the stream completes.
//
// The carrier is mutex-guarded so concurrent stamps (e.g. racing goroutines in
// a fan-out) are data-race-free, and follows last-write-wins semantics: the
// final leaf to serve is the one whose identity survives.
package provenance

import (
	"context"
	"sync"
)

// ctxKey is the unexported context key under which the *Provenance carrier is
// stored, so no other package can collide with or overwrite it.
type ctxKey struct{}

// Provenance is the per-request carrier holding the served model and
// provider_type. Its zero-ish state (empty strings) means "nothing served yet".
type Provenance struct {
	mu    sync.Mutex
	model string
	ptype string
}

// NewContext attaches a fresh Provenance carrier to ctx and returns both the
// derived context and the carrier. The carrier is safe for concurrent use.
func NewContext(ctx context.Context) (context.Context, *Provenance) {
	p := &Provenance{}
	return context.WithValue(ctx, ctxKey{}, p), p
}

// From returns the Provenance carrier attached to ctx, or nil if none is
// present.
func From(ctx context.Context) *Provenance {
	p, _ := ctx.Value(ctxKey{}).(*Provenance)
	return p
}

// Set records the model and provider_type served. Last write wins. Safe for
// concurrent use.
func (p *Provenance) Set(model, ptype string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.model = model
	p.ptype = ptype
}

// Read returns the currently recorded model and provider_type. Safe for
// concurrent use.
func (p *Provenance) Read() (model, ptype string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.model, p.ptype
}

// Stamp records model/provider_type onto the carrier riding on ctx. It is a
// no-op when no carrier is attached, so leaf providers can stamp
// unconditionally without first checking for a carrier.
func Stamp(ctx context.Context, model, ptype string) {
	if p := From(ctx); p != nil {
		p.Set(model, ptype)
	}
}
