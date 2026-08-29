package applestore

import (
	"crypto/x509"
	_ "embed"
	"fmt"
)

// appleRootCAG3PEM is Apple's published root CA that every StoreKit JWS x5c
// chain must terminate at (Verifier.verifySignature). Embedded — not fetched
// at runtime — so verification has no network dependency and can't be
// silently redirected. See apple_root_ca_g3.pem for provenance + fingerprint;
// re-fetch and diff that file before ever rotating it.
//
//go:embed apple_root_ca_g3.pem
var appleRootCAG3PEM []byte

// DefaultRoots parses the embedded Apple Root CA G3 into a cert pool. One
// source of truth for "what root does DraftRight trust for StoreKit" — every
// caller (main.go's composition root today; a future CLI or test harness
// tomorrow) gets it from here rather than re-embedding its own copy.
func DefaultRoots() (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(appleRootCAG3PEM); !ok {
		return nil, fmt.Errorf("applestore: failed to parse embedded Apple Root CA G3 PEM")
	}
	return pool, nil
}
