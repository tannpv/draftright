package shared

import "testing"

func TestMaskSecret(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},                             // unset stays empty
		{"short", "…"},                       // <16 → marker only
		{"0123456789abcde", "…"},             // 15 runes → marker only
		{"0123456789abcdef", "012…cdef"},     // 16 runes → first3+marker+last4
		{"sk-proj-abcd1234wxyz", "sk-…wxyz"}, // typical key
	}
	for _, c := range cases {
		if got := MaskSecret(c.in); got != c.want {
			t.Errorf("MaskSecret(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestContainsMaskMarker(t *testing.T) {
	if !ContainsMaskMarker("sk-…wxyz") {
		t.Error("expected marker detected")
	}
	if ContainsMaskMarker("sk-proj-realkey-no-marker") {
		t.Error("false positive on real key")
	}
}
