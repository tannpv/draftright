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
	GetDefault(ctx context.Context) (AiProvider, error)
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

// ErrNoDefaultProvider maps to Node's BadRequestException('No default AI
// provider configured') thrown by findDefault() when no active default exists.
// The HTTP edge translates this to 400 with that exact message.
var ErrNoDefaultProvider = errors.New("No default AI provider configured")

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

// DefaultComplete resolves the active default provider and runs one blocking
// completion, returning the rewritten text AND the provider's name (Node
// provider.name) plus the elapsed ms. Mirrors Node: aiProviders.findDefault()
// then callProvider(provider, system, user). Returns ErrNoDefaultProvider when
// no default is configured. This is the single DB-resolved completion path —
// Propose, plus the rewrite + extraction usecases (wired in main.go), all
// route through it so every request reports the live DB default provider
// rather than a process-static ENV provider.
func (s *Service) DefaultComplete(ctx context.Context, system, user string) (text, name string, ms int64, err error) {
	p, err := s.repo.GetDefault(ctx)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", "", 0, ErrNoDefaultProvider
		}
		return "", "", 0, err
	}
	c, err := s.factory.For(p)
	if err != nil {
		return "", "", 0, err
	}
	text, ms, err = c.Complete(ctx, system, user)
	if err != nil {
		return "", "", 0, err
	}
	return text, p.Name, ms, nil
}

// Propose resolves the active default provider and runs one blocking
// completion. Mirrors Node: aiProviders.findDefault() then callProvider(
// provider, system, user). Used by errreport/bugreports suggest-fix + both
// hourly crons. Returns ErrNoDefaultProvider when no default is configured.
func (s *Service) Propose(ctx context.Context, system, user string) (string, error) {
	text, _, _, err := s.DefaultComplete(ctx, system, user)
	return text, err
}
