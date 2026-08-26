// Package email is DraftRight's transactional-email vocabulary: which emails
// exist, what they say, and the DB adapter behind them. Sending itself —
// the suppression check, the provider call and the audit row — belongs to
// github.com/tannpv/emailkit, the transport shared with liseuse and bacnam.
// Sends are FIRE-AND-FORGET: they never block or fail the HTTP request.
// NOT shadow-gated (out-of-band).
package email

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/emailkit"
)

// Config carries the env fallbacks. app_settings overrides them per send via
// the resolver wired in NewService.
type Config struct {
	EnvAPIKey string
	EnvFrom   string
}

const defaultFrom = "DraftRight <noreply@draftright.info>"

// CredentialSource supplies the per-send Resend credentials. *PgRepo satisfies
// it; main.go passes the same value it passes as the Store.
type CredentialSource interface {
	Credentials(ctx context.Context) (apiKey, from string, err error)
}

// Service is DraftRight's email vocabulary over the shared transport. It owns
// which emails exist and what they say; emailkit owns sending them, the
// suppression check and the audit row.
type Service struct{ kit *emailkit.Service }

// NewService wires the shared transport. Credentials resolve per send from
// app_settings so an admin key change takes effect without a restart, falling
// back to the env values when the table is empty.
// NewService takes the credential source as an explicit parameter rather than
// type-asserting it out of the store. An assertion that fails would fall back
// to env-only credentials and silently stop honouring the admin override —
// omitting this argument is a compile error instead.
func NewService(store emailkit.Store, creds CredentialSource, cfg Config) *Service {
	resolve := func(ctx context.Context) (string, string, error) {
		return resolveCredentials(ctx, creds, cfg)
	}
	return &Service{kit: emailkit.NewService(store, emailkit.Config{Resolve: resolve}, BuiltinRegistry())}
}

// resolveCredentials implements emailkit.Config.Resolve's contract: the
// store's override, when present, REPLACES the env fallback rather than
// layering over it. Extracted out of NewService's closure — this is the last
// transport-adjacent policy draftright still owns, and a bare closure had no
// name a test could call directly.
//
// An error from creds.Credentials propagates untouched and short-circuits the
// env fallback: see Config.Resolve's doc for why (sending with credentials an
// operator believes they replaced is worse than not sending). A nil error
// with empty strings — whether because no override row exists yet or because
// one exists with blank columns — is "no override configured" and falls
// through to cfg per field.
func resolveCredentials(ctx context.Context, creds CredentialSource, cfg Config) (string, string, error) {
	apiKey, from, err := creds.Credentials(ctx)
	if err != nil {
		return "", "", err
	}
	if apiKey == "" {
		apiKey = cfg.EnvAPIKey
	}
	if from == "" {
		from = cfg.EnvFrom
	}
	if from == "" {
		from = defaultFrom
	}
	return apiKey, from, nil
}

// Wait blocks until in-flight sends finish. Test-only.
func (s *Service) Wait() { s.kit.Wait() }

// SendVerification + SendPasswordReset are the two auth needs. Name fallback
// to "there" is applied here; the templates do not.
func (s *Service) SendVerification(ctx context.Context, to, name, code string) {
	s.kit.Send(ctx, TemplateVerification, to, map[string]string{"name": orThere(name), "code": code})
}

func (s *Service) SendPasswordReset(ctx context.Context, to, name, code string) {
	s.kit.Send(ctx, TemplatePasswordReset, to, map[string]string{"name": orThere(name), "code": code})
}

// SendRenewalReminder reminds the user their subscription renews soon.
func (s *Service) SendRenewalReminder(ctx context.Context, to, name, plan string, expiresAt time.Time, currency string, amount int) {
	s.kit.Send(ctx, TemplateRenewalReminder, to, map[string]string{
		"name": orThere(name), "plan": plan,
		"expires": dateString(expiresAt), "amount": formatAmount(currency, amount),
	})
}

// SendSubscriptionActivated confirms a successful payment — sub now active.
func (s *Service) SendSubscriptionActivated(ctx context.Context, to, name, plan string, expiresAt time.Time, currency string, amount int) {
	s.kit.Send(ctx, TemplateSubscriptionActivated, to, map[string]string{
		"name": orThere(name), "plan": plan,
		"expires": dateString(expiresAt), "amount": formatAmount(currency, amount),
	})
}

// SendPaymentFailed notifies the user a renewal charge failed.
func (s *Service) SendPaymentFailed(ctx context.Context, to, name, plan string) {
	s.kit.Send(ctx, TemplatePaymentFailed, to, map[string]string{"name": orThere(name), "plan": plan})
}

// SendSubscriptionExpired notifies the user their subscription has lapsed.
func (s *Service) SendSubscriptionExpired(ctx context.Context, to, name, plan string) {
	s.kit.Send(ctx, TemplateSubscriptionExpired, to, map[string]string{"name": orThere(name), "plan": plan})
}

// SendRaw fires a pre-rendered email through the same suppression and audit
// path. appsettings depends on this via its EmailSender port.
func (s *Service) SendRaw(ctx context.Context, to, subject, html, label string) {
	s.kit.SendRaw(ctx, to, subject, html, label)
}

// SendTestEmail is the admin-triggered "Send test email" — verifies Resend
// creds + DNS. Builds the inline HTML (no template) with an ISO timestamp,
// mirroring Node sendTestEmail. Fire-and-forget here (Node throws on error
// to surface in the admin toast; the Go HTTP edge handles that seam).
func (s *Service) SendTestEmail(ctx context.Context, to string) {
	html := `<!doctype html>
<html><body style="font-family:-apple-system,system-ui,sans-serif;background:#f5f5f7;padding:32px;margin:0;">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;">
    <h1 style="font-size:20px;margin:0 0 16px;color:#111;">It works.</h1>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">If you can read this, your Resend API key + sender domain are set up correctly. Renewal reminders, verification codes, and payment notices will all flow through this configuration.</p>
    <p style="color:#888;font-size:13px;margin:24px 0 0;">— DraftRight admin test, sent ` + shared.ISOMillis(time.Now()) + `</p>
  </div>
</body></html>`
	s.SendRaw(ctx, to, "DraftRight test email", html, "test email")
}

func orThere(n string) string {
	if n == "" {
		return "there"
	}
	return n
}

// formatAmount mirrors Node: USD → "$" + amount/100 (2dp); else en-US
// thousands-grouped amount + " " + currency. (USD 999 → $9.99; VND 50000
// → 50,000 VND.)
func formatAmount(currency string, amount int) string {
	if currency == "USD" {
		return fmt.Sprintf("$%.2f", float64(amount)/100)
	}
	return groupThousands(amount) + " " + currency
}

// groupThousands renders amount with en-US comma thousands separators
// (1234567 → "1,234,567"). Negative values are not expected (amounts are
// non-negative cents); handled defensively by grouping the magnitude.
func groupThousands(n int) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var b strings.Builder
	first := len(s) % 3
	if first == 0 {
		first = 3
	}
	b.WriteString(s[:first])
	for i := first; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// dateString replicates JS Date.toDateString() → "Www Mmm DD YYYY" (3-letter
// weekday, 3-letter month, zero-padded day). Go's "Mon Jan 02 2006" layout
// zero-pads the day, matching JS for both 1-digit (Fri Jun 05 2026) and
// 2-digit (Mon Jun 15 2026) days.
func dateString(t time.Time) string {
	return t.Format("Mon Jan 02 2006")
}
