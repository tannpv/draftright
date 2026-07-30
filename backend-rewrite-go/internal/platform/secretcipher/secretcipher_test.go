package secretcipher

import "testing"

// Fixed 32-byte test key (0x07 repeated), base64 — matches the Node interop test.
const testKeyB64 = "BwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwc="

// A ciphertext produced by the NODE backend (src/common/crypto/secret-cipher.ts)
// under testKeyB64 for plaintext "s3cret-value". Freezing it here proves the Go
// backend can decrypt what Node wrote — the two share one DB (#50).
const nodeCiphertext = "enc:v1:m2Wuj1hENsi1l5siJIoZNMDAlj3hMeYvNyU9TeGxEO6+jmbncRA6Dw=="

func TestDecrypt_NodeInterop(t *testing.T) {
	t.Setenv(keyEnv, testKeyB64)
	got, err := Decrypt(nodeCiphertext)
	if err != nil {
		t.Fatalf("decrypt node ciphertext: %v", err)
	}
	if got != "s3cret-value" {
		t.Fatalf("node interop: got %q, want %q", got, "s3cret-value")
	}
}

func TestRoundTrip(t *testing.T) {
	t.Setenv(keyEnv, testKeyB64)
	for _, plain := range []string{"sk-abc123", "", "unicode ✓ résumé", "with:colons:and enc:v1:lookalike"} {
		enc, err := Encrypt(plain)
		if err != nil {
			t.Fatalf("encrypt %q: %v", plain, err)
		}
		dec, err := Decrypt(enc)
		if err != nil {
			t.Fatalf("decrypt %q: %v", plain, err)
		}
		if dec != plain {
			t.Fatalf("round trip: got %q, want %q", dec, plain)
		}
	}
}

func TestNoKey_PlaintextPassthrough(t *testing.T) {
	t.Setenv(keyEnv, "") // key unset → no-op
	enc, err := Encrypt("sk-plain")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc != "sk-plain" {
		t.Fatalf("no key must not encrypt, got %q", enc)
	}
	// Legacy plaintext read passes through.
	dec, err := Decrypt("sk-plain")
	if err != nil || dec != "sk-plain" {
		t.Fatalf("plaintext passthrough: got %q err=%v", dec, err)
	}
}

func TestDecrypt_EncryptedButNoKey_Errors(t *testing.T) {
	t.Setenv(keyEnv, "")
	if _, err := Decrypt(nodeCiphertext); err == nil {
		t.Fatal("expected error decrypting ciphertext with no key")
	}
}

func TestEncrypt_Idempotent(t *testing.T) {
	t.Setenv(keyEnv, testKeyB64)
	once, _ := Encrypt("sk-abc")
	twice, _ := Encrypt(once) // already encrypted → unchanged
	if once != twice {
		t.Fatalf("re-encrypt should be a no-op: %q vs %q", once, twice)
	}
}

func TestBadKeyLength_Errors(t *testing.T) {
	t.Setenv(keyEnv, "c2hvcnQ=") // "short" — not 32 bytes
	if _, err := Encrypt("x"); err == nil {
		t.Fatal("expected error for non-32-byte key")
	}
}

// TestGoCiphertext_ForNode prints a Go-produced ciphertext for "s3cret-value"
// under testKeyB64. Run with `-run TestGoCiphertext_ForNode -v` to regenerate
// the frozen vector used by the Node interop test. Kept as a helper, not an
// assertion (nonce is random each run).
func TestGoCiphertext_ForNode(t *testing.T) {
	t.Setenv(keyEnv, testKeyB64)
	ct, err := Encrypt("s3cret-value")
	if err != nil {
		t.Fatal(err)
	}
	// Verify it round-trips locally; the Node test asserts it decrypts there.
	if dec, _ := Decrypt(ct); dec != "s3cret-value" {
		t.Fatalf("self-decrypt mismatch: %q", dec)
	}
	t.Logf("GO_CIPHERTEXT=%s", ct)
}
