package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	auth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
)

// signNestJSStyle produces a token shaped exactly like the ones
// NestJS issues, so the Go verifier is exercised against the same
// payload it will see in prod (Rule #1 — parity over duplication).
func signNestJSStyle(t *testing.T, secret string, sub, role string, exp time.Duration) string {
	t.Helper()
	claims := auth.Claims{
		Sub:  sub,
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(exp)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

func TestVerifier_AcceptsValidNestJSToken(t *testing.T) {
	secret := "test-secret-shared-with-nestjs"
	v := auth.NewVerifier(secret)

	signed := signNestJSStyle(t, secret, "f47ac10b-58cc-4372-a567-0e02b2c3d479", "user", time.Hour)

	claims, err := v.Verify(signed)
	require.NoError(t, err)
	require.Equal(t, "f47ac10b-58cc-4372-a567-0e02b2c3d479", claims.UserID())
	require.False(t, claims.IsAdmin())
}

func TestVerifier_AdminRoleClaimSurfaced(t *testing.T) {
	secret := "test-secret"
	v := auth.NewVerifier(secret)

	signed := signNestJSStyle(t, secret, "admin-uuid", "admin", time.Hour)

	claims, err := v.Verify(signed)
	require.NoError(t, err)
	require.True(t, claims.IsAdmin(), "role='admin' should surface as IsAdmin()")
}

func TestVerifier_RejectsWrongSignature(t *testing.T) {
	signed := signNestJSStyle(t, "the-correct-secret", "user", "user", time.Hour)
	v := auth.NewVerifier("WRONG-SECRET")

	_, err := v.Verify(signed)
	require.Error(t, err)
	require.True(t, errors.Is(err, auth.ErrTokenSignatureInvalid),
		"want ErrTokenSignatureInvalid, got %v", err)
}

func TestVerifier_RejectsExpiredToken(t *testing.T) {
	secret := "s"
	signed := signNestJSStyle(t, secret, "user", "user", -time.Minute) // already expired
	v := auth.NewVerifier(secret)

	_, err := v.Verify(signed)
	require.Error(t, err)
	require.True(t, errors.Is(err, auth.ErrTokenExpired),
		"want ErrTokenExpired, got %v", err)
}

func TestVerifier_RejectsMalformedToken(t *testing.T) {
	v := auth.NewVerifier("s")

	_, err := v.Verify("definitely-not-a-jwt")
	require.Error(t, err)
	require.True(t, errors.Is(err, auth.ErrTokenMalformed),
		"want ErrTokenMalformed, got %v", err)
}

func TestVerifier_RejectsAlgNoneDowngrade(t *testing.T) {
	// "alg":"none" historic downgrade attack — library default already
	// blocks it, but lock the behaviour in with an explicit test so a
	// future careless config change can't re-open the hole.
	t.Parallel()
	tokenUnsigned := jwt.NewWithClaims(jwt.SigningMethodNone, &auth.Claims{Sub: "x"})
	signed, _ := tokenUnsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)

	v := auth.NewVerifier("any-secret")
	_, err := v.Verify(signed)
	require.Error(t, err)
}

func TestVerifier_RejectsMissingSubject(t *testing.T) {
	secret := "s"
	// Sign a claim with empty sub.
	claims := auth.Claims{
		Sub: "",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	signed, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))

	_, err := auth.NewVerifier(secret).Verify(signed)
	require.Error(t, err)
	require.True(t, errors.Is(err, auth.ErrMissingSubject),
		"want ErrMissingSubject, got %v", err)
}
