package subscription

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
