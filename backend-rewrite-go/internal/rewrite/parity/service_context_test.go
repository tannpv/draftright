package parity

import (
	"context"
	"strings"
	"testing"
)

// capturingCompleter records the system prompt it was handed so we can assert
// the per-user context (#173) is (or isn't) prepended.
type capturingCompleter struct {
	system string
}

func (c *capturingCompleter) Complete(_ context.Context, system, _ string) (Completion, error) {
	c.system = system
	return Completion{Text: "ok"}, nil
}

// fakeCtxProvider returns a fixed preamble and records the userID asked for.
type fakeCtxProvider struct {
	preamble  string
	askedUser string
}

func (f *fakeCtxProvider) Preamble(_ context.Context, userID string) string {
	f.askedUser = userID
	return f.preamble
}

const testPreamble = "About the person you are writing for: Lawyer. Apply this to the rewrite, but do not mention it or address the person.\n\n"

func TestRewrite_PrependsUserContextPreamble(t *testing.T) {
	comp := &capturingCompleter{}
	prov := &fakeCtxProvider{preamble: testPreamble}
	svc := NewService(comp, &fakeEnts{limit: -1}, &fakeUsage{}).WithUserContext(prov)

	if _, err := svc.Rewrite(context.Background(), "user-42", "hello", "polished", "", "", ""); err != nil {
		t.Fatalf("rewrite err: %v", err)
	}
	if prov.askedUser != "user-42" {
		t.Fatalf("provider asked for %q, want user-42", prov.askedUser)
	}
	base := ResolvePrompt("polished", "", "", "")
	if comp.system != testPreamble+base {
		t.Fatalf("preamble not prepended.\n system=%q", comp.system)
	}
}

func TestRewrite_NoProvider_PromptUnchanged(t *testing.T) {
	comp := &capturingCompleter{}
	svc := NewService(comp, &fakeEnts{limit: -1}, &fakeUsage{}) // no WithUserContext

	if _, err := svc.Rewrite(context.Background(), "user-42", "hello", "polished", "", "", ""); err != nil {
		t.Fatalf("rewrite err: %v", err)
	}
	if comp.system != ResolvePrompt("polished", "", "", "") {
		t.Fatalf("prompt should be unchanged without a provider, got %q", comp.system)
	}
}

func TestRewrite_EmptyPreamble_PromptUnchanged(t *testing.T) {
	comp := &capturingCompleter{}
	prov := &fakeCtxProvider{preamble: ""} // opted-out user
	svc := NewService(comp, &fakeEnts{limit: -1}, &fakeUsage{}).WithUserContext(prov)

	if _, err := svc.Rewrite(context.Background(), "user-42", "hi", "polished", "", "", ""); err != nil {
		t.Fatalf("rewrite err: %v", err)
	}
	if comp.system != ResolvePrompt("polished", "", "", "") {
		t.Fatalf("empty preamble should leave the prompt unchanged, got %q", comp.system)
	}
}

func TestTrialRewrite_NeverPersonalizes(t *testing.T) {
	comp := &capturingCompleter{}
	prov := &fakeCtxProvider{preamble: testPreamble}
	svc := NewService(comp, &fakeEnts{limit: -1}, &fakeUsage{}).
		WithUserContext(prov).
		WithTrial(&fakeTrialLimiter{}, 999, nil)

	if _, err := svc.TrialRewrite(context.Background(), "hi", "polished", "1.2.3.4", "", "", ""); err != nil {
		t.Fatalf("trial err: %v", err)
	}
	if strings.Contains(comp.system, "Lawyer") {
		t.Fatalf("trial must not personalize, but preamble leaked: %q", comp.system)
	}
	if prov.askedUser != "" {
		t.Fatalf("trial must not query the context provider, asked for %q", prov.askedUser)
	}
}
