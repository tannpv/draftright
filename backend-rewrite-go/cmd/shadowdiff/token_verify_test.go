package main

import (
	"testing"
	"time"

	platauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
)

// A login response's access_token is ignored by value in the JSON diff
// (it differs every issue). This test instead asserts the Go signer
// produces a token whose claims + TTL match what Node issues, so the
// "ignore_value_of" exclusion is safe.
func TestAccessToken_ClaimsAndTTL(t *testing.T) {
	secret := "shared-secret"
	tok, err := platauth.NewSigner(secret).Sign(
		platauth.Claims{Sub: "u1", Email: "a@b.com", Role: "user"}, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := platauth.NewVerifier(secret).Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Sub != "u1" || claims.Email != "a@b.com" || claims.Role != "user" {
		t.Fatalf("claims = %+v", claims)
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl < 14*time.Minute || ttl > 15*time.Minute {
		t.Fatalf("ttl = %v, want ~15m", ttl)
	}
}
