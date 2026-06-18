package appsettings

import (
	"context"
	"errors"
	"strings"
)

// Repo is the persistence port for app settings (satisfied by *PgRepo).
type Repo interface {
	GetOrCreate(ctx context.Context) (AppSettings, error)
	Patch(ctx context.Context, p Patch) (AppSettings, error)
}

// MethodValidator rejects enabling a payment method that has no backend
// strategy. Satisfied by the payment module's AssertMethodsRegisterable
// (wired in main.go). Consumer-side port — kept to one method.
type MethodValidator interface {
	AssertMethodsRegisterable(csv string) error
}

// EmailSender sends a pre-rendered transactional email. Satisfied by
// *email.Service.SendRaw (wired in main.go). Fire-and-forget, no error
// return — mirrors the real SendRaw signature so main.go injects directly.
type EmailSender interface {
	SendRaw(ctx context.Context, to, subject, html, label string)
}

// Service is the admin app-settings use case: read, patch (with payment
// validation), and send a test email. Mirrors admin.controller.ts's
// updateSettings + sendTestEmail handlers.
type Service struct {
	repo      Repo
	validator MethodValidator
	sender    EmailSender
}

func NewService(repo Repo, v MethodValidator, s EmailSender) *Service {
	return &Service{repo: repo, validator: v, sender: s}
}

func (s *Service) Get(ctx context.Context) (AppSettings, error) { return s.repo.GetOrCreate(ctx) }

// InvalidSettingsError marks a Patch failure caused by request-data validation
// (e.g. payment_methods_enabled naming an unregisterable method). The handler
// maps it to HTTP 400; a bare repo error maps to 500. Error() forwards the
// wrapped message verbatim so the client body stays byte-identical to Node's
// BadRequestException payload.
type InvalidSettingsError struct{ Err error }

func (e InvalidSettingsError) Error() string { return e.Err.Error() }
func (e InvalidSettingsError) Unwrap() error { return e.Err }

// Patch validates payment_methods_enabled (when supplied) BEFORE persisting,
// matching Node updateSettings: assertMethodsRegisterable runs first, then
// the row is updated. A validation failure is wrapped in InvalidSettingsError
// (handler → 400); a repo failure propagates raw (handler → 500).
func (s *Service) Patch(ctx context.Context, p Patch) (AppSettings, error) {
	if p.PaymentMethodsEnabled != nil {
		if err := s.validator.AssertMethodsRegisterable(*p.PaymentMethodsEnabled); err != nil {
			return AppSettings{}, InvalidSettingsError{Err: err}
		}
	}
	return s.repo.Patch(ctx, p)
}

// SendTestEmail mirrors admin.controller.ts sendTestEmail: '@' guard then
// send. Error string matches Node's exactly ("Valid recipient email
// required").
func (s *Service) SendTestEmail(ctx context.Context, to string) error {
	if to == "" || !strings.Contains(to, "@") {
		return errors.New("Valid recipient email required")
	}
	s.sender.SendRaw(ctx, to, "DraftRight test email", testEmailHTML(), "test email")
	return nil
}

// testEmailHTML is the "It works." test-email body, mirroring the STRUCTURE
// of email.service.ts sendTestEmail. Node interpolates a non-deterministic
// timestamp; this out-of-band email is not shadow-gated, so we omit the
// timestamp (no time call) and keep a static body.
func testEmailHTML() string {
	return `<!doctype html>
<html><body style="font-family:-apple-system,system-ui,sans-serif;background:#f5f5f7;padding:32px;margin:0;">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;">
    <h1 style="font-size:20px;margin:0 0 16px;color:#111;">It works.</h1>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">If you can read this, your Resend API key + sender domain are set up correctly. Renewal reminders, verification codes, and payment notices will all flow through this configuration.</p>
    <p style="color:#888;font-size:13px;margin:24px 0 0;">— DraftRight admin test</p>
  </div>
</body></html>`
}
