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

	withPlanRow sqlc.GetActiveSubWithPlanRow
	withPlanErr error

	lastExpired    pgtype.Timestamp
	lastExpiredErr error
}

func (f fakeQ) GetActiveSubscriptionByUserID(ctx context.Context, id pgtype.UUID) (sqlc.GetActiveSubscriptionByUserIDRow, error) {
	return f.row, f.err
}

func (f fakeQ) GetActiveSubWithPlan(ctx context.Context, id pgtype.UUID) (sqlc.GetActiveSubWithPlanRow, error) {
	return f.withPlanRow, f.withPlanErr
}

func (f fakeQ) GetLastExpiredAt(ctx context.Context, id pgtype.UUID) (pgtype.Timestamp, error) {
	return f.lastExpired, f.lastExpiredErr
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

func TestActiveWithPlan_NoRow_Nil(t *testing.T) {
	r := sub.NewReader(fakeQ{withPlanErr: pgx.ErrNoRows})
	v, err := r.ActiveWithPlan(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil || v != nil {
		t.Fatalf("want nil,nil got %v,%v", v, err)
	}
}

func TestActiveWithPlan_Found(t *testing.T) {
	exp := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	r := sub.NewReader(fakeQ{withPlanRow: sqlc.GetActiveSubWithPlanRow{
		Status:        "active",
		ExpiresAt:     pgtype.Timestamp{Time: exp, Valid: true},
		PlanName:      "Pro",
		DailyLimit:    100,
		BillingPeriod: "monthly",
	}})
	v, err := r.ActiveWithPlan(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if v == nil || v.Status != "active" || v.PlanName != "Pro" || v.DailyLimit != 100 {
		t.Fatalf("bad view: %+v", v)
	}
	if v.BillingPeriod != "monthly" {
		t.Fatalf("bad billing period: %+v", v)
	}
	if v.ExpiresAt == nil || !v.ExpiresAt.Equal(exp) {
		t.Fatalf("expected expiry %v, got %v", exp, v.ExpiresAt)
	}
}

func TestActiveWithPlan_NullExpiry(t *testing.T) {
	r := sub.NewReader(fakeQ{withPlanRow: sqlc.GetActiveSubWithPlanRow{
		Status:        "active",
		PlanName:      "Free",
		DailyLimit:    10,
		BillingPeriod: "none",
	}})
	v, err := r.ActiveWithPlan(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if v == nil || v.ExpiresAt != nil {
		t.Fatalf("expected non-nil view with nil ExpiresAt, got %+v", v)
	}
}

func TestLastExpiredAt_NoRow_Nil(t *testing.T) {
	r := sub.NewReader(fakeQ{lastExpiredErr: pgx.ErrNoRows})
	tm, err := r.LastExpiredAt(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil || tm != nil {
		t.Fatalf("want nil,nil got %v,%v", tm, err)
	}
}

func TestLastExpiredAt_Found(t *testing.T) {
	at := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	r := sub.NewReader(fakeQ{lastExpired: pgtype.Timestamp{Time: at, Valid: true}})
	tm, err := r.LastExpiredAt(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if tm == nil || !tm.Equal(at) {
		t.Fatalf("expected %v, got %v", at, tm)
	}
}

func TestLastExpiredAt_NotValid_Nil(t *testing.T) {
	r := sub.NewReader(fakeQ{lastExpired: pgtype.Timestamp{Valid: false}})
	tm, err := r.LastExpiredAt(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil || tm != nil {
		t.Fatalf("want nil,nil got %v,%v", tm, err)
	}
}
