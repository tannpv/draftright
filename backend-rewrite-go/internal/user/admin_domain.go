package user

import (
	"encoding/json"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// UserDetail is the FULL user row as the admin endpoints serialise it:
// GET /admin/users/:id (the `user` field) and PATCH /admin/users/:id both
// return the complete TypeORM User entity. 22 columns in
// entity-declaration order (src/users/entities/user.entity.ts). The slim
// user.User is insufficient — this struct carries every column.
//
// Nullable columns render JSON null when absent. password_hash is
// nullable: true on the entity (OAuth-only users have NULL) so it is a
// pointer that emits null, NOT "". The two nullable timestamps render
// null or an ISO-millis string; the two non-null timestamps always render
// the ISO-millis string. MarshalJSON pins field order + ms timestamps.
type UserDetail struct {
	ID                       string
	Email                    string
	PasswordHash             *string
	Name                     string
	IsActive                 bool
	Role                     string
	AuthProvider             string
	GoogleID                 *string
	FacebookID               *string
	TiktokID                 *string
	AppleID                  *string
	AvatarURL                *string
	StripeCustomerID         *string
	EmailVerified            bool
	EmailVerificationCode    *string
	EmailVerificationExpires *time.Time
	PasswordResetCode        *string
	PasswordResetExpires     *time.Time
	PasswordResetAttempts    int
	LemonsqueezyCustomerID   *string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// UserPatchAdmin is the admin update payload — mirrors Node PATCH
// /admin/users/:id @Body UpdateUserDto, which accepts exactly is_active,
// role, name (all @IsOptional). A nil pointer = field unchanged (TypeORM
// partial .update()).
type UserPatchAdmin struct {
	IsActive *bool
	Role     *string
	Name     *string
}

// isoPtr renders a nullable timestamp as Node does: nil → JSON null,
// non-nil → the quoted ISO-millis string.
func isoPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := shared.ISOMillis(*t)
	return &s
}

// MarshalJSON pins field order + ms timestamps. The six secret columns
// (password_hash, email-verification code/expiry, password-reset
// code/expiry/attempts) are deliberately omitted from the wire form — no
// client ever writes them back, so they are dropped, not masked (#31). The
// Node backend strips the same six → byte-identical JSON.
func (u UserDetail) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID                     string  `json:"id"`
		Email                  string  `json:"email"`
		Name                   string  `json:"name"`
		IsActive               bool    `json:"is_active"`
		Role                   string  `json:"role"`
		AuthProvider           string  `json:"auth_provider"`
		GoogleID               *string `json:"google_id"`
		FacebookID             *string `json:"facebook_id"`
		TiktokID               *string `json:"tiktok_id"`
		AppleID                *string `json:"apple_id"`
		AvatarURL              *string `json:"avatar_url"`
		StripeCustomerID       *string `json:"stripe_customer_id"`
		EmailVerified          bool    `json:"email_verified"`
		LemonsqueezyCustomerID *string `json:"lemonsqueezy_customer_id"`
		CreatedAt              string  `json:"created_at"`
		UpdatedAt              string  `json:"updated_at"`
	}{
		ID:                     u.ID,
		Email:                  u.Email,
		Name:                   u.Name,
		IsActive:               u.IsActive,
		Role:                   u.Role,
		AuthProvider:           u.AuthProvider,
		GoogleID:               u.GoogleID,
		FacebookID:             u.FacebookID,
		TiktokID:               u.TiktokID,
		AppleID:                u.AppleID,
		AvatarURL:              u.AvatarURL,
		StripeCustomerID:       u.StripeCustomerID,
		EmailVerified:          u.EmailVerified,
		LemonsqueezyCustomerID: u.LemonsqueezyCustomerID,
		CreatedAt:              shared.ISOMillis(u.CreatedAt),
		UpdatedAt:              shared.ISOMillis(u.UpdatedAt),
	})
}
