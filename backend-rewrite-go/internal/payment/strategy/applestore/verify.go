package applestore

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

type JWSPayload struct {
	BundleID              string `json:"bundleId"`
	Environment           string `json:"environment"`
	ProductID             string `json:"productId"`
	TransactionID         string `json:"transactionId"`
	OriginalTransactionID string `json:"originalTransactionId"`
	ExpiresDate           int64  `json:"expiresDate"`
}

type Verifier struct {
	roots    *x509.CertPool
	bundleID string
	wantEnv  string
}

func NewVerifier(roots *x509.CertPool, bundleID, environment string) *Verifier {
	return &Verifier{roots: roots, bundleID: bundleID, wantEnv: environment}
}

func sha256Sum(b []byte) [32]byte { return sha256.Sum256(b) }

// verifySignature checks the x5c chain to the configured root + the ES256
// signature, and returns the RAW decoded payload bytes. It does NOT check any
// claims — the outer ASSN V2 envelope has no top-level bundleId/environment, so
// claim checks belong only to the inner transaction JWS (see Verify).
func (v *Verifier) verifySignature(jws string) ([]byte, error) {
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		return nil, errors.New("jws: want 3 segments")
	}
	hdrBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("jws header: %w", err)
	}
	var hdr struct {
		Alg string   `json:"alg"`
		X5c []string `json:"x5c"`
	}
	if err := json.Unmarshal(hdrBytes, &hdr); err != nil {
		return nil, fmt.Errorf("jws header json: %w", err)
	}
	if hdr.Alg != "ES256" || len(hdr.X5c) == 0 {
		return nil, errors.New("jws: expect ES256 with x5c")
	}
	var chain []*x509.Certificate
	for _, b64 := range hdr.X5c {
		der, err := base64.StdEncoding.DecodeString(b64) // x5c is standard base64 (RFC 7515)
		if err != nil {
			return nil, fmt.Errorf("x5c decode: %w", err)
		}
		crt, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("x5c parse: %w", err)
		}
		chain = append(chain, crt)
	}
	leaf := chain[0]
	inter := x509.NewCertPool()
	for _, c := range chain[1:] {
		inter.AddCert(c)
	}
	// KeyUsages: ExtKeyUsageAny — Apple's StoreKit leaf is NOT a TLS server-auth
	// cert (it carries Apple's OID 1.2.840.113635.100.6.11.1). Leaving KeyUsages
	// empty defaults to ServerAuth and REJECTS the real Apple chain (review M1).
	// Validity dates are still checked by x509.Verify. OCSP/revocation and the
	// Apple leaf OID are intentionally NOT checked — chain-to-Apple-root is the gate.
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         v.roots,
		Intermediates: inter,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return nil, fmt.Errorf("x5c chain: %w", err)
	}
	pub, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("leaf key not ecdsa")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		return nil, errors.New("jws signature format")
	}
	sum := sha256Sum([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(pub, sum[:], r, s) {
		return nil, errors.New("jws signature invalid")
	}
	payBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("jws payload: %w", err)
	}
	return payBytes, nil
}

// Verify checks signature AND the bundleId/environment claims — use for the
// inner transaction JWS. Returns the decoded transaction payload.
func (v *Verifier) Verify(jws string) (JWSPayload, error) {
	payBytes, err := v.verifySignature(jws)
	if err != nil {
		return JWSPayload{}, err
	}
	var p JWSPayload
	if err := json.Unmarshal(payBytes, &p); err != nil {
		return JWSPayload{}, fmt.Errorf("jws payload json: %w", err)
	}
	if p.BundleID != v.bundleID {
		return JWSPayload{}, fmt.Errorf("bundleId %q != %q", p.BundleID, v.bundleID)
	}
	if v.wantEnv != "" && p.Environment != v.wantEnv {
		return JWSPayload{}, fmt.Errorf("environment %q != %q", p.Environment, v.wantEnv)
	}
	return p, nil
}

// VerifyEnvelope checks the signature of an ASSN V2 outer JWS (no claim checks)
// and returns its raw payload for the notification envelope decode.
func (v *Verifier) VerifyEnvelope(jws string) ([]byte, error) {
	return v.verifySignature(jws)
}
