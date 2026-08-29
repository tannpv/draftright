package payment

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
)

// --- fakes ---------------------------------------------------------------

// fakeVerifier is a one-method Strategy: VerifyWebhook returns a canned action
// (or error); the other three methods are unused by HandleWebhook.
type fakeVerifier struct {
	action strategy.WebhookAction
	err    error
}

func (f fakeVerifier) CreateCheckout(context.Context, strategy.Payment, strategy.Plan, strategy.Options) (strategy.Result, error) {
	return strategy.Result{}, nil
}
func (f fakeVerifier) CustomerPortalURL(context.Context, strategy.PortalUser) (string, error) {
	return "", nil
}
func (f fakeVerifier) CancelSubscription(context.Context, string) (bool, error) { return false, nil }
func (f fakeVerifier) VerifyWebhook(context.Context, []byte, http.Header) (strategy.WebhookAction, error) {
	return f.action, f.err
}

type fakeWebhookRepo struct {
	pay     *WebhookPayment
	payErr  error
	planID  string
	planErr error

	completedRef string
	failedRef    string
	stripeCustID string
	lsCustID     string
	planUpdated  string // "paymentID:planID"
}

func (f *fakeWebhookRepo) PaymentForWebhook(ctx context.Context, ref string) (*WebhookPayment, error) {
	return f.pay, f.payErr
}
func (f *fakeWebhookRepo) MarkPaymentCompleted(ctx context.Context, ref string) error {
	f.completedRef = ref
	if f.pay != nil {
		f.pay.Status = "completed"
	}
	return nil
}
func (f *fakeWebhookRepo) MarkPaymentFailedByRef(ctx context.Context, ref string) error {
	f.failedRef = ref
	if f.pay != nil {
		f.pay.Status = "failed"
	}
	return nil
}
func (f *fakeWebhookRepo) SetStripeCustomerID(ctx context.Context, userID, customerID string) error {
	f.stripeCustID = customerID
	return nil
}
func (f *fakeWebhookRepo) SetLemonSqueezyCustomerID(ctx context.Context, userID, customerID string) error {
	f.lsCustID = customerID
	return nil
}
func (f *fakeWebhookRepo) UpdatePaymentPlan(ctx context.Context, paymentID, planID string) error {
	f.planUpdated = paymentID + ":" + planID
	return nil
}
func (f *fakeWebhookRepo) FindFirstActivePlanID(ctx context.Context, billingPeriod, currency string) (string, error) {
	return f.planID, f.planErr
}
func (f *fakeWebhookRepo) UserEmailName(ctx context.Context, userID string) (string, string, error) {
	return "user@example.com", "Test User", nil
}

type fakeSubsWriter struct {
	granted        bool
	grantStore     string
	stamped        string // "storeType:txnID"
	stampByUserRef string // txnID passed to StampStoreRefByUser (IAP redeem path)
	extended       string
	cancelled      string
	expired        string
	found          *subscription.StoreRefSub
	extendNoMatch  bool // ExtendByStoreRef reports 0 rows matched (simulates a lost first /redeem)
}

func (f *fakeSubsWriter) Grant(ctx context.Context, userID, planID, storeType string, expiresAt *time.Time) error {
	f.granted = true
	f.grantStore = storeType
	return nil
}
func (f *fakeSubsWriter) StampStoreRef(ctx context.Context, referenceCode, storeType, transactionID string) error {
	f.stamped = storeType + ":" + transactionID
	return nil
}
func (f *fakeSubsWriter) StampStoreRefByUser(ctx context.Context, userID, storeType, transactionID string) error {
	f.stampByUserRef = transactionID
	return nil
}
func (f *fakeSubsWriter) ExtendByStoreRef(ctx context.Context, storeType, transactionID string, expiresAt time.Time) (int64, error) {
	f.extended = storeType + ":" + transactionID
	if f.extendNoMatch {
		return 0, nil
	}
	return 1, nil
}
func (f *fakeSubsWriter) CancelByStoreRef(ctx context.Context, storeType, transactionID string) (int64, error) {
	f.cancelled = storeType + ":" + transactionID
	return 1, nil
}
func (f *fakeSubsWriter) ExpireByStoreRef(ctx context.Context, storeType, transactionID string) (int64, error) {
	f.expired = storeType + ":" + transactionID
	return 1, nil
}
func (f *fakeSubsWriter) FindByStoreRef(ctx context.Context, storeType, transactionID string) (*subscription.StoreRefSub, error) {
	return f.found, nil
}

