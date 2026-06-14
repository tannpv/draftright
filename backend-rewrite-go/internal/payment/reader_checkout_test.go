package payment

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

func TestPlanForCheckout_NoRow(t *testing.T) {
	r := NewRepo(fakeQ{planErr: pgx.ErrNoRows})
	p, err := r.PlanForCheckout(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil || p != nil {
		t.Fatalf("no row → (nil,nil), got p=%v err=%v", p, err)
	}
}

func TestPlanForCheckout_Maps(t *testing.T) {
	price := "usd-price-id"
	r := NewRepo(fakeQ{plan: sqlc.GetPlanForCheckoutRow{
		Name: "Pro", PriceCents: 999, StripePriceID: &price, TrialDays: 7,
		BillingPeriod: sqlc.PlansBillingPeriodEnum("monthly"),
	}})
	p, err := r.PlanForCheckout(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Pro" || p.PriceCents != 999 || p.StripePriceID != "usd-price-id" || p.TrialDays != 7 {
		t.Fatalf("plan mapped wrong: %+v", p)
	}
	if p.BillingPeriod != "monthly" {
		t.Fatalf("billing_period mapped wrong: %+v", p)
	}
}

func TestUserForCheckout_NoRow(t *testing.T) {
	r := NewRepo(fakeQ{userErr: pgx.ErrNoRows})
	u, err := r.UserForCheckout(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil || u != nil {
		t.Fatalf("no row → (nil,nil), got u=%v err=%v", u, err)
	}
}

func TestUserForCheckout_Maps(t *testing.T) {
	cust := "cus_123"
	r := NewRepo(fakeQ{user: sqlc.GetUserForCheckoutRow{
		Email: "a@b.com", StripeCustomerID: &cust,
	}})
	u, err := r.UserForCheckout(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "a@b.com" || u.StripeCustomerID != "cus_123" || u.LemonSqueezyCustomerID != "" {
		t.Fatalf("user mapped wrong: %+v", u)
	}
}
