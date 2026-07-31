// Package secretcipher provides at-rest envelope encryption for secret columns
// (ai_providers.api_key, app_settings payment/SMTP secrets). Issue #50.
//
// Format: "enc:v1:<base64(nonce[12] || ciphertext || gcmTag[16])>", AES-256-GCM,
// no AAD. The "enc:v1:" prefix makes reads transparent to legacy plaintext rows
// and MUST stay byte-identical to the Node backend's src/common/crypto (they
// share one DB, so either backend must decrypt the other's ciphertext).
//
// Key-gated: with SECRETS_ENCRYPTION_KEY unset, Encrypt is a no-op (returns
// plaintext) and Decrypt passes plaintext through — deploying this changes
// nothing until the key is provisioned and the migration runs.
package secretcipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	prefix   = "enc:v1:"
	keyEnv   = "SECRETS_ENCRYPTION_KEY"
	tagBytes = 16
)

// loadKey returns the 32-byte AES key, or (nil, nil) when no key is configured.
func loadKey() ([]byte, error) {
	raw := os.Getenv(keyEnv)
	if raw == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid base64: %w", keyEnv, err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("%s must be base64 of exactly 32 bytes (got %d)", keyEnv, len(key))
	}
	return key, nil
}

// IsEncrypted reports whether v is ciphertext this package produced.
func IsEncrypted(v string) bool {
	return strings.HasPrefix(v, prefix)
}

// Encrypt encrypts a secret for storage. No-op (returns input) when no key is
// set or the value is empty / already encrypted, so callers can encrypt
// idempotently.
func Encrypt(plain string) (string, error) {
	key, err := loadKey()
	if err != nil {
		return "", err
	}
	if key == nil || plain == "" || IsEncrypted(plain) {
		return plain, nil
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	// Seal appends the tag: out = ciphertext || tag.
	out := gcm.Seal(nil, nonce, []byte(plain), nil)
	return prefix + base64.StdEncoding.EncodeToString(append(nonce, out...)), nil
}

// Decrypt decrypts a stored secret. Plaintext (no prefix) passes through.
// Returns an error if ciphertext is present but no key is set, or on tamper.
func Decrypt(stored string) (string, error) {
	if !IsEncrypted(stored) {
		return stored, nil
	}
	key, err := loadKey()
	if err != nil {
		return "", err
	}
	if key == nil {
		return "", fmt.Errorf("encrypted secret present but %s is not set", keyEnv)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, prefix))
	if err != nil {
		return "", fmt.Errorf("secretcipher: bad base64: %w", err)
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns+tagBytes {
		return "", errors.New("secretcipher: malformed encrypted secret")
	}
	nonce, ct := raw[:ns], raw[ns:]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("secretcipher: decrypt failed: %w", err)
	}
	return string(pt), nil
}

// EncryptInPlace encrypts each non-nil target in place, stopping on the first
// error. Convenience for wiring the many secret fields of a settings struct.
func EncryptInPlace(targets ...*string) error {
	for _, t := range targets {
		if t == nil {
			continue
		}
		v, err := Encrypt(*t)
		if err != nil {
			return err
		}
		*t = v
	}
	return nil
}

// DecryptInPlace decrypts each non-nil target in place, stopping on the first
// error.
func DecryptInPlace(targets ...*string) error {
	for _, t := range targets {
		if t == nil {
			continue
		}
		v, err := Decrypt(*t)
		if err != nil {
			return err
		}
		*t = v
	}
	return nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
