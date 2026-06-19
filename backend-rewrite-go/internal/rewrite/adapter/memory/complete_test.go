package memory

import (
	"context"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/aicall"
)

// Compile-time proof the fake satisfies the blocking-provider port.
var _ aicall.Completer = (*Provider)(nil)

func TestProvider_Complete_ReturnsScripted(t *testing.T) {
	p := NewProvider("memory-stub", nil)
	p.SetCompletion(`[{"kind":"address"}]`)
	got, ms, err := p.Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != `[{"kind":"address"}]` {
		t.Errorf("text = %q", got)
	}
	if ms < 0 {
		t.Errorf("ms = %d", ms)
	}
	if p.Name() != "memory-stub" {
		t.Errorf("name = %q", p.Name())
	}
}
