package user_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/user"
)

func sptr(s string) *string { return &s }

// TestUserDetail_JSONKeyOrder pins the 22 RAW JSON keys in
// entity-declaration order and confirms an absent nullable string renders
// explicit null. UserDetail is the un-sanitised entity — it KEEPS the six
// secrets because the admin payment rows nest it raw (leftJoinAndSelect,
// no sanitiser), matching Node. The user detail/update endpoints serialise
// via StrippedUserDetail instead — see TestStrippedUserDetail_OmitsSecretColumns (#31).
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
		"id", "email", "password_hash", "name", "is_active", "role",
		"auth_provider", "google_id", "facebook_id", "tiktok_id", "apple_id",
		"avatar_url", "stripe_customer_id", "email_verified",
		"email_verification_code", "email_verification_expires",
		"password_reset_code", "password_reset_expires",
		"password_reset_attempts", "lemonsqueezy_customer_id",
		"created_at", "updated_at",
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

// TestUserDetail_KeepsSecretColumns — the raw entity MUST still expose the
// six secrets. The admin payment rows nest *UserDetail directly (Node's
// leftJoinAndSelect with no ClassSerializerInterceptor leaks them there), so
// dropping them here would break byte parity at /admin/payments.
func TestUserDetail_KeepsSecretColumns(t *testing.T) {
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
		if !strings.Contains(got, k) {
			t.Errorf("raw entity must keep %q (payment rows leak it): %s", k, got)
		}
	}
}

// TestStrippedUserDetail_OmitsSecretColumns — #31: GET /admin/users/:id and
// PATCH /admin/users/:id serialise via StrippedUserDetail, which must never
// carry password_hash, the password-reset code/expiry/attempts, or the
// email-verification code/expiry. Node's stripUserSecrets drops the same six
// → byte parity.
func TestStrippedUserDetail_OmitsSecretColumns(t *testing.T) {
	code := "123456"
	hash := "$2b$10$abcdefghijklmnopqrstuv"
	exp := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	d := user.StrippedUserDetail{UserDetail: user.UserDetail{
		ID: "u1", Email: "a@b.com", PasswordHash: &hash, Name: "A",
		Role: "user", AuthProvider: "local",
		EmailVerificationCode: &code, EmailVerificationExpires: &exp,
		PasswordResetCode: &code, PasswordResetExpires: &exp, PasswordResetAttempts: 3,
		CreatedAt: exp, UpdatedAt: exp,
	}}
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
			t.Errorf("stripped response leaks %q: %s", k, got)
		}
	}
	// Non-secret columns must remain, in order.
	want := []string{
		"id", "email", "name", "is_active", "role",
		"auth_provider", "google_id", "facebook_id", "tiktok_id", "apple_id",
		"avatar_url", "stripe_customer_id", "email_verified",
		"lemonsqueezy_customer_id", "created_at", "updated_at",
	}
	prev := -1
	for _, k := range want {
		idx := strings.Index(got, `"`+k+`"`)
		if idx < 0 {
			t.Errorf("missing expected key %q", k)
			continue
		}
		if idx <= prev {
			t.Errorf("key %q out of order in %s", k, got)
		}
		prev = idx
	}
}
