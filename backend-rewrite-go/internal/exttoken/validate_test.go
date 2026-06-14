package exttoken_test

import (
	"strings"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/exttoken"
)

func TestValidateMint(t *testing.T) {
	const goodUUID = "550e8400-e29b-41d4-a716-446655440000" // v4

	cases := []struct {
		name       string
		deviceID   string
		deviceName string
		want       string // "" means valid
	}{
		{"valid", goodUUID, "iPhone 15", ""},
		{"valid with punctuation", goodUUID, "Tan_s iPhone-15.pro", ""},
		{"bad uuid", "not-a-uuid", "iPhone", "device_id must be a UUID"},
		{"v1 uuid rejected", "550e8400-e29b-11d4-a716-446655440000", "iPhone", "device_id must be a UUID"},
		// Empty name fails @Length (min) AND @Matches (the regex `+` rejects
		// the empty string) — class-validator collects both, joined with ". ".
		// The humanizer adds a period to the length message, so the join
		// produces a double period (".. "), matching Node byte-for-byte.
		{"empty name", goodUUID, "", "Device_name must be at least 1 characters.. device_name must be alphanumeric/space/_/./- only"},
		{"65 char name", goodUUID, strings.Repeat("a", 65), "device_name must be shorter than or equal to 64 characters"},
		{"illegal char", goodUUID, "a@b", "device_name must be alphanumeric/space/_/./- only"},
		// Both device_id and device_name fail → joined in declaration order.
		{"both bad", "nope", "a@b", "device_id must be a UUID. device_name must be alphanumeric/space/_/./- only"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := exttoken.ValidateMint(c.deviceID, c.deviceName)
			if got != c.want {
				t.Fatalf("ValidateMint() = %q, want %q", got, c.want)
			}
		})
	}

	// 64-char name is the boundary and must pass.
	if msg := exttoken.ValidateMint(goodUUID, strings.Repeat("a", 64)); msg != "" {
		t.Fatalf("64-char name should pass, got %q", msg)
	}
}
