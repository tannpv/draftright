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

func TestLoad_ReadsEmailAndAppleEnv(t *testing.T) {
	os.Setenv("JWT_SECRET", "s")
	os.Setenv("RESEND_API_KEY", "re_123")
	os.Setenv("EMAIL_FROM", "DraftRight <x@y.z>")
	os.Setenv("APPLE_AUDIENCES", "com.a,com.b")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("RESEND_API_KEY")
		os.Unsetenv("EMAIL_FROM")
		os.Unsetenv("APPLE_AUDIENCES")
	}()
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ResendAPIKey != "re_123" || c.EmailFrom != "DraftRight <x@y.z>" || c.AppleAudiences != "com.a,com.b" {
		t.Fatalf("got %+v", c)
	}
}
