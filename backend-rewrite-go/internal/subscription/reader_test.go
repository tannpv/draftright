package subscription_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	sub "github.com/tannpv/draftright-rewrite/internal/subscription"
)

type fakeQ struct {
	row sqlc.GetActiveSubscriptionByUserIDRow
	err error

	withPlanRow sqlc.GetActiveSubWithPlanRow
	withPlanErr error

	lastExpired    pgtype.Timestamp
	lastExpiredErr error

	dueRows     []sqlc.SubsDueForRenewalRow
	dueErr      error
	expiredRows []sqlc.ExpireLapsedSubsRow
	expiredErr  error

	countActiveN   int64
	countActiveErr error
	breakdownRows  []sqlc.PlansBreakdownRow
	breakdownErr   error

	// monthly stats — constant return per window
	monthNewSubs  int64
	monthRevenue  int64
	monthChurned  int64
	monthStatsErr error
}

// fakeMonthlyQ satisfies only the monthly-stats subset of Querier.
// Returning fixed values regardless of window lets tests assert loop
// correctness without a real DB.
type fakeMonthlyQ struct {
	newSubs int64
	revenue int64
	churned int64
	err     error
}

func (f *fakeMonthlyQ) GetActiveSubscriptionByUserID(ctx context.Context, id pgtype.UUID) (sqlc.GetActiveSubscriptionByUserIDRow, error) {
	return sqlc.GetActiveSubscriptionByUserIDRow{}, nil
}
func (f *fakeMonthlyQ) GetActiveSubWithPlan(ctx context.Context, id pgtype.UUID) (sqlc.GetActiveSubWithPlanRow, error) {
	return sqlc.GetActiveSubWithPlanRow{}, nil
}
func (f *fakeMonthlyQ) GetLastExpiredAt(ctx context.Context, id pgtype.UUID) (pgtype.Timestamp, error) {
	return pgtype.Timestamp{}, nil
}
func (f *fakeMonthlyQ) SubsDueForRenewal(ctx context.Context, arg sqlc.SubsDueForRenewalParams) ([]sqlc.SubsDueForRenewalRow, error) {
	return nil, nil
}
func (f *fakeMonthlyQ) ExpireLapsedSubs(ctx context.Context, expiresAt pgtype.Timestamp) ([]sqlc.ExpireLapsedSubsRow, error) {
	return nil, nil
}
func (f *fakeMonthlyQ) CountActiveSubscriptions(ctx context.Context) (int64, error) { return 0, nil }
func (f *fakeMonthlyQ) PlansBreakdown(ctx context.Context) ([]sqlc.PlansBreakdownRow, error) {
	return nil, nil
}
func (f *fakeMonthlyQ) CountNewSubsInWindow(ctx context.Context, arg sqlc.CountNewSubsInWindowParams) (int64, error) {
	return f.newSubs, f.err
}
func (f *fakeMonthlyQ) SumRevenueInWindow(ctx context.Context, arg sqlc.SumRevenueInWindowParams) (int64, error) {
	return f.revenue, f.err
}
func (f *fakeMonthlyQ) CountChurnedInWindow(ctx context.Context, arg sqlc.CountChurnedInWindowParams) (int64, error) {
	return f.churned, f.err
}

func (f fakeQ) GetActiveSubscriptionByUserID(ctx context.Context, id pgtype.UUID) (sqlc.GetActiveSubscriptionByUserIDRow, error) {
	return f.row, f.err
}

func (f fakeQ) GetActiveSubWithPlan(ctx context.Context, id pgtype.UUID) (sqlc.GetActiveSubWithPlanRow, error) {
	return f.withPlanRow, f.withPlanErr
}

func (f fakeQ) GetLastExpiredAt(ctx context.Context, id pgtype.UUID) (pgtype.Timestamp, error) {
	return f.lastExpired, f.lastExpiredErr
}

func (f fakeQ) SubsDueForRenewal(ctx context.Context, arg sqlc.SubsDueForRenewalParams) ([]sqlc.SubsDueForRenewalRow, error) {
	return f.dueRows, f.dueErr
}

func (f fakeQ) ExpireLapsedSubs(ctx context.Context, expiresAt pgtype.Timestamp) ([]sqlc.ExpireLapsedSubsRow, error) {
	return f.expiredRows, f.expiredErr
}

