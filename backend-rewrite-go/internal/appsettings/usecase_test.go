package appsettings

import (
	"context"
	"errors"
	"testing"
)

type fakeRepo struct{ patched Patch }

func (f *fakeRepo) GetOrCreate(context.Context) (AppSettings, error) {
	return AppSettings{ID: "1"}, nil
}
func (f *fakeRepo) Patch(_ context.Context, p Patch) (AppSettings, error) {
	f.patched = p
	return AppSettings{ID: "1"}, nil
}

type fakeValidator struct{ err error }

func (f fakeValidator) AssertMethodsRegisterable(string) error { return f.err }

type fakeSender struct{ to, subject string }

func (f *fakeSender) SendRaw(_ context.Context, to, subject, _, _ string) {
	f.to = to
	f.subject = subject
}

func TestPatch_RejectsBadPaymentMethods(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, fakeValidator{err: errors.New("Cannot enable payment method(s)...")}, &fakeSender{})
	_, err := svc.Patch(context.Background(), Patch{PaymentMethodsEnabled: ptr("bogus")})
	if err == nil {
		t.Fatal("invalid payment_methods_enabled must error before persist")
	}
	if repo.patched.PaymentMethodsEnabled != nil {
		t.Fatal("repo.Patch must NOT run when validation fails")
	}
}

func TestPatch_PersistsWhenValid(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, fakeValidator{}, &fakeSender{})
	if _, err := svc.Patch(context.Background(), Patch{PaymentMethodsEnabled: ptr("stripe")}); err != nil {
		t.Fatal(err)
	}
	if repo.patched.PaymentMethodsEnabled == nil || *repo.patched.PaymentMethodsEnabled != "stripe" {
		t.Fatalf("repo.Patch got %v", repo.patched.PaymentMethodsEnabled)
	}
}

func TestTestEmail_RequiresAt(t *testing.T) {
	svc := NewService(&fakeRepo{}, fakeValidator{}, &fakeSender{})
	if err := svc.SendTestEmail(context.Background(), "no-at-sign"); err == nil || err.Error() != "Valid recipient email required" {
		t.Fatalf("got %v", err)
	}
}

func TestTestEmail_Sends(t *testing.T) {
	snd := &fakeSender{}
	svc := NewService(&fakeRepo{}, fakeValidator{}, snd)
	if err := svc.SendTestEmail(context.Background(), "a@b.com"); err != nil {
		t.Fatal(err)
	}
	if snd.to != "a@b.com" || snd.subject != "DraftRight test email" {
		t.Fatalf("sender got to=%q subject=%q", snd.to, snd.subject)
	}
}
