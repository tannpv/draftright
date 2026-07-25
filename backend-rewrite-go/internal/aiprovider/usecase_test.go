package aiprovider

import (
	"context"
	"errors"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

type fakeRepo struct {
	demoted     bool
	inserted    NewProvider
	updatedID   string
	updatedPath ProviderPatch
	provider    *AiProvider
}

func (f *fakeRepo) List(context.Context) ([]AiProvider, error) { return nil, nil }
func (f *fakeRepo) ListPaginated(context.Context, listquery.Built) ([]AiProvider, int, error) {
	return nil, 0, nil
}
func (f *fakeRepo) GetByID(_ context.Context, id string) (AiProvider, error) {
	if f.provider == nil {
		return AiProvider{}, ErrNotFound
	}
	return *f.provider, nil
}
func (f *fakeRepo) GetDefault(context.Context) (AiProvider, error) {
	if f.provider == nil {
		return AiProvider{}, ErrNotFound
	}
	return *f.provider, nil
}
func (f *fakeRepo) DemoteDefaults(context.Context) error { f.demoted = true; return nil }
func (f *fakeRepo) Insert(_ context.Context, in NewProvider) (AiProvider, error) {
	f.inserted = in
	return AiProvider{ID: "new", Name: in.Name, IsDefault: in.IsDefault}, nil
}
func (f *fakeRepo) Update(_ context.Context, id string, p ProviderPatch) (AiProvider, error) {
	f.updatedID = id
	f.updatedPath = p
	return AiProvider{ID: "u"}, nil
}
func (f *fakeRepo) SoftDelete(context.Context, string) error { return nil }

type fakeCompleter struct {
	text string
	err  error
}

func (f fakeCompleter) Complete(context.Context, string, string) (string, int64, error) {
	return f.text, 42, f.err
}

type fakeFactory struct {
	c   Completer
	err error
}

func (f fakeFactory) For(AiProvider) (Completer, error) { return f.c, f.err }

func TestCreate_DemotesWhenDefault(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, fakeFactory{})
	_, err := svc.Create(context.Background(), NewProvider{Name: "X", Type: "openai", Model: "m", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	if !repo.demoted {
		t.Fatal("is_default=true must demote prior defaults before insert")
	}
}

func TestTest_NotFound(t *testing.T) {
	svc := NewService(&fakeRepo{provider: nil}, fakeFactory{})
	res := svc.Test(context.Background(), "missing")
	if res.Success || res.Error != "Provider not found" {
		t.Fatalf("not-found result mismatch: %+v", res)
	}
}

func TestTest_Success(t *testing.T) {
	p := AiProvider{ID: "1", Type: "openai"}
	svc := NewService(&fakeRepo{provider: &p}, fakeFactory{c: fakeCompleter{text: "ok"}})
	res := svc.Test(context.Background(), "1")
	if !res.Success || res.Response != "ok" || res.ResponseTimeMs != 42 {
		t.Fatalf("success result mismatch: %+v", res)
	}
}

func TestTest_CompleterError(t *testing.T) {
	p := AiProvider{ID: "1", Type: "openai"}
	svc := NewService(&fakeRepo{provider: &p}, fakeFactory{c: fakeCompleter{err: errors.New("boom")}})
	res := svc.Test(context.Background(), "1")
	if res.Success || res.Error != "boom" {
		t.Fatalf("error result mismatch: %+v", res)
	}
}

func TestPropose_NoDefault(t *testing.T) {
	// provider==nil → fakeRepo.GetDefault returns ErrNotFound, which Propose
	// must translate to ErrNoDefaultProvider (Node 400 "No default AI provider
	// configured").
	svc := NewService(&fakeRepo{provider: nil}, fakeFactory{})
	_, err := svc.Propose(context.Background(), "sys", "user")
	if !errors.Is(err, ErrNoDefaultProvider) {
		t.Fatalf("expected ErrNoDefaultProvider, got %v", err)
	}
}

func TestPropose_OK(t *testing.T) {
	p := AiProvider{ID: "1", Type: "openai"}
	svc := NewService(&fakeRepo{provider: &p}, fakeFactory{c: fakeCompleter{text: "ANALYSIS"}})
	got, err := svc.Propose(context.Background(), "sys", "user")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ANALYSIS" {
		t.Fatalf("expected completion text ANALYSIS, got %q", got)
	}
}

func TestDefaultComplete_ReportsNameAndMs(t *testing.T) {
	// DefaultComplete must surface the DB provider's name (Node provider.name)
	// and the completer's elapsed ms — the values rewrite/extraction report.
	p := AiProvider{ID: "1", Name: "Ollama Llama 3.2", Type: "ollama"}
	svc := NewService(&fakeRepo{provider: &p}, fakeFactory{c: fakeCompleter{text: "REWRITTEN"}})
	text, name, ms, err := svc.DefaultComplete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatal(err)
	}
	if text != "REWRITTEN" || name != "Ollama Llama 3.2" || ms != 42 {
		t.Fatalf("DefaultComplete mismatch: text=%q name=%q ms=%d", text, name, ms)
	}
}

