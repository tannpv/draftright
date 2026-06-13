package config

import (
	"os"
	"testing"
)

func TestLoad_ReadsRefreshSecret(t *testing.T) {
	os.Setenv("JWT_SECRET", "access-secret")
	os.Setenv("JWT_REFRESH_SECRET", "refresh-secret")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("JWT_REFRESH_SECRET")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.JWTRefreshSecret != "refresh-secret" {
		t.Fatalf("JWTRefreshSecret = %q, want refresh-secret", c.JWTRefreshSecret)
	}
}
