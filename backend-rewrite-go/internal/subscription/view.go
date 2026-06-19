package subscription

import (
	"encoding/json"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// GrantedSub is the POST /admin/subscriptions/grant response. Key order
// mirrors the Node Subscription entity.
type GrantedSub struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"user_id"`
	PlanID             string     `json:"plan_id"`
	Status             string     `json:"status"`
	StoreType          string     `json:"store_type"`
	StoreTransactionID *string    `json:"store_transaction_id"`
	StartedAt          time.Time  `json:"started_at"`
	ExpiresAt          *time.Time `json:"expires_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// MarshalJSON pins GrantedSub to Node's Subscription entity field order and
// timestamp format (ISOMillis — 3-digit millis with trailing "Z"). Go's
// default time.Time marshaling emits microseconds (and drops ".000" on
// whole seconds), which breaks byte-parity with TypeORM's Date.toJSON().
// expires_at is null when nil. Mirrors TransactionRow.MarshalJSON.
func (g GrantedSub) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID                 string  `json:"id"`
		UserID             string  `json:"user_id"`
		PlanID             string  `json:"plan_id"`
		Status             string  `json:"status"`
		StoreType          string  `json:"store_type"`
		StoreTransactionID *string `json:"store_transaction_id"`
		StartedAt          string  `json:"started_at"`
		ExpiresAt          *string `json:"expires_at"`
		CreatedAt          string  `json:"created_at"`
		UpdatedAt          string  `json:"updated_at"`
	}
	w := wire{
		ID:                 g.ID,
		UserID:             g.UserID,
		PlanID:             g.PlanID,
		Status:             g.Status,
		StoreType:          g.StoreType,
		StoreTransactionID: g.StoreTransactionID,
		StartedAt:          shared.ISOMillis(g.StartedAt),
		CreatedAt:          shared.ISOMillis(g.CreatedAt),
		UpdatedAt:          shared.ISOMillis(g.UpdatedAt),
	}
	if g.ExpiresAt != nil {
		s := shared.ISOMillis(*g.ExpiresAt)
		w.ExpiresAt = &s
	}
	return json.Marshal(w)
}

// SubscriptionView is GET /subscription's 200 body.
type SubscriptionView struct {
	Plan       *PlanBrief `json:"plan"`
	Status     *string    `json:"status"`
	ExpiresAt  *string    `json:"expires_at"`
	UsageToday int        `json:"usage_today"`
	Nudge      Nudge      `json:"nudge"`
}

// PlanBrief is the trimmed plan shape in the subscription view.
type PlanBrief struct {
	Name          string `json:"name"`
	DailyLimit    int    `json:"daily_limit"`
	BillingPeriod string `json:"billing_period"`
}

// Nudge is the banner sub-object (camelCase keys, matching nudge.ts).
type Nudge struct {
	Tier       string      `json:"tier"`
	UsageToday int         `json:"usageToday"`
	DailyLimit int         `json:"dailyLimit"`
	ExpiresAt  *string     `json:"expiresAt"`
	Banner     NudgeBanner `json:"banner"`
}

// ReceiptView is POST verify-receipt's 201 body.
type ReceiptView struct {
	Subscription *ReceiptSub `json:"subscription"`
}

// ReceiptSub mirrors Node's verify-receipt subscription object. Plan is
// omitted when empty (sub.plan?.name → undefined → key absent).
type ReceiptSub struct {
	Plan      string  `json:"plan,omitempty"`
	Status    string  `json:"status"`
	ExpiresAt *string `json:"expires_at"`
}