type noopEmailer struct {
	activatedTo    string
	activatedCalls int
	failedTo       string
	failed         bool
	failedName     string
}

func (e *noopEmailer) SubscriptionActivated(ctx context.Context, to, name, planName string) {
	e.activatedTo = to
	e.activatedCalls++
}
func (e *noopEmailer) PaymentFailed(ctx context.Context, to, name, planName string) {
	e.failedTo = to
	e.failed = true
	e.failedName = name
}

type fakeVariants struct {
	monthly, yearly           string // Lemon Squeezy variant ids
	appleMonthly, appleYearly string // App Store product ids
	appleErr                  error
}

func (f fakeVariants) LemonSqueezyVariants(ctx context.Context) (string, string, error) {
	return f.monthly, f.yearly, nil
}

func (f fakeVariants) AppleProducts(ctx context.Context) (string, string, error) {
	return f.appleMonthly, f.appleYearly, f.appleErr
}

// webhookSvc builds a Service wired for the webhook path with the given
// collaborators + a fixed clock (so activateSubscription's s.now() never panics).
func webhookSvc(settings fakeSettings, strat strategy.Strategy, repo WebhookRepo, subs SubsWriter, emailer WebhookEmailer, variants VariantResolver) *Service {
	svc := &Service{
		settings:   settings,
		strategies: map[string]strategy.Strategy{"stripe": strat, "vietqr": strat, "lemonsqueezy": strat, "bank_transfer": strat, "paypal": strat, string(MethodAppleIAP): strat},
		now:        func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}
	return svc.WithWebhook(repo, subs, emailer, variants)
}

// newTestServiceWithAppleProducts wires a Service for resolvePlanIDFromAppleProduct
// tests only: a fake WebhookRepo whose FindFirstActivePlanID returns a stub
// plan id, and the given Apple product ids in the credentials resolver.
// Mirrors the LS resolver's test setup (webhookSvc), scoped to the two
// collaborators the resolver actually touches.
func newTestServiceWithAppleProducts(t *testing.T, monthly, yearly string) *Service {
	t.Helper()
	repo := &fakeWebhookRepo{planID: "pl_apple_stub"}
	svc := &Service{}
	return svc.WithWebhook(repo, &fakeSubsWriter{}, &noopEmailer{}, fakeVariants{appleMonthly: monthly, appleYearly: yearly})
}

// --- tests ---------------------------------------------------------------

func TestHandleWebhook_DisabledMethod404(t *testing.T) {
	svc := webhookSvc(fakeSettings{csv: "vietqr", found: true}, fakeVerifier{}, &fakeWebhookRepo{}, &fakeSubsWriter{}, &noopEmailer{}, fakeVariants{})
	_, err := svc.HandleWebhook(context.Background(), "stripe", nil, http.Header{})
	assertDomainErr(t, err, 404, "Payment method 'stripe' is not enabled.")
}

