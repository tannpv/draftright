package payment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// --- fakes ---
type fakeCheckoutRepo struct {
	plan      *CheckoutPlan
	user      *CheckoutUser
	created   *CreatedPayment
	createErr error
	qrCalls   int
	failCalls int
	failNotes string
}

func (f *fakeCheckoutRepo) PlanForCheckout(_ context.Context, _ string) (*CheckoutPlan, error) {
	return f.plan, nil
}
func (f *fakeCheckoutRepo) UserForCheckout(_ context.Context, _ string) (*CheckoutUser, error) {
	return f.user, nil
}
func (f *fakeCheckoutRepo) CreatePayment(_ context.Context, _, _ string, _ int, _, _, _, _ string, _ time.Time) (*CreatedPayment, error) {
	return f.created, f.createErr
}
func (f *fakeCheckoutRepo) UpdateQRData(_ context.Context, _, _ string) error {
	f.qrCalls++
	return nil
}
func (f *fakeCheckoutRepo) MarkFailed(_ context.Context, _, notes string) error {
	f.failCalls++
	f.failNotes = notes
	return nil
}

type fakeStrategy struct {
	res strategy.Result
	err error
}

func (f fakeStrategy) CreateCheckout(context.Context, strategy.Payment, strategy.Plan, strategy.Options) (strategy.Result, error) {
	return f.res, f.err
}
func (f fakeStrategy) CustomerPortalURL(context.Context, strategy.PortalUser) (string, error) {
	return "", nil
}
func (f fakeStrategy) CancelSubscription(context.Context, string) (bool, error) { return false, nil }
func (f fakeStrategy) VerifyWebhook(context.Context, []byte, http.Header) (strategy.WebhookAction, error) {
	return strategy.Ignored(), nil
}

func errProvider(s string) error { return errors.New(s) }

func assertDomainErr(t *testing.T, err error, status int, msg string) {
	t.Helper()
	var de *DomainError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DomainError, got %T (%v)", err, err)
	}
	if de.Status != status || de.Message != msg {
		t.Fatalf("got {status:%d msg:%q}, want {status:%d msg:%q}", de.Status, de.Message, status, msg)
	}
}

func newTestService(repo CheckoutRepo, strategies map[string]strategy.Strategy, enabled []string) *Service {
	// White-box construction: a Service whose EnabledMethods resolves to `enabled`.
	// Feed `enabled` as the settings CSV (found=true) so the existing precedence
	// returns it; EnabledMethods then applies its normal expansion/dedupe.
	return &Service{
		repo:         nil,
		settings:     fixedSettings(enabled),
		envCSV:       "",
		checkoutRepo: repo,
		strategies:   strategies,
		now:          func() time.Time { return time.Unix(1700000000, 0).UTC() },
		genRef:       func() string { return "DR-PRO-TESTREF" },
	}
}

type fixedSettings []string

func (f fixedSettings) PaymentMethodsEnabled(context.Context) (string, bool, error) {
	csv := ""
	for i, m := range f {
		if i > 0 {
			csv += ","
		}
		csv += m
	}
	return csv, true, nil
}

func svcWith(repo CheckoutRepo, strat strategy.Strategy, enabled []string) *Service {
	return newTestService(repo, map[string]strategy.Strategy{"vietqr": strat, "stripe": strat}, enabled)
}

func TestCheckout_PlanNotFound(t *testing.T) {
	s := svcWith(&fakeCheckoutRepo{plan: nil}, fakeStrategy{}, []string{"vietqr"})
	_, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	assertDomainErr(t, err, 404, "Plan not found")
}

func TestCheckout_FreePlan(t *testing.T) {
	s := svcWith(&fakeCheckoutRepo{plan: &CheckoutPlan{PriceCents: 0}}, fakeStrategy{}, []string{"vietqr"})
	_, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	assertDomainErr(t, err, 400, "Cannot purchase a free plan")
}

func TestCheckout_MethodNotEnabled(t *testing.T) {
	s := svcWith(&fakeCheckoutRepo{plan: &CheckoutPlan{PriceCents: 999}}, fakeStrategy{}, []string{"stripe"})
	_, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	assertDomainErr(t, err, 404, "Payment method 'vietqr' is not enabled.")
}

func TestCheckout_UserNotFound(t *testing.T) {
	s := svcWith(&fakeCheckoutRepo{plan: &CheckoutPlan{PriceCents: 999}, user: nil}, fakeStrategy{}, []string{"vietqr"})
	_, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	assertDomainErr(t, err, 404, "User not found")
}

