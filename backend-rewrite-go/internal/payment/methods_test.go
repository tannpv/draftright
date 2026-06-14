package payment

import (
	"reflect"
	"testing"
)

func TestRegisteredMethods(t *testing.T) {
	got := RegisteredMethods()
	want := []string{"stripe", "vietqr", "bank_transfer", "lemonsqueezy", "apple_pay", "google_pay"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RegisteredMethods() = %v, want %v", got, want)
	}
}

func TestEnabledMethods_OrderingAndVietQRImplication(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"blank yields empty set (default lives in usecase precedence)", "", []string{}},
		{"whitespace-only yields empty set (JS-truthy, Node filters to [])", "   ", []string{}},
		{"single", "lemonsqueezy", []string{"lemonsqueezy"}},
		{"vietqr implies bank_transfer appended last", "vietqr", []string{"vietqr", "bank_transfer"}},
		{"insertion order preserved", "lemonsqueezy,stripe", []string{"lemonsqueezy", "stripe"}},
		{"bank_transfer already present keeps its position", "bank_transfer,vietqr", []string{"bank_transfer", "vietqr"}},
		{"dedupe + trim + lowercase", " Stripe , stripe ,LEMONSQUEEZY", []string{"stripe", "lemonsqueezy"}},
		{"vietqr mid-list still appends bank_transfer at end", "stripe,vietqr,lemonsqueezy", []string{"stripe", "vietqr", "lemonsqueezy", "bank_transfer"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EnabledMethods(c.raw)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("EnabledMethods(%q) = %v, want %v", c.raw, got, c.want)
			}
		})
	}
}

func TestAssertMethodsRegisterable(t *testing.T) {
	if err := AssertMethodsRegisterable("stripe,vietqr"); err != nil {
		t.Fatalf("registerable CSV should pass, got %v", err)
	}
	if err := AssertMethodsRegisterable(""); err != nil {
		t.Fatalf("blank CSV is allowed (falls back to default), got %v", err)
	}
	err := AssertMethodsRegisterable("stripe,paypal,momo")
	if err == nil {
		t.Fatal("paypal+momo have no strategy → must error")
	}
	want := "Cannot enable payment method(s) with no backend strategy: paypal, momo. " +
		"Registered methods: stripe, vietqr, bank_transfer, lemonsqueezy, apple_pay, google_pay."
	if err.Error() != want {
		t.Fatalf("error =\n%q\nwant\n%q", err.Error(), want)
	}
}
