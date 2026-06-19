package plans_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tannpv/draftright-rewrite/internal/plans"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

func TestListActiveEmptyReturnsNonNilEmptySlice(t *testing.T) {
	r := plans.NewReader(fakeQ{rows: nil})

	out, err := r.ListActive(context.Background())
	if err != nil {
		t.Fatalf("ListActive err: %v", err)
	}
	if out == nil {
		t.Fatal("ListActive returned nil slice; want non-nil empty (so handler emits [])")
	}
	if len(out) != 0 {
		t.Fatalf("len = %d; want 0", len(out))
	}
}

func TestListActiveMapsRow(t *testing.T) {
	var id pgtype.UUID
	if err := id.Scan("11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("scan uuid: %v", err)
	}
	usd := "USD"
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	rows := []sqlc.ListActivePlansRow{{
		ID:            id,
		Name:          "Pro",
		DailyLimit:    100,
		PriceCents:    499,
		Currency:      &usd,
		StripePriceID: nil,
		TrialDays:     7,
		BillingPeriod: sqlc.PlansBillingPeriodEnumMonthly,
		IsActive:      true,
		CreatedAt:     pgtype.Timestamp{Time: now, Valid: true},
		UpdatedAt:     pgtype.Timestamp{Time: now, Valid: true},
	}}

	r := plans.NewReader(fakeQ{rows: rows})

	out, err := r.ListActive(context.Background())
	if err != nil {
		t.Fatalf("ListActive err: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len = %d; want 1", len(out))
	}
	p := out[0]
	if p.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q; want dashed uuid", p.ID)
	}
	if p.PriceCents != 499 {
		t.Errorf("PriceCents = %d; want 499 (raw, not /100)", p.PriceCents)
	}
	if p.Currency == nil || *p.Currency != "USD" {
		t.Errorf("Currency = %v; want *USD", p.Currency)
	}
	if p.BillingPeriod != "monthly" {
		t.Errorf("BillingPeriod = %q; want monthly", p.BillingPeriod)
	}
	if p.Name != "Pro" || p.DailyLimit != 100 || p.TrialDays != 7 || !p.IsActive {
		t.Errorf("unexpected scalar mapping: %+v", p)
	}

	// Service path delegates to the reader and returns the same.
	svc := plans.NewService(r)
	sout, err := svc.ListActive(context.Background())
	if err != nil {
		t.Fatalf("Service.ListActive err: %v", err)
	}
	if len(sout) != 1 || sout[0].ID != p.ID || sout[0].PriceCents != p.PriceCents {
		t.Fatalf("Service path diverged from reader: %+v vs %+v", sout, out)
	}
}