func TestCheckout_StrategyErrorMarksFailed(t *testing.T) {
	repo := &fakeCheckoutRepo{
		plan:    &CheckoutPlan{PriceCents: 999, Currency: "VND"},
		user:    &CheckoutUser{ID: "u1", Email: "u@x.com"},
		created: &CreatedPayment{ID: "pay1"},
	}
	s := svcWith(repo, fakeStrategy{err: errProvider("boom")}, []string{"vietqr"})
	_, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	assertDomainErr(t, err, 400, "boom")
	if repo.failCalls != 1 || repo.failNotes != "boom" {
		t.Fatalf("must MarkFailed with err message, calls=%d notes=%q", repo.failCalls, repo.failNotes)
	}
}

func TestCheckout_StrategyEmptyErrorFallsBack(t *testing.T) {
	repo := &fakeCheckoutRepo{
		plan:    &CheckoutPlan{PriceCents: 999, Currency: "VND"},
		user:    &CheckoutUser{ID: "u1", Email: "u@x.com"},
		created: &CreatedPayment{ID: "pay1"},
	}
	s := svcWith(repo, fakeStrategy{err: errProvider("")}, []string{"vietqr"})
	_, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	assertDomainErr(t, err, 400, "Payment provider error")
	if repo.failCalls != 1 || repo.failNotes != "" {
		t.Fatalf("MarkFailed must carry the raw (empty) message, calls=%d notes=%q", repo.failCalls, repo.failNotes)
	}
}

func TestCheckout_VietQRSuccessPersistsQR(t *testing.T) {
	repo := &fakeCheckoutRepo{
		plan:    &CheckoutPlan{Name: "Pro", PriceCents: 124000, Currency: "VND"},
		user:    &CheckoutUser{ID: "u1", Email: "u@x.com"},
		created: &CreatedPayment{ID: "pay1"},
	}
	strat := fakeStrategy{res: strategy.Result{QRData: "https://img.vietqr.io/...", BankInfo: &strategy.BankInfo{BankName: "MB Bank (Quân Đội)"}}}
	s := svcWith(repo, strat, []string{"vietqr"})
	resp, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.qrCalls != 1 {
		t.Fatalf("qr_data must persist, calls=%d", repo.qrCalls)
	}
	if resp.QRData == nil || *resp.QRData == "" || resp.BankInfo == nil {
		t.Fatalf("response must carry qr_data + bank_info: %+v", resp)
	}
	if resp.Payment.Status != "pending" || resp.Payment.Amount != 124000 || resp.Payment.Currency != "VND" {
		t.Fatalf("payment shape wrong: %+v", resp.Payment)
	}
}

// TestCheckout_NestedPlanCarriesFullEntity proves the response's nested
// payment.plan serializes the FULL plan entity (daily_limit/is_active/
// created_at/updated_at) with real DB values — Node attaches
// plansService.findById, the complete row, so the Go port must too. Before
// the fix these were hardcoded (is_active=true) or zero (daily_limit=0,
// zero-time timestamps).
func TestCheckout_NestedPlanCarriesFullEntity(t *testing.T) {
	createdAt := time.Unix(1600000000, 0).UTC()
	updatedAt := time.Unix(1600000001, 0).UTC()
	repo := &fakeCheckoutRepo{
		plan: &CheckoutPlan{
			Name:          "Pro",
			DailyLimit:    100,
			PriceCents:    124000,
			Currency:      "VND",
			TrialDays:     7,
			BillingPeriod: "monthly",
			IsActive:      true,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		},
		user:    &CheckoutUser{ID: "u1", Email: "u@x.com"},
		created: &CreatedPayment{ID: "pay1"},
	}
	strat := fakeStrategy{res: strategy.Result{QRData: "https://img.vietqr.io/...", BankInfo: &strategy.BankInfo{BankName: "MB Bank"}}}
	s := svcWith(repo, strat, []string{"vietqr"})
	resp, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	if err != nil {
		t.Fatal(err)
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)

	wantSubstrings := []string{
		`"daily_limit":100`,
		`"is_active":true`,
		`"created_at":"` + shared.ISOMillis(createdAt) + `"`,
		`"updated_at":"` + shared.ISOMillis(updatedAt) + `"`,
	}
	for _, w := range wantSubstrings {
		if !strings.Contains(got, w) {
			t.Fatalf("nested plan missing %s in response JSON:\n%s", w, got)
		}
	}

	// Direct struct assertions on the nested plan too.
	if resp.Payment.Plan == nil {
		t.Fatal("nested plan must be present")
	}
	p := resp.Payment.Plan
	if p.DailyLimit != 100 || !p.IsActive || !p.CreatedAt.Equal(createdAt) || !p.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("nested plan carries stale values: %+v", p)
	}
}
