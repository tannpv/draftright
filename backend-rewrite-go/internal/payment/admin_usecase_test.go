// Package payment admin usecase tests — verify AdminService.GetStats in
// isolation against a fake repo satisfying paymentAdminRepo. Parity authority:
// src/payment/payment.service.ts getStats() (line 745) which returns
// { total, completed, pending, revenue }. Same fake-repo approach as
// internal/adminstats tests; the DB-backed sqlc path is not unit-tested here.
package payment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/plans"
	"github.com/tannpv/draftright-rewrite/internal/user"
)

// fakeAdminStatsRepo satisfies paymentAdminRepo, returning canned stats/rows or
// an error so the service can be exercised without a database.
type fakeAdminStatsRepo struct {
	stats PaymentStats
	err   error

	// FindAll knobs.
	findRows  []AdminPaymentRow
	findTotal int
	findErr   error
	gotParams *FindAllParams
}

func (f *fakeAdminStatsRepo) Stats(_ context.Context) (PaymentStats, error) {
	return f.stats, f.err
}

func (f *fakeAdminStatsRepo) FindAll(_ context.Context, p FindAllParams) ([]AdminPaymentRow, int, error) {
	f.gotParams = &p
	return f.findRows, f.findTotal, f.findErr
}

// TestAdminPaymentStats_ReturnsRepoValues verifies GetStats passes the repo's
// aggregate row through unchanged as PaymentStats.
func TestAdminPaymentStats_ReturnsRepoValues(t *testing.T) {
	repo := &fakeAdminStatsRepo{stats: PaymentStats{
		Total:     10,
		Completed: 6,
		Pending:   3,
		Revenue:   1998,
	}}
	svc := NewAdminService(repo)

	got, err := svc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	want := PaymentStats{Total: 10, Completed: 6, Pending: 3, Revenue: 1998}
	if got != want {
		t.Errorf("GetStats() = %+v, want %+v", got, want)
	}
}

// TestAdminPaymentStats_PropagatesError verifies a repo error surfaces with a
// zero-value result (no partial data).
func TestAdminPaymentStats_PropagatesError(t *testing.T) {
	boom := errors.New("stats query down")
	svc := NewAdminService(&fakeAdminStatsRepo{err: boom})

	got, err := svc.GetStats(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("GetStats() error = %v, want %v", err, boom)
	}
	if (got != PaymentStats{}) {
		t.Errorf("GetStats() = %+v, want zero value on error", got)
	}
}

// TestAdminPaymentFindAll_DelegatesAndWraps verifies FindAll forwards the
// params, wraps rows+total into a PaymentsPage.
func TestAdminPaymentFindAll_DelegatesAndWraps(t *testing.T) {
	repo := &fakeAdminStatsRepo{
		findRows:  []AdminPaymentRow{{ID: "p1"}, {ID: "p2"}},
		findTotal: 7,
	}
	svc := NewAdminService(repo)

	in := FindAllParams{Page: 2, Limit: 5, Status: "completed", Search: "x", SortBy: "amount", SortOrder: "ASC"}
	page, err := svc.FindAll(context.Background(), in)
	if err != nil {
		t.Fatalf("FindAll() error: %v", err)
	}
	if repo.gotParams == nil || *repo.gotParams != in {
		t.Fatalf("repo got %+v, want %+v", repo.gotParams, in)
	}
	if len(page.Payments) != 2 || page.Total != 7 {
		t.Fatalf("got %d rows total %d, want 2 rows total 7", len(page.Payments), page.Total)
	}
}

