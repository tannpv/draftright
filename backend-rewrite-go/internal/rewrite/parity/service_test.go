package parity

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
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

// fakeTrialLimiter is an in-memory TrialLimiter. start seeds the count that
// the next Incr returns; err forces the fail-open path. lastKey records the
// key Incr was called with (for clientIP assertions).
type fakeTrialLimiter struct {
	count   int64
	err     error
	lastKey string
	lastTTL int
}

func (f *fakeTrialLimiter) Incr(_ context.Context, key string, ttlSec int) (int64, error) {
	f.lastKey, f.lastTTL = key, ttlSec
	if f.err != nil {
		return 0, f.err
	}
	f.count++
	return f.count, nil
}

// fixedNow returns a now func pinned to a known UTC date for key assertions.
func fixedNow() func() time.Time {
	return func() time.Time { return time.Date(2026, 6, 19, 10, 30, 0, 0, time.UTC) }
}

func TestTrialRewrite_OverLimit(t *testing.T) {
	lim := &fakeTrialLimiter{count: 3} // next Incr returns 4 > limit 3
	svc := NewService(&fakeCompleter{text: "x"}, &fakeEnts{}, &fakeUsage{}).
		WithTrial(lim, 3, fixedNow())
	_, err := svc.TrialRewrite(context.Background(), "hi", "polished", "1.2.3.4", "", "")
	if !errors.Is(err, ErrTrialLimit) {
		t.Fatalf("err = %v, want ErrTrialLimit", err)
	}
}

func TestTrialRewrite_NormalToneEnvelope(t *testing.T) {
	lim := &fakeTrialLimiter{}
	svc := NewService(&fakeCompleter{text: "Hello."}, &fakeEnts{}, &fakeUsage{}).
		WithTrial(lim, 3, fixedNow())
	out, err := svc.TrialRewrite(context.Background(), "hi", "polished", "1.2.3.4", "", "")
	if err != nil {
		t.Fatalf("TrialRewrite: %v", err)
	}
	got, _ := json.Marshal(out)
	want := `{"rewritten_text":"Hello."}`
	if string(got) != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
	if lim.lastKey != "trial:1.2.3.4:2026-06-19" {
		t.Fatalf("key = %q", lim.lastKey)
	}
	if lim.lastTTL != 86400 {
		t.Fatalf("ttl = %d, want 86400", lim.lastTTL)
	}
}

func TestTrialRewrite_GrammarCheckEnvelope(t *testing.T) {
	lim := &fakeTrialLimiter{}
	svc := NewService(&fakeCompleter{text: `{"score":90,"issues":[]}`}, &fakeEnts{}, &fakeUsage{}).
		WithTrial(lim, 3, fixedNow())
	out, err := svc.TrialRewrite(context.Background(), "hi", "grammar_check", "1.2.3.4", "", "")
	if err != nil {
		t.Fatalf("TrialRewrite: %v", err)
	}
	got, _ := json.Marshal(out)
	want := `{"grammar":{"score":90,"issues":[]}}`
	if string(got) != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestTrialRewrite_FailOpen(t *testing.T) {
	// Limiter error → treated as count 0 → proceeds (not limited).
	lim := &fakeTrialLimiter{err: errors.New("redis down")}
	svc := NewService(&fakeCompleter{text: "Hello."}, &fakeEnts{}, &fakeUsage{}).
		WithTrial(lim, 3, fixedNow())
	out, err := svc.TrialRewrite(context.Background(), "hi", "polished", "1.2.3.4", "", "")
	if err != nil {
		t.Fatalf("fail-open should proceed, got %v", err)
	}
	got, _ := json.Marshal(out)
	if string(got) != `{"rewritten_text":"Hello."}` {
		t.Fatalf("body = %s", got)
	}
}

func TestTrialRewrite_ProviderFailed(t *testing.T) {
	lim := &fakeTrialLimiter{}
	svc := NewService(&fakeCompleter{err: errors.New("upstream 500")}, &fakeEnts{}, &fakeUsage{}).
		WithTrial(lim, 3, fixedNow())
	_, err := svc.TrialRewrite(context.Background(), "hi", "polished", "1.2.3.4", "", "")
	if !errors.Is(err, ErrProviderFailed) {
		t.Fatalf("err = %v, want ErrProviderFailed", err)
	}
}

func TestRewrite_NoDefaultProvider(t *testing.T) {
	// Node resolves the default provider OUTSIDE callProvider's try, so a
	// missing default is a 400 (ErrNoDefaultProvider), NOT the 502
	// ErrProviderFailed path. callAI must pass the sentinel through.
	svc := NewService(
		&fakeCompleter{err: ErrNoDefaultProvider},
		&fakeEnts{limit: 500},
		&fakeUsage{count: 0},
	)
	_, err := svc.Rewrite(context.Background(), "u1", "hi", "polished", "", "")
	if !errors.Is(err, ErrNoDefaultProvider) {
		t.Fatalf("err = %v, want ErrNoDefaultProvider", err)
	}
	if errors.Is(err, ErrProviderFailed) {
		t.Fatal("no-default must not collapse to ErrProviderFailed")
	}
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
