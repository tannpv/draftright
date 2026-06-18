package aiprovider

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/anthropic"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/ollama"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/openai"
)

// Factory builds a live Completer from a stored AiProvider row, mirroring Node
// AiProvidersService.callProvider → ProviderStrategyRegistry.pick(provider):
//
//   - AnthropicStrategy.matches = type === 'anthropic'.
//   - OpenAiStrategy.matches    = type === 'openai' || 'ollama' || 'custom'
//     (one OpenAI-compatible wire serves openai, ollama, AND custom).
//   - pick with no match throws the registered-strategy error, surfaced
//     verbatim as the test route's `error` field.
//
// The rewrite adapters' *Client each expose Complete(ctx, system, user)
// (string, int64, error), so they satisfy aiprovider.Completer structurally.
type Factory struct{}

// compile-time assertion: Factory satisfies the consumer-side port.
var _ CompleterFactory = Factory{}

// For maps a provider row to its OpenAI-compatible / Anthropic adapter.
func (Factory) For(p AiProvider) (Completer, error) {
	// Adapters need a uuid.UUID. The test route never persists this id, so a
	// fresh mint on a malformed id is harmless (mirrors resolveProviderID's
	// mint-on-malformed policy in cmd/server/main.go).
	id, err := uuid.Parse(p.ID)
	if err != nil {
		id = uuid.New()
	}

	switch p.Type {
	case "openai":
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
	case "ollama":
		return ollama.New(id, ollama.WithEndpoint(p.EndpointURL), ollama.WithModel(p.Model)), nil
	case "custom":
		// Node's OpenAiStrategy also covers CUSTOM: an OpenAI-compatible wire
		// pointed at the operator-supplied custom endpoint.
		return openai.New(id, p.APIKey, openai.WithModel(p.Model), openai.WithEndpoint(p.EndpointURL)), nil
	default:
		return nil, fmt.Errorf("No provider strategy registered for type %q (provider id=%s, name=%s)", p.Type, p.ID, p.Name)
	}
}
