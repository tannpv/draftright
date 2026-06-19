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

// fakeFreePlans satisfies subscription.FreePlanReader.
type fakeFreePlans struct {
	limit int
	found bool
	err   error
}

func (f fakeFreePlans) FreePlanDailyLimit(ctx context.Context) (int, bool, error) {
	return f.limit, f.found, f.err
}

func newSubSvc(t *testing.T, stub *stubReader) *subscription.Service {
	t.Helper()
	// Default: Free plan missing → free path falls back to FreeDailyLimit (10).
	return subscription.NewService(stub, stub, fakeFreePlans{found: false})
}

func newSubSvcWithFree(t *testing.T, stub *stubReader, free fakeFreePlans) *subscription.Service {
	t.Helper()
	return subscription.NewService(stub, stub, free)
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

// Bug 1: free path dailyLimit comes from the Free plan row, not hardcoded 10.
func TestGetSubscription_FreePath_UsesFreePlanDailyLimit(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	stub := &stubReader{sub: nil, lastExpired: nil, usageToday: 3}
	view, err := newSubSvcWithFree(t, stub, fakeFreePlans{limit: 20, found: true}).
		GetSubscription(context.Background(), "u1", now)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if view.Nudge.DailyLimit != 20 {
		t.Fatalf("Nudge.DailyLimit = %d, want 20 (from Free plan row)", view.Nudge.DailyLimit)
	}
}

// Bug 1: Free plan row missing → falls back to FreeDailyLimit (10).
func TestGetSubscription_FreePath_MissingFreePlanFallsBackTo10(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	stub := &stubReader{sub: nil, lastExpired: nil, usageToday: 3}
	view, err := newSubSvcWithFree(t, stub, fakeFreePlans{found: false}).
		GetSubscription(context.Background(), "u1", now)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if view.Nudge.DailyLimit != subscription.FreeDailyLimit {
		t.Fatalf("Nudge.DailyLimit = %d, want %d (fallback)", view.Nudge.DailyLimit, subscription.FreeDailyLimit)
	}
}

// Bug 2: a free-tier user with an active billing_period='none' sub that carries
// an expires_at → top-level expires_at present, but nudge.expiresAt is nil.
func TestGetSubscription_FreePath_NudgeExpiresAtNil_TopLevelPresent(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	exp := now.Add(5 * 24 * time.Hour)
	stub := &stubReader{
		sub: &subscription.SubView{
			Status:        "active",
			ExpiresAt:     &exp,
			PlanName:      "Free",
			DailyLimit:    10,
			BillingPeriod: "none",
		},
		usageToday: 3,
	}
	view, err := newSubSvc(t, stub).GetSubscription(context.Background(), "u1", now)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if view.Nudge.Tier != "free" {
		t.Fatalf("Nudge.Tier = %q, want free", view.Nudge.Tier)
	}
	if view.ExpiresAt == nil {
		t.Fatalf("top-level ExpiresAt = nil, want present (mirrors raw sub expiry)")
	}
	if view.Nudge.ExpiresAt != nil {
		t.Fatalf("Nudge.ExpiresAt = %v, want nil on free path", *view.Nudge.ExpiresAt)
	}
}

// Bug 2: pro path keeps nudge.expiresAt set.
func TestGetSubscription_ProPath_NudgeExpiresAtPresent(t *testing.T) {
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
		usageToday: 1,
	}
	view, err := newSubSvc(t, stub).GetSubscription(context.Background(), "u1", now)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if view.Nudge.ExpiresAt == nil {
		t.Fatalf("Nudge.ExpiresAt = nil, want present on pro path")
	}
}

func TestResolveDailyLimit_Pro_UsesPlanLimit(t *testing.T) {
	stub := &stubReader{
		sub: &subscription.SubView{
			Status:        "active",
			PlanName:      "Pro",
			DailyLimit:    1000,
			BillingPeriod: "monthly",
		},
	}
	limit, err := newSubSvc(t, stub).ResolveDailyLimit(context.Background(), "u1")
	if err != nil {
		t.Fatalf("ResolveDailyLimit: %v", err)
	}
	if limit != 1000 {
		t.Fatalf("limit = %d, want 1000", limit)
	}
}

func TestResolveDailyLimit_ProUnlimited(t *testing.T) {
	stub := &stubReader{
		sub: &subscription.SubView{
			Status:        "active",
			PlanName:      "Pro",
			DailyLimit:    -1,
			BillingPeriod: "yearly",
		},
	}
	limit, err := newSubSvc(t, stub).ResolveDailyLimit(context.Background(), "u1")
	if err != nil {
		t.Fatalf("ResolveDailyLimit: %v", err)
	}
	if limit != -1 {
		t.Fatalf("limit = %d, want -1 (unlimited sentinel)", limit)
	}
}

func TestResolveDailyLimit_Free_UsesFreePlanRow(t *testing.T) {
	stub := &stubReader{sub: nil}
	limit, err := newSubSvcWithFree(t, stub, fakeFreePlans{limit: 20, found: true}).
		ResolveDailyLimit(context.Background(), "u1")
	if err != nil {
		t.Fatalf("ResolveDailyLimit: %v", err)
	}
	if limit != 20 {
		t.Fatalf("limit = %d, want 20 (from Free plan row)", limit)
	}
}

func TestResolveDailyLimit_Free_FallbackWhenRowMissing(t *testing.T) {
	stub := &stubReader{sub: nil}
	limit, err := newSubSvcWithFree(t, stub, fakeFreePlans{found: false}).
		ResolveDailyLimit(context.Background(), "u1")
	if err != nil {
		t.Fatalf("ResolveDailyLimit: %v", err)
	}
	if limit != subscription.FreeDailyLimit {
		t.Fatalf("limit = %d, want %d (fallback)", limit, subscription.FreeDailyLimit)
	}
}

func TestResolveDailyLimit_NoneBillingTreatedAsFree(t *testing.T) {
	stub := &stubReader{
		sub: &subscription.SubView{
			Status:        "active",
			PlanName:      "Free",
			DailyLimit:    1000,
			BillingPeriod: "none",
		},
	}
	limit, err := newSubSvcWithFree(t, stub, fakeFreePlans{limit: 20, found: true}).
		ResolveDailyLimit(context.Background(), "u1")
	if err != nil {
		t.Fatalf("ResolveDailyLimit: %v", err)
	}
	if limit != 20 {
		t.Fatalf("limit = %d, want 20 (billing_period='none' takes free branch, not 1000)", limit)
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
