package aiprovider

import (
	"context"
	"errors"

	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// testSystemPrompt / testUserPrompt are the two fixed literals Node's admin
// test route passes to callProvider (src/admin/admin.controller.ts
// testProvider → callProvider(provider, system, user)). Byte-for-byte parity.
const (
	testSystemPrompt = "Rewrite this text to be more concise."
	testUserPrompt   = "This is a test sentence to verify the connection works properly."
)

// Repo is the consumer-side persistence port, satisfied by *PgRepo (Task 2).
type Repo interface {
	List(ctx context.Context) ([]AiProvider, error)
	ListPaginated(ctx context.Context, b listquery.Built) ([]AiProvider, int, error)
	GetByID(ctx context.Context, id string) (AiProvider, error)
	DemoteDefaults(ctx context.Context) error
	Insert(ctx context.Context, in NewProvider) (AiProvider, error)
	Update(ctx context.Context, id string, p ProviderPatch) (AiProvider, error)
	SoftDelete(ctx context.Context, id string) error
}

// Completer is the consumer-side port for a blocking provider call. It is a
// minimal subset of aicall.Completer: the test route only needs Complete, so
// Name() is intentionally dropped (no unused methods on consumer ports). The
// factory in a later task bridges aicall.Completer (which also exposes Name)
// into this interface.
type Completer interface {
	Complete(ctx context.Context, system, user string) (text string, ms int64, err error)
}

// CompleterFactory builds a Completer from a stored provider row. The real
// implementation (injected from main.go later) maps provider.Type → the
// matching rewrite adapter; tests fake it.
type CompleterFactory interface {
	For(p AiProvider) (Completer, error)
}

// TestResult mirrors the JSON body of Node's POST /admin/ai-providers/:id/test
// route (always 200): { success, response, response_time_ms } on success;
// { success:false, error } on failure or not-found.
type TestResult struct {
	Success        bool   `json:"success"`
	Response       string `json:"response,omitempty"`
	ResponseTimeMs int64  `json:"response_time_ms,omitempty"`
	Error          string `json:"error,omitempty"`
}

// Service is the ai-providers usecase, parity with Node AiProvidersService
// plus the admin controller's testProvider route.
type Service struct {
	repo    Repo
	factory CompleterFactory
}

// NewService wires the repo + completer factory.
func NewService(repo Repo, factory CompleterFactory) *Service {
	return &Service{repo: repo, factory: factory}
}

// List returns all providers ordered created_at ASC (Node findAll).
func (s *Service) List(ctx context.Context) ([]AiProvider, error) {
	return s.repo.List(ctx)
}

// ListPaginated returns the paginated/filtered set (Node findAllPaginated).
func (s *Service) ListPaginated(ctx context.Context, b listquery.Built) ([]AiProvider, int, error) {
	return s.repo.ListPaginated(ctx, b)
}

// Create inserts a provider. Single-default invariant (Node create): when
// is_default=true, demote all prior defaults FIRST, then insert.
func (s *Service) Create(ctx context.Context, in NewProvider) (AiProvider, error) {
	if in.IsDefault {
		if err := s.repo.DemoteDefaults(ctx); err != nil {
			return AiProvider{}, err
		}
	}
	return s.repo.Insert(ctx, in)
}

// Update patches a provider. Same single-default invariant (Node update):
// when the patch sets is_default=true, demote prior defaults FIRST.
func (s *Service) Update(ctx context.Context, id string, p ProviderPatch) (AiProvider, error) {
	if p.IsDefault != nil && *p.IsDefault {
		if err := s.repo.DemoteDefaults(ctx); err != nil {
			return AiProvider{}, err
		}
	}
	return s.repo.Update(ctx, id, p)
}

// SoftDelete deactivates a provider (Node softDelete: is_active=false,
// is_default=false). The repo (Task 2) writes both columns.
func (s *Service) SoftDelete(ctx context.Context, id string) error {
	return s.repo.SoftDelete(ctx, id)
}

// Test runs the admin test route: load the provider, build a completer, send
// the two fixed prompts. Returns the 200 body shape Node returns. Missing
// provider yields { success:false, error:"Provider not found" } (Node returns
// this as a 200 body, NOT a 404).
func (s *Service) Test(ctx context.Context, id string) TestResult {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return TestResult{Success: false, Error: "Provider not found"}
		}
		return TestResult{Success: false, Error: err.Error()}
	}
	c, err := s.factory.For(p)
	if err != nil {
		return TestResult{Success: false, Error: err.Error()}
	}
	text, ms, err := c.Complete(ctx, testSystemPrompt, testUserPrompt)
	if err != nil {
		return TestResult{Success: false, Error: err.Error()}
	}
	return TestResult{Success: true, Response: text, ResponseTimeMs: ms}
}
