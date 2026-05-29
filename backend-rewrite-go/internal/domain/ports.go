package domain

import (
	"context"

	"github.com/google/uuid"
)

// Ports — the seams between the domain and the rest of the world.
// Every concrete dependency lives behind one of these interfaces;
// adapters in internal/adapter/* implement them. The use case layer
// (internal/usecase) takes these as parameters, never the concrete
// types. That's the dependency-inversion half of clean architecture.

// UserRepo loads + mutates the parts of `users` the rewrite flow cares
// about. Implemented by adapter/pg.PgUserRepo today; an in-memory
// version (adapter/memory.FakeUserRepo) handles unit tests.
type UserRepo interface {
	// Find returns the user joined with their active subscription's
	// plan + today's usage count, or ErrUserNotFound. Single query
	// round-trip — see internal/adapter/pg/queries.sql:FindUserWithPlan.
	Find(ctx context.Context, id UserID) (*User, error)

	// LogUsage inserts one usage_logs row. Idempotency is the caller's
	// concern; this just writes.
	LogUsage(ctx context.Context, log UsageLog) error
}

// AiProvider streams a rewrite response back. The contract is
// deliberately narrow: produce tokens via two channels (one for the
// happy stream, one for the terminal error). Both close when the
// upstream is done. ctx cancellation must abort the in-flight call.
//
// Implementations:
//   adapter/openai     — OpenAI chat completions (Task 6)
//   adapter/anthropic  — Anthropic messages (Task 8)
//   adapter/ollama     — local Ollama (Task 8)
//   adapter/memory     — deterministic fake for unit tests
type AiProvider interface {
	// Stream takes a validated request, returns a token channel + an
	// error channel. Caller MUST consume both until they close, or
	// drain via ctx cancellation. Leaking either is a goroutine leak
	// on the provider side.
	Stream(ctx context.Context, req RewriteRequest) (<-chan string, <-chan error)

	// Name identifies the provider in logs + the usage log row's
	// ai_provider_id lookup. "openai" / "anthropic" / "ollama".
	Name() string

	// ID is the ai_providers.id UUID — recorded on every usage_logs
	// row so analytics can attribute load to a provider.
	ID() uuid.UUID
}

// UsageLog is what the rewrite flow records on success. Stored via
// UserRepo.LogUsage — kept here in the domain so the use case can build
// one without depending on the PG schema.
type UsageLog struct {
	UserID         UserID
	Tone           Tone
	InputLength    int32
	OutputLength   int32
	AIProviderID   uuid.UUID
	ResponseTimeMs int32
}

// RateLimiter is a fast pre-check before the DB read — Redis token
// bucket in production, in-memory map in tests. Returns ErrRateLimited
// when the user has burned their per-minute budget (defending against
// runaway clients independently of the daily Plan.DailyLimit).
type RateLimiter interface {
	Check(ctx context.Context, id UserID) error
}
