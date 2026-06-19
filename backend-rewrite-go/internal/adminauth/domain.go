// Package adminauth ports the NestJS AdminAuthService (admin/auth) — login,
// change-password, getProfile — as a byte-identical drop-in. Admin tokens are
// signed with the SAME JWT_SECRET as customer access tokens (so the existing
// auth.Verifier validates them) but carry {role:'admin', isAdmin:true} and use
// hardcoded TTLs (access 15m prod / 24h dev, refresh 7d) instead of the
// app_settings token_expiry values. The RolesGuard equivalent is
// shared.RequireAdmin.
package adminauth

import "time"

// AdminUser mirrors the admin_users row. password_hash is carried for
// verification and stripped by the handler before serialization.
type AdminUser struct {
	ID           string
	Email        string
	PasswordHash string
	Name         string
	IsActive     bool
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Error is a 401-class admin-auth failure carrying the exact Node message. The
// handler maps it to {error: Message, code: "invalid-token"} → 401, matching
// the NestJS UnauthorizedException → inferCode(401)="invalid-token".
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }

// The four admin-auth 401 messages, byte-identical to admin-auth.service.ts.
var (
	ErrInvalidCredentials = &Error{Message: "Invalid credentials"}           // bad email/password
	ErrAccountDisabled    = &Error{Message: "Account disabled"}              // !is_active
	ErrUnauthorized       = &Error{Message: "Unauthorized"}                  // bare UnauthorizedException() (admin row gone)
	ErrCurrentPwIncorrect = &Error{Message: "Current password is incorrect"} //nolint:misspell
)
