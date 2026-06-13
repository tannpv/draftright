package auth

// Exact UnauthorizedException messages from the NestJS auth.service —
// shadow-compare diffs the response `error` string, so these must match
// byte-for-byte.
const (
	msgInvalidCredentials = "Invalid credentials"
	msgAccountDisabled    = "Account disabled"
	msgSocialGeneric      = "This account uses a social sign-in. Use the provider you signed up with."
	msgCurrentPwWrong     = "Current password is incorrect"
	msgInvalidRefresh     = "Invalid refresh token"
)

// providerLabel maps the auth_provider enum to the human label Node uses
// in the social-account login message. Returns "" for local/unknown
// (caller falls back to msgSocialGeneric).
func providerLabel(provider string) string {
	switch provider {
	case "google":
		return "Google"
	case "facebook":
		return "Facebook"
	case "apple":
		return "Apple"
	default:
		return ""
	}
}

// socialLoginMsg builds the friendly "use the X button" message, or the
// generic fallback when there's no label.
func socialLoginMsg(provider string) string {
	if l := providerLabel(provider); l != "" {
		return "This account was created with " + l + ". Use the " + l + " button to sign in."
	}
	return msgSocialGeneric
}
