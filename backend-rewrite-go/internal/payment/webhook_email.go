package payment

import "context"

// rawSender is the consumer-side port for fire-and-forget transactional email.
// *email.Service.SendRaw satisfies it (same seam as subscription.MailCronNotifier).
type rawSender interface {
	SendRaw(ctx context.Context, to, subject, html, label string)
}

// MailWebhookEmailer sends the two webhook-side emails. Best-effort: bodies are
// NOT shadow-gated (out-of-band), so copy is placeholder. TODO: align with the
// Node subscription-notifier + sendPaymentFailed templates.
type MailWebhookEmailer struct{ mail rawSender }

// NewMailWebhookEmailer wires the shared email sender.
func NewMailWebhookEmailer(mail rawSender) *MailWebhookEmailer {
	return &MailWebhookEmailer{mail: mail}
}

// SubscriptionActivated notifies the user their subscription is active.
func (e *MailWebhookEmailer) SubscriptionActivated(ctx context.Context, to, name, planName string) {
	e.mail.SendRaw(ctx, to,
		"Your DraftRight subscription is active",
		"<p>Your "+planName+" subscription is now active. Thank you!</p>",
		"subscription-activated")
}

// PaymentFailed notifies the user a renewal charge failed (update card before
// final dunning retry).
func (e *MailWebhookEmailer) PaymentFailed(ctx context.Context, to, name, planName string) {
	e.mail.SendRaw(ctx, to,
		"Action needed: your DraftRight payment failed",
		"<p>We couldn't process the renewal for your "+planName+" subscription. Please update your payment method.</p>",
		"payment-failed")
}
