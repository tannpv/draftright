package subscription_test

import (
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/subscription"
)

func TestDeriveBanner(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	future := func(days float64) *time.Time {
		x := now.Add(time.Duration(days * float64(24*time.Hour)))
		return &x
	}
	past := func(days float64) *time.Time {
		x := now.Add(-time.Duration(days * float64(24*time.Hour)))
		return &x
	}
	cases := []struct {
		name          string
		tier          string
		expiresAt     *time.Time
		lastExpiredAt *time.Time
		want          subscription.NudgeBanner
	}{
		{"pro expiring in 2d", "pro", future(2), nil, subscription.BannerProExpiring},
		{"pro expiring exactly 3d (inclusive)", "pro", future(3), nil, subscription.BannerProExpiring},
		{"pro 4d away → none", "pro", future(4), nil, subscription.BannerNone},
		{"pro no expiry → none", "pro", nil, nil, subscription.BannerNone},
		{"free just expired 2d ago", "free", nil, past(2), subscription.BannerJustExpired},
		{"free expired exactly 7d ago (inclusive)", "free", nil, past(7), subscription.BannerJustExpired},
		{"free expired 8d ago → counter", "free", nil, past(8), subscription.BannerFreeCounter},
		{"free never expired → counter", "free", nil, nil, subscription.BannerFreeCounter},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := subscription.DeriveBanner(c.tier, c.expiresAt, c.lastExpiredAt, now); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
