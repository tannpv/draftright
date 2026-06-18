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

	// LoadPaymentWithPlan knobs.
	loadRow AdminPaymentDetailRow
	loadErr error
	gotLoad string

	// ConfirmPayment knobs.
	confirmErr      error
	confirmCalls    int
	confirmID       string
	confirmComplete time.Time
	confirmNotes    string
}

func (f *fakeAdminStatsRepo) Stats(_ context.Context) (PaymentStats, error) {
	return f.stats, f.err
}

func (f *fakeAdminStatsRepo) FindAll(_ context.Context, p FindAllParams) ([]AdminPaymentRow, int, error) {
	f.gotParams = &p
	return f.findRows, f.findTotal, f.findErr
}

func (f *fakeAdminStatsRepo) LoadPaymentWithPlan(_ context.Context, id string) (AdminPaymentDetailRow, error) {
	f.gotLoad = id
	return f.loadRow, f.loadErr
}

func (f *fakeAdminStatsRepo) ConfirmPayment(_ context.Context, id string, completedAt time.Time, notes string) error {
	f.confirmCalls++
	f.confirmID = id
	f.confirmComplete = completedAt
	f.confirmNotes = notes
	return f.confirmErr
}

// fakeActivator captures the Activate args + call count and returns a canned
// error. Satisfies subscriptionActivator.
type fakeActivator struct {
	err   error
	calls int
	billing,
	userID,
	planID,
	method string
}

func (f *fakeActivator) Activate(_ context.Context, billing, userID, planID, method string) error {
	f.calls++
	f.billing, f.userID, f.planID, f.method = billing, userID, planID, method
	return f.err
}

// fixedNow is the deterministic clock injected into AdminConfirm tests.
var fixedNow = time.Date(2026, 6, 18, 9, 30, 0, 0, time.UTC)

func newConfirmSvc(repo paymentAdminRepo, act subscriptionActivator) *AdminService {
	return NewAdminService(repo, act, func() time.Time { return fixedNow })
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
	svc := newConfirmSvc(repo, &fakeActivator{})

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
	svc := newConfirmSvc(&fakeAdminStatsRepo{err: boom}, &fakeActivator{})

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
	svc := newConfirmSvc(repo, &fakeActivator{})

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
	svc := newConfirmSvc(&fakeAdminStatsRepo{findRows: nil, findTotal: 0}, &fakeActivator{})
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
	svc := newConfirmSvc(&fakeAdminStatsRepo{findErr: boom}, &fakeActivator{})
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

// pendingDetailRow builds a pending AdminPaymentDetailRow with a yearly plan
// loaded, used as the LoadPaymentWithPlan return for the happy-path tests.
func pendingDetailRow() AdminPaymentDetailRow {
	return AdminPaymentDetailRow{
		ID:            "pay1",
		UserID:        "u1",
		PlanID:        "pl1",
		Plan:          &plans.PlanEntity{ID: "pl1", Name: "Pro", BillingPeriod: "yearly"},
		Amount:        9900,
		Currency:      "USD",
		Method:        "bank_transfer",
		Status:        "pending",
		ReferenceCode: "DR-PRO-001",
	}
}

// TestAdminConfirm_NotFound: a not-found repo load returns ErrPaymentNotFound
// and the activator is never called.
func TestAdminConfirm_NotFound(t *testing.T) {
	repo := &fakeAdminStatsRepo{loadErr: ErrPaymentNotFound}
	act := &fakeActivator{}
	svc := newConfirmSvc(repo, act)

	_, err := svc.AdminConfirm(context.Background(), "missing", "")
	if !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("err = %v, want ErrPaymentNotFound", err)
	}
	if repo.confirmCalls != 0 {
		t.Fatalf("ConfirmPayment called %d times, want 0", repo.confirmCalls)
	}
	if act.calls != 0 {
		t.Fatalf("activator called %d times, want 0", act.calls)
	}
}

