// Package exttoken issues and verifies long-lived scoped extension tokens
// (dr_ext_*) for keyboard/share extensions. Port of NestJS extension-tokens.
package exttoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

// TokenPrefix is the opaque-token marker the dual-auth middleware keys on.
// Node: const TOKEN_PREFIX = 'dr_ext_';
const TokenPrefix = "dr_ext_"

// ScopeRewrite is the only scope currently minted/required.
// Node: scopes: ['rewrite']; REQUIRED_SCOPE = 'rewrite'.
const ScopeRewrite = "rewrite"

// Sentinel errors — strings byte-compared by the shadow gate. Match Node's
// RewriteAuthGuard UnauthorizedException messages verbatim.
var (
	ErrMissingToken = errors.New("Missing bearer token")
	ErrInvalidToken = errors.New("Invalid extension token")
	ErrMissingScope = errors.New("Token missing rewrite scope")
)

// MintResult is what Mint returns to the caller (T13).
type MintResult struct {
	Token string // plaintext dr_ext_… — shown ONCE, never stored
	ID    string // row id
}

// TokenRow is a stored token projected for the list endpoint (T14).
// JSON tags pinned to Node response order/shape (token_hash and user_id are
// stripped by the controller and so are absent here).
type TokenRow struct {
	ID         string     `json:"id"`
	Scopes     []string   `json:"scopes"`
	DeviceID   string     `json:"device_id"`
	DeviceName string     `json:"device_name"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// generateToken returns a fresh plaintext token (TokenPrefix + base64url-nopad
// of 32 random bytes) and its sha256-hex hash.
//
// Node recipe (extension-token.service.ts):
//
//	const raw = crypto.randomBytes(32).toString('base64url');
//	const token = `${TOKEN_PREFIX}${raw}`;
//	const tokenHash = crypto.createHash('sha256').update(token).digest('hex');
//
// base64url == RawURLEncoding (url-safe alphabet, no padding) → 43 chars for
// 32 bytes, total token length 50. The hash is sha256-hex of the FULL
// dr_ext_-prefixed token string.
func generateToken() (plain, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	plain = TokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	return plain, hashToken(plain), nil
}

// hashToken is the at-rest representation (sha256 hex of the plaintext token,
// prefix included — must match what Node stores so tokens verify cross-stack).
func hashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
