package subscription

import "time"

// NudgeBanner mirrors the NestJS NudgeBanner enum string values.
type NudgeBanner string

const (
	BannerNone        NudgeBanner = "none"
	BannerProExpiring NudgeBanner = "pro_expiring"
	BannerJustExpired NudgeBanner = "just_expired"
	BannerFreeCounter NudgeBanner = "free_counter"
)

// Banner-window thresholds from nudge.ts. Comparisons inclusive (<=),
// arithmetic on unrounded day-fraction floats (matches the TS).
const (
	dayMS           = float64(86_400_000)
	proExpiringDays = 3.0
	justExpiredDays = 7.0
	// FreeDailyLimit is the free-tier dailyLimit fallback when no plan applies.
	FreeDailyLimit = 10
)

// DeriveBanner ports subscriptions/nudge.ts deriveBanner. usageToday is
// intentionally absent — the TS signature ignores it. tier is "pro" or
// "free"; expiresAt is the active sub's expiry (pro path); lastExpiredAt
// is the updated_at of the most-recent expired row (free path).
func DeriveBanner(tier string, expiresAt, lastExpiredAt *time.Time, now time.Time) NudgeBanner {
	if tier == "pro" {
		if expiresAt != nil {
			daysLeft := float64(expiresAt.Sub(now).Milliseconds()) / dayMS
			if daysLeft <= proExpiringDays {
				return BannerProExpiring
			}
		}
		return BannerNone
	}
	if lastExpiredAt != nil {
		daysSince := float64(now.Sub(*lastExpiredAt).Milliseconds()) / dayMS
		if daysSince <= justExpiredDays {
			return BannerJustExpired
		}
	}
	return BannerFreeCounter
}
