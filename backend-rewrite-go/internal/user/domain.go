// Package user owns the customer identity record. Phase 1a exposes
// reads + password update + account deletion; register/verification
// land in Phase 1b.
package user

// User is the domain projection used by auth. PasswordHash is empty for
// social-only accounts (no local password).
type User struct {
	ID                   string
	Email                string
	PasswordHash         string
	Name                 string
	IsActive             bool
	Role                 string
	AuthProvider         string // local | google | facebook | tiktok
	EmailVerified        bool
	LemonsqueezyCustomer string // "" when none
}
