package email

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tannpv/emailkit"
)

// recordingStore is both the Store and the CredentialSource, mirroring how
// main.go passes *PgRepo twice. The cred* fields are the configurable
// credentials result: zero values reproduce the old always-empty behaviour
// (resolver falls through to Config), and tests that need a different
// resolver outcome set them instead of declaring a second CredentialSource
// fake. markErr/suppressErr are the webhook-path equivalent, used by
// webhook_responder_test.go to drive emailkit.ErrStoreFailure without a real
// database — one fake covers both the send path and the webhook path rather
// than each test file declaring its own.
type recordingStore struct {
	logs []emailkit.SendRecord

	credAPIKey string
	credFrom   string
	credErr    error

	markErr     error
	suppressErr error
}

func (r *recordingStore) IsSuppressed(context.Context, string) (bool, error) { return false, nil }
func (r *recordingStore) LogSend(_ context.Context, s emailkit.SendRecord) error {
	r.logs = append(r.logs, s)
	return nil
}
func (r *recordingStore) Template(context.Context, string) (string, string, bool) {
	return "", "", false
}
func (r *recordingStore) MarkByProviderID(context.Context, string, string, *string) error {
	return r.markErr
}
func (r *recordingStore) Suppress(context.Context, string, string) error { return r.suppressErr }
func (r *recordingStore) Credentials(context.Context) (string, string, error) {
	return r.credAPIKey, r.credFrom, r.credErr
}

// capturingSender fakes emailkit.Sender so tests can inspect the subject and
// HTML actually handed to the provider. The real Sender NewService wires
// (resendClient) talks to Resend's live API over HTTP and its endpoint is
// unexported inside the emailkit module, so it cannot be pointed at a fake
// server from this package — a capturing Sender is the only in-process seam.
type capturingSender struct{ subject, html string }

func (c *capturingSender) Send(_ context.Context, _, _, _, subject, html string) (string, error) {
	c.subject, c.html = subject, html
	return "test-provider-id", nil
}

// newTestService wires a Service like NewService does, but on a capturing
// Sender with a fixed non-empty credential pair instead of the live Resend
// client — so a product-method test can inspect the render (subject + body)
// without an outbound network call. Test-only: production has no need for
// this seam, only for the real one behind NewService.
func newTestService(store emailkit.Store, sender *capturingSender) *Service {
	kit := emailkit.NewServiceWithSender(store, emailkit.Config{
		Resolve: func(context.Context) (string, string, error) { return "test-key", "from@example.com", nil },
	}, BuiltinRegistry(), sender)
	return &Service{kit: kit}
}

// testName is the name every case passes to its product method. Distinctive
// rather than "X" so strings.Contains cannot match it by accident against
// boilerplate in the shell markup.
const testName = "Testerson"

// The six product methods are the contract four other packages depend on. This
// pins the subject each one produces, so a template rename or a wrong key
// constant fails here rather than silently mailing nothing.
//
// wantBody pins the rendered BODY for every var the SUBJECT does not already
// substitute — a map key drifting from the template's {{var}} (e.g.
// SendRenewalReminder's "amount" renamed) renders as empty, and a
// subject-only assertion stays green while the user is mailed "We'll charge
// to your saved payment method". Per template, the subject substitutes:
//
//	verification, password-reset     — nothing (no {{var}} at all)
//	renewal-reminder                 — {{plan}} and {{expires}}
//	subscription-activated           — {{plan}} only
//	payment-failed                   — {{plan}} only
//	subscription-expired             — {{plan}} only
//
// {{amount}} appears in NO subject, and {{name}} appears in no subject
// either, so both are pinned here for every method that passes them; so is
// subscription-activated's body-only {{expires}}.
func TestProductMethods_ProduceExpectedSubjects(t *testing.T) {
	expires := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	const (
		wantExpires = "Mon Jun 15 2026" // dateString(expires)
		wantAmount  = "$9.99"           // formatAmount("USD", 999)
	)
	cases := []struct {
		name     string
		call     func(*Service)
		subject  string
		wantBody []string
	}{
		{"verification", func(s *Service) { s.SendVerification(context.Background(), "a@b.c", testName, "123456") },
			"Welcome to DraftRight — confirm your email",
			[]string{testName, "123456"}},
		{"password-reset", func(s *Service) { s.SendPasswordReset(context.Background(), "a@b.c", testName, "000111") },
			"Reset your DraftRight password",
			[]string{testName, "000111"}},
		{"renewal-reminder", func(s *Service) {
			s.SendRenewalReminder(context.Background(), "a@b.c", testName, "Pro", expires, "USD", 999)
		},
			"DraftRight Pro renews on Mon Jun 15 2026",
			[]string{testName, wantAmount}},
		{"subscription-activated", func(s *Service) {
			s.SendSubscriptionActivated(context.Background(), "a@b.c", testName, "Pro", expires, "USD", 999)
		},
			"Your DraftRight Pro subscription is active",
			[]string{testName, wantAmount, wantExpires}},
		{"payment-failed", func(s *Service) { s.SendPaymentFailed(context.Background(), "a@b.c", testName, "Pro") },
			"Action needed: renewal payment failed for DraftRight Pro",
			[]string{testName}},
		{"subscription-expired", func(s *Service) {
			s.SendSubscriptionExpired(context.Background(), "a@b.c", testName, "Pro")
		},
			"Your DraftRight Pro subscription has expired",
			[]string{testName}},
	}
	for _, c := range cases {
		st := &recordingStore{}
		sender := &capturingSender{}
		svc := newTestService(st, sender)
		c.call(svc)
		svc.Wait()
		if len(st.logs) != 1 {
			t.Fatalf("%s: want one audit row, got %d", c.name, len(st.logs))
		}
		if st.logs[0].Subject != c.subject {
			t.Errorf("%s: subject = %q, want %q", c.name, st.logs[0].Subject, c.subject)
		}
		for _, want := range c.wantBody {
			if !strings.Contains(sender.html, want) {
				t.Errorf("%s: body = %q, want it to contain %q", c.name, sender.html, want)
			}
		}
	}
}

