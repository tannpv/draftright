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

	// RefundPayment knobs.
	refundErr   error
	refundCalls int
	refundID    string
	refundNotes string
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

func (f *fakeAdminStatsRepo) RefundPayment(_ context.Context, id, notes string) error {
	f.refundCalls++
	f.refundID = id
	f.refundNotes = notes
	return f.refundErr
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

// fakeRefunder captures every stripeRefunder call + returns canned results.
type fakeRefunder struct {
	chargeID  string
	chargeErr error
	chargeCalls,
	createCalls,
	cancelCalls int
	gotChargeSub,
	gotRefundCharge,
	gotRefundReason,
	gotCancelSub string
	refundID  string
	createErr error
	cancelErr error
}

func (f *fakeRefunder) LatestInvoiceCharge(_ context.Context, _, subID string) (string, error) {
	f.chargeCalls++
	f.gotChargeSub = subID
	return f.chargeID, f.chargeErr
}

func (f *fakeRefunder) CreateRefund(_ context.Context, _, chargeID, reason string) (string, error) {
	f.createCalls++
	f.gotRefundCharge, f.gotRefundReason = chargeID, reason
	return f.refundID, f.createErr
}

func (f *fakeRefunder) CancelSubscription(_ context.Context, _, subID string) error {
	f.cancelCalls++
	f.gotCancelSub = subID
	return f.cancelErr
}

// fakeSecretSource returns a canned Stripe secret key / error.
type fakeSecretSource struct {
	key string
	err error
}

func (f *fakeSecretSource) StripeSecretKey(_ context.Context) (string, error) {
	return f.key, f.err
}

// fakeSubLookup returns a canned latest-Stripe-sub lookup result.
type fakeSubLookup struct {
	subID string
	found bool
	err   error
	gotUserID,
	gotPlanID string
}

func (f *fakeSubLookup) FindLatestStripeForUserPlan(_ context.Context, userID, planID string) (string, bool, error) {
	f.gotUserID, f.gotPlanID = userID, planID
	return f.subID, f.found, f.err
}

// fakeSubCanceller captures the CancelByStoreRef call.
type fakeSubCanceller struct {
	err   error
	calls int
	gotStoreType,
	gotStoreRef string
}

func (f *fakeSubCanceller) CancelByStoreRef(_ context.Context, storeType, storeRef string) error {
	f.calls++
	f.gotStoreType, f.gotStoreRef = storeType, storeRef
	return f.err
}

func newConfirmSvc(repo paymentAdminRepo, act subscriptionActivator) *AdminService {
	return NewAdminService(repo, act, nil, nil, nil, nil, nil, func() time.Time { return fixedNow })
}

// newRefundSvc wires the refund-side deps; the activator is unused by Refund so
// a default fake is fine.
func newRefundSvc(repo paymentAdminRepo, ref stripeRefunder, sec stripeSecretSource, lookup stripeSubLookup, canceller subscriptionCanceller) *AdminService {
	return NewAdminService(repo, &fakeActivator{}, ref, sec, lookup, canceller, nil, func() time.Time { return fixedNow })
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
// nested `user` (stripped of the six secret columns, #48) + raw `plan` shapes,
// and the null cases for a missing relation / a nil nullable timestamp.
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

	// Nested user opens with id and is STRIPPED of the six secret columns (#48):
	// password_hash + email/password reset/verification codes must be absent.
	uStart := strings.Index(got, `"user":{`)
	if uStart < 0 {
		t.Fatalf(`missing "user":{ in %s`, got)
	}
	if !strings.HasPrefix(got[uStart:], `"user":{"id":`) {
		t.Fatalf(`nested user must open with "id", got %s`, got[uStart:uStart+40])
	}
	for _, secret := range []string{
		"password_hash", "email_verification_code", "email_verification_expires",
		"password_reset_code", "password_reset_expires", "password_reset_attempts",
	} {
		if strings.Contains(got, `"`+secret+`"`) {
			t.Fatalf("nested user must NOT expose %s (#48), got %s", secret, got)
		}
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

// ── Refund ────────────────────────────────────────────────────────────────

// completedStripeRow builds a completed stripe payment, the canonical refund
// happy-path input.
func completedStripeRow() AdminPaymentDetailRow {
	r := pendingDetailRow()
	r.Status = string(StatusCompleted)
	r.Method = string(MethodStripe)
	return r
}

// TestRefund_NotFound: a not-found load returns ErrPaymentNotFound and no
// Stripe / persist call fires.
func TestRefund_NotFound(t *testing.T) {
	repo := &fakeAdminStatsRepo{loadErr: ErrPaymentNotFound}
	ref := &fakeRefunder{}
	svc := newRefundSvc(repo, ref, &fakeSecretSource{key: "sk"}, &fakeSubLookup{}, &fakeSubCanceller{})

	_, err := svc.Refund(context.Background(), "missing", "")
	if !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("err = %v, want ErrPaymentNotFound", err)
	}
	if ref.chargeCalls+ref.createCalls+ref.cancelCalls != 0 || repo.refundCalls != 0 {
		t.Fatalf("no stripe/persist calls expected on not-found")
	}
}

// TestRefund_AlreadyRefunded: a refunded payment is a no-op — returns the row,
// nil error, NO persist, NO stripe.
func TestRefund_AlreadyRefunded(t *testing.T) {
	row := completedStripeRow()
	row.Status = string(StatusRefunded)
	repo := &fakeAdminStatsRepo{loadRow: row}
	ref := &fakeRefunder{}
	svc := newRefundSvc(repo, ref, &fakeSecretSource{key: "sk"}, &fakeSubLookup{}, &fakeSubCanceller{})

	got, err := svc.Refund(context.Background(), "pay1", "")
	if err != nil {
		t.Fatalf("Refund() error: %v", err)
	}
	if got.Status != string(StatusRefunded) {
		t.Fatalf("status = %q, want refunded", got.Status)
	}
	if ref.chargeCalls+ref.createCalls+ref.cancelCalls != 0 || repo.refundCalls != 0 {
		t.Fatalf("already-refunded must not call stripe or persist")
	}
}

// TestRefund_NotCompleted: a pending payment returns ErrRefundNotCompleted.
func TestRefund_NotCompleted(t *testing.T) {
	repo := &fakeAdminStatsRepo{loadRow: pendingDetailRow()} // status=pending, method=bank_transfer
	svc := newRefundSvc(repo, &fakeRefunder{}, &fakeSecretSource{key: "sk"}, &fakeSubLookup{}, &fakeSubCanceller{})

	_, err := svc.Refund(context.Background(), "pay1", "")
	if !errors.Is(err, ErrRefundNotCompleted) {
		t.Fatalf("err = %v, want ErrRefundNotCompleted", err)
	}
	if err.Error() != "Only completed payments can be refunded" {
		t.Fatalf("msg = %q", err.Error())
	}
}

// TestRefund_UnsupportedMethod: a completed non-stripe payment returns a dynamic
// 400 with the method interpolated, single-quoted.
func TestRefund_UnsupportedMethod(t *testing.T) {
	row := completedStripeRow()
	row.Method = "paypal"
	repo := &fakeAdminStatsRepo{loadRow: row}
	svc := newRefundSvc(repo, &fakeRefunder{}, &fakeSecretSource{key: "sk"}, &fakeSubLookup{}, &fakeSubCanceller{})

	_, err := svc.Refund(context.Background(), "pay1", "")
	if err == nil || err.Error() != "Refund not supported for method 'paypal'" {
		t.Fatalf("err = %v, want \"Refund not supported for method 'paypal'\"", err)
	}
	var de *DomainError
	if !errors.As(err, &de) || de.Status != 400 {
		t.Fatalf("want a 400 DomainError, got %v", err)
	}
}

// TestRefund_MissingKey: an empty Stripe key returns ErrStripeKeyMissing.
func TestRefund_MissingKey(t *testing.T) {
	repo := &fakeAdminStatsRepo{loadRow: completedStripeRow()}
	svc := newRefundSvc(repo, &fakeRefunder{}, &fakeSecretSource{key: ""}, &fakeSubLookup{}, &fakeSubCanceller{})

	_, err := svc.Refund(context.Background(), "pay1", "")
	if !errors.Is(err, ErrStripeKeyMissing) {
		t.Fatalf("err = %v, want ErrStripeKeyMissing", err)
	}
	if err.Error() != "Stripe secret_key is not configured" {
		t.Fatalf("msg = %q", err.Error())
	}
}

// TestRefund_SecretSourceError: a settings lookup error propagates (500).
func TestRefund_SecretSourceError(t *testing.T) {
	boom := errors.New("settings down")
	repo := &fakeAdminStatsRepo{loadRow: completedStripeRow()}
	svc := newRefundSvc(repo, &fakeRefunder{}, &fakeSecretSource{err: boom}, &fakeSubLookup{}, &fakeSubCanceller{})

	if _, err := svc.Refund(context.Background(), "pay1", ""); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

// emDashNote is the exact em-dash segment Node emits: space U+2014 space.
const emDashNote = " — Stripe refund "

// TestRefund_HappySubWithCharge: full happy path — charge resolved, refund
// created, sub cancelled, note carries reason + " — Stripe refund re_1", our DB
// sub cancelled, RefundPayment persisted with the final note.
func TestRefund_HappySubWithCharge(t *testing.T) {
	repo := &fakeAdminStatsRepo{loadRow: completedStripeRow()}
	ref := &fakeRefunder{chargeID: "ch_1", refundID: "re_1"}
	lookup := &fakeSubLookup{subID: "sub_1", found: true}
	canceller := &fakeSubCanceller{}
	svc := newRefundSvc(repo, ref, &fakeSecretSource{key: "sk"}, lookup, canceller)

	got, err := svc.Refund(context.Background(), "pay1", "vip")
	if err != nil {
		t.Fatalf("Refund() error: %v", err)
	}
	if ref.chargeCalls != 1 || ref.gotChargeSub != "sub_1" {
		t.Fatalf("LatestInvoiceCharge calls=%d sub=%q", ref.chargeCalls, ref.gotChargeSub)
	}
	if ref.createCalls != 1 || ref.gotRefundCharge != "ch_1" || ref.gotRefundReason != "vip" {
		t.Fatalf("CreateRefund calls=%d charge=%q reason=%q", ref.createCalls, ref.gotRefundCharge, ref.gotRefundReason)
	}
	if ref.cancelCalls != 1 || ref.gotCancelSub != "sub_1" {
		t.Fatalf("CancelSubscription calls=%d sub=%q", ref.cancelCalls, ref.gotCancelSub)
	}
	want := "Refunded by admin (vip)" + emDashNote + "re_1"
	if got.Status != string(StatusRefunded) {
		t.Fatalf("status = %q, want refunded", got.Status)
	}
	if got.Notes == nil || *got.Notes != want {
		t.Fatalf("notes = %v, want %q", got.Notes, want)
	}
	// Byte-exact em-dash assertion (U+2014 = 0xE2 0x80 0x94).
	if !strings.Contains(*got.Notes, "—") || !strings.Contains(*got.Notes, " — Stripe refund re_1") {
		t.Fatalf("em-dash segment missing/mismatched in %q", *got.Notes)
	}
	if repo.refundCalls != 1 || repo.refundID != "pay1" || repo.refundNotes != want {
		t.Fatalf("RefundPayment calls=%d id=%q notes=%q", repo.refundCalls, repo.refundID, repo.refundNotes)
	}
	if canceller.calls != 1 || canceller.gotStoreType != "stripe" || canceller.gotStoreRef != "sub_1" {
		t.Fatalf("CancelByStoreRef calls=%d type=%q ref=%q", canceller.calls, canceller.gotStoreType, canceller.gotStoreRef)
	}
}

// TestRefund_HappySubNoCharge: sub present but no charge → CreateRefund NOT
// called, note has no "Stripe refund", CancelSubscription STILL called,
// CancelByStoreRef STILL called.
func TestRefund_HappySubNoCharge(t *testing.T) {
	repo := &fakeAdminStatsRepo{loadRow: completedStripeRow()}
	ref := &fakeRefunder{chargeID: ""} // no charge resolvable
	lookup := &fakeSubLookup{subID: "sub_1", found: true}
	canceller := &fakeSubCanceller{}
	svc := newRefundSvc(repo, ref, &fakeSecretSource{key: "sk"}, lookup, canceller)

	got, err := svc.Refund(context.Background(), "pay1", "")
	if err != nil {
		t.Fatalf("Refund() error: %v", err)
	}
	if ref.createCalls != 0 {
		t.Fatalf("CreateRefund must NOT be called without a charge")
	}
	if ref.cancelCalls != 1 {
		t.Fatalf("CancelSubscription must still run, calls=%d", ref.cancelCalls)
	}
	if got.Notes == nil || *got.Notes != "Refunded by admin" {
		t.Fatalf("notes = %v, want \"Refunded by admin\"", got.Notes)
	}
	if strings.Contains(*got.Notes, "Stripe refund") {
		t.Fatalf("note must NOT mention Stripe refund, got %q", *got.Notes)
	}
	if canceller.calls != 1 {
		t.Fatalf("CancelByStoreRef must still run, calls=%d", canceller.calls)
	}
}

// TestRefund_NoSub: no linked sub → zero stripe calls, status refunded, note
// has no "Stripe refund", CancelByStoreRef NOT called.
func TestRefund_NoSub(t *testing.T) {
	repo := &fakeAdminStatsRepo{loadRow: completedStripeRow()}
	ref := &fakeRefunder{}
	lookup := &fakeSubLookup{found: false}
	canceller := &fakeSubCanceller{}
	svc := newRefundSvc(repo, ref, &fakeSecretSource{key: "sk"}, lookup, canceller)

	got, err := svc.Refund(context.Background(), "pay1", "")
	if err != nil {
		t.Fatalf("Refund() error: %v", err)
	}
	if ref.chargeCalls+ref.createCalls+ref.cancelCalls != 0 {
		t.Fatalf("no-sub must make zero stripe calls")
	}
	if got.Status != string(StatusRefunded) {
		t.Fatalf("status = %q, want refunded", got.Status)
	}
	if got.Notes == nil || *got.Notes != "Refunded by admin" {
		t.Fatalf("notes = %v, want \"Refunded by admin\"", got.Notes)
	}
	if strings.Contains(*got.Notes, "Stripe refund") {
		t.Fatalf("no-sub note must not mention Stripe refund")
	}
	if canceller.calls != 0 {
		t.Fatalf("CancelByStoreRef must NOT run without a sub, calls=%d", canceller.calls)
	}
	if repo.refundCalls != 1 || repo.refundNotes != "Refunded by admin" {
		t.Fatalf("RefundPayment calls=%d notes=%q", repo.refundCalls, repo.refundNotes)
	}
}

// TestRefund_NotesJoinExisting: a payment with existing notes prepends them via
// " | ".
func TestRefund_NotesJoinExisting(t *testing.T) {
	row := completedStripeRow()
	row.Notes = sp("old")
	repo := &fakeAdminStatsRepo{loadRow: row}
	ref := &fakeRefunder{chargeID: "ch_1", refundID: "re_1"}
	lookup := &fakeSubLookup{subID: "sub_1", found: true}
	svc := newRefundSvc(repo, ref, &fakeSecretSource{key: "sk"}, lookup, &fakeSubCanceller{})

	got, err := svc.Refund(context.Background(), "pay1", "vip")
	if err != nil {
		t.Fatalf("Refund() error: %v", err)
	}
	want := "old | Refunded by admin (vip)" + emDashNote + "re_1"
	if got.Notes == nil || *got.Notes != want {
		t.Fatalf("notes = %v, want %q", got.Notes, want)
	}
}

// TestRefund_NotesNilExisting: a nil existing notes → just the new note (no
// leading separator).
func TestRefund_NotesNilExisting(t *testing.T) {
	row := completedStripeRow() // Notes is nil
	repo := &fakeAdminStatsRepo{loadRow: row}
	ref := &fakeRefunder{chargeID: "ch_1", refundID: "re_1"}
	lookup := &fakeSubLookup{subID: "sub_1", found: true}
	svc := newRefundSvc(repo, ref, &fakeSecretSource{key: "sk"}, lookup, &fakeSubCanceller{})

	got, err := svc.Refund(context.Background(), "pay1", "")
	if err != nil {
		t.Fatalf("Refund() error: %v", err)
	}
	want := "Refunded by admin" + emDashNote + "re_1"
	if got.Notes == nil || *got.Notes != want {
		t.Fatalf("notes = %v, want %q", got.Notes, want)
	}
}

// TestRefund_CancelSubscriptionNonFatal: a Stripe sub-cancel error is swallowed
// — refund still succeeds, status refunded, persist + DB cancel still run.
func TestRefund_CancelSubscriptionNonFatal(t *testing.T) {
	repo := &fakeAdminStatsRepo{loadRow: completedStripeRow()}
	ref := &fakeRefunder{chargeID: "ch_1", refundID: "re_1", cancelErr: errors.New("already cancelled")}
	lookup := &fakeSubLookup{subID: "sub_1", found: true}
	canceller := &fakeSubCanceller{}
	svc := newRefundSvc(repo, ref, &fakeSecretSource{key: "sk"}, lookup, canceller)

	got, err := svc.Refund(context.Background(), "pay1", "")
	if err != nil {
		t.Fatalf("Refund() error: %v (cancel error must be non-fatal)", err)
	}
	if got.Status != string(StatusRefunded) {
		t.Fatalf("status = %q, want refunded", got.Status)
	}
	if repo.refundCalls != 1 || canceller.calls != 1 {
		t.Fatalf("persist/DB-cancel must still run, refundCalls=%d cancelByRef=%d", repo.refundCalls, canceller.calls)
	}
}

// TestRefund_ChargeLookupError: a LatestInvoiceCharge error propagates (500).
func TestRefund_ChargeLookupError(t *testing.T) {
	boom := errors.New("stripe retrieve failed")
	repo := &fakeAdminStatsRepo{loadRow: completedStripeRow()}
	ref := &fakeRefunder{chargeErr: boom}
	lookup := &fakeSubLookup{subID: "sub_1", found: true}
	svc := newRefundSvc(repo, ref, &fakeSecretSource{key: "sk"}, lookup, &fakeSubCanceller{})

	if _, err := svc.Refund(context.Background(), "pay1", ""); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if repo.refundCalls != 0 {
		t.Fatalf("no persist on charge-lookup error")
	}
}

// TestRefund_CreateRefundError: a CreateRefund error propagates (500).
func TestRefund_CreateRefundError(t *testing.T) {
	boom := errors.New("refund create failed")
	repo := &fakeAdminStatsRepo{loadRow: completedStripeRow()}
	ref := &fakeRefunder{chargeID: "ch_1", createErr: boom}
	lookup := &fakeSubLookup{subID: "sub_1", found: true}
	svc := newRefundSvc(repo, ref, &fakeSecretSource{key: "sk"}, lookup, &fakeSubCanceller{})

	if _, err := svc.Refund(context.Background(), "pay1", ""); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if repo.refundCalls != 0 {
		t.Fatalf("no persist on create-refund error")
	}
}

// TestRefund_PersistError: a RefundPayment error propagates (500).
func TestRefund_PersistError(t *testing.T) {
	boom := errors.New("update failed")
	repo := &fakeAdminStatsRepo{loadRow: completedStripeRow(), refundErr: boom}
	ref := &fakeRefunder{chargeID: "ch_1", refundID: "re_1"}
	lookup := &fakeSubLookup{subID: "sub_1", found: true}
	svc := newRefundSvc(repo, ref, &fakeSecretSource{key: "sk"}, lookup, &fakeSubCanceller{})

	if _, err := svc.Refund(context.Background(), "pay1", ""); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

// TestRefund_CancelByStoreRefError: a CancelByStoreRef error propagates (500,
// Node's cancelByStripeSubId is not wrapped in try/catch).
func TestRefund_CancelByStoreRefError(t *testing.T) {
	boom := errors.New("db cancel failed")
	repo := &fakeAdminStatsRepo{loadRow: completedStripeRow()}
	ref := &fakeRefunder{chargeID: "ch_1", refundID: "re_1"}
	lookup := &fakeSubLookup{subID: "sub_1", found: true}
	canceller := &fakeSubCanceller{err: boom}
	svc := newRefundSvc(repo, ref, &fakeSecretSource{key: "sk"}, lookup, canceller)

	if _, err := svc.Refund(context.Background(), "pay1", ""); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}
