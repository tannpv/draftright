package payment

import (
	"context"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
)

// SubsPort is the subscription read the portal/cancel use cases need.
type SubsPort interface {
	ActiveByUser(ctx context.Context, userID string) (*subscription.AccountSub, error)
}

// PaymentRepo is the consumer-side port for the read paths (satisfied by *Repo).
type PaymentRepo interface {
	GetByReference(ctx context.Context, ref string) (*StatusRow, error)
	ListByUser(ctx context.Context, userID string) ([]PaymentRow, error)
}

// CheckoutRepo is the persistence the checkout use case needs (consumer port;
// the concrete *Repo satisfies it).
type CheckoutRepo interface {
	PlanForCheckout(ctx context.Context, id string) (*CheckoutPlan, error)
	UserForCheckout(ctx context.Context, id string) (*CheckoutUser, error)
	CreatePayment(ctx context.Context, userID, planID string, amount int, currency, method, status, ref string, expiresAt time.Time) (*CreatedPayment, error)
	UpdateQRData(ctx context.Context, paymentID, qr string) error
	MarkFailed(ctx context.Context, paymentID, notes string) error
}

// SettingsReader yields the admin-configured payment_methods_enabled CSV.
// found=false means the app_settings row (or column) is absent → caller falls
// back to env then default.
type SettingsReader interface {
	PaymentMethodsEnabled(ctx context.Context) (string, bool, error)
}

// SubsWriter mutates subscriptions on verified webhook events (Phase 3c). The
// concrete *subscription.WebhookWriter satisfies it; HandleWebhook never imports
// the writer's guts.
type SubsWriter interface {
	Grant(ctx context.Context, userID, planID, storeType string, expiresAt *time.Time) error
	StampStoreRef(ctx context.Context, referenceCode, storeType, transactionID string) error
	ExtendByStoreRef(ctx context.Context, storeType, transactionID string, expiresAt time.Time) (int64, error)
	CancelByStoreRef(ctx context.Context, storeType, transactionID string) (int64, error)
	ExpireByStoreRef(ctx context.Context, storeType, transactionID string) (int64, error)
	FindByStoreRef(ctx context.Context, storeType, transactionID string) (*subscription.StoreRefSub, error)
}

// WebhookRepo is the persistence the webhook path needs (satisfied by *Repo).
type WebhookRepo interface {
	PaymentForWebhook(ctx context.Context, ref string) (*WebhookPayment, error)
	MarkPaymentCompleted(ctx context.Context, ref string) error
	MarkPaymentFailedByRef(ctx context.Context, ref string) error
	SetStripeCustomerID(ctx context.Context, userID, customerID string) error
	SetLemonSqueezyCustomerID(ctx context.Context, userID, customerID string) error
	UpdatePaymentPlan(ctx context.Context, paymentID, planID string) error
	FindFirstActivePlanID(ctx context.Context, billingPeriod, currency string) (string, error)
	UserEmailName(ctx context.Context, userID string) (email, name string, err error)
}

// WebhookEmailer sends the two out-of-band webhook emails (best-effort, not
// shadow-gated). Satisfied by *MailWebhookEmailer.
type WebhookEmailer interface {
	SubscriptionActivated(ctx context.Context, to, name, planName string)
	PaymentFailed(ctx context.Context, to, name, planName string)
}

// VariantResolver resolves the configured Lemon Squeezy monthly/yearly variant
// ids (re-resolve the true plan from the webhook's variant_id).
type VariantResolver interface {
	LemonSqueezyVariants(ctx context.Context) (monthly, yearly string, err error)
}

// Service is the payment use case (methods/status/history + checkout).
type Service struct {
	repo     PaymentRepo
	settings SettingsReader
	envCSV   string // PAYMENT_ENABLED_METHODS, used when settings absent

	// Checkout-side collaborators (Phase 3b). Read-path callers may pass nil.
	checkoutRepo CheckoutRepo
	strategies   map[string]strategy.Strategy
	now          func() time.Time
	genRef       func() string

	// subs feeds the portal + in-app cancel use cases (active-subscription read).
	// main.go always injects the real subscription reader; read-path tests pass nil.
	subs SubsPort

	// Webhook-side collaborators (Phase 3c), injected via WithWebhook. Read/checkout
	// callers leave these nil; only HandleWebhook touches them.
	webhookRepo WebhookRepo
	subsWriter  SubsWriter
	emailer     WebhookEmailer
	variants    VariantResolver
}

