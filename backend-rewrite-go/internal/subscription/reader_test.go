package subscription_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	sub "github.com/tannpv/draftright-rewrite/internal/subscription"
)

type fakeQ struct {
	row sqlc.GetActiveSubscriptionByUserIDRow
	err error
}

func (f fakeQ) GetActiveSubscriptionByUserID(ctx context.Context, id pgtype.UUID) (sqlc.GetActiveSubscriptionByUserIDRow, error) {
	return f.row, f.err
}

func TestActiveByUser_Found(t *testing.T) {
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r := sub.NewReader(fakeQ{row: sqlc.GetActiveSubscriptionByUserIDRow{
		Status: "active", StoreType: "lemonsqueezy",
		StartedAt: pgtype.Timestamp{Time: started, Valid: true},
		PlanName:  "Pro", DailyLimit: 100,
	}})
	got, err := r.ActiveByUser(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.PlanName != "Pro" || got.DailyLimit != 100 || got.Status != "active" {
		t.Fatalf("bad sub: %+v", got)
	}
	if got.StoreType != "lemonsqueezy" || !got.StartedAt.Equal(started) {
		t.Fatalf("bad fields: %+v", got)
	}
	if got.ExpiresAt != nil {
		t.Fatalf("expected nil ExpiresAt, got %v", got.ExpiresAt)
	}
}

func TestActiveByUser_WithExpiry(t *testing.T) {
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	r := sub.NewReader(fakeQ{row: sqlc.GetActiveSubscriptionByUserIDRow{
		Status: "active", StoreType: "stripe",
		StartedAt: pgtype.Timestamp{Time: started, Valid: true},
		ExpiresAt: pgtype.Timestamp{Time: exp, Valid: true},
		PlanName:  "Pro", DailyLimit: 100,
	}})
	got, err := r.ActiveByUser(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("expected expiry %v, got %v", exp, got.ExpiresAt)
	}
}

func TestActiveByUser_None(t *testing.T) {
	r := sub.NewReader(fakeQ{err: pgx.ErrNoRows})
	got, err := r.ActiveByUser(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("want nil sub, got %+v", got)
	}
}
