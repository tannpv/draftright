package subscription_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
)

type fakeWQ struct {
	called bool
	arg    sqlc.CreateFreeSubscriptionParams
}

func (f *fakeWQ) CreateFreeSubscription(_ context.Context, a sqlc.CreateFreeSubscriptionParams) error {
	f.called = true
	f.arg = a
	return nil
}

func TestCreateFree(t *testing.T) {
	q := &fakeWQ{}
	w := subscription.NewWriter(q)
	if err := w.CreateFree(context.Background(),
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatal(err)
	}
	if !q.called {
		t.Fatal("query not called")
	}
	var want pgtype.UUID
	_ = want.Scan("11111111-1111-1111-1111-111111111111")
	if q.arg.UserID != want {
		t.Fatalf("user id not bound: %v", q.arg.UserID)
	}
}