func (f fakeQ) CountActiveSubscriptions(ctx context.Context) (int64, error) {
	return f.countActiveN, f.countActiveErr
}

func (f fakeQ) PlansBreakdown(ctx context.Context) ([]sqlc.PlansBreakdownRow, error) {
	return f.breakdownRows, f.breakdownErr
}

func (f fakeQ) CountNewSubsInWindow(ctx context.Context, arg sqlc.CountNewSubsInWindowParams) (int64, error) {
	return f.monthNewSubs, f.monthStatsErr
}

func (f fakeQ) SumRevenueInWindow(ctx context.Context, arg sqlc.SumRevenueInWindowParams) (int64, error) {
	return f.monthRevenue, f.monthStatsErr
}

func (f fakeQ) CountChurnedInWindow(ctx context.Context, arg sqlc.CountChurnedInWindowParams) (int64, error) {
	return f.monthChurned, f.monthStatsErr
}

func TestActiveByUser_Found(t *testing.T) {
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r := sub.NewReader(fakeQ{row: sqlc.GetActiveSubscriptionByUserIDRow{
		Status: "active", StoreType: "lemonsqueezy",
		StartedAt: pgtype.Timestamp{Time: started, Valid: true},
		PlanName:  "Pro", DailyLimit: 100,
	}})
	got, err := r.ActiveByUser(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.PlanName != "Pro" || got.DailyLimit != 100 || got.Status != "active" {
		t.Fatalf("bad sub: %+v", got)
	}
	if got.StoreType != "lemonsqueezy" || !got.StartedAt.Equal(started) {
		t.Fatalf("bad fields: %+v", got)
	}
	if got.ExpiresAt != nil {
		t.Fatalf("expected nil ExpiresAt, got %v", got.ExpiresAt)
	}
}

func TestActiveByUser_WithExpiry(t *testing.T) {
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	r := sub.NewReader(fakeQ{row: sqlc.GetActiveSubscriptionByUserIDRow{
		Status: "active", StoreType: "stripe",
		StartedAt: pgtype.Timestamp{Time: started, Valid: true},
		ExpiresAt: pgtype.Timestamp{Time: exp, Valid: true},
		PlanName:  "Pro", DailyLimit: 100,
	}})
	got, err := r.ActiveByUser(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("expected expiry %v, got %v", exp, got.ExpiresAt)
	}
}

func TestActiveByUser_None(t *testing.T) {
	r := sub.NewReader(fakeQ{err: pgx.ErrNoRows})
	got, err := r.ActiveByUser(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("want nil sub, got %+v", got)
	}
}

func TestActiveWithPlan_NoRow_Nil(t *testing.T) {
	r := sub.NewReader(fakeQ{withPlanErr: pgx.ErrNoRows})
	v, err := r.ActiveWithPlan(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil || v != nil {
		t.Fatalf("want nil,nil got %v,%v", v, err)
	}
}

func TestActiveWithPlan_Found(t *testing.T) {
	exp := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	r := sub.NewReader(fakeQ{withPlanRow: sqlc.GetActiveSubWithPlanRow{
		Status:        "active",
		ExpiresAt:     pgtype.Timestamp{Time: exp, Valid: true},
		PlanName:      "Pro",
		DailyLimit:    100,
		BillingPeriod: "monthly",
	}})
	v, err := r.ActiveWithPlan(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if v == nil || v.Status != "active" || v.PlanName != "Pro" || v.DailyLimit != 100 {
		t.Fatalf("bad view: %+v", v)
	}
	if v.BillingPeriod != "monthly" {
		t.Fatalf("bad billing period: %+v", v)
	}
	if v.ExpiresAt == nil || !v.ExpiresAt.Equal(exp) {
		t.Fatalf("expected expiry %v, got %v", exp, v.ExpiresAt)
	}
}

func TestActiveWithPlan_NullExpiry(t *testing.T) {
	r := sub.NewReader(fakeQ{withPlanRow: sqlc.GetActiveSubWithPlanRow{
		Status:        "active",
		PlanName:      "Free",
		DailyLimit:    10,
		BillingPeriod: "none",
	}})
	v, err := r.ActiveWithPlan(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if v == nil || v.ExpiresAt != nil {
		t.Fatalf("expected non-nil view with nil ExpiresAt, got %+v", v)
	}
}

func TestLastExpiredAt_NoRow_Nil(t *testing.T) {
	r := sub.NewReader(fakeQ{lastExpiredErr: pgx.ErrNoRows})
	tm, err := r.LastExpiredAt(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil || tm != nil {
		t.Fatalf("want nil,nil got %v,%v", tm, err)
	}
}

func TestLastExpiredAt_Found(t *testing.T) {
	at := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	r := sub.NewReader(fakeQ{lastExpired: pgtype.Timestamp{Time: at, Valid: true}})
	tm, err := r.LastExpiredAt(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if tm == nil || !tm.Equal(at) {
		t.Fatalf("expected %v, got %v", at, tm)
	}
}

func TestLastExpiredAt_NotValid_Nil(t *testing.T) {
	r := sub.NewReader(fakeQ{lastExpired: pgtype.Timestamp{Valid: false}})
	tm, err := r.LastExpiredAt(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil || tm != nil {
		t.Fatalf("want nil,nil got %v,%v", tm, err)
	}
}

// *Reader must satisfy the cron's CronRepo port.
var _ sub.CronRepo = (*sub.Reader)(nil)

func TestDueForRenewal_MapsRows(t *testing.T) {
	exp := time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC)
	var uid pgtype.UUID
	_ = uid.Scan("11111111-1111-1111-1111-111111111111")
	r := sub.NewReader(fakeQ{dueRows: []sqlc.SubsDueForRenewalRow{{
		UserID:    uid,
		Email:     "a@b.com",
		ExpiresAt: pgtype.Timestamp{Time: exp, Valid: true},
		PlanName:  "Pro",
	}}})
	rows, err := r.DueForRenewal(context.Background(),
		time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	got := rows[0]
	if got.UserID != "11111111-1111-1111-1111-111111111111" || got.Email != "a@b.com" ||
		got.PlanName != "Pro" || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("bad mapping: %+v", got)
	}
}

func TestExpireLapsed_MapsRows(t *testing.T) {
	exp := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	var uid pgtype.UUID
	_ = uid.Scan("22222222-2222-2222-2222-222222222222")
	r := sub.NewReader(fakeQ{expiredRows: []sqlc.ExpireLapsedSubsRow{{
		UserID:    uid,
		Email:     "c@d.com",
		ExpiresAt: pgtype.Timestamp{Time: exp, Valid: true},
	}}})
	rows, err := r.ExpireLapsed(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].UserID != "22222222-2222-2222-2222-222222222222" ||
		rows[0].Email != "c@d.com" || !rows[0].ExpiresAt.Equal(exp) {
		t.Fatalf("bad mapping: %+v", rows)
	}
}

// ---------- CountActive ----------

func TestCountActive_ReturnsInt(t *testing.T) {
	r := sub.NewReader(fakeQ{countActiveN: 42})
	n, err := r.CountActive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 42 {
		t.Fatalf("expected 42, got %d", n)
	}
}

func TestCountActive_PropagatesError(t *testing.T) {
	sentinel := errors.New("db down")
	r := sub.NewReader(fakeQ{countActiveErr: sentinel})
	_, err := r.CountActive(context.Background())
	if err != sentinel {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}

// ---------- GetPlansBreakdown ----------

func ptr[T any](v T) *T { return &v }

func TestGetPlansBreakdown_MapsRows(t *testing.T) {
	rows := []sqlc.PlansBreakdownRow{
		{PlanName: ptr("Pro"), PriceCents: ptr(int32(999)), ActiveCount: 3},
		{PlanName: ptr("Free"), PriceCents: ptr(int32(0)), ActiveCount: 7},
	}
	r := sub.NewReader(fakeQ{breakdownRows: rows})
	got, err := r.GetPlansBreakdown(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []sub.PlanBreakdown{
		{PlanName: "Pro", ActiveCount: 3, PriceCents: 999},
		{PlanName: "Free", ActiveCount: 7, PriceCents: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d rows, got %d: %+v", len(want), len(got), got)
	}
	for i, w := range want {
		g := got[i]
		if g.PlanName != w.PlanName || g.ActiveCount != w.ActiveCount || g.PriceCents != w.PriceCents {
			t.Errorf("row %d: got %+v, want %+v", i, g, w)
		}
	}
}

func TestGetPlansBreakdown_Empty_NonNilSlice(t *testing.T) {
	r := sub.NewReader(fakeQ{breakdownRows: nil})
	got, err := r.GetPlansBreakdown(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %+v", got)
	}
}

func TestGetPlansBreakdown_NullPlanName_EmptyString(t *testing.T) {
	rows := []sqlc.PlansBreakdownRow{
		{PlanName: nil, PriceCents: nil, ActiveCount: 5},
	}
	r := sub.NewReader(fakeQ{breakdownRows: rows})
	got, err := r.GetPlansBreakdown(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	if got[0].PlanName != "" {
		t.Errorf("expected empty PlanName, got %q", got[0].PlanName)
	}
	if got[0].PriceCents != 0 {
		t.Errorf("expected 0 PriceCents, got %d", got[0].PriceCents)
	}
	if got[0].ActiveCount != 5 {
		t.Errorf("expected 5 ActiveCount, got %d", got[0].ActiveCount)
	}
}

func TestGetPlansBreakdown_PropagatesError(t *testing.T) {
	sentinel := errors.New("breakdown failed")
	r := sub.NewReader(fakeQ{breakdownErr: sentinel})
	_, err := r.GetPlansBreakdown(context.Background())
	if err != sentinel {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}

// ---------- GetMonthlyStatsAt ----------

func TestGetMonthlyStats_WindowsAndOrder(t *testing.T) {
	// now = 2026-06-18. 12 months back: oldest = 2025-07, newest = 2026-06.
	now := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	f := &fakeMonthlyQ{newSubs: 1, revenue: 500, churned: 0}
	r := sub.NewReader(f)
	got, err := r.GetMonthlyStatsAt(context.Background(), now, 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 12 {
		t.Fatalf("expected 12 entries, got %d", len(got))
	}
	// Oldest entry
	if got[0].Month != "2025-07" {
		t.Errorf("got[0].Month = %q, want 2025-07", got[0].Month)
	}
	// Newest entry
	if got[11].Month != "2026-06" {
		t.Errorf("got[11].Month = %q, want 2026-06", got[11].Month)
	}
	// Verify all fields on newest entry
	want := sub.MonthStat{Month: "2026-06", NewSubscriptions: 1, RevenueCents: 500, Churned: 0}
	if got[11] != want {
		t.Errorf("got[11] = %+v, want %+v", got[11], want)
	}
}

func TestGetMonthlyStats_YearBoundary(t *testing.T) {
	// now = 2026-01-15, 3 months → oldest=2025-11, mid=2025-12, newest=2026-01
	now := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	f := &fakeMonthlyQ{newSubs: 2, revenue: 1000, churned: 1}
	r := sub.NewReader(f)
	got, err := r.GetMonthlyStatsAt(context.Background(), now, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	if got[0].Month != "2025-11" {
		t.Errorf("got[0].Month = %q, want 2025-11", got[0].Month)
	}
	if got[1].Month != "2025-12" {
		t.Errorf("got[1].Month = %q, want 2025-12", got[1].Month)
	}
	if got[2].Month != "2026-01" {
		t.Errorf("got[2].Month = %q, want 2026-01", got[2].Month)
	}
}

func TestGetMonthlyStats_PropagatesError(t *testing.T) {
	sentinel := errors.New("db gone")
	f := &fakeMonthlyQ{err: sentinel}
	r := sub.NewReader(f)
	_, err := r.GetMonthlyStatsAt(context.Background(), time.Now(), 12)
	if err != sentinel {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}

func TestGetMonthlyStats_SingleMonth(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	f := &fakeMonthlyQ{newSubs: 5, revenue: 2500, churned: 2}
	r := sub.NewReader(f)
	got, err := r.GetMonthlyStatsAt(context.Background(), now, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].Month != "2026-03" {
		t.Errorf("got[0].Month = %q, want 2026-03", got[0].Month)
	}
	if got[0].NewSubscriptions != 5 || got[0].RevenueCents != 2500 || got[0].Churned != 2 {
		t.Errorf("unexpected values: %+v", got[0])
	}
}
