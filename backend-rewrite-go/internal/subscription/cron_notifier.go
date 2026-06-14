package subscription

import (
	"context"
	"log/slog"
)

// rawSender is the consumer-side port for fire-and-forget transactional
// email. *email.Service.SendRaw satisfies it; main.go injects the real one.
// Sends are best-effort (the email module never blocks or fails a caller),
// so the adapter always returns nil.
type rawSender interface {
	SendRaw(ctx context.Context, to, subject, html, label string)
}

// MailCronNotifier implements CronNotifier via the shared email sender.
// Best-effort: email-body parity is NOT shadow-gated (cron is not compared
// against Node), so bodies are placeholder text. TODO: align copy with the
// Node subscriptions notifier templates if/when cron is brought under the
// gate.
type MailCronNotifier struct {
	mail rawSender
	log  *slog.Logger
}

// NewMailCronNotifier wires the notifier to the shared email sender.
func NewMailCronNotifier(mail rawSender, log *slog.Logger) *MailCronNotifier {
	return &MailCronNotifier{mail: mail, log: log}
}

var _ CronNotifier = (*MailCronNotifier)(nil)

// SendRenewalReminder emails the user that their subscription renews soon.
func (n *MailCronNotifier) SendRenewalReminder(ctx context.Context, row RenewalRow) error {
	// TODO: match Node renewal-reminder template copy.
	n.mail.SendRaw(ctx, row.Email,
		"Your DraftRight subscription renews soon",
		"<p>Your "+row.PlanName+" subscription renews in a few days.</p>",
		"renewal-reminder")
	return nil
}

// SendExpired emails the user that their subscription has lapsed.
func (n *MailCronNotifier) SendExpired(ctx context.Context, row ExpiredRow) error {
	// TODO: match Node expired-notice template copy.
	n.mail.SendRaw(ctx, row.Email,
		"Your DraftRight subscription has expired",
		"<p>Your subscription has expired. Renew to restore Pro features.</p>",
		"subscription-expired")
	return nil
}
