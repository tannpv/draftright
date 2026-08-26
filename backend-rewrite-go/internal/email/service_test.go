package email

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	"github.com/tannpv/emailkit"
)

// recordingStore is both the Store and the CredentialSource, mirroring how
// main.go passes *PgRepo twice. The cred* fields are the configurable
// credentials result: zero values reproduce the old always-empty behaviour
// (resolver falls through to Config), and tests that need a different
// resolver outcome set them instead of declaring a second CredentialSource
// fake.
type recordingStore struct {
	logs []emailkit.SendRecord

	credAPIKey string
	credFrom   string
	credErr    error
}

func (r *recordingStore) IsSuppressed(context.Context, string) (bool, error) { return false, nil }
func (r *recordingStore) LogSend(_ context.Context, s emailkit.SendRecord) error {
	r.logs = append(r.logs, s)
	return nil
}
func (r *recordingStore) Template(context.Context, string) (string, string, bool) {
	return "", "", false
}
func (r *recordingStore) MarkByProviderID(context.Context, string, string, *string) error { return nil }
func (r *recordingStore) Suppress(context.Context, string, string) error                  { return nil }
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

// The six product methods are the contract four other packages depend on. This
// pins the subject each one produces, so a template rename or a wrong key
// constant fails here rather than silently mailing nothing.
//
// wantCode additionally pins the rendered BODY for verification and
// password-reset: their subjects carry no {{vars}}, so a bug that renamed the
// "code" var going into Send (e.g. SendVerification's map key drifting from
// the template's {{code}}) would leave the subject assertion green while
// mailing an empty code. The other four subjects already embed a substituted
// var (plan/expires/amount), so they don't need a separate body check.
func TestProductMethods_ProduceExpectedSubjects(t *testing.T) {
	expires := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		call     func(*Service)
		subject  string
		wantCode string
	}{
		{"verification", func(s *Service) { s.SendVerification(context.Background(), "a@b.c", "X", "123456") },
			"Welcome to DraftRight — confirm your email", "123456"},
		{"password-reset", func(s *Service) { s.SendPasswordReset(context.Background(), "a@b.c", "X", "000111") },
			"Reset your DraftRight password", "000111"},
		{"renewal-reminder", func(s *Service) {
			s.SendRenewalReminder(context.Background(), "a@b.c", "X", "Pro", expires, "USD", 999)
		},
			"DraftRight Pro renews on Mon Jun 15 2026", ""},
		{"subscription-activated", func(s *Service) {
			s.SendSubscriptionActivated(context.Background(), "a@b.c", "X", "Pro", expires, "USD", 999)
		},
			"Your DraftRight Pro subscription is active", ""},
		{"payment-failed", func(s *Service) { s.SendPaymentFailed(context.Background(), "a@b.c", "X", "Pro") },
			"Action needed: renewal payment failed for DraftRight Pro", ""},
		{"subscription-expired", func(s *Service) { s.SendSubscriptionExpired(context.Background(), "a@b.c", "X", "Pro") },
			"Your DraftRight Pro subscription has expired", ""},
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
		if c.wantCode != "" && !strings.Contains(sender.html, c.wantCode) {
			t.Errorf("%s: body = %q, want it to contain %q", c.name, sender.html, c.wantCode)
		}
	}
}

// stubQuerier is a minimal pgQuerier fake used only to drive PgRepo.Credentials
// through the real adapter (see TestResolveCredentials's ErrNoRows case).
// Every other method is unused by that path and left at its zero value.
type stubQuerier struct {
	settingsRow sqlc.GetEmailSettingsRow
	settingsErr error
}

func (stubQuerier) IsEmailSuppressed(context.Context, string) (bool, error) { return false, nil }
func (stubQuerier) InsertEmailLog(context.Context, sqlc.InsertEmailLogParams) error {
	return nil
}
func (s stubQuerier) GetEmailSettings(context.Context) (sqlc.GetEmailSettingsRow, error) {
	return s.settingsRow, s.settingsErr
}
func (stubQuerier) GetEmailTemplateByKey(context.Context, string) (sqlc.GetEmailTemplateByKeyRow, error) {
	return sqlc.GetEmailTemplateByKeyRow{}, nil
}
func (stubQuerier) MarkEmailByProviderID(context.Context, sqlc.MarkEmailByProviderIDParams) error {
	return nil
}
func (stubQuerier) SuppressEmail(context.Context, sqlc.SuppressEmailParams) error { return nil }

// TestResolveCredentials covers resolveCredentials — the only transport-
// adjacent policy this package still owns: whether a per-send override from
// the store replaces the env fallback, and when an error must withhold the
// send instead of falling back.
//
// The "no row configured" case deliberately drives resolveCredentials through
// a real *PgRepo backed by stubQuerier, not through recordingStore: the fix it
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
			creds:    NewPgRepo(stubQuerier{settingsErr: pgx.ErrNoRows}),
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
