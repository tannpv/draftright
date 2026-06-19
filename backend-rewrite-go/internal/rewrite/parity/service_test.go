package parity

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// --- fakes for the consumer-side ports ---

type fakeCompleter struct {
	text string
	ms   int64
	err  error
}

func (f *fakeCompleter) Complete(_ context.Context, _, _ string) (string, int64, error) {
	return f.text, f.ms, f.err
}

type fakeEnts struct {
	limit int
	err   error
}

func (f *fakeEnts) ResolveDailyLimit(_ context.Context, _ string) (int, error) {
	return f.limit, f.err
}

type fakeUsage struct {
	count int
	err   error
}

func (f *fakeUsage) CountToday(_ context.Context, _ string) (int, error) {
	return f.count, f.err
}

func TestRewrite_RewriteToneEnvelope(t *testing.T) {
	svc := NewService(
		&fakeCompleter{text: "Hello.", ms: 12},
		&fakeEnts{limit: 500},
		&fakeUsage{count: 0},
	)
	out, err := svc.Rewrite(context.Background(), "u1", "hi", "polished", "", "")
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	got, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"rewritten_text":"Hello.","usage_today":1,"daily_limit":500}`
	if string(got) != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRewrite_QuotaExceeded(t *testing.T) {
	svc := NewService(
		&fakeCompleter{text: "Hello."},
		&fakeEnts{limit: 500},
		&fakeUsage{count: 500},
	)
	_, err := svc.Rewrite(context.Background(), "u1", "hi", "polished", "", "")
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("err = %v, want ErrQuotaExceeded", err)
	}
}

func TestRewrite_UnlimitedSentinel(t *testing.T) {
	// limit == -1 is the ONLY unlimited sentinel; usage well over any cap must
	// NOT trip the quota check.
	svc := NewService(
		&fakeCompleter{text: "Hello."},
		&fakeEnts{limit: -1},
		&fakeUsage{count: 9999},
	)
	out, err := svc.Rewrite(context.Background(), "u1", "hi", "polished", "", "")
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	got, _ := json.Marshal(out)
	want := `{"rewritten_text":"Hello.","usage_today":10000,"daily_limit":-1}`
	if string(got) != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRewrite_ProviderFailed(t *testing.T) {
	svc := NewService(
		&fakeCompleter{err: errors.New("upstream 500")},
		&fakeEnts{limit: 500},
		&fakeUsage{count: 0},
	)
	_, err := svc.Rewrite(context.Background(), "u1", "hi", "polished", "", "")
	if !errors.Is(err, ErrProviderFailed) {
		t.Fatalf("err = %v, want ErrProviderFailed", err)
	}
}

func TestRewrite_GrammarCheckEnvelope(t *testing.T) {
	svc := NewService(
		&fakeCompleter{text: `{"score":90,"issues":[]}`},
		&fakeEnts{limit: 500},
		&fakeUsage{count: 2},
	)
	out, err := svc.Rewrite(context.Background(), "u1", "hi", "grammar_check", "", "")
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	got, _ := json.Marshal(out)
	want := `{"grammar":{"score":90,"issues":[]},"usage_today":3,"daily_limit":500}`
	if string(got) != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRewrite_GrammarCheckParseFailure(t *testing.T) {
	svc := NewService(
		&fakeCompleter{text: "not json"},
		&fakeEnts{limit: 500},
		&fakeUsage{count: 0},
	)
	out, err := svc.Rewrite(context.Background(), "u1", "hi", "grammar_check", "", "")
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	got, _ := json.Marshal(out)
	want := `{"grammar":{"score":0,"issues":[],"error":"Failed to parse grammar analysis"},"usage_today":1,"daily_limit":500}`
	if string(got) != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRewrite_UnknownTone(t *testing.T) {
	// Unreachable via the DTO, but callAI's resolvePrompt==null path must stay
	// faithful: a typed *UnknownToneError carrying the tone.
	svc := NewService(
		&fakeCompleter{text: "x"},
		&fakeEnts{limit: 500},
		&fakeUsage{count: 0},
	)
	_, err := svc.Rewrite(context.Background(), "u1", "hi", "bogus", "", "")
	var ute *UnknownToneError
	if !errors.As(err, &ute) {
		t.Fatalf("err = %v, want *UnknownToneError", err)
	}
	if ute.Error() != "Unknown tone: bogus" {
		t.Fatalf("msg = %q", ute.Error())
	}
}
