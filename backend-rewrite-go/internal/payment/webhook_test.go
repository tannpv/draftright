package payment

import (
	"context"
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
	granted    bool
	grantStore string
	stamped    string // "storeType:txnID"
	extended   string
	cancelled  string
	expired    string
	found      *subscription.StoreRefSub
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
func (f *fakeSubsWriter) ExtendByStoreRef(ctx context.Context, storeType, transactionID string, expiresAt time.Time) (int64, error) {
	f.extended = storeType + ":" + transactionID
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
	activatedTo string
	failedTo    string
}

func (e *noopEmailer) SubscriptionActivated(ctx context.Context, to, name, planName string) {
	e.activatedTo = to
}
func (e *noopEmailer) PaymentFailed(ctx context.Context, to, name, planName string) {
	e.failedTo = to
}

type fakeVariants struct {
	monthly, yearly string
}

func (f fakeVariants) LemonSqueezyVariants(ctx context.Context) (string, string, error) {
	return f.monthly, f.yearly, nil
}

// webhookSvc builds a Service wired for the webhook path with the given
// collaborators + a fixed clock (so activateSubscription's s.now() never panics).
func webhookSvc(settings fakeSettings, strat strategy.Strategy, repo WebhookRepo, subs SubsWriter, emailer WebhookEmailer, variants VariantResolver) *Service {
	svc := &Service{
		settings:   settings,
		strategies: map[string]strategy.Strategy{"stripe": strat, "vietqr": strat, "lemonsqueezy": strat, "bank_transfer": strat},
		now:        func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}
	return svc.WithWebhook(repo, subs, emailer, variants)
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
