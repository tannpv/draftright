package payment

import (
	"context"
	"testing"
	"time"
)

type fakeSettings struct {
	csv   string
	found bool
	err   error
}

func (f fakeSettings) PaymentMethodsEnabled(ctx context.Context) (string, bool, error) {
	return f.csv, f.found, f.err
}

type fakeRepo struct {
	status *StatusRow
	hist   []PaymentRow
}

func (f fakeRepo) GetByReference(ctx context.Context, ref string) (*StatusRow, error) {
	return f.status, nil
}
func (f fakeRepo) ListByUser(ctx context.Context, uid string) ([]PaymentRow, error) {
	return f.hist, nil
}

func TestService_EnabledMethods_SettingsWins(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeSettings{csv: "vietqr", found: true}, "lemonsqueezy", nil, nil, nil, nil)
	got, err := svc.EnabledMethods(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"vietqr", "bank_transfer"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("settings CSV must win → %v, got %v", want, got)
	}
}

func TestService_EnabledMethods_EnvFallback(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeSettings{found: false}, "lemonsqueezy", nil, nil, nil, nil)
	got, _ := svc.EnabledMethods(context.Background())
	if len(got) != 1 || got[0] != "lemonsqueezy" {
		t.Fatalf("env fallback → [lemonsqueezy], got %v", got)
	}
}

func TestService_EnabledMethods_DefaultStripe(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeSettings{found: false}, "", nil, nil, nil, nil)
	got, _ := svc.EnabledMethods(context.Background())
	if len(got) != 1 || got[0] != "stripe" {
		t.Fatalf("no settings + no env → [stripe], got %v", got)
	}
}

func TestService_EnabledMethods_EmptySettingsStringFallsThrough(t *testing.T) {
	// found=true but empty CSV must NOT win — fall through to env.
	svc := NewService(fakeRepo{}, fakeSettings{csv: "", found: true}, "lemonsqueezy", nil, nil, nil, nil)
	got, _ := svc.EnabledMethods(context.Background())
	if len(got) != 1 || got[0] != "lemonsqueezy" {
		t.Fatalf("empty settings CSV → env fallback [lemonsqueezy], got %v", got)
	}
}

func TestService_Status_NotFound(t *testing.T) {
	svc := NewService(fakeRepo{status: nil}, fakeSettings{}, "", nil, nil, nil, nil)
	v, err := svc.Status(context.Background(), "DR-PRO-NOPE")
	if err != nil {
		t.Fatal(err)
	}
	if v.NotFound() == false {
		t.Fatal("nil status row → NotFound view")
	}
}

func TestService_Status_FoundProjectsISO(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	name := "Pro"
	svc := NewService(fakeRepo{status: &StatusRow{
		Status: "completed", Method: "stripe", Amount: 900, Currency: "USD",
		ReferenceCode: "DR-PRO-ABCD1234", PlanName: &name, CompletedAt: &ts,
	}}, fakeSettings{}, "", nil, nil, nil, nil)
	v, _ := svc.Status(context.Background(), "DR-PRO-ABCD1234")
	if v.NotFound() {
		t.Fatal("should be found")
	}
	if v.CompletedAt == nil || *v.CompletedAt == "" {
		t.Fatal("completed_at should be ISO string")
	}
	if v.ExpiresAt != nil {
		t.Fatal("nil expires_at must stay nil")
	}
}

func TestService_History_PassesThrough(t *testing.T) {
	rows := []PaymentRow{{ReferenceCode: "DR-PRO-1", CreatedAt: time.Now()}}
	svc := NewService(fakeRepo{hist: rows}, fakeSettings{}, "", nil, nil, nil, nil)
	got, err := svc.History(context.Background(), "u1")
	if err != nil || len(got) != 1 {
		t.Fatalf("history passthrough failed: %v %v", got, err)
	}
}
