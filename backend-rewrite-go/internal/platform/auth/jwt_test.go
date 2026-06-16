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

func TestVerify_ParsesEmailClaim(t *testing.T) {
	secret := "test-secret-at-least-16-chars"
	signer := auth.NewSigner(secret)
	tok, err := signer.Sign(auth.Claims{Sub: "u-1", Email: "a@b.com", Role: "user"}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := auth.NewVerifier(secret).Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Email != "a@b.com" {
		t.Fatalf("email = %q, want a@b.com", claims.Email)
	}
	if claims.Sub != "u-1" || claims.Role != "user" {
		t.Fatalf("claims = %+v, want sub=u-1 role=user", claims)
	}
}

func TestSign_PayloadMatchesNodeShape(t *testing.T) {
	// Node signs { sub, email, role, iat, exp } with HS256. Assert the
	// Go-issued token decodes to exactly those claim keys so a Node
	// verifier (and our own) accept it interchangeably.
	secret := "test-secret-at-least-16-chars"
	tok, err := auth.NewSigner(secret).Sign(auth.Claims{Sub: "u-9", Email: "x@y.z", Role: "admin"}, time.Minute)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	parsed, _ := jwt.Parse(tok, func(*jwt.Token) (any, error) { return []byte(secret), nil })
	m, _ := parsed.Claims.(jwt.MapClaims)
	for _, k := range []string{"sub", "email", "role", "iat", "exp"} {
		if _, ok := m[k]; !ok {
			t.Errorf("issued token missing claim %q", k)
		}
	}
	if m["sub"] != "u-9" || m["email"] != "x@y.z" || m["role"] != "admin" {
		t.Errorf("claim values = %v, want sub=u-9 email=x@y.z role=admin", m)
	}
}

func TestVerify_DecodesIsAdminFlag(t *testing.T) {
	const secret = "test-secret-at-least-16-chars-long"
	signer := auth.NewSigner(secret)
	tok, err := signer.Sign(auth.Claims{Sub: "admin-1", Email: "a@b.c", Role: "admin", IsAdminFlag: true}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := auth.NewVerifier(secret).Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !claims.IsAdminFlag {
		t.Errorf("IsAdminFlag = false, want true (isAdmin claim must round-trip)")
	}
}

func TestVerify_IsAdminFlagDefaultsFalse(t *testing.T) {
	const secret = "test-secret-at-least-16-chars-long"
	tok, _ := auth.NewSigner(secret).Sign(auth.Claims{Sub: "user-1", Role: "user"}, time.Hour)
	claims, err := auth.NewVerifier(secret).Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.IsAdminFlag {
		t.Errorf("IsAdminFlag = true, want false for a customer token")
	}
}