// TestResolveCredentials covers resolveCredentials — the only transport-
// adjacent policy this package still owns: whether a per-send override from
// the store replaces the env fallback, and when an error must withhold the
// send instead of falling back.
//
// The "no row configured" case deliberately drives resolveCredentials through
// a real *PgRepo backed by fakeQuerier, not through recordingStore: the fix it
// guards (Important #1 — PgRepo.Credentials treating pgx.ErrNoRows as "no
// override" rather than a resolver failure) lives in repo_pg.go, and a
// recordingStore-only fake would keep passing even if that fix were reverted.
func TestResolveCredentials(t *testing.T) {
	genericErr := errors.New("settings query: connection refused")

	cases := []struct {
		name              string
		creds             CredentialSource
		cfg               Config
		wantKey, wantFrom string
		wantErr           error
	}{
		{
			name:     "store returns a key and from -> both used",
			creds:    &recordingStore{credAPIKey: "store-key", credFrom: "store-from@x.com"},
			cfg:      Config{EnvAPIKey: "env-key", EnvFrom: "env-from@x.com"},
			wantKey:  "store-key",
			wantFrom: "store-from@x.com",
		},
		{
			name:     "store returns empty strings (row exists, blank columns) -> env used",
			creds:    &recordingStore{},
			cfg:      Config{EnvAPIKey: "env-key", EnvFrom: "env-from@x.com"},
			wantKey:  "env-key",
			wantFrom: "env-from@x.com",
		},
		{
			name:     "store has no app_settings row (pgx.ErrNoRows) -> env used, no error",
			creds:    NewPgRepo(&fakeQuerier{settingsErr: pgx.ErrNoRows}),
			cfg:      Config{EnvAPIKey: "env-key", EnvFrom: "env-from@x.com"},
			wantKey:  "env-key",
			wantFrom: "env-from@x.com",
		},
		{
			name:    "store returns any other error -> propagates, no env fallback",
			creds:   &recordingStore{credErr: genericErr},
			cfg:     Config{EnvAPIKey: "env-key", EnvFrom: "env-from@x.com"},
			wantErr: genericErr,
		},
		{
			name:     "store returns a key but empty from, EnvFrom also empty -> defaultFrom",
			creds:    &recordingStore{credAPIKey: "store-key"},
			cfg:      Config{EnvAPIKey: "env-key", EnvFrom: ""},
			wantKey:  "store-key",
			wantFrom: defaultFrom,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			key, from, err := resolveCredentials(context.Background(), c.creds, c.cfg)
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("err = %v, want %v", err, c.wantErr)
				}
				if key != "" || from != "" {
					t.Fatalf("on error, want no fallback values, got key=%q from=%q", key, from)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if key != c.wantKey {
				t.Errorf("key = %q, want %q", key, c.wantKey)
			}
			if from != c.wantFrom {
				t.Errorf("from = %q, want %q", from, c.wantFrom)
			}
		})
	}
}
