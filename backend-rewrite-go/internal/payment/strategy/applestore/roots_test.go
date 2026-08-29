package applestore

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"testing"
)

// appleRootCAG3SHA256Fingerprint is Apple's published SHA-256 fingerprint for
// Apple Root CA - G3 (https://www.apple.com/certificateauthority/). Pinned
// here so a future edit to apple_root_ca_g3.pem that silently swaps in a
// different (or corrupt) cert fails this test instead of shipping unnoticed.
const appleRootCAG3SHA256Fingerprint = "63343abfb89a6a03ebb57e9b3f5fa7be7c4f5c756f3017b3a8c488c3653e9179"

func TestDefaultRoots_ParsesEmbeddedPEM(t *testing.T) {
	pool, err := DefaultRoots()
	if err != nil {
		t.Fatalf("DefaultRoots: %v", err)
	}
	if pool == nil {
		t.Fatal("DefaultRoots returned a nil pool with no error")
	}
	if len(pool.Subjects()) != 1 { //nolint:staticcheck // Subjects() is deprecated but adequate for a one-cert embedded pool in a test.
		t.Fatalf("pool has %d subjects, want 1 (Apple Root CA G3)", len(pool.Subjects()))
	}
}

func TestDefaultRoots_MatchesPublishedAppleFingerprint(t *testing.T) {
	blk, _ := pem.Decode(appleRootCAG3PEM)
	if blk == nil {
		t.Fatal("embedded apple_root_ca_g3.pem contains no PEM block")
	}
	crt, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		t.Fatalf("parse embedded cert: %v", err)
	}
	sum := sha256.Sum256(crt.Raw)
	got := hex.EncodeToString(sum[:])
	if got != appleRootCAG3SHA256Fingerprint {
		t.Fatalf("fingerprint = %s, want %s (embedded cert is not the real Apple Root CA G3 — apple_root_ca_g3.pem was edited to something other than the pinned cert)", got, appleRootCAG3SHA256Fingerprint)
	}
	if crt.Subject.CommonName != "Apple Root CA - G3" {
		t.Fatalf("subject CN = %q, want %q", crt.Subject.CommonName, "Apple Root CA - G3")
	}
}
