package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// appleJWKSCache is a process-wide 24h cache mirroring Node's static
// _appleJwksCache. Guarded by mu.
type appleJWKSCache struct {
	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey // kid → key
	expiresAt time.Time
}

var appleCache = &appleJWKSCache{}

const appleDefaultAuds = "com.draftright.draftrightMobile.v2,com.draftright.app.v2"

func (v *httpVerifier) verifyApple(ctx context.Context, idToken string, profile InboundProfile) (SocialProfile, error) {
	kid, err := appleKidOf(idToken)
	if err != nil || kid == "" {
		return SocialProfile{}, unauthorized("Invalid Apple token (no kid)")
	}
	pub, err := v.applePublicKey(ctx, kid)
	if err != nil {
		return SocialProfile{}, err // already an *AuthError
	}
	auds := v.appleAuds
	if len(auds) == 0 {
		auds = splitAuds(appleDefaultAuds)
	}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer("https://appleid.apple.com"),
		// clockTolerance stays ZERO (parity invariant) — do NOT add jwt.WithLeeway.
	)
	claims := jwt.MapClaims{}
	_, err = parser.ParseWithClaims(idToken, claims, func(*jwt.Token) (any, error) { return pub, nil })
	if err != nil {
		return SocialProfile{}, unauthorized("Invalid Apple token")
	}
	if !audienceMatches(claims, auds) {
		return SocialProfile{}, unauthorized("Invalid Apple token")
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return SocialProfile{}, unauthorized("Apple token missing sub claim")
	}
	email, _ := claims["email"].(string)
	if email == "" {
		email = profile.Email
	}
	verified := email != "" && truthy(claims["email_verified"]) && hasOwnEmail(claims)
	return SocialProfile{
		SocialID: sub, Email: email, Name: profile.Name, AvatarURL: profile.AvatarURL,
		EmailVerified: verified,
	}, nil
}

// hasOwnEmail mirrors Node `!!payload.email` — only the TOKEN's email
// counts toward verified, not the profile fallback.
func hasOwnEmail(c jwt.MapClaims) bool { e, _ := c["email"].(string); return e != "" }

func (v *httpVerifier) applePublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	appleCache.mu.Lock()
	defer appleCache.mu.Unlock()
	if appleCache.keys == nil || appleCache.expiresAt.Before(time.Now()) {
		if err := v.fetchAppleKeys(ctx); err != nil {
			return nil, unauthorized("Could not fetch Apple signing keys")
		}
	}
	pub, ok := appleCache.keys[kid]
	if !ok {
		appleCache.keys = nil // bust cache (matches Node) — no retry, just throw
		return nil, unauthorized("Apple key not found for kid=" + kid)
	}
	return pub, nil
}

func (v *httpVerifier) fetchAppleKeys(ctx context.Context) error {
	client := v.http
	if client == nil {
		client = http.DefaultClient
	}
	url := v.appleKeyURL
	if url == "" {
		url = "https://appleid.apple.com/auth/keys"
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return errors.New("apple keys http error")
	}
	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		nb, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eb, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(nb),
			E: int(new(big.Int).SetBytes(eb).Int64()),
		}
	}
	appleCache.keys = keys
	appleCache.expiresAt = time.Now().Add(24 * time.Hour)
	return nil
}

// appleKidOf extracts the `kid` from a JWT header WITHOUT verifying.
func appleKidOf(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", errors.New("not a jwt")
	}
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	var h struct {
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(hb, &h); err != nil {
		return "", err
	}
	return h.Kid, nil
}

// splitAuds comma-splits a CSV, trims spaces, drops empties.
func splitAuds(csv string) []string {
	var out []string
	for _, p := range strings.Split(csv, ",") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// audienceMatches reads the `aud` claim (string OR []any) and returns true
// if it intersects auds.
func audienceMatches(claims jwt.MapClaims, auds []string) bool {
	want := map[string]bool{}
	for _, a := range auds {
		want[a] = true
	}
	switch t := claims["aud"].(type) {
	case string:
		return want[t]
	case []any:
		for _, a := range t {
			if s, ok := a.(string); ok && want[s] {
				return true
			}
		}
	}
	return false
}
