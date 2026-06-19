// Package aicall is the consumer-side port for a blocking (non-streaming)
// AI completion. Node's AiProvidersService.callProvider(provider, system,
// user) returns the full assistant text + round-trip ms; the Go rewrite
// adapters are streaming-only, so this port adds the blocking shape.
//
// Parity note: Go resolves the provider from env (AI_PROVIDERS + keys,
// provider-id pinned), NOT from Node's DB `ai_providers WHERE is_default`.
// This follows the existing rewrite divergence (cmd/server/main.go).
package aicall

import "context"

// Completer sends a blocking system+user prompt to the configured default
// provider and returns the full assistant text + round-trip milliseconds.
type Completer interface {
	Complete(ctx context.Context, system, user string) (text string, ms int64, err error)
	// Name is the provider name echoed in responses (e.g. "openai").
	Name() string
}
