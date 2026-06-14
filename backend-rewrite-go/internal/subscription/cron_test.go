package subscription_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	subscription "github.com/tannpv/draftright-rewrite/internal/subscription"
)

type fakeCronRepo struct {
	due     []subscription.RenewalRow
	dueErr  error
	expired []subscription.ExpiredRow
	expErr  error

	dueLo, dueHi time.Time
	expiredNow   time.Time
}

func (f *fakeCronRepo) DueForRenewal(ctx context.Context, lo, hi time.Time) ([]subscription.RenewalRow, error) {
	f.dueLo, f.dueHi = lo, hi
	return f.due, f.dueErr
}

func (f *fakeCronRepo) ExpireLapsed(ctx context.Context, now time.Time) ([]subscription.ExpiredRow, error) {
	f.expiredNow = now
	return f.expired, f.expErr
}

type fakeNotifier struct {
	reminders   int
	expired     int
	failExpired bool
}

func (f *fakeNotifier) SendRenewalReminder(ctx context.Context, row subscription.RenewalRow) error {
	f.reminders++
	return nil
}

func (f *fakeNotifier) SendExpired(ctx context.Context, row subscription.ExpiredRow) error {
	f.expired++
	if f.failExpired {
		return errors.New("notifier boom")
	}
	return nil
}

func TestCron_RunOnce_RemindersAndExpiry(t *testing.T) {
	now := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{
		due:     []subscription.RenewalRow{{Email: "a@b.com", PlanName: "Pro", ExpiresAt: now.Add(3 * 24 * time.Hour)}},
		expired: []subscription.ExpiredRow{{Email: "c@d.com", ExpiresAt: now.Add(-time.Hour)}},
	}
	notif := &fakeNotifier{failExpired: true}
	cron := subscription.NewExpiryCron(repo, notif, slog.Default())
	if err := cron.RunOnce(context.Background(), now); err != nil {
		t.Fatalf("RunOnce err %v", err)
	}
	if !repo.dueLo.Equal(now.Add(60*time.Hour)) || !repo.dueHi.Equal(now.Add(84*time.Hour)) {
		t.Fatalf("window bounds: lo=%v hi=%v", repo.dueLo, repo.dueHi)
	}
	if notif.reminders != 1 || notif.expired != 1 {
		t.Fatalf("notif counts r=%d e=%d", notif.reminders, notif.expired)
	}
}

// A repo error in either pass aborts RunOnce with that error.
func TestCron_RunOnce_RenewalRepoError(t *testing.T) {
	now := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)
	boom := errors.New("db down")
	repo := &fakeCronRepo{dueErr: boom}
	notif := &fakeNotifier{}
	cron := subscription.NewExpiryCron(repo, notif, slog.Default())
	if err := cron.RunOnce(context.Background(), now); !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
	if notif.reminders != 0 || notif.expired != 0 {
		t.Fatalf("no notify expected, got r=%d e=%d", notif.reminders, notif.expired)
	}
}

func TestCron_RunOnce_ExpireRepoError(t *testing.T) {
	now := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)
	boom := errors.New("update failed")
	repo := &fakeCronRepo{expErr: boom}
	notif := &fakeNotifier{}
	cron := subscription.NewExpiryCron(repo, notif, slog.Default())
	if err := cron.RunOnce(context.Background(), now); !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
}

// A notifier that keeps failing on SendExpired must not stop the loop:
// every expired row is still attempted.
func TestCron_RunOnce_NotifierFailureDoesNotStopLoop(t *testing.T) {
	now := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{
		expired: []subscription.ExpiredRow{
			{Email: "a@x.com"}, {Email: "b@x.com"}, {Email: "c@x.com"},
		},
	}
	notif := &fakeNotifier{failExpired: true}
	cron := subscription.NewExpiryCron(repo, notif, slog.Default())
	if err := cron.RunOnce(context.Background(), now); err != nil {
		t.Fatalf("RunOnce err %v", err)
	}
	if notif.expired != 3 {
		t.Fatalf("want 3 expire attempts, got %d", notif.expired)
	}
	if !repo.expiredNow.Equal(now) {
		t.Fatalf("ExpireLapsed now mismatch: %v", repo.expiredNow)
	}
}
