package user_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/user"
)

func sptr(s string) *string { return &s }

// TestUserDetail_JSONKeyOrder pins the 22 JSON keys in entity-declaration
// order and confirms an absent nullable string renders explicit null.
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

// TestUserDetail_NullPasswordHash — the OAuth-user parity case: a nil
// PasswordHash renders explicit JSON null, not "".
func TestUserDetail_NullPasswordHash(t *testing.T) {
	d := user.UserDetail{
		ID:           "u2",
		Email:        "oauth@b.com",
		PasswordHash: nil,
		Name:         "Bob",
		Role:         "user",
		AuthProvider: "google",
		CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"password_hash":null`) {
		t.Errorf("nil password_hash must render null; got %s", got)
	}
	if strings.Contains(got, `"password_hash":""`) {
		t.Errorf("nil password_hash must not render empty string; got %s", got)
	}
}

// TestUserDetail_NullableTimestamps — a non-nil email_verification_expires
// renders as the quoted ISO-millis string; a nil one renders null.
func TestUserDetail_NullableTimestamps(t *testing.T) {
	exp := time.Date(2026, 3, 4, 5, 6, 7, 890_000_000, time.UTC)
	d := user.UserDetail{
		ID:                       "u3",
		Email:                    "c@b.com",
		Name:                     "Carol",
		Role:                     "user",
		AuthProvider:             "local",
		EmailVerificationExpires: &exp,
		// PasswordResetExpires left nil → expect null
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	if !strings.Contains(got, `"email_verification_expires":"2026-03-04T05:06:07.890Z"`) {
		t.Errorf("non-nil timestamp must render ISO-millis string; got %s", got)
	}
	if !strings.Contains(got, `"password_reset_expires":null`) {
		t.Errorf("nil timestamp must render null; got %s", got)
	}
}
