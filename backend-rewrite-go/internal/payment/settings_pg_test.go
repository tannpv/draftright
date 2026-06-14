package payment

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

type fakeCoreQ struct {
	val string
	err error
}

func (f fakeCoreQ) GetPaymentMethodsEnabled(ctx context.Context) (string, error) {
	return f.val, f.err
}

func TestSettingsAdapter_Present(t *testing.T) {
	a := NewSettingsAdapter(fakeCoreQ{val: "stripe,vietqr"})
	got, found, err := a.PaymentMethodsEnabled(context.Background())
	if err != nil || !found || got != "stripe,vietqr" {
		t.Fatalf("present CSV → (csv,true,nil), got (%q,%v,%v)", got, found, err)
	}
}

func TestSettingsAdapter_EmptyString(t *testing.T) {
	a := NewSettingsAdapter(fakeCoreQ{val: ""})
	_, found, err := a.PaymentMethodsEnabled(context.Background())
	if err != nil || found {
		t.Fatalf("empty CSV → found=false (fall through to env), got found=%v err=%v", found, err)
	}
}

func TestSettingsAdapter_NoRow(t *testing.T) {
	a := NewSettingsAdapter(fakeCoreQ{err: pgx.ErrNoRows})
	_, found, err := a.PaymentMethodsEnabled(context.Background())
	if err != nil || found {
		t.Fatalf("no app_settings row → ('',false,nil), got found=%v err=%v", found, err)
	}
}
