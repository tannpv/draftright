package auth

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
