package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// httpVerifier verifies Google/Facebook/Apple tokens via real HTTP. URLs
// are fields so tests inject httptest servers. Apple JWKS lives in
// social_apple.go (Task B5).
type httpVerifier struct {
	http        *http.Client
	googleURL   string // "...tokeninfo?id_token="
	facebookURL string // "...me?fields=...&access_token="
	appleKeyURL string
	appleAuds   []string
}

// Compile-time assertion: httpVerifier satisfies SocialVerifier.
var _ SocialVerifier = (*httpVerifier)(nil)

// NewHTTPSocialVerifier builds the production verifier with Google/Facebook/
// Apple endpoints. appleAudsCSV is a comma-separated list of accepted Apple
// audiences; empty falls back to appleDefaultAuds.
func NewHTTPSocialVerifier(appleAudsCSV string) *httpVerifier {
	auds := splitAuds(appleAudsCSV)
	if len(auds) == 0 {
		auds = splitAuds(appleDefaultAuds)
	}
	return &httpVerifier{
		http:        &http.Client{Timeout: 10 * time.Second},
		googleURL:   "https://oauth2.googleapis.com/tokeninfo?id_token=",
		facebookURL: "https://graph.facebook.com/me?fields=id,name,email,picture.type(large)&access_token=",
		appleKeyURL: "https://appleid.apple.com/auth/keys",
		appleAuds:   auds,
	}
}

func (v *httpVerifier) verifyGoogle(ctx context.Context, idToken string) (SocialProfile, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, v.googleURL+idToken, nil)
	resp, err := v.http.Do(req)
	if err != nil {
		return SocialProfile{}, unauthorized("Invalid Google token")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return SocialProfile{}, unauthorized("Invalid Google token")
	}
	var d struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		EmailVerified any    `json:"email_verified"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&d)
	return SocialProfile{
		SocialID: d.Sub, Email: d.Email, Name: d.Name, AvatarURL: d.Picture,
		EmailVerified: truthy(d.EmailVerified),
	}, nil
}

func (v *httpVerifier) verifyFacebook(ctx context.Context, accessToken string) (SocialProfile, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, v.facebookURL+accessToken, nil)
	resp, err := v.http.Do(req)
	if err != nil {
		return SocialProfile{}, unauthorized("Invalid Facebook token")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return SocialProfile{}, unauthorized("Invalid Facebook token")
	}
	var d struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&d)
	return SocialProfile{
		SocialID: d.ID, Email: d.Email, Name: d.Name, AvatarURL: d.Picture.Data.URL,
		EmailVerified: d.Email != "",
	}, nil
}

// Verify implements SocialVerifier. provider is the canonical enum value.
func (v *httpVerifier) Verify(ctx context.Context, provider, idToken string, profile InboundProfile) (SocialProfile, error) {
	switch provider {
	case "google":
		return v.verifyGoogle(ctx, idToken)
	case "facebook":
		return v.verifyFacebook(ctx, idToken)
	case "apple":
		return v.verifyApple(ctx, idToken, profile)
	case "tiktok":
		return SocialProfile{
			SocialID: idToken, Email: profile.Email, Name: profile.Name,
			AvatarURL: profile.AvatarURL, EmailVerified: false,
		}, nil
	}
	return SocialProfile{}, badRequest("Unsupported provider")
}

// truthy matches Node's `x === true || x === 'true'` (tokeninfo returns
// the string "true"; some payloads a bool).
func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true"
	}
	return false
}
