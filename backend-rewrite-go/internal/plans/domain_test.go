package plans

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPlanEntity_JSONShapeAndOrder(t *testing.T) {
	cur := "USD"
	sp := "price_123"
	p := PlanEntity{
		ID: "11111111-1111-1111-1111-111111111111", Name: "Pro",
		DailyLimit: 100, PriceCents: 499, Currency: &cur, StripePriceID: &sp,
		TrialDays: 30, BillingPeriod: "monthly", IsActive: true,
		CreatedAt: time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC),
	}
	b, _ := json.Marshal(p)
	want := `{"id":"11111111-1111-1111-1111-111111111111","name":"Pro","daily_limit":100,"price_cents":499,"currency":"USD","stripe_price_id":"price_123","trial_days":30,"billing_period":"monthly","is_active":true,"created_at":"2026-06-14T09:00:00.000Z","updated_at":"2026-06-14T09:00:00.000Z"}`
	if string(b) != want {
		t.Fatalf("\n got %s\nwant %s", b, want)
	}
}

func TestPlanEntity_NullCurrency(t *testing.T) {
	p := PlanEntity{ID: "x", BillingPeriod: "none"}
	b, _ := json.Marshal(p)
	if !strings.Contains(string(b), `"currency":null`) || !strings.Contains(string(b), `"stripe_price_id":null`) {
		t.Fatalf("got %s", b)
	}
}
