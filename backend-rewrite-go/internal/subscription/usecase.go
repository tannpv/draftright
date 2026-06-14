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

// Service is the subscription read use case (GET + verify-receipt).
type Service struct {
	r     SubReader
	usage UsageCounter
}

// NewService wires the reader + usage counter.
func NewService(r SubReader, usage UsageCounter) *Service {
	return &Service{r: r, usage: usage}
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
	var expForBanner *time.Time
	if sub != nil {
		if sub.BillingPeriod != "none" {
			tier = "pro"
		}
		view.Plan = &PlanBrief{Name: sub.PlanName, DailyLimit: sub.DailyLimit, BillingPeriod: sub.BillingPeriod}
		st := sub.Status
		view.Status = &st
		if sub.ExpiresAt != nil {
			iso := shared.ISOMillis(*sub.ExpiresAt)
			view.ExpiresAt = &iso
			expForBanner = sub.ExpiresAt
		}
	}

	dailyLimit := FreeDailyLimit
	if sub != nil {
		dailyLimit = sub.DailyLimit
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
		Banner:     DeriveBanner(tier, expForBanner, lastExpired, now),
	}
	if view.ExpiresAt != nil {
		e := *view.ExpiresAt
		nudge.ExpiresAt = &e
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
