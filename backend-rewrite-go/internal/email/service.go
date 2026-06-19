// Package email ports the NestJS EmailService: a single deliver path
// (suppression → creds → Resend POST → email_logs row) behind two
// auth-facing methods. Sends are FIRE-AND-FORGET — never block or fail
// the HTTP request. NOT shadow-gated (out-of-band).
package email

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// sender posts one email. resendClient (prod) + fakeSender (test) satisfy it.
type sender interface {
	send(ctx context.Context, apiKey, from, to, subject, html string) (providerID string, err error)
}

// Querier is the sqlc subset the service needs (consumer-side port).
type Querier interface {
	IsEmailSuppressed(ctx context.Context, email string) (bool, error)
	InsertEmailLog(ctx context.Context, a InsertEmailLogArgs) error
	GetEmailSettings(ctx context.Context) (apiKey, from string, err error)
	GetEmailTemplate(ctx context.Context, key string) (subject, html string, ok bool)
}

// InsertEmailLogArgs is the audit-row payload (thin wrapper so the port
// doesn't leak sqlc param structs to fakes).
type InsertEmailLogArgs struct {
	To, Type, Subject, Status string
	ProviderID, Error         *string
}

// Config carries the env fallbacks (app_settings overrides them).
type Config struct {
	EnvAPIKey string
	EnvFrom   string
}

const defaultFrom = "DraftRight <noreply@draftright.info>"

// Service is the email sender. wg tracks in-flight sends so tests can
// deterministically await them; prod never calls wait().
type Service struct {
	q      Querier
	cfg    Config
	client sender
	wg     sync.WaitGroup
}

// NewService wires the querier + env config + real Resend client.
func NewService(q Querier, cfg Config) *Service {
	return &Service{q: q, cfg: cfg, client: newResendClient()}
}

// SendVerification + SendPasswordReset are the two auth needs. Both
// fire-and-forget. Name fallback to "there" applied here (templates.go
// does not).
func (s *Service) SendVerification(ctx context.Context, to, name, code string) {
	s.fire(ctx, "verification", to, map[string]string{"name": orThere(name), "code": code})
}
func (s *Service) SendPasswordReset(ctx context.Context, to, name, code string) {
	s.fire(ctx, "password-reset", to, map[string]string{"name": orThere(name), "code": code})
}

// SendRenewalReminder reminds the user their subscription renews soon.
// Mirrors Node sendRenewalReminder: vars name(orThere)/plan/expires/amount.
func (s *Service) SendRenewalReminder(ctx context.Context, to, name, plan string, expiresAt time.Time, currency string, amount int) {
	s.fire(ctx, "renewal-reminder", to, map[string]string{
		"name": orThere(name), "plan": plan,
		"expires": dateString(expiresAt), "amount": formatAmount(currency, amount),
	})
}

// SendSubscriptionActivated confirms a successful payment — sub now active.
func (s *Service) SendSubscriptionActivated(ctx context.Context, to, name, plan string, expiresAt time.Time, currency string, amount int) {
	s.fire(ctx, "subscription-activated", to, map[string]string{
		"name": orThere(name), "plan": plan,
		"expires": dateString(expiresAt), "amount": formatAmount(currency, amount),
	})
}

// SendPaymentFailed notifies the user a renewal charge failed.
func (s *Service) SendPaymentFailed(ctx context.Context, to, name, plan string) {
	s.fire(ctx, "payment-failed", to, map[string]string{"name": orThere(name), "plan": plan})
}

// SendSubscriptionExpired notifies the user their subscription has lapsed.
func (s *Service) SendSubscriptionExpired(ctx context.Context, to, name, plan string) {
	s.fire(ctx, "subscription-expired", to, map[string]string{"name": orThere(name), "plan": plan})
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

// SendRaw fires a pre-rendered transactional email (subject + HTML body)
// through the same suppression → creds → Resend → email_logs path as the
// templated sends. label is the email_logs Type column. Fire-and-forget,
// like the templated methods. Used by out-of-band jobs (e.g. the expiry
// cron) that don't have a built-in template.
func (s *Service) SendRaw(ctx context.Context, to, subject, html, label string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() { _ = recover() }()
		s.deliver(context.WithoutCancel(ctx), to, subject, html, label)
	}()
}

func orThere(n string) string {
	if n == "" {
		return "there"
	}
	return n
}

func (s *Service) wait() { s.wg.Wait() } // test-only

func (s *Service) fire(ctx context.Context, key, to string, vars map[string]string) {
	subject, html := s.render(ctx, key, vars)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() { _ = recover() }() // never let an email panic escape
		s.deliver(context.WithoutCancel(ctx), to, subject, html, key)
	}()
}

// render applies a DB template override when present, else the built-in.
func (s *Service) render(ctx context.Context, key string, vars map[string]string) (string, string) {
	if subj, html, ok := s.q.GetEmailTemplate(ctx, key); ok {
		return substitute(subj, vars, false), substitute(html, vars, true)
	}
	return renderTemplate(key, vars)
}

func (s *Service) deliver(ctx context.Context, to, subject, html, label string) {
	if sup, err := s.q.IsEmailSuppressed(ctx, strings.ToLower(to)); err == nil && sup {
		s.log(ctx, to, subject, label, "suppressed", nil, strp("Recipient on suppression list (bounce/complaint)"))
		return
	}
	apiKey, from := s.creds(ctx)
	if apiKey == "" {
		s.log(ctx, to, subject, label, "skipped", nil, strp("Resend not configured"))
		return
	}
	id, err := s.client.send(ctx, apiKey, from, to, subject, html)
	if err != nil {
		slog.Warn("email send failed", "label", label, "to", to, "err", err)
		s.log(ctx, to, subject, label, "failed", nil, strp(err.Error()))
		return
	}
	s.log(ctx, to, subject, label, "sent", strpOrNil(id), nil)
}

func (s *Service) creds(ctx context.Context) (string, string) {
	apiKey, from, err := s.q.GetEmailSettings(ctx)
	if err != nil {
		apiKey, from = "", ""
	}
	if apiKey == "" {
		apiKey = s.cfg.EnvAPIKey
	}
	if from == "" {
		from = s.cfg.EnvFrom
	}
	if from == "" {
		from = defaultFrom
	}
	return apiKey, from
}

func (s *Service) log(ctx context.Context, to, subject, label, status string, providerID, errMsg *string) {
	_ = s.q.InsertEmailLog(ctx, InsertEmailLogArgs{
		To: to, Type: label, Subject: subject, Status: status, ProviderID: providerID, Error: errMsg,
	})
}

func strp(s string) *string { return &s }
func strpOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
