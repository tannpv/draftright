package shared_test

import (
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// nodeBcryptHash is a genuine bcrypt hash of "correct horse battery staple"
// generated with bcryptjs (cost 10) in the NestJS backend:
//
//	node -e 'console.log(require("bcryptjs").hashSync("correct horse battery staple",10))'
//
// It starts with $2b$10$ confirming cost-10 MCF format compatible with
// both bcryptjs (Node) and x/crypto/bcrypt (Go).
const nodeBcryptHash = "$2b$10$gmdILy0hQ9MeL4vg9r.aGOfLKZzIzkJwh2UXFqnCEfy6ScYqJ7y2K"

// TestVerifyPassword_AcceptsNodeGeneratedHash proves that x/crypto/bcrypt can
// verify a hash written by bcryptjs — the core cross-compat requirement.
func TestVerifyPassword_AcceptsNodeGeneratedHash(t *testing.T) {
	ok, err := shared.VerifyPassword("correct horse battery staple", nodeBcryptHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected VerifyPassword to return true for correct passphrase against Node-generated hash")
	}
}

// TestVerifyPassword_RejectsWrongPassword confirms a wrong password returns
// false with no error (not an internal failure).
func TestVerifyPassword_RejectsWrongPassword(t *testing.T) {
	ok, err := shared.VerifyPassword("wrong", nodeBcryptHash)
	if err != nil {
		t.Fatalf("unexpected error on mismatch: %v", err)
	}
	if ok {
		t.Fatal("expected VerifyPassword to return false for wrong password")
	}
}

// TestHashPassword_RoundTrips hashes a password with HashPassword then verifies
// it with VerifyPassword, confirming the Go-side round-trip.
func TestHashPassword_RoundTrips(t *testing.T) {
	hash, err := shared.HashPassword("s3cret")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword returned empty string")
	}
	ok, err := shared.VerifyPassword("s3cret", hash)
	if err != nil {
		t.Fatalf("VerifyPassword returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected VerifyPassword to return true for freshly hashed password")
	}
}
