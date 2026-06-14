package auth

import (
	"regexp"
	"strings"
)

// emailRe is a permissive RFC-ish check matching class-validator's
// @IsEmail acceptance for the inputs real clients send. Exotic-input
// parity is pinned by the shadow gate (see spec §8).
var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// validateRegister reproduces the NestJS ValidationPipe + humanizer for
// RegisterDto, in property-declaration order (email, password, name).
// Each message carries its own trailing period, so they join with a
// single space. Empty result = valid. Code invalid-input, status 400.
func validateRegister(email, password, name string) string {
	var msgs []string
	if !emailRe.MatchString(email) {
		msgs = append(msgs, "Please enter a valid email address.")
	}
	if len(password) < 8 {
		msgs = append(msgs, "Password must be at least 8 characters.")
	}
	if len(name) < 1 {
		msgs = append(msgs, "Name must be at least 1 characters.")
	}
	return strings.Join(msgs, " ")
}
