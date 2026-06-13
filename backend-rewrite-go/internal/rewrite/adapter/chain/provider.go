// Package chain composes multiple domain.AiProvider instances into a
// failover pipeline. Try providers in order; only fall through to the
// next when the current one fails BEFORE sending the first token to
// the caller. Once any bytes have streamed, an error is fatal — we
// can't switch mid-stream without producing garbled output.
//
// Same port (domain.AiProvider) — the use case + handler don't know
// they're talking to a chain. That's the value: failover is a
// transport concern, not a domain concern (Rule #1 — composition,
// not inheritance; the chain is one more adapter, not a special case).
package chain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
)

// Provider is the failover wrapper.
type Provider struct {
	id        uuid.UUID
	name      string
	providers []domain.AiProvider
	log       *slog.Logger
}

// New builds a chain. ids[0] = primary, ids[1+] = fallbacks. Empty
// list is a programmer error (caller's composeDeps validates).
func New(name string, providers []domain.AiProvider, opts ...Option) *Provider {
	p := &Provider{
		id:        uuid.New(),
		name:      name,
		providers: providers,
		log:       slog.Default(),
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Option configures the Provider.
type Option func(*Provider)

// WithLogger plumbs the request-scoped logger so failover events land
// in the same log line stream as the rest of the request.
func WithLogger(l *slog.Logger) Option { return func(p *Provider) { p.log = l } }

// WithID pins the chain's own UUID — useful when a test wants to
// assert the ai_provider_id stamped on the usage log row.
func WithID(id uuid.UUID) Option { return func(p *Provider) { p.id = id } }

// ID returns the chain's UUID. The active provider that actually
// produced the stream is logged separately; the chain UUID is what
// lands in usage_logs.ai_provider_id so analytics treats failover as
// one "logical provider" (the operator's choice).
func (p *Provider) ID() uuid.UUID { return p.id }

// Name identifies the chain (e.g. "chain:openai>anthropic>ollama").
func (p *Provider) Name() string { return p.name }

// Stream tries each provider in order. Failover semantics:
//
//   - Provider errors BEFORE first token  → log, try next.
//   - Provider errors AFTER first token   → fatal, surface to caller.
//   - Provider closes cleanly (with or without tokens) → success.
//   - Context canceled at any point        → propagate ctx.Err().
//   - All providers exhausted             → ErrProviderUnavailable
//                                            wrapping the last error.
//
// Buffer trade-off: tokens channel is unbuffered so back-pressure
// from a slow client propagates to the upstream provider's goroutine,
// preventing unbounded RAM growth.
func (p *Provider) Stream(ctx context.Context, req domain.RewriteRequest) (<-chan string, <-chan error) {
	out := make(chan string)
	errs := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errs)

		if len(p.providers) == 0 {
			errs <- domain.ErrProviderUnavailable
			return
		}

		var lastErr error
		for idx, prov := range p.providers {
			committed, err := p.tryOne(ctx, prov, req, out)
			if err == nil {
				// Clean close.
				return
			}
			if ctx.Err() != nil {
				errs <- ctx.Err()
				return
			}
			if committed {
				// Bytes already streamed; can't retry without
				// corrupting the output.
				errs <- err
				return
			}
			p.log.Warn("provider failed before first token; failing over",
				"chain", p.name,
				"failed_provider", prov.Name(),
				"failed_index", idx,
				"remaining", len(p.providers)-idx-1,
				"err", err.Error())
			lastErr = err
		}

		if lastErr == nil {
			lastErr = domain.ErrProviderUnavailable
		}
		errs <- fmt.Errorf("%w: all providers failed: %v", domain.ErrProviderUnavailable, lastErr)
	}()

	return out, errs
}

// tryOne pumps one provider's output into the shared `out` channel.
// Returns (committed=true) once any token has been forwarded so the
// caller knows whether failover is still safe.
//
// On clean close (provider closes tokens with no terminal error)
// returns (committed, nil). On any error returns (committed, err).
func (p *Provider) tryOne(
	ctx context.Context,
	prov domain.AiProvider,
	req domain.RewriteRequest,
	out chan<- string,
) (bool, error) {
	ptoks, perrs := prov.Stream(ctx, req)
	committed := false

	for ptoks != nil || perrs != nil {
		select {
		case <-ctx.Done():
			// Drain provider channels so its goroutine exits cleanly.
			go drain(ptoks, perrs)
			return committed, ctx.Err()

		case tok, open := <-ptoks:
			if !open {
				ptoks = nil
				continue
			}
			committed = true
			select {
			case <-ctx.Done():
				go drain(nil, perrs)
				return committed, ctx.Err()
			case out <- tok:
			}

		case e, open := <-perrs:
			if !open {
				perrs = nil
				continue
			}
			if e != nil {
				// Drain any in-flight tokens to unblock the provider.
				go drain(ptoks, nil)
				return committed, e
			}
		}
	}
	return committed, nil
}

// drain consumes remaining values until both channels close.
// Used to unblock provider goroutines when we abandon a stream.
func drain(toks <-chan string, errs <-chan error) {
	if toks == nil && errs == nil {
		return
	}
	for toks != nil || errs != nil {
		select {
		case _, ok := <-toks:
			if !ok {
				toks = nil
			}
		case _, ok := <-errs:
			if !ok {
				errs = nil
			}
		}
	}
}

// IsExhausted reports whether an error came from "all chain providers
// failed". Callers (handler error mapping) can use this to distinguish
// chain exhaustion from a single-provider error.
func IsExhausted(err error) bool {
	return errors.Is(err, domain.ErrProviderUnavailable)
}
