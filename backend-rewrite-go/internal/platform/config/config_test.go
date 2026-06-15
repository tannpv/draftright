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

func TestPaymentEnv(t *testing.T) {
	os.Setenv("JWT_SECRET", "s")
	os.Setenv("WEBSITE_URL", "https://draftright.info")
	os.Setenv("STRIPE_SECRET_KEY", "sk_test_x")
	os.Setenv("STRIPE_PUBLISHABLE_KEY", "pk_test_x")
	os.Setenv("APPLE_PAY_MERCHANT_ID", "merchant.info.draftright")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("WEBSITE_URL")
		os.Unsetenv("STRIPE_SECRET_KEY")
		os.Unsetenv("STRIPE_PUBLISHABLE_KEY")
		os.Unsetenv("APPLE_PAY_MERCHANT_ID")
	}()
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.WebsiteURL != "https://draftright.info" {
		t.Fatalf("WebsiteURL=%q", c.WebsiteURL)
	}
	if c.StripeSecretKey != "sk_test_x" || c.StripePublishableKey != "pk_test_x" || c.ApplePayMerchantID != "merchant.info.draftright" {
		t.Fatalf("stripe/apple env not wired: %+v", c)
	}
}

func TestWebsiteURLDefault(t *testing.T) {
	os.Setenv("JWT_SECRET", "s")
	os.Unsetenv("WEBSITE_URL")
	defer os.Unsetenv("JWT_SECRET")
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := c.WebsiteURL; got != "http://localhost:4000" {
		t.Fatalf("default WebsiteURL=%q want http://localhost:4000", got)
	}
}
