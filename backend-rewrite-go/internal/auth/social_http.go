package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
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
	googleAuds  []string
}

// Shipped Google OAuth client ids — the `aud` values the real apps mint tokens
// for. These are PUBLIC values embedded in the clients themselves; the external
// spec is the per-platform Google client config. This list MUST include EVERY
// shipped app's Google client id — a platform whose id is missing fails
// verifyGoogle with "Invalid Google token" (that was Windows, #206). Add the
// new id here whenever a platform's Google OAuth client is created.
const googleDefaultAuds = "22951518033-gf853ftmf4emivffk0su2bik42j7cmai.apps.googleusercontent.com," + // web + Flutter mobile (also the app_settings default)
	"22951518033-dvkn61dhibse9fu83ohh51mlovd7269a.apps.googleusercontent.com," + // macOS — iOS-type client
	"22951518033-oaf0ptahsjrsnu2v2qr0kpul5tslpgf6.apps.googleusercontent.com," + // Linux — Desktop-app client
	"22951518033-oq7okrvvbb26eqsb7c0avsb1ic165ole.apps.googleusercontent.com" // Windows — Desktop-app client

// Compile-time assertion: httpVerifier satisfies SocialVerifier.
var _ SocialVerifier = (*httpVerifier)(nil)

// NewHTTPSocialVerifier builds the production verifier with Google/Facebook/
// Apple endpoints. appleAudsCSV is a comma-separated list of accepted Apple
// audiences; empty falls back to appleDefaultAuds.
func NewHTTPSocialVerifier(appleAudsCSV, googleAudsCSV string) *httpVerifier {
	auds := splitAuds(appleAudsCSV)
	if len(auds) == 0 {
		auds = splitAuds(appleDefaultAuds)
	}
	gauds := splitAuds(googleAudsCSV)
	if len(gauds) == 0 {
		gauds = splitAuds(googleDefaultAuds)
	}
	return &httpVerifier{
		http:        &http.Client{Timeout: 10 * time.Second},
		googleURL:   "https://oauth2.googleapis.com/tokeninfo?id_token=",
		facebookURL: "https://graph.facebook.com/me?fields=id,name,email,picture.type(large)&access_token=",
		appleKeyURL: "https://appleid.apple.com/auth/keys",
		appleAuds:   auds,
		googleAuds:  gauds,
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
		Aud           string `json:"aud"`
		Iss           string `json:"iss"`
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		EmailVerified any    `json:"email_verified"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&d)

	// tokeninfo only proves Google minted the token — NOT that it was minted
	// for us. Without this check an id_token issued to any other OAuth client
	// (any app the user has ever signed into with Google) can be replayed here
	// to take over that user's account. Mirrors the NestJS verifyGoogleToken
	// aud/iss/sub checks byte-for-byte, including the error messages.
	if len(v.googleAuds) == 0 {
		// Fail closed: an empty allow-list must never mean "allow everything".
		return SocialProfile{}, unauthorized("Google sign-in is not configured")
	}
	if d.Aud == "" || !slices.Contains(v.googleAuds, d.Aud) {
		return SocialProfile{}, unauthorized("Invalid Google token")
	}
	// Guard the issuer too; tokeninfo echoes it and both spellings are valid.
	if d.Iss != "" && d.Iss != "accounts.google.com" && d.Iss != "https://accounts.google.com" {
		return SocialProfile{}, unauthorized("Invalid Google token")
	}
	if d.Sub == "" {
		return SocialProfile{}, unauthorized("Google token missing sub claim")
	}

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
