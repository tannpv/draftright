package user_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/user"
)

func sptr(s string) *string { return &s }

// TestUserDetail_JSONKeyOrder pins the 16 exposed JSON keys in
// entity-declaration order and confirms an absent nullable string renders
// explicit null. The 6 secret columns (password_hash, reset/verification
// codes) are dropped — see TestUserDetail_OmitsSecretColumns (#31).
func TestUserDetail_JSONKeyOrder(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 678_000_000, time.UTC)
	updated := time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)
	d := user.UserDetail{
		ID:           "u1",
		Email:        "a@b.com",
		PasswordHash: sptr("$2b$10$hash"),
		Name:         "Alice",
		IsActive:     true,
		Role:         "user",
		AuthProvider: "local",
		// GoogleID left nil → expect "google_id":null
		CreatedAt: created,
		UpdatedAt: updated,
	}

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	want := []string{
		"id", "email", "name", "is_active", "role",
		"auth_provider", "google_id", "facebook_id", "tiktok_id", "apple_id",
		"avatar_url", "stripe_customer_id", "email_verified",
		"lemonsqueezy_customer_id", "created_at", "updated_at",
	}
	prev := -1
	for _, key := range want {
		idx := strings.Index(got, `"`+key+`"`)
		if idx < 0 {
			t.Fatalf("missing key %q in %s", key, got)
		}
		if idx <= prev {
			t.Fatalf("key %q out of order in %s", key, got)
		}
		prev = idx
	}

	if !strings.Contains(got, `"google_id":null`) {
		t.Errorf("absent nullable should render null; got %s", got)
	}
}

// TestUserDetail_OmitsSecretColumns — #31: the admin user JSON must never
// carry password_hash, the password-reset code/expiry/attempts, or the
// email-verification code/expiry. Node drops the same six → byte parity.
func TestUserDetail_OmitsSecretColumns(t *testing.T) {
	code := "123456"
	hash := "$2b$10$abcdefghijklmnopqrstuv"
	exp := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	d := user.UserDetail{
		ID: "u1", Email: "a@b.com", PasswordHash: &hash, Name: "A",
		Role: "user", AuthProvider: "local",
		EmailVerificationCode: &code, EmailVerificationExpires: &exp,
		PasswordResetCode: &code, PasswordResetExpires: &exp, PasswordResetAttempts: 3,
		CreatedAt: exp, UpdatedAt: exp,
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	for _, k := range []string{
		"password_hash", "email_verification_code", "email_verification_expires",
		"password_reset_code", "password_reset_expires", "password_reset_attempts",
	} {
		if strings.Contains(got, k) {
			t.Errorf("response leaks %q: %s", k, got)
		}
	}
	// Non-secret columns must remain.
	for _, k := range []string{"id", "email", "name", "is_active", "auth_provider", "created_at"} {
		if !strings.Contains(got, k) {
			t.Errorf("missing expected key %q", k)
		}
	}
}