// TestAdminConfirm_NotPending: a non-pending payment returns ErrPaymentNotPending
// without confirming or activating.
func TestAdminConfirm_NotPending(t *testing.T) {
	row := pendingDetailRow()
	row.Status = "completed"
	repo := &fakeAdminStatsRepo{loadRow: row}
	act := &fakeActivator{}
	svc := newConfirmSvc(repo, act)

	_, err := svc.AdminConfirm(context.Background(), "pay1", "")
	if !errors.Is(err, ErrPaymentNotPending) {
		t.Fatalf("err = %v, want ErrPaymentNotPending", err)
	}
	if repo.confirmCalls != 0 {
		t.Fatalf("ConfirmPayment called %d times, want 0", repo.confirmCalls)
	}
	if act.calls != 0 {
		t.Fatalf("activator called %d times, want 0", act.calls)
	}
}

// TestAdminConfirm_HappyDefaultNotes: a pending payment with empty notes flips
// to completed, stamps the injected now, defaults the notes, and activates the
// subscription exactly once with (plan.billing_period, userID, planID, method).
func TestAdminConfirm_HappyDefaultNotes(t *testing.T) {
	repo := &fakeAdminStatsRepo{loadRow: pendingDetailRow()}
	act := &fakeActivator{}
	svc := newConfirmSvc(repo, act)

	got, err := svc.AdminConfirm(context.Background(), "pay1", "")
	if err != nil {
		t.Fatalf("AdminConfirm() error: %v", err)
	}
	if got.Status != string(StatusCompleted) {
		t.Fatalf("status = %q, want completed", got.Status)
	}
	if got.CompletedAt == nil || !got.CompletedAt.Equal(fixedNow) {
		t.Fatalf("completed_at = %v, want %v", got.CompletedAt, fixedNow)
	}
	if got.Notes == nil || *got.Notes != "Manually confirmed by admin" {
		t.Fatalf("notes = %v, want default", got.Notes)
	}
	// repo confirm received the same now + defaulted notes + id.
	if repo.confirmCalls != 1 || repo.confirmID != "pay1" ||
		!repo.confirmComplete.Equal(fixedNow) || repo.confirmNotes != "Manually confirmed by admin" {
		t.Fatalf("confirm args: calls=%d id=%q at=%v notes=%q",
			repo.confirmCalls, repo.confirmID, repo.confirmComplete, repo.confirmNotes)
	}
	// activator called once with the plan billing + payment fields.
	if act.calls != 1 {
		t.Fatalf("activator calls = %d, want 1", act.calls)
	}
	if act.billing != "yearly" || act.userID != "u1" || act.planID != "pl1" || act.method != "bank_transfer" {
		t.Fatalf("activate args = (%q,%q,%q,%q), want (yearly,u1,pl1,bank_transfer)",
			act.billing, act.userID, act.planID, act.method)
	}
}

// TestAdminConfirm_NotesPassthrough: a non-empty notes arg is stored verbatim.
func TestAdminConfirm_NotesPassthrough(t *testing.T) {
	repo := &fakeAdminStatsRepo{loadRow: pendingDetailRow()}
	svc := newConfirmSvc(repo, &fakeActivator{})

	got, err := svc.AdminConfirm(context.Background(), "pay1", "vip")
	if err != nil {
		t.Fatalf("AdminConfirm() error: %v", err)
	}
	if got.Notes == nil || *got.Notes != "vip" {
		t.Fatalf("notes = %v, want vip", got.Notes)
	}
	if repo.confirmNotes != "vip" {
		t.Fatalf("repo confirm notes = %q, want vip", repo.confirmNotes)
	}
}

