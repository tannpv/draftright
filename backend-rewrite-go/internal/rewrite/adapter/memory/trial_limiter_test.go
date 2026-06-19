package memory

import (
	"context"
	"testing"
)

func TestTrialLimiter_IncrSameKeyCounts(t *testing.T) {
	l := NewTrialLimiter()
	ctx := context.Background()

	n, err := l.Incr(ctx, "trial:1.2.3.4:2026-06-19", 86400)
	if err != nil || n != 1 {
		t.Fatalf("first incr = (%d, %v), want (1, nil)", n, err)
	}
	n, err = l.Incr(ctx, "trial:1.2.3.4:2026-06-19", 86400)
	if err != nil || n != 2 {
		t.Fatalf("second incr = (%d, %v), want (2, nil)", n, err)
	}
}

func TestTrialLimiter_KeysIndependent(t *testing.T) {
	l := NewTrialLimiter()
	ctx := context.Background()

	if n, _ := l.Incr(ctx, "a", 1); n != 1 {
		t.Fatalf("key a first = %d, want 1", n)
	}
	if n, _ := l.Incr(ctx, "b", 1); n != 1 {
		t.Fatalf("key b first = %d, want 1 (independent of a)", n)
	}
	if n, _ := l.Incr(ctx, "a", 1); n != 2 {
		t.Fatalf("key a second = %d, want 2", n)
	}
}
