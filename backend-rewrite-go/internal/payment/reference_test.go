package payment

import (
	"regexp"
	"testing"
)

func TestGeneratePaymentReference_Format(t *testing.T) {
	re := regexp.MustCompile(`^DR-PRO-[0-9A-F]{8}$`)
	for i := 0; i < 50; i++ {
		ref := GeneratePaymentReference()
		if !re.MatchString(ref) {
			t.Fatalf("ref %q does not match DR-PRO-<8 upper hex>", ref)
		}
	}
}

func TestGeneratePaymentReference_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		ref := GeneratePaymentReference()
		if seen[ref] {
			t.Fatalf("duplicate ref %q within 200 draws", ref)
		}
		seen[ref] = true
	}
}

func TestExtractPaymentReference(t *testing.T) {
	cases := map[string]string{
		"Payment for dr-pro-a1b2c3d4 thanks": "DR-PRO-A1B2C3D4",
		"NoRefHere":                          "",
		"":                                   "",
		"prefix DR-PRO-DEADBEEF suffix":      "DR-PRO-DEADBEEF",
	}
	for in, want := range cases {
		if got := ExtractPaymentReference(in); got != want {
			t.Errorf("ExtractPaymentReference(%q) = %q, want %q", in, got, want)
		}
	}
}
