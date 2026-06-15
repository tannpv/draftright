package vietqr

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

func newStrat() *Strategy {
	return New(Creds{BankID: "MB", AccountNumber: "0000000000", AccountName: "DRAFTRIGHT"})
}

func TestVietQR_QRUrlAndBankInfo(t *testing.T) {
	p := strategy.Payment{Amount: 124000, ReferenceCode: "DR-PRO-ABCD1234", Method: "vietqr"}
	res, err := newStrat().CreateCheckout(context.Background(), p, strategy.Plan{}, strategy.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := "https://img.vietqr.io/image/MB-0000000000-compact2.jpg?amount=124000&addInfo=DR-PRO-ABCD1234&accountName=DRAFTRIGHT"
	if res.QRData != want {
		t.Fatalf("qr=\n%q\nwant\n%q", res.QRData, want)
	}
	if res.BankInfo == nil || res.BankInfo.BankName != "MB Bank (Quân Đội)" || res.BankInfo.Reference != "DR-PRO-ABCD1234" || res.BankInfo.Currency != "VND" || res.BankInfo.Amount != 124000 {
		t.Fatalf("bank_info wrong: %+v", res.BankInfo)
	}
}

func TestVietQR_BankTransferNoQR(t *testing.T) {
	p := strategy.Payment{Amount: 50000, ReferenceCode: "DR-PRO-DEADBEEF", Method: "bank_transfer"}
	res, err := newStrat().CreateCheckout(context.Background(), p, strategy.Plan{}, strategy.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.QRData != "" {
		t.Fatalf("bank_transfer must NOT set qr_data, got %q", res.QRData)
	}
	if res.BankInfo == nil {
		t.Fatal("bank_transfer must still return bank_info")
	}
}

func TestVietQR_Defaults(t *testing.T) {
	res, _ := New(Creds{}).CreateCheckout(context.Background(),
		strategy.Payment{Amount: 1, ReferenceCode: "DR-PRO-00000000", Method: "vietqr"},
		strategy.Plan{}, strategy.Options{})
	if !strings.HasPrefix(res.QRData, "https://img.vietqr.io/image/MB-0000000000-compact2.jpg") {
		t.Fatalf("empty creds must default MB/0000000000, got %q", res.QRData)
	}
}

func TestVietQR_AddInfoEncoding(t *testing.T) {
	// accountName with a space must be %20-encoded (Node encodeURIComponent).
	res, _ := New(Creds{BankID: "MB", AccountNumber: "1", AccountName: "DRAFT RIGHT"}).
		CreateCheckout(context.Background(),
			strategy.Payment{Amount: 1, ReferenceCode: "DR-PRO-00000000", Method: "vietqr"},
			strategy.Plan{}, strategy.Options{})
	if !strings.Contains(res.QRData, "accountName=DRAFT%20RIGHT") {
		t.Fatalf("space must encode as %%20, got %q", res.QRData)
	}
}

func TestVietQR_PortalAndCancelUnsupported(t *testing.T) {
	s := newStrat()
	if url, _ := s.CustomerPortalURL(context.Background(), strategy.PortalUser{}); url != "" {
		t.Fatal("vietqr has no portal")
	}
	if ok, _ := s.CancelSubscription(context.Background(), "x"); ok {
		t.Fatal("vietqr cannot cancel")
	}
}

func TestVietQRVerify_MissingAuth401(t *testing.T) {
	s := New(Creds{CassoAPIKey: "ck", SepayAPIKey: "sk"})
	_, err := s.VerifyWebhook(context.Background(), []byte(`{}`), http.Header{})
	assertWebhookErr(t, err, 401, "Missing webhook authorization")
}

func TestVietQRVerify_NotConfigured401(t *testing.T) {
	s := New(Creds{}) // no keys
	h := http.Header{}
	h.Set("Authorization", "Apikey whatever")
	_, err := s.VerifyWebhook(context.Background(), []byte(`{}`), h)
	assertWebhookErr(t, err, 401, "VietQR webhooks not configured")
}

func TestVietQRVerify_WrongKey401(t *testing.T) {
	s := New(Creds{CassoAPIKey: "ck"})
	h := http.Header{}
	h.Set("Authorization", "Apikey nope")
	_, err := s.VerifyWebhook(context.Background(), []byte(`{}`), h)
	assertWebhookErr(t, err, 401, "Invalid webhook authorization")
}

func TestVietQRVerify_CassoCompleted(t *testing.T) {
	s := New(Creds{CassoAPIKey: "ck"})
	h := http.Header{}
	h.Set("Authorization", "Apikey ck")
	body := `{"data":[{"description":"chuyen khoan DR-PRO-ABCD","amount":124000}]}`
	act, err := s.VerifyWebhook(context.Background(), []byte(body), h)
	if err != nil || act.Type != strategy.ActionPaymentCompleted || act.ReferenceCode != "DR-PRO-ABCD" {
		t.Fatalf("casso: %+v %v", act, err)
	}
}

func TestVietQRVerify_SepayViaSecureToken(t *testing.T) {
	s := New(Creds{SepayAPIKey: "sk"})
	h := http.Header{}
	h.Set("Secure-Token", "sk")
	body := `{"content":"DR-PRO-WXYZ thanks","transferAmount":124000}`
	act, err := s.VerifyWebhook(context.Background(), []byte(body), h)
	if err != nil || act.Type != strategy.ActionPaymentCompleted || act.ReferenceCode != "DR-PRO-WXYZ" {
		t.Fatalf("sepay: %+v %v", act, err)
	}
}

func TestVietQRVerify_AuthedButNoRefIgnored(t *testing.T) {
	s := New(Creds{CassoAPIKey: "ck"})
	h := http.Header{}
	h.Set("Authorization", "Apikey ck")
	act, err := s.VerifyWebhook(context.Background(), []byte(`{"data":[{"description":"no ref here","amount":1}]}`), h)
	if err != nil || act.Type != strategy.ActionIgnored {
		t.Fatalf("want ignored, got %+v %v", act, err)
	}
}

func TestVietQRVerify_ApikeyNoSpaceNotStripped(t *testing.T) {
	// "Apikeyck" has no whitespace after "Apikey" → Node leaves it whole, so it
	// is NOT a valid key match (key is "ck") → 401 invalid.
	s := New(Creds{CassoAPIKey: "ck"})
	h := http.Header{}
	h.Set("Authorization", "Apikeyck")
	_, err := s.VerifyWebhook(context.Background(), []byte(`{}`), h)
	assertWebhookErr(t, err, 401, "Invalid webhook authorization")
}

func TestVietQRVerify_FalsyTransactionIdIgnored(t *testing.T) {
	// transactionId 0 is JS-falsy → MB branch skipped → Ignored (Node parity).
	s := New(Creds{CassoAPIKey: "ck"})
	h := http.Header{}
	h.Set("Authorization", "Apikey ck")
	body := `{"transactionId":0,"description":"chuyen khoan DR-PRO-MBMB","creditAmount":5}`
	act, err := s.VerifyWebhook(context.Background(), []byte(body), h)
	if err != nil || act.Type != strategy.ActionIgnored {
		t.Fatalf("want ignored, got %+v %v", act, err)
	}
}

func assertWebhookErr(t *testing.T, err error, status int, msg string) {
	t.Helper()
	var we *strategy.WebhookError
	if !errors.As(err, &we) || we.Status != status || we.Message != msg {
		t.Fatalf("want WebhookError{%d,%q}, got %v", status, msg, err)
	}
}