func TestHandleWebhook_PaymentCompletedActivates(t *testing.T) {
	repo := &fakeWebhookRepo{pay: &WebhookPayment{
		ID: "p1", UserID: "u1", PlanID: "pl1", Status: "pending",
		Currency: "USD", Method: "stripe", BillingPeriod: "monthly",
	}}
	subs := &fakeSubsWriter{}
	em := &noopEmailer{}
	svc := webhookSvc(fakeSettings{csv: "stripe,vietqr,lemonsqueezy,bank_transfer", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionPaymentCompleted, ReferenceCode: "DR-PRO-AB",
			StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1",
		}}, repo, subs, em, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "stripe", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || res.ReferenceCode == nil || *res.ReferenceCode != "DR-PRO-AB" {
		t.Fatalf("bad result: %+v", res)
	}
	if repo.completedRef != "DR-PRO-AB" {
		t.Fatalf("payment not marked completed: %q", repo.completedRef)
	}
	if !subs.granted || subs.grantStore != "stripe" {
		t.Fatalf("subscription not granted with stripe store: %+v", subs)
	}
	if repo.stripeCustID != "cus_1" {
		t.Fatalf("stripe customer id not set: %q", repo.stripeCustID)
	}
	if subs.stamped != "stripe:sub_1" {
		t.Fatalf("store ref not stamped: %q", subs.stamped)
	}
	if em.activatedTo != "user@example.com" {
		t.Fatalf("activation email not sent: %q", em.activatedTo)
	}
}

func TestHandleWebhook_CompletedAlreadyDoneIsNoOp(t *testing.T) {
	repo := &fakeWebhookRepo{pay: &WebhookPayment{
		ID: "p1", UserID: "u1", PlanID: "pl1", Status: "completed",
		Currency: "USD", Method: "stripe", BillingPeriod: "monthly",
	}}
	subs := &fakeSubsWriter{}
	svc := webhookSvc(fakeSettings{csv: "stripe", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionPaymentCompleted, ReferenceCode: "DR-PRO-AB",
		}}, repo, subs, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "stripe", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("already-completed must still report success: %+v", res)
	}
	if repo.completedRef != "" || subs.granted {
		t.Fatalf("already-completed must be a no-op, got completed=%q granted=%v", repo.completedRef, subs.granted)
	}
}

func TestHandleWebhook_CompletedNotFound(t *testing.T) {
	repo := &fakeWebhookRepo{pay: nil} // no payment for ref
	svc := webhookSvc(fakeSettings{csv: "stripe", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionPaymentCompleted, ReferenceCode: "DR-PRO-NOPE",
		}}, repo, &fakeSubsWriter{}, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "stripe", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Success {
		t.Fatalf("missing payment must report success=false, got %+v", res)
	}
	if res.ReferenceCode == nil || *res.ReferenceCode != "DR-PRO-NOPE" {
		t.Fatalf("reference code should echo: %+v", res)
	}
}

func TestHandleWebhook_RenewedExtends(t *testing.T) {
	subs := &fakeSubsWriter{}
	svc := webhookSvc(fakeSettings{csv: "stripe", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionSubscriptionRenewed, StripeSubscriptionID: "sub_9",
			CurrentPeriodEnd: 1800000000,
		}}, &fakeWebhookRepo{}, subs, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "stripe", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || res.ReferenceCode != nil {
		t.Fatalf("renewed must be {success:true} with no ref: %+v", res)
	}
	if subs.extended != "stripe:sub_9" {
		t.Fatalf("renewal did not extend: %q", subs.extended)
	}
}

func TestHandleWebhook_LSExpiredExpires(t *testing.T) {
	subs := &fakeSubsWriter{}
	svc := webhookSvc(fakeSettings{csv: "lemonsqueezy", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionLSSubscriptionExpired, LSSubscriptionID: "ls_7",
		}}, &fakeWebhookRepo{}, subs, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "lemonsqueezy", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("expired must report success: %+v", res)
	}
	if subs.expired != "lemonsqueezy:ls_7" {
		t.Fatalf("did not expire by store ref: %q", subs.expired)
	}
}

