// Package email ports the NestJS EmailService: a single deliver path
// (suppression → creds → Resend POST → email_logs row) behind two
// auth-facing methods. Sends are FIRE-AND-FORGET — never block or fail
// the HTTP request. NOT shadow-gated (out-of-band).
package email

import (
	"context"
	"log/slog"
	"strings"
	"sync"
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
		return substitute(subj, vars), substitute(html, vars)
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
