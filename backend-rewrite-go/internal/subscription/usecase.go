package subscription

import (
	"context"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// SubReader is the consumer-side port for the active sub + last-expired.
type SubReader interface {
	ActiveWithPlan(ctx context.Context, userID string) (*SubView, error)
	LastExpiredAt(ctx context.Context, userID string) (*time.Time, error)
}

// UsageCounter is the consumer-side port for today's usage count.
// Satisfied by *usage.Counter (extend, don't fork).
type UsageCounter interface {
	CountToday(ctx context.Context, userID string) (int, error)
}

// FreePlanReader yields the Free plan's daily limit (Node:
// plansService.findFreePlan().daily_limit). found=false means the Free
// plan row is missing (bad seed) → caller falls back to FreeDailyLimit.
// Satisfied structurally by *plans.Reader (extend, don't fork).
type FreePlanReader interface {
	FreePlanDailyLimit(ctx context.Context) (int, bool, error)
}

// Service is the subscription read use case (GET + verify-receipt).
type Service struct {
	r         SubReader
	usage     UsageCounter
	freePlans FreePlanReader
}

// NewService wires the reader + usage counter + free-plan reader.
func NewService(r SubReader, usage UsageCounter, freePlans FreePlanReader) *Service {
	return &Service{r: r, usage: usage, freePlans: freePlans}
}

// GetSubscription builds GET /subscription's view. now is injected for tests.
func (s *Service) GetSubscription(ctx context.Context, userID string, now time.Time) (SubscriptionView, error) {
	sub, err := s.r.ActiveWithPlan(ctx, userID)
	if err != nil {
		return SubscriptionView{}, err
	}
	usageToday, err := s.usage.CountToday(ctx, userID)
	if err != nil {
		return SubscriptionView{}, err
	}
	view := SubscriptionView{UsageToday: usageToday}

	tier := "free"
	if sub != nil {
		if sub.BillingPeriod != "none" {
			tier = "pro"
		}
		view.Plan = &PlanBrief{Name: sub.PlanName, DailyLimit: sub.DailyLimit, BillingPeriod: sub.BillingPeriod}
		st := sub.Status
		view.Status = &st
		// Top-level expires_at mirrors the raw sub expiry on BOTH paths
		// (Node controller reads sub.expires_at directly). The nudge's
		// expiresAt is path-dependent — handled below via the entitlement.
		if sub.ExpiresAt != nil {
			iso := shared.ISOMillis(*sub.ExpiresAt)
			view.ExpiresAt = &iso
		}
	}

	// Entitlement expiry (Node resolveEntitlement): pro path carries the
	// sub's expiry; free path FORCES it to null even when a non-pro active
	// sub (billing_period='none') has an expires_at. The nudge reads this.
	var entExpiresAt *time.Time
	if tier == "pro" && sub != nil {
		entExpiresAt = sub.ExpiresAt
	}

	// Free-path dailyLimit comes from the Free plan row (Node:
	// findFreePlan().daily_limit), with FreeDailyLimit as the fallback
	// when the row is missing. Pro path keeps the sub's own limit.
	dailyLimit := FreeDailyLimit
	if sub != nil && tier == "pro" {
		dailyLimit = sub.DailyLimit
	} else {
		limit, found, err := s.freePlans.FreePlanDailyLimit(ctx)
		if err != nil {
			return SubscriptionView{}, err
		}
		if found {
			dailyLimit = limit
		}
	}

	var lastExpired *time.Time
	if tier == "free" {
		lastExpired, err = s.r.LastExpiredAt(ctx, userID)
		if err != nil {
			return SubscriptionView{}, err
		}
	}

	nudge := Nudge{
		Tier:       tier,
		UsageToday: usageToday,
		DailyLimit: dailyLimit,
		Banner:     DeriveBanner(tier, entExpiresAt, lastExpired, now),
	}
	if entExpiresAt != nil {
		iso := shared.ISOMillis(*entExpiresAt)
		nudge.ExpiresAt = &iso
	}
	view.Nudge = nudge
	return view, nil
}

// VerifyReceipt mirrors POST /subscription/verify-receipt: ignores the body,
// returns {subscription:…|null} from the active sub.
func (s *Service) VerifyReceipt(ctx context.Context, userID string) (ReceiptView, error) {
	sub, err := s.r.ActiveWithPlan(ctx, userID)
	if err != nil {
		return ReceiptView{}, err
	}
	if sub == nil {
		return ReceiptView{Subscription: nil}, nil
	}
	rs := &ReceiptSub{Plan: sub.PlanName, Status: sub.Status}
	if sub.ExpiresAt != nil {
		iso := shared.ISOMillis(*sub.ExpiresAt)
		rs.ExpiresAt = &iso
	}
	return ReceiptView{Subscription: rs}, nil
}
