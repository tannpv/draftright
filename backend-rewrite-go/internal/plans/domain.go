package plans

import (
	"encoding/json"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// PlanEntity is the raw plan row as GET /plans serialises it: snake_case,
// in entity-declaration order, price_cents raw (not /100), nullable
// currency/stripe_price_id. MarshalJSON pins field order + ms timestamps.
type PlanEntity struct {
	ID            string
	Name          string
	DailyLimit    int
	PriceCents    int
	Currency      *string
	StripePriceID *string
	TrialDays     int
	BillingPeriod string
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewPlan is the admin create payload — mirrors Node POST /admin/plans
// @Body { name, daily_limit, price_cents, billing_period, currency?,
// trial_days?, stripe_price_id? }. is_active is not a create field (the DB
// default true applies). Defaulting of absent optionals (trial_days,
// currency, billing_period) happens in the usecase/handler; the repo binds
// whatever NewPlan carries.
type NewPlan struct {
	Name          string
	DailyLimit    int
	PriceCents    int
	BillingPeriod string
	Currency      *string
	TrialDays     int
	StripePriceID *string
}

// PlanPatch is the admin update payload — mirrors Node PATCH
// /admin/plans/:id @Body Partial<{ name, daily_limit, price_cents,
// billing_period, is_active, currency, trial_days, stripe_price_id }>. A
// nil pointer = field unchanged (TypeORM partial .update()).
type PlanPatch struct {
	Name          *string
	DailyLimit    *int
	PriceCents    *int
	BillingPeriod *string
	Currency      *string
	TrialDays     *int
	StripePriceID *string
	IsActive      *bool
}

func (p PlanEntity) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID            string  `json:"id"`
		Name          string  `json:"name"`
		DailyLimit    int     `json:"daily_limit"`
		PriceCents    int     `json:"price_cents"`
		Currency      *string `json:"currency"`
		StripePriceID *string `json:"stripe_price_id"`
		TrialDays     int     `json:"trial_days"`
		BillingPeriod string  `json:"billing_period"`
		IsActive      bool    `json:"is_active"`
		CreatedAt     string  `json:"created_at"`
		UpdatedAt     string  `json:"updated_at"`
	}{
		ID: p.ID, Name: p.Name, DailyLimit: p.DailyLimit, PriceCents: p.PriceCents,
		Currency: p.Currency, StripePriceID: p.StripePriceID, TrialDays: p.TrialDays,
		BillingPeriod: p.BillingPeriod, IsActive: p.IsActive,
		CreatedAt: shared.ISOMillis(p.CreatedAt), UpdatedAt: shared.ISOMillis(p.UpdatedAt),
	})
}
