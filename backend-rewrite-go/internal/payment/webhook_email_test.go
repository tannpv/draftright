package payment

import (
	"context"
	"testing"
)

type capRawSender struct {
	calls []struct{ to, subject, label string }
}

func (c *capRawSender) SendRaw(_ context.Context, to, subject, html, label string) {
	c.calls = append(c.calls, struct{ to, subject, label string }{to, subject, label})
}

func TestWebhookEmailer_SubscriptionActivated(t *testing.T) {
	c := &capRawSender{}
	e := NewMailWebhookEmailer(c)
	e.SubscriptionActivated(context.Background(), "u@x.com", "Uma", "Pro")
	if len(c.calls) != 1 || c.calls[0].to != "u@x.com" || c.calls[0].label != "subscription-activated" {
		t.Fatalf("bad activation send: %+v", c.calls)
	}
}

func TestWebhookEmailer_PaymentFailed(t *testing.T) {
	c := &capRawSender{}
	e := NewMailWebhookEmailer(c)
	e.PaymentFailed(context.Background(), "u@x.com", "Uma", "Pro")
	if len(c.calls) != 1 || c.calls[0].label != "payment-failed" {
		t.Fatalf("bad payment-failed send: %+v", c.calls)
	}
}
