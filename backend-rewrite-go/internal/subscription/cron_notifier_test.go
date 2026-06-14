package subscription

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type fakeRawSender struct {
	to, subject, html, label string
	calls                    int
}

func (f *fakeRawSender) SendRaw(_ context.Context, to, subject, html, label string) {
	f.to, f.subject, f.html, f.label = to, subject, html, label
	f.calls++
}

func TestMailCronNotifierSendRenewalReminder(t *testing.T) {
	fake := &fakeRawSender{}
	n := NewMailCronNotifier(fake, slog.Default())
	row := RenewalRow{
		UserID:    "u1",
		Email:     "renew@example.com",
		PlanName:  "Pro",
		ExpiresAt: time.Now(),
	}
	if err := n.SendRenewalReminder(context.Background(), row); err != nil {
		t.Fatalf("SendRenewalReminder err = %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("calls = %d, want 1", fake.calls)
	}
	if fake.to != row.Email {
		t.Fatalf("to = %q, want %q", fake.to, row.Email)
	}
	if !strings.Contains(fake.html, "Pro") {
		t.Fatalf("body %q does not mention plan name", fake.html)
	}
}

func TestMailCronNotifierSendExpired(t *testing.T) {
	fake := &fakeRawSender{}
	n := NewMailCronNotifier(fake, slog.Default())
	row := ExpiredRow{UserID: "u2", Email: "expired@example.com", ExpiresAt: time.Now()}
	if err := n.SendExpired(context.Background(), row); err != nil {
		t.Fatalf("SendExpired err = %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("calls = %d, want 1", fake.calls)
	}
	if fake.to != row.Email {
		t.Fatalf("to = %q, want %q", fake.to, row.Email)
	}
}

func TestMailCronNotifierSatisfiesCronNotifier(t *testing.T) {
	var _ CronNotifier = (*MailCronNotifier)(nil)
}
