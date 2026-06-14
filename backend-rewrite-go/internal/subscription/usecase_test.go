package subscription_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/subscription"
)

// stubReader satisfies subscription.SubReader + UsageCounter.
type stubReader struct {
	sub         *subscription.SubView
	lastExpired *time.Time
	usageToday  int
}

func (s *stubReader) ActiveWithPlan(ctx context.Context, userID string) (*subscription.SubView, error) {
	return s.sub, nil
}

func (s *stubReader) LastExpiredAt(ctx context.Context, userID string) (*time.Time, error) {
	return s.lastExpired, nil
}

func (s *stubReader) CountToday(ctx context.Context, userID string) (int, error) {
	return s.usageToday, nil
}

func newSubSvc(t *testing.T, stub *stubReader) *subscription.Service {
	t.Helper()
	return subscription.NewService(stub, stub)
}

func TestGetSubscription_ProPath(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	exp := now.Add(2 * 24 * time.Hour)
	stub := &stubReader{
		sub: &subscription.SubView{
			Status:        "active",
			ExpiresAt:     &exp,
			PlanName:      "Pro",
			DailyLimit:    1000,
			BillingPeriod: "monthly",
		},
		usageToday: 7,
	}
	view, err := newSubSvc(t, stub).GetSubscription(context.Background(), "u1", now)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if view.Plan == nil || view.Plan.Name != "Pro" {
		t.Fatalf("Plan.Name = %v, want Pro", view.Plan)
	}
	if view.Nudge.Tier != "pro" {
		t.Fatalf("Nudge.Tier = %q, want pro", view.Nudge.Tier)
	}
	if view.Nudge.Banner != subscription.BannerProExpiring {
		t.Fatalf("Nudge.Banner = %q, want pro_expiring", view.Nudge.Banner)
	}
	if view.UsageToday != 7 {
		t.Fatalf("UsageToday = %d, want 7", view.UsageToday)
	}
	if view.Nudge.DailyLimit != 1000 {
		t.Fatalf("Nudge.DailyLimit = %d, want 1000", view.Nudge.DailyLimit)
	}
}

func TestGetSubscription_NoSubFreePath(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	stub := &stubReader{sub: nil, lastExpired: nil, usageToday: 3}
	view, err := newSubSvc(t, stub).GetSubscription(context.Background(), "u1", now)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if view.Plan != nil {
		t.Fatalf("Plan = %v, want nil", view.Plan)
	}
	if view.Status != nil {
		t.Fatalf("Status = %v, want nil", view.Status)
	}
	if view.ExpiresAt != nil {
		t.Fatalf("ExpiresAt = %v, want nil", view.ExpiresAt)
	}
	if view.Nudge.Tier != "free" {
		t.Fatalf("Nudge.Tier = %q, want free", view.Nudge.Tier)
	}
	if view.Nudge.DailyLimit != subscription.FreeDailyLimit {
		t.Fatalf("Nudge.DailyLimit = %d, want %d", view.Nudge.DailyLimit, subscription.FreeDailyLimit)
	}
	if view.Nudge.Banner != subscription.BannerFreeCounter {
		t.Fatalf("Nudge.Banner = %q, want free_counter", view.Nudge.Banner)
	}
}

func TestVerifyReceipt_EmptyPlanNameOmitted(t *testing.T) {
	exp := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	stub := &stubReader{
		sub: &subscription.SubView{
			Status:        "active",
			ExpiresAt:     &exp,
			PlanName:      "",
			DailyLimit:    1000,
			BillingPeriod: "monthly",
		},
	}
	receipt, err := newSubSvc(t, stub).VerifyReceipt(context.Background(), "u1")
	if err != nil {
		t.Fatalf("VerifyReceipt: %v", err)
	}
	b, err := json.Marshal(receipt)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(b), "\"plan\"") {
		t.Fatalf("JSON contains plan key, want omitted: %s", b)
	}
}