// TestAdminConfirm_NilPlanBlankBilling: a nil plan relation yields a blank
// billing string passed to the activator (the webhook side defaults it).
func TestAdminConfirm_NilPlanBlankBilling(t *testing.T) {
	row := pendingDetailRow()
	row.Plan = nil
	repo := &fakeAdminStatsRepo{loadRow: row}
	act := &fakeActivator{}
	svc := newConfirmSvc(repo, act)

	if _, err := svc.AdminConfirm(context.Background(), "pay1", ""); err != nil {
		t.Fatalf("AdminConfirm() error: %v", err)
	}
	if act.billing != "" {
		t.Fatalf("billing = %q, want \"\" for nil plan", act.billing)
	}
}

// TestAdminConfirm_ConfirmError: a ConfirmPayment error propagates and the
// activator is not called.
func TestAdminConfirm_ConfirmError(t *testing.T) {
	boom := errors.New("update failed")
	repo := &fakeAdminStatsRepo{loadRow: pendingDetailRow(), confirmErr: boom}
	act := &fakeActivator{}
	svc := newConfirmSvc(repo, act)

	if _, err := svc.AdminConfirm(context.Background(), "pay1", ""); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if act.calls != 0 {
		t.Fatalf("activator called %d times, want 0", act.calls)
	}
}

// TestAdminConfirm_ActivatorError: an activation error propagates (the DB row is
// already flipped at this point — Node has the same ordering).
func TestAdminConfirm_ActivatorError(t *testing.T) {
	boom := errors.New("grant failed")
	repo := &fakeAdminStatsRepo{loadRow: pendingDetailRow()}
	act := &fakeActivator{err: boom}
	svc := newConfirmSvc(repo, act)

	if _, err := svc.AdminConfirm(context.Background(), "pay1", ""); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if repo.confirmCalls != 1 {
		t.Fatalf("ConfirmPayment called %d times, want 1", repo.confirmCalls)
	}
}

// TestPaymentRow_MarshalJSON_KeyOrder pins AdminPaymentDetailRow's 16-key order
// with NO `user` key, a nested plan, and the null cases (nil plan, nil
// completed_at).
func TestPaymentRow_MarshalJSON_KeyOrder(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 678_000_000, time.UTC)
	updated := time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)

	row := AdminPaymentDetailRow{
		ID:            "pay1",
		UserID:        "u1",
		PlanID:        "pl1",
		Plan:          &plans.PlanEntity{ID: "pl1", Name: "Pro", BillingPeriod: "monthly", CreatedAt: created, UpdatedAt: updated},
		Amount:        999,
		Currency:      "USD",
		Method:        "bank_transfer",
		Status:        "completed",
		ProviderRef:   sp("pi_1"),
		ReferenceCode: "DR-PRO-001",
		QRData:        nil,
		Notes:         sp("ok"),
		ExpiresAt:     nil,
		CompletedAt:   nil,
		CreatedAt:     created,
		UpdatedAt:     updated,
	}

	b, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	keys := []string{
		"id", "user_id", "plan_id", "plan", "amount", "currency", "method",
		"status", "provider_ref", "reference_code", "qr_data", "notes",
		"expires_at", "completed_at", "created_at", "updated_at",
	}
	assertOrder(t, redactNested(got), keys)

	// No top-level `user` key (relations:['plan'] only → user undefined → omitted).
	if strings.Contains(redactNested(got), `"user"`) {
		t.Fatalf("must NOT contain a user key, got %s", got)
	}
	// Nested plan present, opens with id.
	if plStart := strings.Index(got, `"plan":{`); plStart < 0 || !strings.HasPrefix(got[plStart:], `"plan":{"id":`) {
		t.Fatalf(`nested plan must open with "id", got %s`, got)
	}
	// nil completed_at → null.
	if !strings.Contains(got, `"completed_at":null`) {
		t.Fatalf("nil completed_at must render null, got %s", got)
	}

	// nil plan → "plan":null.
	row.Plan = nil
	b2, _ := json.Marshal(row)
	if !strings.Contains(string(b2), `"plan":null`) {
		t.Fatalf(`nil plan must render null, got %s`, b2)
	}
}
