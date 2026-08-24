package aiprovider

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/tannpv/draftright-rewrite/internal/platform/secretcipher"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/anthropic"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/openai"
)

// Factory builds a live Completer from a stored AiProvider row, mirroring Node
// AiProvidersService.callProvider → ProviderStrategyRegistry.pick(provider):
//
//   - AnthropicStrategy.matches = type === 'anthropic'.
//   - OpenAiStrategy.matches    = type === 'openai' || 'ollama' || 'custom'
//     || 'google' (one OpenAI-compatible wire serves openai, ollama, custom,
//     AND Google Gemini via its /v1beta/openai/ endpoint).
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

	// Decrypt the stored api_key at the point of use (#50). No-op for legacy
	// plaintext rows and when SECRETS_ENCRYPTION_KEY is unset.
	apiKey, err := secretcipher.Decrypt(p.APIKey)
	if err != nil {
		return nil, err
	}

	switch p.Type {
	case string(ProviderOpenAI), string(ProviderOllama), string(ProviderCustom), string(ProviderGoogle):
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
		opts = append(opts, openai.WithTemperature(p.Temperature))
		opts = append(opts, openai.WithLocalLayout(p.Type == string(ProviderOllama)))
		return openai.New(id, apiKey, opts...), nil
	case string(ProviderAnthropic):
		opts := []anthropic.Option{anthropic.WithModel(p.Model)}
		if p.EndpointURL != "" {
			opts = append(opts, anthropic.WithEndpoint(p.EndpointURL))
		}
		return anthropic.New(id, apiKey, opts...), nil
	default:
		return nil, fmt.Errorf("No provider strategy registered for type %q (provider id=%s, name=%s)", p.Type, p.ID, p.Name)
	}
}
