// Package auth verifies JWT bearer tokens signed by the NestJS backend.
//
// Critical compatibility note: NestJS signs HS256 tokens with the
// JWT_SECRET env var. This service MUST verify with the SAME secret —
// any drift = users randomly get 401. Rotation is dual-secret-window
// (accept old + new for 24 h) until we move to RS256 in Task 13.
//
// The Claims shape matches NestJS's @nestjs/jwt default payload:
//
//	{ "sub": "<user-uuid>", "role": "user" | "admin", "iat": …, "exp": … }
//
// Only `sub` is required; `role` defaults to "user" when absent. If
// NestJS ever adds a claim, mirror it here and bump a parity-test.
package auth

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the parsed payload. We embed jwt.RegisteredClaims for the
// standard exp/iat/nbf fields; Sub is overridden so callers get a
// concrete string instead of jwt.ClaimStrings.
type Claims struct {
	Sub  string `json:"sub"`
	Role string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

// UserID returns the parsed `sub` claim — the user UUID NestJS signs
// into every access token. Returns "" if absent.
func (c *Claims) UserID() string {
	return c.Sub
}

// IsAdmin checks the role claim. NestJS stamps "admin" on the elevated
// tokens used by the admin portal; everyone else is "user".
func (c *Claims) IsAdmin() bool {
	return c.Role == "admin"
}

// Verifier verifies HS256 tokens against a single shared secret.
// Reusable across handlers — instantiate once at app startup,
// inject into the middleware (Rule #1).
type Verifier struct {
	secret []byte
}

// NewVerifier accepts the raw secret string from config. Stored as
// []byte to avoid re-encoding on every Verify call.
func NewVerifier(secret string) *Verifier {
	return &Verifier{secret: []byte(secret)}
}

// Verify parses + validates a token string. Returns the typed claims
// on success or one of the sentinel errors below on failure.
func (v *Verifier) Verify(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, v.keyFunc)
	if err != nil {
		// Map the library's error categories onto our sentinels so
		// callers (HTTP middleware, tests) can errors.Is them without
		// importing the jwt package.
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, ErrTokenExpired
		case errors.Is(err, jwt.ErrTokenMalformed):
			return nil, ErrTokenMalformed
		case errors.Is(err, jwt.ErrTokenSignatureInvalid):
			return nil, ErrTokenSignatureInvalid
		default:
			return nil, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
		}
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, ErrTokenInvalid
	}
	if claims.Sub == "" {
		return nil, ErrMissingSubject
	}
	return claims, nil
}

// keyFunc enforces HS256 — we explicitly reject any other algorithm so
// the "alg=none" downgrade attack can never succeed.
func (v *Verifier) keyFunc(t *jwt.Token) (any, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("%w: unexpected signing method %q", ErrTokenInvalid, t.Header["alg"])
	}
	return v.secret, nil
}

// Sentinel errors — exported so middleware and tests can errors.Is
// against them without importing the jwt library directly.
var (
	ErrTokenExpired           = errors.New("auth: token expired")
	ErrTokenMalformed         = errors.New("auth: token malformed")
	ErrTokenSignatureInvalid  = errors.New("auth: token signature invalid")
	ErrTokenInvalid           = errors.New("auth: token invalid")
	ErrMissingSubject         = errors.New("auth: missing sub claim")
)
