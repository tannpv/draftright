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

// Phase 1b lifecycle/social messages (register, verify-email, reset,
// social link). Byte-for-byte parity with the NestJS auth.service.
const (
	msgEmailRegistered       = "Email already registered"
	msgInvalidVerifyCode     = "Invalid or expired verification code"
	msgInvalidResetCode      = "Invalid or expired reset code"
	msgPasswordTooShort      = "Password must be at least 8 characters"
	msgEmailRequired         = "Email is required for social login"
	msgEmailRegisteredSocial = "This email is registered. Sign in with your password to link this account."
)

// BadRequestError → 400, code "invalid-input". Carries the exact Node message.
type BadRequestError struct{ Message string }

func (e *BadRequestError) Error() string { return e.Message }

func badRequest(msg string) *BadRequestError { return &BadRequestError{Message: msg} }

// ConflictError → 409, code "conflict".
type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

func conflict(msg string) *ConflictError { return &ConflictError{Message: msg} }

// providerLabel maps the auth_provider enum to the human label Node uses
// in the social-account login message. Returns "" for local/unknown
// (caller falls back to msgSocialGeneric).
func providerLabel(provider string) string {
	switch provider {
	case "google":
		return "Google"
	case "facebook":
		return "Facebook"
	case "tiktok":
		return "TikTok"
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
