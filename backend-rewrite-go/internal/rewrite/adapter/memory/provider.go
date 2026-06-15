package memory

import (
	"context"

	"github.com/google/uuid"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
)

// Provider is an in-memory domain.AiProvider that emits a scripted
// token sequence then optionally a terminal error. ctx cancellation
// stops mid-stream cleanly (matches real provider behaviour).
//
// Token-by-token streaming respects ctx so the producer goroutine
// never leaks — a critical invariant the use case relies on.
type Provider struct {
	id         uuid.UUID
	name       string
	tokens     []string
	final      error
	completion string // scripted blocking-Complete result; see complete.go
}

// NewProvider builds a streaming fake. id + name appear on the
// resulting usage_logs row's ai_provider_id + the provider lookup
// shape (call .ID() and .Name() in test assertions).
//
// Convenience: pass nil for tokens to stream nothing (empty success).
func NewProvider(name string, tokens []string) *Provider {
	return &Provider{
		id:     uuid.New(),
		name:   name,
		tokens: tokens,
	}
}

// WithFinalError makes the provider end the stream with the given
// error — exercises the "AI provider 5xx mid-stream" path.
func (p *Provider) WithFinalError(err error) *Provider {
	p.final = err
	return p
}

// WithID overrides the auto-generated UUID — useful when a test needs
// to assert the recorded ai_provider_id on a usage log.
func (p *Provider) WithID(id uuid.UUID) *Provider {
	p.id = id
	return p
}

// ID returns the provider's ai_providers.id (recorded on usage logs).
func (p *Provider) ID() uuid.UUID { return p.id }

// Name identifies the provider in logs.
func (p *Provider) Name() string { return p.name }

// Stream returns two channels: one for tokens, one for the terminal
// error. Both close when the producer goroutine returns. Producer
// respects ctx so cancellation never leaks the goroutine.
func (p *Provider) Stream(ctx context.Context, _ domain.RewriteRequest) (<-chan string, <-chan error) {
	tokens := make(chan string)
	errs := make(chan error, 1)
	go func() {
		defer close(tokens)
		defer close(errs)
		for _, t := range p.tokens {
			select {
			case <-ctx.Done():
				return
			case tokens <- t:
			}
		}
		if p.final != nil {
			errs <- p.final
		}
	}()
	return tokens, errs
}
