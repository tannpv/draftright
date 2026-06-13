package user

import "testing"

func TestUUIDRoundTrip(t *testing.T) {
	const in = "11111111-1111-1111-1111-111111111111"
	u, err := parseUUID(in)
	if err != nil {
		t.Fatalf("parseUUID(%q) failed: %v", in, err)
	}
	if got := uuidStr(u); got != in {
		t.Fatalf("uuidStr round-trip = %q, want %q", got, in)
	}
}