// WithWebhook injects the Phase 3c webhook collaborators (additive — NewService's
// signature is unchanged). Returns the same Service for chaining at the
// composition root.
func (s *Service) WithWebhook(repo WebhookRepo, subs SubsWriter, emailer WebhookEmailer, variants VariantResolver) *Service {
	s.webhookRepo = repo
	s.subsWriter = subs
	s.emailer = emailer
	s.variants = variants
	return s
}

// NewService wires the repo, settings reader, env fallback CSV, and the
// checkout-side collaborators (checkout repo, strategy registry, clock, and
// reference-code generator).
func NewService(repo PaymentRepo, settings SettingsReader, envCSV string, checkoutRepo CheckoutRepo, strategies map[string]strategy.Strategy, now func() time.Time, genRef func() string, subs SubsPort) *Service {
	if now == nil {
		now = time.Now
	}
	if genRef == nil {
		genRef = GeneratePaymentReference
	}
	return &Service{
		repo:         repo,
		settings:     settings,
		envCSV:       envCSV,
		checkoutRepo: checkoutRepo,
		strategies:   strategies,
		now:          now,
		genRef:       genRef,
		subs:         subs,
	}
}

// EnabledMethods resolves the storefront's visible methods. Precedence mirrors
// getEnabledMethods(): app_settings.payment_methods_enabled ?? env ?? default,
// then EnabledMethods() applies lowercase/dedupe/ordering + vietqr→bank_transfer.
func (s *Service) EnabledMethods(ctx context.Context) ([]string, error) {
	csv, found, err := s.settings.PaymentMethodsEnabled(ctx)
	if err != nil {
		return nil, err
	}
	raw := ""
	switch {
	case found && csv != "":
		raw = csv
	case s.envCSV != "":
		raw = s.envCSV
	default:
		raw = DefaultPaymentMethod
	}
	return EnabledMethods(raw), nil
}

// Status returns the status view for a reference code (NotFound when absent).
func (s *Service) Status(ctx context.Context, ref string) (StatusView, error) {
	row, err := s.repo.GetByReference(ctx, ref)
	if err != nil {
		return StatusView{}, err
	}
	return statusViewFrom(row), nil
}

// History returns the user's 20 newest payments (Node findByUser passthrough).
func (s *Service) History(ctx context.Context, userID string) ([]PaymentRow, error) {
	return s.repo.ListByUser(ctx, userID)
}

// StatusView is GET /payment/status/:ref's response. When NotFound is true the
// handler emits {"status":"not_found"} and nothing else; otherwise it emits the
// full object. Field order + names match the Node controller exactly. The JSON
// MarshalJSON method is added in Task 7.
type StatusView struct {
	notFound      bool
	Status        string
	Method        string
	Amount        int
	Currency      string
	ReferenceCode string
	PlanName      *string
	CompletedAt   *string // ISOMillis or nil
	ExpiresAt     *string // ISOMillis or nil
}

// NotFound reports whether the reference matched no payment.
func (v StatusView) NotFound() bool { return v.notFound }

func statusViewFrom(row *StatusRow) StatusView {
	if row == nil {
		return StatusView{notFound: true}
	}
	v := StatusView{
		Status:        row.Status,
		Method:        row.Method,
		Amount:        row.Amount,
		Currency:      row.Currency,
		ReferenceCode: row.ReferenceCode,
		PlanName:      row.PlanName,
	}
	if row.CompletedAt != nil {
		iso := shared.ISOMillis(*row.CompletedAt)
		v.CompletedAt = &iso
	}
	if row.ExpiresAt != nil {
		iso := shared.ISOMillis(*row.ExpiresAt)
		v.ExpiresAt = &iso
	}
	return v
}
