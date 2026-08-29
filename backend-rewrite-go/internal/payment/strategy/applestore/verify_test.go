package applestore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"
	"time"
)

func makeJWS(t *testing.T, key *ecdsa.PrivateKey, der []byte, payload JWSPayload) string {
	t.Helper()
	hdr := map[string]any{"alg": "ES256", "x5c": []string{base64.StdEncoding.EncodeToString(der)}}
	hb, _ := json.Marshal(hdr)
	pb, _ := json.Marshal(payload)
	seg := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(pb)
	sum := sha256Sum([]byte(seg)) // helper defined in verify.go
	r, s, err := ecdsa.Sign(rand.Reader, key, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return seg + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func selfSigned(t *testing.T) (*ecdsa.PrivateKey, []byte, *x509.CertPool) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	crt, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(crt)
	return key, der, pool
}

// selfSignedWithEKU mirrors selfSigned but restricts the cert's ExtKeyUsage to
// the given usage (e.g. CodeSigning) instead of leaving it unset/Any. Apple's
// real StoreKit leaf is not a TLS server-auth cert, so this fixture proves
// Verify accepts a leaf whose EKU is unrelated to ServerAuth — guarding the
// ExtKeyUsageAny option in verifySignature (see TestVerify_NonServerAuthLeaf).
func selfSignedWithEKU(t *testing.T, eku x509.ExtKeyUsage) (*ecdsa.PrivateKey, []byte, *x509.CertPool) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		ExtKeyUsage: []x509.ExtKeyUsage{eku}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	crt, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(crt)
	return key, der, pool
}

func TestVerify_Valid(t *testing.T) {
	key, der, pool := selfSigned(t)
	v := NewVerifier(pool, "com.draftright.app", "Sandbox")
	jws := makeJWS(t, key, der, JWSPayload{BundleID: "com.draftright.app", Environment: "Sandbox", ProductID: "p", TransactionID: "t", OriginalTransactionID: "o", ExpiresDate: 1000})
	got, err := v.Verify(jws)
	if err != nil {
		t.Fatalf("valid jws rejected: %v", err)
	}
	if got.ProductID != "p" || got.OriginalTransactionID != "o" {
		t.Fatalf("payload not decoded: %+v", got)
	}
}

func TestVerify_TamperedSig(t *testing.T) {
	key, der, pool := selfSigned(t)
	v := NewVerifier(pool, "com.draftright.app", "Sandbox")
	jws := makeJWS(t, key, der, JWSPayload{BundleID: "com.draftright.app", Environment: "Sandbox"})
	if _, err := v.Verify(jws[:len(jws)-2] + "xy"); err == nil {
		t.Fatal("tampered signature accepted")
	}
}

func TestVerify_WrongBundle(t *testing.T) {
	key, der, pool := selfSigned(t)
	v := NewVerifier(pool, "com.draftright.app", "Sandbox")
	jws := makeJWS(t, key, der, JWSPayload{BundleID: "com.evil.app", Environment: "Sandbox"})
	if _, err := v.Verify(jws); err == nil {
		t.Fatal("wrong bundleId accepted")
	}
}

func TestVerify_NonServerAuthLeaf(t *testing.T) {
	// A leaf with an unrelated EKU must still verify (Apple's leaf isn't serverAuth).
	key, der, pool := selfSignedWithEKU(t, x509.ExtKeyUsageCodeSigning)
	v := NewVerifier(pool, "com.draftright.app", "Sandbox")
	jws := makeJWS(t, key, der, JWSPayload{BundleID: "com.draftright.app", Environment: "Sandbox"})
	if _, err := v.Verify(jws); err != nil {
		t.Fatalf("non-serverAuth leaf rejected: %v", err)
	}
}
