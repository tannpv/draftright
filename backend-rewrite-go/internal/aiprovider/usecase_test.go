package aiprovider

import (
	"context"
	"errors"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

type fakeRepo struct {
	demoted  bool
	inserted NewProvider
	provider *AiProvider
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
func (f *fakeRepo) Update(context.Context, string, ProviderPatch) (AiProvider, error) {
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