func TestHandleWebhook_IgnoredReturnsSuccessFalse(t *testing.T) {
	svc := webhookSvc(fakeSettings{csv: "stripe", found: true},
		fakeVerifier{action: strategy.Ignored()},
		&fakeWebhookRepo{}, &fakeSubsWriter{}, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "stripe", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Success || res.ReferenceCode != nil {
		t.Fatalf("ignored must be {success:false} with no ref: %+v", res)
	}
}

func TestHandleWebhook_VerifyErrorPropagates(t *testing.T) {
	svc := webhookSvc(fakeSettings{csv: "stripe", found: true},
		fakeVerifier{err: &strategy.WebhookError{Status: 401, Message: "Invalid signature"}},
		&fakeWebhookRepo{}, &fakeSubsWriter{}, &noopEmailer{}, fakeVariants{})

	_, err := svc.HandleWebhook(context.Background(), "stripe", nil, http.Header{})
	assertDomainErr(t, err, 401, "Invalid signature")
}

func TestHandleWebhook_LSPaymentSuccessReResolvesPlan(t *testing.T) {
	// Pending payment carries plan "pl_old"; the LS variant resolves to a
	// different active plan id ("pl_new"), so the handler re-resolves the plan
	// before completing, stamps the LS store ref, and echoes the reference.
	repo := &fakeWebhookRepo{
		pay: &WebhookPayment{
			ID: "p1", UserID: "u1", PlanID: "pl_old", Status: "pending",
			Currency: "USD", Method: "lemonsqueezy", BillingPeriod: "monthly",
		},
		planID: "pl_new", // FindFirstActivePlanID returns the re-resolved plan
	}
	subs := &fakeSubsWriter{}
	em := &noopEmailer{}
	svc := webhookSvc(fakeSettings{csv: "lemonsqueezy", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionLSPaymentSuccess, ReferenceCode: "DR-PRO-LS",
			LSVariantID: "var_monthly", LSSubscriptionID: "ls_sub_1",
		}}, repo, subs, em, fakeVariants{monthly: "var_monthly", yearly: "var_yearly"})

	res, err := svc.HandleWebhook(context.Background(), "lemonsqueezy", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || res.ReferenceCode == nil || *res.ReferenceCode != "DR-PRO-LS" {
		t.Fatalf("bad result: %+v", res)
	}
	if repo.planUpdated != "p1:pl_new" {
		t.Fatalf("plan not re-resolved: %q", repo.planUpdated)
	}
	if subs.stamped != "lemonsqueezy:ls_sub_1" {
		t.Fatalf("LS store ref not stamped: %q", subs.stamped)
	}
}

func TestHandleWebhook_LSPaymentFailedEmails(t *testing.T) {
	// LS payment-failed: handler finds the affected subscription and emails the
	// user. UserName is empty, so the name falls back to the email address.
	subs := &fakeSubsWriter{found: &subscription.StoreRefSub{
		UserEmail: "u@x.com", UserName: "", PlanName: "Pro",
	}}
	em := &noopEmailer{}
	svc := webhookSvc(fakeSettings{csv: "lemonsqueezy", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionLSPaymentFailed, LSSubscriptionID: "ls_sub_2",
		}}, &fakeWebhookRepo{}, subs, em, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "lemonsqueezy", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || res.ReferenceCode != nil {
		t.Fatalf("LS payment-failed must be {success:true} with no ref: %+v", res)
	}
	if !em.failed {
		t.Fatalf("payment-failed email not sent")
	}
	if em.failedName != "u@x.com" {
		t.Fatalf("empty name should fall back to email, got %q", em.failedName)
	}
}

// --- PayPal webhook actions (port of the Node payment.service.spec PayPal
// cases). First cycle: ACTIVATED carries the reference code → complete the
// pending payment + stamp store ref. Renewals arrive with no reference code →
// extend by store ref, and the response omits reference_code entirely.

func TestHandleWebhook_PayPalActivatedCompletesAndStamps(t *testing.T) {
	repo := &fakeWebhookRepo{pay: &WebhookPayment{
		ID: "p1", UserID: "u1", PlanID: "pl1", Status: "pending",
		Currency: "USD", Method: "paypal", BillingPeriod: "monthly",
	}}
	subs := &fakeSubsWriter{}
	svc := webhookSvc(fakeSettings{csv: "paypal", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionPayPalPaymentSuccess, ReferenceCode: "DR-PRO-PP",
			PayPalSubscriptionID: "I-SUB-9", CurrentPeriodEnd: 1767225600,
		}}, repo, subs, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "paypal", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || res.ReferenceCode == nil || *res.ReferenceCode != "DR-PRO-PP" {
		t.Fatalf("bad result: %+v", res)
	}
	if subs.stamped != "paypal:I-SUB-9" {
		t.Fatalf("PayPal store ref not stamped: %q", subs.stamped)
	}
	if !subs.granted || subs.grantStore != "paypal" {
		t.Fatalf("subscription not granted with paypal store type: %+v", subs)
	}
	if subs.extended != "" {
		t.Fatalf("first cycle must not take the renewal branch: %q", subs.extended)
	}
}

func TestHandleWebhook_PayPalRenewalExtendsWithoutRef(t *testing.T) {
	// PAYMENT.SALE.COMPLETED: no reference code → renewal branch. The response
	// is {success:true} with reference_code omitted (Node: `|| undefined`).
	subs := &fakeSubsWriter{}
	svc := webhookSvc(fakeSettings{csv: "paypal", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type:                 strategy.ActionPayPalPaymentSuccess,
			PayPalSubscriptionID: "I-SUB-1", CurrentPeriodEnd: 1767225600,
		}}, &fakeWebhookRepo{}, subs, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "paypal", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || res.ReferenceCode != nil {
		t.Fatalf("renewal must be {success:true} with no ref key: %+v", res)
	}
	if subs.extended != "paypal:I-SUB-1" {
		t.Fatalf("renewal did not extend by store ref: %q", subs.extended)
	}
}

func TestHandleWebhook_PayPalRenewalZeroPeriodSkipsExtend(t *testing.T) {
	// cpe=0 (next_billing_time unavailable) → Node skips the extend entirely.
	subs := &fakeSubsWriter{}
	svc := webhookSvc(fakeSettings{csv: "paypal", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type:                 strategy.ActionPayPalPaymentSuccess,
			PayPalSubscriptionID: "I-SUB-0",
		}}, &fakeWebhookRepo{}, subs, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "paypal", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || subs.extended != "" {
		t.Fatalf("cpe=0 must not extend: res=%+v extended=%q", res, subs.extended)
	}
}

func TestHandleWebhook_PayPalSettledPaymentTakesRenewalBranch(t *testing.T) {
	// A SALE.COMPLETED right after ACTIVATED: the payment is already completed,
	// so even with a reference code present the pending lookup misses and the
	// renewal branch extends (idempotent no-op to the same next_billing_time).
	repo := &fakeWebhookRepo{pay: &WebhookPayment{
		ID: "p1", UserID: "u1", PlanID: "pl1", Status: "completed",
		Currency: "USD", Method: "paypal", BillingPeriod: "monthly",
	}}
	subs := &fakeSubsWriter{}
	svc := webhookSvc(fakeSettings{csv: "paypal", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionPayPalPaymentSuccess, ReferenceCode: "DR-PRO-PP",
			PayPalSubscriptionID: "I-SUB-9", CurrentPeriodEnd: 1767225600,
		}}, repo, subs, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "paypal", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || res.ReferenceCode == nil || *res.ReferenceCode != "DR-PRO-PP" {
		t.Fatalf("bad result: %+v", res)
	}
	if subs.stamped != "" || subs.extended != "paypal:I-SUB-9" {
		t.Fatalf("settled payment must extend, not stamp: %+v", subs)
	}
}

func TestHandleWebhook_PayPalPaymentFailedEmails(t *testing.T) {
	subs := &fakeSubsWriter{found: &subscription.StoreRefSub{
		UserEmail: "u@x.com", UserName: "", PlanName: "Pro",
	}}
	em := &noopEmailer{}
	svc := webhookSvc(fakeSettings{csv: "paypal", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionPayPalPaymentFailed, PayPalSubscriptionID: "I-SUB-4",
		}}, &fakeWebhookRepo{}, subs, em, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "paypal", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || res.ReferenceCode != nil {
		t.Fatalf("payment-failed must be {success:true} with no ref: %+v", res)
	}
	if !em.failed || em.failedName != "u@x.com" {
		t.Fatalf("expected failure email with email-as-name fallback: %+v", em)
	}
}

func TestHandleWebhook_PayPalPaymentFailedUnknownSubSkips(t *testing.T) {
	em := &noopEmailer{}
	svc := webhookSvc(fakeSettings{csv: "paypal", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionPayPalPaymentFailed, PayPalSubscriptionID: "I-UNKNOWN",
		}}, &fakeWebhookRepo{}, &fakeSubsWriter{}, em, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "paypal", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || em.failed {
		t.Fatalf("unknown sub must succeed without email: res=%+v emailed=%v", res, em.failed)
	}
}

func TestHandleWebhook_PayPalCanceledCancels(t *testing.T) {
	subs := &fakeSubsWriter{}
	svc := webhookSvc(fakeSettings{csv: "paypal", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionPayPalSubCanceled, PayPalSubscriptionID: "I-SUB-C",
		}}, &fakeWebhookRepo{}, subs, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "paypal", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || subs.cancelled != "paypal:I-SUB-C" {
		t.Fatalf("did not cancel by store ref: res=%+v cancelled=%q", res, subs.cancelled)
	}
}

func TestHandleWebhook_PayPalExpiredExpires(t *testing.T) {
	subs := &fakeSubsWriter{}
	svc := webhookSvc(fakeSettings{csv: "paypal", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionPayPalSubExpired, PayPalSubscriptionID: "I-SUB-3",
		}}, &fakeWebhookRepo{}, subs, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), "paypal", nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || subs.expired != "paypal:I-SUB-3" {
		t.Fatalf("did not expire by store ref: res=%+v expired=%q", res, subs.expired)
	}
}

