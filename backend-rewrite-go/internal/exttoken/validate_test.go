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
		wantErr    string // "" means expect nil
	}{
		{"valid", goodUUID, "iPhone 15", ""},
		{"valid with punctuation", goodUUID, "Tan_s iPhone-15.pro", ""},
		{"bad uuid", "not-a-uuid", "iPhone", "device_id must be a valid UUID"},
		{"v1 uuid rejected", "550e8400-e29b-11d4-a716-446655440000", "iPhone", "device_id must be a valid UUID"},
		{"empty name", goodUUID, "", "device_name must be 1-64 characters"},
		{"65 char name", goodUUID, strings.Repeat("a", 65), "device_name must be 1-64 characters"},
		{"illegal char", goodUUID, "a@b", "device_name must be alphanumeric/space/_/./- only"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := exttoken.ValidateMint(c.deviceID, c.deviceName)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %q, got nil", c.wantErr)
			}
			if err.Error() != c.wantErr {
				t.Fatalf("error = %q, want %q", err.Error(), c.wantErr)
			}
		})
	}

	// 64-char name is the boundary and must pass.
	if err := exttoken.ValidateMint(goodUUID, strings.Repeat("a", 64)); err != nil {
		t.Fatalf("64-char name should pass, got %v", err)
	}
}