func TestDefaultComplete_NoDefault(t *testing.T) {
	svc := NewService(&fakeRepo{provider: nil}, fakeFactory{})
	_, _, _, err := svc.DefaultComplete(context.Background(), "sys", "user")
	if !errors.Is(err, ErrNoDefaultProvider) {
		t.Fatalf("expected ErrNoDefaultProvider, got %v", err)
	}
}

func TestPropose_CompleterError(t *testing.T) {
	p := AiProvider{ID: "1", Type: "openai"}
	svc := NewService(&fakeRepo{provider: &p}, fakeFactory{c: fakeCompleter{err: errors.New("boom")}})
	_, err := svc.Propose(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected an error from a failing completer")
	}
	if errors.Is(err, ErrNoDefaultProvider) {
		t.Fatalf("completer error must not be reported as ErrNoDefaultProvider: %v", err)
	}
}

// --- Failover (multi-provider) ------------------------------------------

type failoverRepo struct {
	def AiProvider
	all []AiProvider
}

func (r *failoverRepo) List(context.Context) ([]AiProvider, error) { return r.all, nil }
func (r *failoverRepo) ListPaginated(context.Context, listquery.Built) ([]AiProvider, int, error) {
	return nil, 0, nil
}
func (r *failoverRepo) GetByID(context.Context, string) (AiProvider, error) {
	return AiProvider{}, ErrNotFound
}
func (r *failoverRepo) GetDefault(context.Context) (AiProvider, error) { return r.def, nil }
func (r *failoverRepo) DemoteDefaults(context.Context) error           { return nil }
func (r *failoverRepo) Insert(context.Context, NewProvider) (AiProvider, error) {
	return AiProvider{}, nil
}
func (r *failoverRepo) Update(context.Context, string, ProviderPatch) (AiProvider, error) {
	return AiProvider{}, nil
}
func (r *failoverRepo) SoftDelete(context.Context, string) error { return nil }

type failoverFactory struct{ byID map[string]Completer }

func (f failoverFactory) For(p AiProvider) (Completer, error) {
	c, ok := f.byID[p.ID]
	if !ok {
		return nil, errors.New("no completer for " + p.ID)
	}
	return c, nil
}

func TestDefaultCompleteFull_FailsOverToNextActiveProvider(t *testing.T) {
	primary := AiProvider{ID: "p1", Name: "OpenAI", Type: "openai", Model: "gpt-5-nano", IsDefault: true, IsActive: true}
	fallback := AiProvider{ID: "p2", Name: "Ollama", Type: "ollama", Model: "gpt-oss", IsActive: true}
	inactive := AiProvider{ID: "p3", Name: "Dead", IsActive: false}

	repo := &failoverRepo{def: primary, all: []AiProvider{primary, fallback, inactive}}
	factory := failoverFactory{byID: map[string]Completer{
		"p1": fakeCompleter{err: errors.New("provider 500")},   // default fails
		"p2": fakeCompleter{text: "polished by fallback"},      // fallback wins
		"p3": fakeCompleter{text: "inactive - must never run"}, // skipped (inactive)
	}}
	svc := NewService(repo, factory)

	text, name, model, ptype, _, err := svc.DefaultCompleteFull(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("expected failover success, got err: %v", err)
	}
	if text != "polished by fallback" || name != "Ollama" || model != "gpt-oss" || ptype != "ollama" {
		t.Fatalf("expected fallback provider result, got text=%q name=%q model=%q type=%q", text, name, model, ptype)
	}
}

func TestDefaultCompleteFull_DefaultSucceeds_NoFailover(t *testing.T) {
	primary := AiProvider{ID: "p1", Name: "OpenAI", Type: "openai", Model: "gpt-5-nano", IsDefault: true, IsActive: true}
	fallback := AiProvider{ID: "p2", Name: "Ollama", Type: "ollama", IsActive: true}
	repo := &failoverRepo{def: primary, all: []AiProvider{primary, fallback}}
	factory := failoverFactory{byID: map[string]Completer{
		"p1": fakeCompleter{text: "from default"},
		"p2": fakeCompleter{text: "should not run"},
	}}
	svc := NewService(repo, factory)

	text, name, _, _, _, err := svc.DefaultCompleteFull(context.Background(), "s", "u")
	if err != nil || text != "from default" || name != "OpenAI" {
		t.Fatalf("default must win with no failover: text=%q name=%q err=%v", text, name, err)
	}
}

func TestDefaultCompleteFull_AllProvidersFail_ReturnsLastError(t *testing.T) {
	primary := AiProvider{ID: "p1", IsDefault: true, IsActive: true}
	fallback := AiProvider{ID: "p2", IsActive: true}
	repo := &failoverRepo{def: primary, all: []AiProvider{primary, fallback}}
	boom := errors.New("boom")
	factory := failoverFactory{byID: map[string]Completer{
		"p1": fakeCompleter{err: errors.New("first fail")},
		"p2": fakeCompleter{err: boom},
	}}
	svc := NewService(repo, factory)

	_, _, _, _, _, err := svc.DefaultCompleteFull(context.Background(), "s", "u")
	if !errors.Is(err, boom) {
		t.Fatalf("expected the last provider error, got: %v", err)
	}
}