// --- Apple IAP product→plan resolver (Task 6 of the Apple IAP server
// redemption plan) — mirrors resolvePlanIDFromLSVariant, but an unmatched
// product id must error rather than silently resolve to no plan.

func TestResolvePlanIDFromAppleProduct(t *testing.T) {
	s := newTestServiceWithAppleProducts(t, "com.draftright.pro.monthly", "com.draftright.pro.yearly")

	plan, billing, err := s.resolvePlanIDFromAppleProduct(context.Background(), "com.draftright.pro.yearly")
	if err != nil {
		t.Fatal(err)
	}
	if billing != "yearly" || plan == "" {
		t.Fatalf("got plan=%q billing=%q", plan, billing)
	}

	if _, _, err := s.resolvePlanIDFromAppleProduct(context.Background(), "unknown.product"); err == nil {
		t.Fatal("unknown product id must error, not grant a plan")
	}
}

func TestResolvePlanIDFromAppleProduct_MonthlyResolves(t *testing.T) {
	s := newTestServiceWithAppleProducts(t, "com.draftright.pro.monthly", "com.draftright.pro.yearly")

	plan, billing, err := s.resolvePlanIDFromAppleProduct(context.Background(), "com.draftright.pro.monthly")
	if err != nil {
		t.Fatal(err)
	}
	if billing != "monthly" || plan == "" {
		t.Fatalf("got plan=%q billing=%q", plan, billing)
	}
}

