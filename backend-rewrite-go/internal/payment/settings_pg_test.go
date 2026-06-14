package payment

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeCoreQ struct {
	val      string
	err      error
	creds    sqlc.GetPaymentCredentialsRow
	credsErr error
}

func (f fakeCoreQ) GetPaymentMethodsEnabled(ctx context.Context) (string, error) {
	return f.val, f.err
}

func (f fakeCoreQ) GetPaymentCredentials(ctx context.Context) (sqlc.GetPaymentCredentialsRow, error) {
	return f.creds, f.credsErr
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

func TestSettingsAdapter_Credentials(t *testing.T) {
	a := NewSettingsAdapter(fakeCoreQ{creds: sqlc.GetPaymentCredentialsRow{
		StripeSecretKey: "sk_db", VietqrBankID: "ACB", LemonsqueezyStoreID: "99",
	}})
	c, err := a.Credentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if c.StripeSecretKey != "sk_db" || c.VietQRBankID != "ACB" || c.LemonSqueezyStoreID != "99" {
		t.Fatalf("creds not mapped: %+v", c)
	}
}

func TestSettingsAdapter_CredentialsNoRow(t *testing.T) {
	a := NewSettingsAdapter(fakeCoreQ{credsErr: pgx.ErrNoRows})
	c, err := a.Credentials(context.Background())
	if err != nil {
		t.Fatalf("no row must be (zero,nil), got err=%v", err)
	}
	if c.StripeSecretKey != "" {
		t.Fatalf("no row must yield empty creds, got %+v", c)
	}
}
