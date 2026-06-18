package subscription_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

	// FindAllPaginated (ListSubscriptionsPaginated + CountSubscriptionsPaginated)
	txRows     []sqlc.ListSubscriptionsPaginatedRow
	txCount    int64
	txErr      error
	txCountErr error

	// FindLatestStripeForUserPlan
	stripeRow sqlc.FindLatestStripeForUserPlanRow
	stripeErr error
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
func (f *fakeMonthlyQ) ListSubscriptionsPaginated(ctx context.Context, arg sqlc.ListSubscriptionsPaginatedParams) ([]sqlc.ListSubscriptionsPaginatedRow, error) {
	return nil, nil
}
func (f *fakeMonthlyQ) CountSubscriptionsPaginated(ctx context.Context, search *string) (int64, error) {
	return 0, nil
}
func (f *fakeMonthlyQ) FindLatestStripeForUserPlan(ctx context.Context, arg sqlc.FindLatestStripeForUserPlanParams) (sqlc.FindLatestStripeForUserPlanRow, error) {
	return sqlc.FindLatestStripeForUserPlanRow{}, pgx.ErrNoRows
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

func (f fakeQ) ListSubscriptionsPaginated(ctx context.Context, arg sqlc.ListSubscriptionsPaginatedParams) ([]sqlc.ListSubscriptionsPaginatedRow, error) {
	return f.txRows, f.txErr
}

func (f fakeQ) CountSubscriptionsPaginated(ctx context.Context, search *string) (int64, error) {
	return f.txCount, f.txCountErr
}

func (f fakeQ) FindLatestStripeForUserPlan(ctx context.Context, arg sqlc.FindLatestStripeForUserPlanParams) (sqlc.FindLatestStripeForUserPlanRow, error) {
	return f.stripeRow, f.stripeErr
}

// capturingFakeQ is a pointer-based querier used when tests need to inspect
// the exact params (LIMIT, OFFSET, search) forwarded to the DB layer.
type capturingFakeQ struct {
	txRows     []sqlc.ListSubscriptionsPaginatedRow
	txCount    int64
	txErr      error
	txCountErr error

	lastListParams  sqlc.ListSubscriptionsPaginatedParams
	lastCountSearch *string
	capturedList    bool
	capturedCount   bool

	stripeRow sqlc.FindLatestStripeForUserPlanRow
	stripeErr error
}

// capturingFakeQ satisfies the Querier interface; unused methods are stubs.
func (f *capturingFakeQ) GetActiveSubscriptionByUserID(ctx context.Context, id pgtype.UUID) (sqlc.GetActiveSubscriptionByUserIDRow, error) {
	return sqlc.GetActiveSubscriptionByUserIDRow{}, nil
}
func (f *capturingFakeQ) GetActiveSubWithPlan(ctx context.Context, id pgtype.UUID) (sqlc.GetActiveSubWithPlanRow, error) {
	return sqlc.GetActiveSubWithPlanRow{}, nil
}
func (f *capturingFakeQ) GetLastExpiredAt(ctx context.Context, id pgtype.UUID) (pgtype.Timestamp, error) {
	return pgtype.Timestamp{}, nil
}
func (f *capturingFakeQ) SubsDueForRenewal(ctx context.Context, arg sqlc.SubsDueForRenewalParams) ([]sqlc.SubsDueForRenewalRow, error) {
	return nil, nil
}
func (f *capturingFakeQ) ExpireLapsedSubs(ctx context.Context, expiresAt pgtype.Timestamp) ([]sqlc.ExpireLapsedSubsRow, error) {
	return nil, nil
}
func (f *capturingFakeQ) CountActiveSubscriptions(ctx context.Context) (int64, error) { return 0, nil }
func (f *capturingFakeQ) PlansBreakdown(ctx context.Context) ([]sqlc.PlansBreakdownRow, error) {
	return nil, nil
}
func (f *capturingFakeQ) CountNewSubsInWindow(ctx context.Context, arg sqlc.CountNewSubsInWindowParams) (int64, error) {
	return 0, nil
}
func (f *capturingFakeQ) SumRevenueInWindow(ctx context.Context, arg sqlc.SumRevenueInWindowParams) (int64, error) {
	return 0, nil
}
func (f *capturingFakeQ) CountChurnedInWindow(ctx context.Context, arg sqlc.CountChurnedInWindowParams) (int64, error) {
	return 0, nil
}
func (f *capturingFakeQ) ListSubscriptionsPaginated(ctx context.Context, arg sqlc.ListSubscriptionsPaginatedParams) ([]sqlc.ListSubscriptionsPaginatedRow, error) {
	f.lastListParams = arg
	f.capturedList = true
	return f.txRows, f.txErr
}
func (f *capturingFakeQ) CountSubscriptionsPaginated(ctx context.Context, search *string) (int64, error) {
	f.lastCountSearch = search
	f.capturedCount = true
	return f.txCount, f.txCountErr
}
func (f *capturingFakeQ) FindLatestStripeForUserPlan(ctx context.Context, arg sqlc.FindLatestStripeForUserPlanParams) (sqlc.FindLatestStripeForUserPlanRow, error) {
	return f.stripeRow, f.stripeErr
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

// ─── FindAllPaginated ────────────────────────────────────────────────────────

// ptrStr is a helper that returns a pointer to the given string literal.
func ptrStr(s string) *string { return &s }

// buildUID converts a UUID string to pgtype.UUID for test row building.
func buildUID(s string) pgtype.UUID {
	var uid pgtype.UUID
	_ = uid.Scan(s)
	return uid
}

// TestFindAllPaginated_MapsNullsToEmDash verifies that NULL user + plan
// columns become "—" (U+2014) and that price_cents NULL becomes 0.
// Also confirms TxPage.Total is taken from the count query.
func TestFindAllPaginated_MapsNullsToEmDash(t *testing.T) {
	createdAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, 1, 14, 9, 0, 0, 0, time.UTC)

	var uid pgtype.UUID
	_ = uid.Scan("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	row := sqlc.ListSubscriptionsPaginatedRow{
		ID:                 uid,
		UserEmail:          nil, // NULL → "—"
		UserName:           nil, // NULL → "—"
		UserID:             uid,
		PlanName:           nil, // NULL → "—"
		PriceCents:         nil, // NULL → 0
		StoreType:          "stripe",
		StoreTransactionID: ptrStr("sub_abc"),
		Status:             "active",
		StartedAt:          pgtype.Timestamp{Time: startedAt, Valid: true},
		ExpiresAt:          pgtype.Timestamp{Valid: false},
		CreatedAt:          pgtype.Timestamp{Time: createdAt, Valid: true},
	}

	f := fakeQ{txRows: []sqlc.ListSubscriptionsPaginatedRow{row}, txCount: 99}
	r := sub.NewReader(f)
	page, err := r.FindAllPaginated(context.Background(), sub.TxQuery{Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 99 {
		t.Errorf("Total = %d, want 99", page.Total)
	}
	if len(page.Transactions) != 1 {
		t.Fatalf("expected 1 tx, got %d", len(page.Transactions))
	}
	tx := page.Transactions[0]
	const emDash = "—"
	if tx.UserEmail != emDash {
		t.Errorf("UserEmail = %q, want em-dash %q", tx.UserEmail, emDash)
	}
	if tx.UserName != emDash {
		t.Errorf("UserName = %q, want em-dash %q", tx.UserName, emDash)
	}
	if tx.PlanName != emDash {
		t.Errorf("PlanName = %q, want em-dash %q", tx.PlanName, emDash)
	}
	if tx.PriceCents != 0 {
		t.Errorf("PriceCents = %d, want 0", tx.PriceCents)
	}
	if tx.StoreTransactionID != nil {
		if *tx.StoreTransactionID != "sub_abc" {
			t.Errorf("StoreTransactionID = %q, want sub_abc", *tx.StoreTransactionID)
		}
	}
	if tx.ExpiresAt != nil {
		t.Errorf("ExpiresAt = %v, want nil", tx.ExpiresAt)
	}
	if !tx.StartedAt.Equal(startedAt) {
		t.Errorf("StartedAt = %v, want %v", tx.StartedAt, startedAt)
	}
}

// TestFindAllPaginated_MapsPopulatedRow verifies non-null fields pass through.
func TestFindAllPaginated_MapsPopulatedRow(t *testing.T) {
	exp := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	price := int32(999)

	row := sqlc.ListSubscriptionsPaginatedRow{
		ID:                 buildUID("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		UserEmail:          ptrStr("user@example.com"),
		UserName:           ptrStr("Alice"),
		UserID:             buildUID("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		PlanName:           ptrStr("Pro"),
		PriceCents:         &price,
		StoreType:          "lemonsqueezy",
		StoreTransactionID: ptrStr("ls_xyz"),
		Status:             "active",
		StartedAt:          pgtype.Timestamp{Time: startedAt, Valid: true},
		ExpiresAt:          pgtype.Timestamp{Time: exp, Valid: true},
		CreatedAt:          pgtype.Timestamp{Time: createdAt, Valid: true},
	}

	f := fakeQ{txRows: []sqlc.ListSubscriptionsPaginatedRow{row}, txCount: 1}
	r := sub.NewReader(f)
	page, err := r.FindAllPaginated(context.Background(), sub.TxQuery{Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	tx := page.Transactions[0]
	if tx.UserEmail != "user@example.com" {
		t.Errorf("UserEmail = %q", tx.UserEmail)
	}
	if tx.PlanName != "Pro" {
		t.Errorf("PlanName = %q", tx.PlanName)
	}
	if tx.PriceCents != 999 {
		t.Errorf("PriceCents = %d", tx.PriceCents)
	}
	if tx.ExpiresAt == nil || !tx.ExpiresAt.Equal(exp) {
		t.Errorf("ExpiresAt = %v, want %v", tx.ExpiresAt, exp)
	}
	if tx.StoreType != "lemonsqueezy" {
		t.Errorf("StoreType = %q", tx.StoreType)
	}
}

// TestFindAllPaginated_OffsetComputation verifies (page-1)*limit arithmetic
// by capturing the params sent to the fake querier.
func TestFindAllPaginated_OffsetComputation(t *testing.T) {
	cases := []struct {
		page, limit int
		wantOffset  int32
		wantLimit   int32
	}{
		{1, 20, 0, 20},
		{2, 20, 20, 20},
		{3, 10, 20, 10},
		{5, 20, 80, 20},
	}
	for _, c := range cases {
		f := &capturingFakeQ{}
		r := sub.NewReader(f)
		_, err := r.FindAllPaginated(context.Background(), sub.TxQuery{Page: c.page, Limit: c.limit})
		if err != nil {
			t.Fatalf("page=%d limit=%d: %v", c.page, c.limit, err)
		}
		if !f.capturedList {
			t.Fatalf("page=%d limit=%d: ListSubscriptionsPaginated not called", c.page, c.limit)
		}
		if f.lastListParams.Offset != c.wantOffset {
			t.Errorf("page=%d limit=%d: offset=%d, want %d", c.page, c.limit, f.lastListParams.Offset, c.wantOffset)
		}
		if f.lastListParams.Limit != c.wantLimit {
			t.Errorf("page=%d limit=%d: limit=%d, want %d", c.page, c.limit, f.lastListParams.Limit, c.wantLimit)
		}
	}
}

// TestFindAllPaginated_SearchNilWhenEmpty verifies that an empty search string
// causes nil to be passed to CountSubscriptionsPaginated and
// ListSubscriptionsPaginated (the IS NULL branch matches all).
func TestFindAllPaginated_SearchNilWhenEmpty(t *testing.T) {
	f := &capturingFakeQ{}
	r := sub.NewReader(f)
	_, err := r.FindAllPaginated(context.Background(), sub.TxQuery{Search: "", Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if f.lastListParams.Search != nil {
		t.Errorf("Search = %v, want nil when empty", f.lastListParams.Search)
	}
	if f.lastCountSearch != nil {
		t.Errorf("count Search = %v, want nil when empty", f.lastCountSearch)
	}
}

// TestFindAllPaginated_SearchNonNilWhenSet verifies a non-empty search is
// forwarded as a non-nil *string.
func TestFindAllPaginated_SearchNonNilWhenSet(t *testing.T) {
	f := &capturingFakeQ{}
	r := sub.NewReader(f)
	_, err := r.FindAllPaginated(context.Background(), sub.TxQuery{Search: "alice", Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if f.lastListParams.Search == nil || *f.lastListParams.Search != "alice" {
		t.Errorf("Search = %v, want 'alice'", f.lastListParams.Search)
	}
	if f.lastCountSearch == nil || *f.lastCountSearch != "alice" {
		t.Errorf("count Search = %v, want 'alice'", f.lastCountSearch)
	}
}

// TestFindAllPaginated_PropagatesListError verifies a DB error on the list
// query is forwarded.
func TestFindAllPaginated_PropagatesListError(t *testing.T) {
	sentinel := errors.New("list failed")
	f := fakeQ{txErr: sentinel}
	r := sub.NewReader(f)
	_, err := r.FindAllPaginated(context.Background(), sub.TxQuery{Page: 1, Limit: 20})
	if err != sentinel {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

// TestFindAllPaginated_PropagatesCountError verifies a DB error on the count
// query is forwarded.
func TestFindAllPaginated_PropagatesCountError(t *testing.T) {
	sentinel := errors.New("count failed")
	f := fakeQ{txCountErr: sentinel}
	r := sub.NewReader(f)
	_, err := r.FindAllPaginated(context.Background(), sub.TxQuery{Page: 1, Limit: 20})
	if err != sentinel {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

// TestFindAllPaginated_EmptyResult verifies an empty DB result returns a
// non-nil empty slice (not nil) so the handler encodes [] not null.
func TestFindAllPaginated_EmptyResult(t *testing.T) {
	f := fakeQ{txRows: nil, txCount: 0}
	r := sub.NewReader(f)
	page, err := r.FindAllPaginated(context.Background(), sub.TxQuery{Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Transactions == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(page.Transactions) != 0 {
		t.Fatalf("expected 0 transactions, got %d", len(page.Transactions))
	}
}

// ─── TransactionRow JSON marshal ────────────────────────────────────────────

// TestTransactionRow_JSONMarshal asserts key order, em-dash fallbacks,
// null expires_at, ms-precision timestamp, and store_transaction_id null.
func TestTransactionRow_JSONMarshal(t *testing.T) {
	created := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	started := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	stxID := "sub_123"

	tx := sub.TransactionRow{
		ID:                 "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		UserEmail:          "—",
		UserName:           "—",
		UserID:             "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		PlanName:           "—",
		PriceCents:         0,
		StoreType:          "stripe",
		StoreTransactionID: &stxID,
		Status:             "active",
		StartedAt:          &started,
		ExpiresAt:          nil,
		CreatedAt:          &created,
	}

	b, err := json.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)

	// Key order: id, user_email, user_name, user_id, plan_name, price_cents,
	// store_type, store_transaction_id, status, started_at, expires_at, created_at
	keys := []string{
		`"id"`, `"user_email"`, `"user_name"`, `"user_id"`, `"plan_name"`,
		`"price_cents"`, `"store_type"`, `"store_transaction_id"`, `"status"`,
		`"started_at"`, `"expires_at"`, `"created_at"`,
	}
	prevIdx := -1
	for _, k := range keys {
		idx := strings.Index(s, k)
		if idx < 0 {
			t.Errorf("key %s not found in JSON: %s", k, s)
			continue
		}
		if idx <= prevIdx {
			t.Errorf("key %s out of order (pos %d <= prev %d) in: %s", k, idx, prevIdx, s)
		}
		prevIdx = idx
	}

	// em-dash values present
	const emDash = "—"
	if !strings.Contains(s, emDash) {
		t.Errorf("em-dash (U+2014) not found in JSON: %s", s)
	}

	// expires_at is null
	if !strings.Contains(s, `"expires_at":null`) {
		t.Errorf("expected expires_at:null in JSON: %s", s)
	}

	// ms-precision timestamp for created_at: format 2006-01-02T15:04:05.000Z
	if !strings.Contains(s, `"created_at":"2026-06-01T12:00:00.000Z"`) {
		t.Errorf("created_at not ms-precision: %s", s)
	}

	// started_at ms-precision
	if !strings.Contains(s, `"started_at":"2026-05-01T00:00:00.000Z"`) {
		t.Errorf("started_at not ms-precision: %s", s)
	}

	// store_transaction_id present
	if !strings.Contains(s, `"store_transaction_id":"sub_123"`) {
		t.Errorf("store_transaction_id not present: %s", s)
	}
}

// TestTransactionRow_JSONMarshal_NullStoreTxID asserts that a nil
// store_transaction_id encodes as JSON null (not "—").
func TestTransactionRow_JSONMarshal_NullStoreTxID(t *testing.T) {
	created := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	started := created
	tx := sub.TransactionRow{
		ID:                 "x",
		UserEmail:          "—",
		UserName:           "—",
		UserID:             "x",
		PlanName:           "—",
		StoreType:          "admin_granted",
		StoreTransactionID: nil,
		Status:             "active",
		StartedAt:          &started,
		ExpiresAt:          nil,
		CreatedAt:          &created,
	}
	b, err := json.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"store_transaction_id":null`) {
		t.Errorf("expected null store_transaction_id: %s", s)
	}
}

// ─── FindLatestStripeForUserPlan ─────────────────────────────────────────────

func TestFindLatestStripeForUserPlan_Hit(t *testing.T) {
	txID := "sub_stripe_123"
	var uid pgtype.UUID
	_ = uid.Scan("dddddddd-dddd-dddd-dddd-dddddddddddd")
	row := sqlc.FindLatestStripeForUserPlanRow{
		ID:                 uid,
		StoreTransactionID: &txID,
	}
	f := fakeQ{stripeRow: row}
	r := sub.NewReader(f)
	got, found, err := r.FindLatestStripeForUserPlan(context.Background(),
		"dddddddd-dddd-dddd-dddd-dddddddddddd",
		"eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected found=true, got false")
	}
	if got != txID {
		t.Errorf("storeTxID = %q, want %q", got, txID)
	}
}

// TestFindLatestStripeForUserPlan_NoRows verifies that pgx.ErrNoRows
// collapses to ("", false, nil) — not an error.
func TestFindLatestStripeForUserPlan_NoRows(t *testing.T) {
	f := fakeQ{stripeErr: pgx.ErrNoRows}
	r := sub.NewReader(f)
	got, found, err := r.FindLatestStripeForUserPlan(context.Background(),
		"ffffffff-ffff-ffff-ffff-ffffffffffff",
		"00000000-0000-0000-0000-000000000000",
	)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if found {
		t.Fatal("expected found=false")
	}
	if got != "" {
		t.Errorf("storeTxID = %q, want empty", got)
	}
}

// TestFindLatestStripeForUserPlan_DBError verifies other DB errors propagate.
func TestFindLatestStripeForUserPlan_DBError(t *testing.T) {
	sentinel := errors.New("connection refused")
	f := fakeQ{stripeErr: sentinel}
	r := sub.NewReader(f)
	_, _, err := r.FindLatestStripeForUserPlan(context.Background(),
		"ffffffff-ffff-ffff-ffff-ffffffffffff",
		"00000000-0000-0000-0000-000000000000",
	)
	if err != sentinel {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}
