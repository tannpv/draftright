package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func buildJWKS(t *testing.T, key *rsa.PrivateKey, kid string) string {
	t.Helper()
	n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
	eBytes := big.NewInt(int64(key.E)).Bytes()
	e := base64.RawURLEncoding.EncodeToString(eBytes)
	doc := map[string]any{"keys": []map[string]string{
		{"kty": "RSA", "kid": kid, "n": n, "e": e},
	}}
	b, _ := json.Marshal(doc)
	return string(b)
}

func signApple(t *testing.T, key *rsa.PrivateKey, kid, sub, email string, emailVerified bool, aud string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss":            "https://appleid.apple.com",
		"aud":            aud,
		"sub":            sub,
		"email":          email,
		"email_verified": emailVerified,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerifyApple_ValidToken(t *testing.T) {
	t.Cleanup(func() { appleCache.keys = nil })
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	kid := "testkid"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(buildJWKS(t, key, kid)))
	}))
	defer srv.Close()
	tok := signApple(t, key, kid, "applesub", "a@b.com", true, "com.draftright.app.v2")
	v := &httpVerifier{http: srv.Client(), appleKeyURL: srv.URL, appleAuds: []string{"com.draftright.app.v2"}}
	p, err := v.verifyApple(context.Background(), tok, InboundProfile{})
	if err != nil {
		t.Fatal(err)
	}
	if p.SocialID != "applesub" || p.Email != "a@b.com" || !p.EmailVerified {
		t.Fatalf("%+v", p)
	}
}

func TestVerifyApple_NoKid(t *testing.T) {
	t.Cleanup(func() { appleCache.keys = nil })
	v := &httpVerifier{}
	_, err := v.verifyApple(context.Background(), "not.a.jwt", InboundProfile{})
	assertAuthErr(t, err, "Invalid Apple token (no kid)")
}

func TestVerifyApple_WrongAudience(t *testing.T) {
	t.Cleanup(func() { appleCache.keys = nil })
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	kid := "k"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(buildJWKS(t, key, kid)))
	}))
	defer srv.Close()
	tok := signApple(t, key, kid, "s", "a@b.com", true, "com.evil.app")
	v := &httpVerifier{http: srv.Client(), appleKeyURL: srv.URL, appleAuds: []string{"com.draftright.app.v2"}}
	_, err := v.verifyApple(context.Background(), tok, InboundProfile{})
	assertAuthErr(t, err, "Invalid Apple token")
}