func TestResolvePlanIDFromAppleProduct_UnconfiguredNeverMatchesEmptyProductID(t *testing.T) {
	// Both product ids unset (credentials row absent/blank) — an empty
	// productID must not spuriously match either "" comparison.
	s := newTestServiceWithAppleProducts(t, "", "")

	if _, _, err := s.resolvePlanIDFromAppleProduct(context.Background(), ""); err == nil {
		t.Fatal("blank product id against unconfigured credentials must error, not grant a plan")
	}
}

func TestResolvePlanIDFromAppleProduct_CredentialsErrorPropagates(t *testing.T) {
	repo := &fakeWebhookRepo{planID: "pl_apple_stub"}
	svc := &Service{}
	svc.WithWebhook(repo, &fakeSubsWriter{}, &noopEmailer{}, fakeVariants{appleErr: errors.New("db unavailable")})

	if _, _, err := svc.resolvePlanIDFromAppleProduct(context.Background(), "com.draftright.pro.monthly"); err == nil {
		t.Fatal("credentials resolver error must propagate, not grant a plan")
	}
}

// --- Apple IAP webhook notification lifecycle (Task 8 of the Apple IAP server
// redemption plan). Renewals extend the sub matched by the ORIGINAL
// transaction id (stamped by RedeemAppleTransaction on first purchase) and
// must NOT re-send the activation email — that already happened at redeem
// time. Expiry/refund revoke. An unmatched notification (lost first /redeem)
// must not error — Apple would just retry — and must not grant, since the
// notification carries no user identity in Spec 1.

