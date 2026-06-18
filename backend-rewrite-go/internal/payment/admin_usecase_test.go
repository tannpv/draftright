// Package payment admin usecase tests — verify AdminService.GetStats in
// isolation against a fake repo satisfying paymentAdminRepo. Parity authority:
// src/payment/payment.service.ts getStats() (line 745) which returns
// { total, completed, pending, revenue }. Same fake-repo approach as
// internal/adminstats tests; the DB-backed sqlc path is not unit-tested here.
package payment

import (
	"context"
	"errors"
	"testing"
)

// fakeAdminStatsRepo satisfies paymentAdminRepo, returning canned stats or an
// error so the service can be exercised without a database.
type fakeAdminStatsRepo struct {
	stats PaymentStats
	err   error
}

func (f *fakeAdminStatsRepo) Stats(_ context.Context) (PaymentStats, error) {
	return f.stats, f.err
}

// TestAdminPaymentStats_ReturnsRepoValues verifies GetStats passes the repo's
// aggregate row through unchanged as PaymentStats.
func TestAdminPaymentStats_ReturnsRepoValues(t *testing.T) {
	repo := &fakeAdminStatsRepo{stats: PaymentStats{
		Total:     10,
		Completed: 6,
		Pending:   3,
		Revenue:   1998,
	}}
	svc := NewAdminServiceWithRepo(repo)

	got, err := svc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	want := PaymentStats{Total: 10, Completed: 6, Pending: 3, Revenue: 1998}
	if got != want {
		t.Errorf("GetStats() = %+v, want %+v", got, want)
	}
}

// TestAdminPaymentStats_PropagatesError verifies a repo error surfaces with a
// zero-value result (no partial data).
func TestAdminPaymentStats_PropagatesError(t *testing.T) {
	boom := errors.New("stats query down")
	svc := NewAdminServiceWithRepo(&fakeAdminStatsRepo{err: boom})

	got, err := svc.GetStats(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("GetStats() error = %v, want %v", err, boom)
	}
	if (got != PaymentStats{}) {
		t.Errorf("GetStats() = %+v, want zero value on error", got)
	}
}
