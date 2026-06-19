package auth

import "strings"

// InboundProfile is the client-supplied social profile hints.
type InboundProfile struct {
	Name      string
	Email     string
	AvatarURL string
}

// SocialProfile is the verified identity a provider asserts.
type SocialProfile struct {
	SocialID      string
	Email         string
	Name          string
	AvatarURL     string
	EmailVerified bool
}

// toAuthProvider maps the raw provider string (any case) to the canonical
// enum value, or a *BadRequestError mirroring Node's
// `Unsupported provider: ${provider}` (uses the RAW, pre-lowercase arg).
func toAuthProvider(raw string) (string, error) {
	switch strings.ToLower(raw) {
	case "google":
		return "google", nil
	case "facebook":
		return "facebook", nil
	case "tiktok":
		return "tiktok", nil
	case "apple":
		return "apple", nil
	default:
		return "", badRequest("Unsupported provider: " + raw)
	}
}
