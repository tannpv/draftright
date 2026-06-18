package plans

import "testing"

func sptr(s string) *string { return &s }
func iptr(i int) *int       { return &i }
func bptr(b bool) *bool     { return &b }

// plansPatchSQL must walk fields in domain order and pin updated_at last,
// regardless of which subset is non-nil. name before price_cents proves the
// ordering is deterministic (not map iteration).
func TestPlansPatchSQL_OnlyNonNilFields(t *testing.T) {
	set, args := plansPatchSQL(PlanPatch{Name: sptr("Pro"), PriceCents: iptr(900)})
	if set != "name = $1, price_cents = $2, updated_at = now()" {
		t.Fatalf("set=%q", set)
	}
	if len(args) != 2 {
		t.Fatalf("args=%v", args)
	}
	if args[0] != "Pro" || args[1] != 900 {
		t.Fatalf("args=%v", args)
	}
}

// All fields set: confirms the full walk order — name, daily_limit,
// price_cents, billing_period, currency, trial_days, stripe_price_id,
// is_active — with updated_at appended.
func TestPlansPatchSQL_AllFieldsInOrder(t *testing.T) {
	set, args := plansPatchSQL(PlanPatch{
		Name:          sptr("Pro"),
		DailyLimit:    iptr(100),
		PriceCents:    iptr(900),
		BillingPeriod: sptr("monthly"),
		Currency:      sptr("USD"),
		TrialDays:     iptr(7),
		StripePriceID: sptr("price_x"),
		IsActive:      bptr(true),
	})
	want := "name = $1, daily_limit = $2, price_cents = $3, billing_period = $4, " +
		"currency = $5, trial_days = $6, stripe_price_id = $7, is_active = $8, updated_at = now()"
	if set != want {
		t.Fatalf("set=%q\nwant=%q", set, want)
	}
	if len(args) != 8 {
		t.Fatalf("args=%v", args)
	}
}

// paginatedSQL must project all 11 cols (billing_period forced to ::text)
// from plans, with the listquery WHERE/ORDER spliced in and LIMIT/OFFSET
// left as %d placeholders for positional filling.
func TestPlansPaginatedSQL_UsesListqueryBuilt(t *testing.T) {
	where := "WHERE (name ILIKE $1)"
	order := "ORDER BY created_at DESC"
	got := plansPaginatedSQL(where, order)
	want := "SELECT id, name, daily_limit, price_cents, currency, stripe_price_id, " +
		"trial_days, billing_period::text, is_active, created_at, updated_at FROM plans " +
		"WHERE (name ILIKE $1) ORDER BY created_at DESC LIMIT $%d OFFSET $%d"
	if got != want {
		t.Fatalf("plansPaginatedSQL mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestPlansCountSQL_SharesWhere(t *testing.T) {
	got := plansCountSQL("WHERE (name ILIKE $1)")
	want := "SELECT COUNT(*) FROM plans WHERE (name ILIKE $1)"
	if got != want {
		t.Fatalf("plansCountSQL mismatch:\n got=%q\nwant=%q", got, want)
	}
}
