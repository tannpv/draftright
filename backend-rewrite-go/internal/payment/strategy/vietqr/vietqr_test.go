package vietqr

import (
	"context"
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