// TestAdminPaymentFindAll_NilRowsToEmpty verifies a nil rows slice normalises
// to a non-nil empty slice (so JSON emits [] not null).
func TestAdminPaymentFindAll_NilRowsToEmpty(t *testing.T) {
	svc := NewAdminService(&fakeAdminStatsRepo{findRows: nil, findTotal: 0})
	page, err := svc.FindAll(context.Background(), FindAllParams{Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("FindAll() error: %v", err)
	}
	if page.Payments == nil {
		t.Fatal("Payments must be non-nil empty slice, got nil")
	}
	if len(page.Payments) != 0 {
		t.Fatalf("want 0 rows, got %d", len(page.Payments))
	}
	b, _ := json.Marshal(page)
	if !strings.Contains(string(b), `"payments":[]`) {
		t.Fatalf(`want "payments":[], got %s`, b)
	}
}

// TestAdminPaymentFindAll_PropagatesError verifies a repo error surfaces with a
// zero PaymentsPage.
func TestAdminPaymentFindAll_PropagatesError(t *testing.T) {
	boom := errors.New("findAll query down")
	svc := NewAdminService(&fakeAdminStatsRepo{findErr: boom})
	page, err := svc.FindAll(context.Background(), FindAllParams{Page: 1, Limit: 20})
	if !errors.Is(err, boom) {
		t.Fatalf("FindAll() error = %v, want %v", err, boom)
	}
	if page.Payments != nil || page.Total != 0 {
		t.Fatalf("want zero PaymentsPage on error, got %+v", page)
	}
}

func tptr(t time.Time) *time.Time { return &t }
func sp(s string) *string         { return &s }

// TestAdminPaymentRow_MarshalJSON_KeyOrderAndNesting is the parity-critical
// test: it pins the 17 top-level Payment-entity keys in declaration order, the
// nested raw `user` (incl password_hash) + `plan` shapes, and the null cases
// for a missing relation / a nil nullable timestamp.
func TestAdminPaymentRow_MarshalJSON_KeyOrderAndNesting(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 678_000_000, time.UTC)
	updated := time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)

	row := AdminPaymentRow{
		ID:     "pay1",
		UserID: "u1",
		User: &user.UserDetail{
			ID:           "u1",
			Email:        "a@b.com",
			PasswordHash: sp("$2b$10$hash"),
			Name:         "Alice",
			IsActive:     true,
			Role:         "user",
			AuthProvider: "local",
			CreatedAt:    created,
			UpdatedAt:    updated,
		},
		PlanID: "pl1",
		Plan: &plans.PlanEntity{
			ID:            "pl1",
			Name:          "Pro",
			DailyLimit:    100,
			PriceCents:    999,
			BillingPeriod: "monthly",
			IsActive:      true,
			CreatedAt:     created,
			UpdatedAt:     updated,
		},
		Amount:        999,
		Currency:      "USD",
		Method:        "stripe",
		Status:        "completed",
		ProviderRef:   sp("pi_123"),
		ReferenceCode: "DR-PRO-001",
		QRData:        nil,
		Notes:         nil,
		ExpiresAt:     nil,
		CompletedAt:   tptr(created),
		CreatedAt:     created,
		UpdatedAt:     updated,
	}

	b, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	// Top-level key order (Payment entity declaration order). Redact the nested
	// user/plan objects first so their inner keys (which repeat top-level names
	// like currency/created_at) don't confuse the positional check.
	topKeys := []string{
		"id", "user_id", "user", "plan_id", "plan", "amount", "currency",
		"method", "status", "provider_ref", "reference_code", "qr_data",
		"notes", "expires_at", "completed_at", "created_at", "updated_at",
	}
	assertOrder(t, redactNested(got), topKeys)

	// Nested user opens with id ... and includes the raw password_hash.
	uStart := strings.Index(got, `"user":{`)
	if uStart < 0 {
		t.Fatalf(`missing "user":{ in %s`, got)
	}
	if !strings.HasPrefix(got[uStart:], `"user":{"id":`) {
		t.Fatalf(`nested user must open with "id", got %s`, got[uStart:uStart+40])
	}
	if !strings.Contains(got, `"password_hash":"$2b$10$hash"`) {
		t.Fatalf("nested user must expose raw password_hash, got %s", got)
	}

	// Nested plan opens with id.
	plStart := strings.Index(got, `"plan":{`)
	if plStart < 0 {
		t.Fatalf(`missing "plan":{ in %s`, got)
	}
	if !strings.HasPrefix(got[plStart:], `"plan":{"id":`) {
		t.Fatalf(`nested plan must open with "id", got %s`, got[plStart:plStart+40])
	}

	// Nullables: expires_at nil → null; completed_at non-nil → ISO-millis.
	if !strings.Contains(got, `"expires_at":null`) {
		t.Fatalf(`nil expires_at must render null, got %s`, got)
	}
	if !strings.Contains(got, `"completed_at":"2026-01-02T03:04:05.678Z"`) {
		t.Fatalf("non-nil completed_at must render ISO-millis, got %s", got)
	}
	if !strings.Contains(got, `"qr_data":null`) || !strings.Contains(got, `"notes":null`) {
		t.Fatalf("nil qr_data/notes must render null, got %s", got)
	}
}

// TestAdminPaymentRow_MarshalJSON_NilRelations verifies a missing LEFT-JOIN
// relation renders JSON null for user and plan.
func TestAdminPaymentRow_MarshalJSON_NilRelations(t *testing.T) {
	row := AdminPaymentRow{
		ID:            "pay2",
		UserID:        "u9",
		User:          nil,
		PlanID:        "pl9",
		Plan:          nil,
		Amount:        0,
		Currency:      "VND",
		Method:        "vietqr",
		Status:        "pending",
		ReferenceCode: "DR-PRO-002",
		CreatedAt:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"user":null`) {
		t.Fatalf(`nil user must render null, got %s`, got)
	}
	if !strings.Contains(got, `"plan":null`) {
		t.Fatalf(`nil plan must render null, got %s`, got)
	}
}

// redactNested replaces the values of the nested "user":{...} and "plan":{...}
// objects with empty braces so a positional top-level key scan isn't fooled by
// repeated key names inside them. Keeps the "user":/"plan": keys themselves.
func redactNested(got string) string {
	for _, key := range []string{`"user":{`, `"plan":{`} {
		start := strings.Index(got, key)
		if start < 0 {
			continue
		}
		open := start + len(key) - 1 // index of the '{'
		depth := 0
		end := -1
		for i := open; i < len(got); i++ {
			switch got[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = i
				}
			}
			if end >= 0 {
				break
			}
		}
		if end >= 0 {
			got = got[:open] + "{}" + got[end+1:]
		}
	}
	return got
}

// assertOrder fails if the keys do not appear in the given order in got.
func assertOrder(t *testing.T, got string, keys []string) {
	t.Helper()
	prev := -1
	for _, key := range keys {
		idx := strings.Index(got, `"`+key+`"`)
		if idx < 0 {
			t.Fatalf("missing key %q in %s", key, got)
		}
		if idx <= prev {
			t.Fatalf("key %q out of order in %s", key, got)
		}
		prev = idx
	}
}
