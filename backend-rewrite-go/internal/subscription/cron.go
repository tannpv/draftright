package subscription

import (
	"context"
	"log/slog"
	"time"
)

// RenewalRow / ExpiredRow are the cron's domain projections.
type RenewalRow struct {
	UserID    string
	Email     string
	PlanName  string
	ExpiresAt time.Time
}
type ExpiredRow struct {
	UserID    string
	Email     string
	ExpiresAt time.Time
}

// CronRepo is the consumer-side port for the expiry cron.
type CronRepo interface {
	DueForRenewal(ctx context.Context, lo, hi time.Time) ([]RenewalRow, error)
	ExpireLapsed(ctx context.Context, now time.Time) ([]ExpiredRow, error)
}

// CronNotifier sends the two emails. Best-effort: RunOnce ignores its errors.
type CronNotifier interface {
	SendRenewalReminder(ctx context.Context, row RenewalRow) error
	SendExpired(ctx context.Context, row ExpiredRow) error
}

// ExpiryCron is the daily entitlement-expiry job (port of subscriptions.cron.ts).
type ExpiryCron struct {
	repo  CronRepo
	notif CronNotifier
	log   *slog.Logger
}

// NewExpiryCron wires the cron.
func NewExpiryCron(repo CronRepo, notif CronNotifier, log *slog.Logger) *ExpiryCron {
	return &ExpiryCron{repo: repo, notif: notif, log: log}
}

// Renewal-reminder window: [now+2.5d, now+3.5d].
const (
	renewalLoHours = 60 * time.Hour // 2.5 days
	renewalHiHours = 84 * time.Hour // 3.5 days
)

// RunOnce executes both passes for the given now. Pass 1: renewal reminders
// (no mutation). Pass 2: expire lapsed (flip status, then email). Notifier
// failures are logged, never fatal.
func (c *ExpiryCron) RunOnce(ctx context.Context, now time.Time) error {
	due, err := c.repo.DueForRenewal(ctx, now.Add(renewalLoHours), now.Add(renewalHiHours))
	if err != nil {
		return err
	}
	for _, row := range due {
		if err := c.notif.SendRenewalReminder(ctx, row); err != nil {
			c.log.Warn("renewal reminder failed", "email", row.Email, "err", err)
		}
	}
	expired, err := c.repo.ExpireLapsed(ctx, now)
	if err != nil {
		return err
	}
	for _, row := range expired {
		if err := c.notif.SendExpired(ctx, row); err != nil {
			c.log.Warn("expired notice failed", "email", row.Email, "err", err)
		}
	}
	return nil
}