func TestHandleWebhook_AppleRenew_ExtendsNoEmail(t *testing.T) {
	subs := &fakeSubsWriter{}
	em := &noopEmailer{}
	svc := webhookSvc(fakeSettings{csv: "apple_iap", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionAppleRenewed, AppleOriginalTransactionID: "o1",
			CurrentPeriodEnd: 4102444800, // 2100-01-01
		}}, &fakeWebhookRepo{}, subs, em, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), string(MethodAppleIAP), nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("renewal must report success: %+v", res)
	}
	if subs.extended != "apple_iap:o1" {
		t.Fatalf("renewal did not extend by original transaction id: %q", subs.extended)
	}
	if em.activatedCalls != 0 {
		t.Fatalf("renewal must not re-send the activation email, got %d calls", em.activatedCalls)
	}
}

func TestHandleWebhook_AppleSubscribed_AlsoExtends(t *testing.T) {
	// apple_subscribed can arrive as a re-confirmation of a purchase the
	// client already redeemed (which granted + stamped it) — it must take the
	// same extend path as apple_renewed, never a second grant.
	subs := &fakeSubsWriter{}
	svc := webhookSvc(fakeSettings{csv: "apple_iap", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionAppleSubscribed, AppleOriginalTransactionID: "o2",
			CurrentPeriodEnd: 4102444800,
		}}, &fakeWebhookRepo{}, subs, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), string(MethodAppleIAP), nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || subs.extended != "apple_iap:o2" {
		t.Fatalf("subscribed notification did not extend: res=%+v extended=%q", res, subs.extended)
	}
	if subs.granted {
		t.Fatal("subscribed notification must not grant a second time")
	}
}

func TestHandleWebhook_AppleRenewNoMatch_LogsNoError(t *testing.T) {
	subs := &fakeSubsWriter{extendNoMatch: true}
	svc := webhookSvc(fakeSettings{csv: "apple_iap", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionAppleRenewed, AppleOriginalTransactionID: "orphan",
			CurrentPeriodEnd: 4102444800,
		}}, &fakeWebhookRepo{}, subs, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), string(MethodAppleIAP), nil, http.Header{})
	if err != nil {
		t.Fatalf("unmatched notification must not error (Apple would retry): %v", err)
	}
	if !res.Success {
		t.Fatalf("unmatched notification must still report success: %+v", res)
	}
	if subs.granted {
		t.Fatal("unmatched notification must not grant — no user identity in Spec 1")
	}
}

func TestHandleWebhook_AppleExpired_Revokes(t *testing.T) {
	subs := &fakeSubsWriter{}
	svc := webhookSvc(fakeSettings{csv: "apple_iap", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionAppleExpired, AppleOriginalTransactionID: "o1",
		}}, &fakeWebhookRepo{}, subs, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), string(MethodAppleIAP), nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || subs.expired != "apple_iap:o1" {
		t.Fatalf("did not expire by original transaction id: res=%+v expired=%q", res, subs.expired)
	}
}

func TestHandleWebhook_AppleRefunded_Revokes(t *testing.T) {
	subs := &fakeSubsWriter{}
	svc := webhookSvc(fakeSettings{csv: "apple_iap", found: true},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionAppleRefunded, AppleOriginalTransactionID: "o3",
		}}, &fakeWebhookRepo{}, subs, &noopEmailer{}, fakeVariants{})

	res, err := svc.HandleWebhook(context.Background(), string(MethodAppleIAP), nil, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || subs.expired != "apple_iap:o3" {
		t.Fatalf("refund did not revoke by original transaction id: res=%+v expired=%q", res, subs.expired)
	}
}
