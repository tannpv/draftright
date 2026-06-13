package user

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type recExecer struct {
	stmts  []string
	failOn int // index to fail at; -1 never
	calls  int
}

func (r *recExecer) Exec(_ context.Context, sql string, _ ...any) error {
	r.calls++
	r.stmts = append(r.stmts, sql)
	if r.failOn >= 0 && len(r.stmts)-1 == r.failOn {
		return errors.New("boom")
	}
	return nil
}

func TestDeleteAccountStmts_OrderAndTables(t *testing.T) {
	r := &recExecer{failOn: -1}
	if err := deleteAccountStmts(context.Background(), r, "u1"); err != nil {
		t.Fatal(err)
	}
	wantContains := []string{
		"extension_tokens", "payments", "usage_logs", "subscriptions",
		"feature_votes", "bug_reports", "error_reports", "FROM users",
	}
	if len(r.stmts) != len(wantContains) {
		t.Fatalf("ran %d stmts, want %d", len(r.stmts), len(wantContains))
	}
	for i, frag := range wantContains {
		if !strings.Contains(r.stmts[i], frag) {
			t.Fatalf("stmt[%d]=%q missing %q (order matters)", i, r.stmts[i], frag)
		}
	}
}

func TestDeleteAccountStmts_AbortsOnError(t *testing.T) {
	r := &recExecer{failOn: 2}
	err := deleteAccountStmts(context.Background(), r, "u1")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if r.calls != 3 {
		t.Fatalf("ran %d stmts after failure, want 3 (stops at failure)", r.calls)
	}
}
