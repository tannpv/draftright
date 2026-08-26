package email

import (
	"context"
	"testing"
	"time"

	"github.com/tannpv/emailkit"
)

// recordingStore is both the Store and the CredentialSource, mirroring how
// main.go passes *PgRepo twice. Credentials returns empty strings so the
// resolver falls through to Config, and the send stops before any provider
// call — the audit row is still written, which is what these tests read.
type recordingStore struct {
	logs []emailkit.SendRecord
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
	return "", "", nil
}

// The six product methods are the contract four other packages depend on. This
// pins the subject each one produces, so a template rename or a wrong key
// constant fails here rather than silently mailing nothing.
func TestProductMethods_ProduceExpectedSubjects(t *testing.T) {
	expires := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		call    func(*Service)
		subject string
	}{
		{"verification", func(s *Service) { s.SendVerification(context.Background(), "a@b.c", "X", "123456") },
			"Welcome to DraftRight — confirm your email"},
		{"password-reset", func(s *Service) { s.SendPasswordReset(context.Background(), "a@b.c", "X", "000111") },
			"Reset your DraftRight password"},
		{"renewal-reminder", func(s *Service) {
			s.SendRenewalReminder(context.Background(), "a@b.c", "X", "Pro", expires, "USD", 999)
		},
			"DraftRight Pro renews on Mon Jun 15 2026"},
		{"subscription-activated", func(s *Service) {
			s.SendSubscriptionActivated(context.Background(), "a@b.c", "X", "Pro", expires, "USD", 999)
		},
			"Your DraftRight Pro subscription is active"},
		{"payment-failed", func(s *Service) { s.SendPaymentFailed(context.Background(), "a@b.c", "X", "Pro") },
			"Action needed: renewal payment failed for DraftRight Pro"},
		{"subscription-expired", func(s *Service) { s.SendSubscriptionExpired(context.Background(), "a@b.c", "X", "Pro") },
			"Your DraftRight Pro subscription has expired"},
	}
	for _, c := range cases {
		st := &recordingStore{}
		svc := NewService(st, st, Config{EnvAPIKey: "", EnvFrom: "t@example.com"})
		c.call(svc)
		svc.Wait()
		if len(st.logs) != 1 {
			t.Fatalf("%s: want one audit row, got %d", c.name, len(st.logs))
		}
		if st.logs[0].Subject != c.subject {
			t.Errorf("%s: subject = %q, want %q", c.name, st.logs[0].Subject, c.subject)
		}
	}
}
