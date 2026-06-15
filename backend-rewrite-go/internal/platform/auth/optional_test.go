package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOptionalUserID_NilVerifier_Empty(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("Authorization", "Bearer something")
	if got := OptionalUserID(nil, r); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestOptionalUserID_NoHeader_Empty(t *testing.T) {
	v := NewVerifier("secret")
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	if got := OptionalUserID(v, r); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestOptionalUserID_NonBearer_Empty(t *testing.T) {
	v := NewVerifier("secret")
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("Authorization", "Token abc")
	if got := OptionalUserID(v, r); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestOptionalUserID_MalformedToken_Empty(t *testing.T) {
	v := NewVerifier("secret")
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("Authorization", "Bearer garbage")
	if got := OptionalUserID(v, r); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestOptionalUserID_ValidToken_ReturnsSub(t *testing.T) {
	const secret = "test-secret-shared-with-nestjs"
	signed, err := NewSigner(secret).Sign(Claims{Sub: "user-123", Role: "user"}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	v := NewVerifier(secret)
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("Authorization", "Bearer "+signed)
	if got := OptionalUserID(v, r); got != "user-123" {
		t.Errorf("got %q, want %q", got, "user-123")
	}
}
